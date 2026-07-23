package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type PlatformRuntime struct {
	ContainerID string `json:"-"`
	Container   string `json:"container"`
	Image       string `json:"image"`
	Digest      string `json:"digest"`
}

type platformContainerInspect struct {
	ID     string         `json:"Id"`
	Name   string         `json:"Name"`
	Image  string         `json:"Image"`
	Config map[string]any `json:"Config"`
	State  struct {
		Running bool `json:"Running"`
	} `json:"State"`
	HostConfig      map[string]any `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct {
			Aliases []string `json:"Aliases"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

func (d *Docker) PlatformRuntime(ctx context.Context) (PlatformRuntime, error) {
	current, err := d.currentContainer(ctx)
	if err != nil {
		return PlatformRuntime{}, err
	}
	var image struct {
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := d.get(ctx, "/images/"+url.PathEscape(current.Image)+"/json", &image); err != nil {
		return PlatformRuntime{}, fmt.Errorf("inspect running platform image: %w", err)
	}
	configuredImage := stringValue(current.Config["Image"])
	repository := imageRepository(configuredImage)
	digest := ""
	for _, repoDigest := range image.RepoDigests {
		if repository != "" && !strings.HasPrefix(repoDigest, repository+"@") {
			continue
		}
		if separator := strings.LastIndex(repoDigest, "@"); separator >= 0 {
			digest = repoDigest[separator+1:]
			break
		}
	}
	if digest == "" && len(image.RepoDigests) > 0 {
		if separator := strings.LastIndex(image.RepoDigests[0], "@"); separator >= 0 {
			digest = image.RepoDigests[0][separator+1:]
		}
	}
	return PlatformRuntime{
		ContainerID: current.ID,
		Container:   strings.TrimPrefix(current.Name, "/"),
		Image:       configuredImage,
		Digest:      digest,
	}, nil
}

func imageRepository(image string) string {
	image = strings.TrimSpace(image)
	if separator := strings.LastIndex(image, "@"); separator >= 0 {
		return image[:separator]
	}
	if separator := strings.LastIndex(image, ":"); separator > strings.LastIndex(image, "/") {
		return image[:separator]
	}
	return image
}

func (d *Docker) currentContainer(ctx context.Context) (platformContainerInspect, error) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return platformContainerInspect{}, errors.New("cannot identify the running Dokyr container")
	}
	containers := []dockerContainerSummary{}
	if err := d.get(ctx, "/containers/json?all=1", &containers); err != nil {
		return platformContainerInspect{}, err
	}
	for _, container := range containers {
		if strings.HasPrefix(container.ID, hostname) {
			var inspected platformContainerInspect
			if err := d.get(ctx, "/containers/"+url.PathEscape(container.ID)+"/json", &inspected); err != nil {
				return platformContainerInspect{}, err
			}
			return inspected, nil
		}
	}
	return platformContainerInspect{}, errors.New("self-update is available only when Dokyr runs in Docker")
}

func (d *Docker) PullPlatformImage(ctx context.Context, image string, progress ProgressFunc) error {
	return d.pullImage(ctx, image, nil, progress)
}

func (d *Docker) StartPlatformUpdateHelper(ctx context.Context, jobID, targetImage string) error {
	current, err := d.currentContainer(ctx)
	if err != nil {
		return err
	}
	networkMode := stringValue(current.HostConfig["NetworkMode"])
	if networkMode == "" {
		return errors.New("running Dokyr container has no update network")
	}
	helperName := "dokyr-update-helper-" + jobID
	createBody := map[string]any{
		// Run the trusted helper from the current release. The target image is
		// data to install, not code allowed to control rollback.
		"Image": current.Image,
		"Cmd":   []string{"update-helper", "--container", current.ID, "--target-image", targetImage},
		"Labels": map[string]string{
			"dokyr.update.helper": "true",
			"dokyr.update.job":    jobID,
		},
		"HostConfig": map[string]any{
			"Binds":         []string{d.socket + ":/var/run/docker.sock"},
			"NetworkMode":   networkMode,
			"AutoRemove":    true,
			"RestartPolicy": map[string]string{"Name": "no"},
			"SecurityOpt":   []string{"no-new-privileges:true"},
			"CapDrop":       []string{"ALL"},
		},
	}
	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(helperName), createBody, nil)
	if err != nil {
		return fmt.Errorf("create update helper: %w", err)
	}
	created.Body.Close()
	started, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(helperName)+"/start", nil, nil)
	if err != nil {
		return fmt.Errorf("start update helper: %w", err)
	}
	started.Body.Close()
	return nil
}

// RunPlatformUpdateHelper replaces the container identified by currentID with
// the helper's own image. It runs outside the web process so it survives the
// old container being stopped and can roll it back.
func RunPlatformUpdateHelper(ctx context.Context, currentID, targetImage string) error {
	docker, err := NewDocker()
	if err != nil {
		return err
	}
	defer docker.Close()

	current := platformContainerInspect{}
	if err := docker.get(ctx, "/containers/"+url.PathEscape(currentID)+"/json", &current); err != nil {
		return fmt.Errorf("inspect current platform: %w", err)
	}
	if targetImage == "" {
		return errors.New("update helper has no target image")
	}
	stableName := strings.TrimPrefix(current.Name, "/")
	backupName := stableName + "-update-backup-" + fmt.Sprint(time.Now().Unix())
	networking := endpointConfiguration(current)

	if err := docker.renameContainer(ctx, current.ID, backupName); err != nil {
		return fmt.Errorf("reserve stable container name: %w", err)
	}
	if err := docker.stopContainer(ctx, current.ID); err != nil {
		_ = docker.renameContainer(context.Background(), current.ID, stableName)
		return fmt.Errorf("stop previous platform: %w", err)
	}
	for network := range current.NetworkSettings.Networks {
		if err := docker.disconnectNetwork(ctx, network, current.ID); err != nil {
			_ = docker.restorePlatform(context.Background(), current, stableName, backupName)
			return fmt.Errorf("disconnect previous platform from %s: %w", network, err)
		}
	}

	config := cloneMap(current.Config)
	config["Image"] = targetImage
	// Docker normally derives this from the new container ID. Reusing the old
	// generated hostname would prevent the replacement from identifying itself
	// on the next update.
	delete(config, "Hostname")
	config["HostConfig"] = cloneMap(current.HostConfig)
	config["NetworkingConfig"] = map[string]any{"EndpointsConfig": networking}
	created, err := docker.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(stableName), config, nil)
	if err != nil {
		_ = docker.restorePlatform(context.Background(), current, stableName, backupName)
		return fmt.Errorf("create replacement platform: %w", err)
	}
	created.Body.Close()
	started, err := docker.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(stableName)+"/start", nil, nil)
	if err != nil {
		_ = docker.removeContainer(context.Background(), stableName)
		_ = docker.restorePlatform(context.Background(), current, stableName, backupName)
		return fmt.Errorf("start replacement platform: %w", err)
	}
	started.Body.Close()

	if err := waitForPlatformHealth(ctx, stableName); err != nil {
		_ = docker.removeContainer(context.Background(), stableName)
		_ = docker.restorePlatform(context.Background(), current, stableName, backupName)
		return err
	}
	if err := docker.removeContainer(ctx, current.ID); err != nil {
		return fmt.Errorf("remove previous platform after successful update: %w", err)
	}
	return nil
}

func endpointConfiguration(container platformContainerInspect) map[string]any {
	endpoints := make(map[string]any, len(container.NetworkSettings.Networks))
	for network, settings := range container.NetworkSettings.Networks {
		aliases := make([]string, 0, len(settings.Aliases))
		for _, alias := range settings.Aliases {
			if alias != "" && !strings.HasPrefix(container.ID, alias) {
				aliases = append(aliases, alias)
			}
		}
		endpoints[network] = map[string]any{"Aliases": aliases}
	}
	return endpoints
}

func (d *Docker) restorePlatform(ctx context.Context, previous platformContainerInspect, stableName, _ string) error {
	if err := d.renameContainer(ctx, previous.ID, stableName); err != nil {
		return err
	}
	for network, endpoint := range endpointConfiguration(previous) {
		body := map[string]any{"Container": previous.ID, "EndpointConfig": endpoint}
		res, err := d.request(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/connect", body, nil)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "already connected") {
				continue
			}
			return err
		}
		res.Body.Close()
	}
	res, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(previous.ID)+"/start", nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

func (d *Docker) disconnectNetwork(ctx context.Context, network, container string) error {
	res, err := d.request(ctx, http.MethodPost, "/networks/"+url.PathEscape(network)+"/disconnect", map[string]any{
		"Container": container, "Force": true,
	}, nil)
	if err == nil {
		res.Body.Close()
	}
	return err
}

func (d *Docker) removeContainer(ctx context.Context, id string) error {
	res, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=1", nil, nil)
	if err == nil {
		res.Body.Close()
	}
	return err
}

func waitForPlatformHealth(ctx context.Context, container string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+container+":8080/api/health", nil)
		if res, err := client.Do(req); err == nil {
			var status struct {
				OK bool `json:"ok"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&status)
			res.Body.Close()
			if res.StatusCode == http.StatusOK && decodeErr == nil && status.OK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("replacement platform did not become healthy within 90 seconds")
}

func cloneMap(source map[string]any) map[string]any {
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
