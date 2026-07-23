package platformupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

const (
	manifestAccept = "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
	indexOCI       = "application/vnd.oci.image.index.v1+json"
	indexDocker    = "application/vnd.docker.distribution.manifest.list.v2+json"
)

type Release struct {
	Version   string    `json:"version"`
	Revision  string    `json:"revision"`
	BuildDate string    `json:"buildDate"`
	Image     string    `json:"image"`
	Digest    string    `json:"digest"`
	CheckedAt time.Time `json:"checkedAt"`
}

type Client struct {
	http       *http.Client
	registry   string
	repository string
	channel    string
}

func NewClient(image, channel string) (*Client, error) {
	image = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(image, "https://"), "http://"))
	parts := strings.SplitN(image, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("platform image must include a registry and repository")
	}
	if channel = strings.TrimSpace(channel); channel == "" {
		channel = "latest"
	}
	return &Client{
		http:       &http.Client{Timeout: 30 * time.Second},
		registry:   parts[0],
		repository: strings.TrimSuffix(parts[1], ":"+channel),
		channel:    channel,
	}, nil
}

func (c *Client) Latest(ctx context.Context) (Release, error) {
	manifest, digest, mediaType, err := c.manifest(ctx, c.channel)
	if err != nil {
		return Release{}, err
	}
	rootDigest := digest
	if mediaType == indexOCI || mediaType == indexDocker {
		var index struct {
			Manifests []struct {
				Digest   string `json:"digest"`
				Platform struct {
					OS           string `json:"os"`
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(manifest, &index); err != nil {
			return Release{}, fmt.Errorf("decode image index: %w", err)
		}
		selected := ""
		for _, item := range index.Manifests {
			if item.Platform.OS == "linux" && item.Platform.Architecture == runtime.GOARCH {
				selected = item.Digest
				break
			}
		}
		if selected == "" {
			return Release{}, fmt.Errorf("release has no linux/%s image", runtime.GOARCH)
		}
		manifest, _, _, err = c.manifest(ctx, selected)
		if err != nil {
			return Release{}, err
		}
	}
	var document struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if err := json.Unmarshal(manifest, &document); err != nil {
		return Release{}, fmt.Errorf("decode image manifest: %w", err)
	}
	if document.Config.Digest == "" || rootDigest == "" {
		return Release{}, errors.New("registry response is missing immutable digests")
	}
	blob, err := c.get(ctx, "/v2/"+c.repository+"/blobs/"+document.Config.Digest, "")
	if err != nil {
		return Release{}, fmt.Errorf("read image metadata: %w", err)
	}
	defer blob.Body.Close()
	var config struct {
		Created string `json:"created"`
		Config  struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.NewDecoder(io.LimitReader(blob.Body, 2<<20)).Decode(&config); err != nil {
		return Release{}, fmt.Errorf("decode image metadata: %w", err)
	}
	labels := config.Config.Labels
	version := strings.TrimSpace(labels["org.opencontainers.image.version"])
	if version == "" {
		version = c.channel
	}
	buildDate := strings.TrimSpace(labels["org.opencontainers.image.created"])
	if buildDate == "" {
		buildDate = config.Created
	}
	image := c.registry + "/" + c.repository
	return Release{
		Version: version, Revision: strings.TrimSpace(labels["org.opencontainers.image.revision"]),
		BuildDate: buildDate, Image: image + "@" + rootDigest, Digest: rootDigest, CheckedAt: time.Now().UTC(),
	}, nil
}

func (c *Client) manifest(ctx context.Context, reference string) ([]byte, string, string, error) {
	res, err := c.get(ctx, "/v2/"+c.repository+"/manifests/"+url.PathEscape(reference), manifestAccept)
	if err != nil {
		return nil, "", "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, "", "", err
	}
	mediaType := strings.TrimSpace(strings.Split(res.Header.Get("Content-Type"), ";")[0])
	if mediaType == "" {
		var header struct {
			MediaType string `json:"mediaType"`
		}
		_ = json.Unmarshal(body, &header)
		mediaType = header.MediaType
	}
	return body, res.Header.Get("Docker-Content-Digest"), mediaType, nil
}

func (c *Client) get(ctx context.Context, path, accept string) (*http.Response, error) {
	request := func(token string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+c.registry+path, nil)
		if err != nil {
			return nil, err
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return c.http.Do(req)
	}
	res, err := request("")
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusUnauthorized {
		return checkedResponse(res)
	}
	challenge := res.Header.Get("WWW-Authenticate")
	res.Body.Close()
	token, err := c.token(ctx, challenge)
	if err != nil {
		return nil, err
	}
	res, err = request(token)
	if err != nil {
		return nil, err
	}
	return checkedResponse(res)
}

func checkedResponse(res *http.Response) (*http.Response, error) {
	if res.StatusCode < 300 {
		return res, nil
	}
	defer res.Body.Close()
	message, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	return nil, fmt.Errorf("registry returned %s: %s", res.Status, strings.TrimSpace(string(message)))
}

func (c *Client) token(ctx context.Context, challenge string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return "", errors.New("registry requires unsupported authentication")
	}
	values := map[string]string{}
	for _, part := range strings.Split(strings.TrimSpace(challenge[len("Bearer "):]), ",") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 {
			values[strings.ToLower(pair[0])] = strings.Trim(pair[1], `"`)
		}
	}
	realm := values["realm"]
	if realm == "" {
		return "", errors.New("registry authentication challenge has no token service")
	}
	query := url.Values{}
	for _, key := range []string{"service", "scope"} {
		if values[key] != "" {
			query.Set(key, values[key])
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realm+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	res, err = checkedResponse(res)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var response struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&response); err != nil {
		return "", err
	}
	if response.Token == "" {
		response.Token = response.AccessToken
	}
	if response.Token == "" {
		return "", errors.New("registry token service returned no token")
	}
	return response.Token, nil
}
