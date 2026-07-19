ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_source_type_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_source_type_check
    CHECK (source_type IN ('repository', 'image', 'empty'));

ALTER TABLE application_services
    ADD COLUMN environment_secret_keys TEXT NOT NULL DEFAULT '';

ALTER TABLE deployments
    ADD COLUMN service_id TEXT REFERENCES application_services(id) ON DELETE SET NULL,
    ADD COLUMN service_name TEXT NOT NULL DEFAULT '';

UPDATE deployments deployment
SET service_name = project.name
FROM projects project
WHERE deployment.project_id = project.id AND deployment.service_name = '';

CREATE INDEX deployments_service_created_idx
    ON deployments(service_id, created_at DESC)
    WHERE service_id IS NOT NULL;
