ALTER TABLE application_services
    ADD COLUMN source_type TEXT NOT NULL DEFAULT 'image' CHECK (source_type IN ('image', 'repository')),
    ADD COLUMN connection_id TEXT REFERENCES source_connections(id) ON DELETE SET NULL,
    ADD COLUMN repository TEXT NOT NULL DEFAULT '',
    ADD COLUMN branch TEXT NOT NULL DEFAULT 'main',
    ADD COLUMN dockerfile_path TEXT NOT NULL DEFAULT 'Dockerfile',
    ADD COLUMN build_context TEXT NOT NULL DEFAULT '.';

CREATE INDEX application_services_connection_idx
    ON application_services(connection_id)
    WHERE connection_id IS NOT NULL;
