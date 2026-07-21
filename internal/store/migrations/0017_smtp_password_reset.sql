CREATE TABLE smtp_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    host TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 587 CHECK (port BETWEEN 1 AND 65535),
    encryption TEXT NOT NULL DEFAULT 'starttls' CHECK (encryption IN ('starttls', 'tls', 'none')),
    username TEXT NOT NULL DEFAULT '',
    password_encrypted TEXT NOT NULL DEFAULT '',
    from_name TEXT NOT NULL DEFAULT 'Dokyr',
    from_email TEXT NOT NULL DEFAULT '',
    notify_deployment_failures BOOLEAN NOT NULL DEFAULT TRUE,
    notify_deployment_successes BOOLEAN NOT NULL DEFAULT FALSE,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens(user_id);
CREATE INDEX password_reset_tokens_expiry_idx ON password_reset_tokens(expires_at);
