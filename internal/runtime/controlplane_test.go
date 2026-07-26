package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestControlPlaneContainerNameSupportsDokyrAndLegacyServiceNames(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		service string
		name    string
	}{
		{service: "dokyr", name: "dokyr-dokyr-1"},
		{service: "selfhost", name: "dokyr-selfhost-1"},
	} {
		t.Run(test.service, func(t *testing.T) {
			docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.URL.Path != "/containers/json" {
					t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL.Path)
				}
				body := `[{"Id":"` + hostname + `-control","Names":["/` + test.name + `"],"Labels":{"com.docker.compose.project":"dokyr","com.docker.compose.service":"` + test.service + `"}}]`
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			name, err := docker.ControlPlaneContainerName(context.Background(), "dokyr")
			if err != nil {
				t.Fatal(err)
			}
			if name != test.name {
				t.Fatalf("container name = %q, want %q", name, test.name)
			}
		})
	}
}

func TestRecreateControlPlaneServicePreservesComposeLabelsAndOneStorageDriver(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}

	requests := []string{}
	docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		response := func(status int, body string) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}

		switch request.Method + " " + request.URL.Path {
		case "GET /containers/json":
			return response(http.StatusOK, `[{
				"Id":"`+hostname+`-selfhost",
				"Names":["/dokyr-selfhost-1"],
				"Labels":{"com.docker.compose.project":"dokyr","com.docker.compose.service":"selfhost"}
			},{
				"Id":"registry-id",
				"Names":["/dokyr-registry-1"],
				"Labels":{"com.docker.compose.project":"dokyr","com.docker.compose.service":"registry"}
			}]`)
		case "GET /containers/dokyr-registry-1/json":
			return response(http.StatusOK, `{
				"Name":"/dokyr-registry-1",
				"Config":{
					"Image":"registry:3",
					"Env":[
						"REGISTRY_STORAGE=filesystem",
						"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/var/lib/registry",
						"REGISTRY_STORAGE_S3_REGION=",
						"REGISTRY_STORAGE_S3_ENDPOINT="
					],
					"Labels":{
						"com.docker.compose.project":"dokyr",
						"com.docker.compose.service":"registry",
						"com.docker.compose.container-number":"1"
					}
				},
				"HostConfig":{
					"ExtraHosts":["host.docker.internal:host-gateway"],
					"RestartPolicy":{"Name":"unless-stopped"}
				},
				"NetworkSettings":{"Networks":{"dokyr_control":{"Aliases":["registry"]}}}
			}`)
		case "POST /containers/dokyr-registry-1/stop",
			"POST /containers/dokyr-registry-1/rename",
			"POST /containers/dokyr-registry-1/start",
			"DELETE /containers/dokyr-registry-1-reconfigure":
			return response(http.StatusNoContent, "")
		case "POST /containers/create":
			var body struct {
				Env        []string          `json:"Env"`
				Labels     map[string]string `json:"Labels"`
				HostConfig struct {
					ExtraHosts []string `json:"ExtraHosts"`
				} `json:"HostConfig"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Labels["com.docker.compose.project"] != "dokyr" || body.Labels["com.docker.compose.service"] != "registry" {
				t.Fatalf("replacement labels = %#v", body.Labels)
			}
			if len(body.HostConfig.ExtraHosts) != 1 || body.HostConfig.ExtraHosts[0] != "host.docker.internal:host-gateway" {
				t.Fatalf("replacement extra hosts = %#v", body.HostConfig.ExtraHosts)
			}
			assertEnvironmentValue(t, body.Env, "REGISTRY_STORAGE", "s3")
			assertEnvironmentValue(t, body.Env, "REGISTRY_STORAGE_S3_BUCKET", "docker-images")
			assertEnvironmentValue(t, body.Env, "REGISTRY_STORAGE_S3_REGIONENDPOINT", "http://minio:9000")
			assertEnvironmentMissing(t, body.Env, "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY")
			assertEnvironmentMissing(t, body.Env, "REGISTRY_STORAGE_S3_ENDPOINT")
			return response(http.StatusCreated, `{"Id":"replacement-id"}`)
		default:
			t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL.RequestURI())
			return nil, nil
		}
	})}}

	err = docker.RecreateControlPlaneService(context.Background(), "registry", []string{
		"REGISTRY_STORAGE=s3",
		"REGISTRY_STORAGE_S3_BUCKET=docker-images",
		"REGISTRY_STORAGE_S3_REGIONENDPOINT=http://minio:9000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 7 {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestStorageDriverEnvKeepsOnlySelectedDriver(t *testing.T) {
	base := []string{
		"REGISTRY_STORAGE_DELETE_ENABLED=true",
		"REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY=/var/lib/registry",
		"REGISTRY_STORAGE_S3_BUCKET=docker-images",
	}

	s3 := storageDriverEnv(append([]string{"REGISTRY_STORAGE=s3"}, base...))
	assertEnvironmentMissing(t, s3, "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY")
	assertEnvironmentValue(t, s3, "REGISTRY_STORAGE_S3_BUCKET", "docker-images")

	filesystem := storageDriverEnv(append([]string{"REGISTRY_STORAGE=filesystem"}, base...))
	assertEnvironmentValue(t, filesystem, "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", "/var/lib/registry")
	assertEnvironmentMissing(t, filesystem, "REGISTRY_STORAGE_S3_BUCKET")
}

func assertEnvironmentValue(t *testing.T, env []string, key, want string) {
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

func assertEnvironmentMissing(t *testing.T, env []string, key string) {
	t.Helper()
	for _, entry := range env {
		gotKey, _, _ := strings.Cut(entry, "=")
		if gotKey == key {
			t.Fatalf("%s unexpectedly present in %#v", key, env)
		}
	}
}
