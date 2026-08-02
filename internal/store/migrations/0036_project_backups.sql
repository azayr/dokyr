CREATE TABLE project_backup_schedules (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    object_storage_id TEXT NOT NULL REFERENCES object_storage_connections(id) ON DELETE RESTRICT,
    frequency TEXT NOT NULL DEFAULT 'daily' CHECK (frequency IN ('daily', 'weekly')),
    weekday SMALLINT NOT NULL DEFAULT 0 CHECK (weekday BETWEEN 0 AND 6),
    hour SMALLINT NOT NULL DEFAULT 2 CHECK (hour BETWEEN 0 AND 23),
    minute SMALLINT NOT NULL DEFAULT 0 CHECK (minute BETWEEN 0 AND 59),
    timezone TEXT NOT NULL DEFAULT 'UTC',
    retention_count SMALLINT NOT NULL DEFAULT 7 CHECK (retention_count BETWEEN 1 AND 100),
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT 'never' CHECK (last_status IN ('never', 'queued', 'running', 'succeeded', 'failed')),
    last_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_backup_jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    project_name TEXT NOT NULL DEFAULT '',
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

CREATE INDEX project_backup_jobs_project_created_idx ON project_backup_jobs (project_id, created_at DESC);
CREATE INDEX project_backup_jobs_status_idx ON project_backup_jobs (status);
