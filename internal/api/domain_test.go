package api

import "testing"

func TestDomainDNSInstructionUsesPublicURLTarget(t *testing.T) {
	tests := []struct {
		name       string
		publicURL  string
		recordType string
		value      string
	}{
		{name: "IPv4", publicURL: "http://203.0.113.10:8888", recordType: "A", value: "203.0.113.10"},
		{name: "IPv6", publicURL: "https://[2001:db8::10]", recordType: "AAAA", value: "2001:db8::10"},
		{name: "hostname", publicURL: "https://panel.example.com", recordType: "CNAME", value: "panel.example.com"},
		{name: "local development", publicURL: "http://localhost:8888", recordType: "A", value: "127.0.0.1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &API{publicURL: test.publicURL}
			got := api.domainDNSInstruction("app.example.com")
			if got.Type != test.recordType || got.Value != test.value || got.Name != "app.example.com" {
				t.Fatalf("domainDNSInstruction() = %#v, want type=%q name=%q value=%q", got, test.recordType, "app.example.com", test.value)
			}
		})
	}
}
