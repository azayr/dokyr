ALTER TABLE github_app_installations
    ADD COLUMN contents_permission TEXT NOT NULL DEFAULT ''
        CHECK (contents_permission IN ('', 'read', 'write'));
