package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

type SMTPBootstrap struct {
	Present                   bool
	Enabled                   bool
	Host                      string
	Port                      int
	Encryption                string
	Username                  string
	Password                  string
	FromName                  string
	FromEmail                 string
	NotifyDeploymentFailures  bool
	NotifyDeploymentSuccesses bool
}

type RegistryBootstrap struct {
	Present          bool
	Storage          string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3Endpoint       string
	S3ForcePathStyle bool
	S3Secure         bool
}

type Config struct {
	Address                      string
	Frontend                     string
	DatabaseURL                  string
	JWTSecret                    string
	JWTIssuer                    string
	CookieSecure                 bool
	PublicURL                    string
	EncryptionKey                string
	GitLabClientID               string
	GitLabClientSecret           string
	GitLabBaseURL                string
	GiteaClientID                string
	GiteaClientSecret            string
	GiteaBaseURL                 string
	CaddyAdminURL                string
	ControlUpstream              string
	ControlHosts                 []string
	RegistryHosts                []string
	RegistryTokenIssuer          string
	RegistryTokenService         string
	RegistryTokenPrivateKeyPath  string
	RegistryTokenCertificatePath string
	RegistryInternalSecret       string
	PlatformImage                string
	UpdateChannel                string
	MailStalwartURL              string
	MailStalwartAPIKey           string
	MailStalwartUser             string
	MailStalwartPassword         string
	MailStalwartHostname         string
	MailStalwartDefaultDomain    string
	MailStalwartRelayHost        string
	MailStalwartRelayPort        int
	MailStalwartRelayPassword    string
	SMTP                         SMTPBootstrap
	Registry                     RegistryBootstrap
}

func Load() Config {
	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpFromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	registryStorage := strings.ToLower(strings.TrimSpace(env("REGISTRY_STORAGE", "filesystem")))
	s3Bucket := strings.TrimSpace(os.Getenv("REGISTRY_S3_BUCKET"))
	encryptionKey := env("SELFHOST_ENCRYPTION_KEY", "development-encryption-key-change-now")
	registryInternalSeed := env("REGISTRY_INTERNAL_SECRET", encryptionKey)
	registryInternalHash := sha256.Sum256([]byte(registryInternalSeed))
	return Config{
		Address:                      env("SELFHOST_ADDRESS", ":8080"),
		Frontend:                     env("SELFHOST_FRONTEND_DIR", "./web/build"),
		DatabaseURL:                  env("DATABASE_URL", "postgres://selfhost:selfhost@localhost:5432/selfhost?sslmode=disable"),
		JWTSecret:                    env("SELFHOST_JWT_SECRET", "development-only-change-this-secret-now"),
		JWTIssuer:                    env("SELFHOST_JWT_ISSUER", "selfhost"),
		CookieSecure:                 env("SELFHOST_COOKIE_SECURE", "false") == "true",
		PublicURL:                    env("SELFHOST_PUBLIC_URL", "http://localhost:8080"),
		EncryptionKey:                encryptionKey,
		GitLabClientID:               os.Getenv("GITLAB_CLIENT_ID"),
		GitLabClientSecret:           os.Getenv("GITLAB_CLIENT_SECRET"),
		GitLabBaseURL:                env("GITLAB_BASE_URL", "https://gitlab.com"),
		GiteaClientID:                os.Getenv("GITEA_CLIENT_ID"),
		GiteaClientSecret:            os.Getenv("GITEA_CLIENT_SECRET"),
		GiteaBaseURL:                 env("GITEA_BASE_URL", "https://gitea.com"),
		CaddyAdminURL:                env("CADDY_ADMIN_URL", "unix:///run/caddy-admin/admin.sock"),
		ControlUpstream:              env("DOKYR_CONTROL_UPSTREAM", "selfhost:8080"),
		ControlHosts:                 splitHosts(env("SELFHOST_CONTROL_HOSTS", "localhost")),
		RegistryHosts:                splitHosts(env("REGISTRY_HOSTS", "registry.invalid")),
		RegistryTokenIssuer:          env("REGISTRY_TOKEN_ISSUER", "dokyr-registry"),
		RegistryTokenService:         env("REGISTRY_TOKEN_SERVICE", "dokyr-registry"),
		RegistryTokenPrivateKeyPath:  env("REGISTRY_TOKEN_PRIVATE_KEY_PATH", "/run/registry-auth/registry-token.key"),
		RegistryTokenCertificatePath: env("REGISTRY_TOKEN_CERTIFICATE_PATH", "/run/registry-auth/registry-token.crt"),
		RegistryInternalSecret:       hex.EncodeToString(registryInternalHash[:]),
		PlatformImage:                env("SELFHOST_PLATFORM_IMAGE", "ghcr.io/azayr/dokyr"),
		UpdateChannel:                env("SELFHOST_UPDATE_CHANNEL", "latest"),
		MailStalwartURL:              strings.TrimSpace(os.Getenv("MAIL_STALWART_URL")),
		MailStalwartAPIKey:           strings.TrimSpace(os.Getenv("MAIL_STALWART_API_KEY")),
		MailStalwartUser:             strings.TrimSpace(os.Getenv("MAIL_STALWART_USER")),
		MailStalwartPassword:         os.Getenv("MAIL_STALWART_PASSWORD"),
		MailStalwartHostname:         strings.TrimSpace(os.Getenv("MAIL_STALWART_HOSTNAME")),
		MailStalwartDefaultDomain:    strings.TrimSpace(os.Getenv("MAIL_STALWART_DEFAULT_DOMAIN")),
		MailStalwartRelayHost:        strings.TrimSpace(os.Getenv("MAIL_STALWART_RELAY_HOST")),
		MailStalwartRelayPort:        envInt("MAIL_STALWART_RELAY_PORT", 465),
		MailStalwartRelayPassword:    os.Getenv("MAIL_STALWART_RELAY_PASSWORD"),
		SMTP: SMTPBootstrap{
			Present:                   smtpHost != "" || smtpFromEmail != "",
			Enabled:                   envBool("SMTP_ENABLED", true),
			Host:                      smtpHost,
			Port:                      envInt("SMTP_PORT", 587),
			Encryption:                env("SMTP_ENCRYPTION", "starttls"),
			Username:                  strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password:                  os.Getenv("SMTP_PASSWORD"),
			FromName:                  env("SMTP_FROM_NAME", "Dokyr"),
			FromEmail:                 smtpFromEmail,
			NotifyDeploymentFailures:  envBool("SMTP_NOTIFY_DEPLOYMENT_FAILURES", true),
			NotifyDeploymentSuccesses: envBool("SMTP_NOTIFY_DEPLOYMENT_SUCCESSES", false),
		},
		Registry: RegistryBootstrap{
			Present:          registryStorage == "s3" || s3Bucket != "",
			Storage:          registryStorage,
			S3Region:         strings.TrimSpace(os.Getenv("REGISTRY_S3_REGION")),
			S3Bucket:         s3Bucket,
			S3AccessKey:      strings.TrimSpace(os.Getenv("REGISTRY_S3_ACCESSKEY")),
			S3SecretKey:      os.Getenv("REGISTRY_S3_SECRETKEY"),
			S3Endpoint:       strings.TrimSpace(os.Getenv("REGISTRY_S3_ENDPOINT")),
			S3ForcePathStyle: envBool("REGISTRY_S3_FORCEPATHSTYLE", false),
			S3Secure:         envBool("REGISTRY_S3_SECURE", true),
		},
	}
}

func splitHosts(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' })
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
