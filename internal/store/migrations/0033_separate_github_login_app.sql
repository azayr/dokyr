ALTER TABLE provider_app_configs
    DROP CONSTRAINT IF EXISTS provider_app_configs_provider_check;

ALTER TABLE provider_app_configs
    ADD CONSTRAINT provider_app_configs_provider_check
    CHECK (provider IN ('github', 'github_login'));
