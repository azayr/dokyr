ALTER TABLE users
    ADD COLUMN two_factor_secret_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN two_factor_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN github_account_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN github_login TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX users_github_account_unique
    ON users (github_account_id)
    WHERE github_account_id <> '';

CREATE TABLE auth_oauth_states (
    state_hash TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('github')),
    mode TEXT NOT NULL CHECK (mode IN ('login', 'link')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX auth_oauth_states_expiry_idx ON auth_oauth_states(expires_at);
