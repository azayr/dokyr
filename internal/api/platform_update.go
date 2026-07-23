package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/platformupdate"
	"github.com/azayr/selfhost/internal/store"
	"github.com/azayr/selfhost/internal/version"
)

const platformUpdateSchedulerInterval = 5 * time.Minute

type platformUpdateSettingsInput struct {
	AutoUpdate           bool   `json:"autoUpdate"`
	CheckIntervalMinutes int    `json:"checkIntervalMinutes"`
	MaintenanceHour      int    `json:"maintenanceHour"`
	Timezone             string `json:"timezone"`
}

type platformUpdateStatus struct {
	Current         version.Info                 `json:"current"`
	CurrentImage    string                       `json:"currentImage,omitempty"`
	CurrentDigest   string                       `json:"currentDigest,omitempty"`
	Latest          *platformupdate.Release      `json:"latest,omitempty"`
	UpdateAvailable bool                         `json:"updateAvailable"`
	UpdateSupported bool                         `json:"updateSupported"`
	Checking        bool                         `json:"checking,omitempty"`
	Error           string                       `json:"error,omitempty"`
	Settings        store.PlatformUpdateSettings `json:"settings"`
	Job             *store.PlatformUpdateJob     `json:"job,omitempty"`
}

func (a *API) platformUpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := a.readPlatformUpdateStatus(r.Context(), r.URL.Query().Get("refresh") == "true")
	write(w, http.StatusOK, status)
}

func (a *API) checkPlatformUpdate(w http.ResponseWriter, r *http.Request) {
	write(w, http.StatusOK, a.readPlatformUpdateStatus(r.Context(), true))
}

func (a *API) updatePlatformUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var input platformUpdateSettingsInput
	if !decode(w, r, &input) {
		return
	}
	settings := store.PlatformUpdateSettings{
		Configured: true, AutoUpdate: input.AutoUpdate,
		CheckIntervalMinutes: input.CheckIntervalMinutes,
		MaintenanceHour:      input.MaintenanceHour, Timezone: strings.TrimSpace(input.Timezone),
	}
	if settings.CheckIntervalMinutes < 15 || settings.CheckIntervalMinutes > 10080 {
		bad(w, "check interval must be between 15 minutes and 7 days")
		return
	}
	if settings.MaintenanceHour < 0 || settings.MaintenanceHour > 23 {
		bad(w, "maintenance hour must be between 0 and 23")
		return
	}
	if settings.Timezone == "" {
		bad(w, "timezone is required")
		return
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		bad(w, "timezone is not recognized")
		return
	}
	if err := a.store.UpsertPlatformUpdateSettings(r.Context(), settings); err != nil {
		problem(w, err)
		return
	}
	saved, err := a.store.PlatformUpdateSettings(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, saved)
}

func (a *API) applyPlatformUpdate(w http.ResponseWriter, r *http.Request) {
	status := a.readPlatformUpdateStatus(r.Context(), true)
	if !status.UpdateSupported {
		bad(w, "self-update is available only for a published Dokyr Docker installation")
		return
	}
	if status.Latest == nil || !status.UpdateAvailable {
		bad(w, "Dokyr is already up to date")
		return
	}
	if status.Job != nil && (status.Job.Status == "pending" || status.Job.Status == "pulling" || status.Job.Status == "restarting") {
		write(w, http.StatusConflict, map[string]string{"error": "a platform update is already running"})
		return
	}
	claims, _ := auth.FromContext(r.Context())
	job, err := a.beginPlatformUpdate(status.Current.Version, *status.Latest, claims.Subject)
	if err != nil {
		if strings.Contains(err.Error(), "platform_update_one_active") {
			write(w, http.StatusConflict, map[string]string{"error": "a platform update is already running"})
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusAccepted, map[string]any{
		"job":     job,
		"message": "Update started. Dokyr will briefly reconnect while the new container is verified.",
	})
}

func (a *API) readPlatformUpdateStatus(ctx context.Context, force bool) platformUpdateStatus {
	status := platformUpdateStatus{Current: version.Current()}
	settings, err := a.store.PlatformUpdateSettings(ctx)
	if store.NotFound(err) {
		settings = store.DefaultPlatformUpdateSettings()
	} else if err != nil {
		status.Error = "Could not read update settings."
	}
	status.Settings = settings
	if job, err := a.store.LatestPlatformUpdateJob(ctx); err == nil {
		status.Job = &job
	}
	if current, err := a.docker.PlatformRuntime(ctx); err == nil {
		status.CurrentImage = current.Image
		status.CurrentDigest = current.Digest
		status.UpdateSupported = !isDevelopmentVersion(status.Current.Version) && current.Digest != ""
	} else {
		status.Error = err.Error()
	}
	if !status.UpdateSupported {
		return status
	}

	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if force || a.latestRelease == nil || time.Since(a.latestRelease.CheckedAt) >= 5*time.Minute {
		release, releaseErr := a.updates.Latest(ctx)
		if releaseErr != nil {
			status.Error = "Could not check the release registry: " + releaseErr.Error()
		} else {
			a.latestRelease = &release
			_ = a.store.MarkPlatformUpdateChecked(ctx, release.CheckedAt)
		}
	}
	status.Latest = a.latestRelease
	if status.Latest != nil && status.UpdateSupported {
		if status.CurrentDigest != "" {
			status.UpdateAvailable = status.CurrentDigest != status.Latest.Digest
		} else {
			status.UpdateAvailable = status.Current.Version != "dev" && status.Current.Version != status.Latest.Version
		}
	}
	return status
}

func isDevelopmentVersion(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "dev" || value == "development" || strings.HasSuffix(value, "-dev")
}

func (a *API) beginPlatformUpdate(sourceVersion string, release platformupdate.Release, requestedBy string) (store.PlatformUpdateJob, error) {
	job := store.PlatformUpdateJob{
		ID: newID("upd"), SourceVersion: sourceVersion, TargetVersion: release.Version,
		TargetImage: release.Image, TargetDigest: release.Digest, Status: "pending",
		Message: "Waiting to pull the release image", RequestedBy: requestedBy,
	}
	if err := a.store.CreatePlatformUpdateJob(context.Background(), job); err != nil {
		return store.PlatformUpdateJob{}, err
	}
	go a.executePlatformUpdate(job)
	return job, nil
}

func (a *API) executePlatformUpdate(job store.PlatformUpdateJob) {
	a.updateExecutionMu.Lock()
	defer a.updateExecutionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	_ = a.store.SetPlatformUpdateJobStatus(ctx, job.ID, "pulling", "Pulling "+job.TargetVersion, false)
	if err := a.docker.PullPlatformImage(ctx, job.TargetImage, nil); err != nil {
		_ = a.store.SetPlatformUpdateJobStatus(context.Background(), job.ID, "failed", "Could not pull the release image: "+err.Error(), true)
		return
	}
	_ = a.store.SetPlatformUpdateJobStatus(ctx, job.ID, "restarting", "Handed off to the rollback-safe update helper", false)
	if err := a.docker.StartPlatformUpdateHelper(ctx, job.ID, job.TargetImage); err != nil {
		_ = a.store.SetPlatformUpdateJobStatus(context.Background(), job.ID, "failed", "Could not start the update helper: "+err.Error(), true)
	}
}

func (a *API) StartPlatformUpdateScheduler(ctx context.Context) {
	go func() {
		a.runAutomaticPlatformUpdate(ctx)
		ticker := time.NewTicker(platformUpdateSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.runAutomaticPlatformUpdate(ctx)
			}
		}
	}()
}

func (a *API) runAutomaticPlatformUpdate(ctx context.Context) {
	settings, err := a.store.PlatformUpdateSettings(ctx)
	if err != nil || !settings.AutoUpdate {
		return
	}
	if settings.LastCheckedAt != nil && time.Since(*settings.LastCheckedAt) < time.Duration(settings.CheckIntervalMinutes)*time.Minute {
		return
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil || time.Now().In(location).Hour() != settings.MaintenanceHour {
		return
	}
	if _, err := a.store.ActivePlatformUpdateJob(ctx); err == nil {
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		return
	}
	status := a.readPlatformUpdateStatus(ctx, true)
	if status.UpdateSupported && status.UpdateAvailable && status.Latest != nil {
		if _, err := a.beginPlatformUpdate(status.Current.Version, *status.Latest, ""); err != nil {
			a.log.Error("start automatic platform update", "error", err)
		}
	}
}
