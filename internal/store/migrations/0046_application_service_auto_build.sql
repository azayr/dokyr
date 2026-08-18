ALTER TABLE application_services
    DROP CONSTRAINT application_services_build_strategy_check;

UPDATE application_services
SET build_strategy = 'auto'
WHERE build_strategy <> 'dockerfile';

ALTER TABLE application_services
    ADD CONSTRAINT application_services_build_strategy_check
    CHECK (build_strategy IN ('dockerfile', 'auto'));
