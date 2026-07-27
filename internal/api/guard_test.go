package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/authz"
)

// routePermissions is the expected permission for every authenticated route.
//
// This is a golden table on purpose. Adding a route means adding a line here,
// which forces a second, explicit decision about who may call it — and makes a
// widening of access visible in review as a diff to this list rather than as an
// easily missed argument at the registration site.
var routePermissions = map[string]authz.Permission{
	"GET /api/auth/me":                  openToAnyRole,
	"GET /api/account/security":         openToAnyRole,
	"PUT /api/account/password":         openToAnyRole,
	"POST /api/account/2fa/setup":       openToAnyRole,
	"POST /api/account/2fa/confirm":     openToAnyRole,
	"DELETE /api/account/2fa":           openToAnyRole,
	"GET /api/account/github/start":     openToAnyRole,
	"DELETE /api/account/github":        openToAnyRole,
	"GET /api/settings/smtp":            authz.PermPlatformWrite,
	"PUT /api/settings/smtp":            authz.PermPlatformWrite,
	"POST /api/settings/smtp/test":      authz.PermPlatformWrite,
	"GET /api/settings/platform/update": authz.PermPlatformWrite,

	"POST /api/settings/platform/update/check": authz.PermPlatformWrite,
	"PUT /api/settings/platform/update":        authz.PermPlatformWrite,
	"POST /api/settings/platform/update/apply": authz.PermPlatformWrite,

	"GET /api/users":                  authz.PermUserManage,
	"POST /api/users":                 authz.PermUserManage,
	"POST /api/users/{id}/invitation": authz.PermUserManage,
	"PUT /api/users/{id}/role":        authz.PermUserManage,
	"DELETE /api/users/{id}":          authz.PermUserManage,

	"GET /api/dashboard":                       authz.PermProjectRead,
	"GET /api/domains":                         authz.PermProjectRead,
	"GET /api/projects":                        authz.PermProjectRead,
	"POST /api/projects":                       authz.PermProjectWrite,
	"GET /api/projects/{id}":                   authz.PermProjectRead,
	"PUT /api/projects/{id}":                   authz.PermProjectWrite,
	"DELETE /api/projects/{id}":                authz.PermProjectWrite,
	"PUT /api/projects/{id}/domain":            authz.PermIngressWrite,
	"POST /api/projects/{id}/deploy":           authz.PermProjectDeploy,
	"POST /api/projects/{id}/stop":             authz.PermProjectDeploy,
	"POST /api/projects/{id}/restart":          authz.PermProjectDeploy,
	"GET /api/projects/{id}/logs":              authz.PermProjectRead,
	"GET /api/projects/{id}/metrics":           authz.PermProjectRead,
	"GET /api/projects/{id}/environment":       authz.PermSecretRead,
	"PUT /api/projects/{id}/environment":       authz.PermSecretWrite,
	"POST /api/projects/{id}/databases":        authz.PermProjectWrite,
	"POST /api/projects/{id}/services":         authz.PermProjectWrite,
	"POST /api/projects/{id}/compose/validate": authz.PermProjectWrite,
	"POST /api/projects/{id}/compose":          authz.PermProjectWrite,

	"PUT /api/services/{id}":                       authz.PermProjectWrite,
	"POST /api/services/{id}/deploy":               authz.PermProjectDeploy,
	"POST /api/services/{id}/stop":                 authz.PermProjectDeploy,
	"POST /api/services/{id}/restart":              authz.PermProjectDeploy,
	"POST /api/services/{id}/exec":                 authz.PermContainerExec,
	"GET /api/services/{id}/deployment-triggers":   authz.PermProjectRead,
	"PUT /api/services/{id}/deployment-triggers":   authz.PermProjectWrite,
	"GET /api/services/{id}/logs":                  authz.PermProjectRead,
	"GET /api/services/{id}/environment":           authz.PermSecretRead,
	"PUT /api/services/{id}/environment":           authz.PermSecretWrite,
	"DELETE /api/services/{id}":                    authz.PermProjectWrite,
	"GET /api/databases/{id}/credentials":          authz.PermSecretRead,
	"PUT /api/databases/{id}/exposure":             authz.PermInfraWrite,
	"POST /api/databases/{id}/stop":                authz.PermProjectDeploy,
	"POST /api/databases/{id}/restart":             authz.PermProjectDeploy,
	"GET /api/databases/{id}/logs":                 authz.PermProjectRead,
	"GET /api/databases/{id}/events":               authz.PermProjectRead,
	"DELETE /api/databases/{id}":                   authz.PermProjectWrite,
	"GET /api/deployments":                         authz.PermProjectRead,
	"GET /api/deployments/{id}":                    authz.PermProjectRead,
	"POST /api/deployments/{id}/cancel":            authz.PermProjectDeploy,
	"GET /api/integrations":                        authz.PermProjectRead,
	"GET /api/integrations/oauth/{provider}/start": authz.PermIntegrationWrite,
	"GET /api/integrations/github/install/start":   authz.PermIntegrationWrite,

	"POST /api/integrations/github/installations/sync": authz.PermIntegrationWrite,
	"GET /api/integrations/sources/{id}/repositories":  authz.PermProjectRead,
	"DELETE /api/integrations/sources/{id}":            authz.PermIntegrationWrite,
	"POST /api/integrations/registries":                authz.PermIntegrationWrite,
	"POST /api/integrations/registries/{id}/check":     authz.PermIntegrationWrite,
	"DELETE /api/integrations/registries/{id}":         authz.PermIntegrationWrite,

	"GET /api/caddy/config":    authz.PermIngressWrite,
	"PUT /api/caddy/config":    authz.PermIngressWrite,
	"POST /api/caddy/reset":    authz.PermIngressWrite,
	"GET /api/registry/status": authz.PermProjectRead,

	"GET /api/infrastructure/metrics":               authz.PermProjectRead,
	"GET /api/infrastructure/control-plane/metrics": authz.PermProjectRead,
	"GET /api/infrastructure/control-plane/logs":    authz.PermInfraWrite,
	"GET /api/infrastructure/cleanup":               authz.PermInfraWrite,
	"POST /api/infrastructure/cleanup":              authz.PermInfraWrite,
	"GET /api/infrastructure/cleanup/schedule":      authz.PermInfraWrite,
	"PUT /api/infrastructure/cleanup/schedule":      authz.PermInfraWrite,

	"GET /api/registry/settings":              authz.PermRegistryWrite,
	"PUT /api/registry/settings":              authz.PermRegistryWrite,
	"GET /api/registry/domain":                authz.PermRegistryWrite,
	"PUT /api/registry/domain":                authz.PermIngressWrite,
	"GET /api/registry/access-tokens":         authz.PermSecretRead,
	"POST /api/registry/access-tokens":        authz.PermRegistryWrite,
	"DELETE /api/registry/access-tokens/{id}": authz.PermRegistryWrite,
	"GET /api/registry/repositories":          authz.PermProjectRead,

	"DELETE /api/registry/repositories/{name}/tags/{tag}": authz.PermRegistryWrite,
	"DELETE /api/registry/tags":                           authz.PermRegistryWrite,
	"POST /api/registry/garbage-collection":               authz.PermRegistryWrite,
}

// TestEveryRouteHasAnExpectedPermission fails when a route is added, removed, or
// re-permissioned without updating routePermissions. A new endpoint is the most
// likely way for an unguarded route to appear, and this is what catches it.
func TestEveryRouteHasAnExpectedPermission(t *testing.T) {
	registered := registeredRoutes(t)
	for _, pattern := range sortedPatterns(registered) {
		want, listed := routePermissions[pattern]
		if !listed {
			t.Errorf("route %q is registered but missing from routePermissions; choose a permission for it deliberately", pattern)
			continue
		}
		if registered[pattern] != want {
			t.Errorf("route %q requires %q, expected %q", pattern, registered[pattern], want)
		}
	}
	for _, pattern := range sortedPatterns(routePermissions) {
		if _, ok := registered[pattern]; !ok {
			t.Errorf("route %q is listed in routePermissions but no longer registered", pattern)
		}
	}
}

// TestSecretRoutesAreNotReadableByViewers checks the tier that leaks credentials
// rather than merely allowing an action.
func TestSecretRoutesAreNotReadableByViewers(t *testing.T) {
	registered := registeredRoutes(t)
	secretRoutes := []string{
		"GET /api/projects/{id}/environment",
		"GET /api/services/{id}/environment",
		"GET /api/databases/{id}/credentials",
		"GET /api/settings/smtp",
		"GET /api/registry/access-tokens",
	}
	for _, pattern := range secretRoutes {
		perm, ok := registered[pattern]
		if !ok {
			t.Fatalf("expected %q to be registered", pattern)
		}
		if authz.Can(authz.RoleViewer, perm) {
			t.Errorf("viewer can reach %q via %q", pattern, perm)
		}
	}
}

// TestIngressRoutesAreOwnerOnly covers the escalation path that motivated the
// permission: whoever can rewrite Caddy's routing table controls every hostname
// the server answers on, including the control panel's.
func TestIngressRoutesAreOwnerOnly(t *testing.T) {
	registered := registeredRoutes(t)
	for _, pattern := range []string{
		"PUT /api/caddy/config",
		"POST /api/caddy/reset",
		"PUT /api/projects/{id}/domain",
		"PUT /api/registry/domain",
	} {
		perm, ok := registered[pattern]
		if !ok {
			t.Fatalf("expected %q to be registered", pattern)
		}
		for _, role := range []string{authz.RoleViewer, authz.RoleDeveloper, authz.RoleAdmin} {
			if authz.Can(role, perm) {
				t.Errorf("%q can reach %q via %q", role, pattern, perm)
			}
		}
	}
}

// TestAuthorizeDeniesInsufficientRole exercises the middleware itself rather
// than the table, including the case where claims are absent entirely.
func TestAuthorizeDeniesInsufficientRole(t *testing.T) {
	a := &API{}
	reached := false
	handler := a.authorize(authz.PermIngressWrite, func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name   string
		role   string
		claims bool
		status int
	}{
		{"no claims", "", false, http.StatusUnauthorized},
		{"viewer", authz.RoleViewer, true, http.StatusForbidden},
		{"developer", authz.RoleDeveloper, true, http.StatusForbidden},
		{"admin", authz.RoleAdmin, true, http.StatusForbidden},
		{"unknown role", "root", true, http.StatusForbidden},
		{"owner", authz.RoleOwner, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodPut, "/api/caddy/config", nil)
			if tc.claims {
				req = req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Role: tc.role}))
			}
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if reached != (tc.status == http.StatusOK) {
				t.Fatalf("handler reached = %v, want %v", reached, tc.status == http.StatusOK)
			}
		})
	}
}

// TestAuthorizeAllowsAnyRoleForOwnAccountRoutes confirms openToAnyRole still
// requires authentication, and admits even a viewer.
func TestAuthorizeAllowsAnyRoleForOwnAccountRoutes(t *testing.T) {
	a := &API{}
	handler := a.authorize(openToAnyRole, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Role: authz.RoleViewer}))
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// registeredRoutes builds the route table the way Handler does, without needing
// the dependencies a live API has, so the policy can be asserted in a unit test.
func registeredRoutes(t *testing.T) map[string]authz.Permission {
	t.Helper()
	a := &API{}
	mux := newGuardedMux(a)
	a.registerProtectedRoutes(mux)
	return mux.patterns()
}

// TestUserManagementRoutesAreOwnerOnly covers the escalation path that matters
// most once invitations exist: whoever can grant roles can grant themselves any
// permission, so the route set must be owner-only in its entirety.
func TestUserManagementRoutesAreOwnerOnly(t *testing.T) {
	registered := registeredRoutes(t)
	found := 0
	for pattern, perm := range registered {
		if !strings.HasPrefix(trimMethod(pattern), "/api/users") {
			continue
		}
		found++
		if perm != authz.PermUserManage {
			t.Errorf("route %q requires %q, want %q", pattern, perm, authz.PermUserManage)
		}
		for _, role := range []string{authz.RoleViewer, authz.RoleDeveloper, authz.RoleAdmin} {
			if authz.Can(role, perm) {
				t.Errorf("%q can reach %q", role, pattern)
			}
		}
	}
	if found == 0 {
		t.Fatal("expected user management routes to be registered")
	}
}

func trimMethod(pattern string) string {
	if index := strings.IndexByte(pattern, ' '); index >= 0 {
		return pattern[index+1:]
	}
	return pattern
}
