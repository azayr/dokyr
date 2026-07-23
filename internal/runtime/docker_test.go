package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestContainerLifecycleActionsUseExpectedDockerEndpoints(t *testing.T) {
	tests := []struct {
		name string
		path string
		run  func(*Docker) error
	}{
		{name: "stop project", path: "/containers/selfhost-prj_one/stop?t=10", run: func(d *Docker) error { return d.StopProject(context.Background(), "prj_one") }},
		{name: "restart project", path: "/containers/selfhost-prj_one/restart?t=10", run: func(d *Docker) error { return d.RestartProject(context.Background(), "prj_one") }},
		{name: "stop application", path: "/containers/selfhost-svc-svc_one/stop?t=10", run: func(d *Docker) error { return d.StopApplication(context.Background(), "svc_one") }},
		{name: "restart application", path: "/containers/selfhost-svc-svc_one/restart?t=10", run: func(d *Docker) error { return d.RestartApplication(context.Background(), "svc_one") }},
		{name: "stop database", path: "/containers/selfhost-db-db_one/stop?t=10", run: func(d *Docker) error { return d.StopDatabase(context.Background(), "db_one") }},
		{name: "restart database", path: "/containers/selfhost-db-db_one/restart?t=10", run: func(d *Docker) error { return d.RestartDatabase(context.Background(), "db_one") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.RequestURI() != test.path {
					t.Fatalf("request = %s %s, want POST %s", request.Method, request.URL.RequestURI(), test.path)
				}
				return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})}}
			if err := test.run(docker); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecuteApplicationCommandUsesScopedContainerAndDecodesOutput(t *testing.T) {
	requests := []string{}
	docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		switch request.URL.Path {
		case "/containers/selfhost-svc-svc_one/exec":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			command, _ := body["Cmd"].([]any)
			if len(command) != 3 || command[2] != "printf test" || body["WorkingDir"] != "/app" {
				t.Fatalf("unexpected exec body: %#v", body)
			}
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"Id":"exec_one"}`)), Header: make(http.Header)}, nil
		case "/exec/exec_one/start":
			var stream bytes.Buffer
			for streamID, value := range map[byte]string{1: "hello\n", 2: "warning\n"} {
				header := make([]byte, 8)
				header[0] = streamID
				binary.BigEndian.PutUint32(header[4:], uint32(len(value)))
				stream.Write(header)
				stream.WriteString(value)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(stream.Bytes())), Header: make(http.Header)}, nil
		case "/exec/exec_one/json":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Running":false,"ExitCode":7}`)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL.RequestURI())
			return nil, nil
		}
	})}}

	result, err := docker.ExecuteApplicationCommand(context.Background(), "svc_one", "printf test", "/app")
	if err != nil {
		t.Fatal(err)
	}
	if result.Container != "selfhost-svc-svc_one" || result.Stdout != "hello" || result.Stderr != "warning" || result.ExitCode != 7 {
		t.Fatalf("unexpected command result: %#v", result)
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestStoppedContainersReportStoppedStatus(t *testing.T) {
	docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"Name":"/example","State":{"Running":false,"Health":{"Status":""}},"Config":{"Image":"example:latest"},"NetworkSettings":{"Ports":{}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	application, err := docker.ApplicationService(context.Background(), "svc_one", "API")
	if err != nil {
		t.Fatal(err)
	}
	project, err := docker.ProjectService(context.Background(), "prj_one")
	if err != nil {
		t.Fatal(err)
	}
	database, err := docker.DatabaseRuntime(context.Background(), "db_one", 5432)
	if err != nil {
		t.Fatal(err)
	}
	if application.Status != "stopped" || project.Status != "stopped" || database.Status != "stopped" {
		t.Fatalf("stopped statuses = application %q, project %q, database %q", application.Status, project.Status, database.Status)
	}
}

func TestCheckRegistryConnectionUsesDockerAuthEndpoint(t *testing.T) {
	auth := &RegistryAuth{Username: "octocat", Password: "token", ServerAddress: "ghcr.io"}
	docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/auth" {
			t.Fatalf("request = %s %s, want POST /auth", request.Method, request.URL.Path)
		}
		var received RegistryAuth
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		if received != *auth {
			t.Fatalf("auth = %#v, want %#v", received, *auth)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Status":"Login Succeeded"}`)), Header: make(http.Header)}, nil
	})}}

	if err := docker.CheckRegistryConnection(context.Background(), auth); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeImagePullConsumesCompleteJSONStream(t *testing.T) {
	events := []string{}
	err := decodeImagePull(strings.NewReader("{\"status\":\"Pulling\",\"id\":\"layer-one\"}\n{\"status\":\"Download complete\",\"id\":\"layer-one\"}\n"), func(stage, eventType, message string) {
		events = append(events, stage+":"+eventType+":"+message)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || !strings.Contains(events[1], "Download complete") {
		t.Fatalf("unexpected pull events: %#v", events)
	}
}

func TestDecodeImagePullReturnsRegistryError(t *testing.T) {
	err := decodeImagePull(strings.NewReader("{\"errorDetail\":{\"message\":\"denied\"}}\n"), nil)
	if err == nil || err.Error() != "denied" {
		t.Fatalf("error = %v, want denied", err)
	}
}

func TestEncodeRegistryAuthUsesDockerCompatiblePaddedBase64URL(t *testing.T) {
	auth := &RegistryAuth{Username: "u", Password: "pw", ServerAddress: "ghcr.io"}
	encoded, err := encodeRegistryAuth(auth)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(encoded, "=") {
		t.Fatalf("encoded auth = %q, want RFC 4648 padding", encoded)
	}
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode auth: %v", err)
	}
	var roundTrip RegistryAuth
	if err := json.Unmarshal(decoded, &roundTrip); err != nil {
		t.Fatalf("unmarshal auth: %v", err)
	}
	if roundTrip != *auth {
		t.Fatalf("decoded auth = %#v, want %#v", roundTrip, *auth)
	}
}

func TestApplicationCandidateContainerNameIsDistinctAndStablePrefix(t *testing.T) {
	started := time.Unix(0, 123456789)
	name := applicationCandidateContainerName("svc_example", started)
	if name == applicationContainerName("svc_example") || !strings.HasPrefix(name, applicationContainerName("svc_example")+"-next-") {
		t.Fatalf("candidate name = %q", name)
	}
	if again := applicationCandidateContainerName("svc_example", started); again != name {
		t.Fatalf("candidate name changed: %q != %q", again, name)
	}
}

func TestRemoveOrphanApplicationCandidatesPreservesStableContainers(t *testing.T) {
	candidate := "selfhost-svc-svc_one-next-abc123"
	deleted := []string{}
	docker := &Docker{client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.Method {
		case http.MethodGet:
			if request.URL.Path != "/containers/json" || !strings.Contains(request.URL.Query().Get("filters"), "selfhost.service.kind") {
				t.Fatalf("unexpected list request: %s", request.URL.String())
			}
			body := `[
				{"Names":["/selfhost-svc-svc_one"],"Labels":{"selfhost.managed":"true","selfhost.service.kind":"application"}},
				{"Names":["/` + candidate + `"],"Labels":{"selfhost.managed":"true","selfhost.service.kind":"application"}},
				{"Names":["/unmanaged-next-container"],"Labels":{}}
			]`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case http.MethodDelete:
			deleted = append(deleted, request.URL.Path)
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request method %s", request.Method)
			return nil, nil
		}
	})}}

	removed, err := docker.RemoveOrphanApplicationCandidates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != candidate {
		t.Fatalf("removed candidates = %#v, want %q", removed, candidate)
	}
	if len(deleted) != 1 || deleted[0] != "/containers/"+candidate {
		t.Fatalf("delete requests = %#v", deleted)
	}
}

func TestApplicationHTTPHealthCheckUsesValidHostHeader(t *testing.T) {
	receivedHost := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		receivedHost <- request.Host
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(target.Port())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkApplicationHTTP(context.Background(), target.Hostname(), port, "/health"); err != nil {
		t.Fatal(err)
	}
	if host := <-receivedHost; host != "localhost" {
		t.Fatalf("health check Host = %q, want localhost", host)
	}
}

func TestImagePullQuery(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		fromImage string
		tag       string
	}{
		{name: "explicit Docker Hub tag", reference: "nginx:stable-alpine3.24", fromImage: "nginx", tag: "stable-alpine3.24"},
		{name: "implicit latest tag", reference: "nginx", fromImage: "nginx", tag: "latest"},
		{name: "private registry port", reference: "registry.example.test:5000/team/app:v2", fromImage: "registry.example.test:5000/team/app", tag: "v2"},
		{name: "registry port without tag", reference: "registry.example.test:5000/team/app", fromImage: "registry.example.test:5000/team/app", tag: "latest"},
		{name: "digest", reference: "nginx@sha256:0123456789abcdef", fromImage: "nginx", tag: "sha256:0123456789abcdef"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, err := url.ParseQuery(imagePullQuery(test.reference))
			if err != nil {
				t.Fatalf("parse query: %v", err)
			}
			if got := query.Get("fromImage"); got != test.fromImage {
				t.Errorf("fromImage = %q, want %q", got, test.fromImage)
			}
			if got := query.Get("tag"); got != test.tag {
				t.Errorf("tag = %q, want %q", got, test.tag)
			}
		})
	}
}

func TestHostCPUPercent(t *testing.T) {
	percent, ok := hostCPUPercent(hostCPUSample{total: 1000, idle: 700}, hostCPUSample{total: 1200, idle: 820})
	if !ok || percent != 40 {
		t.Fatalf("host CPU percent = %v, %v; want 40, true", percent, ok)
	}
}

func TestParseHostMemory(t *testing.T) {
	usage, total, err := parseHostMemory(strings.NewReader("MemTotal: 1000 kB\nMemAvailable: 250 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if usage != 750*1024 || total != 1000*1024 {
		t.Fatalf("host memory = %d/%d, want %d/%d", usage, total, 750*1024, 1000*1024)
	}
}

func TestMetricsReturnsCachedSnapshot(t *testing.T) {
	checkedAt := time.Now().UTC()
	docker := &Docker{metrics: &metricsCache{snapshot: MetricsSnapshot{
		CheckedAt:  checkedAt,
		Global:     GlobalMetrics{Running: 3},
		Containers: []ContainerMetrics{{ID: "one"}},
	}}}
	snapshot, err := docker.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.CheckedAt.Equal(checkedAt) || snapshot.Global.Running != 3 {
		t.Fatalf("unexpected cached snapshot: %#v", snapshot)
	}
	snapshot.Containers[0].ID = "changed"
	if docker.metrics.snapshot.Containers[0].ID != "one" {
		t.Fatal("Metrics returned the cache's mutable container slice")
	}
}

func TestGlobalMetricsOmitsContainers(t *testing.T) {
	docker := &Docker{metrics: &metricsCache{snapshot: MetricsSnapshot{
		CheckedAt:  time.Now().UTC(),
		Global:     GlobalMetrics{Containers: 2, Running: 1},
		Containers: []ContainerMetrics{{ID: "one"}, {ID: "two"}},
	}}}
	snapshot, err := docker.GlobalMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Containers != nil {
		t.Fatalf("global metrics exposed %d containers", len(snapshot.Containers))
	}
	if snapshot.Global.Containers != 2 || snapshot.Global.Running != 1 {
		t.Fatalf("global totals changed: %#v", snapshot.Global)
	}
}

func TestProjectMetricsFiltersAndAggregatesContainers(t *testing.T) {
	docker := &Docker{metrics: &metricsCache{snapshot: MetricsSnapshot{
		CheckedAt: time.Now().UTC(),
		Global:    GlobalMetrics{CPUCores: 4, MemoryLimit: 1000},
		Containers: []ContainerMetrics{
			{ID: "app", ProjectID: "project-one", State: "running", CPUPercent: 12.5, MemoryUsage: 200, DiskIO: IOMetrics{Read: 10, Write: 20}, NetworkIO: NetworkMetrics{Receive: 30, Transmit: 40}},
			{ID: "db", ProjectID: "project-one", State: "running", CPUPercent: 2.5, MemoryUsage: 100, DiskIO: IOMetrics{Read: 5, Write: 7}, NetworkIO: NetworkMetrics{Receive: 11, Transmit: 13}},
			{ID: "other", ProjectID: "project-two", State: "running", CPUPercent: 90, MemoryUsage: 600},
		},
	}}}
	snapshot, err := docker.ProjectMetrics(context.Background(), "project-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Containers) != 2 || snapshot.Containers[0].ID != "app" || snapshot.Containers[1].ID != "db" {
		t.Fatalf("unexpected project containers: %#v", snapshot.Containers)
	}
	if snapshot.Global.Containers != 2 || snapshot.Global.Running != 2 || snapshot.Global.CPUPercent != 15 || snapshot.Global.MemoryUsage != 300 || snapshot.Global.MemoryPercent != 30 {
		t.Fatalf("unexpected project totals: %#v", snapshot.Global)
	}
	if snapshot.Global.DiskIO != (IOMetrics{Read: 15, Write: 27}) || snapshot.Global.NetworkIO != (NetworkMetrics{Receive: 41, Transmit: 53}) {
		t.Fatalf("unexpected project I/O totals: %#v", snapshot.Global)
	}
}

func TestControlPlaneMetricsFiltersAndOrdersDependencies(t *testing.T) {
	docker := &Docker{metrics: &metricsCache{snapshot: MetricsSnapshot{
		CheckedAt: time.Now(),
		Global:    GlobalMetrics{CPUCores: 4, MemoryLimit: 1000},
		Containers: []ContainerMetrics{
			{ID: "project", ProjectID: "project-one", State: "running", CPUPercent: 20},
			{ID: "proxy", ControlPlaneService: "caddy", State: "running", CPUPercent: 2, MemoryUsage: 50},
			{ID: "api", ControlPlaneService: "selfhost", State: "running", CPUPercent: 5, MemoryUsage: 100},
			{ID: "database", ControlPlaneService: "postgres", State: "running", CPUPercent: 3, MemoryUsage: 200},
		},
	}}}
	snapshot, err := docker.ControlPlaneMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Global.Containers != 3 || snapshot.Global.Running != 3 {
		t.Fatalf("control-plane counts = %d/%d, want 3/3", snapshot.Global.Running, snapshot.Global.Containers)
	}
	if snapshot.Global.CPUPercent != 10 || snapshot.Global.MemoryUsage != 350 {
		t.Fatalf("control-plane aggregate = cpu %.2f memory %d", snapshot.Global.CPUPercent, snapshot.Global.MemoryUsage)
	}
	got := []string{}
	for _, container := range snapshot.Containers {
		got = append(got, container.ControlPlaneService)
	}
	if strings.Join(got, ",") != "selfhost,postgres,caddy" {
		t.Fatalf("control-plane order = %v", got)
	}
}

func TestControlPlaneServiceUsesCurrentComposeProject(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project": "dokyr",
		"com.docker.compose.service": "postgres",
	}
	if got := controlPlaneService(labels, "dokyr"); got != "postgres" {
		t.Fatalf("controlPlaneService = %q, want postgres", got)
	}
	if got := controlPlaneService(labels, "another-project"); got != "" {
		t.Fatalf("controlPlaneService accepted a different Compose project: %q", got)
	}
}

func TestContainerMetricsCalculations(t *testing.T) {
	var stats dockerStats
	stats.PreCPUStats.CPUUsage.TotalUsage = 100
	stats.CPUStats.CPUUsage.TotalUsage = 300
	stats.PreCPUStats.SystemUsage = 1000
	stats.CPUStats.SystemUsage = 2000
	stats.CPUStats.OnlineCPUs = 4
	if got := containerCPUPercent(stats); got != 80 {
		t.Fatalf("CPU percent = %v, want 80", got)
	}
	stats.MemoryStats.Usage = 900
	stats.MemoryStats.Limit = 2000
	stats.MemoryStats.Stats = map[string]uint64{"inactive_file": 100}
	usage, limit, percent := containerMemory(stats)
	if usage != 800 || limit != 2000 || percent != 40 {
		t.Fatalf("memory = %d/%d (%v), want 800/2000 (40)", usage, limit, percent)
	}
	stats.BlkioStats.Recursive = append(stats.BlkioStats.Recursive,
		struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		}{Op: "Read", Value: 120},
		struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		}{Op: "Write", Value: 340},
	)
	block := containerBlockIO(stats)
	if block.Read != 120 || block.Write != 340 {
		t.Fatalf("block IO = %#v", block)
	}
}

func TestPruneFilters(t *testing.T) {
	encoded := pruneFilters(map[string]map[string]bool{"dangling": {"false": true}})
	if !strings.Contains(encoded, `"dangling"`) || !strings.Contains(encoded, `"false":true`) {
		t.Fatalf("unexpected filters: %s", encoded)
	}
}

func TestMergeEnvironment(t *testing.T) {
	existing := []string{"PATH=/usr/local/bin", "APP_MODE=old", "REMOVED=value"}
	replacement := []string{"APP_MODE=production", "NEW_FLAG=1"}
	merged := mergeEnvironment(existing, replacement, []string{"APP_MODE", "REMOVED"})
	want := []string{"PATH=/usr/local/bin", "APP_MODE=production", "NEW_FLAG=1"}
	if len(merged) != len(want) {
		t.Fatalf("merged environment has %d entries, want %d: %#v", len(merged), len(want), merged)
	}
	for index := range want {
		if merged[index] != want[index] {
			t.Errorf("merged[%d] = %q, want %q", index, merged[index], want[index])
		}
	}
}

func TestParseContainerCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{name: "keycloak command", command: "start-dev", want: []string{"start-dev"}},
		{name: "arguments", command: "server --http-port 8080", want: []string{"server", "--http-port", "8080"}},
		{name: "quoted argument", command: `serve --message "hello world"`, want: []string{"serve", "--message", "hello world"}},
		{name: "empty quoted argument", command: `command '' tail`, want: []string{"command", "", "tail"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseContainerCommand(test.command)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("arguments = %#v, want %#v", got, test.want)
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("arguments = %#v, want %#v", got, test.want)
				}
			}
		})
	}
}

func TestParseContainerCommandRejectsInvalidQuoting(t *testing.T) {
	if _, err := parseContainerCommand(`start "unfinished`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if _, err := parseContainerCommand(`start trailing\`); err == nil {
		t.Fatal("expected trailing escape error")
	}
}
