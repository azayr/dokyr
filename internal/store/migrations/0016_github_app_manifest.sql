CREATE TABLE provider_app_configs (
    provider TEXT PRIMARY KEY CHECK (provider IN ('github')),
    app_id TEXT NOT NULL,
    app_slug TEXT NOT NULL,
    client_id_encrypted TEXT NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    private_key_encrypted TEXT NOT NULL DEFAULT '',
    webhook_secret_encrypted TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE provider_setup_states (
    state_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('github')),
    mode TEXT NOT NULL CHECK (mode IN ('account_link')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX provider_setup_states_expiry_idx ON provider_setup_states(expires_at);
