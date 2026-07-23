package api

import (
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/store"
)

func TestNextCleanupRunDailyUsesConfiguredTimezone(t *testing.T) {
	after := time.Date(2026, time.July, 23, 10, 30, 0, 0, time.UTC)
	next, err := nextCleanupRun(store.CleanupSchedule{
		Frequency: "daily",
		Hour:      3,
		Minute:    15,
		Timezone:  "Africa/Casablanca",
	}, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 24, 2, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next, want)
	}
}

func TestNextCleanupRunWeeklyMovesToNextConfiguredDay(t *testing.T) {
	after := time.Date(2026, time.July, 23, 10, 30, 0, 0, time.UTC)
	next, err := nextCleanupRun(store.CleanupSchedule{
		Frequency: "weekly",
		Weekday:   int(time.Sunday),
		Hour:      3,
		Timezone:  "UTC",
	}, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 26, 3, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next, want)
	}
}

func TestCleanCleanupScheduleRejectsEmptyAutomaticPlan(t *testing.T) {
	_, err := cleanCleanupSchedule(cleanupScheduleInput{
		Enabled:   true,
		Frequency: "daily",
		Hour:      3,
		Timezone:  "UTC",
	}, time.Now())
	if err == nil {
		t.Fatal("expected an empty automatic cleanup plan to be rejected")
	}
}
