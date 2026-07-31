CREATE TABLE server_backup_schedule (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    object_storage_id TEXT NOT NULL REFERENCES object_storage_connections(id) ON DELETE RESTRICT,
    frequency TEXT NOT NULL DEFAULT 'daily' CHECK (frequency IN ('daily', 'weekly')),
    weekday SMALLINT NOT NULL DEFAULT 0 CHECK (weekday BETWEEN 0 AND 6),
    hour SMALLINT NOT NULL DEFAULT 2 CHECK (hour BETWEEN 0 AND 23),
    minute SMALLINT NOT NULL DEFAULT 0 CHECK (minute BETWEEN 0 AND 59),
    timezone TEXT NOT NULL DEFAULT 'UTC',
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT 'never' CHECK (last_status IN ('never', 'queued', 'running', 'succeeded', 'failed')),
    last_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE server_backup_jobs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('backup', 'restore')),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    object_storage_id TEXT,
    object_storage_name TEXT NOT NULL DEFAULT '',
    object_key TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    source_job_id TEXT,
    trigger TEXT NOT NULL DEFAULT 'manual' CHECK (trigger IN ('manual', 'scheduled', 'restore')),
    message TEXT NOT NULL DEFAULT '',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX server_backup_jobs_created_at_idx ON server_backup_jobs (created_at DESC);
CREATE INDEX server_backup_jobs_status_idx ON server_backup_jobs (status);
