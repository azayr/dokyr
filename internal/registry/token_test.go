package registry

import (
	"testing"

	"github.com/azayr/selfhost/internal/store"
)

func TestAllowedAccessScopesPushByRole(t *testing.T) {
	scopes := []string{"repository:team/app:pull,push"}
	viewer := allowedAccess("viewer", "read_write", scopes)
	if len(viewer) != 1 || len(viewer[0].Actions) != 1 || viewer[0].Actions[0] != "pull" {
		t.Fatalf("viewer should only receive pull access: %#v", viewer)
	}
	developer := allowedAccess("developer", "read_write", scopes)
	if len(developer) != 1 || len(developer[0].Actions) != 2 || developer[0].Actions[0] != "pull" || developer[0].Actions[1] != "push" {
		t.Fatalf("developer should receive pull and push access: %#v", developer)
	}
	readOnly := allowedAccess("owner", "read_only", scopes)
	if len(readOnly) != 1 || len(readOnly[0].Actions) != 1 || readOnly[0].Actions[0] != "pull" {
		t.Fatalf("read-only token should only receive pull access: %#v", readOnly)
	}
}

func TestAllowedAccessKeepsRepositoryScope(t *testing.T) {
	access := allowedAccess("owner", "read_write", []string{"repository:team/app:pull", "registry:catalog:*"})
	if len(access) != 1 {
		t.Fatalf("only repository scopes should be granted: %#v", access)
	}
	if access[0].Type != "repository" || access[0].Name != "team/app" || access[0].Actions[0] != "pull" {
		t.Fatalf("unexpected repository scope: %#v", access[0])
	}
}

func TestIssueRejectsAnotherTokenService(t *testing.T) {
	issuer := &TokenIssuer{config: TokenAuthConfig{Service: "dokyr-registry"}}
	_, _, _, err := issuer.Issue(store.User{Email: "owner@example.com", Role: "owner"}, TokenRequest{
		Service: "another-service", Permission: "read_write",
	})
	if err == nil {
		t.Fatal("expected mismatched registry service to be rejected")
	}
}
