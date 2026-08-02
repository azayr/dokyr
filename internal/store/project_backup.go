package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type ProjectBackupSchedule struct {
	Configured        bool       `json:"configured"`
	ProjectID         string     `json:"projectId"`
	Enabled           bool       `json:"enabled"`
	ObjectStorageID   string     `json:"objectStorageId"`
	ObjectStorageName string     `json:"objectStorageName,omitempty"`
	Frequency         string     `json:"frequency"`
	Weekday           int        `json:"weekday"`
	Hour              int        `json:"hour"`
	Minute            int        `json:"minute"`
	Timezone          string     `json:"timezone"`
	RetentionCount    int        `json:"retentionCount"`
	LastRunAt         *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt         *time.Time `json:"nextRunAt,omitempty"`
	LastStatus        string     `json:"lastStatus"`
	LastMessage       string     `json:"lastMessage,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ProjectBackupJob struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"projectId"`
	ProjectName       string     `json:"projectName"`
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

func DefaultProjectBackupSchedule(projectID string) ProjectBackupSchedule {
	return ProjectBackupSchedule{ProjectID: projectID, Frequency: "daily", Hour: 2, Timezone: "UTC", RetentionCount: 7, LastStatus: "never"}
}

func (s *Store) ProjectBackupSchedule(ctx context.Context, projectID string) (ProjectBackupSchedule, error) {
	var v ProjectBackupSchedule
	err := s.db.QueryRowContext(ctx, `SELECT b.project_id,b.enabled,b.object_storage_id,o.name,b.frequency,b.weekday,b.hour,b.minute,b.timezone,b.retention_count,b.last_run_at,b.next_run_at,b.last_status,b.last_message,b.created_at,b.updated_at FROM project_backup_schedules b JOIN object_storage_connections o ON o.id=b.object_storage_id WHERE b.project_id=$1`, projectID).Scan(&v.ProjectID, &v.Enabled, &v.ObjectStorageID, &v.ObjectStorageName, &v.Frequency, &v.Weekday, &v.Hour, &v.Minute, &v.Timezone, &v.RetentionCount, &v.LastRunAt, &v.NextRunAt, &v.LastStatus, &v.LastMessage, &v.CreatedAt, &v.UpdatedAt)
	if err == nil {
		v.Configured = true
	}
	return v, err
}

func (s *Store) UpsertProjectBackupSchedule(ctx context.Context, v ProjectBackupSchedule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_backup_schedules(project_id,enabled,object_storage_id,frequency,weekday,hour,minute,timezone,retention_count,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(project_id) DO UPDATE SET enabled=EXCLUDED.enabled,object_storage_id=EXCLUDED.object_storage_id,frequency=EXCLUDED.frequency,weekday=EXCLUDED.weekday,hour=EXCLUDED.hour,minute=EXCLUDED.minute,timezone=EXCLUDED.timezone,retention_count=EXCLUDED.retention_count,next_run_at=EXCLUDED.next_run_at,updated_at=NOW()`, v.ProjectID, v.Enabled, v.ObjectStorageID, v.Frequency, v.Weekday, v.Hour, v.Minute, v.Timezone, v.RetentionCount, v.NextRunAt)
	return err
}

func (s *Store) CreateProjectBackupJob(ctx context.Context, v ProjectBackupJob) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_backup_jobs(id,project_id,project_name,kind,status,object_storage_id,object_storage_name,object_key,filename,size_bytes,source_job_id,trigger,message,created_by,created_at) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,NULLIF($11,''),$12,$13,NULLIF($14,''),$15)`, v.ID, v.ProjectID, v.ProjectName, v.Kind, v.Status, v.ObjectStorageID, v.ObjectStorageName, v.ObjectKey, v.Filename, v.SizeBytes, v.SourceJobID, v.Trigger, v.Message, v.CreatedBy, v.CreatedAt)
	return err
}

func scanProjectBackupJob(scanner interface{ Scan(...any) error }) (ProjectBackupJob, error) {
	var v ProjectBackupJob
	err := scanner.Scan(&v.ID, &v.ProjectID, &v.ProjectName, &v.Kind, &v.Status, &v.ObjectStorageID, &v.ObjectStorageName, &v.ObjectKey, &v.Filename, &v.SizeBytes, &v.SourceJobID, &v.Trigger, &v.Message, &v.CreatedBy, &v.CreatedAt, &v.StartedAt, &v.FinishedAt)
	return v, err
}

const projectBackupJobColumns = `id,project_id,project_name,kind,status,COALESCE(object_storage_id,''),object_storage_name,object_key,filename,size_bytes,COALESCE(source_job_id,''),trigger,message,COALESCE(created_by,''),created_at,started_at,finished_at`

func (s *Store) ProjectBackupJobs(ctx context.Context, projectID string, limit int) ([]ProjectBackupJob, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+projectBackupJobColumns+` FROM project_backup_jobs WHERE project_id=$1 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectBackupJob{}
	for rows.Next() {
		v, e := scanProjectBackupJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ProjectBackupJob(ctx context.Context, id string) (ProjectBackupJob, error) {
	return scanProjectBackupJob(s.db.QueryRowContext(ctx, `SELECT `+projectBackupJobColumns+` FROM project_backup_jobs WHERE id=$1`, id))
}
func (s *Store) PendingProjectBackupJobIDs(ctx context.Context) ([]string, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id FROM project_backup_jobs WHERE status='queued' ORDER BY created_at`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) FailInterruptedProjectBackupJobs(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, `UPDATE project_backup_jobs SET status='failed',message='Dokyr restarted while this job was running',finished_at=NOW() WHERE status='running'`)
	return e
}
func (s *Store) ClaimProjectBackupJob(ctx context.Context, id string, at time.Time) (bool, error) {
	r, e := s.db.ExecContext(ctx, `UPDATE project_backup_jobs SET status='running',started_at=$2,message='' WHERE id=$1 AND status='queued'`, id, at)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *Store) FinishProjectBackupJob(ctx context.Context, id, status, message, key, filename string, size int64, at time.Time) error {
	r, e := s.db.ExecContext(ctx, `UPDATE project_backup_jobs SET status=$2,message=$3,object_key=CASE WHEN $4='' THEN object_key ELSE $4 END,filename=CASE WHEN $5='' THEN filename ELSE $5 END,size_bytes=CASE WHEN $6=0 THEN size_bytes ELSE $6 END,finished_at=$7 WHERE id=$1`, id, status, message, key, filename, size, at)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n == 0 {
		return sql.ErrNoRows
	}
	return e
}
func (s *Store) ProjectBackupRetentionCandidates(ctx context.Context, projectID string, keep int) ([]ProjectBackupJob, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT `+projectBackupJobColumns+` FROM project_backup_jobs WHERE project_id=$1 AND kind='backup' AND status='succeeded' AND object_key<>'' ORDER BY created_at DESC OFFSET $2`, projectID, keep)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []ProjectBackupJob
	for rows.Next() {
		v, e := scanProjectBackupJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ForgetProjectBackupObject(ctx context.Context, id string) error {
	_, e := s.db.ExecContext(ctx, `UPDATE project_backup_jobs SET object_key='',message='Expired by retention policy' WHERE id=$1`)
	return e
}

func (s *Store) DueProjectBackupSchedules(ctx context.Context, now time.Time) ([]ProjectBackupSchedule, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT b.project_id,b.enabled,b.object_storage_id,o.name,b.frequency,b.weekday,b.hour,b.minute,b.timezone,b.retention_count,b.last_run_at,b.next_run_at,b.last_status,b.last_message,b.created_at,b.updated_at FROM project_backup_schedules b JOIN object_storage_connections o ON o.id=b.object_storage_id WHERE b.enabled=TRUE AND b.next_run_at<=$1`, now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []ProjectBackupSchedule
	for rows.Next() {
		var v ProjectBackupSchedule
		e = rows.Scan(&v.ProjectID, &v.Enabled, &v.ObjectStorageID, &v.ObjectStorageName, &v.Frequency, &v.Weekday, &v.Hour, &v.Minute, &v.Timezone, &v.RetentionCount, &v.LastRunAt, &v.NextRunAt, &v.LastStatus, &v.LastMessage, &v.CreatedAt, &v.UpdatedAt)
		if e != nil {
			return nil, e
		}
		v.Configured = true
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) QueueDueProjectBackup(ctx context.Context, v ProjectBackupSchedule, next time.Time, job ProjectBackupJob) (bool, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, `UPDATE project_backup_schedules SET last_run_at=NOW(),next_run_at=$2,last_status='queued',last_message='',updated_at=NOW() WHERE project_id=$1 AND enabled=TRUE AND next_run_at<=NOW()`, v.ProjectID, next)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return false, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO project_backup_jobs(id,project_id,project_name,kind,status,object_storage_id,object_storage_name,trigger,created_at) VALUES($1,$2,$3,'backup','queued',$4,$5,'scheduled',$6)`, job.ID, job.ProjectID, job.ProjectName, job.ObjectStorageID, job.ObjectStorageName, job.CreatedAt)
	if e != nil {
		return false, e
	}
	return true, tx.Commit()
}
func (s *Store) FinishProjectBackupSchedule(ctx context.Context, projectID, status, message string) error {
	_, e := s.db.ExecContext(ctx, `UPDATE project_backup_schedules SET last_status=$2,last_message=$3 WHERE project_id=$1`, projectID, status, message)
	return e
}

type ProjectBackupData struct {
	Project      Project                      `json:"project"`
	Applications []ApplicationService         `json:"applications"`
	Databases    []DatabaseService            `json:"databases"`
	Environment  []ProjectEnvironmentVariable `json:"environment"`
	Domains      []ProjectDomainBinding       `json:"domains"`
}

func (s *Store) RestoreProjectBackupData(ctx context.Context, d ProjectBackupData) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var connectionID, registryID any
	if d.Project.ConnectionID != "" {
		connectionID = d.Project.ConnectionID
	}
	if d.Project.RegistryID != "" {
		registryID = d.Project.RegistryID
	}
	_, e = tx.ExecContext(ctx, `UPDATE projects SET name=$2,repository=$3,branch=$4,status=$5,domain=$6,source_type=$7,connection_id=$8,registry_id=$9,image_url=$10,container_port=$11,https_enabled=$12,updated_at=NOW() WHERE id=$1`, d.Project.ID, d.Project.Name, d.Project.Repository, d.Project.Branch, d.Project.Status, d.Project.Domain, d.Project.SourceType, connectionID, registryID, d.Project.ImageURL, d.Project.ContainerPort, d.Project.HTTPSEnabled)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `UPDATE deployments SET service_id=NULL WHERE project_id=$1`, d.Project.ID)
	if e != nil {
		return e
	}
	for _, table := range []string{"project_domain_bindings", "project_environment_variables", "application_services", "database_services"} {
		if _, e = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE project_id=$1`, d.Project.ID); e != nil {
			return e
		}
	}
	for i, v := range d.Environment {
		_, e = tx.ExecContext(ctx, `INSERT INTO project_environment_variables(project_id,key,value_encrypted,is_secret,position) VALUES($1,$2,$3,$4,$5)`, d.Project.ID, v.Key, v.ValueEncrypted, v.Secret, i)
		if e != nil {
			return e
		}
	}
	for _, v := range d.Applications {
		var rid, cid any
		if v.RegistryID != "" {
			rid = v.RegistryID
		}
		if v.ConnectionID != "" {
			cid = v.ConnectionID
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO application_services(id,project_id,name,source_type,image_url,registry_id,internal_registry,connection_id,repository,branch,dockerfile_path,build_context,build_strategy,auto_deploy,registry_webhook_secret_encrypted,registry_webhook_tag,container_port,command,health_check_type,health_check_path,health_check_command,health_check_timeout_seconds,environment,environment_secret_keys,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)`, v.ID, d.Project.ID, v.Name, v.SourceType, v.ImageURL, rid, v.InternalRegistry, cid, v.Repository, v.Branch, v.DockerfilePath, v.BuildContext, v.BuildStrategy, v.AutoDeploy, v.RegistryWebhookSecret, v.RegistryWebhookTag, v.ContainerPort, v.Command, v.HealthCheckType, v.HealthCheckPath, v.HealthCheckCommand, v.HealthCheckTimeout, v.EnvironmentEncrypted, strings.Join(v.EnvironmentSecretKeys, "\n"), v.Status)
		if e != nil {
			return e
		}
	}
	for _, v := range d.Databases {
		var port any
		if v.PublicEnabled {
			port = v.PublicPort
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO database_services(id,project_id,name,engine,image,internal_port,public_enabled,public_port,volume_name,username,database_name,password_encrypted) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, v.ID, d.Project.ID, v.Name, v.Engine, v.Image, v.InternalPort, v.PublicEnabled, port, v.VolumeName, v.Username, v.DatabaseName, v.PasswordEncrypted)
		if e != nil {
			return e
		}
	}
	for pos, b := range d.Domains {
		_, e = tx.ExecContext(ctx, `INSERT INTO project_domain_bindings(id,project_id,domain,https_enabled,position) VALUES($1,$2,$3,$4,$5)`, b.ID, d.Project.ID, b.Domain, b.HTTPSEnabled, pos)
		if e != nil {
			return e
		}
		for rp, r := range b.Rules {
			var sid any
			if r.ServiceID != "" {
				sid = r.ServiceID
			}
			_, e = tx.ExecContext(ctx, `INSERT INTO project_domain_binding_rules(binding_id,path_pattern,upstream_port,service_id,position) VALUES($1,$2,$3,$4,$5)`, b.ID, r.Path, r.Port, sid, rp)
			if e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
