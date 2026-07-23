package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/store"
)

const cleanupSchedulerInterval = 30 * time.Second

type cleanupScheduleInput struct {
	Enabled    bool   `json:"enabled"`
	Frequency  string `json:"frequency"`
	Weekday    int    `json:"weekday"`
	Hour       int    `json:"hour"`
	Minute     int    `json:"minute"`
	Timezone   string `json:"timezone"`
	Containers bool   `json:"containers"`
	Images     bool   `json:"images"`
	BuildCache bool   `json:"buildCache"`
	Networks   bool   `json:"networks"`
}

func (a *API) cleanupSchedule(w http.ResponseWriter, r *http.Request) {
	schedule, err := a.store.CleanupSchedule(r.Context())
	if store.NotFound(err) {
		write(w, http.StatusOK, store.DefaultCleanupSchedule())
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, schedule)
}

func (a *API) updateCleanupSchedule(w http.ResponseWriter, r *http.Request) {
	var input cleanupScheduleInput
	if !decode(w, r, &input) {
		return
	}
	schedule, err := cleanCleanupSchedule(input, time.Now().UTC())
	if err != nil {
		bad(w, err.Error())
		return
	}
	if err := a.store.UpsertCleanupSchedule(r.Context(), schedule); err != nil {
		problem(w, err)
		return
	}
	schedule, err = a.store.CleanupSchedule(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	a.log.Info("Docker cleanup schedule updated", "enabled", schedule.Enabled, "frequency", schedule.Frequency, "nextRunAt", schedule.NextRunAt)
	write(w, http.StatusOK, schedule)
}

func cleanCleanupSchedule(input cleanupScheduleInput, now time.Time) (store.CleanupSchedule, error) {
	schedule := store.CleanupSchedule{
		Configured: true,
		Enabled:    input.Enabled,
		Frequency:  strings.ToLower(strings.TrimSpace(input.Frequency)),
		Weekday:    input.Weekday,
		Hour:       input.Hour,
		Minute:     input.Minute,
		Timezone:   strings.TrimSpace(input.Timezone),
		Containers: input.Containers,
		Images:     input.Images,
		BuildCache: input.BuildCache,
		Networks:   input.Networks,
	}
	if schedule.Frequency != "daily" && schedule.Frequency != "weekly" {
		return schedule, fmt.Errorf("frequency must be daily or weekly")
	}
	if schedule.Weekday < 0 || schedule.Weekday > 6 {
		return schedule, fmt.Errorf("weekday must be between 0 and 6")
	}
	if schedule.Hour < 0 || schedule.Hour > 23 || schedule.Minute < 0 || schedule.Minute > 59 {
		return schedule, fmt.Errorf("enter a valid cleanup time")
	}
	if schedule.Timezone == "" {
		return schedule, fmt.Errorf("timezone is required")
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return schedule, fmt.Errorf("timezone is not recognized")
	}
	if schedule.Enabled && !schedule.Containers && !schedule.Images && !schedule.BuildCache && !schedule.Networks {
		return schedule, fmt.Errorf("select at least one resource for automatic cleanup")
	}
	if schedule.Enabled {
		next, err := nextCleanupRun(schedule, now)
		if err != nil {
			return schedule, err
		}
		schedule.NextRunAt = &next
	}
	return schedule, nil
}

func nextCleanupRun(schedule store.CleanupSchedule, after time.Time) (time.Time, error) {
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load cleanup timezone: %w", err)
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

func (a *API) StartCleanupScheduler(ctx context.Context) {
	go func() {
		a.runDueCleanup(ctx)
		ticker := time.NewTicker(cleanupSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.runDueCleanup(ctx)
			}
		}
	}()
}

func (a *API) runDueCleanup(ctx context.Context) {
	schedule, err := a.store.CleanupSchedule(ctx)
	if store.NotFound(err) {
		return
	}
	if err != nil {
		a.log.Warn("read Docker cleanup schedule", "error", err)
		return
	}
	if !schedule.Enabled || schedule.NextRunAt == nil || schedule.NextRunAt.After(time.Now().UTC()) {
		return
	}

	startedAt := time.Now().UTC()
	nextRunAt, err := nextCleanupRun(schedule, startedAt)
	if err != nil {
		a.log.Error("calculate next Docker cleanup", "error", err)
		return
	}
	claimed, err := a.store.ClaimCleanupSchedule(ctx, schedule.UpdatedAt, startedAt, nextRunAt)
	if err != nil {
		a.log.Error("claim Docker cleanup schedule", "error", err)
		return
	}
	if !claimed {
		return
	}

	a.cleanupMu.Lock()
	defer a.cleanupMu.Unlock()

	cleanupContext, cancel := context.WithTimeout(ctx, 30*time.Minute)
	result, cleanupErr := a.docker.Cleanup(cleanupContext, runtime.CleanupOptions{
		Containers: schedule.Containers,
		Images:     schedule.Images,
		BuildCache: schedule.BuildCache,
		Networks:   schedule.Networks,
	})
	cancel()

	status := "succeeded"
	message := fmt.Sprintf("Removed %d resources and reclaimed %d bytes", result.Deleted, result.SpaceReclaimed)
	if cleanupErr != nil {
		status = "failed"
		message = cleanupErr.Error()
		a.log.Error("scheduled Docker cleanup failed", "error", cleanupErr)
	} else {
		a.log.Info("scheduled Docker cleanup completed", "deleted", result.Deleted, "reclaimed", result.SpaceReclaimed)
	}

	finishContext, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err := a.store.FinishCleanupSchedule(finishContext, status, message, result.Deleted, result.SpaceReclaimed); err != nil {
		a.log.Error("record scheduled Docker cleanup result", "error", err)
	}
}
