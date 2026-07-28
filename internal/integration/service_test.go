package integration

import (
	"errors"
	"net/http"
	"testing"
)

func TestGitHubAppResponseErrorRecognizesRemovedApp(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		if err := githubAppResponseError(status, http.StatusText(status), []byte(`{"message":"Not Found"}`)); !errors.Is(err, errGitHubAppRemoved) {
			t.Fatalf("status %d error = %v, want removed app", status, err)
		}
	}
}

func TestGitHubAppResponseErrorPreservesTemporaryFailures(t *testing.T) {
	if err := githubAppResponseError(http.StatusServiceUnavailable, "503 Service Unavailable", []byte(`{"message":"try later"}`)); err == nil || errors.Is(err, errGitHubAppRemoved) {
		t.Fatalf("error = %v, want temporary provider failure", err)
	}
}

func TestGitHubAppResponseErrorAcceptsSuccess(t *testing.T) {
	if err := githubAppResponseError(http.StatusOK, "200 OK", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubLoginManifestIsPublicAndIdentityOnly(t *testing.T) {
	service := New(nil, nil, Config{PublicURL: "https://dokyr.example"})
	manifest, _, err := service.githubManifest(githubAccountLinkMode, "1234abcd")
	if err != nil {
		t.Fatal(err)
	}
	if manifest["public"] != true {
		t.Fatalf("login App public = %v, want true", manifest["public"])
	}
	if _, exists := manifest["default_permissions"]; exists {
		t.Fatal("login App must not request repository permissions")
	}
	callbacks := manifest["callback_urls"].([]string)
	if len(callbacks) != 1 || callbacks[0] != "https://dokyr.example/api/auth/github/callback" {
		t.Fatalf("login App callbacks = %v", callbacks)
	}
}

func TestGitHubRepositoryManifestRemainsPrivateWithContentsRead(t *testing.T) {
	service := New(nil, nil, Config{PublicURL: "https://dokyr.example"})
	manifest, _, err := service.githubManifest(githubRepositoryInstallMode, "1234abcd")
	if err != nil {
		t.Fatal(err)
	}
	if manifest["public"] != false {
		t.Fatalf("repository App public = %v, want false", manifest["public"])
	}
	permissions := manifest["default_permissions"].(map[string]string)
	if permissions["contents"] != "read" {
		t.Fatalf("repository App contents permission = %q", permissions["contents"])
	}
}
