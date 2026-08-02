package mailgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/store"
)

type Config struct {
	StalwartURL    string
	StalwartAPIKey string
}

type Gateway struct {
	baseURL  string
	apiKey   string
	client   *http.Client
	resolver *net.Resolver
}

func New(config Config) (*Gateway, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.StalwartURL), "/")
	apiKey := strings.TrimSpace(config.StalwartAPIKey)
	if (baseURL == "") != (apiKey == "") {
		return nil, errors.New("MAIL_STALWART_URL and MAIL_STALWART_API_KEY must be configured together")
	}
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, errors.New("MAIL_STALWART_URL must be an absolute HTTP or HTTPS URL")
		}
	}
	return &Gateway{
		baseURL:  baseURL,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 15 * time.Second},
		resolver: net.DefaultResolver,
	}, nil
}

func (g *Gateway) Configured() bool { return g != nil && g.baseURL != "" && g.apiKey != "" }

func (g *Gateway) ProvisionDomain(ctx context.Context, name string) (string, []store.MailDNSRecord, error) {
	if !g.Configured() {
		return "", nil, errors.New("Stalwart is not connected")
	}
	create := map[string]any{
		"methodCalls": []any{[]any{"x:Domain/set", map[string]any{"create": map[string]any{"dokyr": map[string]any{
			"name":                  name,
			"aliases":               []string{},
			"certificateManagement": map[string]any{"@type": "Manual"},
			"dkimManagement":        map[string]any{"@type": "Automatic"},
			"dnsManagement":         map[string]any{"@type": "Manual"},
			"subAddressing":         map[string]any{"@type": "Enabled"},
		}}}, "create"}},
		"using": []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"},
	}
	response, err := g.call(ctx, create)
	if err != nil {
		return "", nil, err
	}
	args, method, err := firstMethodResponse(response)
	if err != nil {
		return "", nil, err
	}
	if method == "error" {
		return "", nil, fmt.Errorf("Stalwart rejected the domain: %s", jmapError(args))
	}
	var created struct {
		Created map[string]struct {
			ID string `json:"id"`
		} `json:"created"`
		NotCreated map[string]json.RawMessage `json:"notCreated"`
	}
	if err := json.Unmarshal(args, &created); err != nil {
		return "", nil, fmt.Errorf("decode Stalwart domain response: %w", err)
	}
	id := created.Created["dokyr"].ID
	if id == "" {
		if detail, ok := created.NotCreated["dokyr"]; ok {
			return "", nil, fmt.Errorf("Stalwart rejected the domain: %s", jmapError(detail))
		}
		return "", nil, errors.New("Stalwart did not return the new domain identifier")
	}
	zone, err := g.domainZone(ctx, id)
	if err != nil {
		_ = g.DeleteDomain(context.Background(), id)
		return "", nil, err
	}
	return id, ParseZoneFile(zone, name), nil
}

func (g *Gateway) domainZone(ctx context.Context, id string) (string, error) {
	request := map[string]any{
		"methodCalls": []any{[]any{"x:Domain/get", map[string]any{"ids": []string{id}}, "get"}},
		"using":       []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"},
	}
	response, err := g.call(ctx, request)
	if err != nil {
		return "", err
	}
	args, method, err := firstMethodResponse(response)
	if err != nil {
		return "", err
	}
	if method == "error" {
		return "", fmt.Errorf("Stalwart could not read the domain: %s", jmapError(args))
	}
	var result struct {
		List []struct {
			DNSZoneFile string `json:"dnsZoneFile"`
		} `json:"list"`
	}
	if err := json.Unmarshal(args, &result); err != nil || len(result.List) != 1 {
		return "", errors.New("Stalwart returned an invalid domain record")
	}
	if strings.TrimSpace(result.List[0].DNSZoneFile) == "" {
		return "", errors.New("Stalwart did not generate DNS records for the domain")
	}
	return result.List[0].DNSZoneFile, nil
}

func (g *Gateway) DeleteDomain(ctx context.Context, id string) error {
	if !g.Configured() || strings.TrimSpace(id) == "" {
		return nil
	}
	request := map[string]any{
		"methodCalls": []any{[]any{"x:Domain/set", map[string]any{"destroy": []string{id}}, "delete"}},
		"using":       []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"},
	}
	response, err := g.call(ctx, request)
	if err != nil {
		return err
	}
	args, method, err := firstMethodResponse(response)
	if err != nil {
		return err
	}
	if method == "error" {
		return fmt.Errorf("Stalwart could not remove the domain: %s", jmapError(args))
	}
	var result struct {
		NotDestroyed map[string]json.RawMessage `json:"notDestroyed"`
	}
	if json.Unmarshal(args, &result) == nil {
		if detail, exists := result.NotDestroyed[id]; exists {
			return fmt.Errorf("Stalwart could not remove the domain: %s", jmapError(detail))
		}
	}
	return nil
}

func (g *Gateway) call(ctx context.Context, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/api", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to Stalwart: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Stalwart returned HTTP %d", response.StatusCode)
	}
	var result json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Stalwart response: %w", err)
	}
	return result, nil
}

func firstMethodResponse(response json.RawMessage) (json.RawMessage, string, error) {
	var envelope struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil || len(envelope.MethodResponses) == 0 || len(envelope.MethodResponses[0]) < 2 {
		return nil, "", errors.New("Stalwart returned an invalid JMAP response")
	}
	var method string
	if err := json.Unmarshal(envelope.MethodResponses[0][0], &method); err != nil {
		return nil, "", errors.New("Stalwart returned an invalid JMAP method name")
	}
	return envelope.MethodResponses[0][1], method, nil
}

func jmapError(raw json.RawMessage) string {
	var problem struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if json.Unmarshal(raw, &problem) == nil {
		if problem.Description != "" {
			return problem.Description
		}
		if problem.Type != "" {
			return problem.Type
		}
	}
	return "unknown error"
}

func (g *Gateway) VerifyRecord(ctx context.Context, record store.MailDNSRecord) store.MailDNSRecord {
	record.Status = "missing"
	record.LastError = "record was not found in public DNS"
	switch record.Type {
	case "TXT":
		values, err := g.resolver.LookupTXT(ctx, record.Name)
		if err == nil {
			for _, value := range values {
				if strings.TrimSpace(value) == strings.TrimSpace(record.Value) {
					record.Status, record.LastError = "verified", ""
					return record
				}
			}
			record.Status, record.LastError = "incorrect", "TXT record exists but its value does not match"
		} else if !isNotFound(err) {
			record.LastError = err.Error()
		}
	case "MX":
		values, err := g.resolver.LookupMX(ctx, record.Name)
		if err == nil {
			for _, value := range values {
				if strings.EqualFold(strings.TrimSuffix(value.Host, "."), strings.TrimSuffix(record.Value, ".")) && int(value.Pref) == record.Priority {
					record.Status, record.LastError = "verified", ""
					return record
				}
			}
			record.Status, record.LastError = "incorrect", "MX record exists but its target or priority does not match"
		} else if !isNotFound(err) {
			record.LastError = err.Error()
		}
	case "CNAME":
		value, err := g.resolver.LookupCNAME(ctx, record.Name)
		if err == nil {
			if strings.EqualFold(strings.TrimSuffix(value, "."), strings.TrimSuffix(record.Value, ".")) {
				record.Status, record.LastError = "verified", ""
			} else {
				record.Status, record.LastError = "incorrect", "CNAME record exists but its target does not match"
			}
		} else if !isNotFound(err) {
			record.LastError = err.Error()
		}
	}
	return record
}

func isNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func ParseZoneFile(zone, origin string) []store.MailDNSRecord {
	origin = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(origin)), ".")
	lines := logicalZoneLines(zone)
	records := []store.MailDNSRecord{}
	seen := map[string]bool{}
	for _, line := range lines {
		fields := zoneFields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "$") {
			if len(fields) >= 2 && strings.EqualFold(fields[0], "$ORIGIN") {
				origin = strings.TrimSuffix(strings.ToLower(fields[1]), ".")
			}
			continue
		}
		typeIndex := -1
		for i, field := range fields {
			switch strings.ToUpper(field) {
			case "TXT", "MX", "CNAME":
				typeIndex = i
			}
			if typeIndex >= 0 {
				break
			}
		}
		if typeIndex < 1 || typeIndex+1 >= len(fields) {
			continue
		}
		recordType := strings.ToUpper(fields[typeIndex])
		name := absoluteDNSName(fields[0], origin)
		valueFields := fields[typeIndex+1:]
		priority := 0
		if recordType == "MX" {
			if len(valueFields) < 2 {
				continue
			}
			priority, _ = strconv.Atoi(valueFields[0])
			valueFields = valueFields[1:]
		}
		value := strings.Join(valueFields, "")
		if recordType != "TXT" {
			value = absoluteDNSName(value, origin)
		}
		purpose, required := recordPurpose(recordType, name, value)
		key := recordType + "\x00" + name + "\x00" + value
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		records = append(records, store.MailDNSRecord{Type: recordType, Name: name, Value: value, Priority: priority, Purpose: purpose, Required: required, Status: "pending"})
	}
	return records
}

func recordPurpose(recordType, name, value string) (string, bool) {
	lowerName, lowerValue := strings.ToLower(name), strings.ToLower(value)
	switch {
	case strings.Contains(lowerName, "._domainkey."):
		return "DKIM", true
	case strings.HasPrefix(lowerName, "_dmarc."):
		return "DMARC", false
	case recordType == "TXT" && strings.HasPrefix(lowerValue, "v=spf1"):
		return "SPF", true
	case recordType == "MX":
		return "Return path", true
	case strings.Contains(lowerName, "autoconfig") || strings.Contains(lowerName, "autodiscover"):
		return "Client discovery", false
	default:
		return "Mail configuration", false
	}
}

func absoluteDNSName(name, origin string) string {
	name = strings.TrimSpace(strings.Trim(name, `"`))
	if name == "@" {
		return origin
	}
	if strings.HasSuffix(name, ".") {
		return strings.TrimSuffix(strings.ToLower(name), ".")
	}
	if strings.EqualFold(name, origin) || strings.HasSuffix(strings.ToLower(name), "."+origin) {
		return strings.ToLower(name)
	}
	return strings.ToLower(name + "." + origin)
}

func logicalZoneLines(zone string) []string {
	var lines []string
	var current strings.Builder
	depth := 0
	for _, raw := range strings.Split(zone, "\n") {
		line := stripZoneComment(raw)
		depth += strings.Count(line, "(") - strings.Count(line, ")")
		line = strings.ReplaceAll(strings.ReplaceAll(line, "(", " "), ")", " ")
		if strings.TrimSpace(line) != "" {
			current.WriteByte(' ')
			current.WriteString(strings.TrimSpace(line))
		}
		if depth <= 0 && current.Len() > 0 {
			lines = append(lines, strings.TrimSpace(current.String()))
			current.Reset()
			depth = 0
		}
	}
	if current.Len() > 0 {
		lines = append(lines, strings.TrimSpace(current.String()))
	}
	return lines
}

func stripZoneComment(line string) string {
	quoted, escaped := false, false
	for index, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == ';' && !quoted {
			return line[:index]
		}
	}
	return line
}

func zoneFields(line string) []string {
	fields := []string{}
	var current strings.Builder
	quoted, escaped := false, false
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if (r == ' ' || r == '\t' || r == '\r') && !quoted {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return fields
}
