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
