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
	client, err := New("http://caddy:2019", []string{"localhost", "127.0.0.1"}, nil)
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
	for _, expected := range []string{"admin unix//run/caddy-admin/admin.sock", "@project0 host hello.test", "reverse_proxy selfhost-prj_demo:80", "@controlIP header_regexp Host", "@control host 127.0.0.1 localhost", "reverse_proxy selfhost:8080", "@registry host registry.invalid", "handle /api/registry/token", "reverse_proxy registry:5000", "respond \"Not Found\" 404"} {
		if !strings.Contains(received, expected) {
			t.Fatalf("rendered config does not contain %q:\n%s", expected, received)
		}
	}
}

func TestRenderRegistryHostsBeforeCatchAll(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"}, []string{"registry.example.com", "Registry2.Example.com"}, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render(nil)
	for _, expected := range []string{"@control host panel.example.com", "reverse_proxy dokyr:8080", "@registry host registry.example.com registry2.example.com", "handle /api/registry/token", "reverse_proxy registry:5000"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("rendered registry config does not contain %q:\n%s", expected, configuration)
		}
	}
	if strings.Index(configuration, "@registry host") < strings.Index(configuration, "@control host") {
		t.Fatal("registry route should be rendered after the control route")
	}
	if strings.Index(configuration, "@registry host") > strings.LastIndex(configuration, "respond \"Not Found\" 404") {
		t.Fatal("registry route should be rendered before the 404 catch-all")
	}
}

func TestControlUpstreamValidation(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"localhost"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	if client.ControlUpstream() != "dokyr:8080" {
		t.Fatalf("control upstream = %q", client.ControlUpstream())
	}
	for _, invalid := range []string{"dokyr", "http://dokyr:8080", "dokyr:0", "bad_name:8080"} {
		if _, err := New("http://caddy:2019", []string{"localhost"}, nil, invalid); err == nil {
			t.Fatalf("control upstream %q should fail", invalid)
		}
	}
}

func TestRenderAutomaticHTTPSRoute(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"}, nil)
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
	client, err := New("http://caddy:2019", []string{"localhost"}, nil)
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
	client, err := New("http://caddy:2019", []string{"localhost"}, nil)
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
	client, err := New("http://caddy:2019", []string{"localhost"}, nil)
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

func TestRenderRegistryDomainWithAutomaticHTTPS(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"control.example.com"}, []string{"registry.invalid"}, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{
		Domain:   "registry.example.com",
		HTTPS:    true,
		Upstream: "registry:5000",
		Paths: []PathRoute{{
			Path:     "/api/registry/token",
			Upstream: "dokyr:8080",
		}},
	}})
	for _, expected := range []string{
		"registry.example.com {",
		"path /api/registry/token",
		"reverse_proxy dokyr:8080",
		"reverse_proxy registry:5000",
		"redir https://{host}{uri} permanent",
	} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("registry domain config does not contain %q:\n%s", expected, configuration)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestIsControlHost(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com", "Localhost"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"panel.example.com", "PANEL.example.com", "panel.example.com.", " panel.example.com ", "localhost"} {
		if !client.IsControlHost(domain) {
			t.Errorf("IsControlHost(%q) = false, want true", domain)
		}
	}
	for _, domain := range []string{"", "app.example.com", "notpanel.example.com", "panel.example.com.evil.com"} {
		if client.IsControlHost(domain) {
			t.Errorf("IsControlHost(%q) = true, want false", domain)
		}
	}
}

func TestControlDomainAddsHTTPSRouteAndRemainsReserved(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"127.0.0.1"}, []string{"registry.invalid"}, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetControlDomain("panel.example.com", true); err != nil {
		t.Fatal(err)
	}
	configuration := client.Render(nil)
	for _, expected := range []string{
		"panel.example.com {",
		"reverse_proxy dokyr:8080",
		"@controlDomain host panel.example.com",
		"redir https://{host}{uri} permanent",
	} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("control-domain config does not contain %q:\n%s", expected, configuration)
		}
	}
	if !client.IsControlHost("panel.example.com") {
		t.Fatal("configured platform domain must be reserved from project routes")
	}
	if !client.ControlDomainHTTPSEnabled() {
		t.Fatal("configured platform domain should use automatic HTTPS by default")
	}
}

func TestControlDomainCanUseHTTPBehindExternalProxy(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"127.0.0.1"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetControlDomain("panel.example.com", false); err != nil {
		t.Fatal(err)
	}

	configuration := client.Render(nil)
	for _, expected := range []string{
		"@controlDomain host panel.example.com",
		"reverse_proxy dokyr:8080",
	} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("HTTP-origin control-domain config does not contain %q:\n%s", expected, configuration)
		}
	}
	for _, unexpected := range []string{
		"panel.example.com {",
		"redir https://{host}{uri} permanent",
	} {
		if strings.Contains(configuration, unexpected) {
			t.Fatalf("HTTP-origin control-domain config unexpectedly contains %q:\n%s", unexpected, configuration)
		}
	}
	if client.ControlDomainHTTPSEnabled() {
		t.Fatal("HTTP-origin control domain should report automatic HTTPS as disabled")
	}
	if !client.IsControlHost("panel.example.com") {
		t.Fatal("HTTP-origin platform domain must remain reserved from project routes")
	}
}

func TestMailHostnameOnlyForwardsACMEChallenge(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetMailHostname("mail.example.com"); err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{Domain: "mail.example.com", Upstream: "project:80"}})
	for _, expected := range []string{"@mailAcme host mail.example.com", "handle /.well-known/acme-challenge/*", "reverse_proxy stalwart:8080"} {
		if !strings.Contains(configuration, expected) {
			t.Fatalf("mail ACME config does not contain %q:\n%s", expected, configuration)
		}
	}
	if strings.Contains(configuration, "reverse_proxy project:80") {
		t.Fatalf("mail hostname must not be assigned to a project:\n%s", configuration)
	}
	if !client.IsControlHost("mail.example.com") {
		t.Fatal("mail hostname must be reserved from project domains")
	}
}

// TestRenderKeepsControlHostAheadOfProjects covers the shadowing case: Caddy
// stops at the first matching handle block, so a project route must never be
// written before the control-panel matcher.
func TestRenderControlHostBeforeProjectRoutes(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{{Domain: "app.example.com", Upstream: "selfhost-prj_app:80"}})
	control, project := strings.Index(configuration, "@control host"), strings.Index(configuration, "@project0 host")
	if project < 0 {
		t.Fatalf("project route missing:\n%s", configuration)
	}
	if control > project {
		t.Fatalf("control route must be rendered before project routes:\n%s", configuration)
	}
}

// TestRenderDropsRouteClaimingControlHost is the defense in depth behind the
// API-level rejection: a stored binding for a control hostname is discarded
// rather than allowed to take over the panel.
func TestRenderDropsRouteClaimingControlHost(t *testing.T) {
	client, err := New("http://caddy:2019", []string{"panel.example.com"}, nil, "dokyr:8080")
	if err != nil {
		t.Fatal(err)
	}
	configuration := client.Render([]Route{
		{Domain: "panel.example.com", Upstream: "selfhost-prj_evil:80", HTTPS: true},
		{Domain: "app.example.com", Upstream: "selfhost-prj_app:80"},
	})
	if strings.Contains(configuration, "selfhost-prj_evil") {
		t.Fatalf("route claiming the control host should be dropped:\n%s", configuration)
	}
	if !strings.Contains(configuration, "@control host panel.example.com") {
		t.Fatalf("control route missing:\n%s", configuration)
	}
	if !strings.Contains(configuration, "selfhost-prj_app") {
		t.Fatalf("unrelated project route should survive:\n%s", configuration)
	}
}
