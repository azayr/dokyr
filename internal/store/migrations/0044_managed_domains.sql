CREATE TABLE managed_domains (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'failed')),
    observed_records TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX managed_domains_domain_unique ON managed_domains(LOWER(domain));

-- Domains created by older Dokyr versions become part of the reusable catalog.
INSERT INTO managed_domains(id, domain, status, created_at, updated_at)
SELECT 'mnd_' || SUBSTRING(MD5(binding.domain) FROM 1 FOR 20), binding.domain, 'pending', binding.created_at, binding.updated_at
FROM project_domain_bindings binding
ON CONFLICT DO NOTHING;
