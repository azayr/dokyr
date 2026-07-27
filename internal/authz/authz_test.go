package authz

import "testing"

// TestPolicyMatrix pins the whole policy. A change to who may do what has to be
// made here deliberately, which is the point: the table is the security
// boundary, so an accidental widening should fail a test rather than ship.
func TestPolicyMatrix(t *testing.T) {
	allow := map[string][]Permission{
		RoleViewer: {PermProjectRead},
		RoleDeveloper: {
			PermProjectRead, PermProjectWrite, PermProjectDeploy,
			PermSecretRead, PermSecretWrite, PermContainerExec,
		},
		RoleAdmin: {
			PermProjectRead, PermProjectWrite, PermProjectDeploy,
			PermSecretRead, PermSecretWrite, PermContainerExec,
			PermIntegrationWrite, PermRegistryWrite, PermInfraWrite,
		},
		RoleOwner: {
			PermProjectRead, PermProjectWrite, PermProjectDeploy,
			PermSecretRead, PermSecretWrite, PermContainerExec,
			PermIntegrationWrite, PermRegistryWrite, PermInfraWrite,
			PermIngressWrite, PermPlatformWrite, PermUserManage,
		},
	}
	every := []Permission{
		PermProjectRead, PermProjectWrite, PermProjectDeploy,
		PermSecretRead, PermSecretWrite, PermContainerExec,
		PermIntegrationWrite, PermRegistryWrite, PermInfraWrite,
		PermIngressWrite, PermPlatformWrite, PermUserManage,
	}
	for role, granted := range allow {
		want := map[Permission]bool{}
		for _, perm := range granted {
			want[perm] = true
		}
		for _, perm := range every {
			if got := Can(role, perm); got != want[perm] {
				t.Errorf("Can(%q, %q) = %v, want %v", role, perm, got, want[perm])
			}
		}
	}
}

// TestOwnerOnlyPermissions guards the three permissions that can be escalated
// into control of the host or of the control panel itself.
func TestOwnerOnlyPermissions(t *testing.T) {
	for _, perm := range []Permission{PermIngressWrite, PermPlatformWrite, PermUserManage} {
		for _, role := range []string{RoleViewer, RoleDeveloper, RoleAdmin} {
			if Can(role, perm) {
				t.Errorf("%q must not hold %q", role, perm)
			}
		}
		if !Can(RoleOwner, perm) {
			t.Errorf("owner must hold %q", perm)
		}
	}
}

// TestUnknownRoleHoldsNothing covers the fail-closed default: a blank role from
// a malformed token or a row written outside the application grants no access.
func TestUnknownRoleHoldsNothing(t *testing.T) {
	for _, role := range []string{"", "root", "Owner", "OWNER", "superuser"} {
		if perms := Permissions(role); len(perms) != 0 {
			t.Errorf("role %q holds %v, want none", role, perms)
		}
		if Can(role, PermProjectRead) {
			t.Errorf("role %q must not hold any permission", role)
		}
	}
}

func TestKnownRole(t *testing.T) {
	for _, role := range []string{RoleOwner, RoleAdmin, RoleDeveloper, RoleViewer} {
		if !KnownRole(role) {
			t.Errorf("KnownRole(%q) = false, want true", role)
		}
	}
	for _, role := range []string{"", "root", "Admin"} {
		if KnownRole(role) {
			t.Errorf("KnownRole(%q) = true, want false", role)
		}
	}
}

// TestAssignableRolesOrdering keeps the UI's role picker ordered from least to
// most privileged so the least dangerous option reads first.
func TestAssignableRolesOrdering(t *testing.T) {
	got := AssignableRoles()
	want := []string{RoleViewer, RoleDeveloper, RoleAdmin, RoleOwner}
	if len(got) != len(want) {
		t.Fatalf("AssignableRoles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AssignableRoles() = %v, want %v", got, want)
		}
	}
}

// TestCanOnMatchesCanWhileGlobal documents that permissions are not yet
// project-scoped. When membership lands, this test should be replaced by one
// asserting that a non-member is denied.
func TestCanOnMatchesCanWhileGlobal(t *testing.T) {
	if !CanOn(RoleDeveloper, PermProjectDeploy, "prj_other") {
		t.Fatal("CanOn should currently mirror Can")
	}
	if CanOn(RoleViewer, PermProjectDeploy, "prj_other") {
		t.Fatal("CanOn must still deny a permission the role lacks")
	}
}
