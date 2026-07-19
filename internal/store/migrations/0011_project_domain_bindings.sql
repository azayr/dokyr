CREATE TABLE project_domain_bindings (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    https_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX project_domain_bindings_domain_unique
    ON project_domain_bindings(LOWER(domain));
CREATE INDEX project_domain_bindings_project_idx
    ON project_domain_bindings(project_id, position, created_at);

CREATE TABLE project_domain_binding_rules (
    id BIGSERIAL PRIMARY KEY,
    binding_id TEXT NOT NULL REFERENCES project_domain_bindings(id) ON DELETE CASCADE,
    path_pattern TEXT NOT NULL,
    upstream_port INTEGER NOT NULL CHECK (upstream_port BETWEEN 1 AND 65535),
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(binding_id, path_pattern)
);

CREATE INDEX project_domain_binding_rules_binding_idx
    ON project_domain_binding_rules(binding_id, position, id);

INSERT INTO project_domain_bindings(id, project_id, domain, https_enabled, position)
SELECT 'dom_' || SUBSTRING(MD5(id || ':' || domain) FROM 1 FOR 20), id, domain, https_enabled, 0
FROM projects
WHERE domain <> '';

INSERT INTO project_domain_binding_rules(binding_id, path_pattern, upstream_port, position)
SELECT binding.id, COALESCE(defaults.path_pattern, '/*'), projects.container_port, 0
FROM project_domain_bindings binding
JOIN projects ON projects.id = binding.project_id
LEFT JOIN project_ingress_defaults defaults ON defaults.project_id = projects.id;

INSERT INTO project_domain_binding_rules(binding_id, path_pattern, upstream_port, position)
SELECT binding.id, rules.path_pattern, rules.upstream_port,
       ROW_NUMBER() OVER (PARTITION BY binding.id ORDER BY LENGTH(rules.path_pattern) DESC, rules.path_pattern)::INTEGER
FROM project_domain_bindings binding
JOIN project_ingress_rules rules ON rules.project_id = binding.project_id
LEFT JOIN project_ingress_defaults defaults ON defaults.project_id = binding.project_id
WHERE rules.path_pattern <> COALESCE(defaults.path_pattern, '/*');
