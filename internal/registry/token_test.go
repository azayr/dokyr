package registry

import (
	"crypto/x509"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/azayr/selfhost/internal/store"
	"github.com/golang-jwt/jwt/v5"
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

func TestRepositoryDeleteAccessUsesDeleteAction(t *testing.T) {
	access := repositoryDeleteAccess("mariadb")
	if len(access) != 1 || access[0].Type != "repository" || access[0].Name != "mariadb" {
		t.Fatalf("unexpected delete access: %#v", access)
	}
	if len(access[0].Actions) != 1 || access[0].Actions[0] != "delete" {
		t.Fatalf("delete access must request the registry delete action: %#v", access[0].Actions)
	}
}

func TestIssuedTokenIncludesSigningCertificate(t *testing.T) {
	directory := t.TempDir()
	issuer, err := NewTokenIssuer(TokenAuthConfig{
		Issuer:          "dokyr-registry",
		Service:         "dokyr-registry",
		PrivateKeyPath:  filepath.Join(directory, "registry-token.key"),
		CertificatePath: filepath.Join(directory, "registry-token.crt"),
	})
	if err != nil {
		t.Fatal(err)
	}

	signed, _, _, err := issuer.Issue(store.User{Email: "owner@example.com", Role: "owner"}, TokenRequest{
		Service: "dokyr-registry", Permission: "read_write", Scopes: []string{"repository:team/app:pull,push"},
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := jwt.NewParser().ParseUnverified(signed, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	chain, ok := token.Header["x5c"].([]any)
	if !ok || len(chain) != 1 {
		t.Fatalf("expected one x5c certificate, got %#v", token.Header["x5c"])
	}
	encoded, ok := chain[0].(string)
	if !ok {
		t.Fatalf("expected string certificate, got %#v", chain[0])
	}
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := jwt.Parse(signed, func(*jwt.Token) (any, error) {
		return certificate.PublicKey, nil
	})
	if err != nil || !verified.Valid {
		t.Fatalf("verify token with x5c certificate: token=%#v error=%v", verified, err)
	}
}
