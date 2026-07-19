CREATE TABLE source_connections (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('github', 'gitlab')),
    account_id TEXT NOT NULL,
    account_name TEXT NOT NULL,
    account_avatar TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL,
    access_token_encrypted TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, account_id, base_url)
);

CREATE TABLE oauth_states (
    state_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('github', 'gitlab')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE registry_credentials (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    registry_url TEXT NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    password_encrypted TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE projects
    ADD COLUMN source_type TEXT NOT NULL DEFAULT 'repository' CHECK (source_type IN ('repository', 'image')),
    ADD COLUMN connection_id TEXT REFERENCES source_connections(id) ON DELETE SET NULL,
    ADD COLUMN registry_id TEXT REFERENCES registry_credentials(id) ON DELETE SET NULL,
    ADD COLUMN image_url TEXT NOT NULL DEFAULT '';

CREATE INDEX source_connections_user_idx ON source_connections(user_id, provider);
CREATE INDEX oauth_states_expiry_idx ON oauth_states(expires_at);
CREATE INDEX registry_credentials_user_idx ON registry_credentials(created_by);
