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
	"github.com/azayr/selfhost/internal/s3"
	"github.com/azayr/selfhost/internal/store"
	"github.com/azayr/selfhost/internal/version"
)

const (
	serverBackupSchedulerInterval = 30 * time.Second
	serverBackupFormatVersion     = 1
)

type serverBackupScheduleInput struct {
	Enabled         bool   `json:"enabled"`
	ObjectStorageID string `json:"objectStorageId"`
	Frequency       string `json:"frequency"`
	Weekday         int    `json:"weekday"`
	Hour            int    `json:"hour"`
	Minute          int    `json:"minute"`
	Timezone        string `json:"timezone"`
}

type createServerBackupInput struct {
	ObjectStorageID string `json:"objectStorageId"`
}

type restoreServerBackupInput struct {
	Confirmation string `json:"confirmation"`
}

type serverBackupManifest struct {
	FormatVersion         int       `json:"formatVersion"`
	Type                  string    `json:"type"`
	CreatedAt             time.Time `json:"createdAt"`
	DokyrVersion          string    `json:"dokyrVersion"`
	DatabaseFile          string    `json:"databaseFile"`
	DatabaseFormat        string    `json:"databaseFormat"`
	Includes              []string  `json:"includes"`
	EncryptionKeyRequired bool      `json:"encryptionKeyRequired"`
}

func (a *API) serverBackups(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.store.ServerBackupJobs(r.Context(), 50)
	if err != nil {
		problem(w, err)
		return
	}
	schedule, err := a.store.ServerBackupSchedule(r.Context())
	if store.NotFound(err) {
		schedule = store.DefaultServerBackupSchedule()
	} else if err != nil {
		problem(w, err)
		return
	}
	connections, err := a.store.ObjectStorageConnections(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	destinations := make([]map[string]any, 0, len(connections))
	for _, connection := range connections {
		destinations = append(destinations, map[string]any{
			"id":       connection.ID,
			"name":     connection.Name,
			"provider": connection.Provider,
			"bucket":   connection.Bucket,
			"region":   connection.Region,
		})
	}
	write(w, http.StatusOK, map[string]any{
		"jobs":         jobs,
		"schedule":     schedule,
		"destinations": destinations,
	})
}

func (a *API) createServerBackup(w http.ResponseWriter, r *http.Request) {
	var input createServerBackupInput
	if !decode(w, r, &input) {
		return
	}
	connection, err := a.store.ObjectStorageConnection(r.Context(), strings.TrimSpace(input.ObjectStorageID))
	if store.NotFound(err) {
		bad(w, "select an available object storage connection")
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	job := store.ServerBackupJob{
		ID:                newID("bkp"),
		Kind:              "backup",
		Status:            "queued",
		ObjectStorageID:   connection.ID,
		ObjectStorageName: connection.Name,
		Trigger:           "manual",
		CreatedBy:         claims.Subject,
		CreatedAt:         time.Now().UTC(),
	}
	if err := a.store.CreateServerBackupJob(r.Context(), job); err != nil {
		problem(w, err)
		return
	}
	a.enqueueServerBackupJob(job.ID)
	write(w, http.StatusAccepted, map[string]any{"job": job})
}

func (a *API) updateServerBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var input serverBackupScheduleInput
	if !decode(w, r, &input) {
		return
	}
	schedule, err := cleanServerBackupSchedule(input, time.Now().UTC())
	if err != nil {
		bad(w, err.Error())
		return
	}
	if _, err := a.store.ObjectStorageConnection(r.Context(), schedule.ObjectStorageID); store.NotFound(err) {
		bad(w, "select an available object storage connection")
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	if err := a.store.UpsertServerBackupSchedule(r.Context(), schedule); err != nil {
		problem(w, err)
		return
	}
	schedule, err = a.store.ServerBackupSchedule(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, schedule)
}

func (a *API) restoreServerBackup(w http.ResponseWriter, r *http.Request) {
	var input restoreServerBackupInput
	if !decode(w, r, &input) {
		return
	}
	if input.Confirmation != "RESTORE SERVER" {
		bad(w, `type "RESTORE SERVER" to confirm`)
		return
	}
	source, err := a.store.ServerBackupJob(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "backup not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if source.Kind != "backup" || source.Status != "succeeded" || source.ObjectKey == "" {
		bad(w, "only a completed backup can be restored")
		return
	}
	if _, err := a.store.ObjectStorageConnection(r.Context(), source.ObjectStorageID); store.NotFound(err) {
		bad(w, "the backup's object storage connection is no longer available")
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	job := store.ServerBackupJob{
		ID:                newID("rst"),
		Kind:              "restore",
		Status:            "queued",
		ObjectStorageID:   source.ObjectStorageID,
		ObjectStorageName: source.ObjectStorageName,
		ObjectKey:         source.ObjectKey,
		Filename:          source.Filename,
		SizeBytes:         source.SizeBytes,
		SourceJobID:       source.ID,
		Trigger:           "restore",
		CreatedBy:         claims.Subject,
		CreatedAt:         time.Now().UTC(),
	}
	if err := a.store.CreateServerBackupJob(r.Context(), job); err != nil {
		problem(w, err)
		return
	}
	a.enqueueServerBackupJob(job.ID)
	write(w, http.StatusAccepted, map[string]any{"job": job})
}

func cleanServerBackupSchedule(input serverBackupScheduleInput, now time.Time) (store.ServerBackupSchedule, error) {
	schedule := store.ServerBackupSchedule{
		Configured:      true,
		Enabled:         input.Enabled,
		ObjectStorageID: strings.TrimSpace(input.ObjectStorageID),
		Frequency:       strings.ToLower(strings.TrimSpace(input.Frequency)),
		Weekday:         input.Weekday,
		Hour:            input.Hour,
		Minute:          input.Minute,
		Timezone:        strings.TrimSpace(input.Timezone),
	}
	if schedule.ObjectStorageID == "" {
		return schedule, errors.New("select an object storage connection")
	}
	if schedule.Frequency != "daily" && schedule.Frequency != "weekly" {
		return schedule, errors.New("frequency must be daily or weekly")
	}
	if schedule.Weekday < 0 || schedule.Weekday > 6 {
		return schedule, errors.New("weekday must be between 0 and 6")
	}
	if schedule.Hour < 0 || schedule.Hour > 23 || schedule.Minute < 0 || schedule.Minute > 59 {
		return schedule, errors.New("enter a valid backup time")
	}
	if schedule.Timezone == "" {
		return schedule, errors.New("timezone is required")
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return schedule, errors.New("timezone is not recognized")
	}
	if schedule.Enabled {
		next, err := nextServerBackupRun(schedule, now)
		if err != nil {
			return schedule, err
		}
		schedule.NextRunAt = &next
	}
	return schedule, nil
}

func nextServerBackupRun(schedule store.ServerBackupSchedule, after time.Time) (time.Time, error) {
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load backup timezone: %w", err)
	}
	localAfter := after.In(location)
	candidate := time.Date(localAfter.Year(), localAfter.Month(), localAfter.Day(), schedule.Hour, schedule.Minute, 0, 0, location)
	if schedule.Frequency == "weekly" {
		daysAhead := (schedule.Weekday - int(localAfter.Weekday()) + 7) % 7
		candidate = candidate.AddDate(0, 0, daysAhead)
		if !candidate.After(localAfter) {
			candidate = candidate.AddDate(0, 0, 7)
		}
	} else if !candidate.After(localAfter) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate.UTC(), nil
}

func (a *API) StartServerBackupWorker(ctx context.Context) {
	if err := a.store.FailInterruptedServerBackupJobs(ctx); err != nil {
		a.log.Warn("mark interrupted server backup jobs failed", "error", err)
	}
	go a.serverBackupWorker(ctx)
	a.enqueuePendingServerBackupJobs(ctx)
	a.runDueServerBackup(ctx)
	go func() {
		ticker := time.NewTicker(serverBackupSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.enqueuePendingServerBackupJobs(ctx)
				a.runDueServerBackup(ctx)
			}
		}
	}()
}

func (a *API) enqueuePendingServerBackupJobs(ctx context.Context) {
	ids, err := a.store.PendingServerBackupJobIDs(ctx)
	if err != nil {
		a.log.Warn("read queued server backup jobs", "error", err)
		return
	}
	for _, id := range ids {
		a.enqueueServerBackupJob(id)
	}
}

func (a *API) enqueueServerBackupJob(id string) {
	select {
	case a.backupQueue <- id:
	default:
		a.log.Info("server backup queue is full; persisted job will be retried", "job", id)
	}
}

func (a *API) serverBackupWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-a.backupQueue:
			a.runServerBackupJob(ctx, id)
		}
	}
}

func (a *API) runDueServerBackup(ctx context.Context) {
	schedule, err := a.store.ServerBackupSchedule(ctx)
	if store.NotFound(err) {
		return
	}
	if err != nil {
		a.log.Warn("read server backup schedule", "error", err)
		return
	}
	now := time.Now().UTC()
	if !schedule.Enabled || schedule.NextRunAt == nil || schedule.NextRunAt.After(now) {
		return
	}
	nextRunAt, err := nextServerBackupRun(schedule, now)
	if err != nil {
		a.log.Error("calculate next server backup", "error", err)
		return
	}
	job := store.ServerBackupJob{
		ID:                newID("bkp"),
		Kind:              "backup",
		Status:            "queued",
		ObjectStorageID:   schedule.ObjectStorageID,
		ObjectStorageName: schedule.ObjectStorageName,
		Trigger:           "scheduled",
		CreatedAt:         now,
	}
	queued, err := a.store.QueueDueServerBackup(ctx, schedule.UpdatedAt, now, nextRunAt, job)
	if err != nil {
		a.log.Error("queue scheduled server backup", "error", err)
		return
	}
	if queued {
		a.enqueueServerBackupJob(job.ID)
	}
}

func (a *API) runServerBackupJob(parent context.Context, id string) {
	startedAt := time.Now().UTC()
	claimed, err := a.store.ClaimServerBackupJob(parent, id, startedAt)
	if err != nil || !claimed {
		if err != nil {
			a.log.Error("claim server backup job", "job", id, "error", err)
		}
		return
	}
	job, err := a.store.ServerBackupJob(parent, id)
	if err != nil {
		a.log.Error("read claimed server backup job", "job", id, "error", err)
		return
	}
	if job.Trigger == "scheduled" {
		_ = a.store.MarkServerBackupScheduleRunning(parent)
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Hour)
	var result backupExecutionResult
	if job.Kind == "restore" {
		result, err = a.executeServerRestore(ctx, job)
	} else {
		result, err = a.executeServerBackup(ctx, job)
	}
	cancel()

	finishedAt := time.Now().UTC()
	status := "succeeded"
	message := result.message
	if err != nil {
		status = "failed"
		message = err.Error()
		a.log.Error("server backup job failed", "job", id, "kind", job.Kind, "error", err)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer finishCancel()
	if job.Kind == "restore" && err == nil {
		job.StartedAt = &startedAt
		if finishErr := a.store.CompleteRestoredServerBackupJob(finishCtx, job, message, finishedAt); finishErr != nil {
			a.log.Error("record completed server restore", "job", id, "error", finishErr)
		}
	} else if finishErr := a.store.FinishServerBackupJob(finishCtx, id, status, message, result.objectKey, result.filename, result.sizeBytes, finishedAt); finishErr != nil {
		a.log.Error("record server backup result", "job", id, "error", finishErr)
	}
	if job.Trigger == "scheduled" {
		if finishErr := a.store.FinishServerBackupSchedule(finishCtx, status, message); finishErr != nil {
			a.log.Error("record scheduled server backup result", "error", finishErr)
		}
	}
}

type backupExecutionResult struct {
	message   string
	objectKey string
	filename  string
	sizeBytes int64
}

func (a *API) executeServerBackup(ctx context.Context, job store.ServerBackupJob) (backupExecutionResult, error) {
	connection, client, err := a.serverBackupStorage(ctx, job.ObjectStorageID)
	if err != nil {
		return backupExecutionResult{}, err
	}
	tempDir, err := os.MkdirTemp("", "dokyr-server-backup-")
	if err != nil {
		return backupExecutionResult{}, err
	}
	defer os.RemoveAll(tempDir)
	databasePath := filepath.Join(tempDir, "database.sql")
	database, err := os.OpenFile(databasePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return backupExecutionResult{}, err
	}
	if err := a.docker.ExportControlPlaneDatabase(ctx, database); err != nil {
		database.Close()
		return backupExecutionResult{}, err
	}
	if err := database.Close(); err != nil {
		return backupExecutionResult{}, err
	}

	createdAt := time.Now().UTC()
	filename := "dokyr-server-" + createdAt.Format("20060102T150405Z") + ".tar.gz"
	archivePath := filepath.Join(tempDir, filename)
	manifest := serverBackupManifest{
		FormatVersion:         serverBackupFormatVersion,
		Type:                  "dokyr-server",
		CreatedAt:             createdAt,
		DokyrVersion:          version.Current().Version,
		DatabaseFile:          "database.sql",
		DatabaseFormat:        "PostgreSQL plain SQL",
		Includes:              []string{"project configurations", "Dokyr configuration", "Dokyr PostgreSQL database"},
		EncryptionKeyRequired: true,
	}
	if err := createServerBackupArchive(archivePath, databasePath, manifest); err != nil {
		return backupExecutionResult{}, err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return backupExecutionResult{}, err
	}
	objectKey := "dokyr/server/" + createdAt.Format("2006/01") + "/" + filename
	if err := client.PutFile(ctx, objectKey, archivePath); err != nil {
		return backupExecutionResult{}, fmt.Errorf("upload backup to %s: %w", connection.Name, err)
	}
	return backupExecutionResult{
		message:   "Backup uploaded to " + connection.Name,
		objectKey: objectKey,
		filename:  filename,
		sizeBytes: info.Size(),
	}, nil
}

func (a *API) executeServerRestore(ctx context.Context, job store.ServerBackupJob) (backupExecutionResult, error) {
	connection, client, err := a.serverBackupStorage(ctx, job.ObjectStorageID)
	if err != nil {
		return backupExecutionResult{}, err
	}
	tempDir, err := os.MkdirTemp("", "dokyr-server-restore-")
	if err != nil {
		return backupExecutionResult{}, err
	}
	defer os.RemoveAll(tempDir)
	archivePath := filepath.Join(tempDir, "backup.tar.gz")
	if err := client.GetFile(ctx, job.ObjectKey, archivePath); err != nil {
		return backupExecutionResult{}, fmt.Errorf("download backup from %s: %w", connection.Name, err)
	}
	databasePath := filepath.Join(tempDir, "database.sql")
	if _, err := extractServerBackupArchive(archivePath, databasePath); err != nil {
		return backupExecutionResult{}, err
	}
	database, err := os.Open(databasePath)
	if err != nil {
		return backupExecutionResult{}, err
	}
	info, err := database.Stat()
	if err != nil {
		database.Close()
		return backupExecutionResult{}, err
	}
	if err := a.docker.RestoreControlPlaneDatabase(ctx, database, info.Size()); err != nil {
		database.Close()
		return backupExecutionResult{}, err
	}
	if err := database.Close(); err != nil {
		return backupExecutionResult{}, err
	}
	if err := a.SyncDomains(ctx); err != nil {
		a.log.Warn("synchronize domains after server restore", "error", err)
	}
	return backupExecutionResult{message: "Server configuration and database restored from " + job.Filename}, nil
}

func (a *API) serverBackupStorage(ctx context.Context, id string) (store.ObjectStorageConnection, *s3.Client, error) {
	connection, err := a.store.ObjectStorageConnection(ctx, id)
	if err != nil {
		return connection, nil, fmt.Errorf("load backup destination: %w", err)
	}
	secret, err := a.box.Decrypt(connection.SecretKeyEncrypted)
	if err != nil {
		return connection, nil, fmt.Errorf("decrypt backup destination credentials: %w", err)
	}
	client, err := s3.New(s3.Config{
		Region:         connection.Region,
		Bucket:         connection.Bucket,
		Endpoint:       connection.Endpoint,
		AccessKey:      connection.AccessKey,
		SecretKey:      secret,
		ForcePathStyle: connection.ForcePathStyle,
		Secure:         connection.Secure,
	})
	if err != nil {
		return connection, nil, err
	}
	return connection, client, nil
}

func createServerBackupArchive(archivePath, databasePath string, manifest serverBackupManifest) error {
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		err = writeTarBytes(tarWriter, "manifest.json", manifestBytes)
	}
	if err == nil {
		err = writeTarFile(tarWriter, "database.sql", databasePath)
	}
	if closeErr := tarWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gzipWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	return err
}

func writeTarBytes(writer *tar.Writer, name string, contents []byte) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), ModTime: time.Now().UTC()}); err != nil {
		return err
	}
	_, err := writer.Write(contents)
	return err
}

func writeTarFile(writer *tar.Writer, name, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: time.Now().UTC()}); err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func extractServerBackupArchive(archivePath, databasePath string) (serverBackupManifest, error) {
	var manifest serverBackupManifest
	archive, err := os.Open(archivePath)
	if err != nil {
		return manifest, err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return manifest, errors.New("backup is not a valid .tar.gz archive")
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	foundManifest := false
	foundDatabase := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return manifest, fmt.Errorf("read backup archive: %w", err)
		}
		switch header.Name {
		case "manifest.json":
			if header.Size > 1<<20 {
				return manifest, errors.New("backup manifest is too large")
			}
			if err := json.NewDecoder(io.LimitReader(reader, header.Size)).Decode(&manifest); err != nil {
				return manifest, errors.New("backup manifest is invalid")
			}
			foundManifest = true
		case "database.sql":
			if header.Size <= 0 {
				return manifest, errors.New("backup database dump is empty")
			}
			database, err := os.OpenFile(databasePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return manifest, err
			}
			_, copyErr := io.CopyN(database, reader, header.Size)
			closeErr := database.Close()
			if copyErr != nil {
				return manifest, copyErr
			}
			if closeErr != nil {
				return manifest, closeErr
			}
			foundDatabase = true
		}
	}
	if !foundManifest || manifest.Type != "dokyr-server" || manifest.FormatVersion != serverBackupFormatVersion {
		return manifest, errors.New("backup manifest is missing or incompatible")
	}
	if !foundDatabase || manifest.DatabaseFile != "database.sql" {
		return manifest, errors.New("backup does not contain the Dokyr database")
	}
	return manifest, nil
}
