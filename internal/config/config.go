package config

import (
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

type Config struct {
	Address            string
	Frontend           string
	DatabaseURL        string
	JWTSecret          string
	JWTIssuer          string
	CookieSecure       bool
	PublicURL          string
	EncryptionKey      string
	GitLabClientID     string
	GitLabClientSecret string
	GitLabBaseURL      string
	CaddyAdminURL      string
	ControlHosts       []string
	SMTP               SMTPBootstrap
}

func Load() Config {
	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpFromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	return Config{
		Address:            env("SELFHOST_ADDRESS", ":8080"),
		Frontend:           env("SELFHOST_FRONTEND_DIR", "./web/build"),
		DatabaseURL:        env("DATABASE_URL", "postgres://selfhost:selfhost@localhost:5432/selfhost?sslmode=disable"),
		JWTSecret:          env("SELFHOST_JWT_SECRET", "development-only-change-this-secret-now"),
		JWTIssuer:          env("SELFHOST_JWT_ISSUER", "selfhost"),
		CookieSecure:       env("SELFHOST_COOKIE_SECURE", "false") == "true",
		PublicURL:          env("SELFHOST_PUBLIC_URL", "http://localhost:8080"),
		EncryptionKey:      env("SELFHOST_ENCRYPTION_KEY", "development-encryption-key-change-now"),
		GitLabClientID:     os.Getenv("GITLAB_CLIENT_ID"),
		GitLabClientSecret: os.Getenv("GITLAB_CLIENT_SECRET"),
		GitLabBaseURL:      env("GITLAB_BASE_URL", "https://gitlab.com"),
		CaddyAdminURL:      env("CADDY_ADMIN_URL", "unix:///run/caddy-admin/admin.sock"),
		ControlHosts:       splitHosts(env("SELFHOST_CONTROL_HOSTS", "localhost")),
		SMTP: SMTPBootstrap{
			Present:                   smtpHost != "" || smtpFromEmail != "",
			Enabled:                   envBool("SMTP_ENABLED", true),
			Host:                      smtpHost,
			Port:                      envInt("SMTP_PORT", 587),
			Encryption:                env("SMTP_ENCRYPTION", "starttls"),
			Username:                  strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password:                  os.Getenv("SMTP_PASSWORD"),
			FromName:                  env("SMTP_FROM_NAME", "DeployForge"),
			FromEmail:                 smtpFromEmail,
			NotifyDeploymentFailures:  envBool("SMTP_NOTIFY_DEPLOYMENT_FAILURES", true),
			NotifyDeploymentSuccesses: envBool("SMTP_NOTIFY_DEPLOYMENT_SUCCESSES", false),
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
