package secretbox

import "testing"

func TestRoundTrip(t *testing.T) {
	box, err := New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Encrypt("top-secret")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "top-secret" {
		t.Fatal("secret was not encrypted")
	}
	plain, err := box.Decrypt(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "top-secret" {
		t.Fatalf("got %q", plain)
	}
}

func TestRejectsShortKey(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatal("expected an error")
	}
}
