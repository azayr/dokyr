package integration

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/store"
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

func TestGitHubRepositoryManifestIsPublicWithContentsRead(t *testing.T) {
	service := New(nil, nil, Config{PublicURL: "https://dokyr.example"})
	manifest, _, err := service.githubManifest(githubRepositoryInstallMode, "1234abcd")
	if err != nil {
		t.Fatal(err)
	}
	if manifest["public"] != true {
		t.Fatalf("repository App public = %v, want true", manifest["public"])
	}
	permissions := manifest["default_permissions"].(map[string]string)
	if permissions["contents"] != "read" {
		t.Fatalf("repository App contents permission = %q", permissions["contents"])
	}
}

func TestGiteaTokenExchangeUsesInstanceEndpointAndKeepsRefreshToken(t *testing.T) {
	service := New(nil, nil, Config{PublicURL: "https://dokyr.example", GiteaBaseURL: "http://gitea.local:3000"})
	service.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/login/oauth/access_token" {
			t.Fatalf("token path = %q", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret" || r.Form.Get("code") != "code" {
			t.Fatalf("unexpected token form: %v", r.Form)
		}
		return jsonResponse(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"scope":"read:user read:repository"}`), nil
	})}
	credential, scopes, err := service.exchangeWithCredentials(context.Background(), "gitea", "code", "https://dokyr.example/api/integrations/oauth/gitea/callback", "client", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if credential.AccessToken != "access" || credential.RefreshToken != "refresh" {
		t.Fatalf("unexpected credential: %+v", credential)
	}
	if scopes != "read:user read:repository" {
		t.Fatalf("scopes = %q", scopes)
	}
	if time.Until(credential.ExpiresAt) < 59*time.Minute {
		t.Fatalf("credential expiry = %v", credential.ExpiresAt)
	}
}

func TestGiteaRepositoriesIncludePrivateAccessibleResults(t *testing.T) {
	service := New(nil, nil, Config{GiteaBaseURL: "http://gitea.local:3000"})
	service.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/repos/search" {
			t.Fatalf("repository path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("private") != "true" || r.URL.Query().Get("sort") != "updated" || r.URL.Query().Get("order") != "desc" {
			t.Fatalf("unexpected repository query: %v", r.URL.Query())
		}
		if r.URL.Query().Get("page") != "1" {
			return jsonResponse(`{"data":[]}`), nil
		}
		return jsonResponse(`{"data":[{"id":42,"name":"control","full_name":"platform/control","clone_url":"http://gitea.local:3000/platform/control.git","default_branch":"main","private":true,"updated_at":"2026-08-17T08:00:00Z"}]}`), nil
	})}

	repositories, err := service.giteaRepositories(context.Background(), "http://gitea.local:3000", "access")
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].FullName != "platform/control" || !repositories[0].Private {
		t.Fatalf("unexpected repositories: %+v", repositories)
	}
}

func TestSecureCloneURLAllowsOnlyConfiguredGiteaHTTPOrigin(t *testing.T) {
	connection := store.SourceConnection{Provider: "gitea", BaseURL: "http://192.168.1.20:3000"}
	for raw, want := range map[string]bool{
		"http://192.168.1.20:3000/team/app.git":  true,
		"http://192.168.1.21:3000/team/app.git":  false,
		"https://gitea.example.com/team/app.git": true,
		"ssh://git@192.168.1.20/team/app.git":    false,
	} {
		if got := secureCloneURL(connection, raw); got != want {
			t.Errorf("secureCloneURL(%q) = %v, want %v", raw, got, want)
		}
	}
	if secureCloneURL(store.SourceConnection{Provider: "gitlab", BaseURL: connection.BaseURL}, "http://192.168.1.20:3000/team/app.git") {
		t.Fatal("HTTP clone must remain restricted to explicitly configured Gitea connections")
	}
}

func TestGiteaHTTPClonePinsIPv4ResolutionForGit(t *testing.T) {
	service := New(nil, nil, Config{})
	service.lookupIPv4 = func(_ context.Context, host string) (string, error) {
		if host != "gitea.localhost" {
			t.Fatalf("lookup host = %q", host)
		}
		return "192.168.65.254", nil
	}
	option, err := service.giteaHTTPCloneResolveOption(
		context.Background(),
		store.SourceConnection{Provider: "gitea"},
		"http://gitea.localhost:3030/team/app.git",
	)
	if err != nil {
		t.Fatal(err)
	}
	if option != "http.curloptResolve=gitea.localhost:3030:192.168.65.254" {
		t.Fatalf("resolve option = %q", option)
	}
}

func TestGiteaCloneDoesNotPinHTTPSOrLiteralIP(t *testing.T) {
	service := New(nil, nil, Config{})
	service.lookupIPv4 = func(_ context.Context, host string) (string, error) {
		t.Fatalf("unexpected lookup for %s", host)
		return "", nil
	}
	connection := store.SourceConnection{Provider: "gitea"}
	for _, cloneURL := range []string{
		"https://gitea.example.com/team/app.git",
		"http://192.168.1.20:3000/team/app.git",
	} {
		option, err := service.giteaHTTPCloneResolveOption(context.Background(), connection, cloneURL)
		if err != nil {
			t.Fatal(err)
		}
		if option != "" {
			t.Fatalf("resolve option for %q = %q", cloneURL, option)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
