ALTER TABLE application_services
    ADD COLUMN health_check_type TEXT NOT NULL DEFAULT 'none'
        CHECK (health_check_type IN ('none', 'http', 'command')),
    ADD COLUMN health_check_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN health_check_command TEXT NOT NULL DEFAULT '',
    ADD COLUMN health_check_timeout_seconds INTEGER NOT NULL DEFAULT 60
        CHECK (health_check_timeout_seconds BETWEEN 5 AND 600);
