package store

import (
	"context"
	"database/sql"
	"time"
)

type ServerBackupSchedule struct {
	Configured        bool       `json:"configured"`
	Enabled           bool       `json:"enabled"`
	ObjectStorageID   string     `json:"objectStorageId"`
	ObjectStorageName string     `json:"objectStorageName,omitempty"`
	Frequency         string     `json:"frequency"`
	Weekday           int        `json:"weekday"`
	Hour              int        `json:"hour"`
	Minute            int        `json:"minute"`
	Timezone          string     `json:"timezone"`
	LastRunAt         *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt         *time.Time `json:"nextRunAt,omitempty"`
	LastStatus        string     `json:"lastStatus"`
	LastMessage       string     `json:"lastMessage,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ServerBackupJob struct {
	ID                string     `json:"id"`
	Kind              string     `json:"kind"`
	Status            string     `json:"status"`
	ObjectStorageID   string     `json:"objectStorageId,omitempty"`
	ObjectStorageName string     `json:"objectStorageName"`
	ObjectKey         string     `json:"objectKey,omitempty"`
	Filename          string     `json:"filename,omitempty"`
	SizeBytes         int64      `json:"sizeBytes"`
	SourceJobID       string     `json:"sourceJobId,omitempty"`
	Trigger           string     `json:"trigger"`
	Message           string     `json:"message,omitempty"`
	CreatedBy         string     `json:"-"`
	CreatedAt         time.Time  `json:"createdAt"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
}

func DefaultServerBackupSchedule() ServerBackupSchedule {
	return ServerBackupSchedule{
		Frequency:  "daily",
		Hour:       2,
		Timezone:   "UTC",
		LastStatus: "never",
	}
}

func (s *Store) ServerBackupSchedule(ctx context.Context) (ServerBackupSchedule, error) {
	var schedule ServerBackupSchedule
	err := s.db.QueryRowContext(ctx, `SELECT b.enabled,b.object_storage_id,o.name,b.frequency,b.weekday,b.hour,b.minute,b.timezone,
		b.last_run_at,b.next_run_at,b.last_status,b.last_message,b.created_at,b.updated_at
		FROM server_backup_schedule b
		JOIN object_storage_connections o ON o.id=b.object_storage_id
		WHERE b.singleton=TRUE`).Scan(
		&schedule.Enabled, &schedule.ObjectStorageID, &schedule.ObjectStorageName, &schedule.Frequency,
		&schedule.Weekday, &schedule.Hour, &schedule.Minute, &schedule.Timezone,
		&schedule.LastRunAt, &schedule.NextRunAt, &schedule.LastStatus, &schedule.LastMessage,
		&schedule.CreatedAt, &schedule.UpdatedAt,
	)
	if err == nil {
		schedule.Configured = true
	}
	return schedule, err
}

func (s *Store) UpsertServerBackupSchedule(ctx context.Context, schedule ServerBackupSchedule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_backup_schedule(
		singleton,enabled,object_storage_id,frequency,weekday,hour,minute,timezone,next_run_at)
		VALUES(TRUE,$1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(singleton) DO UPDATE SET
		enabled=EXCLUDED.enabled,object_storage_id=EXCLUDED.object_storage_id,frequency=EXCLUDED.frequency,
		weekday=EXCLUDED.weekday,hour=EXCLUDED.hour,minute=EXCLUDED.minute,timezone=EXCLUDED.timezone,
		next_run_at=EXCLUDED.next_run_at,updated_at=NOW()`,
		schedule.Enabled, schedule.ObjectStorageID, schedule.Frequency, schedule.Weekday,
		schedule.Hour, schedule.Minute, schedule.Timezone, schedule.NextRunAt,
	)
	return err
}

func (s *Store) CreateServerBackupJob(ctx context.Context, job ServerBackupJob) error {
	var createdBy any
	if job.CreatedBy != "" {
		createdBy = job.CreatedBy
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_backup_jobs(
		id,kind,status,object_storage_id,object_storage_name,object_key,filename,size_bytes,
		source_job_id,trigger,message,created_by,created_at)
		VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12,$13)`,
		job.ID, job.Kind, job.Status, job.ObjectStorageID, job.ObjectStorageName, job.ObjectKey,
		job.Filename, job.SizeBytes, job.SourceJobID, job.Trigger, job.Message, createdBy, job.CreatedAt,
	)
	return err
}

func (s *Store) ServerBackupJobs(ctx context.Context, limit int) ([]ServerBackupJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,status,COALESCE(object_storage_id,''),object_storage_name,
		object_key,filename,size_bytes,COALESCE(source_job_id,''),trigger,message,COALESCE(created_by,''),
		created_at,started_at,finished_at
		FROM server_backup_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []ServerBackupJob{}
	for rows.Next() {
		var job ServerBackupJob
		if err := rows.Scan(&job.ID, &job.Kind, &job.Status, &job.ObjectStorageID, &job.ObjectStorageName,
			&job.ObjectKey, &job.Filename, &job.SizeBytes, &job.SourceJobID, &job.Trigger, &job.Message,
			&job.CreatedBy, &job.CreatedAt, &job.StartedAt, &job.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) ServerBackupJob(ctx context.Context, id string) (ServerBackupJob, error) {
	var job ServerBackupJob
	err := s.db.QueryRowContext(ctx, `SELECT id,kind,status,COALESCE(object_storage_id,''),object_storage_name,
		object_key,filename,size_bytes,COALESCE(source_job_id,''),trigger,message,COALESCE(created_by,''),
		created_at,started_at,finished_at FROM server_backup_jobs WHERE id=$1`, id).Scan(
		&job.ID, &job.Kind, &job.Status, &job.ObjectStorageID, &job.ObjectStorageName,
		&job.ObjectKey, &job.Filename, &job.SizeBytes, &job.SourceJobID, &job.Trigger, &job.Message,
		&job.CreatedBy, &job.CreatedAt, &job.StartedAt, &job.FinishedAt,
	)
	return job, err
}

func (s *Store) PendingServerBackupJobIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM server_backup_jobs WHERE status='queued' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) FailInterruptedServerBackupJobs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_backup_jobs SET status='failed',
		message='Dokyr restarted while this job was running',finished_at=NOW()
		WHERE status='running'`)
	return err
}

func (s *Store) ClaimServerBackupJob(ctx context.Context, id string, startedAt time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE server_backup_jobs SET status='running',started_at=$2,message=''
		WHERE id=$1 AND status='queued'`, id, startedAt)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) FinishServerBackupJob(ctx context.Context, id, status, message, objectKey, filename string, sizeBytes int64, finishedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE server_backup_jobs SET status=$2,message=$3,
		object_key=CASE WHEN $4='' THEN object_key ELSE $4 END,
		filename=CASE WHEN $5='' THEN filename ELSE $5 END,
		size_bytes=CASE WHEN $6=0 THEN size_bytes ELSE $6 END,finished_at=$7
		WHERE id=$1`, id, status, message, objectKey, filename, sizeBytes, finishedAt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CompleteRestoredServerBackupJob recreates the audit row when a database
// restore replaced the table with the older copy contained in the archive.
func (s *Store) CompleteRestoredServerBackupJob(ctx context.Context, job ServerBackupJob, message string, finishedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO server_backup_jobs(
		id,kind,status,object_storage_id,object_storage_name,object_key,filename,size_bytes,
		source_job_id,trigger,message,created_by,created_at,started_at,finished_at)
		VALUES($1,'restore','succeeded',NULLIF($2,''),$3,$4,$5,$6,NULLIF($7,''),'restore',$8,NULLIF($9,''),$10,$11,$12)
		ON CONFLICT(id) DO UPDATE SET status='succeeded',message=EXCLUDED.message,finished_at=EXCLUDED.finished_at`,
		job.ID, job.ObjectStorageID, job.ObjectStorageName, job.ObjectKey, job.Filename, job.SizeBytes,
		job.SourceJobID, message, job.CreatedBy, job.CreatedAt, job.StartedAt, finishedAt,
	)
	if err != nil {
		return err
	}
	if job.SourceJobID != "" {
		_, err = tx.ExecContext(ctx, `UPDATE server_backup_jobs SET
			status='succeeded',object_storage_id=NULLIF($2,''),object_storage_name=$3,object_key=$4,
			filename=$5,size_bytes=$6,message='Backup available',finished_at=COALESCE(finished_at,$7)
			WHERE id=$1`,
			job.SourceJobID, job.ObjectStorageID, job.ObjectStorageName, job.ObjectKey,
			job.Filename, job.SizeBytes, finishedAt,
		)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE server_backup_schedule SET
		last_status='succeeded',last_message='Backup restored'
		WHERE singleton=TRUE AND last_status IN ('queued','running')`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) QueueDueServerBackup(ctx context.Context, expectedUpdatedAt, startedAt, nextRunAt time.Time, job ServerBackupJob) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE server_backup_schedule SET
		last_run_at=$1,next_run_at=$2,last_status='queued',last_message='',updated_at=NOW()
		WHERE singleton=TRUE AND enabled=TRUE AND next_run_at<=$1 AND updated_at=$3`,
		startedAt, nextRunAt, expectedUpdatedAt)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return false, err
	}
	var createdBy any
	if job.CreatedBy != "" {
		createdBy = job.CreatedBy
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO server_backup_jobs(
		id,kind,status,object_storage_id,object_storage_name,trigger,message,created_by,created_at)
		VALUES($1,'backup','queued',$2,$3,'scheduled','',$4,$5)`,
		job.ID, job.ObjectStorageID, job.ObjectStorageName, createdBy, job.CreatedAt)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) FinishServerBackupSchedule(ctx context.Context, status, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_backup_schedule SET last_status=$1,last_message=$2 WHERE singleton=TRUE`,
		status, message)
	return err
}

func (s *Store) MarkServerBackupScheduleRunning(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_backup_schedule SET last_status='running',last_message='' WHERE singleton=TRUE`)
	return err
}
