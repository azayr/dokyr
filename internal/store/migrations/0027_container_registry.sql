CREATE TABLE registry_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    storage TEXT NOT NULL DEFAULT 'filesystem' CHECK (storage IN ('filesystem', 's3')),
    s3_region TEXT NOT NULL DEFAULT '',
    s3_bucket TEXT NOT NULL DEFAULT '',
    s3_access_key TEXT NOT NULL DEFAULT '',
    s3_secret_key_encrypted TEXT NOT NULL DEFAULT '',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_force_path_style BOOLEAN NOT NULL DEFAULT FALSE,
    s3_secure BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
