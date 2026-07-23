package platformupdate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestLatestResolvesPlatformManifestAndMetadata(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/azayr/dokyr/manifests/latest":
			w.Header().Set("Content-Type", indexOCI)
			w.Header().Set("Docker-Content-Digest", "sha256:index")
			fmt.Fprintf(w, `{"mediaType":%q,"manifests":[{"digest":"sha256:release","platform":{"os":"linux","architecture":%q}}]}`, indexOCI, runtime.GOARCH)
		case "/v2/azayr/dokyr/manifests/sha256:release":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", "sha256:release")
			fmt.Fprint(w, `{"config":{"digest":"sha256:config"}}`)
		case "/v2/azayr/dokyr/blobs/sha256:config":
			fmt.Fprint(w, `{"created":"2026-07-23T10:00:00Z","config":{"Labels":{"org.opencontainers.image.version":"1.4.0","org.opencontainers.image.revision":"abcdef123456","org.opencontainers.image.created":"2026-07-23T10:00:00Z"}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{
		http: server.Client(), registry: strings.TrimPrefix(server.URL, "https://"),
		repository: "azayr/dokyr", channel: "latest",
	}
	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "1.4.0" || release.Revision != "abcdef123456" || release.Digest != "sha256:index" {
		t.Fatalf("unexpected release: %+v", release)
	}
	if release.Image != client.registry+"/azayr/dokyr@sha256:index" {
		t.Fatalf("image = %q", release.Image)
	}
}

func TestNewClientValidatesPublishedImage(t *testing.T) {
	client, err := NewClient("ghcr.io/azayr/dokyr", "")
	if err != nil {
		t.Fatal(err)
	}
	if client.registry != "ghcr.io" || client.repository != "azayr/dokyr" || client.channel != "latest" {
		t.Fatalf("unexpected client: %+v", client)
	}
	if _, err := NewClient("dokyr", "latest"); err == nil {
		t.Fatal("expected image without registry to be rejected")
	}
}
