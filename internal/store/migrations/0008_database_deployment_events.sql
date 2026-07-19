CREATE TABLE database_deployment_events (
    id BIGSERIAL PRIMARY KEY,
    database_service_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX database_deployment_events_service_id_idx
    ON database_deployment_events(database_service_id, id);
