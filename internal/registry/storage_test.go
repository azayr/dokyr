package registry

import (
	"strings"
	"testing"

	"github.com/azayr/selfhost/internal/store"
)

func TestEnvironmentUsesDistributionS3RegionEndpoint(t *testing.T) {
	env := environment(store.RegistrySettings{
		Storage:          "s3",
		S3Region:         "us-east-1",
		S3Bucket:         "docker-images",
		S3AccessKey:      "access-key",
		S3Endpoint:       "http://minio:9000",
		S3ForcePathStyle: true,
	}, "secret-key")

	assertRegistryEnvironment(t, env, "REGISTRY_STORAGE_S3_REGIONENDPOINT", "http://minio:9000")
	for _, entry := range env {
		if strings.HasPrefix(entry, "REGISTRY_STORAGE_S3_ENDPOINT=") {
			t.Fatalf("legacy S3 endpoint variable unexpectedly present: %q", entry)
		}
	}
}

func assertRegistryEnvironment(t *testing.T, env []string, key, want string) {
	t.Helper()
	for _, entry := range env {
		gotKey, got, ok := strings.Cut(entry, "=")
		if ok && gotKey == key {
			if got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s is missing from %#v", key, env)
}
