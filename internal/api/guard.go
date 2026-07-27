package api

import (
	"net/http"
	"sort"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/authz"
)

// guardedMux wraps a ServeMux so that authenticated routes cannot be registered
// without naming the permission they require.
//
// The embedded mux is unexported and never returned by a method that also
// accepts a handler, so `protected.HandleFunc(pattern, handler)` does not
// compile. Adding a route means choosing a permission — the failure mode for a
// forgotten check is a build error rather than an endpoint that is reachable by
// every account.
type guardedMux struct {
	mux    *http.ServeMux
	api    *API
	routes map[string]authz.Permission
}

// controlHostConflictMessage is returned when a caller tries to bind a domain
// that the control panel itself answers on. Taking it would hide the panel
// behind the caller's own service.
const controlHostConflictMessage = "this domain is reserved for the Dokyr control panel"

// openToAnyRole marks a route that every authenticated caller may reach
// regardless of role, such as reading or changing one's own account. It is a
// deliberate, greppable choice rather than the absence of one.
const openToAnyRole authz.Permission = ""

func newGuardedMux(a *API) *guardedMux {
	return &guardedMux{mux: http.NewServeMux(), api: a, routes: map[string]authz.Permission{}}
}

// handle registers pattern behind perm. Callers reach the handler only if the
// role resolved for the request holds perm.
func (g *guardedMux) handle(pattern string, perm authz.Permission, h http.HandlerFunc) {
	if _, exists := g.routes[pattern]; exists {
		// Registering the same pattern twice would panic in ServeMux anyway;
		// failing here names the duplicate.
		panic("api: duplicate route registration for " + pattern)
	}
	g.routes[pattern] = perm
	g.mux.HandleFunc(pattern, g.api.authorize(perm, h))
}

// handleAnyRole registers a route every authenticated caller may reach.
func (g *guardedMux) handleAnyRole(pattern string, h http.HandlerFunc) {
	g.handle(pattern, openToAnyRole, h)
}

// handler returns the underlying mux for mounting behind session authentication.
func (g *guardedMux) handler() http.Handler { return g.mux }

// patterns lists every registered route with its required permission. Used by
// tests to assert the policy of the whole surface at once.
func (g *guardedMux) patterns() map[string]authz.Permission {
	out := make(map[string]authz.Permission, len(g.routes))
	for pattern, perm := range g.routes {
		out[pattern] = perm
	}
	return out
}

// authorize rejects a request whose caller does not hold perm. Session validity
// is already established by auth.Require, so a missing claim here means the
// route was mounted outside the authenticated tree and is treated as a denial.
func (a *API) authorize(perm authz.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.FromContext(r.Context())
		if !ok {
			write(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		if perm != openToAnyRole && !authz.Can(claims.Role, perm) {
			write(w, http.StatusForbidden, map[string]string{"error": forbiddenMessage(perm)})
			return
		}
		next(w, r)
	}
}

// forbiddenMessage explains a denial in terms of the action rather than the
// permission name, which is an internal identifier.
func forbiddenMessage(perm authz.Permission) string {
	switch perm {
	case authz.PermProjectWrite:
		return "your role cannot create or change projects"
	case authz.PermProjectDeploy:
		return "your role cannot deploy or restart services"
	case authz.PermSecretRead:
		return "your role cannot read environment variables or credentials"
	case authz.PermSecretWrite:
		return "your role cannot change environment variables or credentials"
	case authz.PermContainerExec:
		return "your role cannot execute commands in containers"
	case authz.PermIntegrationWrite:
		return "your role cannot change source or registry connections"
	case authz.PermRegistryWrite:
		return "your role cannot change the built-in registry"
	case authz.PermIngressWrite:
		return "only the owner can change domains or proxy configuration"
	case authz.PermInfraWrite:
		return "your role cannot change infrastructure settings"
	case authz.PermPlatformWrite:
		return "only the owner can change platform settings"
	case authz.PermUserManage:
		return "only the owner can manage users"
	default:
		return "your role cannot perform this action"
	}
}

// sortedPatterns is a test helper kept next to the router so the ordering rule
// lives with the code it describes.
func sortedPatterns(routes map[string]authz.Permission) []string {
	out := make([]string, 0, len(routes))
	for pattern := range routes {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}
