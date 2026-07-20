package runtime

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

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
