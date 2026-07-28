CREATE TABLE object_storage_connections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('aws', 'r2', 'minio', 'digitalocean', 'custom')),
    region TEXT NOT NULL,
    bucket TEXT NOT NULL,
    endpoint TEXT NOT NULL DEFAULT '',
    access_key TEXT NOT NULL,
    secret_key_encrypted TEXT NOT NULL,
    force_path_style BOOLEAN NOT NULL DEFAULT FALSE,
    secure BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX object_storage_connections_name_unique
    ON object_storage_connections (LOWER(name));

ALTER TABLE registry_settings
    ADD COLUMN object_storage_id TEXT REFERENCES object_storage_connections(id) ON DELETE RESTRICT;

-- Preserve S3 settings created before reusable object storage connections were
-- introduced. The registry keeps the exact same credentials and starts
-- pointing at the imported connection without requiring operator action.
INSERT INTO object_storage_connections (
    id, name, provider, region, bucket, endpoint, access_key,
    secret_key_encrypted, force_path_style, secure, created_by, created_at, updated_at
)
SELECT
    'obj_legacy_registry',
    'Registry storage',
    CASE
        WHEN s3_endpoint ILIKE '%.r2.cloudflarestorage.com%' THEN 'r2'
        WHEN s3_endpoint ILIKE '%.digitaloceanspaces.com%' THEN 'digitalocean'
        WHEN s3_endpoint = '' THEN 'aws'
        WHEN s3_force_path_style THEN 'minio'
        ELSE 'custom'
    END,
    s3_region,
    s3_bucket,
    s3_endpoint,
    s3_access_key,
    s3_secret_key_encrypted,
    s3_force_path_style,
    s3_secure,
    created_by,
    created_at,
    updated_at
FROM registry_settings
WHERE singleton = TRUE
  AND storage = 's3'
  AND s3_bucket <> ''
  AND s3_secret_key_encrypted <> '';

UPDATE registry_settings
SET object_storage_id = 'obj_legacy_registry'
WHERE singleton = TRUE
  AND storage = 's3'
  AND EXISTS (
      SELECT 1 FROM object_storage_connections WHERE id = 'obj_legacy_registry'
  );
