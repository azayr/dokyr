package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/store"
)

func TestServerBackupArchiveRoundTrip(t *testing.T) {
	temp := t.TempDir()
	databasePath := filepath.Join(temp, "source.sql")
	const database = "CREATE TABLE example (id integer);\nINSERT INTO example VALUES (1);\n"
	if err := os.WriteFile(databasePath, []byte(database), 0o600); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	manifest := serverBackupManifest{
		FormatVersion:         serverBackupFormatVersion,
		Type:                  "dokyr-server",
		CreatedAt:             createdAt,
		DokyrVersion:          "1.2.3",
		DatabaseFile:          "database.sql",
		DatabaseFormat:        "PostgreSQL plain SQL",
		Includes:              []string{"project configurations", "Dokyr configuration", "Dokyr PostgreSQL database"},
		EncryptionKeyRequired: true,
	}
	archivePath := filepath.Join(temp, "backup.tar.gz")
	if err := createServerBackupArchive(archivePath, databasePath, manifest); err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(temp, "restored.sql")
	got, err := extractServerBackupArchive(archivePath, restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != manifest.Type || got.FormatVersion != manifest.FormatVersion || got.DokyrVersion != manifest.DokyrVersion {
		t.Fatalf("unexpected manifest: %#v", got)
	}
	contents, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != database {
		t.Fatalf("restored database mismatch: %q", contents)
	}
}

func TestExtractServerBackupArchiveRejectsNonGzipInput(t *testing.T) {
	temp := t.TempDir()
	filename := filepath.Join(temp, "not-a-backup.tar.gz")
	if err := os.WriteFile(filename, []byte("not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := extractServerBackupArchive(filename, filepath.Join(temp, "database.sql"))
	if err == nil || !strings.Contains(err.Error(), "valid .tar.gz") {
		t.Fatalf("expected invalid archive error, got %v", err)
	}
}

func TestNextServerBackupRunHonorsWeeklyScheduleTimezone(t *testing.T) {
	schedule := store.ServerBackupSchedule{
		Frequency: "weekly",
		Weekday:   int(time.Sunday),
		Hour:      2,
		Minute:    30,
		Timezone:  "Africa/Casablanca",
	}
	after := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	got, err := nextServerBackupRun(schedule, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 2, 1, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next run = %s, want %s", got, want)
	}
}

func TestCleanServerBackupScheduleRequiresDestination(t *testing.T) {
	_, err := cleanServerBackupSchedule(serverBackupScheduleInput{
		Enabled:   true,
		Frequency: "daily",
		Hour:      2,
		Timezone:  "UTC",
	}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "object storage") {
		t.Fatalf("expected object storage validation error, got %v", err)
	}
}
