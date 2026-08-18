ALTER TABLE source_connections
    DROP CONSTRAINT IF EXISTS source_connections_provider_check;

ALTER TABLE source_connections
    ADD CONSTRAINT source_connections_provider_check
    CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE oauth_states
    DROP CONSTRAINT IF EXISTS oauth_states_provider_check;

ALTER TABLE oauth_states
    ADD CONSTRAINT oauth_states_provider_check
    CHECK (provider IN ('github', 'gitlab', 'gitea'));
