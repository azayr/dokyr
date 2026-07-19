ALTER TABLE provider_setup_states
    DROP CONSTRAINT IF EXISTS provider_setup_states_mode_check;

ALTER TABLE provider_setup_states
    ADD CONSTRAINT provider_setup_states_mode_check
    CHECK (mode IN ('account_link', 'repository_install'));

CREATE TABLE github_app_installations (
    installation_id BIGINT PRIMARY KEY,
    connection_id TEXT NOT NULL UNIQUE REFERENCES source_connections(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    account_login TEXT NOT NULL,
    account_avatar TEXT NOT NULL DEFAULT '',
    repository_selection TEXT NOT NULL DEFAULT 'selected'
        CHECK (repository_selection IN ('all', 'selected')),
    manage_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX github_app_installations_user_idx
    ON github_app_installations(user_id, updated_at DESC);
