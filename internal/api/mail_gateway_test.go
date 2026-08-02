package api

import "testing"

func TestValidMailDomain(t *testing.T) {
	for _, value := range []string{"example.com", "send.example.co.uk", "a1-b.example"} {
		if !validMailDomain(value) {
			t.Errorf("%q should be valid", value)
		}
	}
	for _, value := range []string{"localhost", "-bad.example", "bad-.example", "bad_name.example", "*.example.com", "EXAMPLE.com"} {
		if validMailDomain(value) {
			t.Errorf("%q should be invalid", value)
		}
	}
}

func TestAddressDomain(t *testing.T) {
	if got := addressDomain("alerts@Send.Example.com"); got != "send.example.com" {
		t.Fatalf("addressDomain = %q", got)
	}
}
