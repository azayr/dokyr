CREATE TABLE application_services (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    image_url TEXT NOT NULL,
    registry_id TEXT REFERENCES registry_credentials(id) ON DELETE SET NULL,
    container_port INTEGER NOT NULL DEFAULT 80 CHECK (container_port BETWEEN 1 AND 65535),
    environment TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'created',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX application_services_project_name_unique
    ON application_services(project_id, LOWER(name));
CREATE INDEX application_services_project_idx
    ON application_services(project_id, created_at);

ALTER TABLE project_domain_binding_rules
    ADD COLUMN service_id TEXT REFERENCES application_services(id) ON DELETE RESTRICT;

CREATE INDEX project_domain_binding_rules_service_idx
    ON project_domain_binding_rules(service_id)
    WHERE service_id IS NOT NULL;
