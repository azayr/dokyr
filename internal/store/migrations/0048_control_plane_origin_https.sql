ALTER TABLE control_plane_settings
    ADD COLUMN origin_https_enabled BOOLEAN NOT NULL DEFAULT TRUE;
