package api

import (
	"net/http/httptest"
	"strings"
	"testing"
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
