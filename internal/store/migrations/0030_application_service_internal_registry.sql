ALTER TABLE application_services
    ADD COLUMN internal_registry BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX application_services_internal_registry_auto_deploy_idx
    ON application_services(image_url)
    WHERE source_type = 'image' AND internal_registry = TRUE AND auto_deploy = TRUE;
