package store

import (
	"context"
	"database/sql"
	"time"
)

type PlatformUpdateSettings struct {
	Configured           bool       `json:"configured"`
	AutoUpdate           bool       `json:"autoUpdate"`
	CheckIntervalMinutes int        `json:"checkIntervalMinutes"`
	MaintenanceHour      int        `json:"maintenanceHour"`
	Timezone             string     `json:"timezone"`
	LastCheckedAt        *time.Time `json:"lastCheckedAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type PlatformUpdateJob struct {
	ID            string     `json:"id"`
	SourceVersion string     `json:"sourceVersion"`
	TargetVersion string     `json:"targetVersion"`
	TargetImage   string     `json:"targetImage"`
	TargetDigest  string     `json:"targetDigest"`
	Status        string     `json:"status"`
	Message       string     `json:"message,omitempty"`
	RequestedBy   string     `json:"-"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func DefaultPlatformUpdateSettings() PlatformUpdateSettings {
	return PlatformUpdateSettings{CheckIntervalMinutes: 60, MaintenanceHour: 3, Timezone: "UTC"}
}

func (s *Store) PlatformUpdateSettings(ctx context.Context) (PlatformUpdateSettings, error) {
	var settings PlatformUpdateSettings
	err := s.db.QueryRowContext(ctx, `SELECT auto_update,check_interval_minutes,maintenance_hour,timezone,
		last_checked_at,created_at,updated_at FROM platform_update_settings WHERE singleton=TRUE`).Scan(
		&settings.AutoUpdate, &settings.CheckIntervalMinutes, &settings.MaintenanceHour, &settings.Timezone,
		&settings.LastCheckedAt, &settings.CreatedAt, &settings.UpdatedAt,
	)
	if err == nil {
		settings.Configured = true
	}
	return settings, err
}

func (s *Store) UpsertPlatformUpdateSettings(ctx context.Context, settings PlatformUpdateSettings) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO platform_update_settings(
		singleton,auto_update,check_interval_minutes,maintenance_hour,timezone)
		VALUES(TRUE,$1,$2,$3,$4)
		ON CONFLICT(singleton) DO UPDATE SET auto_update=EXCLUDED.auto_update,
		check_interval_minutes=EXCLUDED.check_interval_minutes,
		maintenance_hour=EXCLUDED.maintenance_hour,timezone=EXCLUDED.timezone,updated_at=NOW()`,
		settings.AutoUpdate, settings.CheckIntervalMinutes, settings.MaintenanceHour, settings.Timezone)
	return err
}

func (s *Store) MarkPlatformUpdateChecked(ctx context.Context, checkedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO platform_update_settings(singleton,last_checked_at)
		VALUES(TRUE,$1) ON CONFLICT(singleton) DO UPDATE SET last_checked_at=EXCLUDED.last_checked_at`, checkedAt)
	return err
}

func (s *Store) CreatePlatformUpdateJob(ctx context.Context, job PlatformUpdateJob) error {
	var requestedBy any
	if job.RequestedBy != "" {
		requestedBy = job.RequestedBy
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO platform_update_jobs(
		id,source_version,target_version,target_image,target_digest,status,message,requested_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		job.ID, job.SourceVersion, job.TargetVersion, job.TargetImage, job.TargetDigest, job.Status, job.Message, requestedBy)
	return err
}

func (s *Store) SetPlatformUpdateJobStatus(ctx context.Context, id, status, message string, finished bool) error {
	var finishedAt any
	if finished {
		finishedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE platform_update_jobs SET status=$2,message=$3,
		finished_at=COALESCE($4,finished_at),updated_at=NOW() WHERE id=$1`, id, status, message, finishedAt)
	return err
}

func (s *Store) ActivePlatformUpdateJob(ctx context.Context) (PlatformUpdateJob, error) {
	return s.platformUpdateJob(ctx, `WHERE status IN ('pending','pulling','restarting') ORDER BY started_at DESC LIMIT 1`)
}

func (s *Store) LatestPlatformUpdateJob(ctx context.Context) (PlatformUpdateJob, error) {
	return s.platformUpdateJob(ctx, `ORDER BY started_at DESC LIMIT 1`)
}

func (s *Store) platformUpdateJob(ctx context.Context, suffix string) (PlatformUpdateJob, error) {
	var job PlatformUpdateJob
	var requestedBy sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,source_version,target_version,target_image,target_digest,
		status,message,requested_by,started_at,finished_at,updated_at FROM platform_update_jobs `+suffix).Scan(
		&job.ID, &job.SourceVersion, &job.TargetVersion, &job.TargetImage, &job.TargetDigest,
		&job.Status, &job.Message, &requestedBy, &job.StartedAt, &job.FinishedAt, &job.UpdatedAt)
	if requestedBy.Valid {
		job.RequestedBy = requestedBy.String
	}
	return job, err
}

func (s *Store) ReconcilePlatformUpdateJob(ctx context.Context, currentVersion string) error {
	job, err := s.ActivePlatformUpdateJob(ctx)
	if err != nil {
		return err
	}
	if job.TargetVersion == currentVersion {
		return s.SetPlatformUpdateJobStatus(ctx, job.ID, "succeeded", "Dokyr restarted on "+currentVersion, true)
	}
	return s.SetPlatformUpdateJobStatus(ctx, job.ID, "failed", "The previous release was restored after the update did not become healthy", true)
}
