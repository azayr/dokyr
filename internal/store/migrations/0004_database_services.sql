CREATE TABLE database_services (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    engine TEXT NOT NULL CHECK (engine IN ('mysql', 'mariadb', 'postgres')),
    image TEXT NOT NULL,
    internal_port INTEGER NOT NULL CHECK (internal_port BETWEEN 1 AND 65535),
    public_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    public_port INTEGER CHECK (public_port BETWEEN 1 AND 65535),
    volume_name TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    database_name TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX database_services_project_name_unique
    ON database_services (project_id, LOWER(name));

CREATE UNIQUE INDEX database_services_public_port_unique
    ON database_services (public_port)
    WHERE public_enabled = TRUE;

CREATE INDEX database_services_project_idx
    ON database_services (project_id, created_at);
