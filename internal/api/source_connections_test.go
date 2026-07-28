package api

import (
	"testing"

	"github.com/azayr/selfhost/internal/authz"
	"github.com/azayr/selfhost/internal/store"
)

func TestSourceConnectionsAreSharedWithoutForeignManageLinks(t *testing.T) {
	connections := []store.SourceConnection{
		{ID: "mine", UserID: "user-1", ManageURL: "https://github.com/settings/installations/1"},
		{ID: "shared", UserID: "user-2", ManageURL: "https://github.com/settings/installations/2"},
	}

	items := sourceConnectionsForUser(connections, "user-1", authz.RoleAdmin)

	if items[0].ManageURL == "" {
		t.Fatal("the installation owner should receive the GitHub management URL")
	}
	if !items[0].CanDelete {
		t.Fatal("an admin should be able to unlink their own source connection")
	}
	if items[1].ManageURL != "" {
		t.Fatal("another user must not receive the GitHub management URL")
	}
	if items[1].CanDelete {
		t.Fatal("an admin must not be able to unlink another user's source connection")
	}
	if connections[1].ManageURL == "" {
		t.Fatal("filtering the response must not mutate the stored connection")
	}
}

func TestOwnerCanDeleteEveryIntegrationResource(t *testing.T) {
	if !canDeleteIntegrationResource("owner", authz.RoleOwner, "admin") {
		t.Fatal("the owner should be able to delete another user's integration resource")
	}
	if canDeleteIntegrationResource("developer", authz.RoleDeveloper, "developer") {
		t.Fatal("a role without integration write permission must not be able to delete integration resources")
	}
}

func TestRegistryCredentialsExposePerItemDeletePermission(t *testing.T) {
	registries := []store.RegistryCredential{
		{ID: "mine", CreatedBy: "admin-1"},
		{ID: "shared", CreatedBy: "admin-2"},
	}

	adminItems := registryCredentialsForUser(registries, "admin-1", authz.RoleAdmin)
	if !adminItems[0].CanDelete || adminItems[1].CanDelete {
		t.Fatalf("admin delete permissions = [%t, %t], want [true, false]", adminItems[0].CanDelete, adminItems[1].CanDelete)
	}

	ownerItems := registryCredentialsForUser(registries, "owner", authz.RoleOwner)
	if !ownerItems[0].CanDelete || !ownerItems[1].CanDelete {
		t.Fatalf("owner delete permissions = [%t, %t], want [true, true]", ownerItems[0].CanDelete, ownerItems[1].CanDelete)
	}
}
