package mailgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestParseZoneFileSelectsVerifiableMailRecords(t *testing.T) {
	zone := `$ORIGIN example.com.
@ 3600 IN MX 10 mail.example.net.
@ 3600 IN TXT "v=spf1" " mx -all"
v1-rsa._domainkey 3600 IN TXT ("v=DKIM1; k=rsa; "
  "p=abc123")
_dmarc IN TXT "v=DMARC1; p=none"
autoconfig IN CNAME mail.example.net.
`
	records := ParseZoneFile(zone, "example.com")
	if len(records) != 5 {
		t.Fatalf("records = %#v, want 5", records)
	}
	want := map[string]string{
		"Return path":      "mail.example.net",
		"SPF":              "v=spf1 mx -all",
		"DKIM":             "v=DKIM1; k=rsa; p=abc123",
		"DMARC":            "v=DMARC1; p=none",
		"Client discovery": "mail.example.net",
	}
	for _, record := range records {
		if got := want[record.Purpose]; got != record.Value {
			t.Errorf("%s value = %q, want %q", record.Purpose, record.Value, got)
		}
		if (record.Purpose == "DMARC" || record.Purpose == "Client discovery") && record.Required {
			t.Errorf("%s unexpectedly required", record.Purpose)
		}
	}
}

func TestNewRequiresCompleteStalwartConnection(t *testing.T) {
	if _, err := New(Config{StalwartURL: "https://mail.example.com"}); err == nil {
		t.Fatal("expected incomplete connection to fail")
	}
	if _, err := New(Config{StalwartURL: "https://mail.example.com", StalwartUser: "admin"}); err == nil {
		t.Fatal("expected incomplete basic authentication to fail")
	}
	if gateway, err := New(Config{}); err != nil || gateway.Configured() {
		t.Fatalf("empty optional connection = (%v, %v)", gateway, err)
	}
}

func TestProvisionDomainCreatesThenFetchesDNSZone(t *testing.T) {
	calls := 0
	gateway, err := New(Config{StalwartURL: "https://mail.example", StalwartAPIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/jmap" {
			t.Fatalf("path = %q, want /jmap", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var method string
		_ = json.Unmarshal(payload.MethodCalls[0][0], &method)
		var body string
		switch method {
		case "x:Domain/set":
			body = `{"methodResponses":[["x:Domain/set",{"created":{"dokyr":{"id":"dom1"}}},"create"]]}`
		case "x:Domain/get":
			body = `{"methodResponses":[["x:Domain/get",{"list":[{"dnsZoneFile":"$ORIGIN example.com.\n@ IN MX 10 mail.example.net.\n@ IN TXT \"v=spf1 mx -all\""}]},"get"]]}`
		default:
			t.Fatalf("unexpected method %q", method)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	id, records, err := gateway.ProvisionDomain(t.Context(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "dom1" || calls != 2 || len(records) != 2 {
		t.Fatalf("id=%q calls=%d records=%#v", id, calls, records)
	}
}

func TestEnsureBootstrapUsesBasicAuthAndRequestsRestart(t *testing.T) {
	calls := 0
	gateway, err := New(Config{
		StalwartURL: "http://stalwart:8080", StalwartUser: "admin", StalwartPassword: "secret",
		BootstrapHostname: "mail.example.com", BootstrapDefaultDomain: "dokyr.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "secret" {
			t.Fatalf("basic auth = (%q, %q, %v)", username, password, ok)
		}
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var method string
		_ = json.Unmarshal(payload.MethodCalls[0][0], &method)
		body := `{"methodResponses":[["x:Bootstrap/get",{"list":[{"id":"singleton"}]},"get"]]}`
		if method == "x:Bootstrap/set" {
			body = `{"methodResponses":[["x:Bootstrap/set",{"updated":{"singleton":{"username":"admin@dokyr.test","secret":"generated"}}},"bootstrap"]]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	restart, err := gateway.EnsureBootstrap(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !restart || calls != 2 {
		t.Fatalf("restart=%v calls=%d, want true and 2", restart, calls)
	}
}

func TestPrepareSenderCreatesDomainAccount(t *testing.T) {
	calls := 0
	gateway, err := New(Config{
		StalwartURL: "http://stalwart:8080", StalwartUser: "admin", StalwartPassword: "secret",
		RelayHost: "stalwart", RelayPort: 465, RelayPassword: "relay-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var method string
		_ = json.Unmarshal(payload.MethodCalls[0][0], &method)
		body := `{"methodResponses":[["x:Account/query",{"ids":[]},"query"]]}`
		if method == "x:Account/set" {
			body = `{"methodResponses":[["x:Account/set",{"created":{"dokyr":{"id":"account1"}}},"create"]]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	relay, err := gateway.PrepareSender(t.Context(), "domain1", "Hello@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || relay.Username != "hello@example.com" || relay.Password != "relay-secret" || !relay.InsecureSkipVerify {
		t.Fatalf("calls=%d relay=%+v", calls, relay)
	}
}
