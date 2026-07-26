package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/registry"
	"github.com/azayr/selfhost/internal/store"
)

func TestRegistrySettingsInputAcceptsResponseMetadata(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/registry/settings", strings.NewReader(`{
		"storage":"s3",
		"s3Region":"us-east-1",
		"s3Bucket":"docker-images",
		"s3AccessKey":"access-key",
		"s3SecretKey":"secret-key",
		"s3Endpoint":"http://minio:9000",
		"s3ForcePathStyle":true,
		"s3Secure":false,
		"hasS3SecretKey":false,
		"updatedAt":"2026-07-25T22:00:00Z"
	}`))
	recorder := httptest.NewRecorder()
	var input registrySettingsInput

	if !decode(recorder, request, &input) {
		t.Fatalf("expected registry settings response metadata to be accepted, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if input.Storage != "s3" || input.S3Bucket != "docker-images" || !input.S3ForcePathStyle {
		t.Fatalf("unexpected decoded registry settings: %#v", input)
	}
}

func TestRegistrySettingsInputStillRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest("PUT", "/api/registry/settings", strings.NewReader(`{
		"storage":"filesystem",
		"unexpected":true
	}`))
	recorder := httptest.NewRecorder()
	var input registrySettingsInput

	if decode(recorder, request, &input) {
		t.Fatal("expected an unknown registry settings field to be rejected")
	}
	if recorder.Code != 400 {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestRegistryDomainResponseUsesAttachedDomain(t *testing.T) {
	response := registryDomainResponse(store.RegistryDomainSettings{
		Domain:       "registry.example.com",
		HTTPSEnabled: true,
	}, []string{"registry.example.com"})

	if response["attached"] != true || response["domain"] != "registry.example.com" {
		t.Fatalf("unexpected registry domain response: %#v", response)
	}
	hosts, ok := response["registryHosts"].([]string)
	if !ok || len(hosts) != 1 || hosts[0] != "registry.example.com" {
		t.Fatalf("unexpected registry hosts: %#v", response["registryHosts"])
	}
}

func TestAttachRegistryPushTimesUsesLatestMatchingCurrentDigest(t *testing.T) {
	older := time.Date(2026, time.July, 26, 10, 0, 0, 0, time.UTC)
	newer := older.Add(3 * time.Hour)
	repositories := []registry.Repository{{
		Name: "demo-app",
		Images: []registry.Image{{
			Digest: "sha256:current",
			Tags:   []string{"latest", "stable"},
		}},
	}}
	pushes := []store.RegistryImagePush{
		{Repository: "demo-app", Tag: "latest", Digest: "sha256:old", PushedAt: newer.Add(time.Hour)},
		{Repository: "demo-app", Tag: "latest", Digest: "sha256:current", PushedAt: older},
		{Repository: "demo-app", Tag: "stable", Digest: "sha256:current", PushedAt: newer},
	}

	attachRegistryPushTimes(repositories, pushes)

	if repositories[0].Images[0].PushedAt == nil || !repositories[0].Images[0].PushedAt.Equal(newer) {
		t.Fatalf("pushedAt = %v, want %v", repositories[0].Images[0].PushedAt, newer)
	}
}
