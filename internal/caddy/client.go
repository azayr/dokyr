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
	"strings"
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
	adminURL     string
	controlHosts []string
	http         *http.Client
}

func New(adminURL string, controlHosts []string) (*Client, error) {
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
	sort.Strings(normalizedHosts)
	return &Client{
		adminURL:     strings.TrimRight(adminURL, "/"),
		controlHosts: normalizedHosts,
		http:         httpClient,
	}, nil
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
	return render(routes, c.controlHosts)
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

func render(routes []Route, controlHosts []string) string {
	sorted := append([]Route(nil), routes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Domain < sorted[j].Domain })
	var body strings.Builder
	body.WriteString("{\n\tadmin unix//run/caddy-admin/admin.sock\n}\n\n")
	for _, route := range sorted {
		if route.HTTPS {
			fmt.Fprintf(&body, "%s {\n\tencode zstd gzip\n", route.Domain)
			writeProxyHandlers(&body, route, "tls"+safeMatcherName(route.Domain), "\t")
			body.WriteString("}\n\n")
		}
	}
	body.WriteString(":80 {\n\tencode zstd gzip\n")
	for index, route := range sorted {
		fmt.Fprintf(&body, "\t@project%d host %s\n\thandle @project%d {\n", index, route.Domain, index)
		if route.HTTPS {
			body.WriteString("\t\tredir https://{host}{uri} permanent\n")
		} else {
			writeProxyHandlers(&body, route, fmt.Sprintf("project%d", index), "\t\t")
		}
		body.WriteString("\t}\n")
	}
	body.WriteString("\t@controlIP header_regexp Host \"^(?:[0-9]{1,3}[.]){3}[0-9]{1,3}(?::[0-9]+)?$\"\n\thandle @controlIP {\n\t\treverse_proxy selfhost:8080\n\t}\n")
	fmt.Fprintf(&body, "\t@control host %s\n\thandle @control {\n\t\treverse_proxy selfhost:8080\n\t}\n", strings.Join(controlHosts, " "))
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
