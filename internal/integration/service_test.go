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
