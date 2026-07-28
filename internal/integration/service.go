package integration

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/azayr/selfhost/internal/secretbox"
	"github.com/azayr/selfhost/internal/store"
)

type Config struct {
	PublicURL          string
	GitLabClientID     string
	GitLabClientSecret string
	GitLabBaseURL      string
}

type Service struct {
	store *store.Store
	box   *secretbox.Box
	cfg   Config
	http  *http.Client
}

type Repository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	CloneURL      string `json:"cloneUrl"`
	DefaultBranch string `json:"defaultBranch"`
	Private       bool   `json:"private"`
	UpdatedAt     string `json:"updatedAt"`
}

type GitHubIdentity struct {
	AccountID string `json:"accountId"`
	Login     string `json:"login"`
	Avatar    string `json:"avatar"`
}

type GitHubManifestStart struct {
	Action   string
	State    string
	Manifest string
	Purpose  string
}

type githubOAuthCredentials struct {
	clientID     string
	clientSecret string
}

type githubInstallationRemote struct {
	ID                  int64             `json:"id"`
	RepositorySelection string            `json:"repository_selection"`
	HTMLURL             string            `json:"html_url"`
	Permissions         map[string]string `json:"permissions"`
	Account             struct {
		ID        json.Number `json:"id"`
		Login     string      `json:"login"`
		AvatarURL string      `json:"avatar_url"`
	} `json:"account"`
}

var ErrGitHubAccountNotConfigured = errors.New("GitHub account authorization is not configured")
var errGitHubAppRemoved = errors.New("the configured GitHub App no longer exists")

const (
	githubRepositoryAppProvider = "github"
	githubLoginAppProvider      = "github_login"
	githubAccountLinkMode       = "account_link"
	githubRepositoryInstallMode = "repository_install"
)

func New(s *store.Store, box *secretbox.Box, cfg Config) *Service {
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	cfg.GitLabBaseURL = strings.TrimRight(cfg.GitLabBaseURL, "/")
	return &Service{store: s, box: box, cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

func (s *Service) ProviderStatus(ctx context.Context) (map[string]any, error) {
	githubConfig, err := s.store.ProviderAppConfig(ctx, githubRepositoryAppProvider)
	githubConfigured := err == nil
	if err != nil && !store.NotFound(err) {
		return nil, err
	}
	_, loginErr := s.store.ProviderAppConfig(ctx, githubLoginAppProvider)
	loginConfigured := loginErr == nil
	if loginErr != nil && !store.NotFound(loginErr) {
		return nil, loginErr
	}
	githubSlug := ""
	if githubConfigured {
		githubSlug = githubConfig.AppSlug
	}
	return map[string]any{
		"github": map[string]any{
			"configured":      githubConfigured,
			"managed":         githubConfigured,
			"appSlug":         githubSlug,
			"loginConfigured": loginConfigured,
			"callbackUrl":     s.installationCallbackURL(),
		},
		"gitlab": map[string]any{"configured": s.cfg.GitLabClientID != "" && s.cfg.GitLabClientSecret != "", "callbackUrl": s.callbackURL("gitlab"), "baseUrl": s.cfg.GitLabBaseURL},
	}, nil
}

func (s *Service) AccountProviderStatus(ctx context.Context) (map[string]any, error) {
	config, err := s.store.ProviderAppConfig(ctx, githubLoginAppProvider)
	configured := err == nil
	if err != nil && !store.NotFound(err) {
		return nil, err
	}
	appSlug := ""
	if configured {
		appSlug = config.AppSlug
	}
	return map[string]any{
		"github": map[string]any{
			"configured":  configured,
			"callbackUrl": s.accountCallbackURL(),
			"managed":     configured,
			"appSlug":     appSlug,
		},
	}, nil
}

func (s *Service) StartGitHubAccountOAuth(ctx context.Context, userID, mode string) (string, error) {
	credentials, err := s.githubOAuthCredentials(ctx, githubLoginAppProvider)
	if err != nil {
		if store.NotFound(err) {
			return "", ErrGitHubAccountNotConfigured
		}
		return "", err
	}
	if err := s.ensureGitHubAppAvailable(ctx, githubLoginAppProvider); err != nil {
		return "", err
	}
	if mode != "login" && mode != "link" {
		return "", errors.New("invalid GitHub OAuth mode")
	}
	if mode == "link" && userID == "" {
		return "", errors.New("linking a GitHub account requires authentication")
	}
	randomState, err := randomToken(32)
	if err != nil {
		return "", err
	}
	state := "account." + randomState
	hash := sha256.Sum256([]byte(state))
	if err := s.store.SaveAuthOAuthState(ctx, store.AuthOAuthState{
		StateHash: hex.EncodeToString(hash[:]),
		UserID:    userID,
		Provider:  "github",
		Mode:      mode,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}); err != nil {
		return "", err
	}
	values := url.Values{
		"client_id":    {credentials.clientID},
		"redirect_uri": {s.accountCallbackURL()},
		"state":        {state},
	}
	return "https://github.com/login/oauth/authorize?" + values.Encode(), nil
}

func (s *Service) CompleteGitHubAccountOAuth(ctx context.Context, stateValue, code string) (store.AuthOAuthState, GitHubIdentity, error) {
	if stateValue == "" || code == "" {
		return store.AuthOAuthState{}, GitHubIdentity{}, errors.New("GitHub OAuth callback is missing state or code")
	}
	hash := sha256.Sum256([]byte(stateValue))
	state, err := s.store.ConsumeAuthOAuthState(ctx, hex.EncodeToString(hash[:]), "github")
	if err != nil {
		return store.AuthOAuthState{}, GitHubIdentity{}, errors.New("GitHub OAuth state is invalid or expired")
	}
	credentials, err := s.githubOAuthCredentials(ctx, githubLoginAppProvider)
	if err != nil {
		if store.NotFound(err) {
			return store.AuthOAuthState{}, GitHubIdentity{}, ErrGitHubAccountNotConfigured
		}
		return store.AuthOAuthState{}, GitHubIdentity{}, err
	}
	token, _, err := s.exchangeWithCredentials(ctx, "github", code, s.accountCallbackURL(), credentials.clientID, credentials.clientSecret)
	if err != nil {
		return store.AuthOAuthState{}, GitHubIdentity{}, err
	}
	profile, err := s.profile(ctx, "github", token)
	if err != nil {
		return store.AuthOAuthState{}, GitHubIdentity{}, err
	}
	return state, GitHubIdentity{AccountID: profile.id, Login: profile.name, Avatar: profile.avatar}, nil
}

func (s *Service) StartGitHubLoginManifest(ctx context.Context, userID string) (GitHubManifestStart, error) {
	return s.startGitHubManifest(ctx, userID, githubAccountLinkMode)
}

func (s *Service) StartGitHubManifest(ctx context.Context, userID string) (GitHubManifestStart, error) {
	return s.startGitHubManifest(ctx, userID, githubRepositoryInstallMode)
}

func (s *Service) startGitHubManifest(ctx context.Context, userID, mode string) (GitHubManifestStart, error) {
	if userID == "" {
		return GitHubManifestStart{}, errors.New("GitHub App setup requires authentication")
	}
	if mode != githubAccountLinkMode && mode != githubRepositoryInstallMode {
		return GitHubManifestStart{}, errors.New("invalid GitHub App setup mode")
	}
	randomState, err := randomToken(32)
	if err != nil {
		return GitHubManifestStart{}, err
	}
	statePrefix := "manifest.repository."
	if mode == githubAccountLinkMode {
		statePrefix = "manifest.login."
	}
	stateValue := statePrefix + randomState
	hash := sha256.Sum256([]byte(stateValue))
	if err := s.store.SaveProviderSetupState(ctx, store.ProviderSetupState{
		StateHash: hex.EncodeToString(hash[:]),
		UserID:    userID,
		Provider:  githubRepositoryAppProvider,
		Mode:      mode,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		return GitHubManifestStart{}, err
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return GitHubManifestStart{}, err
	}
	manifest, purpose, err := s.githubManifest(mode, hex.EncodeToString(suffix))
	if err != nil {
		return GitHubManifestStart{}, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return GitHubManifestStart{}, err
	}
	return GitHubManifestStart{
		Action:   "https://github.com/settings/apps/new?state=" + url.QueryEscape(stateValue),
		State:    stateValue,
		Manifest: string(encoded),
		Purpose:  purpose,
	}, nil
}

func (s *Service) githubManifest(mode, suffix string) (map[string]any, string, error) {
	manifest := map[string]any{
		"url":    s.cfg.PublicURL,
		"public": true,
	}
	purpose := "Create a public GitHub App that users can install for explicitly selected repositories."
	if mode == githubAccountLinkMode {
		manifest["name"] = "dokyr-login-" + suffix
		manifest["redirect_url"] = s.loginManifestCallbackURL()
		manifest["callback_urls"] = []string{s.accountCallbackURL()}
		manifest["description"] = "Identity-only GitHub login for this Dokyr control plane."
		purpose = "Create a public, identity-only GitHub App for login. It requests no repository permissions."
	} else if mode == githubRepositoryInstallMode {
		manifest["name"] = "dokyr-selfhost-" + suffix
		manifest["redirect_url"] = s.manifestCallbackURL()
		manifest["setup_url"] = s.installationCallbackURL()
		manifest["setup_on_update"] = true
		manifest["description"] = "Selected repository access for this Dokyr control plane."
		manifest["hook_attributes"] = map[string]any{
			"url":    s.cfg.PublicURL + "/api/webhooks/github",
			"active": true,
		}
		manifest["default_events"] = []string{"push"}
		manifest["default_permissions"] = map[string]string{
			"metadata": "read",
			"contents": "read",
		}
	} else {
		return nil, "", errors.New("invalid GitHub App setup mode")
	}
	return manifest, purpose, nil
}

func (s *Service) StartGitHubInstallation(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", errors.New("GitHub repository selection requires authentication")
	}
	config, err := s.store.ProviderAppConfig(ctx, githubRepositoryAppProvider)
	if store.NotFound(err) {
		return "", ErrGitHubAccountNotConfigured
	}
	if err != nil {
		return "", err
	}
	if err := s.ensureGitHubAppAvailable(ctx, githubRepositoryAppProvider); err != nil {
		return "", err
	}
	stateValue, err := randomToken(32)
	if err != nil {
		return "", err
	}
	stateValue = "install." + stateValue
	hash := sha256.Sum256([]byte(stateValue))
	if err := s.store.SaveProviderSetupState(ctx, store.ProviderSetupState{
		StateHash: hex.EncodeToString(hash[:]), UserID: userID, Provider: githubRepositoryAppProvider, Mode: githubRepositoryInstallMode, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		return "", err
	}
	return "https://github.com/apps/" + url.PathEscape(config.AppSlug) + "/installations/new?state=" + url.QueryEscape(stateValue), nil
}

func (s *Service) CompleteGitHubInstallation(ctx context.Context, stateValue string, installationID int64) (store.SourceConnection, error) {
	if installationID <= 0 {
		return store.SourceConnection{}, errors.New("GitHub installation callback is missing an installation ID")
	}
	userID := ""
	if stateValue != "" {
		hash := sha256.Sum256([]byte(stateValue))
		state, err := s.store.ConsumeProviderSetupState(ctx, hex.EncodeToString(hash[:]), githubRepositoryAppProvider)
		if err != nil || state.Mode != githubRepositoryInstallMode {
			return store.SourceConnection{}, errors.New("GitHub installation state is invalid or expired")
		}
		userID = state.UserID
	} else {
		existing, err := s.store.GitHubInstallation(ctx, installationID)
		if err != nil {
			return store.SourceConnection{}, errors.New("GitHub installation update is not recognized")
		}
		userID = existing.UserID
	}
	remote, err := s.githubInstallation(ctx, installationID)
	if err != nil {
		return store.SourceConnection{}, err
	}
	return s.saveGitHubInstallation(ctx, userID, remote)
}

func (s *Service) saveGitHubInstallation(ctx context.Context, userID string, remote githubInstallationRemote) (store.SourceConnection, error) {
	if userID == "" || remote.ID <= 0 || remote.Account.ID == "" {
		return store.SourceConnection{}, errors.New("GitHub installation identity is incomplete")
	}
	manageURL := remote.HTMLURL
	if manageURL == "" {
		manageURL = "https://github.com/settings/installations/" + strconv.FormatInt(remote.ID, 10)
	}
	connection := store.SourceConnection{
		ID: newID("src"), UserID: userID, Provider: "github", AccountID: string(remote.Account.ID), AccountName: remote.Account.Login,
		AccountAvatar: remote.Account.AvatarURL, BaseURL: "https://github.com", Scopes: "github_app_installation",
	}
	connectionID, err := s.store.UpsertSourceConnectionReturningID(ctx, connection)
	if err != nil {
		return store.SourceConnection{}, err
	}
	connection.ID = connectionID
	connection.InstallationID = remote.ID
	connection.RepositorySelection = remote.RepositorySelection
	connection.ManageURL = manageURL
	connection.ContentsPermission = strings.ToLower(remote.Permissions["contents"])
	if err := s.store.UpsertGitHubInstallation(ctx, store.GitHubInstallation{
		InstallationID: remote.ID, ConnectionID: connectionID, UserID: userID, AccountID: connection.AccountID,
		AccountLogin: connection.AccountName, AccountAvatar: connection.AccountAvatar,
		RepositorySelection: remote.RepositorySelection, ManageURL: manageURL, ContentsPermission: connection.ContentsPermission,
	}); err != nil {
		return store.SourceConnection{}, err
	}
	return connection, nil
}

// SyncGitHubInstallations recovers personal GitHub App installations when the
// browser did not return through the installation setup callback. Only an
// installation whose account id exactly matches the GitHub identity already
// linked to this Dokyr user is imported.
func (s *Service) SyncGitHubInstallations(ctx context.Context, userID, linkedAccountID string) ([]store.SourceConnection, string, error) {
	if userID == "" || linkedAccountID == "" {
		return nil, "", errors.New("link your GitHub account before synchronizing repository access")
	}
	jwt, err := s.githubAppJWT(ctx, githubRepositoryAppProvider)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/app/installations?per_page=100", nil)
	if err != nil {
		return nil, "", err
	}
	s.bearer(req, jwt, "github")
	var installed []githubInstallationRemote
	if err := s.doJSON(req, &installed); err != nil {
		return nil, "", fmt.Errorf("list GitHub App installations: %w", err)
	}
	connections := []store.SourceConnection{}
	warning := ""
	for _, remote := range installed {
		if string(remote.Account.ID) != linkedAccountID {
			continue
		}
		connection, err := s.saveGitHubInstallation(ctx, userID, remote)
		if err != nil {
			return nil, "", err
		}
		connections = append(connections, connection)
		if permission := strings.ToLower(remote.Permissions["contents"]); permission != "read" && permission != "write" {
			warning = "The GitHub App can list repositories but needs Contents: Read permission before it can clone and deploy private repositories."
		}
	}
	return connections, warning, nil
}

func (s *Service) CompleteGitHubManifest(ctx context.Context, stateValue, code string) (store.ProviderSetupState, error) {
	if stateValue == "" || code == "" {
		return store.ProviderSetupState{}, errors.New("GitHub App setup callback is missing state or code")
	}
	hash := sha256.Sum256([]byte(stateValue))
	state, err := s.store.ConsumeProviderSetupState(ctx, hex.EncodeToString(hash[:]), githubRepositoryAppProvider)
	if err != nil {
		return store.ProviderSetupState{}, errors.New("GitHub App setup state is invalid or expired")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/app-manifests/"+url.PathEscape(code)+"/conversions", nil)
	if err != nil {
		return store.ProviderSetupState{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "selfhost-control-plane")
	var converted struct {
		ID            json.Number `json:"id"`
		Slug          string      `json:"slug"`
		ClientID      string      `json:"client_id"`
		ClientSecret  string      `json:"client_secret"`
		PEM           string      `json:"pem"`
		WebhookSecret string      `json:"webhook_secret"`
	}
	if err := s.doJSON(request, &converted); err != nil {
		return store.ProviderSetupState{}, fmt.Errorf("convert GitHub App manifest: %w", err)
	}
	if converted.ID == "" || converted.ClientID == "" || converted.ClientSecret == "" {
		return store.ProviderSetupState{}, errors.New("GitHub did not return complete app credentials")
	}
	clientID, err := s.box.Encrypt(converted.ClientID)
	if err != nil {
		return store.ProviderSetupState{}, err
	}
	clientSecret, err := s.box.Encrypt(converted.ClientSecret)
	if err != nil {
		return store.ProviderSetupState{}, err
	}
	privateKey, err := s.box.Encrypt(converted.PEM)
	if err != nil {
		return store.ProviderSetupState{}, err
	}
	webhookSecret, err := s.box.Encrypt(converted.WebhookSecret)
	if err != nil {
		return store.ProviderSetupState{}, err
	}
	providerApp := githubRepositoryAppProvider
	if state.Mode == githubAccountLinkMode {
		providerApp = githubLoginAppProvider
	} else if state.Mode != githubRepositoryInstallMode {
		return store.ProviderSetupState{}, errors.New("GitHub App setup mode is invalid")
	}
	if err := s.store.UpsertProviderAppConfig(ctx, store.ProviderAppConfig{
		Provider:               providerApp,
		AppID:                  string(converted.ID),
		AppSlug:                converted.Slug,
		ClientIDEncrypted:      clientID,
		ClientSecretEncrypted:  clientSecret,
		PrivateKeyEncrypted:    privateKey,
		WebhookSecretEncrypted: webhookSecret,
		CreatedBy:              state.UserID,
	}); err != nil {
		return store.ProviderSetupState{}, err
	}
	return state, nil
}

func (s *Service) githubOAuthCredentials(ctx context.Context, providerApp string) (githubOAuthCredentials, error) {
	config, err := s.store.ProviderAppConfig(ctx, providerApp)
	if err != nil {
		return githubOAuthCredentials{}, err
	}
	clientID, err := s.box.Decrypt(config.ClientIDEncrypted)
	if err != nil {
		return githubOAuthCredentials{}, err
	}
	clientSecret, err := s.box.Decrypt(config.ClientSecretEncrypted)
	if err != nil {
		return githubOAuthCredentials{}, err
	}
	return githubOAuthCredentials{clientID: clientID, clientSecret: clientSecret}, nil
}

func (s *Service) Start(ctx context.Context, userID, provider string) (string, error) {
	if provider != "gitlab" {
		return "", errors.New("unsupported source provider")
	}
	clientID, clientSecret := s.cfg.GitLabClientID, s.cfg.GitLabClientSecret
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("%s OAuth is not configured", provider)
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(state))
	if err := s.store.SaveOAuthState(ctx, store.OAuthState{StateHash: hex.EncodeToString(hash[:]), UserID: userID, Provider: provider, ExpiresAt: time.Now().Add(10 * time.Minute)}); err != nil {
		return "", err
	}
	callback := s.callbackURL(provider)
	var endpoint string
	values := url.Values{"client_id": {clientID}, "redirect_uri": {callback}, "state": {state}, "response_type": {"code"}}
	switch provider {
	case "gitlab":
		endpoint = s.cfg.GitLabBaseURL + "/oauth/authorize"
		values.Set("scope", "read_api read_repository")
	default:
		return "", errors.New("unsupported source provider")
	}
	return endpoint + "?" + values.Encode(), nil
}

func (s *Service) Complete(ctx context.Context, provider, stateValue, code string) error {
	if provider != "gitlab" {
		return errors.New("unsupported source provider")
	}
	if stateValue == "" || code == "" {
		return errors.New("OAuth callback is missing state or code")
	}
	hash := sha256.Sum256([]byte(stateValue))
	state, err := s.store.ConsumeOAuthState(ctx, hex.EncodeToString(hash[:]), provider)
	if err != nil {
		return errors.New("OAuth state is invalid or expired")
	}
	token, scopes, err := s.exchange(ctx, provider, code)
	if err != nil {
		return err
	}
	profile, err := s.profile(ctx, provider, token)
	if err != nil {
		return err
	}
	sealed, err := s.box.Encrypt(token)
	if err != nil {
		return err
	}
	return s.store.UpsertSourceConnection(ctx, store.SourceConnection{ID: newID("src"), UserID: state.UserID, Provider: provider, AccountID: profile.id, AccountName: profile.name, AccountAvatar: profile.avatar, BaseURL: s.providerBase(provider), AccessTokenEncrypted: sealed, Scopes: scopes})
}

func (s *Service) Repositories(ctx context.Context, connection store.SourceConnection) ([]Repository, error) {
	if connection.Provider == "github" && connection.InstallationID > 0 {
		return s.githubInstallationRepositories(ctx, connection.InstallationID)
	}
	token, err := s.box.Decrypt(connection.AccessTokenEncrypted)
	if err != nil {
		return nil, err
	}
	if connection.Provider == "github" {
		return s.githubRepositories(ctx, token)
	}
	if connection.Provider == "gitlab" {
		return s.gitlabRepositories(ctx, connection.BaseURL, token)
	}
	return nil, errors.New("unsupported source provider")
}

func (s *Service) Repository(ctx context.Context, connection store.SourceConnection, fullName string) (Repository, error) {
	repositories, err := s.Repositories(ctx, connection)
	if err != nil {
		return Repository{}, err
	}
	for _, repository := range repositories {
		if repository.FullName == fullName {
			return repository, nil
		}
	}
	return Repository{}, errors.New("repository is not available to the connected account")
}

func (s *Service) RepositoryToken(ctx context.Context, connection store.SourceConnection) (string, error) {
	if connection.Provider == "github" && connection.InstallationID > 0 {
		return s.githubInstallationToken(ctx, connection.InstallationID, true)
	}
	return s.box.Decrypt(connection.AccessTokenEncrypted)
}

type lineWriter struct {
	mu       sync.Mutex
	buffer   string
	progress func(string)
}

func (w *lineWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer += string(value)
	for {
		line, remaining, found := strings.Cut(w.buffer, "\n")
		if !found {
			break
		}
		w.buffer = remaining
		if line = strings.TrimSpace(line); line != "" && w.progress != nil {
			w.progress(line)
		}
	}
	return len(value), nil
}

func (s *Service) CloneRepository(ctx context.Context, connection store.SourceConnection, repository Repository, branch, destination string, progress func(string)) error {
	token, err := s.RepositoryToken(ctx, connection)
	if err != nil {
		return fmt.Errorf("create repository credential: %w", err)
	}
	if !strings.HasPrefix(repository.CloneURL, "https://") {
		return errors.New("only HTTPS repository clone URLs are supported")
	}
	credentialsDir, err := os.MkdirTemp("", "selfhost-git-credentials-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(credentialsDir)
	askPassPath := credentialsDir + "/askpass.sh"
	askPass := "#!/bin/sh\ncase \"$1\" in\n  *Username*) printf '%s\\n' \"$SELFHOST_GIT_USERNAME\" ;;\n  *) printf '%s\\n' \"$SELFHOST_GIT_TOKEN\" ;;\nesac\n"
	if err := os.WriteFile(askPassPath, []byte(askPass), 0700); err != nil {
		return err
	}
	username := "oauth2"
	if connection.Provider == "github" {
		username = "x-access-token"
	}
	if progress != nil {
		progress("Cloning " + repository.FullName + " at " + branch)
	}
	output := &lineWriter{progress: progress}
	command := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", "--branch", branch, repository.CloneURL, destination)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="+askPassPath, "SELFHOST_GIT_USERNAME="+username, "SELFHOST_GIT_TOKEN="+token)
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return fmt.Errorf("git clone %s: %w", repository.FullName, err)
	}
	return nil
}

type profile struct{ id, name, avatar string }

func (s *Service) exchange(ctx context.Context, provider, code string) (string, string, error) {
	return s.exchangeWithCredentials(ctx, provider, code, s.callbackURL(provider), s.cfg.GitLabClientID, s.cfg.GitLabClientSecret)
}

func (s *Service) exchangeWithCredentials(ctx context.Context, provider, code, callback, clientID, clientSecret string) (string, string, error) {
	values := url.Values{"client_id": {clientID}, "client_secret": {clientSecret}, "code": {code}, "redirect_uri": {callback}, "grant_type": {"authorization_code"}}
	endpoint := "https://github.com/login/oauth/access_token"
	if provider == "gitlab" {
		endpoint = s.cfg.GitLabBaseURL + "/oauth/token"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var out struct {
		AccessToken      string `json:"access_token"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := s.doJSON(req, &out); err != nil {
		return "", "", err
	}
	if out.AccessToken == "" {
		return "", "", fmt.Errorf("OAuth token exchange failed: %s %s", out.Error, out.ErrorDescription)
	}
	return out.AccessToken, out.Scope, nil
}

func (s *Service) profile(ctx context.Context, provider, token string) (profile, error) {
	endpoint := "https://api.github.com/user"
	if provider == "gitlab" {
		endpoint = s.cfg.GitLabBaseURL + "/api/v4/user"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return profile{}, err
	}
	s.bearer(req, token, provider)
	var out struct {
		ID        json.Number `json:"id"`
		Login     string      `json:"login"`
		Username  string      `json:"username"`
		Name      string      `json:"name"`
		AvatarURL string      `json:"avatar_url"`
	}
	if err := s.doJSON(req, &out); err != nil {
		return profile{}, err
	}
	name := out.Login
	if name == "" {
		name = out.Username
	}
	if name == "" {
		name = out.Name
	}
	return profile{id: string(out.ID), name: name, avatar: out.AvatarURL}, nil
}

func (s *Service) githubRepositories(ctx context.Context, token string) ([]Repository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/repos?visibility=all&affiliation=owner,collaborator,organization_member&sort=updated&per_page=100", nil)
	if err != nil {
		return nil, err
	}
	s.bearer(req, token, "github")
	var raw []struct {
		ID            json.Number `json:"id"`
		Name          string      `json:"name"`
		FullName      string      `json:"full_name"`
		CloneURL      string      `json:"clone_url"`
		DefaultBranch string      `json:"default_branch"`
		Private       bool        `json:"private"`
		UpdatedAt     string      `json:"updated_at"`
	}
	if err := s.doJSON(req, &raw); err != nil {
		return nil, err
	}
	items := make([]Repository, 0, len(raw))
	for _, r := range raw {
		items = append(items, Repository{ID: string(r.ID), Name: r.Name, FullName: r.FullName, CloneURL: r.CloneURL, DefaultBranch: r.DefaultBranch, Private: r.Private, UpdatedAt: r.UpdatedAt})
	}
	return items, nil
}

func (s *Service) githubInstallationRepositories(ctx context.Context, installationID int64) ([]Repository, error) {
	token, err := s.githubInstallationToken(ctx, installationID, false)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	s.bearer(req, token, "github")
	var out struct {
		Repositories []struct {
			ID            json.Number `json:"id"`
			Name          string      `json:"name"`
			FullName      string      `json:"full_name"`
			CloneURL      string      `json:"clone_url"`
			DefaultBranch string      `json:"default_branch"`
			Private       bool        `json:"private"`
			UpdatedAt     string      `json:"updated_at"`
		} `json:"repositories"`
	}
	if err := s.doJSON(req, &out); err != nil {
		return nil, err
	}
	items := make([]Repository, 0, len(out.Repositories))
	for _, repository := range out.Repositories {
		items = append(items, Repository{ID: string(repository.ID), Name: repository.Name, FullName: repository.FullName, CloneURL: repository.CloneURL, DefaultBranch: repository.DefaultBranch, Private: repository.Private, UpdatedAt: repository.UpdatedAt})
	}
	return items, nil
}

func (s *Service) githubInstallation(ctx context.Context, installationID int64) (githubInstallationRemote, error) {
	jwt, err := s.githubAppJWT(ctx, githubRepositoryAppProvider)
	if err != nil {
		return githubInstallationRemote{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/app/installations/"+strconv.FormatInt(installationID, 10), nil)
	if err != nil {
		return githubInstallationRemote{}, err
	}
	s.bearer(req, jwt, "github")
	var installation githubInstallationRemote
	if err := s.doJSON(req, &installation); err != nil {
		return installation, fmt.Errorf("read GitHub installation: %w", err)
	}
	return installation, nil
}

func (s *Service) githubInstallationToken(ctx context.Context, installationID int64, requireContents bool) (string, error) {
	jwt, err := s.githubAppJWT(ctx, githubRepositoryAppProvider)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/app/installations/"+strconv.FormatInt(installationID, 10)+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	s.bearer(req, jwt, "github")
	var out struct {
		Token       string            `json:"token"`
		Permissions map[string]string `json:"permissions"`
	}
	if err := s.doJSON(req, &out); err != nil {
		return "", fmt.Errorf("create GitHub installation token: %w", err)
	}
	if out.Token == "" {
		return "", errors.New("GitHub returned an empty installation token")
	}
	if requireContents {
		permission := strings.ToLower(out.Permissions["contents"])
		if permission != "read" && permission != "write" {
			return "", errors.New("GitHub App permission missing: enable Repository permissions → Contents: Read-only, then approve the permission update before deploying this private repository")
		}
	}
	return out.Token, nil
}

func (s *Service) githubAppJWT(ctx context.Context, providerApp string) (string, error) {
	config, err := s.store.ProviderAppConfig(ctx, providerApp)
	if err != nil {
		return "", err
	}
	privatePEM, err := s.box.Decrypt(config.PrivateKeyEncrypted)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return "", errors.New("GitHub App private key is invalid")
	}
	var key *rsa.PrivateKey
	if parsed, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes); parseErr == nil {
		key = parsed
	} else {
		parsedAny, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return "", errors.New("GitHub App private key format is unsupported")
		}
		var ok bool
		key, ok = parsedAny.(*rsa.PrivateKey)
		if !ok {
			return "", errors.New("GitHub App private key is not RSA")
		}
	}
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(8 * time.Minute).Unix(), "iss": config.AppID})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *Service) ensureGitHubAppAvailable(ctx context.Context, providerApp string) error {
	jwt, err := s.githubAppJWT(ctx, providerApp)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/app", nil)
	if err != nil {
		return err
	}
	s.bearer(req, jwt, "github")
	response, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("verify GitHub App: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return err
	}
	if err := githubAppResponseError(response.StatusCode, response.Status, body); err != nil {
		if errors.Is(err, errGitHubAppRemoved) {
			if resetErr := s.store.ResetProviderAppConfig(ctx, providerApp); resetErr != nil {
				return fmt.Errorf("reset removed GitHub App: %w", resetErr)
			}
			return ErrGitHubAccountNotConfigured
		}
		return err
	}
	return nil
}

func githubAppResponseError(statusCode int, status string, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusNotFound {
		return errGitHubAppRemoved
	}
	return fmt.Errorf("verify GitHub App returned %s: %s", status, strings.TrimSpace(string(bytes.TrimSpace(body))))
}

func (s *Service) gitlabRepositories(ctx context.Context, baseURL, token string) ([]Repository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v4/projects?membership=true&simple=true&order_by=last_activity_at&sort=desc&per_page=100", nil)
	if err != nil {
		return nil, err
	}
	s.bearer(req, token, "gitlab")
	var raw []struct {
		ID                int64  `json:"id"`
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		HTTPURL           string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
		Visibility        string `json:"visibility"`
		LastActivityAt    string `json:"last_activity_at"`
	}
	if err := s.doJSON(req, &raw); err != nil {
		return nil, err
	}
	items := make([]Repository, 0, len(raw))
	for _, r := range raw {
		items = append(items, Repository{ID: strconv.FormatInt(r.ID, 10), Name: r.Name, FullName: r.PathWithNamespace, CloneURL: r.HTTPURL, DefaultBranch: r.DefaultBranch, Private: r.Visibility != "public", UpdatedAt: r.LastActivityAt})
	}
	return items, nil
}

func (s *Service) doJSON(req *http.Request, out any) error {
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(bytes.TrimSpace(body))))
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	return dec.Decode(out)
}

func (s *Service) bearer(req *http.Request, token, provider string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "selfhost-control-plane")
	if provider == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
}

func (s *Service) callbackURL(provider string) string {
	return s.cfg.PublicURL + "/api/integrations/oauth/" + provider + "/callback"
}
func (s *Service) accountCallbackURL() string {
	return s.cfg.PublicURL + "/api/auth/github/callback"
}
func (s *Service) loginManifestCallbackURL() string {
	return s.cfg.PublicURL + "/api/auth/github/manifest/callback"
}
func (s *Service) manifestCallbackURL() string {
	return s.cfg.PublicURL + "/api/integrations/github/manifest/callback"
}
func (s *Service) installationCallbackURL() string {
	return s.cfg.PublicURL + "/api/integrations/github/install/callback"
}
func (s *Service) providerBase(provider string) string {
	if provider == "github" {
		return "https://github.com"
	}
	return s.cfg.GitLabBaseURL
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func newID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}
