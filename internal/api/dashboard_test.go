package api

import (
	"testing"
	"time"

	"github.com/azayr/selfhost/internal/store"
)

func TestLatestServiceDeploymentsKeepsOnlyNewestPerService(t *testing.T) {
	now := time.Now()
	deployments := []store.Deployment{
		{ID: "service-a-new", ProjectID: "project-1", ServiceID: "service-a", Status: "healthy", CreatedAt: now},
		{ID: "service-b-new", ProjectID: "project-1", ServiceID: "service-b", Status: "failed", CreatedAt: now.Add(-time.Minute)},
		{ID: "service-a-old", ProjectID: "project-1", ServiceID: "service-a", Status: "failed", CreatedAt: now.Add(-2 * time.Minute)},
	}

	got := latestServiceDeployments(deployments)
	if len(got) != 2 {
		t.Fatalf("latestServiceDeployments() returned %d deployments, want 2", len(got))
	}
	if got[0].ID != "service-a-new" || got[1].ID != "service-b-new" {
		t.Fatalf("latestServiceDeployments() returned %q and %q", got[0].ID, got[1].ID)
	}
}

func TestLatestServiceDeploymentsGroupsLegacyRowsByProjectAndServiceName(t *testing.T) {
	deployments := []store.Deployment{
		{ID: "web-new", ProjectID: "project-1", ServiceName: "web", Status: "healthy"},
		{ID: "worker-new", ProjectID: "project-1", ServiceName: "worker", Status: "healthy"},
		{ID: "web-old", ProjectID: "project-1", ServiceName: "web", Status: "failed"},
		{ID: "other-project-web", ProjectID: "project-2", ServiceName: "web", Status: "failed"},
	}

	got := latestServiceDeployments(deployments)
	if len(got) != 3 {
		t.Fatalf("latestServiceDeployments() returned %d deployments, want 3", len(got))
	}
}

func TestTemporaryPublicURL(t *testing.T) {
	tests := []struct {
		url       string
		temporary bool
	}{
		{url: "http://203.0.113.20:3030", temporary: true},
		{url: "http://localhost:8888", temporary: true},
		{url: "https://panel.example.com", temporary: false},
	}
	for _, test := range tests {
		if got := temporaryPublicURL(test.url); got != test.temporary {
			t.Errorf("temporaryPublicURL(%q) = %v, want %v", test.url, got, test.temporary)
		}
	}
}
