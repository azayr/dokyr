package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/caddy"
	"github.com/azayr/selfhost/internal/config"
	"github.com/azayr/selfhost/internal/registry"
	"github.com/azayr/selfhost/internal/store"
)

type registrySettingsInput struct {
	Storage          string `json:"storage"`
	S3Region         string `json:"s3Region"`
	S3Bucket         string `json:"s3Bucket"`
	S3AccessKey      string `json:"s3AccessKey"`
	S3SecretKey      string `json:"s3SecretKey"`
	S3Endpoint       string `json:"s3Endpoint"`
	S3ForcePathStyle bool   `json:"s3ForcePathStyle"`
	S3Secure         bool   `json:"s3Secure"`

	// Response metadata is accepted and ignored so a client that round-trips a
	// GET response can still save settings. New clients send only editable
	// fields.
	HasS3SecretKey bool      `json:"hasS3SecretKey"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func defaultRegistrySettings() store.RegistrySettings {
	return store.RegistrySettings{Storage: "filesystem", S3Secure: true}
}

func registrySettingsResponse(settings store.RegistrySettings) map[string]any {
	return map[string]any{
		"storage":          settings.Storage,
		"s3Region":         settings.S3Region,
		"s3Bucket":         settings.S3Bucket,
		"s3AccessKey":      settings.S3AccessKey,
		"hasS3SecretKey":   settings.S3SecretKeyEncrypted != "",
		"s3Endpoint":       settings.S3Endpoint,
		"s3ForcePathStyle": settings.S3ForcePathStyle,
		"s3Secure":         settings.S3Secure,
		"updatedAt":        settings.UpdatedAt,
	}
}

func cleanRegistrySettings(in registrySettingsInput) (registrySettingsInput, error) {
	in.Storage = strings.ToLower(strings.TrimSpace(in.Storage))
	in.S3Region = strings.TrimSpace(in.S3Region)
	in.S3Bucket = strings.TrimSpace(in.S3Bucket)
	in.S3AccessKey = strings.TrimSpace(in.S3AccessKey)
	in.S3Endpoint = strings.TrimRight(strings.TrimSpace(in.S3Endpoint), "/")
	if in.Storage != "filesystem" && in.Storage != "s3" {
		return in, errors.New("choose filesystem or S3 storage")
	}
	if in.Storage != "s3" {
		return in, nil
	}
	if in.S3Bucket == "" || len(in.S3Bucket) > 63 || strings.ContainsAny(in.S3Bucket, " /\t\r\n") {
		return in, errors.New("enter an S3 bucket name")
	}
	if in.S3Region == "" || len(in.S3Region) > 100 || strings.ContainsAny(in.S3Region, " /\t\r\n") {
		return in, errors.New("enter an S3 region")
	}
	if in.S3AccessKey == "" || len(in.S3AccessKey) > 500 || strings.ContainsAny(in.S3AccessKey, "\r\n") {
		return in, errors.New("enter an S3 access key")
	}
	if in.S3Endpoint != "" && !strings.HasPrefix(in.S3Endpoint, "http://") && !strings.HasPrefix(in.S3Endpoint, "https://") {
		return in, errors.New("the S3 endpoint must start with http:// or https://")
	}
	if len(in.S3Endpoint) > 500 {
		return in, errors.New("the S3 endpoint is too long")
	}
	return in, nil
}

// BootstrapRegistrySettings imports an optional registry storage configuration
// from the Compose environment once. Like SMTP, PostgreSQL is the source of
// truth afterward and restarts never overwrite UI changes.
func (a *API) BootstrapRegistrySettings(ctx context.Context, bootstrap config.RegistryBootstrap) (bool, error) {
	if !bootstrap.Present {
		return false, nil
	}
	if _, err := a.store.RegistrySettings(ctx); err == nil {
		return false, nil
	} else if !store.NotFound(err) {
		return false, err
	}
	clean, err := cleanRegistrySettings(registrySettingsInput{
		Storage: bootstrap.Storage, S3Region: bootstrap.S3Region, S3Bucket: bootstrap.S3Bucket,
		S3AccessKey: bootstrap.S3AccessKey, S3SecretKey: bootstrap.S3SecretKey, S3Endpoint: bootstrap.S3Endpoint,
		S3ForcePathStyle: bootstrap.S3ForcePathStyle, S3Secure: bootstrap.S3Secure,
	})
	if err != nil {
		return false, err
	}
	secretEncrypted := ""
	if clean.S3SecretKey != "" {
		secretEncrypted, err = a.box.Encrypt(clean.S3SecretKey)
		if err != nil {
			return false, err
		}
	}
	return a.store.CreateRegistrySettingsIfMissing(ctx, store.RegistrySettings{
		Storage: clean.Storage, S3Region: clean.S3Region, S3Bucket: clean.S3Bucket, S3AccessKey: clean.S3AccessKey,
		S3SecretKeyEncrypted: secretEncrypted, S3Endpoint: clean.S3Endpoint,
		S3ForcePathStyle: clean.S3ForcePathStyle, S3Secure: clean.S3Secure,
	})
}

func (a *API) registryStatus(w http.ResponseWriter, r *http.Request) {
	status := a.registry.Status(r.Context())
	write(w, 200, status)
}

func (a *API) registryToken(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Dokyr Registry"`)
		write(w, http.StatusUnauthorized, map[string]string{"error": "registry login required"})
		return
	}
	var user store.User
	permission := ""
	if strings.TrimSpace(username) == "dokyr-internal" && a.registryInternalSecret != "" && hmac.Equal([]byte(password), []byte(a.registryInternalSecret)) {
		user = store.User{Email: "dokyr-internal", Role: "owner"}
		permission = "read_only"
	} else {
		passwordHash := sha256.Sum256([]byte(password))
		accessUser, accessToken, err := a.store.UserByRegistryAccessToken(
			r.Context(),
			strings.TrimSpace(username),
			hex.EncodeToString(passwordHash[:]),
		)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dokyr Registry"`)
			write(w, http.StatusUnauthorized, map[string]string{"error": "invalid registry credentials"})
			return
		}
		user = accessUser
		permission = accessToken.Permission
	}
	token, expiresIn, issuedAt, err := a.registryTokens.Issue(user, registry.TokenRequest{
		Service:    r.URL.Query().Get("service"),
		Scopes:     r.URL.Query()["scope"],
		Permission: permission,
	})
	if err != nil {
		if errors.Is(err, registry.ErrInvalidService) {
			bad(w, "invalid registry token service")
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"token":        token,
		"access_token": token,
		"expires_in":   expiresIn,
		"issued_at":    issuedAt.Format(time.RFC3339),
	})
}

func (a *API) registryAccessTokens(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	tokens, err := a.store.RegistryAccessTokens(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"tokens":        tokens,
		"username":      claims.Email,
		"registryHosts": a.effectiveRegistryHosts(r.Context()),
	})
}

func (a *API) createRegistryAccessToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       string `json:"name"`
		Permission string `json:"permission"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Permission = strings.TrimSpace(in.Permission)
	if in.Name == "" || len(in.Name) > 80 || strings.ContainsAny(in.Name, "\r\n") {
		bad(w, "enter a token name of at most 80 characters")
		return
	}
	if in.Permission != "read_only" && in.Permission != "read_write" {
		bad(w, "choose read-only or read-write access")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	if in.Permission == "read_write" && claims.Role == "viewer" {
		write(w, http.StatusForbidden, map[string]string{"error": "viewer accounts can only create read-only registry tokens"})
		return
	}
	secret, err := newRegistryAccessSecret()
	if err != nil {
		problem(w, err)
		return
	}
	sum := sha256.Sum256([]byte(secret))
	token := store.RegistryAccessToken{
		ID:          newID("rtk"),
		UserID:      claims.Subject,
		Name:        in.Name,
		TokenHash:   hex.EncodeToString(sum[:]),
		TokenPrefix: secret[:12],
		Permission:  in.Permission,
		CreatedAt:   time.Now().UTC(),
	}
	if err := a.store.CreateRegistryAccessToken(r.Context(), token); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{
		"token":         token,
		"secret":        secret,
		"username":      claims.Email,
		"registryHosts": a.effectiveRegistryHosts(r.Context()),
	})
}

func (a *API) deleteRegistryAccessToken(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	if err := a.store.DeleteRegistryAccessToken(r.Context(), strings.TrimSpace(r.PathValue("id")), claims.Subject); err != nil {
		if store.NotFound(err) {
			write(w, http.StatusNotFound, map[string]string{"error": "registry token not found"})
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]bool{"revoked": true})
}

func newRegistryAccessSecret() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "dkr_" + base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func (a *API) registrySettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.RegistrySettings(r.Context())
	if store.NotFound(err) {
		write(w, 200, registrySettingsResponse(defaultRegistrySettings()))
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	write(w, 200, registrySettingsResponse(settings))
}

func (a *API) updateRegistrySettings(w http.ResponseWriter, r *http.Request) {
	var in registrySettingsInput
	if !decode(w, r, &in) {
		return
	}
	clean, err := cleanRegistrySettings(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	claims, _ := auth.FromContext(r.Context())
	settings := store.RegistrySettings{
		Storage: clean.Storage, S3Region: clean.S3Region, S3Bucket: clean.S3Bucket, S3AccessKey: clean.S3AccessKey,
		S3Endpoint: clean.S3Endpoint, S3ForcePathStyle: clean.S3ForcePathStyle, S3Secure: clean.S3Secure,
		CreatedBy: claims.Subject,
	}
	secretKey := clean.S3SecretKey
	if secretKey == "" {
		existing, err := a.store.RegistrySettings(r.Context())
		if err == nil && existing.S3SecretKeyEncrypted != "" {
			settings.S3SecretKeyEncrypted = existing.S3SecretKeyEncrypted
		}
	}
	if settings.S3SecretKeyEncrypted == "" {
		if secretKey == "" && clean.Storage == "s3" {
			bad(w, "enter the S3 secret key")
			return
		}
		if secretKey != "" {
			encrypted, err := a.box.Encrypt(secretKey)
			if err != nil {
				problem(w, err)
				return
			}
			settings.S3SecretKeyEncrypted = encrypted
		}
	}
	if settings.S3SecretKeyEncrypted != "" {
		decrypted, err := a.box.Decrypt(settings.S3SecretKeyEncrypted)
		if err != nil {
			problem(w, err)
			return
		}
		secretKey = decrypted
	}
	if err := a.store.UpsertRegistrySettings(r.Context(), settings); err != nil {
		problem(w, err)
		return
	}
	if err := a.registry.ApplySettings(r.Context(), settings, secretKey); err != nil {
		a.log.Warn("recreate registry container", "error", err)
		write(w, 200, map[string]any{
			"settings": registrySettingsResponse(settings),
			"warning":  "settings were saved but the registry container could not be restarted with them: " + err.Error(),
		})
		return
	}
	write(w, 200, map[string]any{"settings": registrySettingsResponse(settings)})
}

func (a *API) registryDomain(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.RegistryDomainSettings(r.Context())
	if store.NotFound(err) {
		settings = store.RegistryDomainSettings{HTTPSEnabled: true}
	} else if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, registryDomainResponse(settings, a.effectiveRegistryHosts(r.Context())))
}

func (a *API) updateRegistryDomain(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domain       string `json:"domain"`
		HTTPSEnabled bool   `json:"httpsEnabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	domain, err := caddy.NormalizeDomain(in.Domain)
	if err != nil {
		bad(w, err.Error())
		return
	}
	a.domainMu.Lock()
	defer a.domainMu.Unlock()

	if domain != "" {
		assigned, lookupErr := a.store.ProjectDomainBindingByDomain(r.Context(), domain)
		if lookupErr == nil && assigned.Domain != "" {
			write(w, http.StatusConflict, map[string]string{"error": "this domain is already assigned to a project"})
			return
		}
		if lookupErr != nil && !store.NotFound(lookupErr) {
			problem(w, lookupErr)
			return
		}
	}

	previous, previousErr := a.store.RegistryDomainSettings(r.Context())
	if previousErr != nil && !store.NotFound(previousErr) {
		problem(w, previousErr)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	settings := store.RegistryDomainSettings{
		Domain:       domain,
		HTTPSEnabled: in.HTTPSEnabled,
		CreatedBy:    claims.Subject,
	}
	if err := a.store.UpsertRegistryDomainSettings(r.Context(), settings); err != nil {
		problem(w, err)
		return
	}
	if err := a.syncDomains(r.Context()); err != nil {
		if previousErr == nil {
			_ = a.store.UpsertRegistryDomainSettings(r.Context(), previous)
		} else {
			_ = a.store.UpsertRegistryDomainSettings(r.Context(), store.RegistryDomainSettings{HTTPSEnabled: true})
		}
		_ = a.syncDomains(r.Context())
		a.log.Warn("configure registry domain", "domain", domain, "error", err)
		write(w, http.StatusBadGateway, map[string]string{"error": "Caddy could not activate this registry domain; the previous route was restored"})
		return
	}
	if saved, err := a.store.RegistryDomainSettings(r.Context()); err == nil {
		settings = saved
	}
	write(w, http.StatusOK, registryDomainResponse(settings, a.effectiveRegistryHosts(r.Context())))
}

func registryDomainResponse(settings store.RegistryDomainSettings, hosts []string) map[string]any {
	return map[string]any{
		"domain":        settings.Domain,
		"httpsEnabled":  settings.HTTPSEnabled,
		"attached":      settings.Domain != "",
		"registryHosts": hosts,
		"updatedAt":     settings.UpdatedAt,
	}
}

func (a *API) effectiveRegistryHosts(ctx context.Context) []string {
	settings, err := a.store.RegistryDomainSettings(ctx)
	if err == nil && settings.Domain != "" {
		return []string{settings.Domain}
	}
	return append([]string(nil), a.registryHosts...)
}

func (a *API) registryDomainMatches(ctx context.Context, domain string) (bool, error) {
	settings, err := a.store.RegistryDomainSettings(ctx)
	if store.NotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return settings.Domain != "" && strings.EqualFold(settings.Domain, domain), nil
}

func (a *API) registryRepositories(w http.ResponseWriter, r *http.Request) {
	repositories, err := a.registry.Repositories(r.Context())
	if errors.Is(err, registry.ErrNotFound) {
		write(w, 404, map[string]string{"error": "the registry is not reachable"})
		return
	}
	if err != nil {
		a.log.Warn("list registry repositories", "error", err)
		write(w, 502, map[string]string{"error": "could not list registry repositories"})
		return
	}
	write(w, 200, map[string]any{"repositories": repositories, "count": len(repositories), "registryHosts": a.effectiveRegistryHosts(r.Context())})
}

func (a *API) registryDeleteTag(w http.ResponseWriter, r *http.Request) {
	name := strings.Trim(r.PathValue("name"), "/")
	tag := strings.TrimSpace(r.PathValue("tag"))
	if name == "" && tag == "" {
		name = strings.Trim(r.URL.Query().Get("name"), "/")
		tag = strings.TrimSpace(r.URL.Query().Get("tag"))
	}
	if name == "" || tag == "" {
		bad(w, "repository name and tag are required")
		return
	}
	if err := a.registry.DeleteTag(r.Context(), name, tag); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			write(w, 404, map[string]string{"error": "repository or tag was not found"})
			return
		}
		a.log.Warn("delete registry tag", "repository", name, "tag", tag, "error", err)
		write(w, 502, map[string]string{"error": "could not delete the tag: " + err.Error()})
		return
	}
	write(w, 200, map[string]any{"deleted": true})
}

func (a *API) registryGarbageCollect(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DryRun bool `json:"dryRun"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if !decode(w, r, &in) {
			return
		}
	}
	result, err := a.registry.GarbageCollect(r.Context(), in.DryRun)
	if errors.Is(err, registry.ErrNotFound) {
		write(w, 404, map[string]string{"error": "the registry container is not running"})
		return
	}
	if err != nil {
		a.log.Warn("registry garbage collection", "error", err)
		write(w, 502, map[string]string{"error": "garbage collection failed: " + err.Error()})
		return
	}
	write(w, 200, result)
}
