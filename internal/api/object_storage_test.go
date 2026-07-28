package api

import "testing"

func TestCleanObjectStorageInputSupportsCommonProviders(t *testing.T) {
	for _, provider := range []string{"aws", "r2", "minio", "digitalocean", "custom"} {
		t.Run(provider, func(t *testing.T) {
			endpoint := "https://s3.example.com/"
			if provider == "aws" {
				endpoint = ""
			}
			clean, err := cleanObjectStorageInput(objectStorageInput{
				Name: " Production ", Provider: provider, Region: "us-east-1",
				Bucket: "images", Endpoint: endpoint, AccessKey: "access",
				SecretKey: "secret", Secure: true,
			})
			if err != nil {
				t.Fatalf("clean input: %v", err)
			}
			if clean.Name != "Production" || clean.Endpoint == "https://s3.example.com/" {
				t.Fatalf("input was not normalized: %#v", clean)
			}
		})
	}
}

func TestCleanObjectStorageInputRejectsEndpointPaths(t *testing.T) {
	_, err := cleanObjectStorageInput(objectStorageInput{
		Name: "MinIO", Provider: "minio", Region: "us-east-1", Bucket: "images",
		Endpoint: "https://minio.example.com/s3", AccessKey: "access", SecretKey: "secret",
	})
	if err == nil {
		t.Fatal("expected an endpoint path to be rejected")
	}
}

func TestInferObjectStorageProvider(t *testing.T) {
	tests := map[string]struct {
		endpoint string
		force    bool
		want     string
	}{
		"aws":          {"", false, "aws"},
		"r2":           {"https://account.r2.cloudflarestorage.com", true, "r2"},
		"digitalocean": {"https://nyc3.digitaloceanspaces.com", false, "digitalocean"},
		"minio":        {"https://minio.example.com", true, "minio"},
		"custom":       {"https://objects.example.com", false, "custom"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := inferObjectStorageProvider(test.endpoint, test.force); got != test.want {
				t.Fatalf("provider = %q, want %q", got, test.want)
			}
		})
	}
}
