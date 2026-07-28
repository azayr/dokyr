package api

import (
	"testing"

	"github.com/azayr/selfhost/internal/store"
)

func TestSourceConnectionsAreSharedWithoutForeignManageLinks(t *testing.T) {
	connections := []store.SourceConnection{
		{ID: "mine", UserID: "user-1", ManageURL: "https://github.com/settings/installations/1"},
		{ID: "shared", UserID: "user-2", ManageURL: "https://github.com/settings/installations/2"},
	}

	items := sourceConnectionsForUser(connections, "user-1")

	if items[0].ManageURL == "" {
		t.Fatal("the installation owner should receive the GitHub management URL")
	}
	if items[1].ManageURL != "" {
		t.Fatal("another user must not receive the GitHub management URL")
	}
	if connections[1].ManageURL == "" {
		t.Fatal("filtering the response must not mutate the stored connection")
	}
}
