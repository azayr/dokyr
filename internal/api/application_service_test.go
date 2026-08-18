package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

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

func TestCleanApplicationServiceInputAcceptsManagedRegistryImage(t *testing.T) {
	clean, err := cleanApplicationServiceInput(applicationServiceInput{
		Name: "api", SourceType: "image", ImageURL: "team/api:latest", RegistryID: "reg_external",
		InternalRegistry: true, AutoDeploy: true, ContainerPort: 8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !clean.InternalRegistry || !clean.AutoDeploy || clean.RegistryID != "" || clean.ImageURL != "team/api:latest" {
		t.Fatalf("unexpected managed registry input: %#v", clean)
	}
}

func TestCleanApplicationServiceInputRejectsUnselectedManagedRegistryImage(t *testing.T) {
	for _, image := range []string{"", "team/api", "registry:5000/team/api:latest", "team/api@sha256:abc"} {
		_, err := cleanApplicationServiceInput(applicationServiceInput{
			Name: "api", SourceType: "image", ImageURL: image, InternalRegistry: true, ContainerPort: 8080,
		})
		if err == nil {
			t.Fatalf("expected managed registry image %q to be rejected", image)
		}
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
		BuildStrategy:  "auto",
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

func TestCleanApplicationServiceInputAcceptsOnlyDockerfileOrAuto(t *testing.T) {
	base := applicationServiceInput{
		Name: "api", SourceType: "repository", ConnectionID: "src_test",
		Repository: "owner/api", Branch: "main", ContainerPort: 8080,
	}
	for _, strategy := range []string{"dockerfile", "auto"} {
		input := base
		input.BuildStrategy = strategy
		if _, err := cleanApplicationServiceInput(input); err != nil {
			t.Fatalf("strategy %q was rejected: %v", strategy, err)
		}
	}
	for _, strategy := range []string{"railpack", "legacy-builder"} {
		input := base
		input.BuildStrategy = strategy
		if _, err := cleanApplicationServiceInput(input); err == nil {
			t.Fatalf("strategy %q should be rejected", strategy)
		}
	}
}

func TestCleanApplicationCommandInput(t *testing.T) {
	clean, err := cleanApplicationCommandInput(applicationCommandInput{Command: "  php artisan about  ", WorkingDir: " /app "})
	if err != nil {
		t.Fatal(err)
	}
	if clean.Command != "php artisan about" || clean.WorkingDir != "/app" {
		t.Fatalf("unexpected cleaned command input: %#v", clean)
	}
	for _, input := range []applicationCommandInput{
		{},
		{Command: "pwd", WorkingDir: "app"},
		{Command: "pwd", WorkingDir: "/app\n/tmp"},
		{Command: "echo \x00"},
	} {
		if _, err := cleanApplicationCommandInput(input); err == nil {
			t.Fatalf("expected input to be rejected: %#v", input)
		}
	}
}

func TestContainerCommandRoles(t *testing.T) {
	for _, role := range []string{"owner", "admin", "developer"} {
		if !canExecuteContainerCommands(role) {
			t.Fatalf("expected %q to execute container commands", role)
		}
	}
	for _, role := range []string{"", "viewer"} {
		if canExecuteContainerCommands(role) {
			t.Fatalf("expected %q to be denied container commands", role)
		}
	}
}

func TestDeploymentCancellationCanOnlyBeClaimedOnce(t *testing.T) {
	api := &API{}
	ctx, cancel := context.WithCancel(context.Background())
	api.registerDeployment("dep_test", cancel)

	claimed, ok := api.claimDeploymentCancellation("dep_test")
	if !ok {
		t.Fatal("expected registered deployment cancellation")
	}
	claimed()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("context error = %v, want context.Canceled", ctx.Err())
	}
	if _, ok := api.claimDeploymentCancellation("dep_test"); ok {
		t.Fatal("expected cancellation to be removed after it was claimed")
	}
}

func TestUnregisterDeploymentRemovesCancellation(t *testing.T) {
	api := &API{}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	api.registerDeployment("dep_test", cancel)
	api.unregisterDeployment("dep_test")

	if _, ok := api.claimDeploymentCancellation("dep_test"); ok {
		t.Fatal("expected completed deployment cancellation to be unregistered")
	}
}

func TestDeploymentCancelledRecognizesContextAndWrappedErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !deploymentCancelled(ctx, errors.New("docker request failed")) {
		t.Fatal("expected cancelled context to be recognized")
	}
	if !deploymentCancelled(context.Background(), fmt.Errorf("pull image: %w", context.Canceled)) {
		t.Fatal("expected wrapped context.Canceled to be recognized")
	}
	if deploymentCancelled(context.Background(), context.DeadlineExceeded) {
		t.Fatal("deadline exceeded must remain a failed deployment")
	}
}
