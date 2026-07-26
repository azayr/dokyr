CREATE TABLE registry_access_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    permission TEXT NOT NULL CHECK (permission IN ('read_only', 'read_write')),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX registry_access_tokens_user_created_idx
    ON registry_access_tokens(user_id, created_at DESC);
