package store

import (
	"context"
	"time"
)

type CleanupSchedule struct {
	Configured    bool       `json:"configured"`
	Enabled       bool       `json:"enabled"`
	Frequency     string     `json:"frequency"`
	Weekday       int        `json:"weekday"`
	Hour          int        `json:"hour"`
	Minute        int        `json:"minute"`
	Timezone      string     `json:"timezone"`
	Containers    bool       `json:"containers"`
	Images        bool       `json:"images"`
	BuildCache    bool       `json:"buildCache"`
	Networks      bool       `json:"networks"`
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt     *time.Time `json:"nextRunAt,omitempty"`
	LastStatus    string     `json:"lastStatus"`
	LastMessage   string     `json:"lastMessage,omitempty"`
	LastDeleted   int        `json:"lastDeleted"`
	LastReclaimed int64      `json:"lastReclaimed"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func DefaultCleanupSchedule() CleanupSchedule {
	return CleanupSchedule{
		Frequency:  "weekly",
		Weekday:    0,
		Hour:       3,
		Timezone:   "UTC",
		Containers: true,
		Images:     true,
		BuildCache: true,
		Networks:   true,
		LastStatus: "never",
	}
}

func (s *Store) CleanupSchedule(ctx context.Context) (CleanupSchedule, error) {
	var schedule CleanupSchedule
	err := s.db.QueryRowContext(ctx, `SELECT enabled,frequency,weekday,hour,minute,timezone,
		cleanup_containers,cleanup_images,cleanup_build_cache,cleanup_networks,
		last_run_at,next_run_at,last_status,last_message,last_deleted,last_reclaimed,created_at,updated_at
		FROM cleanup_schedule WHERE singleton=TRUE`).Scan(
		&schedule.Enabled, &schedule.Frequency, &schedule.Weekday, &schedule.Hour, &schedule.Minute, &schedule.Timezone,
		&schedule.Containers, &schedule.Images, &schedule.BuildCache, &schedule.Networks,
		&schedule.LastRunAt, &schedule.NextRunAt, &schedule.LastStatus, &schedule.LastMessage,
		&schedule.LastDeleted, &schedule.LastReclaimed, &schedule.CreatedAt, &schedule.UpdatedAt,
	)
	if err == nil {
		schedule.Configured = true
	}
	return schedule, err
}

func (s *Store) UpsertCleanupSchedule(ctx context.Context, schedule CleanupSchedule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO cleanup_schedule(
		singleton,enabled,frequency,weekday,hour,minute,timezone,
		cleanup_containers,cleanup_images,cleanup_build_cache,cleanup_networks,next_run_at)
		VALUES(TRUE,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(singleton) DO UPDATE SET
		enabled=EXCLUDED.enabled,frequency=EXCLUDED.frequency,weekday=EXCLUDED.weekday,
		hour=EXCLUDED.hour,minute=EXCLUDED.minute,timezone=EXCLUDED.timezone,
		cleanup_containers=EXCLUDED.cleanup_containers,cleanup_images=EXCLUDED.cleanup_images,
		cleanup_build_cache=EXCLUDED.cleanup_build_cache,cleanup_networks=EXCLUDED.cleanup_networks,
		next_run_at=EXCLUDED.next_run_at,updated_at=NOW()`,
		schedule.Enabled, schedule.Frequency, schedule.Weekday, schedule.Hour, schedule.Minute, schedule.Timezone,
		schedule.Containers, schedule.Images, schedule.BuildCache, schedule.Networks, schedule.NextRunAt,
	)
	return err
}

func (s *Store) ClaimCleanupSchedule(ctx context.Context, expectedUpdatedAt, startedAt, nextRunAt time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE cleanup_schedule SET
		last_run_at=$1,next_run_at=$2,last_status='running',last_message='',last_deleted=0,last_reclaimed=0
		WHERE singleton=TRUE AND enabled=TRUE AND next_run_at<=$1 AND updated_at=$3`,
		startedAt, nextRunAt, expectedUpdatedAt,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) FinishCleanupSchedule(ctx context.Context, status, message string, deleted int, reclaimed int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE cleanup_schedule SET
		last_status=$1,last_message=$2,last_deleted=$3,last_reclaimed=$4
		WHERE singleton=TRUE`,
		status, message, deleted, reclaimed,
	)
	return err
}
