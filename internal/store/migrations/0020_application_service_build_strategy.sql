ALTER TABLE application_services
    ADD COLUMN build_strategy TEXT NOT NULL DEFAULT 'dockerfile'
    CHECK (build_strategy IN ('dockerfile', 'railpack', 'nixpacks'));
