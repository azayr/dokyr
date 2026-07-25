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

// ControlPlaneContainerName returns the container name for one service of the
// Compose project Dokyr itself runs in. It resolves the project from the
// running control-plane container so it works regardless of the Compose
// project name the operator chose.
func (d *Docker) ControlPlaneContainerName(ctx context.Context, service string) (string, error) {
	service = strings.ToLower(strings.TrimSpace(service))
	if service == "" {
		return "", errors.New("control-plane service is required")
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "", errors.New("control-plane project is not discoverable")
	}
	containers := []dockerContainerSummary{}
	if err := d.get(ctx, "/containers/json", &containers); err != nil {
		return "", err
	}
	project := ""
	for _, container := range containers {
		if strings.HasPrefix(container.ID, hostname) {
			project = container.Labels["com.docker.compose.project"]
			break
		}
	}
	if project == "" {
		return "", errors.New("control-plane project is not discoverable")
	}
	for _, container := range containers {
		if container.Labels["com.docker.compose.project"] == project && container.Labels["com.docker.compose.service"] == service {
			return strings.TrimPrefix(firstString(container.Names), "/"), nil
		}
	}
	return "", ErrNotFound
}

// ExecInContainer runs a command inside an arbitrary container and captures the
// demultiplexed stdout and stderr streams.
func (d *Docker) ExecInContainer(ctx context.Context, container string, cmd []string) (CommandResult, error) {
	if len(cmd) == 0 {
		return CommandResult{}, errors.New("command is required")
	}
	startedAt := time.Now()
	created, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(container)+"/exec", map[string]any{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
	}, nil)
	if err != nil {
		return CommandResult{}, err
	}
	var execution struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(created.Body).Decode(&execution); err != nil {
		created.Body.Close()
		return CommandResult{}, err
	}
	created.Body.Close()
	if execution.ID == "" {
		return CommandResult{}, errors.New("docker did not return an exec ID")
	}

	started, err := d.request(ctx, http.MethodPost, "/exec/"+url.PathEscape(execution.ID)+"/start", map[string]any{"Detach": false, "Tty": false}, nil)
	if err != nil {
		return CommandResult{}, err
	}
	const outputLimit = 2 << 20
	output := &cappedOutput{limit: outputLimit}
	_, copyErr := io.Copy(output, started.Body)
	closeErr := started.Body.Close()
	if copyErr != nil {
		return CommandResult{}, copyErr
	}
	if closeErr != nil {
		return CommandResult{}, closeErr
	}

	var inspected struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	}
	if err := d.get(ctx, "/exec/"+url.PathEscape(execution.ID)+"/json", &inspected); err != nil {
		return CommandResult{}, err
	}
	if inspected.Running {
		return CommandResult{}, errors.New("container command is still running")
	}
	stdout, stderr := decodeExecStream(output.Bytes())
	return CommandResult{
		Container:  container,
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   inspected.ExitCode,
		DurationMS: time.Since(startedAt).Milliseconds(),
		Truncated:  output.truncated,
	}, nil
}

// RecreateControlPlaneService replaces a control-plane Compose service container
// with a new container running the same image, volumes, networks, and port
// bindings but a replacement environment. It is used to reconfigure services
// such as the bundled registry whose configuration is expressed as environment
// variables.
func (d *Docker) RecreateControlPlaneService(ctx context.Context, service string, env []string) error {
	container, err := d.ControlPlaneContainerName(ctx, service)
	if err != nil {
		return err
	}
	res, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(container)+"/json", nil, nil)
	if err != nil {
		return err
	}
	var inspected struct {
		Name   string `json:"Name"`
		Config struct {
			Image        string              `json:"Image"`
			Env          []string            `json:"Env"`
			Entrypoint   []string            `json:"Entrypoint"`
			Cmd          []string            `json:"Cmd"`
			ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		} `json:"Config"`
		HostConfig struct {
			Binds         []string `json:"Binds"`
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
		NetworkSettings struct {
			Networks map[string]struct {
				NetworkID string   `json:"NetworkID"`
				Aliases   []string `json:"Aliases"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&inspected); err != nil {
		res.Body.Close()
		return err
	}
	res.Body.Close()
	if inspected.Config.Image == "" {
		return errors.New("control-plane container image is not discoverable")
	}

	name := strings.TrimPrefix(inspected.Name, "/")
	if name == "" {
		name = container
	}
	tempName := name + "-reconfigure"

	if _, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/stop", nil, nil); err != nil {
		return fmt.Errorf("stop control-plane service: %w", err)
	}
	if _, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/rename?name="+url.QueryEscape(tempName), nil, nil); err != nil {
		_, _ = d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
		return fmt.Errorf("rename control-plane service: %w", err)
	}

	createBody := map[string]any{
		"Image": inspected.Config.Image,
		"Env":   mergeEnv(inspected.Config.Env, env),
		"HostConfig": map[string]any{
			"Binds":        inspected.HostConfig.Binds,
			"PortBindings": inspected.HostConfig.PortBindings,
			"RestartPolicy": map[string]any{
				"Name": inspected.HostConfig.RestartPolicy.Name,
			},
		},
	}
	if len(inspected.Config.ExposedPorts) > 0 {
		createBody["ExposedPorts"] = inspected.Config.ExposedPorts
	}
	if len(inspected.Config.Entrypoint) > 0 {
		createBody["Entrypoint"] = inspected.Config.Entrypoint
	}
	if len(inspected.Config.Cmd) > 0 {
		createBody["Cmd"] = inspected.Config.Cmd
	}
	if len(inspected.NetworkSettings.Networks) > 0 {
		endpoints := map[string]any{}
		for networkName, network := range inspected.NetworkSettings.Networks {
			aliases := []string{}
			for _, alias := range network.Aliases {
				if alias != name && alias != tempName {
					aliases = append(aliases, alias)
				}
			}
			aliases = append(aliases, name)
			endpoints[networkName] = map[string]any{"Aliases": aliases}
		}
		createBody["NetworkingConfig"] = map[string]any{"EndpointsConfig": endpoints}
	}

	created, err := d.request(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), createBody, nil)
	if err != nil {
		_, _ = d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(tempName)+"/rename?name="+url.QueryEscape(name), nil, nil)
		_, _ = d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
		return fmt.Errorf("create replacement control-plane service: %w", err)
	}
	io.Copy(io.Discard, created.Body)
	created.Body.Close()

	if _, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil); err != nil {
		_, _ = d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(name)+"?force=1", nil, nil)
		_, _ = d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(tempName)+"/rename?name="+url.QueryEscape(name), nil, nil)
		_, _ = d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(name)+"/start", nil, nil)
		return fmt.Errorf("start replacement control-plane service: %w", err)
	}
	if _, err := d.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(tempName)+"?force=1", nil, nil); err != nil {
		return fmt.Errorf("remove replaced control-plane service: %w", err)
	}
	return nil
}

func mergeEnv(existing, overrides []string) []string {
	values := map[string]string{}
	order := []string{}
	for _, entry := range existing {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = entry
	}
	for _, entry := range overrides {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = entry
	}
	merged := make([]string, 0, len(order))
	for _, key := range order {
		merged = append(merged, values[key])
	}
	return merged
}
