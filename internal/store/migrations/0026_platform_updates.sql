CREATE TABLE platform_update_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    auto_update BOOLEAN NOT NULL DEFAULT FALSE,
    check_interval_minutes INTEGER NOT NULL DEFAULT 60 CHECK (check_interval_minutes BETWEEN 15 AND 10080),
    maintenance_hour SMALLINT NOT NULL DEFAULT 3 CHECK (maintenance_hour BETWEEN 0 AND 23),
    timezone TEXT NOT NULL DEFAULT 'UTC',
    last_checked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE platform_update_jobs (
    id TEXT PRIMARY KEY,
    source_version TEXT NOT NULL,
    target_version TEXT NOT NULL,
    target_image TEXT NOT NULL,
    target_digest TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'pulling', 'restarting', 'succeeded', 'failed')),
    message TEXT NOT NULL DEFAULT '',
    requested_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX platform_update_one_active
    ON platform_update_jobs ((TRUE))
    WHERE status IN ('pending', 'pulling', 'restarting');
