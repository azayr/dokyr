package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/store"
)

const (
	serviceName    = "registry"
	registryConfig = "/etc/distribution/config.yml"
	requestTimeout = 20 * time.Second
)

// Repository is one image repository with the tags and manifests currently
// stored in the registry.
type Repository struct {
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Images []Image  `json:"images"`
}

// Image is one manifest. Multiple tags can point to the same digest, so the UI
// presents deletion at this level instead of implying that a tag is unlinked.
type Image struct {
	Digest   string     `json:"digest,omitempty"`
	Tags     []string   `json:"tags"`
	Size     int64      `json:"size,omitempty"`
	PushedAt *time.Time `json:"pushedAt,omitempty"`
}

type registryManifest struct {
	Config struct {
		Size int64 `json:"size"`
	} `json:"config"`
	Layers []struct {
		Size int64 `json:"size"`
	} `json:"layers"`
	Manifests []json.RawMessage `json:"manifests"`
}

// Status summarizes the registry container and HTTP API reachability.
type Status struct {
	Available bool   `json:"available"`
	Container string `json:"container,omitempty"`
	State     string `json:"state,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GarbageCollectionResult captures the output of `registry garbage-collect`.
type GarbageCollectionResult struct {
	DryRun    bool   `json:"dryRun"`
	Output    string `json:"output"`
	ExitCode  int    `json:"exitCode"`
	Truncated bool   `json:"truncated"`
}

var ErrNotFound = errors.New("registry is not available")

type Service struct {
	docker *runtime.Docker
	http   *http.Client
	tokens *TokenIssuer
}

func New(docker *runtime.Docker, tokens *TokenIssuer) *Service {
	return &Service{docker: docker, http: &http.Client{Timeout: requestTimeout}, tokens: tokens}
}

// Status reports whether the registry container is present and whether its
// HTTP API answers the /v2/ ping endpoint.
func (s *Service) Status(ctx context.Context) Status {
	container, err := s.docker.ControlPlaneContainerName(ctx, serviceName)
	if err != nil {
		if errors.Is(err, runtime.ErrNotFound) {
			return Status{Available: false, Error: "the registry container is not running in this Compose project"}
		}
		return Status{Available: false, Error: err.Error()}
	}
	address := s.address(ctx)
	if address == "" {
		return Status{Available: false, Container: container, Error: "registry address is not resolvable"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, address+"/v2/", nil)
	if err != nil {
		return Status{Available: false, Container: container, Error: err.Error()}
	}
	if err := s.authorize(req, nil); err != nil {
		return Status{Available: false, Container: container, State: "running", Error: "registry authentication failed: " + err.Error()}
	}
	res, err := s.http.Do(req)
	if err != nil {
		return Status{Available: false, Container: container, State: "running", Error: "registry API is not reachable: " + err.Error()}
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Status{Available: false, Container: container, State: "running", Error: "registry API answered with status " + res.Status}
	}
	return Status{Available: true, Container: container, State: "running"}
}

// Repositories lists the full catalog with every tag for each repository.
func (s *Service) Repositories(ctx context.Context) ([]Repository, error) {
	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := s.get(ctx, "/v2/_catalog", []AccessEntry{{Type: "registry", Name: "catalog", Actions: []string{"*"}}}, &catalog); err != nil {
		return nil, err
	}
	sort.Strings(catalog.Repositories)
	repositories := make([]Repository, 0, len(catalog.Repositories))
	for _, name := range catalog.Repositories {
		var listed struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		}
		if err := s.get(ctx, "/v2/"+repositoryPath(name)+"/tags/list", repositoryAccess(name, "pull"), &listed); err != nil {
			return nil, err
		}
		tags := listed.Tags
		if tags == nil {
			tags = []string{}
		}
		sort.Strings(tags)
		images := make([]Image, 0, len(tags))
		imageIndex := make(map[string]int, len(tags))
		for _, tag := range tags {
			image, err := s.manifestImage(ctx, name, tag)
			if err != nil {
				// Keep the catalog useful when an older or non-standard
				// registry cannot return manifest metadata.
				image = Image{Tags: []string{tag}}
			}
			key := image.Digest
			if key == "" {
				key = "tag:" + tag
			}
			if index, ok := imageIndex[key]; ok {
				images[index].Tags = append(images[index].Tags, tag)
				continue
			}
			image.Tags = []string{tag}
			imageIndex[key] = len(images)
			images = append(images, image)
		}
		sort.Slice(images, func(i, j int) bool {
			if len(images[i].Tags) == 0 || len(images[j].Tags) == 0 {
				return images[i].Digest < images[j].Digest
			}
			return images[i].Tags[0] < images[j].Tags[0]
		})
		repositories = append(repositories, Repository{Name: name, Tags: tags, Images: images})
	}
	return repositories, nil
}

// DeleteTag removes one tag by resolving it to a manifest digest and deleting
// that manifest. Layers remain stored until garbage collection runs.
func (s *Service) DeleteTag(ctx context.Context, name, tag string) error {
	if name == "" || tag == "" {
		return errors.New("repository and tag are required")
	}
	digest, err := s.manifestDigest(ctx, name, tag)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.address(ctx)+"/v2/"+repositoryPath(name)+"/manifests/"+url.PathEscape(digest), nil)
	if err != nil {
		return err
	}
	if err := s.authorize(req, repositoryDeleteAccess(name)); err != nil {
		return err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("delete tag failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

// GarbageCollect runs the distribution garbage collector inside the registry
// container. In dry-run mode nothing is deleted and the output shows what
// would be removed.
func (s *Service) GarbageCollect(ctx context.Context, dryRun bool) (GarbageCollectionResult, error) {
	container, err := s.docker.ControlPlaneContainerName(ctx, serviceName)
	if err != nil {
		if errors.Is(err, runtime.ErrNotFound) {
			return GarbageCollectionResult{}, ErrNotFound
		}
		return GarbageCollectionResult{}, err
	}
	cmd := []string{"registry", "garbage-collect"}
	if dryRun {
		cmd = append(cmd, "--dry-run")
	}
	cmd = append(cmd, registryConfig)
	result, err := s.docker.ExecInContainer(ctx, container, cmd)
	if err != nil {
		return GarbageCollectionResult{}, err
	}
	output := strings.TrimSpace(strings.Join([]string{result.Stdout, result.Stderr}, "\n"))
	return GarbageCollectionResult{DryRun: dryRun, Output: output, ExitCode: result.ExitCode, Truncated: result.Truncated}, nil
}

// ApplySettings recreates the registry container so a changed storage backend
// takes effect. The new environment is generated from the saved settings.
func (s *Service) ApplySettings(ctx context.Context, settings store.RegistrySettings, s3SecretKey string) error {
	return s.docker.RecreateControlPlaneService(ctx, serviceName, environment(settings, s3SecretKey))
}

// Environment renders the registry container environment for a settings row.
// The secret key is supplied decrypted so this function can also be used by
// the API bootstrap path.
func environment(settings store.RegistrySettings, s3SecretKey string) []string {
	values := map[string]string{
		"REGISTRY_STORAGE": settings.Storage,
	}
	if settings.Storage == "s3" {
		values["REGISTRY_STORAGE_S3_REGION"] = settings.S3Region
		values["REGISTRY_STORAGE_S3_BUCKET"] = settings.S3Bucket
		values["REGISTRY_STORAGE_S3_ACCESSKEY"] = settings.S3AccessKey
		values["REGISTRY_STORAGE_S3_SECRETKEY"] = s3SecretKey
		values["REGISTRY_STORAGE_S3_REGIONENDPOINT"] = settings.S3Endpoint
		values["REGISTRY_STORAGE_S3_FORCEPATHSTYLE"] = strconv.FormatBool(settings.S3ForcePathStyle)
		values["REGISTRY_STORAGE_S3_SECURE"] = strconv.FormatBool(settings.S3Secure)
		values["REGISTRY_STORAGE_S3_ENCRYPT"] = "false"
	} else {
		values["REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY"] = "/var/lib/registry"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

// Environment exposes the generated environment for callers that recreate the
// registry container directly.
func Environment(settings store.RegistrySettings, s3SecretKey string) []string {
	return environment(settings, s3SecretKey)
}

func (s *Service) manifestDigest(ctx context.Context, name, tag string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.address(ctx)+"/v2/"+repositoryPath(name)+"/manifests/"+url.PathEscape(tag), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json")
	if err := s.authorize(req, repositoryAccess(name, "pull")); err != nil {
		return "", err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve manifest failed with status %s", res.Status)
	}
	digest := res.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", errors.New("registry did not return a manifest digest")
	}
	return digest, nil
}

func (s *Service) manifestImage(ctx context.Context, name, tag string) (Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.address(ctx)+"/v2/"+repositoryPath(name)+"/manifests/"+url.PathEscape(tag), nil)
	if err != nil {
		return Image{}, err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json")
	if err := s.authorize(req, repositoryAccess(name, "pull")); err != nil {
		return Image{}, err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return Image{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Image{}, ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return Image{}, fmt.Errorf("resolve manifest metadata failed: %s", strings.TrimSpace(string(body)))
	}
	digest := res.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return Image{}, errors.New("registry did not return a manifest digest")
	}
	var manifest registryManifest
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&manifest); err != nil {
		return Image{}, err
	}
	return Image{Digest: digest, Size: manifestContentSize(manifest)}, nil
}

func manifestContentSize(manifest registryManifest) int64 {
	// A single-platform image contains config and layer descriptors. An index
	// only contains child-manifest descriptors, which are not the image bytes,
	// so leave its size unknown rather than showing a misleading total.
	if len(manifest.Manifests) > 0 {
		return 0
	}
	size := manifest.Config.Size
	for _, layer := range manifest.Layers {
		size += layer.Size
	}
	return size
}

func repositoryPath(name string) string {
	segments := strings.Split(strings.Trim(name, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func (s *Service) get(ctx context.Context, path string, access []AccessEntry, out any) error {
	address := s.address(ctx)
	if address == "" {
		return ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+path, nil)
	if err != nil {
		return err
	}
	if err := s.authorize(req, access); err != nil {
		return err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("registry request failed: %s", strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (s *Service) authorize(req *http.Request, access []AccessEntry) error {
	if s.tokens == nil {
		return errors.New("registry token issuer is not configured")
	}
	token, err := s.tokens.IssueServiceToken(access)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func repositoryAccess(name string, actions ...string) []AccessEntry {
	return []AccessEntry{{Type: "repository", Name: name, Actions: actions}}
}

func repositoryDeleteAccess(name string) []AccessEntry {
	return repositoryAccess(name, "delete")
}

// address resolves the registry HTTP endpoint through its container name, which
// is DNS-reachable on the shared control network.
func (s *Service) address(ctx context.Context) string {
	container, err := s.docker.ControlPlaneContainerName(ctx, serviceName)
	if err != nil || container == "" {
		return ""
	}
	return "http://" + container + ":5000"
}
