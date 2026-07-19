package caddy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	tests := map[string]string{
		" Hello.TEST. ":   "hello.test",
		"app.example.com": "app.example.com",
	}
	for input, expected := range tests {
		got, err := NormalizeDomain(input)
		if err != nil || got != expected {
			t.Fatalf("NormalizeDomain(%q) = %q, %v; want %q", input, got, err, expected)
		}
	}
	for _, input := range []string{"localhost", "http://hello.test", "hello.test:8080", "bad_domain.test"} {
		if _, err := NormalizeDomain(input); err == nil {
			t.Fatalf("NormalizeDomain(%q) should fail", input)
		}
	}
}

func TestNormalizeControlHost(t *testing.T) {
	for input, expected := range map[string]string{
		"localhost":              "localhost",
		"127.0.0.1:8080":         "127.0.0.1",
		"Panel.Example.COM:8080": "panel.example.com",
	} {
		got, err := NormalizeControlHost(input)
		if err != nil || got != expected {
			t.Fatalf("NormalizeControlHost(%q) = %q, %v; want %q", input, got, err, expected)
		}
	}
}

func TestApplyLoadsHostRoutes(t *testing.T) {
	var received string
	client, err := New("http://caddy:2019", []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/load" || r.Header.Get("Content-Type") != "text/caddyfile" {
			t.Fatalf("unexpected request: %s %s %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(""))}, nil
	})}

	if err := client.Apply(context.Background(), []Route{{Domain: "hello.test", Upstream: "selfhost-prj_demo:80"}}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"admin unix//run/caddy-admin/admin.sock", "@project0 host hello.test", "reverse_proxy selfhost-prj_demo:80", "@controlIP header_regexp Host", "@control host 127.0.0.1 localhost", "reverse_proxy selfhost:8080", "respond \"Not Found\" 404"} {
		if !strings.Contains(received, expected) {
			t.Fatalf("rendered config does not contain %q:\n%s", expected, received)
		}
	}
}

func TestRenderAutomaticHTTPSRoute(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{Domain: "api.example.com", Upstream: "selfhost-prj_api:8080", HTTPS: true}})
	for _, expected := range []string{"api.example.com {", "reverse_proxy selfhost-prj_api:8080", "redir https://{host}{uri} permanent"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("rendered HTTPS config does not contain %q:\n%s", expected, configuration)
		}
	}
}

func TestRenderPathSpecificUpstreamsBeforeFallback(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{Domain: "app.test", Upstream: "selfhost-prj_app:80", Paths: []PathRoute{{Path: "/api/*", Upstream: "selfhost-prj_app:8080"}, {Path: "/socket/*", Upstream: "selfhost-prj_app:9001"}}}})
	for _, expected := range []string{"path /api/*", "reverse_proxy selfhost-prj_app:8080", "path /socket/*", "reverse_proxy selfhost-prj_app:9001", "reverse_proxy selfhost-prj_app:80"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("rendered path config does not contain %q:\n%s", expected, configuration)
		}
	}
	if strings.Index(configuration, "path /api/*") > strings.LastIndex(configuration, "reverse_proxy selfhost-prj_app:80") {
		t.Fatal("path rules must be rendered before the fallback")
	}
}

func TestRenderRestrictedDefaultPath(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{Domain: "api.test", Upstream: "selfhost-prj_api:8080", DefaultPath: "/api/*"}})
	for _, expected := range []string{"path /api/*", "reverse_proxy selfhost-prj_api:8080", "respond \"Not Found\" 404"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("restricted default config does not contain %q:\n%s", expected, configuration)
		}
	}
}

func TestRenderMultipleDomainsWithIndependentPaths(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	routes := []Route{
		{Domain: "domain.local", RejectUnmatched: true, Paths: []PathRoute{
			{Path: "/api/*", Upstream: "selfhost-prj_app:8080"},
			{Path: "/static/*", Upstream: "selfhost-prj_app:8080"},
		}},
		{Domain: "domain2.local", RejectUnmatched: true, Paths: []PathRoute{
			{Path: "/api/*", Upstream: "selfhost-prj_app:8080"},
		}},
	}
	configuration := client.Render(routes)
	for _, expected := range []string{"host domain.local", "host domain2.local", "path /api/*", "path /static/*", "reverse_proxy selfhost-prj_app:8080"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("multi-domain config does not contain %q:\n%s", expected, configuration)
		}
	}
	if count := strings.Count(configuration, "respond \"Not Found\" 404"); count < 3 {
		t.Fatalf("each host must reject unmatched paths; got %d 404 handlers:\n%s", count, configuration)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
