ALTER TABLE application_services
    ADD COLUMN auto_deploy BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN registry_webhook_secret_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN registry_webhook_tag TEXT NOT NULL DEFAULT '';

CREATE INDEX application_services_auto_deploy_repository_idx
    ON application_services(LOWER(repository), branch)
    WHERE source_type = 'repository' AND auto_deploy = TRUE;

CREATE TABLE webhook_deliveries (
    provider TEXT NOT NULL,
    delivery_id TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(provider, delivery_id)
);

CREATE INDEX webhook_deliveries_received_at_idx
    ON webhook_deliveries(received_at);
