package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("docker resource not found")

type Health struct {
	Connected  bool      `json:"connected"`
	Version    string    `json:"version"`
	Containers int       `json:"containers"`
	Running    int       `json:"running"`
	CheckedAt  time.Time `json:"checkedAt"`
	Error      string    `json:"error,omitempty"`
}

type RegistryAuth struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	ServerAddress string `json:"serveraddress"`
}

type ProgressFunc func(stage, eventType, message string)

type Service struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	Container string `json:"container"`
	HostPort  string `json:"hostPort,omitempty"`
}

type DatabasePreset struct {
	Engine       string
	Image        string
	Port         int
	VolumeTarget string
}

type DatabaseSpec struct {
	ID            string
	ProjectID     string
	Engine        string
	Image         string
	Port          int
	VolumeName    string
	Username      string
	DatabaseName  string
	Password      string
	PublicEnabled bool
	PublicPort    int
}

type DatabaseRuntime struct {
	Status    string `json:"status"`
	Container string `json:"container"`
	HostPort  int    `json:"hostPort,omitempty"`
	Health    string `json:"health,omitempty"`
}

func DatabaseEngine(engine string) (DatabasePreset, bool) {
	switch engine {
	case "mysql":
		return DatabasePreset{Engine: engine, Image: "mysql:8.4", Port: 3306, VolumeTarget: "/var/lib/mysql"}, true
	case "mariadb":
		return DatabasePreset{Engine: engine, Image: "mariadb:11.8", Port: 3306, VolumeTarget: "/var/lib/mysql"}, true
	case "postgres":
		return DatabasePreset{Engine: engine, Image: "postgres:17-alpine", Port: 5432, VolumeTarget: "/var/lib/postgresql/data"}, true
	default:
		return DatabasePreset{}, false
	}
}

type Docker struct {
	client  *http.Client
	socket  string
	metrics *metricsCache
}

func NewDocker() (*Docker, error) {
	socket := "/var/run/docker.sock"
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		socket = strings.TrimPrefix(host, "unix://")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &Docker{client: &http.Client{Transport: transport, Timeout: 10 * time.Minute}, socket: socket, metrics: newMetricsCache()}, nil
}

func (d *Docker) Close() error { d.client.CloseIdleConnections(); return nil }

func (d *Docker) request(ctx context.Context, method, path string, body any, headers map[string]string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, ErrNotFound
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
		var problem struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(message, &problem) == nil && problem.Message != "" {
			return nil, fmt.Errorf("docker: %s", problem.Message)
		}
		return nil, fmt.Errorf("docker API returned %s", res.Status)
	}
	return res, nil
}

func (d *Docker) rawRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
		return nil, fmt.Errorf("docker API returned %s: %s", res.Status, strings.TrimSpace(string(message)))
	}
	return res, nil
}

func (d *Docker) get(ctx context.Context, path string, out any) error {
	res, err := d.request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(out)
}

func (d *Docker) Health(ctx context.Context) Health {
	h := Health{CheckedAt: time.Now().UTC()}
	var version struct {
		Version string `json:"Version"`
	}
	if err := d.get(ctx, "/version", &version); err != nil {
		h.Error = err.Error()
		return h
	}
	var info struct {
		Containers        int `json:"Containers"`
		ContainersRunning int `json:"ContainersRunning"`
	}
	if err := d.get(ctx, "/info", &info); err != nil {
		h.Error = err.Error()
		return h
	}
	h.Connected = true
	h.Version = version.Version
	h.Containers = info.Containers
	h.Running = info.ContainersRunning
	return h
}

func containerName(projectID string) string            { return "selfhost-" + projectID }
func applicationContainerName(serviceID string) string { return "selfhost-svc-" + serviceID }
func databaseContainerName(serviceID string) string    { return "selfhost-db-" + serviceID }

func imagePullQuery(image string) string {
	reference := strings.TrimSpace(image)
	fromImage := reference
	tag := "latest"

	if digestSeparator := strings.LastIndex(reference, "@"); digestSeparator >= 0 {
		fromImage = reference[:digestSeparator]
		if digest := reference[digestSeparator+1:]; digest != "" {
			tag = digest
		}
	} else if tagSeparator := strings.LastIndex(reference, ":"); tagSeparator > strings.LastIndex(reference, "/") {
		fromImage = reference[:tagSeparator]
		if explicitTag := reference[tagSeparator+1:]; explicitTag != "" {
			tag = explicitTag
		}
	}

	query := url.Values{}
	query.Set("fromImage", fromImage)
	query.Set("tag", tag)
	return query.Encode()
}

func (d *Docker) pullImage(ctx context.Context, image string, headers map[string]string, progress ProgressFunc) error {
	res, err := d.request(ctx, http.MethodPost, "/images/create?"+imagePullQuery(image), nil, headers)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	for decoder.More() {
		var update struct {
			Status      string `json:"status"`
			ID          string `json:"id"`
			Progress    string `json:"progress"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&update); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if update.Error != "" || update.ErrorDetail.Message != "" {
			message := update.ErrorDetail.Message
			if message == "" {
				message = update.Error
			}
			return errors.New(message)
		}
		if progress != nil && update.Status != "" {
			message := update.Status
			if update.ID != "" {
				message = update.ID + ": " + message
			}
			if strings.TrimSpace(update.Progress) != "" {
				message += " " + strings.TrimSpace(update.Progress)
			}
			progress("pull", "log", message)
		}
	}
	return nil
}

func (d *Docker) DeployImage(ctx context.Context, projectID, image string, containerPort int, auth *RegistryAuth, progress ProgressFunc) (Service, error) {
	if containerPort < 1 || containerPort > 65535 {
		return Service{}, fmt.Errorf("container port must be between 1 and 65535")
	}
	headers := map[string]string{}
	if auth != nil {
		encoded, err := json.Marshal(auth)
		if err != nil {
			return Service{}, err
		}
		headers["X-Registry-Auth"] = base64.RawURLEncoding.EncodeToString(encoded)
	}
	if progress != nil {
		progress("pull", "start", "Pulling "+image)
	}
	if err := d.pullImage(ctx, image, headers, progress); err != nil {
		return Service{}, fmt.Errorf("pull image: %w", err)
	}
	if progress != nil {
		progress("pull", "complete", "Image is ready locally")
		progress("replace", "start", "Checking for an existing container")
	}

	name := containerName(projectID)
	if existing, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil); err == nil {
		existing.Body.Close()
		if progress != nil {
			progress("replace", "log", "Stopped and removed the previous container")
		}
	} else if !errors.Is(err, ErrNotFound) {
		return Service{}, fmt.Errorf("replace container: %w", err)
	} else if progress != nil {
		progress("replace", "log", "No previous container found")
	}
	if progress != nil {
		progress("replace", "complete", "Container slot is ready")
		progress("create", "start", "Creating "+name)
	}

	portKey := fmt.Sprintf("%d/tcp", containerPort)
	createBody := map[string]any{
		"Image":        image,
		"ExposedPorts": map[string]any{portKey: map[string]any{}},
		"Labels": map[string]string{
			"selfhost.managed":      "true",
			"selfhost.project.id":   projectID,
			"selfhost.project.port": fmt.Sprint(containerPort),
		},
		"HostConfig": map[string]any{
			"NetworkMode":     "selfhost-proxy",
			"PublishAllPorts": false,
			"RestartPolicy":   map[string]string{"Name": "unless-stopped"},
			"SecurityOpt":     []string{"no-new-privileges:true"},
		},
	}
	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), createBody, nil)
	if err != nil {
		return Service{}, fmt.Errorf("create container: %w", err)
	}
	created.Body.Close()
	if progress != nil {
		progress("create", "complete", "Container created on selfhost-proxy")
		progress("start", "start", "Starting "+name)
	}

	started, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
	if err != nil {
		return Service{}, fmt.Errorf("start container: %w", err)
	}
	started.Body.Close()
	if progress != nil {
		progress("start", "complete", "Container process started")
		progress("verify", "start", "Inspecting the running service")
	}
	service, err := d.ProjectService(ctx, projectID)
	if err != nil {
		return Service{}, err
	}
	if progress != nil {
		progress("verify", "complete", "Runtime reports "+service.Status)
	}
	return service, nil
}

func (d *Docker) DeployApplicationImage(ctx context.Context, serviceID, projectID, serviceName, image string, containerPort int, environment []string, command string, auth *RegistryAuth, progress ProgressFunc) (Service, error) {
	if containerPort < 1 || containerPort > 65535 {
		return Service{}, fmt.Errorf("container port must be between 1 and 65535")
	}
	headers := map[string]string{}
	if auth != nil {
		encoded, err := json.Marshal(auth)
		if err != nil {
			return Service{}, err
		}
		headers["X-Registry-Auth"] = base64.RawURLEncoding.EncodeToString(encoded)
	}
	if progress != nil {
		progress("pull", "start", "Pulling "+image)
	}
	if err := d.pullImage(ctx, image, headers, progress); err != nil {
		return Service{}, fmt.Errorf("pull image: %w", err)
	}
	if progress != nil {
		progress("pull", "complete", "Image is ready locally")
	}
	return d.deployApplicationContainer(ctx, serviceID, projectID, serviceName, image, containerPort, environment, command, progress)
}

func (d *Docker) BuildApplicationImage(ctx context.Context, serviceID, sourceDir, buildStrategy, dockerfilePath, buildContext string, progress ProgressFunc) (string, error) {
	image := "selfhost-built-" + serviceID + ":latest"
	switch buildStrategy {
	case "", "dockerfile":
		return d.buildDockerfileImage(ctx, image, sourceDir, dockerfilePath, buildContext, progress)
	case "nixpacks":
		return d.buildWithPack(ctx, "nixpacks", image, sourceDir, progress)
	case "railpack":
		return d.buildWithPack(ctx, "railpack", image, sourceDir, progress)
	default:
		return "", fmt.Errorf("unsupported build strategy %q", buildStrategy)
	}
}

func (d *Docker) buildDockerfileImage(ctx context.Context, image, sourceDir, dockerfilePath, buildContext string, progress ProgressFunc) (string, error) {
	contextDir := filepath.Clean(filepath.Join(sourceDir, buildContext))
	repositoryDir := filepath.Clean(sourceDir)
	if contextDir != repositoryDir && !strings.HasPrefix(contextDir, repositoryDir+string(os.PathSeparator)) {
		return "", errors.New("build context escapes the repository")
	}
	dockerfile := filepath.Clean(filepath.Join(repositoryDir, dockerfilePath))
	relativeDockerfile, err := filepath.Rel(contextDir, dockerfile)
	if err != nil || relativeDockerfile == ".." || strings.HasPrefix(relativeDockerfile, ".."+string(os.PathSeparator)) {
		return "", errors.New("Dockerfile must be inside the build context")
	}
	if _, err := os.Stat(dockerfile); err != nil {
		return "", fmt.Errorf("find Dockerfile: %w", err)
	}
	reader, writer := io.Pipe()
	go func() {
		tarWriter := tar.NewWriter(writer)
		err := filepath.Walk(contextDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(contextDir, path)
			if err != nil {
				return err
			}
			if relative == ".git" || strings.HasPrefix(relative, ".git"+string(os.PathSeparator)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relative)
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tarWriter, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		})
		if closeErr := tarWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
	}()
	query := url.Values{"t": {image}, "dockerfile": {filepath.ToSlash(relativeDockerfile)}, "rm": {"1"}, "forcerm": {"1"}}
	if progress != nil {
		progress("build", "start", "Building "+image+" from "+filepath.ToSlash(buildContext)+"/"+filepath.ToSlash(relativeDockerfile))
	}
	res, err := d.rawRequest(ctx, http.MethodPost, "/build?"+query.Encode(), reader, "application/x-tar")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	for decoder.More() {
		var update struct {
			Stream      string `json:"stream"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&update); err != nil {
			return "", err
		}
		if update.Error != "" || update.ErrorDetail.Message != "" {
			message := update.ErrorDetail.Message
			if message == "" {
				message = update.Error
			}
			return "", errors.New(message)
		}
		if progress != nil {
			for _, line := range strings.Split(strings.TrimSpace(update.Stream), "\n") {
				if line != "" {
					progress("build", "log", line)
				}
			}
		}
	}
	if progress != nil {
		progress("build", "complete", "Image built successfully")
	}
	return image, nil
}

type buildProgressWriter struct {
	mu       sync.Mutex
	pending  string
	progress ProgressFunc
}

func (w *buildProgressWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending += string(value)
	for {
		index := strings.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSpace(w.pending[:index])
		w.pending = w.pending[index+1:]
		if line != "" && w.progress != nil {
			w.progress("build", "log", line)
		}
	}
	return len(value), nil
}

func (w *buildProgressWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if line := strings.TrimSpace(w.pending); line != "" && w.progress != nil {
		w.progress("build", "log", line)
	}
	w.pending = ""
}

func (d *Docker) buildWithPack(ctx context.Context, pack, image, sourceDir string, progress ProgressFunc) (string, error) {
	if progress != nil {
		progress("build", "start", "Analyzing the repository with "+strings.Title(pack))
	}
	var command *exec.Cmd
	switch pack {
	case "nixpacks":
		command = exec.CommandContext(ctx, "nixpacks", "build", sourceDir, "--name", image, "--docker-host", "unix:///var/run/docker.sock")
	case "railpack":
		if buildkitHost() == "" {
			return "", errors.New("Railpack requires SELFHOST_BUILDKIT_HOST to point to a reachable BuildKit daemon")
		}
		command = exec.CommandContext(ctx, "railpack", "build", "--name", image, "--progress", "plain", sourceDir)
		command.Env = append(os.Environ(), "BUILDKIT_HOST="+buildkitHost())
	default:
		return "", fmt.Errorf("unsupported build pack %q", pack)
	}
	writer := &buildProgressWriter{progress: progress}
	command.Stdout = writer
	command.Stderr = writer
	if err := command.Run(); err != nil {
		writer.Flush()
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%s is not installed in this Selfhost image; rebuild Selfhost with the build-pack tools enabled", pack)
		}
		return "", fmt.Errorf("%s build: %w", pack, err)
	}
	writer.Flush()
	if progress != nil {
		progress("build", "complete", strings.Title(pack)+" built "+image)
	}
	return image, nil
}

func buildkitHost() string {
	if value := strings.TrimSpace(os.Getenv("SELFHOST_BUILDKIT_HOST")); value != "" {
		return value
	}
	return ""
}

func (d *Docker) DeployApplicationBuiltImage(ctx context.Context, serviceID, projectID, serviceName, image string, containerPort int, environment []string, command string, progress ProgressFunc) (Service, error) {
	return d.deployApplicationContainer(ctx, serviceID, projectID, serviceName, image, containerPort, environment, command, progress)
}

func (d *Docker) deployApplicationContainer(ctx context.Context, serviceID, projectID, serviceName, image string, containerPort int, environment []string, command string, progress ProgressFunc) (Service, error) {
	commandArguments, err := parseContainerCommand(command)
	if err != nil {
		return Service{}, fmt.Errorf("invalid container command: %w", err)
	}
	if progress != nil {
		progress("replace", "start", "Checking for an existing service container")
	}

	name := applicationContainerName(serviceID)
	if existing, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil); err == nil {
		existing.Body.Close()
		if progress != nil {
			progress("replace", "log", "Stopped and removed the previous service container")
		}
	} else if !errors.Is(err, ErrNotFound) {
		return Service{}, fmt.Errorf("replace container: %w", err)
	}
	if progress != nil {
		progress("replace", "complete", "Service container slot is ready")
		progress("create", "start", "Creating "+name)
	}

	portKey := fmt.Sprintf("%d/tcp", containerPort)
	createBody := map[string]any{
		"Image":        image,
		"Env":          environment,
		"ExposedPorts": map[string]any{portKey: map[string]any{}},
		"Labels": map[string]string{
			"selfhost.managed":      "true",
			"selfhost.project.id":   projectID,
			"selfhost.service.id":   serviceID,
			"selfhost.service.name": serviceName,
			"selfhost.service.kind": "application",
			"selfhost.project.port": fmt.Sprint(containerPort),
		},
		"HostConfig": map[string]any{
			"NetworkMode":     "selfhost-proxy",
			"PublishAllPorts": false,
			"RestartPolicy":   map[string]string{"Name": "unless-stopped"},
			"SecurityOpt":     []string{"no-new-privileges:true"},
		},
	}
	if len(commandArguments) > 0 {
		createBody["Cmd"] = commandArguments
	}
	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), createBody, nil)
	if err != nil {
		return Service{}, fmt.Errorf("create container: %w", err)
	}
	created.Body.Close()
	if progress != nil {
		if len(commandArguments) > 0 {
			progress("create", "log", "Container command: "+command)
		}
		progress("create", "complete", "Service container created on selfhost-proxy")
		progress("start", "start", "Starting "+name)
	}
	started, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
	if err != nil {
		return Service{}, fmt.Errorf("start container: %w", err)
	}
	started.Body.Close()
	if progress != nil {
		progress("start", "complete", "Service process started")
	}
	return d.ApplicationService(ctx, serviceID, serviceName)
}

func parseContainerCommand(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	arguments := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	tokenStarted := false
	flush := func() {
		if tokenStarted {
			arguments = append(arguments, current.String())
			current.Reset()
			tokenStarted = false
		}
	}
	for _, character := range value {
		if escaped {
			current.WriteRune(character)
			tokenStarted = true
			escaped = false
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			tokenStarted = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				current.WriteRune(character)
			}
			tokenStarted = true
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			tokenStarted = true
		case ' ', '\t', '\r', '\n':
			flush()
		default:
			current.WriteRune(character)
			tokenStarted = true
		}
	}
	if escaped {
		return nil, errors.New("command ends with an escape character")
	}
	if quote != 0 {
		return nil, errors.New("command contains an unterminated quote")
	}
	flush()
	return arguments, nil
}

func mergeEnvironment(existing, replacement, replacedKeys []string) []string {
	replaced := make(map[string]bool, len(replacedKeys)+len(replacement))
	for _, key := range replacedKeys {
		replaced[key] = true
	}
	for _, variable := range replacement {
		key, _, _ := strings.Cut(variable, "=")
		replaced[key] = true
	}
	merged := make([]string, 0, len(existing)+len(replacement))
	for _, variable := range existing {
		key, _, _ := strings.Cut(variable, "=")
		if !replaced[key] {
			merged = append(merged, variable)
		}
	}
	return append(merged, replacement...)
}

func (d *Docker) RestartProjectWithEnvironment(ctx context.Context, projectID string, environment, previousKeys []string) (Service, error) {
	return d.restartContainerWithEnvironment(ctx, containerName(projectID), environment, previousKeys, func() (Service, error) {
		return d.ProjectService(ctx, projectID)
	})
}

func (d *Docker) RestartApplicationWithEnvironment(ctx context.Context, serviceID, serviceName string, environment, previousKeys []string) (Service, error) {
	return d.restartContainerWithEnvironment(ctx, applicationContainerName(serviceID), environment, previousKeys, func() (Service, error) {
		return d.ApplicationService(ctx, serviceID, serviceName)
	})
}

func (d *Docker) restartContainerWithEnvironment(ctx context.Context, name string, environment, previousKeys []string, inspect func() (Service, error)) (Service, error) {
	backupName := name + "-environment-backup"
	var inspected struct {
		Config     map[string]any `json:"Config"`
		HostConfig map[string]any `json:"HostConfig"`
		State      struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	if err := d.get(ctx, "/containers/"+url.PathEscape(name)+"/json", &inspected); err != nil {
		return Service{}, err
	}
	existingEnvironment := []string{}
	if values, ok := inspected.Config["Env"].([]any); ok {
		for _, value := range values {
			if text, ok := value.(string); ok {
				existingEnvironment = append(existingEnvironment, text)
			}
		}
	} else if values, ok := inspected.Config["Env"].([]string); ok {
		existingEnvironment = values
	}
	inspected.Config["Env"] = mergeEnvironment(existingEnvironment, environment, previousKeys)
	delete(inspected.Config, "Hostname")
	delete(inspected.Config, "Domainname")
	createBody := inspected.Config
	createBody["HostConfig"] = inspected.HostConfig

	if stale, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(backupName)+"?force=1", nil, nil); err == nil {
		stale.Body.Close()
	} else if !errors.Is(err, ErrNotFound) {
		return Service{}, fmt.Errorf("clear previous restart backup: %w", err)
	}
	if inspected.State.Running {
		stopped, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/stop?t=10", nil, nil)
		if err != nil {
			return Service{}, fmt.Errorf("stop existing container: %w", err)
		}
		stopped.Body.Close()
	}
	renamed, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/rename?name="+url.QueryEscape(backupName), nil, nil)
	if err != nil {
		if inspected.State.Running {
			if restarted, startErr := d.request(context.Background(), http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil); startErr == nil {
				restarted.Body.Close()
			}
		}
		return Service{}, fmt.Errorf("prepare existing container: %w", err)
	}
	renamed.Body.Close()
	restore := func() {
		if created, removeErr := d.request(context.Background(), http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil); removeErr == nil {
			created.Body.Close()
		}
		if restored, renameErr := d.request(context.Background(), http.MethodPost, "/containers/"+url.PathEscape(backupName)+"/rename?name="+url.QueryEscape(name), nil, nil); renameErr == nil {
			restored.Body.Close()
			if inspected.State.Running {
				if started, startErr := d.request(context.Background(), http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil); startErr == nil {
					started.Body.Close()
				}
			}
		}
	}

	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), createBody, nil)
	if err != nil {
		restore()
		return Service{}, fmt.Errorf("recreate container: %w", err)
	}
	created.Body.Close()
	started, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
	if err != nil {
		restore()
		return Service{}, fmt.Errorf("restart container: %w", err)
	}
	started.Body.Close()
	service, err := inspect()
	if err != nil || service.Status != "healthy" {
		restore()
		if err != nil {
			return Service{}, fmt.Errorf("verify restarted container: %w", err)
		}
		return Service{}, errors.New("restarted container did not remain running")
	}
	removed, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(backupName)+"?force=1", nil, nil)
	if err == nil {
		removed.Body.Close()
	}
	return service, nil
}

func (d *Docker) DeployDatabase(ctx context.Context, spec DatabaseSpec, progress ...ProgressFunc) (DatabaseRuntime, error) {
	var report ProgressFunc
	if len(progress) > 0 {
		report = progress[0]
	}
	preset, ok := DatabaseEngine(spec.Engine)
	if !ok {
		return DatabaseRuntime{}, fmt.Errorf("unsupported database engine %q", spec.Engine)
	}
	if report != nil {
		report("pull", "start", "Pulling "+spec.Image)
	}
	if err := d.pullImage(ctx, spec.Image, nil, report); err != nil {
		return DatabaseRuntime{}, fmt.Errorf("pull database image: %w", err)
	}
	if report != nil {
		report("pull", "complete", "Database image is ready")
		report("volume", "start", "Preparing persistent volume "+spec.VolumeName)
	}
	volumeBody := map[string]any{
		"Name": spec.VolumeName,
		"Labels": map[string]string{
			"selfhost.managed":     "true",
			"selfhost.project.id":  spec.ProjectID,
			"selfhost.database.id": spec.ID,
		},
	}
	volume, err := d.request(ctx, http.MethodPost, "/volumes/create", volumeBody, nil)
	if err != nil {
		return DatabaseRuntime{}, fmt.Errorf("create database volume: %w", err)
	}
	volume.Body.Close()
	if report != nil {
		report("volume", "complete", "Persistent volume is ready")
		report("replace", "start", "Checking for an existing database container")
	}

	name := databaseContainerName(spec.ID)
	if existing, deleteErr := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil); deleteErr == nil {
		existing.Body.Close()
	} else if !errors.Is(deleteErr, ErrNotFound) {
		return DatabaseRuntime{}, fmt.Errorf("replace database container: %w", deleteErr)
	}
	if report != nil {
		report("replace", "complete", "Container slot is ready")
		report("create", "start", "Creating database container")
	}

	portKey := fmt.Sprintf("%d/tcp", spec.Port)
	environment := []string{}
	var command []string
	var healthTest []string
	switch spec.Engine {
	case "mysql":
		environment = []string{"MYSQL_ROOT_PASSWORD=" + spec.Password, "MYSQL_DATABASE=" + spec.DatabaseName, "MYSQL_USER=" + spec.Username, "MYSQL_PASSWORD=" + spec.Password}
		healthTest = []string{"CMD-SHELL", "mysqladmin ping -h 127.0.0.1 -uroot -p\"$MYSQL_ROOT_PASSWORD\" --silent"}
	case "mariadb":
		environment = []string{"MARIADB_ROOT_PASSWORD=" + spec.Password, "MARIADB_DATABASE=" + spec.DatabaseName, "MARIADB_USER=" + spec.Username, "MARIADB_PASSWORD=" + spec.Password}
		healthTest = []string{"CMD", "healthcheck.sh", "--connect", "--innodb_initialized"}
	case "postgres":
		environment = []string{"POSTGRES_DB=" + spec.DatabaseName, "POSTGRES_USER=" + spec.Username, "POSTGRES_PASSWORD=" + spec.Password}
		healthTest = []string{"CMD-SHELL", "pg_isready -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\""}
	}
	hostConfig := map[string]any{
		"NetworkMode":     "selfhost-proxy",
		"PublishAllPorts": false,
		"RestartPolicy":   map[string]string{"Name": "unless-stopped"},
		"SecurityOpt":     []string{"no-new-privileges:true"},
		"Binds":           []string{spec.VolumeName + ":" + preset.VolumeTarget},
	}
	if spec.PublicEnabled {
		hostConfig["PortBindings"] = map[string]any{portKey: []map[string]string{{"HostIp": "0.0.0.0", "HostPort": fmt.Sprint(spec.PublicPort)}}}
	}
	createBody := map[string]any{
		"Image":        spec.Image,
		"Env":          environment,
		"Cmd":          command,
		"ExposedPorts": map[string]any{portKey: map[string]any{}},
		"Healthcheck": map[string]any{
			"Test":        healthTest,
			"Interval":    int64(5 * time.Second),
			"Timeout":     int64(3 * time.Second),
			"Retries":     10,
			"StartPeriod": int64(15 * time.Second),
		},
		"Labels": map[string]string{
			"selfhost.managed":      "true",
			"selfhost.project.id":   spec.ProjectID,
			"selfhost.database.id":  spec.ID,
			"selfhost.service.kind": "database",
		},
		"HostConfig": hostConfig,
	}
	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), createBody, nil)
	if err != nil {
		return DatabaseRuntime{}, fmt.Errorf("create database container: %w", err)
	}
	created.Body.Close()
	if report != nil {
		report("create", "complete", "Database container created")
		report("start", "start", "Starting database container")
	}
	started, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
	if err != nil {
		return DatabaseRuntime{}, fmt.Errorf("start database container: %w", err)
	}
	started.Body.Close()
	if report != nil {
		report("start", "complete", "Database container started")
		report("verify", "start", "Reading container health")
	}
	runtimeState, err := d.DatabaseRuntime(ctx, spec.ID, spec.Port)
	if err != nil {
		return DatabaseRuntime{}, err
	}
	if report != nil {
		report("verify", "complete", "Database container health is "+runtimeState.Status)
	}
	return runtimeState, nil
}

func (d *Docker) DatabaseRuntime(ctx context.Context, serviceID string, internalPort int) (DatabaseRuntime, error) {
	name := databaseContainerName(serviceID)
	var inspected struct {
		Name  string `json:"Name"`
		State struct {
			Running bool `json:"Running"`
			Health  struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := d.get(ctx, "/containers/"+url.PathEscape(name)+"/json", &inspected); err != nil {
		return DatabaseRuntime{}, err
	}
	status := "degraded"
	if inspected.State.Running {
		status = "healthy"
		if inspected.State.Health.Status == "starting" {
			status = "deploying"
		} else if inspected.State.Health.Status == "unhealthy" {
			status = "degraded"
		}
	}
	hostPort := 0
	bindings := inspected.NetworkSettings.Ports[fmt.Sprintf("%d/tcp", internalPort)]
	if len(bindings) > 0 {
		fmt.Sscan(bindings[0].HostPort, &hostPort)
	}
	return DatabaseRuntime{Status: status, Container: strings.TrimPrefix(inspected.Name, "/"), HostPort: hostPort, Health: inspected.State.Health.Status}, nil
}

func (d *Docker) RemoveDatabase(ctx context.Context, serviceID, volumeName string, removeVolume bool) error {
	name := databaseContainerName(serviceID)
	if response, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil); err == nil {
		response.Body.Close()
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if removeVolume {
		if response, err := d.request(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(volumeName)+"?force=1", nil, nil); err == nil {
			response.Body.Close()
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

func (d *Docker) RemoveProject(ctx context.Context, projectID string) error {
	name := containerName(projectID)
	response, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1&v=1", nil, nil)
	if err == nil {
		response.Body.Close()
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (d *Docker) RemoveApplication(ctx context.Context, serviceID string) error {
	response, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(applicationContainerName(serviceID))+"?force=1&v=1", nil, nil)
	if err == nil {
		response.Body.Close()
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (d *Docker) ApplicationService(ctx context.Context, serviceID, serviceName string) (Service, error) {
	name := applicationContainerName(serviceID)
	var inspected struct {
		Name  string `json:"Name"`
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := d.get(ctx, "/containers/"+url.PathEscape(name)+"/json", &inspected); err != nil {
		return Service{}, err
	}
	status := "degraded"
	if inspected.State.Running {
		status = "healthy"
	}
	return Service{Name: serviceName, Image: inspected.Config.Image, Status: status, Container: strings.TrimPrefix(inspected.Name, "/")}, nil
}

func (d *Docker) ProjectService(ctx context.Context, projectID string) (Service, error) {
	name := containerName(projectID)
	var inspected struct {
		Name  string `json:"Name"`
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := d.get(ctx, "/containers/"+url.PathEscape(name)+"/json", &inspected); err != nil {
		return Service{}, err
	}
	status := "degraded"
	if inspected.State.Running {
		status = "healthy"
	}
	hostPort := ""
	for _, bindings := range inspected.NetworkSettings.Ports {
		if len(bindings) > 0 && bindings[0].HostPort != "" {
			hostPort = bindings[0].HostPort
			break
		}
	}
	return Service{Name: "application", Image: inspected.Config.Image, Status: status, Container: strings.TrimPrefix(inspected.Name, "/"), HostPort: hostPort}, nil
}

func (d *Docker) ProjectLogs(ctx context.Context, projectID string, tail int) ([]string, error) {
	return d.containerLogs(ctx, containerName(projectID), tail)
}

func (d *Docker) ApplicationLogs(ctx context.Context, serviceID string, tail int) ([]string, error) {
	return d.containerLogs(ctx, applicationContainerName(serviceID), tail)
}

func (d *Docker) DatabaseLogs(ctx context.Context, serviceID string, tail int) ([]string, error) {
	return d.containerLogs(ctx, databaseContainerName(serviceID), tail)
}

func (d *Docker) containerLogs(ctx context.Context, name string, tail int) ([]string, error) {
	if tail < 1 || tail > 1000 {
		tail = 200
	}
	path := "/containers/" + url.PathEscape(name) + "/logs?stdout=1&stderr=1&timestamps=1&tail=" + fmt.Sprint(tail)
	res, err := d.request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return decodeLogStream(raw), nil
}

func decodeLogStream(raw []byte) []string {
	var output strings.Builder
	remaining := raw
	decodedFrames := false
	for len(remaining) >= 8 && remaining[0] <= 2 {
		size := int(binary.BigEndian.Uint32(remaining[4:8]))
		if size < 0 || size > len(remaining)-8 {
			break
		}
		decodedFrames = true
		output.Write(remaining[8 : 8+size])
		remaining = remaining[8+size:]
	}
	text := string(raw)
	if decodedFrames {
		text = output.String()
	}
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}
