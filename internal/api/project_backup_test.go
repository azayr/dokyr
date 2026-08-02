package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/store"
)

func TestProjectBackupArchiveRoundTrip(t *testing.T) {
	temp := t.TempDir()
	volume := filepath.Join(temp, "db_one.tar")
	if err := os.WriteFile(volume, []byte("portable docker archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := projectBackupManifest{FormatVersion: 1, Type: "dokyr-project", CreatedAt: time.Now(), ProjectID: "prj_one", ProjectName: "One", Data: store.ProjectBackupData{Project: store.Project{ID: "prj_one", Name: "One"}}, Volumes: []projectBackupVolume{{ServiceID: "db_one", ServiceName: "Postgres", Engine: "postgres", File: "volumes/db_one.tar"}}}
	archive := filepath.Join(temp, "project.tar.gz")
	if err := createProjectBackupArchive(archive, temp, manifest); err != nil {
		t.Fatal(err)
	}
	restore := filepath.Join(temp, "restore")
	if err := os.Mkdir(restore, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := extractProjectBackupArchive(archive, restore)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != manifest.ProjectID || got.ProjectName != manifest.ProjectName {
		t.Fatalf("manifest mismatch: %#v", got)
	}
	contents, err := os.ReadFile(filepath.Join(restore, "db_one.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "portable docker archive" {
		t.Fatalf("volume mismatch: %q", contents)
	}
}

func TestCleanProjectBackupScheduleDefaultsRetention(t *testing.T) {
	v, err := cleanProjectBackupSchedule("prj_one", projectBackupScheduleInput{ObjectStorageID: "obj_one", Frequency: "daily", Hour: 2, Timezone: "UTC"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if v.RetentionCount != 7 {
		t.Fatalf("retention = %d, want 7", v.RetentionCount)
	}
}

func TestCleanProjectBackupScheduleRejectsInvalidRetention(t *testing.T) {
	_, err := cleanProjectBackupSchedule("prj_one", projectBackupScheduleInput{ObjectStorageID: "obj_one", Frequency: "daily", Hour: 2, Timezone: "UTC", RetentionCount: 101}, time.Now())
	if err == nil {
		t.Fatal("expected retention validation error")
	}
}
