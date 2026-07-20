package api

import "testing"

func TestCleanApplicationServiceInputKeepsValidBuildStrategyForImage(t *testing.T) {
	clean, err := cleanApplicationServiceInput(applicationServiceInput{
		Name:          "nginx",
		SourceType:    "image",
		ImageURL:      "nginx:stable-alpine",
		ContainerPort: 80,
		BuildStrategy: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if clean.BuildStrategy != "dockerfile" {
		t.Fatalf("build strategy = %q, want dockerfile", clean.BuildStrategy)
	}
	if clean.ImageURL != "nginx:stable-alpine" {
		t.Fatalf("image URL = %q, want nginx:stable-alpine", clean.ImageURL)
	}
	if clean.HealthCheckType != "none" || clean.HealthCheckTimeoutSeconds != 60 {
		t.Fatalf("health check defaults = %q/%d, want none/60", clean.HealthCheckType, clean.HealthCheckTimeoutSeconds)
	}
}

func TestCleanApplicationServiceInputNormalizesHTTPHealthCheck(t *testing.T) {
	clean, err := cleanApplicationServiceInput(applicationServiceInput{
		Name: "api", SourceType: "image", ImageURL: "example/api:latest", ContainerPort: 8080,
		HealthCheckType: "http", HealthCheckPath: "", HealthCheckTimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if clean.HealthCheckPath != "/" || clean.HealthCheckCommand != "" || clean.HealthCheckTimeoutSeconds != 30 {
		t.Fatalf("unexpected HTTP health check: %#v", clean)
	}
}

func TestCleanApplicationServiceInputRejectsInvalidHealthCheck(t *testing.T) {
	_, err := cleanApplicationServiceInput(applicationServiceInput{
		Name: "api", SourceType: "image", ImageURL: "example/api:latest", ContainerPort: 8080,
		HealthCheckType: "http", HealthCheckPath: "health check",
	})
	if err == nil {
		t.Fatal("expected invalid health check path error")
	}
	_, err = cleanApplicationServiceInput(applicationServiceInput{
		Name: "api", SourceType: "image", ImageURL: "example/api:latest", ContainerPort: 8080,
		HealthCheckType: "command", HealthCheckCommand: "",
	})
	if err == nil {
		t.Fatal("expected empty health check command error")
	}
}

func TestCleanApplicationServiceInputClearsRepositoryFieldsForImage(t *testing.T) {
	clean, err := cleanApplicationServiceInput(applicationServiceInput{
		Name:           "nginx",
		SourceType:     "image",
		ImageURL:       "nginx:stable-alpine",
		ConnectionID:   "src_stale",
		Repository:     "owner/stale",
		Branch:         "main",
		DockerfilePath: "Dockerfile",
		BuildContext:   ".",
		BuildStrategy:  "railpack",
		ContainerPort:  80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if clean.ConnectionID != "" || clean.Repository != "" || clean.Branch != "" || clean.DockerfilePath != "" || clean.BuildContext != "" {
		t.Fatalf("repository fields were not cleared: %#v", clean)
	}
	if clean.BuildStrategy != "dockerfile" {
		t.Fatalf("build strategy = %q, want dockerfile", clean.BuildStrategy)
	}
}
