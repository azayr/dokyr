// Package authz holds the role and permission policy for the control plane.
//
// The policy is deliberately a single table so the whole authorization surface
// can be read on one screen and asserted in one test. Handlers never compare
// role strings directly; they ask Can whether a role holds a permission.
package authz

import "sort"

// Permission names a capability a request may require. Permissions describe
// what a caller may do, not which screen the action lives on, so a single
// permission can gate several routes that share a blast radius.
type Permission string

const (
	// Read-only visibility over projects, services, databases, deployments,
	// and infrastructure metrics.
	PermProjectRead Permission = "project:read"
	// Create, edit, and delete projects, services, and database services.
	PermProjectWrite Permission = "project:write"
	// Deploy, stop, restart, and cancel deployments.
	PermProjectDeploy Permission = "project:deploy"
	// Read decrypted secrets: environment variables, database credentials,
	// SMTP settings, and registry access tokens.
	PermSecretRead Permission = "secret:read"
	// Write secrets and environment variables.
	PermSecretWrite Permission = "secret:write"
	// Run commands inside a managed container. Equivalent to reading every
	// secret available to that container, so it is not granted to viewers.
	PermContainerExec Permission = "container:exec"
	// Connect and remove source providers and external registries.
	PermIntegrationWrite Permission = "integration:write"
	// Manage the built-in registry: settings, tokens, tag deletion, GC.
	PermRegistryWrite Permission = "registry:write"
	// Change ingress: project and registry domains, and the raw Caddy
	// configuration. Grants effective control over all inbound traffic,
	// including the control panel's own hostname, so it is owner-only.
	PermIngressWrite Permission = "ingress:write"
	// Destructive or host-wide infrastructure actions: Docker cleanup,
	// cleanup schedules, and publishing a database on a host port.
	PermInfraWrite Permission = "infra:write"
	// Change control-plane settings and replace the control-plane container.
	PermPlatformWrite Permission = "platform:write"
	// Create, invite, re-role, and remove users.
	PermUserManage Permission = "user:manage"
)

// Roles, ordered from least to most privileged. The role strings match the
// users.role check constraint in the database schema.
const (
	RoleViewer    = "viewer"
	RoleDeveloper = "developer"
	RoleAdmin     = "admin"
	RoleOwner     = "owner"
)

// rolePermissions is the entire authorization policy.
//
// Owner is intentionally the only role holding PermIngressWrite,
// PermPlatformWrite, and PermUserManage: each of those can be turned into
// control of the host or of the control panel itself, so they must not be
// reachable by an account the owner invited for day-to-day work.
var rolePermissions = map[string]map[Permission]bool{
	RoleViewer: {
		PermProjectRead: true,
	},
	RoleDeveloper: {
		PermProjectRead:   true,
		PermProjectWrite:  true,
		PermProjectDeploy: true,
		PermSecretRead:    true,
		PermSecretWrite:   true,
		PermContainerExec: true,
	},
	RoleAdmin: {
		PermProjectRead:      true,
		PermProjectWrite:     true,
		PermProjectDeploy:    true,
		PermSecretRead:       true,
		PermSecretWrite:      true,
		PermContainerExec:    true,
		PermIntegrationWrite: true,
		PermRegistryWrite:    true,
		PermInfraWrite:       true,
	},
	RoleOwner: {
		PermProjectRead:      true,
		PermProjectWrite:     true,
		PermProjectDeploy:    true,
		PermSecretRead:       true,
		PermSecretWrite:      true,
		PermContainerExec:    true,
		PermIntegrationWrite: true,
		PermRegistryWrite:    true,
		PermInfraWrite:       true,
		PermIngressWrite:     true,
		PermPlatformWrite:    true,
		PermUserManage:       true,
	},
}

// Can reports whether role holds perm. An unknown role holds nothing, so a
// malformed or empty role fails closed rather than inheriting a default.
func Can(role string, perm Permission) bool {
	return rolePermissions[role][perm]
}

// CanOn reports whether role holds perm for a specific project.
//
// Permissions are currently global: every user who can read projects can read
// every project. This indirection exists so per-project membership can be added
// by changing this function and the membership lookup it consults, without
// revisiting the call sites in the router.
func CanOn(role string, perm Permission, projectID string) bool {
	_ = projectID
	return Can(role, perm)
}

// KnownRole reports whether value is a role the policy recognizes.
func KnownRole(value string) bool {
	_, ok := rolePermissions[value]
	return ok
}

// AssignableRoles lists the roles an owner may grant, least privileged first.
// Owner is included: transferring ownership is a deliberate owner action.
func AssignableRoles() []string {
	roles := make([]string, 0, len(rolePermissions))
	for role := range rolePermissions {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool {
		return len(rolePermissions[roles[i]]) < len(rolePermissions[roles[j]])
	})
	return roles
}

// Permissions lists the permissions held by role, sorted, for the UI to hide
// controls the caller cannot use. Hiding is cosmetic; the server remains the
// only boundary that matters.
func Permissions(role string) []Permission {
	held := rolePermissions[role]
	out := make([]Permission, 0, len(held))
	for perm := range held {
		out = append(out, perm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
