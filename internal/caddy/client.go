package caddy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type Route struct {
	Domain          string      `json:"domain"`
	Upstream        string      `json:"upstream,omitempty"`
	HTTPS           bool        `json:"https"`
	Paths           []PathRoute `json:"paths,omitempty"`
	DefaultPath     string      `json:"defaultPath,omitempty"`
	RejectUnmatched bool        `json:"rejectUnmatched,omitempty"`
}

type PathRoute struct {
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
}

type Client struct {
	mu              sync.RWMutex
	adminURL        string
	controlUpstream string
	controlHosts    []string
	controlDomain   string
	registryHosts   []string
	mailHostname    string
	http            *http.Client
}

func New(adminURL string, controlHosts []string, registryHosts []string, requestedControlUpstream ...string) (*Client, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	if strings.HasPrefix(adminURL, "unix://") {
		socketPath := strings.TrimPrefix(adminURL, "unix://")
		httpClient.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		}
		adminURL = "http://caddy"
	}
	normalizedHosts := make([]string, 0, len(controlHosts))
	seen := map[string]bool{}
	for _, value := range controlHosts {
		host, err := NormalizeControlHost(value)
		if err != nil {
			return nil, fmt.Errorf("control host %q: %w", value, err)
		}
		if !seen[host] {
			normalizedHosts = append(normalizedHosts, host)
			seen[host] = true
		}
	}
	if len(normalizedHosts) == 0 {
		return nil, fmt.Errorf("at least one control host is required")
	}
	normalizedRegistryHosts, err := normalizeOptionalHosts(registryHosts)
	if err != nil {
		return nil, fmt.Errorf("registry host: %w", err)
	}
	if len(normalizedRegistryHosts) == 0 {
		normalizedRegistryHosts = []string{"registry.invalid"}
	}
	controlUpstream := "selfhost:8080"
	if len(requestedControlUpstream) > 0 && strings.TrimSpace(requestedControlUpstream[0]) != "" {
		controlUpstream = requestedControlUpstream[0]
	}
	controlUpstream, err = normalizeControlUpstream(controlUpstream)
	if err != nil {
		return nil, err
	}
	sort.Strings(normalizedHosts)
	sort.Strings(normalizedRegistryHosts)
	return &Client{
		adminURL:        strings.TrimRight(adminURL, "/"),
		controlUpstream: controlUpstream,
		controlHosts:    normalizedHosts,
		registryHosts:   normalizedRegistryHosts,
		http:            httpClient,
	}, nil
}

func normalizeControlUpstream(value string) (string, error) {
	host, port, err := net.SplitHostPort(strings.ToLower(strings.TrimSpace(value)))
	if err != nil || host == "" {
		return "", fmt.Errorf("control upstream must be a hostname and port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("control upstream port must be between 1 and 65535")
	}
	if net.ParseIP(host) == nil {
		for _, label := range strings.Split(host, ".") {
			if !labelPattern.MatchString(label) {
				return "", fmt.Errorf("control upstream must use a valid hostname")
			}
		}
	}
	return net.JoinHostPort(host, port), nil
}

func normalizeOptionalHosts(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		host, err := NormalizeControlHost(value)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", value, err)
		}
		if !seen[host] {
			normalized = append(normalized, host)
			seen[host] = true
		}
	}
	return normalized, nil
}

func NormalizeControlHost(value string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if host == "" || strings.Contains(host, "://") || strings.ContainsAny(host, "/?#") {
		return "", fmt.Errorf("enter a hostname or IP address")
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = strings.Trim(parsedHost, "[]")
	}
	if host == "localhost" || net.ParseIP(host) != nil {
		return host, nil
	}
	domain, err := NormalizeDomain(host)
	if err != nil {
		return "", fmt.Errorf("enter a valid hostname or IP address")
	}
	return domain, nil
}

func NormalizeDomain(value string) (string, error) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if domain == "" {
		return "", nil
	}
	if strings.Contains(domain, "://") || strings.ContainsAny(domain, "/?#") {
		return "", fmt.Errorf("enter only a domain name, without a scheme or path")
	}
	if host, port, err := net.SplitHostPort(domain); err == nil || host != "" || port != "" {
		return "", fmt.Errorf("enter only a domain name, without a port")
	}
	if len(domain) > 253 || net.ParseIP(domain) != nil {
		return "", fmt.Errorf("enter a valid domain name")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain must contain at least one dot")
	}
	for _, label := range labels {
		if !labelPattern.MatchString(label) {
			return "", fmt.Errorf("enter a valid domain name")
		}
	}
	return domain, nil
}

func (c *Client) Apply(ctx context.Context, routes []Route) error {
	body := c.Render(routes)
	return c.ApplyRaw(ctx, body)
}

func (c *Client) Render(routes []Route) string {
	c.mu.RLock()
	mailHostname := c.mailHostname
	controlDomain := c.controlDomain
	controlHosts := append([]string(nil), c.controlHosts...)
	registryHosts := append([]string(nil), c.registryHosts...)
	controlUpstream := c.controlUpstream
	c.mu.RUnlock()
	if controlDomain != "" {
		controlHosts = append(controlHosts, controlDomain)
		sort.Strings(controlHosts)
	}
	return render(routes, controlHosts, registryHosts, controlUpstream, mailHostname, controlDomain)
}

// SetMailHostname reserves the public mail hostname and exposes only
// Stalwart's HTTP-01 ACME challenge path through Caddy's port 80 listener.
func (c *Client) SetMailHostname(hostname string) error {
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if hostname != "" {
		var err error
		hostname, err = NormalizeDomain(hostname)
		if err != nil {
			return fmt.Errorf("mail hostname: %w", err)
		}
	}
	c.mu.Lock()
	c.mailHostname = hostname
	c.mu.Unlock()
	return nil
}

func (c *Client) ControlUpstream() string {
	return c.controlUpstream
}

func (c *Client) ControlHosts() []string {
	c.mu.RLock()
	hosts := append([]string(nil), c.controlHosts...)
	if c.controlDomain != "" {
		hosts = append(hosts, c.controlDomain)
	}
	c.mu.RUnlock()
	sort.Strings(hosts)
	return hosts

}

func (c *Client) ControlDomain() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.controlDomain
}

// SetControlDomain promotes a user-configured hostname to the HTTPS control
// plane route. Environment-provided control hosts remain available as recovery
// addresses if DNS is changed or removed later.
func (c *Client) SetControlDomain(domain string) error {
	normalized, err := NormalizeDomain(domain)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.controlDomain = normalized
	c.mu.Unlock()
	return nil
}

// IsControlHost reports whether domain is one of the hostnames that must reach
// the control panel itself.
//
// Rendered project routes are matched before the control-host route, so binding
// a project to a control hostname would shadow the panel: the operator would
// lose the UI, and whatever the project serves would receive requests — and
// session cookies — meant for Dokyr. Callers reject such a domain before
// persisting it.
func (c *Client) IsControlHost(domain string) bool {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if domain == "" {
		return false
	}
	c.mu.RLock()
	controlDomain := c.controlDomain
	controlHosts := append([]string(nil), c.controlHosts...)
	mailHostname := c.mailHostname
	c.mu.RUnlock()
	if controlDomain == domain {
		return true
	}
	for _, host := range controlHosts {
		if strings.TrimSuffix(strings.ToLower(host), ".") == domain {
			return true
		}
	}
	if mailHostname != "" && mailHostname == domain {
		return true
	}
	return false
}

func (c *Client) Ping(ctx context.Context) error {
	endpoint, err := url.JoinPath(c.adminURL, "config")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("Caddy admin API returned %s", res.Status)
}

func (c *Client) ApplyRaw(ctx context.Context, body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("Caddy configuration cannot be empty")
	}
	endpoint, err := url.JoinPath(c.adminURL, "load")
	if err != nil {
		return fmt.Errorf("build Caddy admin URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("create Caddy request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("reach Caddy admin API: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	return fmt.Errorf("Caddy rejected configuration (%s): %s", res.Status, strings.TrimSpace(string(message)))
}

func render(routes []Route, controlHosts []string, registryHosts []string, controlUpstream, mailHostname, controlDomain string) string {
	sorted := append([]Route(nil), routes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Domain < sorted[j].Domain })
	// Drop any route that claims a control hostname. Callers reject these when
	// the domain is saved; dropping them here as well means a record that
	// predates that check, or one written directly to the database, still cannot
	// take the control panel's hostname away from it.
	control := make(map[string]bool, len(controlHosts))
	for _, host := range controlHosts {
		control[strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")] = true
	}
	if mailHostname != "" {
		control[strings.TrimSuffix(strings.ToLower(strings.TrimSpace(mailHostname)), ".")] = true
	}
	kept := sorted[:0]
	for _, route := range sorted {
		if !control[strings.TrimSuffix(strings.ToLower(route.Domain), ".")] {
			kept = append(kept, route)
		}
	}
	sorted = kept
	var body strings.Builder
	body.WriteString("{\n\tadmin unix//run/caddy-admin/admin.sock\n}\n\n")
	if controlDomain != "" {
		fmt.Fprintf(&body, "%s {\n\tencode zstd gzip\n\treverse_proxy %s\n}\n\n", controlDomain, controlUpstream)
	}
	for _, route := range sorted {
		if route.HTTPS {
			fmt.Fprintf(&body, "%s {\n\tencode zstd gzip\n", route.Domain)
			writeProxyHandlers(&body, route, "tls"+safeMatcherName(route.Domain), "\t")
			body.WriteString("}\n\n")
		}
	}
	body.WriteString(":80 {\n\tencode zstd gzip\n")
	if controlDomain != "" {
		fmt.Fprintf(&body, "\t@controlDomain host %s\n\thandle @controlDomain {\n\t\tredir https://{host}{uri} permanent\n\t}\n", controlDomain)
	}
	// Caddy stops at the first matching handle block, so the control-panel
	// matchers are written before any project route. The panel stays reachable
	// even if a project somehow holds one of its hostnames.
	fmt.Fprintf(&body, "\t@controlIP header_regexp Host \"^(?:[0-9]{1,3}[.]){3}[0-9]{1,3}(?::[0-9]+)?$\"\n\thandle @controlIP {\n\t\treverse_proxy %s\n\t}\n", controlUpstream)
	fmt.Fprintf(&body, "\t@control host %s\n\thandle @control {\n\t\treverse_proxy %s\n\t}\n", strings.Join(controlHosts, " "), controlUpstream)
	if mailHostname != "" {
		fmt.Fprintf(&body, "\t@mailAcme host %s\n\thandle @mailAcme {\n\t\thandle /.well-known/acme-challenge/* {\n\t\t\treverse_proxy stalwart:8080\n\t\t}\n\t\thandle {\n\t\t\trespond \"Not Found\" 404\n\t\t}\n\t}\n", mailHostname)
	}
	for index, route := range sorted {
		fmt.Fprintf(&body, "\t@project%d host %s\n\thandle @project%d {\n", index, route.Domain, index)
		if route.HTTPS {
			body.WriteString("\t\tredir https://{host}{uri} permanent\n")
		} else {
			writeProxyHandlers(&body, route, fmt.Sprintf("project%d", index), "\t\t")
		}
		body.WriteString("\t}\n")
	}
	fmt.Fprintf(&body, "\t@registry host %s\n\thandle @registry {\n\t\thandle /api/registry/token {\n\t\t\treverse_proxy %s\n\t\t}\n\t\treverse_proxy registry:5000\n\t}\n", strings.Join(registryHosts, " "), controlUpstream)
	body.WriteString("\thandle {\n\t\trespond \"Not Found\" 404\n\t}\n}\n")
	return body.String()
}

func writeProxyHandlers(body *strings.Builder, route Route, prefix, indent string) {
	paths := append([]PathRoute(nil), route.Paths...)
	sort.SliceStable(paths, func(i, j int) bool {
		if paths[i].Path == "/*" {
			return false
		}
		if paths[j].Path == "/*" {
			return true
		}
		return len(paths[i].Path) > len(paths[j].Path)
	})
	for index, pathRoute := range paths {
		matcher := fmt.Sprintf("%spath%d", prefix, index)
		fmt.Fprintf(body, "%s@%s path %s\n%shandle @%s {\n%s\treverse_proxy %s\n%s}\n", indent, matcher, pathRoute.Path, indent, matcher, indent, pathRoute.Upstream, indent)
	}
	if route.RejectUnmatched {
		fmt.Fprintf(body, "%shandle {\n%s\trespond \"Not Found\" 404\n%s}\n", indent, indent, indent)
		return
	}
	if route.DefaultPath == "" || route.DefaultPath == "/*" {
		fmt.Fprintf(body, "%shandle {\n%s\treverse_proxy %s\n%s}\n", indent, indent, route.Upstream, indent)
		return
	}
	matcher := prefix + "default"
	fmt.Fprintf(body, "%s@%s path %s\n%shandle @%s {\n%s\treverse_proxy %s\n%s}\n", indent, matcher, route.DefaultPath, indent, matcher, indent, route.Upstream, indent)
	fmt.Fprintf(body, "%shandle {\n%s\trespond \"Not Found\" 404\n%s}\n", indent, indent, indent)
}

func safeMatcherName(value string) string {
	return strings.NewReplacer(".", "_", "-", "_").Replace(value)
}
