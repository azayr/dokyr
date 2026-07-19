CREATE UNIQUE INDEX projects_domain_unique_idx
    ON projects (LOWER(domain))
    WHERE domain <> '';
