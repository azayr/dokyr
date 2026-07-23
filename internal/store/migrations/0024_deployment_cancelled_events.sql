ALTER TABLE deployment_events
    DROP CONSTRAINT deployment_events_event_type_check;

ALTER TABLE deployment_events
    ADD CONSTRAINT deployment_events_event_type_check
    CHECK (event_type IN ('start','log','complete','error','cancelled'));
