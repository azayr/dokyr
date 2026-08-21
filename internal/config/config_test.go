package config

import "testing"

func TestSMTPBootstrapAbsentWhenRequiredValuesAreEmpty(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM_EMAIL", "")

	loaded := Load()
	if loaded.SMTP.Present {
		t.Fatal("empty SMTP environment must not trigger a database bootstrap")
	}
}

func TestSMTPBootstrapReadsEnvironment(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM_EMAIL", "deploy@example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_ENCRYPTION", "tls")
	t.Setenv("SMTP_USERNAME", "deploy")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_ENABLED", "false")
	t.Setenv("SMTP_NOTIFY_DEPLOYMENT_FAILURES", "false")
	t.Setenv("SMTP_NOTIFY_DEPLOYMENT_SUCCESSES", "true")

	loaded := Load()
	if !loaded.SMTP.Present || loaded.SMTP.Host != "smtp.example.com" || loaded.SMTP.FromEmail != "deploy@example.com" {
		t.Fatalf("unexpected SMTP bootstrap identity: %+v", loaded.SMTP)
	}
	if loaded.SMTP.Port != 465 || loaded.SMTP.Encryption != "tls" || loaded.SMTP.Enabled {
		t.Fatalf("unexpected SMTP connection settings: %+v", loaded.SMTP)
	}
	if loaded.SMTP.NotifyDeploymentFailures || !loaded.SMTP.NotifyDeploymentSuccesses {
		t.Fatalf("unexpected SMTP notification settings: %+v", loaded.SMTP)
	}
}

func TestControlUpstreamDefaultsToLegacyServiceAndAcceptsDokyr(t *testing.T) {
	t.Setenv("DOKYR_CONTROL_UPSTREAM", "")
	if loaded := Load(); loaded.ControlUpstream != "selfhost:8080" {
		t.Fatalf("default control upstream = %q", loaded.ControlUpstream)
	}
	t.Setenv("DOKYR_CONTROL_UPSTREAM", "dokyr:8080")
	if loaded := Load(); loaded.ControlUpstream != "dokyr:8080" {
		t.Fatalf("configured control upstream = %q", loaded.ControlUpstream)
	}
}

func TestDokyrEnvironmentPrefersCurrentNameAndAcceptsLegacyName(t *testing.T) {
	t.Setenv("DOKYR_ENCRYPTION_KEY", "")
	t.Setenv("SELFHOST_ENCRYPTION_KEY", "legacy-encryption-key-value")
	if loaded := Load(); loaded.EncryptionKey != "legacy-encryption-key-value" {
		t.Fatalf("legacy encryption key = %q", loaded.EncryptionKey)
	}

	t.Setenv("DOKYR_ENCRYPTION_KEY", "current-encryption-key-value")
	if loaded := Load(); loaded.EncryptionKey != "current-encryption-key-value" {
		t.Fatalf("current encryption key = %q", loaded.EncryptionKey)
	}
}

func TestGiteaConfigurationSupportsLocalNetworkOrigin(t *testing.T) {
	t.Setenv("GITEA_CLIENT_ID", "dokyr-local")
	t.Setenv("GITEA_CLIENT_SECRET", "secret")
	t.Setenv("GITEA_BASE_URL", "http://192.168.1.20:3000")

	loaded := Load()
	if loaded.GiteaClientID != "dokyr-local" || loaded.GiteaClientSecret != "secret" {
		t.Fatalf("unexpected Gitea OAuth credentials: %+v", loaded)
	}
	if loaded.GiteaBaseURL != "http://192.168.1.20:3000" {
		t.Fatalf("Gitea base URL = %q", loaded.GiteaBaseURL)
	}
}
