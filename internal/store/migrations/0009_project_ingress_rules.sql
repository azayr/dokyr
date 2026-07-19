CREATE TABLE project_ingress_rules (
    id BIGSERIAL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path_pattern TEXT NOT NULL,
    upstream_port INTEGER NOT NULL CHECK (upstream_port BETWEEN 1 AND 65535),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, path_pattern)
);

CREATE INDEX project_ingress_rules_project_idx ON project_ingress_rules(project_id);
