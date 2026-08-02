package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/store"
	"github.com/azayr/selfhost/internal/version"
)

const projectBackupFormatVersion = 1

type projectBackupScheduleInput struct {
	Enabled         bool   `json:"enabled"`
	ObjectStorageID string `json:"objectStorageId"`
	Frequency       string `json:"frequency"`
	Weekday         int    `json:"weekday"`
	Hour            int    `json:"hour"`
	Minute          int    `json:"minute"`
	Timezone        string `json:"timezone"`
	RetentionCount  int    `json:"retentionCount"`
}
type projectBackupSecrets struct {
	Environment            []string            `json:"environment"`
	ApplicationEnvironment map[string]string   `json:"applicationEnvironment"`
	ApplicationSecretKeys  map[string][]string `json:"applicationSecretKeys"`
	ApplicationWebhooks    map[string]string   `json:"applicationWebhooks"`
	DatabasePasswords      map[string]string   `json:"databasePasswords"`
}
type projectBackupVolume struct {
	ServiceID   string `json:"serviceId"`
	ServiceName string `json:"serviceName"`
	Engine      string `json:"engine"`
	VolumeName  string `json:"volumeName"`
	File        string `json:"file"`
}
type projectBackupManifest struct {
	FormatVersion int                     `json:"formatVersion"`
	Type          string                  `json:"type"`
	CreatedAt     time.Time               `json:"createdAt"`
	DokyrVersion  string                  `json:"dokyrVersion"`
	ProjectID     string                  `json:"projectId"`
	ProjectName   string                  `json:"projectName"`
	Data          store.ProjectBackupData `json:"data"`
	Secrets       projectBackupSecrets    `json:"secrets"`
	Volumes       []projectBackupVolume   `json:"volumes"`
}

func (a *API) projectBackups(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, e := a.store.Project(r.Context(), projectID); store.NotFound(e) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if e != nil {
		problem(w, e)
		return
	}
	jobs, e := a.store.ProjectBackupJobs(r.Context(), projectID, 50)
	if e != nil {
		problem(w, e)
		return
	}
	schedule, e := a.store.ProjectBackupSchedule(r.Context(), projectID)
	if store.NotFound(e) {
		schedule = store.DefaultProjectBackupSchedule(projectID)
	} else if e != nil {
		problem(w, e)
		return
	}
	connections, e := a.store.ObjectStorageConnections(r.Context())
	if e != nil {
		problem(w, e)
		return
	}
	destinations := make([]map[string]any, 0, len(connections))
	for _, c := range connections {
		destinations = append(destinations, map[string]any{"id": c.ID, "name": c.Name, "provider": c.Provider, "bucket": c.Bucket, "region": c.Region})
	}
	write(w, 200, map[string]any{"jobs": jobs, "schedule": schedule, "destinations": destinations})
}

func (a *API) createProjectBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ObjectStorageID string `json:"objectStorageId"`
	}
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	p, e := a.store.Project(r.Context(), projectID)
	if store.NotFound(e) {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	} else if e != nil {
		problem(w, e)
		return
	}
	c, e := a.store.ObjectStorageConnection(r.Context(), strings.TrimSpace(in.ObjectStorageID))
	if e != nil {
		bad(w, "select an available object storage connection")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	job := store.ProjectBackupJob{ID: newID("pbk"), ProjectID: p.ID, ProjectName: p.Name, Kind: "backup", Status: "queued", ObjectStorageID: c.ID, ObjectStorageName: c.Name, Trigger: "manual", CreatedBy: claims.Subject, CreatedAt: time.Now().UTC()}
	if e = a.store.CreateProjectBackupJob(r.Context(), job); e != nil {
		problem(w, e)
		return
	}
	a.enqueueProjectBackupJob(job.ID)
	write(w, http.StatusAccepted, map[string]any{"job": job})
}

func (a *API) updateProjectBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var in projectBackupScheduleInput
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, e := a.store.Project(r.Context(), projectID); e != nil {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	v, e := cleanProjectBackupSchedule(projectID, in, time.Now().UTC())
	if e != nil {
		bad(w, e.Error())
		return
	}
	if _, e = a.store.ObjectStorageConnection(r.Context(), v.ObjectStorageID); e != nil {
		bad(w, "select an available object storage connection")
		return
	}
	if e = a.store.UpsertProjectBackupSchedule(r.Context(), v); e != nil {
		problem(w, e)
		return
	}
	v, e = a.store.ProjectBackupSchedule(r.Context(), projectID)
	if e != nil {
		problem(w, e)
		return
	}
	write(w, 200, v)
}

func cleanProjectBackupSchedule(projectID string, in projectBackupScheduleInput, now time.Time) (store.ProjectBackupSchedule, error) {
	v := store.ProjectBackupSchedule{Configured: true, ProjectID: projectID, Enabled: in.Enabled, ObjectStorageID: strings.TrimSpace(in.ObjectStorageID), Frequency: strings.ToLower(strings.TrimSpace(in.Frequency)), Weekday: in.Weekday, Hour: in.Hour, Minute: in.Minute, Timezone: strings.TrimSpace(in.Timezone), RetentionCount: in.RetentionCount}
	if v.ObjectStorageID == "" {
		return v, errors.New("select an object storage connection")
	}
	if v.Frequency != "daily" && v.Frequency != "weekly" {
		return v, errors.New("frequency must be daily or weekly")
	}
	if v.Weekday < 0 || v.Weekday > 6 || v.Hour < 0 || v.Hour > 23 || v.Minute < 0 || v.Minute > 59 {
		return v, errors.New("enter a valid schedule")
	}
	if v.RetentionCount == 0 {
		v.RetentionCount = 7
	}
	if v.RetentionCount < 1 || v.RetentionCount > 100 {
		return v, errors.New("backups to keep must be between 1 and 100")
	}
	if _, e := time.LoadLocation(v.Timezone); e != nil {
		return v, errors.New("timezone is not recognized")
	}
	if v.Enabled {
		n, e := nextServerBackupRun(store.ServerBackupSchedule{Frequency: v.Frequency, Weekday: v.Weekday, Hour: v.Hour, Minute: v.Minute, Timezone: v.Timezone}, now)
		if e != nil {
			return v, e
		}
		v.NextRunAt = &n
	}
	return v, nil
}

func (a *API) restoreProjectBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirmation string `json:"confirmation"`
	}
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	p, e := a.store.Project(r.Context(), projectID)
	if e != nil {
		write(w, 404, map[string]string{"error": "project not found"})
		return
	}
	if in.Confirmation != "RESTORE "+p.Name {
		bad(w, `type "RESTORE `+p.Name+`" to confirm`)
		return
	}
	source, e := a.store.ProjectBackupJob(r.Context(), strings.TrimSpace(r.PathValue("backupId")))
	if e != nil || source.ProjectID != projectID {
		write(w, 404, map[string]string{"error": "backup not found"})
		return
	}
	if source.Kind != "backup" || source.Status != "succeeded" || source.ObjectKey == "" {
		bad(w, "only a completed backup can be restored")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	job := store.ProjectBackupJob{ID: newID("prst"), ProjectID: p.ID, ProjectName: p.Name, Kind: "restore", Status: "queued", ObjectStorageID: source.ObjectStorageID, ObjectStorageName: source.ObjectStorageName, ObjectKey: source.ObjectKey, Filename: source.Filename, SizeBytes: source.SizeBytes, SourceJobID: source.ID, Trigger: "restore", CreatedBy: claims.Subject, CreatedAt: time.Now().UTC()}
	if e = a.store.CreateProjectBackupJob(r.Context(), job); e != nil {
		problem(w, e)
		return
	}
	a.enqueueProjectBackupJob(job.ID)
	write(w, http.StatusAccepted, map[string]any{"job": job})
}

func (a *API) StartProjectBackupWorker(ctx context.Context) {
	if e := a.store.FailInterruptedProjectBackupJobs(ctx); e != nil {
		a.log.Warn("mark interrupted project backup jobs failed", "error", e)
	}
	go a.projectBackupWorker(ctx)
	a.enqueuePendingProjectBackupJobs(ctx)
	a.runDueProjectBackups(ctx)
	go func() {
		ticker := time.NewTicker(serverBackupSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.enqueuePendingProjectBackupJobs(ctx)
				a.runDueProjectBackups(ctx)
			}
		}
	}()
}
func (a *API) enqueueProjectBackupJob(id string) {
	select {
	case a.projectBackupQueue <- id:
	default:
		a.log.Info("project backup queue is full; persisted job will be retried", "job", id)
	}
}
func (a *API) enqueuePendingProjectBackupJobs(ctx context.Context) {
	ids, e := a.store.PendingProjectBackupJobIDs(ctx)
	if e != nil {
		return
	}
	for _, id := range ids {
		a.enqueueProjectBackupJob(id)
	}
}
func (a *API) projectBackupWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-a.projectBackupQueue:
			a.runProjectBackupJob(ctx, id)
		}
	}
}
func (a *API) runDueProjectBackups(ctx context.Context) {
	now := time.Now().UTC()
	items, e := a.store.DueProjectBackupSchedules(ctx, now)
	if e != nil {
		a.log.Warn("read project backup schedules", "error", e)
		return
	}
	for _, v := range items {
		p, e := a.store.Project(ctx, v.ProjectID)
		if e != nil {
			continue
		}
		n, e := nextServerBackupRun(store.ServerBackupSchedule{Frequency: v.Frequency, Weekday: v.Weekday, Hour: v.Hour, Minute: v.Minute, Timezone: v.Timezone}, now)
		if e != nil {
			continue
		}
		job := store.ProjectBackupJob{ID: newID("pbk"), ProjectID: p.ID, ProjectName: p.Name, ObjectStorageID: v.ObjectStorageID, ObjectStorageName: v.ObjectStorageName, CreatedAt: now}
		ok, e := a.store.QueueDueProjectBackup(ctx, v, n, job)
		if e == nil && ok {
			a.enqueueProjectBackupJob(job.ID)
		}
	}
}
func (a *API) runProjectBackupJob(parent context.Context, id string) {
	started := time.Now().UTC()
	ok, e := a.store.ClaimProjectBackupJob(parent, id, started)
	if e != nil || !ok {
		return
	}
	job, e := a.store.ProjectBackupJob(parent, id)
	if e != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Hour)
	var result backupExecutionResult
	if job.Kind == "restore" {
		result, e = a.executeProjectRestore(ctx, job)
	} else {
		result, e = a.executeProjectBackup(ctx, job)
	}
	cancel()
	status := "succeeded"
	message := result.message
	if e != nil {
		status = "failed"
		message = e.Error()
		a.log.Error("project backup job failed", "job", id, "error", e)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer finishCancel()
	_ = a.store.FinishProjectBackupJob(finishCtx, id, status, message, result.objectKey, result.filename, result.sizeBytes, time.Now().UTC())
	if job.Trigger == "scheduled" {
		_ = a.store.FinishProjectBackupSchedule(finishCtx, job.ProjectID, status, message)
	}
	if e == nil && job.Kind == "backup" {
		a.applyProjectBackupRetention(finishCtx, job.ProjectID, job.ObjectStorageID)
	}
}

func (a *API) projectBackupData(ctx context.Context, projectID string) (store.ProjectBackupData, error) {
	var d store.ProjectBackupData
	var e error
	if d.Project, e = a.store.Project(ctx, projectID); e != nil {
		return d, e
	}
	if d.Applications, e = a.store.ApplicationServices(ctx, projectID); e != nil {
		return d, e
	}
	if d.Databases, e = a.store.ProjectDatabaseServices(ctx, projectID); e != nil {
		return d, e
	}
	if d.Environment, e = a.store.ProjectEnvironmentVariables(ctx, projectID); e != nil {
		return d, e
	}
	d.Domains, e = a.store.ProjectDomainBindings(ctx, projectID)
	return d, e
}
func projectBackupManifestFor(d store.ProjectBackupData) projectBackupManifest {
	m := projectBackupManifest{FormatVersion: projectBackupFormatVersion, Type: "dokyr-project", CreatedAt: time.Now().UTC(), DokyrVersion: version.Current().Version, ProjectID: d.Project.ID, ProjectName: d.Project.Name, Data: d, Secrets: projectBackupSecrets{ApplicationEnvironment: map[string]string{}, ApplicationSecretKeys: map[string][]string{}, ApplicationWebhooks: map[string]string{}, DatabasePasswords: map[string]string{}}}
	for _, v := range d.Environment {
		m.Secrets.Environment = append(m.Secrets.Environment, v.ValueEncrypted)
	}
	for _, v := range d.Applications {
		m.Secrets.ApplicationEnvironment[v.ID] = v.EnvironmentEncrypted
		m.Secrets.ApplicationSecretKeys[v.ID] = v.EnvironmentSecretKeys
		m.Secrets.ApplicationWebhooks[v.ID] = v.RegistryWebhookSecret
	}
	for _, v := range d.Databases {
		m.Secrets.DatabasePasswords[v.ID] = v.PasswordEncrypted
		m.Volumes = append(m.Volumes, projectBackupVolume{ServiceID: v.ID, ServiceName: v.Name, Engine: v.Engine, VolumeName: v.VolumeName, File: "volumes/" + v.ID + ".tar"})
	}
	return m
}

func (a *API) executeProjectBackup(ctx context.Context, job store.ProjectBackupJob) (backupExecutionResult, error) {
	connection, client, e := a.serverBackupStorage(ctx, job.ObjectStorageID)
	if e != nil {
		return backupExecutionResult{}, e
	}
	data, e := a.projectBackupData(ctx, job.ProjectID)
	if e != nil {
		return backupExecutionResult{}, e
	}
	m := projectBackupManifestFor(data)
	temp, e := os.MkdirTemp("", "dokyr-project-backup-")
	if e != nil {
		return backupExecutionResult{}, e
	}
	defer os.RemoveAll(temp)
	for _, db := range data.Databases {
		path := filepath.Join(temp, db.ID+".tar")
		file, e := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if e != nil {
			return backupExecutionResult{}, e
		}
		wasRunning := true
		if e = a.docker.StopDatabase(ctx, db.ID); errors.Is(e, runtime.ErrNotFound) {
			wasRunning = false
			e = nil
		}
		if e == nil {
			e = a.docker.ExportDatabaseVolume(ctx, db.ID, db.Engine, file)
		}
		file.Close()
		if wasRunning {
			_ = a.docker.RestartDatabase(context.Background(), db.ID)
		}
		if e != nil {
			return backupExecutionResult{}, fmt.Errorf("archive %s volume: %w", db.Name, e)
		}
	}
	filename := "dokyr-project-" + safeBackupName(data.Project.Name) + "-" + m.CreatedAt.Format("20060102T150405Z") + ".tar.gz"
	archive := filepath.Join(temp, filename)
	if e = createProjectBackupArchive(archive, temp, m); e != nil {
		return backupExecutionResult{}, e
	}
	info, e := os.Stat(archive)
	if e != nil {
		return backupExecutionResult{}, e
	}
	key := "dokyr/projects/" + job.ProjectID + "/" + m.CreatedAt.Format("2006/01") + "/" + filename
	if e = client.PutFile(ctx, key, archive); e != nil {
		return backupExecutionResult{}, fmt.Errorf("upload backup to %s: %w", connection.Name, e)
	}
	return backupExecutionResult{message: fmt.Sprintf("Project backup uploaded with %d volume(s)", len(m.Volumes)), objectKey: key, filename: filename, sizeBytes: info.Size()}, nil
}
func safeBackupName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	for _, r := range v {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	return out
}

func (a *API) executeProjectRestore(ctx context.Context, job store.ProjectBackupJob) (backupExecutionResult, error) {
	connection, client, e := a.serverBackupStorage(ctx, job.ObjectStorageID)
	if e != nil {
		return backupExecutionResult{}, e
	}
	temp, e := os.MkdirTemp("", "dokyr-project-restore-")
	if e != nil {
		return backupExecutionResult{}, e
	}
	defer os.RemoveAll(temp)
	archive := filepath.Join(temp, "backup.tar.gz")
	if e = client.GetFile(ctx, job.ObjectKey, archive); e != nil {
		return backupExecutionResult{}, fmt.Errorf("download backup from %s: %w", connection.Name, e)
	}
	m, e := extractProjectBackupArchive(archive, temp)
	if e != nil {
		return backupExecutionResult{}, e
	}
	if m.ProjectID != job.ProjectID {
		return backupExecutionResult{}, errors.New("backup belongs to a different project")
	}
	currentApps, _ := a.store.ApplicationServices(ctx, job.ProjectID)
	currentDBs, _ := a.store.ProjectDatabaseServices(ctx, job.ProjectID)
	for _, v := range currentApps {
		_ = a.docker.RemoveApplication(ctx, v.ID)
	}
	for _, v := range currentDBs {
		_ = a.docker.RemoveDatabase(ctx, v.ID, v.VolumeName, true)
	}
	if m.Data.Project.SourceType != "empty" {
		_ = a.docker.StopProject(ctx, job.ProjectID)
	}
	hydrateProjectBackupSecrets(&m)
	if e = a.store.RestoreProjectBackupData(ctx, m.Data); e != nil {
		return backupExecutionResult{}, fmt.Errorf("restore project configuration: %w", e)
	}
	for _, db := range m.Data.Databases {
		password, e := a.box.Decrypt(db.PasswordEncrypted)
		if e != nil {
			return backupExecutionResult{}, fmt.Errorf("decrypt %s credentials: %w", db.Name, e)
		}
		if e = a.docker.RemoveDatabase(ctx, db.ID, db.VolumeName, true); e != nil {
			return backupExecutionResult{}, fmt.Errorf("clear %s before restore: %w", db.Name, e)
		}
		_, e = a.docker.DeployDatabase(ctx, runtime.DatabaseSpec{ID: db.ID, ProjectID: job.ProjectID, Engine: db.Engine, Image: db.Image, Port: db.InternalPort, VolumeName: db.VolumeName, Username: db.Username, DatabaseName: db.DatabaseName, Password: password, PublicEnabled: db.PublicEnabled, PublicPort: db.PublicPort})
		if e != nil {
			return backupExecutionResult{}, fmt.Errorf("prepare %s for restore: %w", db.Name, e)
		}
		if e = a.docker.StopDatabase(ctx, db.ID); e != nil {
			return backupExecutionResult{}, e
		}
		volume, e := os.Open(filepath.Join(temp, db.ID+".tar"))
		if e != nil {
			return backupExecutionResult{}, e
		}
		e = a.docker.RestoreDatabaseVolume(ctx, db.ID, db.Engine, volume)
		volume.Close()
		if e != nil {
			return backupExecutionResult{}, fmt.Errorf("restore %s volume: %w", db.Name, e)
		}
		if e = a.docker.RestartDatabase(ctx, db.ID); e != nil {
			return backupExecutionResult{}, e
		}
	}
	if e = a.SyncDomains(ctx); e != nil {
		a.log.Warn("synchronize domains after project restore", "error", e)
	}
	queuedApplications := 0
	for _, application := range m.Data.Applications {
		if _, _, deployErr := a.startApplicationServiceDeployment(context.Background(), application.ID, "Redeploy after project restore", ""); deployErr != nil {
			a.log.Warn("redeploy application after project restore", "service", application.ID, "error", deployErr)
		} else {
			queuedApplications++
		}
	}
	return backupExecutionResult{message: fmt.Sprintf("Project configuration and %d volume(s) restored; %d application deployment(s) queued from %s", len(m.Volumes), queuedApplications, job.Filename)}, nil
}
func hydrateProjectBackupSecrets(m *projectBackupManifest) {
	for i := range m.Data.Environment {
		if i < len(m.Secrets.Environment) {
			m.Data.Environment[i].ValueEncrypted = m.Secrets.Environment[i]
		}
	}
	for i := range m.Data.Applications {
		id := m.Data.Applications[i].ID
		m.Data.Applications[i].EnvironmentEncrypted = m.Secrets.ApplicationEnvironment[id]
		m.Data.Applications[i].EnvironmentSecretKeys = m.Secrets.ApplicationSecretKeys[id]
		m.Data.Applications[i].RegistryWebhookSecret = m.Secrets.ApplicationWebhooks[id]
	}
	for i := range m.Data.Databases {
		m.Data.Databases[i].PasswordEncrypted = m.Secrets.DatabasePasswords[m.Data.Databases[i].ID]
	}
}

func createProjectBackupArchive(filename, temp string, m projectBackupManifest) error {
	file, e := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if e != nil {
		return e
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	data, e := json.MarshalIndent(m, "", "  ")
	if e == nil {
		e = writeTarBytes(tw, "manifest.json", data)
	}
	for _, v := range m.Volumes {
		if e == nil {
			e = writeTarFile(tw, v.File, filepath.Join(temp, v.ServiceID+".tar"))
		}
	}
	if ce := tw.Close(); e == nil {
		e = ce
	}
	if ce := gz.Close(); e == nil {
		e = ce
	}
	if ce := file.Close(); e == nil {
		e = ce
	}
	return e
}
func extractProjectBackupArchive(filename, temp string) (projectBackupManifest, error) {
	var m projectBackupManifest
	file, e := os.Open(filename)
	if e != nil {
		return m, e
	}
	defer file.Close()
	gz, e := gzip.NewReader(file)
	if e != nil {
		return m, errors.New("backup is not a valid .tar.gz archive")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	volumeFiles := map[string]bool{}
	for {
		h, e := tr.Next()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return m, e
		}
		if h.Size < 0 || h.Size > 1<<40 {
			return m, errors.New("backup entry is invalid")
		}
		if h.Name == "manifest.json" {
			if h.Size > 10<<20 {
				return m, errors.New("backup manifest is too large")
			}
			if e = json.NewDecoder(io.LimitReader(tr, h.Size)).Decode(&m); e != nil {
				return m, errors.New("backup manifest is invalid")
			}
			continue
		}
		if strings.HasPrefix(h.Name, "volumes/") && strings.HasSuffix(h.Name, ".tar") {
			id := strings.TrimSuffix(strings.TrimPrefix(h.Name, "volumes/"), ".tar")
			if id == "" || strings.ContainsAny(id, "/\\") {
				return m, errors.New("backup volume path is invalid")
			}
			out, e := os.OpenFile(filepath.Join(temp, id+".tar"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if e != nil {
				return m, e
			}
			_, e = io.CopyN(out, tr, h.Size)
			ce := out.Close()
			if e != nil {
				return m, e
			}
			if ce != nil {
				return m, ce
			}
			volumeFiles[id] = true
		}
	}
	if m.Type != "dokyr-project" || m.FormatVersion != projectBackupFormatVersion || m.ProjectID == "" {
		return m, errors.New("backup manifest is missing or incompatible")
	}
	for _, v := range m.Volumes {
		if !volumeFiles[v.ServiceID] {
			return m, fmt.Errorf("backup volume for %s is missing", v.ServiceName)
		}
	}
	return m, nil
}

func (a *API) applyProjectBackupRetention(ctx context.Context, projectID, storageID string) {
	schedule, e := a.store.ProjectBackupSchedule(ctx, projectID)
	if store.NotFound(e) {
		schedule = store.DefaultProjectBackupSchedule(projectID)
	} else if e != nil {
		return
	}
	items, e := a.store.ProjectBackupRetentionCandidates(ctx, projectID, schedule.RetentionCount)
	if e != nil {
		return
	}
	_, client, e := a.serverBackupStorage(ctx, storageID)
	if e != nil {
		return
	}
	for _, v := range items {
		if v.ObjectStorageID != storageID {
			continue
		}
		if e = client.Delete(ctx, v.ObjectKey); e != nil {
			a.log.Warn("delete expired project backup", "backup", v.ID, "error", e)
			continue
		}
		_ = a.store.ForgetProjectBackupObject(ctx, v.ID)
	}
}
