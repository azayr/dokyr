CREATE TABLE deployment_events (
    id BIGSERIAL PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('start','log','complete','error')),
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX deployment_events_deployment_id_idx
    ON deployment_events(deployment_id, id);
