-- Keep control-plane state independent from the Docker container lifecycle so
-- slow image pulls and failed starts remain visible in the database catalog.
ALTER TABLE database_services
    ADD COLUMN status TEXT NOT NULL DEFAULT 'created',
    ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
