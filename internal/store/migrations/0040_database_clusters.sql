-- Databases are infrastructure resources, not project-owned services. A
-- cluster can be attached to any number of project-private networks.
ALTER TABLE database_services
    DROP CONSTRAINT database_services_project_id_fkey,
    ALTER COLUMN project_id DROP NOT NULL;

DROP INDEX database_services_project_name_unique;
CREATE INDEX database_services_name_idx
    ON database_services (LOWER(name));

CREATE TABLE database_cluster_databases (
    id TEXT PRIMARY KEY,
    database_service_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    username TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX database_cluster_databases_name_unique
    ON database_cluster_databases (database_service_id, LOWER(name));

CREATE TABLE project_database_attachments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    database_service_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    database_id TEXT NOT NULL REFERENCES database_cluster_databases(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, database_service_id)
);

CREATE UNIQUE INDEX project_database_attachments_alias_unique
    ON project_database_attachments (project_id, LOWER(alias));
CREATE INDEX project_database_attachments_cluster_idx
    ON project_database_attachments (database_service_id, created_at);

-- Preserve every existing database and its project relationship.
INSERT INTO database_cluster_databases(id,database_service_id,name,username,password_encrypted,created_at,updated_at)
SELECT id || '_database',id,database_name,username,password_encrypted,created_at,updated_at
FROM database_services;

INSERT INTO project_database_attachments(id,project_id,database_service_id,database_id,alias,created_at,updated_at)
SELECT id || '_attachment',project_id,id,id || '_database',
       COALESCE(NULLIF(LEFT(LOWER(REGEXP_REPLACE(REGEXP_REPLACE(name, '[^a-zA-Z0-9-]+', '-', 'g'), '(^-+|-+$)', '', 'g')), 50), ''), 'database') || '-' || RIGHT(id, 8),
       created_at,updated_at
FROM database_services;

UPDATE database_services SET project_id = NULL;
