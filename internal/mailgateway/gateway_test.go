package mailgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

func TestConfigureServerUpdatesAnExistingStalwartInstallation(t *testing.T) {
	methods := []string{}
	gateway, err := New(Config{StalwartURL: "https://mail.example", StalwartUser: "admin", StalwartPassword: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var method string
		_ = json.Unmarshal(payload.MethodCalls[0][0], &method)
		methods = append(methods, method)
		body := `{"methodResponses":[["x:Bootstrap/get",{"list":[]},"get"]]}`
		if method == "x:SystemSettings/set" {
			var arguments struct {
				Update map[string]struct {
					DefaultHostname string `json:"defaultHostname"`
				} `json:"update"`
			}
			if err := json.Unmarshal(payload.MethodCalls[0][1], &arguments); err != nil {
				t.Fatal(err)
			}
			if arguments.Update["singleton"].DefaultHostname != "mail.example.com" {
				t.Fatalf("hostname = %q", arguments.Update["singleton"].DefaultHostname)
			}
			body = `{"methodResponses":[["x:SystemSettings/set",{"updated":{"singleton":null}},"configure-hostname"]]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	restart, err := gateway.ConfigureServer(t.Context(), "mail.example.com")
	if err != nil || restart {
		t.Fatalf("configure = (restart %v, error %v)", restart, err)
	}
	want := []string{"x:Bootstrap/get", "x:SystemSettings/set"}
	if len(methods) != len(want) || methods[0] != want[0] || methods[1] != want[1] {
		t.Fatalf("methods = %v, want %v", methods, want)
	}
}

func TestValidPublicHostnameRejectsDevelopmentDefaults(t *testing.T) {
	for _, hostname := range []string{"mail.dokyr.test", "mail.localhost", "localhost", "127.0.0.1", "-mail.example.com"} {
		if ValidPublicHostname(hostname) {
			t.Errorf("ValidPublicHostname(%q) = true", hostname)
		}
	}
	if !ValidPublicHostname("mail.example.com") {
		t.Fatal("expected public hostname to be valid")
	}
}

func TestServerHostnameForDomainUsesMailSubdomain(t *testing.T) {
	hostname, err := ServerHostnameForDomain(" Example.COM. ")
	if err != nil || hostname != "mail.example.com" {
		t.Fatalf("ServerHostnameForDomain = %q, %v", hostname, err)
	}
	if domain := DomainForServerHostname(hostname); domain != "example.com" {
		t.Fatalf("DomainForServerHostname = %q", domain)
	}
}

func TestProvisionSMTPKeyCreatesDomainAccount(t *testing.T) {
	gateway, err := New(Config{StalwartURL: "https://mail.example", StalwartAPIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var arguments struct {
			Create map[string]struct {
				Name        string `json:"name"`
				DomainID    string `json:"domainId"`
				Credentials map[string]struct {
					Secret string `json:"secret"`
				} `json:"credentials"`
			} `json:"create"`
		}
		if err := json.Unmarshal(payload.MethodCalls[0][1], &arguments); err != nil {
			t.Fatal(err)
		}
		account := arguments.Create["smtp-key"]
		if account.Name != "smtp-demo" || account.DomainID != "domain1" || account.Credentials["0"].Secret != "dkr_mail_12345678901234567890123456789012" {
			t.Fatalf("unexpected account payload: %#v", account)
		}
		body := `{"methodResponses":[["x:Account/set",{"created":{"smtp-key":{"id":"account1"}}},"create-smtp-key"]]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	id, err := gateway.ProvisionSMTPKey(t.Context(), "domain1", "smtp-demo@example.com", "dkr_mail_12345678901234567890123456789012")
	if err != nil || id != "account1" {
		t.Fatalf("ProvisionSMTPKey = %q, %v", id, err)
	}
}

func TestConfigureSMTPSubmissionPolicyScopesSenderDomain(t *testing.T) {
	gateway, err := New(Config{StalwartURL: "https://mail.example", StalwartAPIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		encoded := string(payload.MethodCalls[0][1])
		for _, expected := range []string{"mustMatchSender", "authenticated_as", "sender_domain", "email_part"} {
			if !strings.Contains(encoded, expected) {
				t.Fatalf("SMTP sender policy does not contain %q: %s", expected, encoded)
			}
		}
		var arguments struct {
			Update map[string]struct {
				MustMatchSender struct {
					Match map[string]json.RawMessage `json:"match"`
				} `json:"mustMatchSender"`
			} `json:"update"`
		}
		if err := json.Unmarshal(payload.MethodCalls[0][1], &arguments); err != nil {
			t.Fatal(err)
		}
		if _, ok := arguments.Update["singleton"].MustMatchSender.Match["0"]; !ok {
			t.Fatalf("SMTP sender policy must use Stalwart 0.16 keyed match shape: %s", encoded)
		}
		body := `{"methodResponses":[["x:MtaStageAuth/set",{"updated":{"singleton":null}},"configure-smtp-senders"]]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	if err := gateway.ConfigureSMTPSubmissionPolicy(t.Context()); err != nil {
		t.Fatal(err)
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
			var arguments struct {
				Create map[string]struct {
					Aliases map[string]bool `json:"aliases"`
				} `json:"create"`
			}
			if err := json.Unmarshal(payload.MethodCalls[0][1], &arguments); err != nil {
				t.Fatalf("decode domain create arguments: %v", err)
			}
			if arguments.Create["dokyr"].Aliases == nil {
				t.Fatal("domain aliases must be encoded as a Stalwart set object")
			}
			body = `{"methodResponses":[["x:Domain/set",{"created":{"dokyr":{"id":"dom1"}}},"create"]]}`
		case "x:Domain/get":
			body = `{"methodResponses":[["x:Domain/get",{"list":[{"dnsZoneFile":"$ORIGIN example.com.\n@ IN MX 10 mail.example.net.\n@ IN TXT \"v=spf1 mx -all\"\nv1._domainkey IN TXT \"v=DKIM1; k=ed25519; p=abc123\""}]},"get"]]}`
		default:
			t.Fatalf("unexpected method %q", method)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	id, records, err := gateway.ProvisionDomain(t.Context(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "dom1" || calls != 2 || len(records) != 3 {
		t.Fatalf("id=%q calls=%d records=%#v", id, calls, records)
	}
}

func TestDeleteDomainRemovesGeneratedDKIMKeysFirst(t *testing.T) {
	methods := []string{}
	gateway, err := New(Config{StalwartURL: "https://mail.example", StalwartAPIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var method string
		_ = json.Unmarshal(payload.MethodCalls[0][0], &method)
		methods = append(methods, method)
		var body string
		switch method {
		case "x:Account/query":
			body = `{"methodResponses":[["x:Account/query",{"ids":[]},"query-domain-accounts"]]}`
		case "x:DkimSignature/query":
			body = `{"methodResponses":[["x:DkimSignature/query",{"ids":["key1","key2"]},"query-dkim"]]}`
		case "x:DkimSignature/set":
			body = `{"methodResponses":[["x:DkimSignature/set",{"destroyed":["key1","key2"]},"delete-dkim"]]}`
		case "x:Domain/set":
			body = `{"methodResponses":[["x:Domain/set",{"destroyed":["domain1"]},"delete"]]}`
		default:
			t.Fatalf("unexpected method %q", method)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	if err := gateway.DeleteDomain(t.Context(), "domain1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"x:Account/query", "x:DkimSignature/query", "x:DkimSignature/set", "x:Domain/set"}
	if len(methods) != len(want) {
		t.Fatalf("methods = %v, want %v", methods, want)
	}
	for index := range want {
		if methods[index] != want[index] {
			t.Fatalf("methods = %v, want %v", methods, want)
		}
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
