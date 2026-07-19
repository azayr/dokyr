CREATE TABLE project_ingress_defaults (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    path_pattern TEXT NOT NULL DEFAULT '/*',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO project_ingress_defaults(project_id, path_pattern)
SELECT id, '/*' FROM projects
ON CONFLICT(project_id) DO NOTHING;
