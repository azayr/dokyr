package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/caddy"
	"github.com/azayr/selfhost/internal/config"
	"github.com/azayr/selfhost/internal/integration"
	"github.com/azayr/selfhost/internal/mailer"
	"github.com/azayr/selfhost/internal/platformupdate"
	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/secretbox"
	"github.com/azayr/selfhost/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type API struct {
	store             *store.Store
	docker            *runtime.Docker
	auth              *auth.Manager
	integrations      *integration.Service
	box               *secretbox.Box
	log               *slog.Logger
	caddy             *caddy.Client
	publicURL         string
	domainMu          sync.Mutex
	databaseMu        sync.Mutex
	applicationMu     sync.Mutex
	projectMu         sync.Mutex
	cleanupMu         sync.Mutex
	deploymentMu      sync.Mutex
	updateMu          sync.Mutex
	updateExecutionMu sync.Mutex
	updates           *platformupdate.Client
	latestRelease     *platformupdate.Release
	deploymentCancels map[string]context.CancelFunc
}

type domainRuleInput struct {
	Path      string `json:"path"`
	Port      int    `json:"port"`
	ServiceID string `json:"serviceId"`
}

type domainBindingInput struct {
	Domain       string            `json:"domain"`
	HTTPSEnabled bool              `json:"httpsEnabled"`
	Rules        []domainRuleInput `json:"rules"`
}

func New(s *store.Store, d *runtime.Docker, a *auth.Manager, integrations *integration.Service, box *secretbox.Box, caddyClient *caddy.Client, updates *platformupdate.Client, publicURL string, log *slog.Logger) *API {
	return &API{
		store:             s,
		docker:            d,
		auth:              a,
		integrations:      integrations,
		box:               box,
		caddy:             caddyClient,
		updates:           updates,
		publicURL:         strings.TrimRight(publicURL, "/"),
		log:               log,
		deploymentCancels: make(map[string]context.CancelFunc),
	}
}
func (a *API) Handler() http.Handler {
	public := http.NewServeMux()
	protected := http.NewServeMux()
	public.HandleFunc("GET /api/health", a.health)
	public.HandleFunc("GET /api/setup/status", a.setupStatus)
	public.HandleFunc("POST /api/setup", a.setup)
	public.HandleFunc("POST /api/auth/login", a.login)
	public.HandleFunc("GET /api/auth/password-reset/status", a.passwordResetStatus)
	public.HandleFunc("POST /api/auth/password-reset/request", a.requestPasswordReset)
	public.HandleFunc("POST /api/auth/password-reset/confirm", a.confirmPasswordReset)
	public.HandleFunc("GET /api/auth/providers", a.authProviders)
	public.HandleFunc("POST /api/auth/2fa", a.completeTwoFactorLogin)
	public.HandleFunc("POST /api/auth/logout", a.logout)
	public.HandleFunc("GET /api/auth/github/start", a.githubLoginStart)
	public.HandleFunc("GET /api/auth/github/callback", a.githubAccountCallback)
	public.HandleFunc("GET /api/auth/github/manifest/callback", a.githubManifestCallback)
	public.HandleFunc("GET /api/integrations/github/install/callback", a.githubInstallationCallback)
	public.HandleFunc("GET /api/integrations/oauth/{provider}/callback", a.oauthCallback)
	public.HandleFunc("POST /api/webhooks/github", a.githubWebhook)
	public.HandleFunc("POST /api/webhooks/registry/{id}/{token}", a.registryWebhook)
	protected.HandleFunc("GET /api/auth/me", a.me)
	protected.HandleFunc("GET /api/account/security", a.accountSecurity)
	protected.HandleFunc("PUT /api/account/password", a.updatePassword)
	protected.HandleFunc("POST /api/account/2fa/setup", a.setupTwoFactor)
	protected.HandleFunc("POST /api/account/2fa/confirm", a.confirmTwoFactor)
	protected.HandleFunc("DELETE /api/account/2fa", a.disableTwoFactor)
	protected.HandleFunc("GET /api/account/github/start", a.githubLinkStart)
	protected.HandleFunc("DELETE /api/account/github", a.unlinkGitHub)
	protected.HandleFunc("GET /api/settings/smtp", a.smtpSettings)
	protected.HandleFunc("PUT /api/settings/smtp", a.updateSMTPSettings)
	protected.HandleFunc("POST /api/settings/smtp/test", a.testSMTPSettings)
	protected.HandleFunc("GET /api/settings/platform/update", a.platformUpdateStatusHandler)
	protected.HandleFunc("POST /api/settings/platform/update/check", a.checkPlatformUpdate)
	protected.HandleFunc("PUT /api/settings/platform/update", a.updatePlatformUpdateSettings)
	protected.HandleFunc("POST /api/settings/platform/update/apply", a.applyPlatformUpdate)
	protected.HandleFunc("GET /api/dashboard", a.dashboard)
	protected.HandleFunc("GET /api/projects", a.projects)
	protected.HandleFunc("POST /api/projects", a.createProject)
	protected.HandleFunc("GET /api/projects/{id}", a.project)
	protected.HandleFunc("PUT /api/projects/{id}", a.updateProject)
	protected.HandleFunc("DELETE /api/projects/{id}", a.deleteProject)
	protected.HandleFunc("PUT /api/projects/{id}/domain", a.updateProjectDomain)
	protected.HandleFunc("POST /api/projects/{id}/deploy", a.deployProject)
	protected.HandleFunc("POST /api/projects/{id}/stop", a.stopProjectService)
	protected.HandleFunc("POST /api/projects/{id}/restart", a.restartProjectService)
	protected.HandleFunc("GET /api/projects/{id}/logs", a.projectLogs)
	protected.HandleFunc("GET /api/projects/{id}/metrics", a.projectMetrics)
	protected.HandleFunc("GET /api/projects/{id}/environment", a.projectEnvironment)
	protected.HandleFunc("PUT /api/projects/{id}/environment", a.updateProjectEnvironment)
	protected.HandleFunc("POST /api/projects/{id}/databases", a.createDatabaseService)
	protected.HandleFunc("POST /api/projects/{id}/services", a.createApplicationService)
	protected.HandleFunc("PUT /api/services/{id}", a.updateApplicationService)
	protected.HandleFunc("POST /api/services/{id}/deploy", a.deployApplicationService)
	protected.HandleFunc("POST /api/services/{id}/stop", a.stopApplicationService)
	protected.HandleFunc("POST /api/services/{id}/restart", a.restartApplicationService)
	protected.HandleFunc("POST /api/services/{id}/exec", a.executeApplicationServiceCommand)
	protected.HandleFunc("GET /api/services/{id}/deployment-triggers", a.applicationServiceDeploymentTriggers)
	protected.HandleFunc("PUT /api/services/{id}/deployment-triggers", a.updateApplicationServiceDeploymentTriggers)
	protected.HandleFunc("GET /api/services/{id}/logs", a.applicationServiceLogs)
	protected.HandleFunc("GET /api/services/{id}/environment", a.applicationServiceEnvironment)
	protected.HandleFunc("PUT /api/services/{id}/environment", a.updateApplicationServiceEnvironment)
	protected.HandleFunc("DELETE /api/services/{id}", a.deleteApplicationService)
	protected.HandleFunc("GET /api/databases/{id}/credentials", a.databaseCredentials)
	protected.HandleFunc("PUT /api/databases/{id}/exposure", a.updateDatabaseExposure)
	protected.HandleFunc("POST /api/databases/{id}/stop", a.stopDatabaseService)
	protected.HandleFunc("POST /api/databases/{id}/restart", a.restartDatabaseService)
	protected.HandleFunc("GET /api/databases/{id}/logs", a.databaseLogs)
	protected.HandleFunc("GET /api/databases/{id}/events", a.databaseDeploymentEvents)
	protected.HandleFunc("DELETE /api/databases/{id}", a.deleteDatabaseService)
	protected.HandleFunc("GET /api/deployments", a.deployments)
	protected.HandleFunc("GET /api/deployments/{id}", a.deployment)
	protected.HandleFunc("POST /api/deployments/{id}/cancel", a.cancelDeployment)
	protected.HandleFunc("GET /api/integrations", a.integrationsIndex)
	protected.HandleFunc("GET /api/integrations/oauth/{provider}/start", a.oauthStart)
	protected.HandleFunc("GET /api/integrations/github/install/start", a.githubInstallationStart)
	protected.HandleFunc("POST /api/integrations/github/installations/sync", a.syncGitHubInstallations)
	protected.HandleFunc("GET /api/integrations/sources/{id}/repositories", a.repositories)
	protected.HandleFunc("DELETE /api/integrations/sources/{id}", a.deleteSourceConnection)
	protected.HandleFunc("POST /api/integrations/registries", a.createRegistry)
	protected.HandleFunc("DELETE /api/integrations/registries/{id}", a.deleteRegistry)
	protected.HandleFunc("GET /api/caddy/config", a.caddyConfig)
	protected.HandleFunc("PUT /api/caddy/config", a.applyCaddyConfig)
	protected.HandleFunc("POST /api/caddy/reset", a.resetCaddyConfig)
	protected.HandleFunc("GET /api/infrastructure/metrics", a.infrastructureMetrics)
	protected.HandleFunc("GET /api/infrastructure/control-plane/metrics", a.controlPlaneMetrics)
	protected.HandleFunc("GET /api/infrastructure/control-plane/logs", a.controlPlaneLogs)
	protected.HandleFunc("GET /api/infrastructure/cleanup", a.dockerCleanupPreview)
	protected.HandleFunc("POST /api/infrastructure/cleanup", a.dockerCleanup)
	protected.HandleFunc("GET /api/infrastructure/cleanup/schedule", a.cleanupSchedule)
	protected.HandleFunc("PUT /api/infrastructure/cleanup/schedule", a.updateCleanupSchedule)
	root := http.NewServeMux()
	root.Handle("/api/auth/me", a.auth.Require(protected))
	root.Handle("/api/account/", a.auth.Require(protected))
	root.Handle("/api/settings/", a.auth.Require(protected))
	root.Handle("/api/dashboard", a.auth.Require(protected))
	root.Handle("/api/projects", a.auth.Require(protected))
	root.Handle("/api/projects/", a.auth.Require(protected))
	root.Handle("/api/deployments", a.auth.Require(protected))
	root.Handle("/api/deployments/", a.auth.Require(protected))
	root.Handle("/api/databases/", a.auth.Require(protected))
	root.Handle("/api/services/", a.auth.Require(protected))
	root.Handle("GET /api/integrations/oauth/{provider}/callback", public)
	root.Handle("GET /api/integrations/github/install/callback", public)
	root.Handle("/api/integrations", a.auth.Require(protected))
	root.Handle("/api/integrations/", a.auth.Require(protected))
	root.Handle("/api/caddy/", a.auth.Require(protected))
	root.Handle("/api/infrastructure/", a.auth.Require(protected))
	root.Handle("/", public)
	return withJSON(root)
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	err := a.store.Ping(r.Context())
	write(w, 200, map[string]any{"ok": err == nil, "database": err == nil, "docker": a.docker.Health(r.Context())})
}

func (a *API) infrastructureMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := a.docker.GlobalMetrics(r.Context())
	if err != nil {
		a.log.Warn("read Docker metrics", "error", err)
		write(w, 502, map[string]string{"error": "Docker metrics are unavailable: " + err.Error()})
		return
	}
	write(w, 200, metrics)
}

func (a *API) controlPlaneMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := a.docker.ControlPlaneMetrics(r.Context())
	if err != nil {
		a.log.Warn("read Dokyr control-plane metrics", "error", err)
		write(w, 502, map[string]string{"error": "Dokyr control-plane metrics are unavailable: " + err.Error()})
		return
	}
	write(w, 200, metrics)
}

func (a *API) controlPlaneLogs(w http.ResponseWriter, r *http.Request) {
	service := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("service")))
	if service != "selfhost" && service != "postgres" && service != "caddy" {
		bad(w, "service must be selfhost, postgres, or caddy")
		return
	}
	tail := 300
	if requested := strings.TrimSpace(r.URL.Query().Get("lines")); requested != "" {
		parsed, err := strconv.Atoi(requested)
		if err != nil || parsed < 1 || parsed > 1000 {
			bad(w, "lines must be a number between 1 and 1000")
			return
		}
		tail = parsed
	}
	lines, container, err := a.docker.ControlPlaneLogs(r.Context(), service, tail)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 404, map[string]string{"error": "control-plane container is not available"})
		return
	}
	if err != nil {
		a.log.Warn("read Dokyr control-plane logs", "service", service, "error", err)
		write(w, 502, map[string]string{"error": "could not read control-plane logs"})
		return
	}
	write(w, 200, map[string]any{"service": service, "container": container, "lines": lines, "count": len(lines), "limit": tail})
}

func (a *API) projectMetrics(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	metrics, err := a.docker.ProjectMetrics(r.Context(), projectID)
	if err != nil {
		a.log.Warn("read project Docker metrics", "project", projectID, "error", err)
		write(w, 502, map[string]string{"error": "Project metrics are unavailable: " + err.Error()})
		return
	}
	write(w, 200, metrics)
}

func (a *API) dockerCleanupPreview(w http.ResponseWriter, r *http.Request) {
	preview, err := a.docker.CleanupPreview(r.Context())
	if err != nil {
		a.log.Warn("inspect Docker cleanup", "error", err)
		write(w, 502, map[string]string{"error": "Docker cleanup preview is unavailable: " + err.Error()})
		return
	}
	write(w, 200, preview)
}

func (a *API) dockerCleanup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Containers   bool   `json:"containers"`
		Images       bool   `json:"images"`
		BuildCache   bool   `json:"buildCache"`
		Networks     bool   `json:"networks"`
		Volumes      bool   `json:"volumes"`
		Confirmation string `json:"confirmation"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Confirmation != "CLEAN DOCKER" {
		bad(w, "type CLEAN DOCKER to confirm cleanup")
		return
	}
	options := runtime.CleanupOptions{Containers: in.Containers, Images: in.Images, BuildCache: in.BuildCache, Networks: in.Networks, Volumes: in.Volumes}
	if !options.Containers && !options.Images && !options.BuildCache && !options.Networks && !options.Volumes {
		bad(w, "select at least one cleanup category")
		return
	}
	a.cleanupMu.Lock()
	defer a.cleanupMu.Unlock()
	result, err := a.docker.Cleanup(r.Context(), options)
	if err != nil {
		a.log.Error("clean Docker resources", "error", err)
		write(w, 502, map[string]string{"error": err.Error()})
		return
	}
	a.log.Info("Docker cleanup completed", "deleted", result.Deleted, "reclaimed", result.SpaceReclaimed, "volumes", options.Volumes)
	write(w, 200, result)
}
func (a *API) setupStatus(w http.ResponseWriter, r *http.Request) {
	configured, err := a.store.IsConfigured(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]bool{"configured": configured})
}
func (a *API) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Name == "" {
		bad(w, "name is required")
		return
	}
	if _, err := mail.ParseAddress(in.Email); err != nil || !strings.Contains(in.Email, "@") {
		bad(w, "enter a valid email address")
		return
	}
	if len(in.Password) < 10 {
		bad(w, "password must contain at least 10 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		problem(w, err)
		return
	}
	u := store.User{ID: newID("usr"), Name: in.Name, Email: in.Email, PasswordHash: string(hash), Role: "owner"}
	if err := a.store.CreateInitialUser(r.Context(), u); errors.Is(err, store.ErrAlreadyConfigured) {
		write(w, 409, map[string]string{"error": "selfhost is already configured"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	if !a.issueSession(w, u) {
		return
	}
	write(w, 201, map[string]any{"user": u})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := a.store.UserByEmail(r.Context(), strings.TrimSpace(in.Email))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		write(w, 401, map[string]string{"error": "invalid email or password"})
		return
	}
	if u.TwoFactorEnabled {
		if !a.issueTwoFactorChallenge(w, u.ID) {
			return
		}
		write(w, 200, map[string]any{"requiresTwoFactor": true})
		return
	}
	if !a.issueSession(w, u) {
		return
	}
	write(w, 200, map[string]any{"user": u})
}
func (a *API) authProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := a.integrations.AccountProviderStatus(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, providers)
}
func (a *API) completeTwoFactorLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, err := a.auth.ParseChallenge(r)
	if err != nil {
		write(w, 401, map[string]string{"error": "Your two-factor challenge expired. Sign in again."})
		return
	}
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil || !u.TwoFactorEnabled || !a.verifyUserTOTP(u, in.Code) {
		write(w, 401, map[string]string{"error": "The authentication code is invalid."})
		return
	}
	if !a.issueSession(w, u) {
		return
	}
	a.auth.ClearChallengeCookie(w)
	write(w, 200, map[string]any{"user": u})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.auth.ClearCookie(w)
	a.auth.ClearChallengeCookie(w)
	write(w, 200, map[string]bool{"ok": true})
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		write(w, 401, map[string]string{"error": "authentication required"})
		return
	}
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		write(w, 401, map[string]string{"error": "user no longer exists"})
		return
	}
	write(w, 200, map[string]any{"user": u})
}
func (a *API) issueSession(w http.ResponseWriter, u store.User) bool {
	token, err := a.auth.Token(u.ID, u.Name, u.Email, u.Role)
	if err != nil {
		problem(w, err)
		return false
	}
	a.auth.SetCookie(w, token)
	return true
}
func (a *API) issueTwoFactorChallenge(w http.ResponseWriter, userID string) bool {
	token, err := a.auth.ChallengeToken(userID)
	if err != nil {
		problem(w, err)
		return false
	}
	a.auth.ClearCookie(w)
	a.auth.SetChallengeCookie(w, token)
	return true
}

func (a *API) accountSecurity(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	providers, err := a.integrations.AccountProviderStatus(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{
		"twoFactorEnabled": u.TwoFactorEnabled,
		"github": map[string]any{
			"linked": u.GitHubAccountID != "",
			"login":  u.GitHubLogin,
		},
		"providers": providers,
	})
}

func (a *API) updatePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
		Code            string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.CurrentPassword)) != nil {
		write(w, 401, map[string]string{"error": "Current password is incorrect."})
		return
	}
	if len(in.NewPassword) < 12 {
		bad(w, "New password must contain at least 12 characters.")
		return
	}
	if len(in.NewPassword) > 128 {
		bad(w, "New password must contain no more than 128 characters.")
		return
	}
	if in.CurrentPassword == in.NewPassword {
		bad(w, "Choose a password different from your current password.")
		return
	}
	if u.TwoFactorEnabled && !a.verifyUserTOTP(u, in.Code) {
		write(w, 401, map[string]string{"error": "Enter a valid authentication code."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		problem(w, err)
		return
	}
	if err := a.store.UpdatePassword(r.Context(), u.ID, string(hash)); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"updated": true, "message": "Password updated."})
}

type smtpSettingsInput struct {
	Enabled                   bool   `json:"enabled"`
	Host                      string `json:"host"`
	Port                      int    `json:"port"`
	Encryption                string `json:"encryption"`
	Username                  string `json:"username"`
	Password                  string `json:"password"`
	FromName                  string `json:"fromName"`
	FromEmail                 string `json:"fromEmail"`
	NotifyDeploymentFailures  bool   `json:"notifyDeploymentFailures"`
	NotifyDeploymentSuccesses bool   `json:"notifyDeploymentSuccesses"`
}

func defaultSMTPSettings() store.SMTPSettings {
	return store.SMTPSettings{Port: 587, Encryption: "starttls", FromName: "Dokyr", NotifyDeploymentFailures: true}
}

func smtpConfigured(settings store.SMTPSettings) bool {
	return strings.TrimSpace(settings.Host) != "" && settings.Port > 0 && strings.TrimSpace(settings.FromEmail) != ""
}

func smtpSettingsResponse(settings store.SMTPSettings) map[string]any {
	return map[string]any{
		"enabled": settings.Enabled, "configured": smtpConfigured(settings), "host": settings.Host, "port": settings.Port,
		"encryption": settings.Encryption, "username": settings.Username, "hasPassword": settings.PasswordEncrypted != "",
		"fromName": settings.FromName, "fromEmail": settings.FromEmail,
		"notifyDeploymentFailures": settings.NotifyDeploymentFailures, "notifyDeploymentSuccesses": settings.NotifyDeploymentSuccesses,
		"updatedAt": settings.UpdatedAt,
	}
}

func cleanSMTPSettings(in smtpSettingsInput) (smtpSettingsInput, error) {
	in.Host = strings.TrimSpace(in.Host)
	in.Encryption = strings.ToLower(strings.TrimSpace(in.Encryption))
	in.Username = strings.TrimSpace(in.Username)
	in.FromName = strings.TrimSpace(in.FromName)
	in.FromEmail = strings.ToLower(strings.TrimSpace(in.FromEmail))
	if in.Host == "" || len(in.Host) > 255 || strings.ContainsAny(in.Host, " /\t\r\n:") {
		return in, errors.New("enter an SMTP hostname without a scheme or port")
	}
	if in.Port < 1 || in.Port > 65535 {
		return in, errors.New("SMTP port must be between 1 and 65535")
	}
	if in.Encryption != "starttls" && in.Encryption != "tls" && in.Encryption != "none" {
		return in, errors.New("choose STARTTLS, TLS, or no encryption")
	}
	if in.FromName == "" || len(in.FromName) > 100 || strings.ContainsAny(in.FromName, "\r\n") {
		return in, errors.New("sender name is required and must be at most 100 characters")
	}
	if _, err := mail.ParseAddress(in.FromEmail); err != nil || !strings.Contains(in.FromEmail, "@") {
		return in, errors.New("enter a valid sender email address")
	}
	if len(in.Username) > 500 || len(in.Password) > 4096 {
		return in, errors.New("SMTP credentials are too long")
	}
	return in, nil
}

// BootstrapSMTPSettings copies an optional Docker environment configuration
// into PostgreSQL once. PostgreSQL remains the source of truth after the first
// insert: ON CONFLICT DO NOTHING guarantees later container restarts cannot
// overwrite values saved from the settings interface.
func (a *API) BootstrapSMTPSettings(ctx context.Context, bootstrap config.SMTPBootstrap) (bool, error) {
	if !bootstrap.Present {
		return false, nil
	}
	if _, err := a.store.SMTPSettings(ctx); err == nil {
		return false, nil
	} else if !store.NotFound(err) {
		return false, fmt.Errorf("check existing SMTP settings: %w", err)
	}
	clean, err := cleanSMTPSettings(smtpSettingsInput{
		Enabled: bootstrap.Enabled, Host: bootstrap.Host, Port: bootstrap.Port, Encryption: bootstrap.Encryption,
		Username: bootstrap.Username, Password: bootstrap.Password, FromName: bootstrap.FromName, FromEmail: bootstrap.FromEmail,
		NotifyDeploymentFailures: bootstrap.NotifyDeploymentFailures, NotifyDeploymentSuccesses: bootstrap.NotifyDeploymentSuccesses,
	})
	if err != nil {
		return false, fmt.Errorf("validate SMTP bootstrap settings: %w", err)
	}
	passwordEncrypted := ""
	if clean.Password != "" {
		passwordEncrypted, err = a.box.Encrypt(clean.Password)
		if err != nil {
			return false, fmt.Errorf("encrypt SMTP bootstrap password: %w", err)
		}
	}
	return a.store.CreateSMTPSettingsIfMissing(ctx, store.SMTPSettings{
		Enabled: clean.Enabled, Host: clean.Host, Port: clean.Port, Encryption: clean.Encryption, Username: clean.Username,
		PasswordEncrypted: passwordEncrypted, FromName: clean.FromName, FromEmail: clean.FromEmail,
		NotifyDeploymentFailures: clean.NotifyDeploymentFailures, NotifyDeploymentSuccesses: clean.NotifyDeploymentSuccesses,
	})
}

func (a *API) smtpSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.SMTPSettings(r.Context())
	if store.NotFound(err) {
		write(w, 200, smtpSettingsResponse(defaultSMTPSettings()))
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, smtpSettingsResponse(settings))
}

func (a *API) updateSMTPSettings(w http.ResponseWriter, r *http.Request) {
	var in smtpSettingsInput
	if !decode(w, r, &in) {
		return
	}
	clean, err := cleanSMTPSettings(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	existing, err := a.store.SMTPSettings(r.Context())
	if err != nil && !store.NotFound(err) {
		problem(w, err)
		return
	}
	passwordEncrypted := existing.PasswordEncrypted
	if clean.Password != "" {
		passwordEncrypted, err = a.box.Encrypt(clean.Password)
		if err != nil {
			problem(w, err)
			return
		}
	}
	claims, _ := auth.FromContext(r.Context())
	settings := store.SMTPSettings{
		Enabled: clean.Enabled, Host: clean.Host, Port: clean.Port, Encryption: clean.Encryption, Username: clean.Username,
		PasswordEncrypted: passwordEncrypted, FromName: clean.FromName, FromEmail: clean.FromEmail,
		NotifyDeploymentFailures: clean.NotifyDeploymentFailures, NotifyDeploymentSuccesses: clean.NotifyDeploymentSuccesses,
		CreatedBy: claims.Subject,
	}
	if err := a.store.UpsertSMTPSettings(r.Context(), settings); err != nil {
		problem(w, err)
		return
	}
	updated, err := a.store.SMTPSettings(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"settings": smtpSettingsResponse(updated), "message": "SMTP settings saved."})
}

func (a *API) smtpMailerConfig(ctx context.Context) (store.SMTPSettings, mailer.Config, error) {
	settings, err := a.store.SMTPSettings(ctx)
	if err != nil {
		return settings, mailer.Config{}, err
	}
	password := ""
	if settings.PasswordEncrypted != "" {
		password, err = a.box.Decrypt(settings.PasswordEncrypted)
		if err != nil {
			return settings, mailer.Config{}, err
		}
	}
	config := mailer.Config{Host: settings.Host, Port: settings.Port, Encryption: settings.Encryption, Username: settings.Username, Password: password, FromName: settings.FromName, FromEmail: settings.FromEmail}
	return settings, config, nil
}

func (a *API) testSMTPSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Recipient string `json:"recipient"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	recipient := strings.ToLower(strings.TrimSpace(in.Recipient))
	if recipient == "" {
		u, err := a.store.User(r.Context(), claims.Subject)
		if err != nil {
			problem(w, err)
			return
		}
		recipient = u.Email
	}
	if _, err := mail.ParseAddress(recipient); err != nil {
		bad(w, "enter a valid test recipient")
		return
	}
	settings, config, err := a.smtpMailerConfig(r.Context())
	if err != nil || !smtpConfigured(settings) {
		write(w, 409, map[string]string{"error": "Save a complete SMTP configuration before sending a test."})
		return
	}
	message := mailer.Message{
		To: recipient, Subject: "Dokyr SMTP test",
		Text: "Your Dokyr SMTP configuration is working. This server can now send account recovery and deployment notification emails.",
		HTML: `<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto;padding:32px"><p style="color:#087a51;font-weight:700">DEPLOYFORGE</p><h1 style="font-size:24px">SMTP is connected</h1><p>Your server can now send account recovery links and deployment notifications.</p><p style="color:#667085;font-size:13px">You can return to Settings → SMTP to choose which deployment events generate email.</p></div>`,
	}
	if err := mailer.Send(r.Context(), config, message); err != nil {
		a.log.Warn("send SMTP test", "recipient", recipient, "error", err)
		write(w, 502, map[string]string{"error": err.Error()})
		return
	}
	write(w, 200, map[string]any{"sent": true, "message": "Test email sent to " + recipient + "."})
}

func (a *API) passwordResetStatus(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.SMTPSettings(r.Context())
	write(w, 200, map[string]bool{"enabled": err == nil && settings.Enabled && smtpConfigured(settings)})
}

func (a *API) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	settings, config, err := a.smtpMailerConfig(r.Context())
	if err != nil || !settings.Enabled || !smtpConfigured(settings) {
		write(w, 503, map[string]string{"error": "Password recovery email is not configured on this server."})
		return
	}
	generic := map[string]any{"accepted": true, "message": "If an account exists for that email, a password reset link has been sent."}
	u, err := a.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		write(w, http.StatusAccepted, generic)
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		problem(w, err)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	if err := a.store.CreatePasswordResetToken(r.Context(), hex.EncodeToString(hash[:]), u.ID, time.Now().Add(30*time.Minute)); err != nil {
		problem(w, err)
		return
	}
	resetURL := a.publicURL + "/reset-password?token=" + url.QueryEscape(token)
	name := html.EscapeString(u.Name)
	link := html.EscapeString(resetURL)
	message := mailer.Message{
		To: u.Email, Subject: "Reset your Dokyr password",
		Text: "Reset your Dokyr password using this link (valid for 30 minutes):\n\n" + resetURL + "\n\nIf you did not request this, you can ignore this email.",
		HTML: `<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto;padding:32px"><p style="color:#087a51;font-weight:700">DEPLOYFORGE</p><h1 style="font-size:24px">Reset your password</h1><p>Hello ` + name + `,</p><p>Use the button below to choose a new password. This one-time link expires in 30 minutes.</p><p style="margin:28px 0"><a href="` + link + `" style="background:#087a51;color:white;text-decoration:none;padding:12px 18px;border-radius:7px;font-weight:700">Reset password</a></p><p style="color:#667085;font-size:13px">If you did not request this, you can ignore this email.</p></div>`,
	}
	if err := mailer.Send(r.Context(), config, message); err != nil {
		a.log.Warn("send password reset email", "user", u.ID, "error", err)
	}
	write(w, http.StatusAccepted, generic)
}

func (a *API) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.NewPassword) < 12 || len(in.NewPassword) > 128 {
		bad(w, "New password must contain between 12 and 128 characters.")
		return
	}
	if len(strings.TrimSpace(in.Token)) < 32 {
		bad(w, "This password reset link is invalid or expired.")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		problem(w, err)
		return
	}
	tokenHash := sha256.Sum256([]byte(strings.TrimSpace(in.Token)))
	if err := a.store.ConsumePasswordResetToken(r.Context(), hex.EncodeToString(tokenHash[:]), string(passwordHash)); store.NotFound(err) {
		bad(w, "This password reset link is invalid or expired.")
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	a.auth.ClearCookie(w)
	a.auth.ClearChallengeCookie(w)
	write(w, 200, map[string]any{"updated": true, "message": "Password updated. You can now sign in."})
}

func (a *API) setupTwoFactor(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if u.TwoFactorEnabled {
		write(w, 409, map[string]string{"error": "Two-factor authentication is already enabled."})
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		problem(w, err)
		return
	}
	sealed, err := a.box.Encrypt(secret)
	if err != nil {
		problem(w, err)
		return
	}
	if err := a.store.SetTwoFactor(r.Context(), u.ID, sealed, false); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{
		"secret": secret,
		"uri":    auth.TOTPURI(secret, "Dokyr", u.Email),
	})
}

func (a *API) confirmTwoFactor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if u.TwoFactorSecretEncrypted == "" {
		bad(w, "Start two-factor setup before confirming a code.")
		return
	}
	if !a.verifyUserTOTP(u, in.Code) {
		write(w, 422, map[string]string{"error": "The authentication code is invalid."})
		return
	}
	if err := a.store.SetTwoFactor(r.Context(), u.ID, u.TwoFactorSecretEncrypted, true); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"enabled": true, "message": "Two-factor authentication enabled."})
}

func (a *API) disableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if !u.TwoFactorEnabled {
		bad(w, "Two-factor authentication is not enabled.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil || !a.verifyUserTOTP(u, in.Code) {
		write(w, 401, map[string]string{"error": "Password or authentication code is incorrect."})
		return
	}
	if err := a.store.SetTwoFactor(r.Context(), u.ID, "", false); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"enabled": false, "message": "Two-factor authentication disabled."})
}

func (a *API) verifyUserTOTP(u store.User, code string) bool {
	if u.TwoFactorSecretEncrypted == "" {
		return false
	}
	secret, err := a.box.Decrypt(u.TwoFactorSecretEncrypted)
	return err == nil && auth.VerifyTOTP(secret, code, time.Now())
}

func (a *API) githubLoginStart(w http.ResponseWriter, r *http.Request) {
	destination, err := a.integrations.StartGitHubAccountOAuth(r.Context(), "", "login")
	if err != nil {
		if errors.Is(err, integration.ErrGitHubAccountNotConfigured) {
			err = errors.New("The GitHub App was removed or is not configured. Sign in with your password, then reconnect GitHub in Settings → Security.")
		}
		http.Redirect(w, r, "/login?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (a *API) githubLinkStart(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	destination, err := a.integrations.StartGitHubAccountOAuth(r.Context(), claims.Subject, "link")
	if errors.Is(err, integration.ErrGitHubAccountNotConfigured) {
		manifest, manifestErr := a.integrations.StartGitHubManifest(r.Context(), claims.Subject)
		if manifestErr != nil {
			problem(w, manifestErr)
			return
		}
		a.renderGitHubManifestForm(w, manifest)
		return
	}
	if err != nil {
		http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (a *API) githubManifestCallback(w http.ResponseWriter, r *http.Request) {
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape("GitHub App setup was cancelled."), http.StatusFound)
		return
	}
	state, err := a.integrations.CompleteGitHubManifest(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		a.log.Warn("GitHub App manifest callback failed", "error", err)
		http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	destination, err := a.integrations.StartGitHubAccountOAuth(r.Context(), state.UserID, "link")
	if err != nil {
		a.log.Warn("start GitHub authorization after app setup", "error", err)
		http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (a *API) renderGitHubManifestForm(w http.ResponseWriter, manifest integration.GitHubManifestStart) {
	const page = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connecting GitHub — Dokyr</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0c1117;color:#e7edf3;font:14px system-ui,sans-serif}.card{width:min(420px,calc(100% - 40px));padding:32px;border:1px solid #223040;border-radius:12px;background:#121922;text-align:center}.mark{width:42px;height:42px;margin:auto;display:grid;place-items:center;border-radius:10px;background:#0b63e5;color:#ffffff;font-weight:800}h1{font-size:22px;margin:18px 0 8px}p{color:#8494a5;line-height:1.6}button{height:42px;margin-top:14px;padding:0 18px;border:0;border-radius:7px;background:#0b63e5;color:#ffffff;font-weight:750;cursor:pointer}</style></head><body><main class="card"><div class="mark">GH</div><h1>Continue to GitHub</h1><p>Dokyr is redirecting you to create and authorize a private GitHub App for this server.</p><form id="github-manifest" method="post" action="{{.Action}}"><input type="hidden" name="manifest" value="{{.Manifest}}"><button type="submit">Continue to GitHub</button></form></main><script>document.getElementById('github-manifest').submit()</script></body></html>`
	tmpl, err := template.New("github-manifest").Parse(page)
	if err != nil {
		problem(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := tmpl.Execute(w, manifest); err != nil {
		a.log.Warn("render GitHub manifest redirect", "error", err)
	}
}

func (a *API) githubAccountCallback(w http.ResponseWriter, r *http.Request) {
	errorDestination := "/login"
	if _, err := a.auth.Parse(r); err == nil {
		errorDestination = "/settings?section=security"
	}
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		http.Redirect(w, r, errorDestination+querySeparator(errorDestination)+"error="+url.QueryEscape("GitHub authorization was cancelled."), http.StatusFound)
		return
	}
	state, identity, err := a.integrations.CompleteGitHubAccountOAuth(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		a.log.Warn("GitHub account OAuth callback failed", "error", err)
		http.Redirect(w, r, errorDestination+querySeparator(errorDestination)+"error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if state.Mode == "link" {
		if err := a.store.LinkGitHubAccount(r.Context(), state.UserID, identity.AccountID, identity.Login); err != nil {
			a.log.Warn("link GitHub account", "error", err)
			http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape("This GitHub account is already linked to another user."), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/settings?section=security&github=linked", http.StatusFound)
		return
	}
	u, err := a.store.UserByGitHubAccount(r.Context(), identity.AccountID)
	if store.NotFound(err) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("No Dokyr account is linked to @"+identity.Login+". Sign in with your password and link it in Settings."), http.StatusFound)
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if u.TwoFactorEnabled {
		if !a.issueTwoFactorChallenge(w, u.ID) {
			return
		}
		http.Redirect(w, r, "/login?twoFactor=github", http.StatusFound)
		return
	}
	if !a.issueSession(w, u) {
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *API) unlinkGitHub(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	u, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		write(w, 401, map[string]string{"error": "Current password is incorrect."})
		return
	}
	if u.TwoFactorEnabled && !a.verifyUserTOTP(u, in.Code) {
		write(w, 401, map[string]string{"error": "Enter a valid authentication code."})
		return
	}
	if err := a.store.UnlinkGitHubAccount(r.Context(), u.ID); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"linked": false, "message": "GitHub account unlinked."})
}

func querySeparator(destination string) string {
	if strings.Contains(destination, "?") {
		return "&"
	}
	return "?"
}
func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	projects, err := a.store.Projects(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	deployments, err := a.store.Deployments(r.Context(), "")
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"projects": projects, "deployments": deployments, "docker": a.docker.Health(r.Context())})
}
func (a *API) projects(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Projects(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, items)
}
func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          string `json:"name"`
		Repository    string `json:"repository"`
		Branch        string `json:"branch"`
		Domain        string `json:"domain"`
		SourceType    string `json:"sourceType"`
		ConnectionID  string `json:"connectionId"`
		RegistryID    string `json:"registryId"`
		ImageURL      string `json:"imageUrl"`
		ContainerPort int    `json:"containerPort"`
		HTTPSEnabled  bool   `json:"httpsEnabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Repository = strings.TrimSpace(in.Repository)
	in.Branch = strings.TrimSpace(in.Branch)
	in.SourceType = strings.TrimSpace(in.SourceType)
	domain, err := caddy.NormalizeDomain(in.Domain)
	if err != nil {
		bad(w, err.Error())
		return
	}
	if in.SourceType == "" {
		in.SourceType = "empty"
	}
	if in.ContainerPort == 0 {
		in.ContainerPort = 80
	}
	if in.ContainerPort < 1 || in.ContainerPort > 65535 {
		bad(w, "container port must be between 1 and 65535")
		return
	}
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	if in.Name == "" {
		bad(w, "name is required")
		return
	}
	if in.SourceType != "repository" && in.SourceType != "image" && in.SourceType != "empty" {
		bad(w, "source type must be empty, repository, or image")
		return
	}
	if in.SourceType == "repository" && in.Repository == "" {
		bad(w, "repository is required")
		return
	}
	if in.SourceType == "image" && in.ImageURL == "" {
		bad(w, "image URL is required")
		return
	}
	if in.SourceType == "empty" {
		in.Repository = ""
		in.ConnectionID = ""
		in.RegistryID = ""
		in.ImageURL = ""
		if domain != "" {
			bad(w, "add a service before assigning a domain")
			return
		}
	}
	claims, _ := auth.FromContext(r.Context())
	if in.ConnectionID != "" {
		if _, err := a.store.SourceConnection(r.Context(), in.ConnectionID, claims.Subject); err != nil {
			bad(w, "source connection is invalid")
			return
		}
	}
	if in.RegistryID != "" {
		if _, err := a.store.Registry(r.Context(), in.RegistryID, claims.Subject); err != nil {
			bad(w, "registry is invalid")
			return
		}
	}
	if in.Branch == "" {
		in.Branch = "main"
	}
	if domain != "" {
		if _, err := a.store.ProjectByDomain(r.Context(), domain); err == nil {
			write(w, 409, map[string]string{"error": "this domain is already assigned to another project"})
			return
		} else if !store.NotFound(err) {
			problem(w, err)
			return
		}
	}
	status := "healthy"
	if in.SourceType == "empty" {
		status = "created"
	}
	p := store.Project{ID: newID("prj"), Name: in.Name, Repository: in.Repository, Branch: in.Branch, Domain: domain, Status: status, SourceType: in.SourceType, ConnectionID: in.ConnectionID, RegistryID: in.RegistryID, ImageURL: in.ImageURL, ContainerPort: in.ContainerPort, HTTPSEnabled: in.HTTPSEnabled}
	if err := a.store.CreateProject(r.Context(), p); err != nil {
		problem(w, err)
		return
	}
	if domain != "" {
		binding := store.ProjectDomainBinding{
			ID: newID("dom"), ProjectID: p.ID, Domain: domain, HTTPSEnabled: in.HTTPSEnabled,
			Rules: []store.ProjectDomainBindingRule{{Path: "/*", Port: in.ContainerPort}},
		}
		if err := a.store.ReplaceProjectDomainBindings(r.Context(), p.ID, []store.ProjectDomainBinding{binding}); err != nil {
			_ = a.store.DeleteProject(r.Context(), p.ID)
			problem(w, err)
			return
		}
	}
	created, err := a.store.Project(r.Context(), p.ID)
	if err != nil {
		problem(w, err)
		return
	}
	if domain != "" {
		if err := a.SyncDomains(r.Context()); err != nil {
			_ = a.store.ReplaceProjectDomainBindings(r.Context(), p.ID, nil)
			_ = a.store.UpdateProjectDomain(r.Context(), p.ID, "", false)
			a.log.Warn("configure project domain", "project", p.ID, "domain", domain, "error", err)
			write(w, 502, map[string]string{"error": "project was created, but Caddy could not activate its domain"})
			return
		}
	}
	write(w, 201, created)
}

func (a *API) updateProjectDomain(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domain       string                     `json:"domain"`
		HTTPSEnabled bool                       `json:"httpsEnabled"`
		DefaultPort  int                        `json:"defaultPort"`
		DefaultPath  string                     `json:"defaultPath"`
		Rules        []store.ProjectIngressRule `json:"rules"`
		Domains      *[]domainBindingInput      `json:"domains"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.domainMu.Lock()
	defer a.domainMu.Unlock()
	project, err := a.store.Project(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	inputs := []domainBindingInput{}
	if in.Domains != nil {
		inputs = *in.Domains
	} else if strings.TrimSpace(in.Domain) != "" {
		defaultPort := in.DefaultPort
		if defaultPort == 0 {
			defaultPort = project.ContainerPort
		}
		defaultPath := in.DefaultPath
		if strings.TrimSpace(defaultPath) == "" {
			defaultPath = "/*"
		}
		rules := []domainRuleInput{{Path: defaultPath, Port: defaultPort}}
		for _, rule := range in.Rules {
			rules = append(rules, domainRuleInput{Path: rule.Path, Port: rule.Port})
		}
		inputs = []domainBindingInput{{Domain: in.Domain, HTTPSEnabled: in.HTTPSEnabled, Rules: rules}}
	}
	if len(inputs) > 25 {
		bad(w, "a project can have at most 25 domains")
		return
	}
	cleanBindings := make([]store.ProjectDomainBinding, 0, len(inputs))
	applicationServices, err := a.store.ApplicationServices(r.Context(), project.ID)
	if err != nil {
		problem(w, err)
		return
	}
	servicePorts := map[string]int{}
	for _, service := range applicationServices {
		servicePorts[service.ID] = service.ContainerPort
	}
	seenDomains := map[string]bool{}
	for _, input := range inputs {
		domain, err := caddy.NormalizeDomain(input.Domain)
		if err != nil || domain == "" {
			if err != nil {
				bad(w, err.Error())
			} else {
				bad(w, "domain name is required")
			}
			return
		}
		if seenDomains[domain] {
			bad(w, "each domain can only be added once")
			return
		}
		seenDomains[domain] = true
		assigned, lookupErr := a.store.ProjectDomainBindingByDomain(r.Context(), domain)
		if lookupErr == nil && assigned.ProjectID != project.ID {
			write(w, 409, map[string]string{"error": "this domain is already assigned to another project"})
			return
		}
		if lookupErr != nil && !store.NotFound(lookupErr) {
			problem(w, lookupErr)
			return
		}
		if len(input.Rules) == 0 {
			bad(w, "each domain needs at least one path rule")
			return
		}
		if len(input.Rules) > 50 {
			bad(w, "a domain can have at most 50 path rules")
			return
		}
		binding := store.ProjectDomainBinding{ID: newID("dom"), ProjectID: project.ID, Domain: domain, HTTPSEnabled: input.HTTPSEnabled, Rules: []store.ProjectDomainBindingRule{}}
		seenPaths := map[string]bool{}
		for _, inputRule := range input.Rules {
			serviceID := strings.TrimSpace(inputRule.ServiceID)
			if serviceID == "main" {
				serviceID = ""
			}
			if serviceID == "" && project.SourceType == "empty" {
				bad(w, "choose an application service for every route")
				return
			}
			if serviceID != "" {
				if _, exists := servicePorts[serviceID]; !exists {
					bad(w, "route target must be an application service in this project")
					return
				}
			}
			path := normalizeIngressPath(inputRule.Path)
			if !validIngressPath(path, true) {
				bad(w, "route paths must look like /*, /api/*, or /health")
				return
			}
			if inputRule.Port < 1 || inputRule.Port > 65535 {
				bad(w, "route ports must be between 1 and 65535")
				return
			}
			if seenPaths[path] {
				bad(w, "each path must be unique within its domain")
				return
			}
			seenPaths[path] = true
			binding.Rules = append(binding.Rules, store.ProjectDomainBindingRule{Path: path, Port: inputRule.Port, ServiceID: serviceID})
		}
		cleanBindings = append(cleanBindings, binding)
	}
	previousBindings, err := a.store.ProjectDomainBindings(r.Context(), project.ID)
	if err != nil {
		problem(w, err)
		return
	}
	primaryDomain := ""
	primaryHTTPS := false
	primaryPort := project.ContainerPort
	if len(cleanBindings) > 0 {
		primaryDomain = cleanBindings[0].Domain
		primaryHTTPS = cleanBindings[0].HTTPSEnabled
		if cleanBindings[0].Rules[0].ServiceID == "" {
			primaryPort = cleanBindings[0].Rules[0].Port
		}
		for _, rule := range cleanBindings[0].Rules {
			if rule.Path == "/*" && rule.ServiceID == "" {
				primaryPort = rule.Port
				break
			}
		}
	}
	if err := a.store.ReplaceProjectDomainBindings(r.Context(), project.ID, cleanBindings); err != nil {
		problem(w, err)
		return
	}
	if err := a.store.UpdateProjectIngress(r.Context(), project.ID, primaryDomain, primaryHTTPS, primaryPort); err != nil {
		_ = a.store.ReplaceProjectDomainBindings(r.Context(), project.ID, previousBindings)
		problem(w, err)
		return
	}
	if err := a.syncDomains(r.Context()); err != nil {
		_ = a.store.UpdateProjectIngress(r.Context(), project.ID, project.Domain, project.HTTPSEnabled, project.ContainerPort)
		_ = a.store.ReplaceProjectDomainBindings(r.Context(), project.ID, previousBindings)
		_ = a.syncDomains(r.Context())
		a.log.Warn("configure project domains", "project", project.ID, "error", err)
		write(w, 502, map[string]string{"error": "Caddy could not activate this domain; the previous route was restored"})
		return
	}
	project.Domain = primaryDomain
	project.HTTPSEnabled = primaryHTTPS
	project.ContainerPort = primaryPort
	write(w, 200, map[string]any{"project": project, "active": len(cleanBindings) > 0, "domainBindings": cleanBindings})
}

func normalizeIngressPath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasSuffix(path, "/**") {
		return strings.TrimSuffix(path, "**") + "*"
	}
	return path
}

func validIngressPath(path string, allowRoot bool) bool {
	if path == "" || (!allowRoot && path == "/") || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\r\n?#") {
		return false
	}
	return !strings.Contains(path, "*") || strings.HasSuffix(path, "*") && strings.Count(path, "*") == 1
}

func (a *API) SyncDomains(ctx context.Context) error {
	a.domainMu.Lock()
	defer a.domainMu.Unlock()
	return a.syncDomains(ctx)
}

func (a *API) syncDomains(ctx context.Context) error {
	routes, err := a.managedRoutes(ctx)
	if err != nil {
		return err
	}
	return a.caddy.Apply(ctx, routes)
}

func (a *API) managedRoutes(ctx context.Context) ([]caddy.Route, error) {
	projects, err := a.store.Projects(ctx)
	if err != nil {
		return nil, err
	}
	routes := make([]caddy.Route, 0, len(projects))
	for _, project := range projects {
		applicationServices, serviceErr := a.store.ApplicationServices(ctx, project.ID)
		if serviceErr != nil {
			return nil, serviceErr
		}
		serviceContainers := map[string]string{}
		for _, service := range applicationServices {
			serviceContainers[service.ID] = "selfhost-svc-" + service.ID
		}
		bindings, bindingErr := a.store.ProjectDomainBindings(ctx, project.ID)
		if bindingErr != nil {
			return nil, bindingErr
		}
		for _, binding := range bindings {
			paths := make([]caddy.PathRoute, 0, len(binding.Rules))
			for _, rule := range binding.Rules {
				container := "selfhost-" + project.ID
				if rule.ServiceID != "" {
					var exists bool
					container, exists = serviceContainers[rule.ServiceID]
					if !exists {
						return nil, fmt.Errorf("domain %s references an application service that no longer exists", binding.Domain)
					}
				}
				paths = append(paths, caddy.PathRoute{Path: rule.Path, Upstream: fmt.Sprintf("%s:%d", container, rule.Port)})
			}
			routes = append(routes, caddy.Route{Domain: binding.Domain, HTTPS: binding.HTTPSEnabled, Paths: paths, RejectUnmatched: true})
		}
	}
	return routes, nil
}

func (a *API) caddyConfig(w http.ResponseWriter, r *http.Request) {
	routes, err := a.managedRoutes(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	connectionError := ""
	if err := a.caddy.Ping(r.Context()); err != nil {
		connectionError = err.Error()
	}
	write(w, 200, map[string]any{"connected": connectionError == "", "connectionError": connectionError, "mode": "managed", "routes": routes, "configuration": a.caddy.Render(routes)})
}

func (a *API) applyCaddyConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Configuration string `json:"configuration"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Configuration) > 256*1024 {
		bad(w, "Caddy configuration must be smaller than 256 KB")
		return
	}
	a.domainMu.Lock()
	defer a.domainMu.Unlock()
	if err := a.caddy.ApplyRaw(r.Context(), in.Configuration); err != nil {
		write(w, 422, map[string]string{"error": err.Error()})
		return
	}
	write(w, 200, map[string]any{"applied": true, "configuration": in.Configuration, "message": "Caddy accepted the runtime configuration"})
}

func (a *API) resetCaddyConfig(w http.ResponseWriter, r *http.Request) {
	if err := a.SyncDomains(r.Context()); err != nil {
		problem(w, err)
		return
	}
	routes, _ := a.managedRoutes(r.Context())
	write(w, 200, map[string]any{"applied": true, "routes": routes, "configuration": a.caddy.Render(routes), "message": "Managed routes restored"})
}

func (a *API) integrationsIndex(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	connections, err := a.store.SourceConnections(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	registries, err := a.store.Registries(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	providers, err := a.integrations.ProviderStatus(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	user, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if github, ok := providers["github"].(map[string]any); ok {
		github["linked"] = user.GitHubAccountID != ""
		github["login"] = user.GitHubLogin
	}
	write(w, 200, map[string]any{"providers": providers, "connections": connections, "registries": registries})
}

func (a *API) githubInstallationStart(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	user, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if user.GitHubAccountID == "" {
		http.Redirect(w, r, "/settings?section=security&error="+url.QueryEscape("Link your GitHub account before selecting repositories."), http.StatusFound)
		return
	}
	destination, err := a.integrations.StartGitHubInstallation(r.Context(), claims.Subject)
	if errors.Is(err, integration.ErrGitHubAccountNotConfigured) {
		manifest, manifestErr := a.integrations.StartGitHubManifest(r.Context(), claims.Subject)
		if manifestErr != nil {
			problem(w, manifestErr)
			return
		}
		a.renderGitHubManifestForm(w, manifest)
		return
	}
	if err != nil {
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (a *API) syncGitHubInstallations(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	user, err := a.store.User(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	if user.GitHubAccountID == "" {
		bad(w, "link your GitHub account in Settings before synchronizing repository access")
		return
	}
	connections, warning, err := a.integrations.SyncGitHubInstallations(r.Context(), user.ID, user.GitHubAccountID)
	if err != nil {
		a.log.Warn("synchronize GitHub App installations", "user", user.ID, "error", err)
		write(w, 502, map[string]string{"error": err.Error()})
		return
	}
	message := "No GitHub App installation was found for @" + user.GitHubLogin + ". Select repositories to install the app for this account."
	if len(connections) > 0 {
		message = "GitHub repository access synchronized."
	}
	write(w, 200, map[string]any{"synced": len(connections), "connections": connections, "warning": warning, "message": message})
}

func (a *API) githubInstallationCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("setup_action") == "delete" {
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape("The GitHub App installation was removed."), http.StatusFound)
		return
	}
	installationID, err := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	if err != nil || installationID <= 0 {
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape("GitHub did not return a valid installation."), http.StatusFound)
		return
	}
	if _, err := a.integrations.CompleteGitHubInstallation(r.Context(), r.URL.Query().Get("state"), installationID); err != nil {
		a.log.Warn("complete GitHub App installation", "installation", installationID, "error", err)
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/integrations?connected=github", http.StatusFound)
}

func (a *API) oauthStart(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	destination, err := a.integrations.Start(r.Context(), claims.Subject, r.PathValue("provider"))
	if err != nil {
		bad(w, err.Error())
		return
	}
	http.Redirect(w, r, destination, http.StatusFound)
}

func (a *API) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "github" && strings.HasPrefix(r.URL.Query().Get("state"), "account.") {
		a.githubAccountCallback(w, r)
		return
	}
	if oauthError := r.URL.Query().Get("error"); oauthError != "" {
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape(oauthError), http.StatusFound)
		return
	}
	if err := a.integrations.Complete(r.Context(), provider, r.URL.Query().Get("state"), r.URL.Query().Get("code")); err != nil {
		a.log.Warn("OAuth callback failed", "provider", provider, "error", err)
		http.Redirect(w, r, "/integrations?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/integrations?connected="+url.QueryEscape(provider), http.StatusFound)
}

func (a *API) repositories(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	connection, err := a.store.SourceConnection(r.Context(), r.PathValue("id"), claims.Subject)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "source connection not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	items, err := a.integrations.Repositories(r.Context(), connection)
	if err != nil {
		a.log.Warn("list repositories", "provider", connection.Provider, "error", err)
		write(w, 502, map[string]string{"error": "could not load repositories from " + connection.Provider})
		return
	}
	write(w, 200, map[string]any{
		"connection":   connection,
		"repositories": items,
	})
}

func (a *API) deleteSourceConnection(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	connection, err := a.store.SourceConnection(r.Context(), r.PathValue("id"), claims.Subject)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "source connection not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if err := a.store.DeleteSourceConnection(r.Context(), connection.ID, claims.Subject); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]string{"message": "Git source unlinked. Existing containers keep running, but repository services need a new source connection before their next deployment."})
}

func (a *API) createRegistry(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		RegistryURL string `json:"registryUrl"`
		Username    string `json:"username"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.RegistryURL = strings.TrimSpace(strings.TrimSuffix(in.RegistryURL, "/"))
	in.Username = strings.TrimSpace(in.Username)
	if in.Name == "" || in.RegistryURL == "" {
		bad(w, "name and registry URL are required")
		return
	}
	if strings.ContainsAny(in.RegistryURL, " \t\r\n") {
		bad(w, "registry URL is invalid")
		return
	}
	sealed := ""
	var err error
	if in.Password != "" {
		sealed, err = a.box.Encrypt(in.Password)
		if err != nil {
			problem(w, err)
			return
		}
	}
	claims, _ := auth.FromContext(r.Context())
	c := store.RegistryCredential{ID: newID("reg"), Name: in.Name, RegistryURL: in.RegistryURL, Username: in.Username, PasswordEncrypted: sealed, CreatedBy: claims.Subject}
	if err := a.store.CreateRegistry(r.Context(), c); err != nil {
		problem(w, err)
		return
	}
	created, err := a.store.Registry(r.Context(), c.ID, claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 201, created)
}

func (a *API) deleteRegistry(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	err := a.store.DeleteRegistry(r.Context(), r.PathValue("id"), claims.Subject)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "registry not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *API) project(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	p, err := a.store.Project(r.Context(), id)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	deployments, err := a.store.Deployments(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	services := []runtime.Service{}
	if p.SourceType != "empty" {
		if service, serviceErr := a.docker.ProjectService(r.Context(), id); serviceErr == nil {
			services = append(services, service)
		} else if !errors.Is(serviceErr, runtime.ErrNotFound) {
			a.log.Warn("inspect project container", "project", id, "error", serviceErr)
		}
	}
	databaseServices, err := a.databaseServices(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	ingressRules, err := a.store.ProjectIngressRules(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	ingressDefaultPath, err := a.store.ProjectIngressDefaultPath(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	domainBindings, err := a.store.ProjectDomainBindings(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	applicationServices, err := a.store.ApplicationServices(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	for index := range applicationServices {
		runtimeService, runtimeErr := a.docker.ApplicationService(r.Context(), applicationServices[index].ID, applicationServices[index].Name)
		if runtimeErr == nil {
			applicationServices[index].Status = runtimeService.Status
			applicationServices[index].Container = runtimeService.Container
		} else if !errors.Is(runtimeErr, runtime.ErrNotFound) {
			a.log.Warn("inspect application service", "service", applicationServices[index].ID, "error", runtimeErr)
		}
	}
	write(w, 200, map[string]any{"project": p, "deployments": deployments, "services": services, "applicationServices": applicationServices, "databaseServices": databaseServices, "ingressRules": ingressRules, "ingressDefaultPath": ingressDefaultPath, "domainBindings": domainBindings})
}

func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          string `json:"name"`
		SourceType    string `json:"sourceType"`
		Repository    string `json:"repository"`
		Branch        string `json:"branch"`
		ConnectionID  string `json:"connectionId"`
		ImageURL      string `json:"imageUrl"`
		RegistryID    string `json:"registryId"`
		ContainerPort int    `json:"containerPort"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.projectMu.Lock()
	defer a.projectMu.Unlock()
	project, err := a.store.Project(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.SourceType = strings.TrimSpace(in.SourceType)
	in.Repository = strings.TrimSpace(in.Repository)
	in.Branch = strings.TrimSpace(in.Branch)
	in.ConnectionID = strings.TrimSpace(in.ConnectionID)
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	in.RegistryID = strings.TrimSpace(in.RegistryID)
	if in.ContainerPort < 1 || in.ContainerPort > 65535 {
		bad(w, "container port must be between 1 and 65535")
		return
	}
	if in.Name == "" || len(in.Name) > 100 {
		bad(w, "project name is required and must be at most 100 characters")
		return
	}
	if in.SourceType != "repository" && in.SourceType != "image" && in.SourceType != "empty" {
		bad(w, "source type must be empty, repository, or image")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	if in.SourceType == "repository" {
		if in.Repository == "" || strings.ContainsAny(in.Repository, " \t\r\n") {
			bad(w, "enter a valid repository URL")
			return
		}
		if in.Branch == "" {
			in.Branch = "main"
		}
		if in.ConnectionID != "" {
			if _, err := a.store.SourceConnection(r.Context(), in.ConnectionID, claims.Subject); err != nil {
				bad(w, "source connection is invalid")
				return
			}
		}
		in.ImageURL = ""
		in.RegistryID = ""
	} else if in.SourceType == "image" {
		if in.ImageURL == "" || strings.ContainsAny(in.ImageURL, " \t\r\n") {
			bad(w, "enter a valid container image")
			return
		}
		if in.RegistryID != "" {
			if _, err := a.store.Registry(r.Context(), in.RegistryID, claims.Subject); err != nil {
				bad(w, "registry credential is invalid")
				return
			}
		}
		in.Repository = ""
		in.ConnectionID = ""
		if in.Branch == "" {
			in.Branch = "main"
		}
	} else {
		in.Repository = ""
		in.ConnectionID = ""
		in.ImageURL = ""
		in.RegistryID = ""
		in.Branch = "main"
	}
	project.Name = in.Name
	project.SourceType = in.SourceType
	project.Repository = in.Repository
	project.Branch = in.Branch
	project.ConnectionID = in.ConnectionID
	project.ImageURL = in.ImageURL
	project.RegistryID = in.RegistryID
	portChanged := project.ContainerPort != in.ContainerPort
	project.ContainerPort = in.ContainerPort
	if err := a.store.UpdateProject(r.Context(), project); err != nil {
		problem(w, err)
		return
	}
	updated, err := a.store.Project(r.Context(), project.ID)
	if err != nil {
		problem(w, err)
		return
	}
	if portChanged && updated.Domain != "" {
		if err := a.SyncDomains(r.Context()); err != nil {
			a.log.Warn("refresh project route after port change", "project", project.ID, "error", err)
		}
	}
	write(w, 200, updated)
}

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirmation string `json:"confirmation"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.projectMu.Lock()
	defer a.projectMu.Unlock()
	project, err := a.store.Project(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if in.Confirmation != project.Name {
		write(w, 409, map[string]string{"error": "type the exact project name to confirm deletion"})
		return
	}

	a.databaseMu.Lock()
	applicationServices, err := a.store.ApplicationServices(r.Context(), project.ID)
	if err != nil {
		a.databaseMu.Unlock()
		problem(w, err)
		return
	}
	databaseServices, err := a.store.ProjectDatabaseServices(r.Context(), project.ID)
	if err != nil {
		a.databaseMu.Unlock()
		problem(w, err)
		return
	}
	if err := a.docker.RemoveProject(r.Context(), project.ID); err != nil {
		a.databaseMu.Unlock()
		a.log.Warn("remove project container", "project", project.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not remove the project container"})
		return
	}
	for _, service := range applicationServices {
		if err := a.docker.RemoveApplication(r.Context(), service.ID); err != nil {
			a.databaseMu.Unlock()
			a.log.Warn("remove application service", "project", project.ID, "service", service.ID, "error", err)
			write(w, 502, map[string]string{"error": "could not remove all application service containers"})
			return
		}
	}
	for _, service := range databaseServices {
		if err := a.docker.RemoveDatabase(r.Context(), service.ID, service.VolumeName, true); err != nil {
			a.databaseMu.Unlock()
			a.log.Warn("remove project database", "project", project.ID, "database", service.ID, "error", err)
			write(w, 502, map[string]string{"error": "could not remove all project database resources"})
			return
		}
	}
	a.databaseMu.Unlock()

	if err := a.store.DeleteProject(r.Context(), project.ID); err != nil {
		problem(w, err)
		return
	}
	warning := ""
	a.domainMu.Lock()
	if err := a.syncDomains(r.Context()); err != nil {
		warning = "Project deleted, but Caddy routes could not be refreshed immediately."
		a.log.Warn("refresh domains after project deletion", "project", project.ID, "error", err)
	}
	a.domainMu.Unlock()
	write(w, 200, map[string]any{"ok": true, "warning": warning})
}

type environmentVariableInput struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func (a *API) environmentVariables(ctx context.Context, projectID string) ([]environmentVariableInput, []store.ProjectEnvironmentVariable, error) {
	stored, err := a.store.ProjectEnvironmentVariables(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	variables := make([]environmentVariableInput, 0, len(stored))
	for _, variable := range stored {
		value, err := a.box.Decrypt(variable.ValueEncrypted)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt environment variable %s: %w", variable.Key, err)
		}
		variables = append(variables, environmentVariableInput{Key: variable.Key, Value: value, Secret: variable.Secret})
	}
	return variables, stored, nil
}

func (a *API) projectEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	variables, _, err := a.environmentVariables(r.Context(), projectID)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"variables": variables})
}

func (a *API) updateProjectEnvironment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Variables []environmentVariableInput `json:"variables"`
	}
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	a.projectMu.Lock()
	defer a.projectMu.Unlock()
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	if _, err := a.docker.ProjectService(r.Context(), projectID); errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy the application before applying environment variables"})
		return
	} else if err != nil {
		write(w, 502, map[string]string{"error": "could not inspect the running application"})
		return
	}

	seen := map[string]bool{}
	stored := make([]store.ProjectEnvironmentVariable, 0, len(in.Variables))
	runtimeEnvironment := make([]string, 0, len(in.Variables))
	clean := make([]environmentVariableInput, 0, len(in.Variables))
	for _, variable := range in.Variables {
		variable.Key = strings.TrimSpace(variable.Key)
		if variable.Key == "" || len(variable.Key) > 128 {
			bad(w, "each environment variable needs a key of at most 128 characters")
			return
		}
		for index, character := range variable.Key {
			if !(character == '_' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || index > 0 && character >= '0' && character <= '9') {
				bad(w, "environment variable keys must use letters, numbers, and underscores and cannot start with a number")
				return
			}
		}
		if seen[variable.Key] {
			bad(w, "environment variable keys must be unique")
			return
		}
		if len(variable.Value) > 16<<10 || strings.ContainsRune(variable.Value, '\x00') {
			bad(w, "environment variable values must be at most 16 KB and cannot contain null characters")
			return
		}
		seen[variable.Key] = true
		encrypted, err := a.box.Encrypt(variable.Value)
		if err != nil {
			problem(w, err)
			return
		}
		stored = append(stored, store.ProjectEnvironmentVariable{ProjectID: projectID, Key: variable.Key, ValueEncrypted: encrypted, Secret: variable.Secret})
		runtimeEnvironment = append(runtimeEnvironment, variable.Key+"="+variable.Value)
		clean = append(clean, variable)
	}

	_, previousStored, err := a.environmentVariables(r.Context(), projectID)
	if err != nil {
		problem(w, err)
		return
	}
	previousKeys := make([]string, 0, len(previousStored))
	for _, variable := range previousStored {
		previousKeys = append(previousKeys, variable.Key)
	}
	if err := a.store.ReplaceProjectEnvironmentVariables(r.Context(), projectID, stored); err != nil {
		problem(w, err)
		return
	}
	service, err := a.docker.RestartProjectWithEnvironment(r.Context(), projectID, runtimeEnvironment, previousKeys)
	if err != nil {
		if rollbackErr := a.store.ReplaceProjectEnvironmentVariables(context.Background(), projectID, previousStored); rollbackErr != nil {
			a.log.Error("rollback environment variables", "project", projectID, "error", rollbackErr)
		}
		a.log.Warn("restart project with environment", "project", projectID, "error", err)
		write(w, 502, map[string]string{"error": "environment variables were not applied; the previous container was restored"})
		return
	}
	_ = a.store.UpdateProjectStatus(r.Context(), projectID, service.Status)
	write(w, 200, map[string]any{"variables": clean, "service": service, "message": "Environment saved and application restarted without rebuilding"})
}

func (a *API) databaseServices(ctx context.Context, projectID string) ([]store.DatabaseService, error) {
	services, err := a.store.ProjectDatabaseServices(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for index := range services {
		services[index].Container = "selfhost-db-" + services[index].ID
		services[index].InternalAddress = services[index].Container + ":" + strconv.Itoa(services[index].InternalPort)
		services[index].Status = "degraded"
		runtimeState, runtimeErr := a.docker.DatabaseRuntime(ctx, services[index].ID, services[index].InternalPort)
		if runtimeErr == nil {
			services[index].Status = runtimeState.Status
			services[index].Container = runtimeState.Container
		} else if !errors.Is(runtimeErr, runtime.ErrNotFound) {
			a.log.Warn("inspect database container", "database", services[index].ID, "error", runtimeErr)
		}
	}
	return services, nil
}

func (a *API) createDatabaseService(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          string `json:"name"`
		Engine        string `json:"engine"`
		DatabaseName  string `json:"databaseName"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		PublicEnabled bool   `json:"publicEnabled"`
		PublicPort    int    `json:"publicPort"`
	}
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	in.Engine = strings.ToLower(strings.TrimSpace(in.Engine))
	preset, ok := runtime.DatabaseEngine(in.Engine)
	if !ok {
		bad(w, "database engine must be mysql, mariadb, or postgres")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		in.Name = strings.ToUpper(in.Engine[:1]) + in.Engine[1:]
	}
	if len(in.Name) > 50 || strings.ContainsAny(in.Name, "\r\n\t") {
		bad(w, "service name must contain at most 50 characters")
		return
	}
	in.DatabaseName = strings.TrimSpace(in.DatabaseName)
	if in.DatabaseName == "" {
		in.DatabaseName = "app"
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		in.Username = "app"
	}
	if !databaseIdentifier(in.DatabaseName) || !databaseIdentifier(in.Username) {
		bad(w, "database and user names may contain letters, numbers, and underscores")
		return
	}
	if (in.Engine == "mysql" || in.Engine == "mariadb") && strings.EqualFold(in.Username, "root") {
		bad(w, "use an application username other than root")
		return
	}
	if in.Password == "" {
		in.Password = randomSecret()
	} else if len(in.Password) < 12 {
		bad(w, "password must contain at least 12 characters")
		return
	}
	if in.PublicEnabled {
		if in.PublicPort == 0 {
			in.PublicPort = preset.Port
		}
		if in.PublicPort < 1 || in.PublicPort > 65535 {
			bad(w, "public port must be between 1 and 65535")
			return
		}
	} else {
		in.PublicPort = 0
	}
	sealed, err := a.box.Encrypt(in.Password)
	if err != nil {
		problem(w, err)
		return
	}
	serviceID := newID("db")
	service := store.DatabaseService{ID: serviceID, ProjectID: projectID, Name: in.Name, Engine: in.Engine, Image: preset.Image, InternalPort: preset.Port, PublicEnabled: in.PublicEnabled, PublicPort: in.PublicPort, VolumeName: "selfhost-data-" + serviceID, Username: in.Username, DatabaseName: in.DatabaseName, PasswordEncrypted: sealed}

	a.databaseMu.Lock()
	defer a.databaseMu.Unlock()
	if err := a.store.CreateDatabaseService(r.Context(), service); err != nil {
		if strings.Contains(err.Error(), "database_services_public_port_unique") {
			write(w, 409, map[string]string{"error": "this public port is already assigned to another database"})
			return
		}
		if strings.Contains(err.Error(), "database_services_project_name_unique") {
			write(w, 409, map[string]string{"error": "a database service with this name already exists"})
			return
		}
		problem(w, err)
		return
	}
	report := func(stage, eventType, message string) {
		a.recordDatabaseDeploymentEvent(r.Context(), service.ID, stage, eventType, message)
	}
	if _, err := a.docker.DeployDatabase(r.Context(), databaseSpec(service, in.Password), report); err != nil {
		_ = a.docker.RemoveDatabase(r.Context(), service.ID, service.VolumeName, true)
		_ = a.store.DeleteDatabaseService(r.Context(), service.ID)
		a.log.Error("deploy database", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": err.Error()})
		return
	}
	created, err := a.store.DatabaseService(r.Context(), service.ID)
	if err != nil {
		problem(w, err)
		return
	}
	created.Container = "selfhost-db-" + created.ID
	created.InternalAddress = created.Container + ":" + strconv.Itoa(created.InternalPort)
	created.Status = "deploying"
	write(w, 201, map[string]any{"service": created, "credentials": credentialsFor(created, in.Password)})
}

func (a *API) databaseCredentials(w http.ResponseWriter, r *http.Request) {
	service, err := a.store.DatabaseService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	password, err := a.box.Decrypt(service.PasswordEncrypted)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, credentialsFor(service, password))
}

func (a *API) databaseDeploymentEvents(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.DatabaseService(r.Context(), id); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	events, err := a.store.DatabaseDeploymentEvents(r.Context(), id)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"events": events})
}

func (a *API) databaseLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	service, err := a.store.DatabaseService(r.Context(), id)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	tail := 300
	if requested := strings.TrimSpace(r.URL.Query().Get("lines")); requested != "" {
		parsed, parseErr := strconv.Atoi(requested)
		if parseErr != nil || parsed < 1 || parsed > 1000 {
			bad(w, "lines must be a number between 1 and 1000")
			return
		}
		tail = parsed
	}
	lines, err := a.docker.DatabaseLogs(r.Context(), id, tail)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 404, map[string]string{"error": "database container is not running"})
		return
	}
	if err != nil {
		a.log.Warn("read database logs", "database", id, "error", err)
		write(w, 502, map[string]string{"error": "could not read database container logs"})
		return
	}
	write(w, 200, map[string]any{"lines": lines, "count": len(lines), "limit": tail, "container": "selfhost-db-" + service.ID})
}

func (a *API) stopDatabaseService(w http.ResponseWriter, r *http.Request) {
	a.databaseMu.Lock()
	defer a.databaseMu.Unlock()

	service, err := a.store.DatabaseService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	runtimeState, err := a.docker.DatabaseRuntime(r.Context(), service.ID, service.InternalPort)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this database before stopping it"})
		return
	} else if err != nil {
		a.log.Warn("inspect database before stop", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not inspect the database container"})
		return
	}
	if runtimeState.Status != "stopped" {
		if err := a.docker.StopDatabase(r.Context(), service.ID); err != nil {
			a.log.Warn("stop database service", "database", service.ID, "error", err)
			write(w, 502, map[string]string{"error": "could not stop the database container"})
			return
		}
		a.recordDatabaseDeploymentEvent(r.Context(), service.ID, "control", "complete", "Database container stopped manually")
	}
	service.Status = "stopped"
	service.Container = runtimeState.Container
	service.InternalAddress = service.Container + ":" + strconv.Itoa(service.InternalPort)
	write(w, 200, map[string]any{"service": service, "message": service.Name + " stopped"})
}

func (a *API) restartDatabaseService(w http.ResponseWriter, r *http.Request) {
	a.databaseMu.Lock()
	defer a.databaseMu.Unlock()

	service, err := a.store.DatabaseService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if err := a.docker.RestartDatabase(r.Context(), service.ID); errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this database before restarting it"})
		return
	} else if err != nil {
		a.log.Warn("restart database service", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not restart the database container"})
		return
	}
	runtimeState, err := a.docker.DatabaseRuntime(r.Context(), service.ID, service.InternalPort)
	if err != nil {
		a.log.Warn("inspect restarted database service", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "database restarted, but its runtime status is unavailable"})
		return
	}
	service.Status = runtimeState.Status
	service.Container = runtimeState.Container
	service.InternalAddress = service.Container + ":" + strconv.Itoa(service.InternalPort)
	a.recordDatabaseDeploymentEvent(r.Context(), service.ID, "control", "complete", "Database container restarted manually")
	write(w, 200, map[string]any{"service": service, "message": service.Name + " restarted"})
}

func (a *API) deleteDatabaseService(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirmation string `json:"confirmation"`
		RemoveVolume bool   `json:"removeVolume"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.databaseMu.Lock()
	defer a.databaseMu.Unlock()
	id := strings.TrimSpace(r.PathValue("id"))
	service, err := a.store.DatabaseService(r.Context(), id)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if in.Confirmation != service.Name {
		bad(w, "type the database service name exactly to confirm removal")
		return
	}
	if err := a.docker.RemoveDatabase(r.Context(), service.ID, service.VolumeName, in.RemoveVolume); err != nil {
		a.log.Error("remove database", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "Docker could not remove the database container"})
		return
	}
	if err := a.store.DeleteDatabaseService(r.Context(), service.ID); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"removed": true, "volumeRemoved": in.RemoveVolume, "retainedVolume": map[bool]string{true: "", false: service.VolumeName}[in.RemoveVolume]})
}

func (a *API) updateDatabaseExposure(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.databaseMu.Lock()
	defer a.databaseMu.Unlock()
	service, err := a.store.DatabaseService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "database service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if in.Enabled {
		if in.Port == 0 {
			in.Port = service.InternalPort
		}
		if in.Port < 1 || in.Port > 65535 {
			bad(w, "public port must be between 1 and 65535")
			return
		}
	} else {
		in.Port = 0
	}
	password, err := a.box.Decrypt(service.PasswordEncrypted)
	if err != nil {
		problem(w, err)
		return
	}
	oldEnabled, oldPort := service.PublicEnabled, service.PublicPort
	if err := a.store.UpdateDatabaseExposure(r.Context(), service.ID, in.Enabled, in.Port); err != nil {
		if strings.Contains(err.Error(), "database_services_public_port_unique") {
			write(w, 409, map[string]string{"error": "this public port is already assigned to another database"})
			return
		}
		problem(w, err)
		return
	}
	service.PublicEnabled, service.PublicPort = in.Enabled, in.Port
	report := func(stage, eventType, message string) {
		a.recordDatabaseDeploymentEvent(r.Context(), service.ID, stage, eventType, message)
	}
	if _, err := a.docker.DeployDatabase(r.Context(), databaseSpec(service, password), report); err != nil {
		_ = a.store.UpdateDatabaseExposure(r.Context(), service.ID, oldEnabled, oldPort)
		service.PublicEnabled, service.PublicPort = oldEnabled, oldPort
		_, _ = a.docker.DeployDatabase(r.Context(), databaseSpec(service, password))
		a.log.Error("update database exposure", "database", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "Docker could not apply this port; the previous exposure was restored"})
		return
	}
	service.Container = "selfhost-db-" + service.ID
	service.InternalAddress = service.Container + ":" + strconv.Itoa(service.InternalPort)
	service.Status = "deploying"
	write(w, 200, map[string]any{"service": service})
}

func databaseSpec(service store.DatabaseService, password string) runtime.DatabaseSpec {
	return runtime.DatabaseSpec{ID: service.ID, ProjectID: service.ProjectID, Engine: service.Engine, Image: service.Image, Port: service.InternalPort, VolumeName: service.VolumeName, Username: service.Username, DatabaseName: service.DatabaseName, Password: password, PublicEnabled: service.PublicEnabled, PublicPort: service.PublicPort}
}

func (a *API) recordDatabaseDeploymentEvent(ctx context.Context, serviceID, stage, eventType, message string) {
	if err := a.store.AppendDatabaseDeploymentEvent(ctx, store.DatabaseDeploymentEvent{DatabaseServiceID: serviceID, Stage: stage, Type: eventType, Message: message}); err != nil {
		a.log.Warn("record database deployment event", "database", serviceID, "stage", stage, "error", err)
	}
}

func credentialsFor(service store.DatabaseService, password string) map[string]any {
	host := "selfhost-db-" + service.ID
	scheme := "mysql"
	if service.Engine == "postgres" {
		scheme = "postgresql"
	}
	userinfo := url.UserPassword(service.Username, password).String()
	connection := fmt.Sprintf("%s://%s@%s:%d/%s", scheme, userinfo, host, service.InternalPort, service.DatabaseName)
	return map[string]any{"username": service.Username, "password": password, "database": service.DatabaseName, "host": host, "port": service.InternalPort, "connectionUrl": connection, "publicEnabled": service.PublicEnabled, "publicPort": service.PublicPort}
}

func databaseIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 63 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func randomSecret() string {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}

func (a *API) deployProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	project, err := a.store.Project(r.Context(), id)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if project.SourceType == "empty" {
		bad(w, "this project has no legacy application; deploy one of its services")
		return
	}
	if project.SourceType != "image" {
		bad(w, "repository builds are not available yet; deploy a container image")
		return
	}
	if strings.TrimSpace(project.ImageURL) == "" {
		bad(w, "project has no container image")
		return
	}

	deployment := store.Deployment{
		ID:          newID("dep"),
		ProjectID:   project.ID,
		ServiceName: project.Name,
		Commit:      project.ImageURL,
		Message:     "Deploy " + project.Name,
		Status:      "deploying",
	}
	if err := a.store.CreateDeployment(r.Context(), deployment); err != nil {
		problem(w, err)
		return
	}
	_ = a.store.UpdateProjectStatus(r.Context(), project.ID, "deploying")
	started := time.Now()

	var registryAuth *runtime.RegistryAuth
	if project.RegistryID != "" {
		claims, _ := auth.FromContext(r.Context())
		credential, credentialErr := a.store.Registry(r.Context(), project.RegistryID, claims.Subject)
		if credentialErr != nil {
			a.recordDeploymentEvent(r.Context(), deployment.ID, "prepare", "error", "Registry credential is unavailable")
			a.failDeployment(r.Context(), deployment.ID, project.ID, started, "Registry credential is unavailable", credentialErr)
			write(w, 400, map[string]string{"error": "registry credential is unavailable"})
			return
		}
		password := ""
		if credential.PasswordEncrypted != "" {
			password, err = a.box.Decrypt(credential.PasswordEncrypted)
			if err != nil {
				a.recordDeploymentEvent(r.Context(), deployment.ID, "prepare", "error", "Registry credential could not be decrypted")
				a.failDeployment(r.Context(), deployment.ID, project.ID, started, "Registry credential could not be decrypted", err)
				problem(w, err)
				return
			}
		}
		registryAuth = &runtime.RegistryAuth{Username: credential.Username, Password: password, ServerAddress: credential.RegistryURL}
	}

	a.recordDeploymentEvent(r.Context(), deployment.ID, "prepare", "complete", "Deployment accepted by the control plane")
	deploymentCtx, cancelDeployment := context.WithCancel(context.Background())
	a.registerDeployment(deployment.ID, cancelDeployment)
	go a.runImageDeployment(deploymentCtx, deployment, project, registryAuth, started)
	write(w, http.StatusAccepted, map[string]any{"deployment": deployment})
}

func (a *API) runImageDeployment(deploymentCtx context.Context, deployment store.Deployment, project store.Project, registryAuth *runtime.RegistryAuth, started time.Time) {
	ctx, cancel := context.WithTimeout(deploymentCtx, 15*time.Minute)
	defer cancel()
	defer a.unregisterDeployment(deployment.ID)
	activeStage := "prepare"
	progress := func(stage, eventType, message string) {
		if eventType == "start" {
			activeStage = stage
		}
		a.recordDeploymentEvent(ctx, deployment.ID, stage, eventType, message)
	}

	service, err := a.docker.DeployImage(ctx, project.ID, project.ImageURL, project.ContainerPort, registryAuth, progress)
	duration := int(time.Since(started).Round(time.Second).Seconds())
	if err != nil {
		if deploymentCancelled(ctx, err) {
			if current, currentErr := a.docker.ProjectService(context.Background(), project.ID); currentErr == nil {
				_ = a.store.UpdateProjectStatus(context.Background(), project.ID, current.Status)
			} else {
				_ = a.store.UpdateProjectStatus(context.Background(), project.ID, "stopped")
			}
			a.recordDeploymentEvent(context.Background(), deployment.ID, activeStage, "cancelled", "Deployment stopped by user")
			_ = a.store.FinishDeployment(context.Background(), deployment.ID, "cancelled", "Deployment stopped", duration)
			return
		}
		message := "Deployment failed"
		a.recordDeploymentEvent(context.Background(), deployment.ID, activeStage, "error", err.Error())
		a.failDeployment(context.Background(), deployment.ID, project.ID, started, message, err)
		return
	}
	if err := a.store.FinishDeployment(ctx, deployment.ID, "healthy", "Deployed "+project.ImageURL, duration); err != nil {
		a.log.Error("finish deployment", "deployment", deployment.ID, "error", err)
		return
	}
	_ = a.store.UpdateProjectStatus(ctx, project.ID, "healthy")
	a.recordDeploymentEvent(ctx, deployment.ID, "complete", "complete", "Deployment finished successfully on "+service.Container)
	a.queueDeploymentNotification(deployment.ID, project.ID, project.Name, "healthy", "Deployment finished successfully on "+service.Container)
}

func (a *API) stopProjectService(w http.ResponseWriter, r *http.Request) {
	a.projectMu.Lock()
	defer a.projectMu.Unlock()

	project, err := a.store.Project(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if project.SourceType == "empty" {
		write(w, 409, map[string]string{"error": "this project has no legacy application"})
		return
	}
	if project.Status == "deploying" {
		write(w, 409, map[string]string{"error": "wait for the current deployment to finish before stopping this service"})
		return
	}
	service, err := a.docker.ProjectService(r.Context(), project.ID)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this service before stopping it"})
		return
	} else if err != nil {
		a.log.Warn("inspect project service before stop", "project", project.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not inspect the service container"})
		return
	}
	if service.Status != "stopped" {
		if err := a.docker.StopProject(r.Context(), project.ID); err != nil {
			a.log.Warn("stop project service", "project", project.ID, "error", err)
			write(w, 502, map[string]string{"error": "could not stop the service container"})
			return
		}
	}
	_ = a.store.UpdateProjectStatus(r.Context(), project.ID, "stopped")
	service.Name = project.Name
	service.Status = "stopped"
	write(w, 200, map[string]any{"service": service, "message": project.Name + " stopped"})
}

func (a *API) restartProjectService(w http.ResponseWriter, r *http.Request) {
	a.projectMu.Lock()
	defer a.projectMu.Unlock()

	project, err := a.store.Project(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if project.SourceType == "empty" {
		write(w, 409, map[string]string{"error": "this project has no legacy application"})
		return
	}
	if project.Status == "deploying" {
		write(w, 409, map[string]string{"error": "wait for the current deployment to finish before restarting this service"})
		return
	}
	if err := a.docker.RestartProject(r.Context(), project.ID); errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this service before restarting it"})
		return
	} else if err != nil {
		a.log.Warn("restart project service", "project", project.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not restart the service container"})
		return
	}
	service, err := a.docker.ProjectService(r.Context(), project.ID)
	if err != nil {
		a.log.Warn("inspect restarted project service", "project", project.ID, "error", err)
		write(w, 502, map[string]string{"error": "service restarted, but its runtime status is unavailable"})
		return
	}
	service.Name = project.Name
	_ = a.store.UpdateProjectStatus(r.Context(), project.ID, service.Status)
	write(w, 200, map[string]any{"service": service, "message": project.Name + " restarted"})
}

func parseServiceEnvironment(value string) ([]string, error) {
	lines := []string{}
	seen := map[string]bool{}
	for _, raw := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, variableValue, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || !environmentKey(key) {
			return nil, fmt.Errorf("environment lines must use KEY=value syntax")
		}
		if seen[key] {
			return nil, fmt.Errorf("environment variable %s is duplicated", key)
		}
		seen[key] = true
		lines = append(lines, key+"="+strings.TrimSpace(variableValue))
	}
	return lines, nil
}

func environmentKey(value string) bool {
	if value == "" || len(value) > 128 || value[0] >= '0' && value[0] <= '9' {
		return false
	}
	for _, character := range value {
		if character != '_' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

type applicationServiceInput struct {
	Name                      string `json:"name"`
	SourceType                string `json:"sourceType"`
	ImageURL                  string `json:"imageUrl"`
	RegistryID                string `json:"registryId"`
	ConnectionID              string `json:"connectionId"`
	Repository                string `json:"repository"`
	Branch                    string `json:"branch"`
	DockerfilePath            string `json:"dockerfilePath"`
	BuildContext              string `json:"buildContext"`
	BuildStrategy             string `json:"buildStrategy"`
	ContainerPort             int    `json:"containerPort"`
	Command                   string `json:"command"`
	HealthCheckType           string `json:"healthCheckType"`
	HealthCheckPath           string `json:"healthCheckPath"`
	HealthCheckCommand        string `json:"healthCheckCommand"`
	HealthCheckTimeoutSeconds int    `json:"healthCheckTimeoutSeconds"`
	Environment               string `json:"environment"`
}

func cleanApplicationServiceInput(in applicationServiceInput) (applicationServiceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.SourceType = strings.TrimSpace(in.SourceType)
	if in.SourceType == "" {
		in.SourceType = "image"
	}
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	in.RegistryID = strings.TrimSpace(in.RegistryID)
	in.ConnectionID = strings.TrimSpace(in.ConnectionID)
	in.Repository = strings.TrimSpace(in.Repository)
	in.Branch = strings.TrimSpace(in.Branch)
	in.DockerfilePath = strings.TrimSpace(in.DockerfilePath)
	in.BuildContext = strings.TrimSpace(in.BuildContext)
	in.BuildStrategy = strings.TrimSpace(in.BuildStrategy)
	in.Command = strings.TrimSpace(in.Command)
	in.HealthCheckType = strings.TrimSpace(in.HealthCheckType)
	in.HealthCheckPath = strings.TrimSpace(in.HealthCheckPath)
	in.HealthCheckCommand = strings.TrimSpace(in.HealthCheckCommand)
	if in.Name == "" || len(in.Name) > 63 {
		return in, errors.New("service name is required and must be at most 63 characters")
	}
	if in.SourceType != "image" && in.SourceType != "repository" {
		return in, errors.New("choose a Docker image or Git repository source")
	}
	if in.SourceType == "image" {
		if in.ImageURL == "" || strings.ContainsAny(in.ImageURL, " \t\r\n") {
			return in, errors.New("enter a valid container image")
		}
		// build_strategy is irrelevant for a prebuilt image, but the database
		// deliberately keeps it constrained to the supported strategy values.
		// Preserve the schema default instead of writing an empty string, which
		// would reject every image service before its deployment can start.
		in.ConnectionID, in.Repository, in.Branch, in.DockerfilePath, in.BuildContext = "", "", "", "", ""
		in.BuildStrategy = "dockerfile"
	} else {
		if in.ConnectionID == "" {
			return in, errors.New("choose a connected GitHub or GitLab account")
		}
		if in.Repository == "" || strings.ContainsAny(in.Repository, " \t\r\n") {
			return in, errors.New("choose a repository")
		}
		if in.Branch == "" {
			in.Branch = "main"
		}
		if strings.ContainsAny(in.Branch, " \t\r\n") {
			return in, errors.New("branch names cannot contain spaces")
		}
		if in.BuildStrategy == "" {
			in.BuildStrategy = "dockerfile"
		}
		if in.BuildStrategy != "dockerfile" && in.BuildStrategy != "railpack" && in.BuildStrategy != "nixpacks" {
			return in, errors.New("choose Dockerfile, Railpack, or Nixpacks")
		}
		if in.BuildStrategy == "dockerfile" {
			if in.DockerfilePath == "" {
				in.DockerfilePath = "Dockerfile"
			}
			if in.BuildContext == "" {
				in.BuildContext = "."
			}
			for label, value := range map[string]string{"Dockerfile path": in.DockerfilePath, "build context": in.BuildContext} {
				clean := filepath.Clean(value)
				if filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
					return in, fmt.Errorf("%s must stay inside the repository", label)
				}
			}
		} else {
			in.DockerfilePath, in.BuildContext = "", "."
		}
		in.ImageURL, in.RegistryID = "", ""
	}
	if in.ContainerPort == 0 {
		in.ContainerPort = 80
	}
	if in.ContainerPort < 1 || in.ContainerPort > 65535 {
		return in, errors.New("container port must be between 1 and 65535")
	}
	if len(in.Command) > 4096 || strings.ContainsRune(in.Command, '\x00') {
		return in, errors.New("container command must be at most 4096 characters and cannot contain null characters")
	}
	if in.HealthCheckType == "" {
		in.HealthCheckType = "none"
	}
	if in.HealthCheckTimeoutSeconds == 0 {
		in.HealthCheckTimeoutSeconds = 60
	}
	if in.HealthCheckTimeoutSeconds < 5 || in.HealthCheckTimeoutSeconds > 600 {
		return in, errors.New("health check timeout must be between 5 and 600 seconds")
	}
	switch in.HealthCheckType {
	case "none":
		in.HealthCheckPath, in.HealthCheckCommand = "", ""
	case "http":
		if in.HealthCheckPath == "" {
			in.HealthCheckPath = "/"
		}
		if !strings.HasPrefix(in.HealthCheckPath, "/") || len(in.HealthCheckPath) > 2048 || strings.ContainsAny(in.HealthCheckPath, " \t\r\n") {
			return in, errors.New("health check path must start with / and cannot contain spaces")
		}
		in.HealthCheckCommand = ""
	case "command":
		if in.HealthCheckCommand == "" {
			return in, errors.New("enter a health check command")
		}
		if len(in.HealthCheckCommand) > 4096 || strings.ContainsRune(in.HealthCheckCommand, '\x00') {
			return in, errors.New("health check command must be at most 4096 characters and cannot contain null characters")
		}
		in.HealthCheckPath = ""
	default:
		return in, errors.New("choose container running, HTTP path, or command health verification")
	}
	return in, nil
}

func (a *API) createApplicationService(w http.ResponseWriter, r *http.Request) {
	var in applicationServiceInput
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	var err error
	in, err = cleanApplicationServiceInput(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	environment, err := parseServiceEnvironment(in.Environment)
	if err != nil {
		bad(w, err.Error())
		return
	}
	environmentEncrypted, err := a.box.Encrypt(strings.Join(environment, "\n"))
	if err != nil {
		problem(w, err)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	if in.RegistryID != "" {
		if _, err := a.store.Registry(r.Context(), in.RegistryID, claims.Subject); err != nil {
			bad(w, "registry credential is invalid")
			return
		}
	}
	if in.SourceType == "repository" {
		connection, err := a.store.SourceConnection(r.Context(), in.ConnectionID, claims.Subject)
		if err != nil {
			bad(w, "source connection is invalid")
			return
		}
		if _, err := a.integrations.Repository(r.Context(), connection, in.Repository); err != nil {
			bad(w, err.Error())
			return
		}
	}
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()
	existing, err := a.store.ApplicationServices(r.Context(), projectID)
	if err != nil {
		problem(w, err)
		return
	}
	if len(existing) >= 25 {
		bad(w, "a project can have at most 25 additional services")
		return
	}
	for _, item := range existing {
		if strings.EqualFold(item.Name, in.Name) {
			write(w, 409, map[string]string{"error": "a service with this name already exists"})
			return
		}
	}
	service := store.ApplicationService{ID: newID("svc"), ProjectID: projectID, Name: in.Name, SourceType: in.SourceType, ImageURL: in.ImageURL, RegistryID: in.RegistryID, ConnectionID: in.ConnectionID, Repository: in.Repository, Branch: in.Branch, DockerfilePath: in.DockerfilePath, BuildContext: in.BuildContext, BuildStrategy: in.BuildStrategy, ContainerPort: in.ContainerPort, Command: in.Command, HealthCheckType: in.HealthCheckType, HealthCheckPath: in.HealthCheckPath, HealthCheckCommand: in.HealthCheckCommand, HealthCheckTimeout: in.HealthCheckTimeoutSeconds, EnvironmentEncrypted: environmentEncrypted, Status: "created"}
	if err := a.store.CreateApplicationService(r.Context(), service); err != nil {
		problem(w, err)
		return
	}
	created, err := a.store.ApplicationService(r.Context(), service.ID)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"service": created})
}

func (a *API) updateApplicationService(w http.ResponseWriter, r *http.Request) {
	var in applicationServiceInput
	if !decode(w, r, &in) {
		return
	}
	clean, err := cleanApplicationServiceInput(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	if clean.RegistryID != "" {
		if _, err := a.store.Registry(r.Context(), clean.RegistryID, claims.Subject); err != nil {
			bad(w, "registry credential is invalid")
			return
		}
	}
	if clean.SourceType == "repository" {
		connection, err := a.store.SourceConnection(r.Context(), clean.ConnectionID, claims.Subject)
		if err != nil {
			bad(w, "source connection is invalid")
			return
		}
		if _, err := a.integrations.Repository(r.Context(), connection, clean.Repository); err != nil {
			bad(w, err.Error())
			return
		}
	}
	existing, err := a.store.ApplicationServices(r.Context(), service.ProjectID)
	if err != nil {
		problem(w, err)
		return
	}
	for _, item := range existing {
		if item.ID != service.ID && strings.EqualFold(item.Name, clean.Name) {
			write(w, 409, map[string]string{"error": "a service with this name already exists"})
			return
		}
	}
	service.Name = clean.Name
	service.SourceType = clean.SourceType
	service.ImageURL = clean.ImageURL
	service.RegistryID = clean.RegistryID
	service.ConnectionID = clean.ConnectionID
	service.Repository = clean.Repository
	service.Branch = clean.Branch
	service.DockerfilePath = clean.DockerfilePath
	service.BuildContext = clean.BuildContext
	service.BuildStrategy = clean.BuildStrategy
	service.ContainerPort = clean.ContainerPort
	service.Command = clean.Command
	service.HealthCheckType = clean.HealthCheckType
	service.HealthCheckPath = clean.HealthCheckPath
	service.HealthCheckCommand = clean.HealthCheckCommand
	service.HealthCheckTimeout = clean.HealthCheckTimeoutSeconds
	if err := a.store.UpdateApplicationService(r.Context(), service); err != nil {
		problem(w, err)
		return
	}
	updated, err := a.store.ApplicationService(r.Context(), service.ID)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"service": updated, "message": "Service configuration saved; deploy the service to apply it"})
}

type deploymentTriggerInput struct {
	AutoDeploy             bool   `json:"autoDeploy"`
	RegistryWebhookEnabled bool   `json:"registryWebhookEnabled"`
	RegistryWebhookTag     string `json:"registryWebhookTag"`
}

func validRegistryTag(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 128 {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || (index > 0 && (character == '.' || character == '-')) {
			continue
		}
		return false
	}
	return true
}

func (a *API) deploymentTriggerResponse(ctx context.Context, service store.ApplicationService) (map[string]any, error) {
	response := map[string]any{
		"serviceId":              service.ID,
		"sourceType":             service.SourceType,
		"autoDeploy":             service.AutoDeploy,
		"branch":                 service.Branch,
		"registryWebhookEnabled": service.RegistryWebhookSecret != "",
		"registryWebhookTag":     service.RegistryWebhookTag,
		"webhookUrl":             "",
		"webhookConfigured":      false,
	}
	if service.SourceType == "repository" {
		config, err := a.store.ProviderAppConfig(ctx, "github")
		if err != nil && !store.NotFound(err) {
			return nil, err
		}
		if err == nil && config.WebhookSecretEncrypted != "" {
			secret, decryptErr := a.box.Decrypt(config.WebhookSecretEncrypted)
			if decryptErr != nil {
				return nil, decryptErr
			}
			if secret != "" {
				response["webhookUrl"] = a.publicURL + "/api/webhooks/github"
				response["webhookConfigured"] = true
			}
		}
		return response, nil
	}
	if service.RegistryWebhookSecret == "" {
		return response, nil
	}
	secret, err := a.box.Decrypt(service.RegistryWebhookSecret)
	if err != nil {
		return nil, err
	}
	response["webhookUrl"] = a.publicURL + "/api/webhooks/registry/" + url.PathEscape(service.ID) + "/" + url.PathEscape(secret)
	response["webhookConfigured"] = true
	return response, nil
}

func (a *API) applicationServiceDeploymentTriggers(w http.ResponseWriter, r *http.Request) {
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	response, err := a.deploymentTriggerResponse(r.Context(), service)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, response)
}

func (a *API) updateApplicationServiceDeploymentTriggers(w http.ResponseWriter, r *http.Request) {
	var in deploymentTriggerInput
	if !decode(w, r, &in) {
		return
	}
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	registryTag := strings.TrimSpace(in.RegistryWebhookTag)
	if !validRegistryTag(registryTag) {
		bad(w, "registry tag must contain only letters, numbers, dots, dashes, or underscores")
		return
	}
	autoDeploy := in.AutoDeploy
	registrySecret := service.RegistryWebhookSecret
	if service.SourceType == "repository" {
		registrySecret, registryTag = "", ""
		if autoDeploy {
			current, responseErr := a.deploymentTriggerResponse(r.Context(), service)
			if responseErr != nil {
				problem(w, responseErr)
				return
			}
			if configured, _ := current["webhookConfigured"].(bool); !configured {
				bad(w, "reconnect the GitHub App in Sources before enabling automatic deployment")
				return
			}
		}
	} else {
		autoDeploy = in.RegistryWebhookEnabled
		if in.RegistryWebhookEnabled && registrySecret == "" {
			registrySecret, err = a.box.Encrypt(randomSecret())
			if err != nil {
				problem(w, err)
				return
			}
		}
		if !in.RegistryWebhookEnabled {
			registrySecret, registryTag = "", ""
		}
	}
	if err := a.store.UpdateApplicationServiceDeploymentTriggers(r.Context(), service.ID, autoDeploy, registrySecret, registryTag); err != nil {
		problem(w, err)
		return
	}
	updated, err := a.store.ApplicationService(r.Context(), service.ID)
	if err != nil {
		problem(w, err)
		return
	}
	response, err := a.deploymentTriggerResponse(r.Context(), updated)
	if err != nil {
		problem(w, err)
		return
	}
	response["message"] = "Deployment triggers saved"
	write(w, 200, response)
}

func (a *API) registryAuthForService(ctx context.Context, registryID, userID string) (*runtime.RegistryAuth, error) {
	if registryID == "" {
		return nil, nil
	}
	credential, err := a.store.Registry(ctx, registryID, userID)
	if err != nil {
		return nil, err
	}
	password := ""
	if credential.PasswordEncrypted != "" {
		password, err = a.box.Decrypt(credential.PasswordEncrypted)
		if err != nil {
			return nil, err
		}
	}
	return &runtime.RegistryAuth{Username: credential.Username, Password: password, ServerAddress: credential.RegistryURL}, nil
}

func (a *API) deployApplicationService(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	service, deployment, err := a.startApplicationServiceDeployment(r.Context(), strings.TrimSpace(r.PathValue("id")), claims.Subject, "", "")
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "already deploying") {
			write(w, 409, map[string]string{"error": err.Error()})
		} else {
			bad(w, err.Error())
		}
		return
	}
	write(w, http.StatusAccepted, map[string]any{"service": service, "deployment": deployment})
}

func (a *API) startApplicationServiceDeployment(ctx context.Context, serviceID, userID, message, commitOverride string) (store.ApplicationService, store.Deployment, error) {
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()
	service, err := a.store.ApplicationService(ctx, serviceID)
	if err != nil {
		return store.ApplicationService{}, store.Deployment{}, err
	}
	if service.Status == "deploying" {
		return service, store.Deployment{}, errors.New("service is already deploying")
	}
	environmentValue, err := a.box.Decrypt(service.EnvironmentEncrypted)
	if err != nil {
		return service, store.Deployment{}, err
	}
	service.Environment = []string{}
	if environmentValue != "" {
		service.Environment = strings.Split(environmentValue, "\n")
	}
	registryAuth, err := a.registryAuthForService(ctx, service.RegistryID, userID)
	if err != nil {
		return service, store.Deployment{}, errors.New("registry credential is unavailable")
	}
	var sourceConnection *store.SourceConnection
	var repository *integration.Repository
	commit := service.ImageURL
	if service.SourceType == "repository" {
		connection, err := a.store.SourceConnection(ctx, service.ConnectionID, userID)
		if err != nil {
			return service, store.Deployment{}, errors.New("source connection is unavailable")
		}
		selected, err := a.integrations.Repository(ctx, connection, service.Repository)
		if err != nil {
			return service, store.Deployment{}, err
		}
		sourceConnection, repository = &connection, &selected
		commit = service.Repository + "@" + service.Branch
	}
	if commitOverride != "" {
		commit = commitOverride
	}
	if message == "" {
		message = "Deploy " + service.Name
	}
	deployment := store.Deployment{ID: newID("dep"), ProjectID: service.ProjectID, ServiceID: service.ID, ServiceName: service.Name, Commit: commit, Message: message, Status: "deploying"}
	if err := a.store.CreateDeployment(ctx, deployment); err != nil {
		return service, store.Deployment{}, err
	}
	if err := a.store.UpdateApplicationServiceStatus(ctx, service.ID, "deploying", ""); err != nil {
		_ = a.store.FinishDeployment(ctx, deployment.ID, "failed", "Could not start deployment", 0)
		return service, deployment, err
	}
	service.Status = "deploying"
	a.recordDeploymentEvent(ctx, deployment.ID, "prepare", "complete", "Deployment accepted for service "+service.Name)
	deploymentCtx, cancelDeployment := context.WithCancel(context.Background())
	a.registerDeployment(deployment.ID, cancelDeployment)
	go a.runApplicationServiceDeployment(deploymentCtx, deployment, service, registryAuth, sourceConnection, repository, time.Now())
	return service, deployment, nil
}

func webhookBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
}

func (a *API) githubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := webhookBody(w, r)
	if err != nil {
		bad(w, "webhook body is too large")
		return
	}
	appConfig, err := a.store.ProviderAppConfig(r.Context(), "github")
	if err != nil {
		write(w, http.StatusServiceUnavailable, map[string]string{"error": "GitHub App webhook verification is not configured"})
		return
	}
	secret, err := a.box.Decrypt(appConfig.WebhookSecretEncrypted)
	if err != nil || secret == "" {
		write(w, http.StatusServiceUnavailable, map[string]string{"error": "GitHub App webhook secret is unavailable"})
		return
	}
	signature := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")), "sha256=")
	provided, err := hex.DecodeString(signature)
	if err != nil {
		write(w, http.StatusUnauthorized, map[string]string{"error": "invalid GitHub webhook signature"})
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		write(w, http.StatusUnauthorized, map[string]string{"error": "invalid GitHub webhook signature"})
		return
	}
	event := strings.ToLower(strings.TrimSpace(r.Header.Get("X-GitHub-Event")))
	if event == "ping" {
		write(w, 200, map[string]any{"ok": true, "message": "GitHub webhook connected"})
		return
	}
	if event != "push" {
		write(w, http.StatusAccepted, map[string]any{"triggered": 0, "ignored": "event " + event + " is not a push"})
		return
	}
	deliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	if deliveryID == "" {
		digest := sha256.Sum256(body)
		deliveryID = hex.EncodeToString(digest[:])
	}
	claimed, err := a.store.ClaimWebhookDelivery(r.Context(), "github", deliveryID)
	if err != nil {
		problem(w, err)
		return
	}
	if !claimed {
		write(w, http.StatusAccepted, map[string]any{"triggered": 0, "duplicate": true})
		return
	}
	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Deleted    bool   `json:"deleted"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Pusher struct {
			Name string `json:"name"`
		} `json:"pusher"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		bad(w, "invalid GitHub push payload")
		return
	}
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if payload.Deleted || branch == payload.Ref || payload.Repository.FullName == "" {
		write(w, http.StatusAccepted, map[string]any{"triggered": 0, "ignored": "push does not update a deployable branch"})
		return
	}
	services, err := a.store.AutoDeployRepositoryServices(r.Context(), payload.Repository.FullName, branch)
	if err != nil {
		problem(w, err)
		return
	}
	owner, err := a.store.OwnerUser(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	deploymentIDs := []string{}
	for _, service := range services {
		message := "Auto-deploy " + service.Name + " from GitHub push"
		if payload.Pusher.Name != "" {
			message += " by " + payload.Pusher.Name
		}
		_, deployment, startErr := a.startApplicationServiceDeployment(r.Context(), service.ID, owner.ID, message, payload.After)
		if startErr != nil {
			a.log.Warn("start GitHub auto-deploy", "service", service.ID, "error", startErr)
			continue
		}
		deploymentIDs = append(deploymentIDs, deployment.ID)
	}
	write(w, http.StatusAccepted, map[string]any{"triggered": len(deploymentIDs), "deployments": deploymentIDs, "repository": payload.Repository.FullName, "branch": branch})
}

func collectRegistryWebhookTags(value any, tags map[string]bool) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			lowerKey := strings.ToLower(key)
			if text, ok := child.(string); ok && (lowerKey == "tag" || lowerKey == "tag_name" || lowerKey == "ref") {
				text = strings.TrimPrefix(strings.TrimSpace(text), "refs/tags/")
				if text != "" {
					tags[text] = true
				}
			}
			collectRegistryWebhookTags(child, tags)
		}
	case []any:
		for _, child := range current {
			collectRegistryWebhookTags(child, tags)
		}
	}
}

func (a *API) registryWebhook(w http.ResponseWriter, r *http.Request) {
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "webhook not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if service.SourceType != "image" || !service.AutoDeploy || service.RegistryWebhookSecret == "" {
		write(w, 404, map[string]string{"error": "webhook not found"})
		return
	}
	secret, err := a.box.Decrypt(service.RegistryWebhookSecret)
	if err != nil || !hmac.Equal([]byte(secret), []byte(r.PathValue("token"))) {
		write(w, 404, map[string]string{"error": "webhook not found"})
		return
	}
	body, err := webhookBody(w, r)
	if err != nil {
		bad(w, "webhook body is too large")
		return
	}
	deliveryID := ""
	for _, header := range []string{"X-Registry-Event-ID", "X-GitHub-Delivery", "X-Gitlab-Event-UUID", "X-Request-Id"} {
		if deliveryID = strings.TrimSpace(r.Header.Get(header)); deliveryID != "" {
			break
		}
	}
	if deliveryID == "" {
		digest := sha256.Sum256(body)
		deliveryID = hex.EncodeToString(digest[:])
	}
	claimed, err := a.store.ClaimWebhookDelivery(r.Context(), "registry:"+service.ID, deliveryID)
	if err != nil {
		problem(w, err)
		return
	}
	if !claimed {
		write(w, http.StatusAccepted, map[string]any{"triggered": false, "duplicate": true})
		return
	}
	if service.RegistryWebhookTag != "" {
		var payload any
		if err := json.Unmarshal(body, &payload); err != nil {
			bad(w, "invalid registry webhook payload")
			return
		}
		tags := map[string]bool{}
		collectRegistryWebhookTags(payload, tags)
		if !tags[service.RegistryWebhookTag] {
			write(w, http.StatusAccepted, map[string]any{"triggered": false, "ignored": "pushed tag does not match " + service.RegistryWebhookTag})
			return
		}
	}
	owner, err := a.store.OwnerUser(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	_, deployment, err := a.startApplicationServiceDeployment(r.Context(), service.ID, owner.ID, "Auto-deploy "+service.Name+" from registry webhook", service.ImageURL)
	if err != nil {
		if strings.Contains(err.Error(), "already deploying") {
			write(w, http.StatusAccepted, map[string]any{"triggered": false, "ignored": err.Error()})
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusAccepted, map[string]any{"triggered": true, "deployment": deployment.ID})
}

func (a *API) runApplicationServiceDeployment(deploymentCtx context.Context, deployment store.Deployment, service store.ApplicationService, registryAuth *runtime.RegistryAuth, sourceConnection *store.SourceConnection, repository *integration.Repository, started time.Time) {
	ctx, cancel := context.WithTimeout(deploymentCtx, 15*time.Minute)
	defer cancel()
	defer a.unregisterDeployment(deployment.ID)
	activeStage := "prepare"
	progress := func(stage, eventType, message string) {
		if eventType == "start" {
			activeStage = stage
			if stage == "promote" {
				// Promotion changes the stable release. Once it begins, let it finish
				// atomically instead of cancelling between container renames.
				a.unregisterDeployment(deployment.ID)
			}
		}
		a.recordDeploymentEvent(ctx, deployment.ID, stage, eventType, message)
	}
	var runtimeService runtime.Service
	var err error
	healthCheck := runtime.ApplicationHealthCheck{Type: service.HealthCheckType, Path: service.HealthCheckPath, Command: service.HealthCheckCommand, Timeout: time.Duration(service.HealthCheckTimeout) * time.Second}
	if service.SourceType == "repository" {
		workspace, workspaceErr := os.MkdirTemp("", "selfhost-build-"+service.ID+"-")
		if workspaceErr != nil {
			err = workspaceErr
		} else {
			defer os.RemoveAll(workspace)
			progress("clone", "start", "Preparing "+service.Repository+" at "+service.Branch)
			err = a.integrations.CloneRepository(ctx, *sourceConnection, *repository, service.Branch, workspace, func(message string) { progress("clone", "log", message) })
			if err == nil {
				progress("clone", "complete", "Repository checkout is ready")
				var image string
				image, err = a.docker.BuildApplicationImage(ctx, service.ID, workspace, service.BuildStrategy, service.DockerfilePath, service.BuildContext, progress)
				if err == nil {
					runtimeService, err = a.docker.DeployApplicationBuiltImage(ctx, service.ID, service.ProjectID, service.Name, image, service.ContainerPort, service.Environment, service.Command, healthCheck, progress)
				}
			}
		}
	} else {
		runtimeService, err = a.docker.DeployApplicationImage(ctx, service.ID, service.ProjectID, service.Name, service.ImageURL, service.ContainerPort, service.Environment, service.Command, healthCheck, registryAuth, progress)
	}
	duration := int(time.Since(started).Round(time.Second).Seconds())
	if err != nil {
		fallback, fallbackErr := a.docker.ApplicationService(context.Background(), service.ID, service.Name)
		if deploymentCancelled(ctx, err) {
			if fallbackErr == nil {
				_ = a.store.UpdateApplicationServiceStatus(context.Background(), service.ID, fallback.Status, "")
				a.recordDeploymentEvent(context.Background(), deployment.ID, "rollback", "complete", "Candidate discarded; previous release remains unchanged")
			} else {
				_ = a.store.UpdateApplicationServiceStatus(context.Background(), service.ID, "created", "")
			}
			a.recordDeploymentEvent(context.Background(), deployment.ID, activeStage, "cancelled", "Deployment stopped by user")
			_ = a.store.FinishDeployment(context.Background(), deployment.ID, "cancelled", "Deployment stopped for "+service.Name, duration)
			return
		}
		if fallbackErr == nil && fallback.Status == "healthy" {
			_ = a.store.UpdateApplicationServiceStatus(context.Background(), service.ID, fallback.Status, err.Error())
			_ = a.store.UpdateProjectStatus(context.Background(), service.ProjectID, fallback.Status)
			a.recordDeploymentEvent(context.Background(), deployment.ID, "rollback", "complete", "Candidate discarded; previous healthy release remains online")
		} else {
			_ = a.store.UpdateApplicationServiceStatus(context.Background(), service.ID, "failed", err.Error())
			_ = a.store.UpdateProjectStatus(context.Background(), service.ProjectID, "degraded")
		}
		a.recordDeploymentEvent(context.Background(), deployment.ID, activeStage, "error", err.Error())
		_ = a.store.FinishDeployment(context.Background(), deployment.ID, "failed", "Deployment failed for "+service.Name, duration)
		a.log.Error("deploy application service", "service", service.ID, "error", err)
		a.queueDeploymentNotification(deployment.ID, service.ProjectID, service.Name, "failed", err.Error())
		return
	}
	_ = a.store.UpdateApplicationServiceStatus(context.Background(), service.ID, runtimeService.Status, "")
	_ = a.store.UpdateProjectStatus(context.Background(), service.ProjectID, runtimeService.Status)
	_ = a.store.FinishDeployment(context.Background(), deployment.ID, runtimeService.Status, "Deployed "+service.Name, duration)
	a.recordDeploymentEvent(context.Background(), deployment.ID, "complete", "complete", "Deployment finished successfully on "+runtimeService.Container)
	a.queueDeploymentNotification(deployment.ID, service.ProjectID, service.Name, runtimeService.Status, "Deployment finished successfully on "+runtimeService.Container)
}

func (a *API) applicationServiceLogs(w http.ResponseWriter, r *http.Request) {
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	tail := 300
	if requested := strings.TrimSpace(r.URL.Query().Get("lines")); requested != "" {
		parsed, parseErr := strconv.Atoi(requested)
		if parseErr != nil || parsed < 1 || parsed > 1000 {
			bad(w, "lines must be a number between 1 and 1000")
			return
		}
		tail = parsed
	}
	lines, err := a.docker.ApplicationLogs(r.Context(), service.ID, tail)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 404, map[string]string{"error": "deploy this service before viewing logs"})
		return
	}
	if err != nil {
		write(w, 502, map[string]string{"error": "could not read service logs"})
		return
	}
	write(w, 200, map[string]any{"lines": lines, "count": len(lines), "limit": tail, "container": "selfhost-svc-" + service.ID})
}

func (a *API) stopApplicationService(w http.ResponseWriter, r *http.Request) {
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()

	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if service.Status == "deploying" {
		write(w, 409, map[string]string{"error": "wait for the current deployment to finish before stopping this service"})
		return
	}
	runtimeService, err := a.docker.ApplicationService(r.Context(), service.ID, service.Name)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this service before stopping it"})
		return
	} else if err != nil {
		a.log.Warn("inspect application service before stop", "service", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not inspect the service container"})
		return
	}
	if runtimeService.Status != "stopped" {
		if err := a.docker.StopApplication(r.Context(), service.ID); err != nil {
			a.log.Warn("stop application service", "service", service.ID, "error", err)
			write(w, 502, map[string]string{"error": "could not stop the service container"})
			return
		}
	}
	_ = a.store.UpdateApplicationServiceStatus(r.Context(), service.ID, "stopped", "")
	service.Status = "stopped"
	service.Container = runtimeService.Container
	write(w, 200, map[string]any{"service": service, "message": service.Name + " stopped"})
}

func (a *API) restartApplicationService(w http.ResponseWriter, r *http.Request) {
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()

	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if service.Status == "deploying" {
		write(w, 409, map[string]string{"error": "wait for the current deployment to finish before restarting this service"})
		return
	}
	if err := a.docker.RestartApplication(r.Context(), service.ID); errors.Is(err, runtime.ErrNotFound) {
		write(w, 409, map[string]string{"error": "deploy this service before restarting it"})
		return
	} else if err != nil {
		a.log.Warn("restart application service", "service", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "could not restart the service container"})
		return
	}
	runtimeService, err := a.docker.ApplicationService(r.Context(), service.ID, service.Name)
	if err != nil {
		a.log.Warn("inspect restarted application service", "service", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "service restarted, but its runtime status is unavailable"})
		return
	}
	_ = a.store.UpdateApplicationServiceStatus(r.Context(), service.ID, runtimeService.Status, "")
	service.Status = runtimeService.Status
	service.Container = runtimeService.Container
	write(w, 200, map[string]any{"service": service, "message": service.Name + " restarted"})
}

type applicationCommandInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"workingDir"`
}

func cleanApplicationCommandInput(input applicationCommandInput) (applicationCommandInput, error) {
	input.Command = strings.TrimSpace(input.Command)
	input.WorkingDir = strings.TrimSpace(input.WorkingDir)
	if input.Command == "" {
		return applicationCommandInput{}, errors.New("command is required")
	}
	if len(input.Command) > 4096 || strings.ContainsRune(input.Command, '\x00') {
		return applicationCommandInput{}, errors.New("command must be at most 4096 characters and cannot contain null bytes")
	}
	if input.WorkingDir != "" {
		if len(input.WorkingDir) > 512 || !strings.HasPrefix(input.WorkingDir, "/") || strings.ContainsAny(input.WorkingDir, "\x00\r\n") {
			return applicationCommandInput{}, errors.New("working directory must be an absolute container path")
		}
	}
	return input, nil
}

func canExecuteContainerCommands(role string) bool {
	return role == "owner" || role == "admin" || role == "developer"
}

func (a *API) executeApplicationServiceCommand(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok || !canExecuteContainerCommands(claims.Role) {
		write(w, http.StatusForbidden, map[string]string{"error": "your role cannot execute commands in containers"})
		return
	}
	var input applicationCommandInput
	if !decode(w, r, &input) {
		return
	}
	clean, err := cleanApplicationCommandInput(input)
	if err != nil {
		bad(w, err.Error())
		return
	}
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	runtimeService, err := a.docker.ApplicationService(r.Context(), service.ID, service.Name)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, http.StatusConflict, map[string]string{"error": "deploy this service before opening its terminal"})
		return
	}
	if err != nil {
		a.log.Warn("inspect application service before command", "service", service.ID, "error", err)
		write(w, http.StatusBadGateway, map[string]string{"error": "could not inspect the service container"})
		return
	}
	if runtimeService.Status == "stopped" {
		write(w, http.StatusConflict, map[string]string{"error": "start this service before executing a command"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := a.docker.ExecuteApplicationCommand(ctx, service.ID, clean.Command, clean.WorkingDir)
	if err != nil {
		a.log.Warn("execute application service command", "service", service.ID, "user", claims.Subject, "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			write(w, http.StatusGatewayTimeout, map[string]string{"error": "command response exceeded 30 seconds; the process may still be running in the container"})
			return
		}
		if errors.Is(err, runtime.ErrNotFound) {
			write(w, http.StatusConflict, map[string]string{"error": "the service container is no longer available"})
			return
		}
		write(w, http.StatusBadGateway, map[string]string{"error": "could not execute the container command: " + err.Error()})
		return
	}
	a.log.Info("application service command executed", "service", service.ID, "user", claims.Subject, "exit_code", result.ExitCode, "duration_ms", result.DurationMS)
	write(w, http.StatusOK, map[string]any{"result": result})
}

func (a *API) applicationEnvironmentVariables(service store.ApplicationService) ([]environmentVariableInput, []string, error) {
	value := ""
	var err error
	if service.EnvironmentEncrypted != "" {
		value, err = a.box.Decrypt(service.EnvironmentEncrypted)
		if err != nil {
			return nil, nil, err
		}
	}
	secretKeys := map[string]bool{}
	for _, key := range service.EnvironmentSecretKeys {
		secretKeys[key] = true
	}
	variables := []environmentVariableInput{}
	keys := []string{}
	if value == "" {
		return variables, keys, nil
	}
	for _, line := range strings.Split(value, "\n") {
		key, variableValue, found := strings.Cut(line, "=")
		if !found || key == "" {
			continue
		}
		variables = append(variables, environmentVariableInput{Key: key, Value: variableValue, Secret: secretKeys[key]})
		keys = append(keys, key)
	}
	return variables, keys, nil
}

func (a *API) applicationServiceEnvironment(w http.ResponseWriter, r *http.Request) {
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	variables, _, err := a.applicationEnvironmentVariables(service)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"variables": variables, "service": service})
}

func cleanServiceEnvironment(variables []environmentVariableInput) ([]environmentVariableInput, []string, []string, error) {
	seen := map[string]bool{}
	clean := make([]environmentVariableInput, 0, len(variables))
	runtimeEnvironment := make([]string, 0, len(variables))
	secretKeys := []string{}
	for _, variable := range variables {
		variable.Key = strings.TrimSpace(variable.Key)
		if !environmentKey(variable.Key) {
			return nil, nil, nil, fmt.Errorf("environment variable keys must use letters, numbers, and underscores and cannot start with a number")
		}
		if seen[variable.Key] {
			return nil, nil, nil, fmt.Errorf("environment variable keys must be unique")
		}
		if len(variable.Value) > 16<<10 || strings.ContainsRune(variable.Value, '\x00') {
			return nil, nil, nil, fmt.Errorf("environment variable values must be at most 16 KB and cannot contain null characters")
		}
		seen[variable.Key] = true
		clean = append(clean, variable)
		runtimeEnvironment = append(runtimeEnvironment, variable.Key+"="+variable.Value)
		if variable.Secret {
			secretKeys = append(secretKeys, variable.Key)
		}
	}
	return clean, runtimeEnvironment, secretKeys, nil
}

func (a *API) updateApplicationServiceEnvironment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Variables []environmentVariableInput `json:"variables"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	clean, runtimeEnvironment, secretKeys, err := cleanServiceEnvironment(in.Variables)
	if err != nil {
		bad(w, err.Error())
		return
	}
	_, previousKeys, err := a.applicationEnvironmentVariables(service)
	if err != nil {
		problem(w, err)
		return
	}
	encrypted, err := a.box.Encrypt(strings.Join(runtimeEnvironment, "\n"))
	if err != nil {
		problem(w, err)
		return
	}
	if err := a.store.UpdateApplicationServiceEnvironment(r.Context(), service.ID, encrypted, secretKeys); err != nil {
		problem(w, err)
		return
	}
	runtimeService, err := a.docker.RestartApplicationWithEnvironment(r.Context(), service.ID, service.Name, runtimeEnvironment, previousKeys)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 200, map[string]any{"variables": clean, "service": service, "restarted": false, "message": "Environment saved and will be applied on the first deployment"})
		return
	}
	if err != nil {
		_ = a.store.UpdateApplicationServiceEnvironment(context.Background(), service.ID, service.EnvironmentEncrypted, service.EnvironmentSecretKeys)
		a.log.Warn("restart application service with environment", "service", service.ID, "error", err)
		write(w, 502, map[string]string{"error": "environment variables were not applied; the previous container was restored"})
		return
	}
	_ = a.store.UpdateApplicationServiceStatus(r.Context(), service.ID, runtimeService.Status, "")
	write(w, 200, map[string]any{"variables": clean, "service": runtimeService, "restarted": true, "message": "Environment saved and service restarted without rebuilding"})
}

func (a *API) deleteApplicationService(w http.ResponseWriter, r *http.Request) {
	a.applicationMu.Lock()
	defer a.applicationMu.Unlock()
	service, err := a.store.ApplicationService(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "application service not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	routeCount, err := a.store.ApplicationServiceRouteCount(r.Context(), service.ID)
	if err != nil {
		problem(w, err)
		return
	}
	if routeCount > 0 {
		write(w, 409, map[string]string{"error": "remove this service from domain routing before deleting it"})
		return
	}
	if err := a.docker.RemoveApplication(r.Context(), service.ID); err != nil {
		write(w, 502, map[string]string{"error": "could not remove the service container"})
		return
	}
	if err := a.store.DeleteApplicationService(r.Context(), service.ID); err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *API) projectLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), id); store.NotFound(err) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	tail := 300
	if requested := strings.TrimSpace(r.URL.Query().Get("lines")); requested != "" {
		parsed, err := strconv.Atoi(requested)
		if err != nil || parsed < 1 || parsed > 1000 {
			write(w, 400, map[string]string{"error": "lines must be a number between 1 and 1000"})
			return
		}
		tail = parsed
	}
	lines, err := a.docker.ProjectLogs(r.Context(), id, tail)
	if errors.Is(err, runtime.ErrNotFound) {
		write(w, 404, map[string]string{"error": "deploy the project before viewing container logs"})
		return
	}
	if err != nil {
		a.log.Warn("read project logs", "project", id, "error", err)
		write(w, 502, map[string]string{"error": "could not read container logs"})
		return
	}
	write(w, 200, map[string]any{"lines": lines, "count": len(lines), "limit": tail, "container": "selfhost-" + id})
}

func (a *API) recordDeploymentEvent(ctx context.Context, deploymentID, stage, eventType, message string) {
	if err := a.store.AppendDeploymentEvent(ctx, store.DeploymentEvent{DeploymentID: deploymentID, Stage: stage, Type: eventType, Message: message}); err != nil {
		a.log.Warn("record deployment event", "deployment", deploymentID, "stage", stage, "error", err)
	}
}

func (a *API) failDeployment(ctx context.Context, deploymentID, projectID string, started time.Time, message string, cause error) {
	duration := int(time.Since(started).Round(time.Second).Seconds())
	_ = a.store.FinishDeployment(ctx, deploymentID, "failed", message, duration)
	_ = a.store.UpdateProjectStatus(ctx, projectID, "degraded")
	a.log.Error("deploy project", "project", projectID, "deployment", deploymentID, "error", cause)
	a.queueDeploymentNotification(deploymentID, projectID, "application", "failed", cause.Error())
}

func (a *API) queueDeploymentNotification(deploymentID, projectID, serviceName, status, detail string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		settings, config, err := a.smtpMailerConfig(ctx)
		if err != nil || !settings.Enabled || !smtpConfigured(settings) {
			return
		}
		failed := status == "failed" || status == "degraded"
		if failed && !settings.NotifyDeploymentFailures || !failed && !settings.NotifyDeploymentSuccesses {
			return
		}
		owner, err := a.store.OwnerUser(ctx)
		if err != nil {
			return
		}
		project, err := a.store.Project(ctx, projectID)
		if err != nil {
			return
		}
		label := "succeeded"
		color := "#087a51"
		if failed {
			label = "failed"
			color = "#c53b36"
		}
		deploymentURL := a.publicURL + "/deployments/" + url.PathEscape(deploymentID)
		message := mailer.Message{
			To: owner.Email, Subject: "Deployment " + label + ": " + project.Name + " / " + serviceName,
			Text: "Deployment " + label + " for " + project.Name + " / " + serviceName + ".\n\n" + detail + "\n\nView deployment: " + deploymentURL,
			HTML: `<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto;padding:32px"><p style="color:` + color + `;font-weight:700">DEPLOYMENT ` + strings.ToUpper(label) + `</p><h1 style="font-size:24px">` + html.EscapeString(project.Name) + ` / ` + html.EscapeString(serviceName) + `</h1><p>` + html.EscapeString(detail) + `</p><p style="margin:28px 0"><a href="` + html.EscapeString(deploymentURL) + `" style="background:#111827;color:white;text-decoration:none;padding:12px 18px;border-radius:7px;font-weight:700">View deployment</a></p></div>`,
		}
		if err := mailer.Send(ctx, config, message); err != nil {
			a.log.Warn("send deployment notification", "deployment", deploymentID, "error", err)
		}
	}()
}

func (a *API) registerDeployment(deploymentID string, cancel context.CancelFunc) {
	a.deploymentMu.Lock()
	defer a.deploymentMu.Unlock()
	if a.deploymentCancels == nil {
		a.deploymentCancels = make(map[string]context.CancelFunc)
	}
	a.deploymentCancels[deploymentID] = cancel
}

func (a *API) unregisterDeployment(deploymentID string) {
	a.deploymentMu.Lock()
	defer a.deploymentMu.Unlock()
	delete(a.deploymentCancels, deploymentID)
}

func (a *API) claimDeploymentCancellation(deploymentID string) (context.CancelFunc, bool) {
	a.deploymentMu.Lock()
	defer a.deploymentMu.Unlock()
	cancel, ok := a.deploymentCancels[deploymentID]
	if ok {
		delete(a.deploymentCancels, deploymentID)
	}
	return cancel, ok
}

func deploymentCancelled(ctx context.Context, err error) bool {
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled)
}

func (a *API) deployments(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Deployments(r.Context(), "")
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, items)
}
func (a *API) deployment(w http.ResponseWriter, r *http.Request) {
	d, err := a.store.Deployment(r.Context(), r.PathValue("id"))
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "deployment not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	p, err := a.store.Project(r.Context(), d.ProjectID)
	if err != nil {
		problem(w, err)
		return
	}
	events, err := a.store.DeploymentEvents(r.Context(), d.ID)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, map[string]any{"deployment": d, "project": p, "events": events})
}

func (a *API) cancelDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID := strings.TrimSpace(r.PathValue("id"))
	deployment, err := a.store.Deployment(r.Context(), deploymentID)
	if store.NotFound(err) {
		write(w, 404, map[string]string{"error": "deployment not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if deployment.Status == "cancelled" {
		write(w, 200, map[string]any{"deployment": deployment, "message": "Deployment is already stopped"})
		return
	}
	if deployment.Status != "deploying" && deployment.Status != "building" {
		write(w, 409, map[string]string{"error": "deployment is no longer running"})
		return
	}
	cancel, ok := a.claimDeploymentCancellation(deployment.ID)
	if !ok {
		write(w, 409, map[string]string{"error": "deployment is no longer cancellable"})
		return
	}
	a.recordDeploymentEvent(r.Context(), deployment.ID, "cancel", "log", "Stop requested by user")
	cancel()
	write(w, http.StatusAccepted, map[string]any{"deployment": deployment, "message": "Stopping deployment"})
}

func newID(prefix string) string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		bad(w, "invalid JSON request")
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, message string) { write(w, 400, map[string]string{"error": message}) }
func problem(w http.ResponseWriter, err error) {
	write(w, 500, map[string]string{"error": "internal server error"})
}
func withJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
