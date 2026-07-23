package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/store"
	"gopkg.in/yaml.v3"
)

type composeIssue struct {
	Service string `json:"service,omitempty"`
	Message string `json:"message"`
}

type composePreviewService struct {
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	Image            string   `json:"image"`
	ContainerPort    int      `json:"containerPort"`
	PublicPort       int      `json:"publicPort,omitempty"`
	EnvironmentCount int      `json:"environmentCount"`
	Command          string   `json:"command,omitempty"`
	Engine           string   `json:"engine,omitempty"`
	DatabaseName     string   `json:"databaseName,omitempty"`
	Username         string   `json:"username,omitempty"`
	Warnings         []string `json:"warnings"`
}

type composePreview struct {
	Valid        bool                    `json:"valid"`
	Services     []composePreviewService `json:"services"`
	Errors       []composeIssue          `json:"errors"`
	Warnings     []composeIssue          `json:"warnings"`
	Applications int                     `json:"applications"`
	Databases    int                     `json:"databases"`
}

type composeApplicationPlan struct {
	Name          string
	Image         string
	Port          int
	Command       string
	Environment   []string
	HealthType    string
	HealthCommand string
}

type composeDatabasePlan struct {
	Name          string
	Engine        string
	Image         string
	Port          int
	PublicEnabled bool
	PublicPort    int
	DatabaseName  string
	Username      string
	Password      string
}

type composePlan struct {
	Preview      composePreview
	Applications []composeApplicationPlan
	Databases    []composeDatabasePlan
}

type composeRequest struct {
	Compose string `json:"compose"`
}

func (a *API) validateCompose(w http.ResponseWriter, r *http.Request) {
	var in composeRequest
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	plan := parseCompose(in.Compose)
	if err := a.addComposeProjectValidation(r.Context(), projectID, &plan); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, plan.Preview)
}

func (a *API) importCompose(w http.ResponseWriter, r *http.Request) {
	var in composeRequest
	if !decode(w, r, &in) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, err := a.store.Project(r.Context(), projectID); store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}

	a.applicationMu.Lock()
	a.databaseMu.Lock()
	plan := parseCompose(in.Compose)
	if err := a.addComposeProjectValidation(r.Context(), projectID, &plan); err != nil {
		a.databaseMu.Unlock()
		a.applicationMu.Unlock()
		problem(w, err)
		return
	}
	if !plan.Preview.Valid {
		a.databaseMu.Unlock()
		a.applicationMu.Unlock()
		write(w, http.StatusBadRequest, map[string]any{"error": "compose file has validation errors", "preview": plan.Preview})
		return
	}

	applications := make([]store.ApplicationService, 0, len(plan.Applications))
	databases := make([]store.DatabaseService, 0, len(plan.Databases))
	serviceHosts := make(map[string]string, len(plan.Applications)+len(plan.Databases))
	for _, item := range plan.Applications {
		id := newID("svc")
		serviceHosts[item.Name] = "selfhost-svc-" + id
		applications = append(applications, store.ApplicationService{
			ID: id, ProjectID: projectID, Name: item.Name, SourceType: "image", ImageURL: item.Image,
			BuildStrategy: "dockerfile", ContainerPort: item.Port, Command: item.Command,
			HealthCheckType: item.HealthType, HealthCheckCommand: item.HealthCommand, HealthCheckTimeout: 60,
			Status: "created",
		})
	}
	for _, item := range plan.Databases {
		id := newID("db")
		serviceHosts[item.Name] = "selfhost-db-" + id
		sealed, err := a.box.Encrypt(item.Password)
		if err != nil {
			a.databaseMu.Unlock()
			a.applicationMu.Unlock()
			problem(w, err)
			return
		}
		databases = append(databases, store.DatabaseService{
			ID: id, ProjectID: projectID, Name: item.Name, Engine: item.Engine, Image: item.Image,
			InternalPort: item.Port, PublicEnabled: item.PublicEnabled, PublicPort: item.PublicPort,
			VolumeName: "selfhost-data-" + id, Username: item.Username, DatabaseName: item.DatabaseName,
			PasswordEncrypted: sealed,
		})
	}
	for index, item := range plan.Applications {
		environment := rewriteComposeServiceHosts(item.Environment, serviceHosts)
		sealed, err := a.box.Encrypt(strings.Join(environment, "\n"))
		if err != nil {
			a.databaseMu.Unlock()
			a.applicationMu.Unlock()
			problem(w, err)
			return
		}
		applications[index].EnvironmentEncrypted = sealed
	}
	if err := a.store.CreateImportedServices(r.Context(), applications, databases); err != nil {
		a.databaseMu.Unlock()
		a.applicationMu.Unlock()
		if strings.Contains(err.Error(), "project_name_unique") {
			write(w, http.StatusConflict, map[string]string{"error": "one of these service names is already in use"})
			return
		}
		if strings.Contains(err.Error(), "database_services_public_port_unique") {
			write(w, http.StatusConflict, map[string]string{"error": "one of these database public ports is already assigned"})
			return
		}
		problem(w, err)
		return
	}
	a.databaseMu.Unlock()
	a.applicationMu.Unlock()

	rollback := func() {
		for _, database := range databases {
			_ = a.docker.RemoveDatabase(context.Background(), database.ID, database.VolumeName, true)
			_ = a.store.DeleteDatabaseService(context.Background(), database.ID)
		}
		for _, service := range applications {
			_ = a.store.DeleteApplicationService(context.Background(), service.ID)
		}
	}
	for index, database := range databases {
		password := plan.Databases[index].Password
		report := func(stage, eventType, message string) {
			a.recordDatabaseDeploymentEvent(r.Context(), database.ID, stage, eventType, message)
		}
		if _, err := a.docker.DeployDatabase(r.Context(), databaseSpec(database, password), report); err != nil {
			rollback()
			a.log.Error("import compose database", "database", database.ID, "error", err)
			write(w, http.StatusBadGateway, map[string]string{"error": "could not deploy database " + database.Name + ": " + err.Error()})
			return
		}
	}

	claims, _ := auth.FromContext(r.Context())
	deployments := []store.Deployment{}
	deploymentErrors := []composeIssue{}
	for _, service := range applications {
		_, deployment, err := a.startApplicationServiceDeployment(r.Context(), service.ID, claims.Subject, "Deploy "+service.Name+" from Compose import", "")
		if err != nil {
			deploymentErrors = append(deploymentErrors, composeIssue{Service: service.Name, Message: err.Error()})
			continue
		}
		deployments = append(deployments, deployment)
	}
	write(w, http.StatusCreated, map[string]any{
		"services": applications, "databases": databases, "deployments": deployments,
		"deploymentErrors": deploymentErrors,
	})
}

func (a *API) addComposeProjectValidation(ctx context.Context, projectID string, plan *composePlan) error {
	applications, err := a.store.ApplicationServices(ctx, projectID)
	if err != nil {
		return err
	}
	databases, err := a.store.ProjectDatabaseServices(ctx, projectID)
	if err != nil {
		return err
	}
	if len(applications)+len(plan.Applications) > 25 {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "a project can have at most 25 additional application services"})
	}
	existingNames := map[string]bool{}
	for _, service := range applications {
		existingNames[strings.ToLower(service.Name)] = true
	}
	for _, database := range databases {
		existingNames[strings.ToLower(database.Name)] = true
	}
	for _, service := range plan.Preview.Services {
		if existingNames[strings.ToLower(service.Name)] {
			plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Service: service.Name, Message: "a service with this name already exists in this project"})
		}
	}
	for _, database := range plan.Databases {
		if !database.PublicEnabled {
			continue
		}
		inUse, err := a.store.DatabasePublicPortInUse(ctx, database.PublicPort)
		if err != nil {
			return err
		}
		if inUse {
			plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Service: database.Name, Message: fmt.Sprintf("public port %d is already assigned to another database", database.PublicPort)})
		}
	}
	plan.Preview.Valid = len(plan.Preview.Errors) == 0
	return nil
}

func parseCompose(content string) composePlan {
	plan := composePlan{Preview: composePreview{Services: []composePreviewService{}, Errors: []composeIssue{}, Warnings: []composeIssue{}}}
	if strings.TrimSpace(content) == "" {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "paste or choose a Compose YAML file"})
		return plan
	}
	if len(content) > 1<<20 {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "compose file must be smaller than 1 MB"})
		return plan
	}
	var document struct {
		Services map[string]map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "invalid Compose YAML: " + err.Error()})
		return plan
	}
	if len(document.Services) == 0 {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "compose file must define at least one service"})
		return plan
	}
	if len(document.Services) > 25 {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Message: "compose file can contain at most 25 services"})
	}
	names := make([]string, 0, len(document.Services))
	for name := range document.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		parseComposeService(&plan, name, document.Services[name])
	}
	seenNames := map[string]string{}
	seenPublicPorts := map[int]string{}
	for _, service := range plan.Preview.Services {
		key := strings.ToLower(service.Name)
		if previous := seenNames[key]; previous != "" {
			plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Service: service.Name, Message: "service name conflicts with " + previous + " when compared case-insensitively"})
		} else {
			seenNames[key] = service.Name
		}
		if service.Kind == "database" && service.PublicPort > 0 {
			if previous := seenPublicPorts[service.PublicPort]; previous != "" {
				plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Service: service.Name, Message: fmt.Sprintf("public port %d is also assigned to %s", service.PublicPort, previous)})
			} else {
				seenPublicPorts[service.PublicPort] = service.Name
			}
		}
	}
	plan.Preview.Applications = len(plan.Applications)
	plan.Preview.Databases = len(plan.Databases)
	plan.Preview.Valid = len(plan.Preview.Errors) == 0
	return plan
}

func parseComposeService(plan *composePlan, name string, raw map[string]any) {
	preview := composePreviewService{Name: name, Kind: "application", Warnings: []string{}}
	addError := func(message string) {
		plan.Preview.Errors = append(plan.Preview.Errors, composeIssue{Service: name, Message: message})
	}
	addWarning := func(message string) {
		preview.Warnings = append(preview.Warnings, message)
		plan.Preview.Warnings = append(plan.Preview.Warnings, composeIssue{Service: name, Message: message})
	}
	if !validComposeServiceName(name) {
		addError("service name must start with a letter or number and use at most 63 letters, numbers, dots, underscores, or hyphens")
	}
	image, ok := composeString(raw["image"])
	image = strings.TrimSpace(image)
	if !ok || image == "" {
		addError("prebuilt image is required; build-only Compose services are not supported")
	} else if literal, err := composeLiteral(image); err != nil {
		addError("image interpolation is not supported; replace variables with the final image reference")
	} else {
		image = literal
	}
	if _, exists := raw["build"]; exists {
		if image == "" {
			addError("repository build contexts cannot be uploaded with a Compose file")
		} else {
			addWarning("build configuration is ignored because the prebuilt image is used")
		}
	}
	preview.Image = image
	environment, envErr := composeEnvironment(raw["environment"])
	if envErr != nil {
		addError(envErr.Error())
	}
	preview.EnvironmentCount = len(environment)
	if _, exists := raw["env_file"]; exists {
		addError("env_file is not available during import; inline those values under environment")
	}
	if _, exists := raw["secrets"]; exists {
		addError("Compose secrets are not supported; use Dokyr environment secrets after import")
	}
	if _, exists := raw["configs"]; exists {
		addError("Compose configs are not supported")
	}

	targetPort, publicPort, portCount, portErr := composePorts(raw["ports"], raw["expose"])
	if portErr != nil {
		addError(portErr.Error())
	}
	if portCount > 1 {
		addWarning("only the first container port is used; add other routes from the Domains tab")
	}
	command, commandErr := composeCommand(raw["command"])
	if commandErr != nil {
		addError(commandErr.Error())
	}
	preview.Command = command
	healthType, healthCommand, healthErr := composeHealthcheck(raw["healthcheck"])
	if healthErr != nil {
		addError(healthErr.Error())
	}

	engine := composeDatabaseEngine(image)
	if engine != "" {
		preset, _ := runtime.DatabaseEngine(engine)
		if targetPort != 0 && targetPort != preset.Port {
			addWarning(fmt.Sprintf("managed %s uses its standard container port %d instead of %d", engine, preset.Port, targetPort))
		}
		targetPort = preset.Port
		preview.Kind, preview.Engine, preview.ContainerPort = "database", engine, targetPort
		preview.PublicPort = publicPort
		values := environmentMap(environment)
		databaseName, username, password, extra := composeDatabaseCredentials(engine, values)
		if !databaseIdentifier(databaseName) || !databaseIdentifier(username) {
			addError("database and user names may contain only letters, numbers, and underscores")
		}
		if (engine == "mysql" || engine == "mariadb") && strings.EqualFold(username, "root") {
			addError("managed MySQL and MariaDB services require an application user other than root")
		}
		if password == "" {
			password = randomSecret()
			addWarning("no database password was provided; Dokyr will generate one")
		} else if len(password) < 12 {
			addError("database password must contain at least 12 characters")
		}
		if command != "" {
			addWarning("custom database command is ignored by the managed database runtime")
		}
		if healthType != "none" {
			addWarning("custom healthcheck is replaced by Dokyr's database healthcheck")
		}
		if _, exists := raw["volumes"]; exists {
			addWarning("Compose volume mounts are replaced by a Dokyr-managed persistent volume")
		}
		if extra > 0 {
			addWarning(fmt.Sprintf("%d database environment variable(s) are not part of the managed database preset and will be ignored", extra))
		}
		preview.DatabaseName, preview.Username = databaseName, username
		plan.Databases = append(plan.Databases, composeDatabasePlan{
			Name: name, Engine: engine, Image: image, Port: targetPort, PublicEnabled: publicPort > 0,
			PublicPort: publicPort, DatabaseName: databaseName, Username: username, Password: password,
		})
	} else {
		if targetPort == 0 {
			targetPort = 80
			addWarning("no container port was declared; port 80 will be used")
		}
		preview.ContainerPort = targetPort
		if publicPort > 0 {
			addWarning("published host port is ignored; expose this service with a Dokyr domain route")
		}
		if _, exists := raw["volumes"]; exists {
			addError("application volume mounts are not supported by Dokyr")
		}
		plan.Applications = append(plan.Applications, composeApplicationPlan{
			Name: name, Image: image, Port: targetPort, Command: command, Environment: environment,
			HealthType: healthType, HealthCommand: healthCommand,
		})
	}
	for _, field := range []string{"privileged", "network_mode", "devices", "cap_add", "cap_drop", "userns_mode"} {
		if _, exists := raw[field]; exists {
			addWarning(field + " is ignored; Dokyr applies its managed container security policy")
		}
	}
	for _, field := range []string{"depends_on", "networks", "container_name", "restart", "hostname"} {
		if _, exists := raw[field]; exists {
			addWarning(field + " is handled by Dokyr and is not copied")
		}
	}
	plan.Preview.Services = append(plan.Preview.Services, preview)
}

func validComposeServiceName(value string) bool {
	if len(value) < 1 || len(value) > 63 {
		return false
	}
	for index, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		if index > 0 && (character == '.' || character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}

func composeString(value any) (string, bool) {
	switch current := value.(type) {
	case string:
		return current, true
	case int:
		return strconv.Itoa(current), true
	case int64:
		return strconv.FormatInt(current, 10), true
	case uint64:
		return strconv.FormatUint(current, 10), true
	case float64:
		return strconv.FormatFloat(current, 'g', -1, 64), true
	case bool:
		return strconv.FormatBool(current), true
	default:
		return "", false
	}
}

func composeEnvironment(value any) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	values := []string{}
	switch current := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !environmentKey(key) {
				return nil, fmt.Errorf("environment variable %q is invalid", key)
			}
			if current[key] == nil {
				return nil, fmt.Errorf("environment variable %s needs an explicit value", key)
			}
			text, ok := composeString(current[key])
			if !ok {
				return nil, fmt.Errorf("environment variable %s must be a scalar value", key)
			}
			expanded, err := composeLiteral(text)
			if err != nil {
				return nil, fmt.Errorf("environment variable %s: %w", key, err)
			}
			values = append(values, key+"="+expanded)
		}
	case []any:
		seen := map[string]bool{}
		for _, item := range current {
			text, ok := composeString(item)
			if !ok {
				return nil, errors.New("environment list entries must use KEY=value")
			}
			key, rawValue, found := strings.Cut(text, "=")
			if !found || !environmentKey(key) {
				return nil, errors.New("environment list entries must use KEY=value")
			}
			if seen[key] {
				return nil, fmt.Errorf("environment variable %s is duplicated", key)
			}
			seen[key] = true
			expanded, err := composeLiteral(rawValue)
			if err != nil {
				return nil, fmt.Errorf("environment variable %s: %w", key, err)
			}
			values = append(values, key+"="+expanded)
		}
	default:
		return nil, errors.New("environment must be a map or a KEY=value list")
	}
	return values, nil
}

func composeLiteral(value string) (string, error) {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '$' {
			result.WriteByte(value[index])
			continue
		}
		if index+1 < len(value) && value[index+1] == '$' {
			result.WriteByte('$')
			index++
			continue
		}
		return "", errors.New("variable interpolation is not supported; provide the final value or escape a literal $ as $$")
	}
	return result.String(), nil
}

func composePorts(portsValue, exposeValue any) (int, int, int, error) {
	targets := []int{}
	public := 0
	if portsValue != nil {
		ports, ok := portsValue.([]any)
		if !ok {
			return 0, 0, 0, errors.New("ports must be a list")
		}
		for _, item := range ports {
			target, published, err := composePort(item)
			if err != nil {
				return 0, 0, 0, err
			}
			targets = append(targets, target)
			if public == 0 {
				public = published
			}
		}
	}
	if exposeValue != nil {
		expose, ok := exposeValue.([]any)
		if !ok {
			return 0, 0, 0, errors.New("expose must be a list")
		}
		for _, item := range expose {
			target, _, err := composePort(item)
			if err != nil {
				return 0, 0, 0, err
			}
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return 0, public, 0, nil
	}
	return targets[0], public, len(targets), nil
}

func composePort(value any) (int, int, error) {
	if mapping, ok := value.(map[string]any); ok {
		if protocol, _ := composeString(mapping["protocol"]); protocol != "" && !strings.EqualFold(protocol, "tcp") {
			return 0, 0, errors.New("only TCP ports are supported")
		}
		if hostIP, _ := composeString(mapping["host_ip"]); hostIP != "" && !composePublicHostIP(hostIP) {
			return 0, 0, errors.New("host IP-specific port bindings are not supported")
		}
		targetText, targetOK := composeString(mapping["target"])
		if !targetOK {
			return 0, 0, errors.New("long port syntax requires a target port")
		}
		target, err := parseComposePortNumber(targetText)
		if err != nil {
			return 0, 0, err
		}
		published := 0
		if mapping["published"] != nil {
			publishedText, ok := composeString(mapping["published"])
			if !ok {
				return 0, 0, errors.New("published port must be a number")
			}
			published, err = parseComposePortNumber(publishedText)
			if err != nil {
				return 0, 0, err
			}
		}
		return target, published, nil
	}
	text, ok := composeString(value)
	if !ok {
		return 0, 0, errors.New("port entries must use short or long Compose syntax")
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(text)), "/udp") {
		return 0, 0, errors.New("only TCP ports are supported")
	}
	text = strings.TrimSpace(strings.TrimSuffix(text, "/tcp"))
	if strings.Contains(text, "-") {
		return 0, 0, errors.New("port ranges are not supported")
	}
	parts := strings.Split(text, ":")
	if len(parts) >= 3 {
		hostIP := strings.Join(parts[:len(parts)-2], ":")
		if !composePublicHostIP(hostIP) {
			return 0, 0, errors.New("host IP-specific port bindings are not supported")
		}
	}
	target, err := parseComposePortNumber(parts[len(parts)-1])
	if err != nil {
		return 0, 0, err
	}
	published := 0
	if len(parts) >= 2 {
		published, err = parseComposePortNumber(parts[len(parts)-2])
		if err != nil {
			return 0, 0, err
		}
	}
	return target, published, nil
}

func composePublicHostIP(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	return value == "" || value == "0.0.0.0" || value == "::"
}

func parseComposePortNumber(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("ports must be numbers between 1 and 65535")
	}
	return port, nil
}

func composeCommand(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		literal, err := composeLiteral(strings.TrimSpace(text))
		if err != nil {
			return "", fmt.Errorf("command: %w", err)
		}
		return literal, nil
	}
	items, ok := value.([]any)
	if !ok {
		return "", errors.New("command must be a string or a list")
	}
	arguments := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := composeString(item)
		if !ok {
			return "", errors.New("command list items must be scalar values")
		}
		text, err := composeLiteral(text)
		if err != nil {
			return "", fmt.Errorf("command: %w", err)
		}
		arguments = append(arguments, shellQuote(text))
	}
	return strings.Join(arguments, " "), nil
}

func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n'\"\\") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func composeHealthcheck(value any) (string, string, error) {
	if value == nil {
		return "none", "", nil
	}
	health, ok := value.(map[string]any)
	if !ok {
		return "", "", errors.New("healthcheck must be a mapping")
	}
	if disabled, ok := health["disable"].(bool); ok && disabled {
		return "none", "", nil
	}
	test := health["test"]
	if test == nil {
		return "none", "", nil
	}
	if text, ok := test.(string); ok {
		if strings.EqualFold(strings.TrimSpace(text), "NONE") {
			return "none", "", nil
		}
		return "command", strings.TrimSpace(text), nil
	}
	items, ok := test.([]any)
	if !ok || len(items) == 0 {
		return "", "", errors.New("healthcheck test must be a string or command list")
	}
	mode, ok := composeString(items[0])
	if !ok {
		return "", "", errors.New("healthcheck command is invalid")
	}
	if strings.EqualFold(mode, "NONE") {
		return "none", "", nil
	}
	if !strings.EqualFold(mode, "CMD") && !strings.EqualFold(mode, "CMD-SHELL") {
		return "", "", errors.New("healthcheck test must start with CMD, CMD-SHELL, or NONE")
	}
	if strings.EqualFold(mode, "CMD-SHELL") && len(items) == 2 {
		command, ok := composeString(items[1])
		if !ok || strings.TrimSpace(command) == "" {
			return "", "", errors.New("healthcheck command cannot be empty")
		}
		command, err := composeLiteral(strings.TrimSpace(command))
		if err != nil {
			return "", "", fmt.Errorf("healthcheck command: %w", err)
		}
		return "command", command, nil
	}
	command, err := composeCommand(items[1:])
	if err != nil {
		return "", "", err
	}
	if command == "" {
		return "", "", errors.New("healthcheck command cannot be empty")
	}
	return "command", command, nil
}

func composeDatabaseEngine(image string) string {
	reference := strings.ToLower(strings.TrimSpace(image))
	if separator := strings.Index(reference, "@"); separator >= 0 {
		reference = reference[:separator]
	}
	parts := strings.Split(reference, "/")
	last := parts[len(parts)-1]
	if separator := strings.Index(last, ":"); separator >= 0 {
		last = last[:separator]
	}
	official := len(parts) == 1 ||
		len(parts) == 2 && (parts[0] == "library" || parts[0] == "docker.io") ||
		len(parts) == 3 && (parts[0] == "docker.io" || parts[0] == "registry-1.docker.io") && parts[1] == "library"
	if !official {
		return ""
	}
	switch last {
	case "mysql", "mariadb", "postgres":
		return last
	default:
		return ""
	}
}

func environmentMap(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, line := range environment {
		key, value, _ := strings.Cut(line, "=")
		values[key] = value
	}
	return values
}

func composeDatabaseCredentials(engine string, values map[string]string) (string, string, string, int) {
	databaseName, username, password := "app", "app", ""
	known := map[string]bool{}
	switch engine {
	case "postgres":
		known = map[string]bool{"POSTGRES_DB": true, "POSTGRES_USER": true, "POSTGRES_PASSWORD": true}
		if values["POSTGRES_DB"] != "" {
			databaseName = values["POSTGRES_DB"]
		}
		if values["POSTGRES_USER"] != "" {
			username = values["POSTGRES_USER"]
		}
		password = values["POSTGRES_PASSWORD"]
	case "mysql":
		known = map[string]bool{"MYSQL_DATABASE": true, "MYSQL_USER": true, "MYSQL_PASSWORD": true, "MYSQL_ROOT_PASSWORD": true}
		if values["MYSQL_DATABASE"] != "" {
			databaseName = values["MYSQL_DATABASE"]
		}
		if values["MYSQL_USER"] != "" {
			username = values["MYSQL_USER"]
		}
		password = values["MYSQL_PASSWORD"]
		if password == "" {
			password = values["MYSQL_ROOT_PASSWORD"]
		}
	case "mariadb":
		known = map[string]bool{"MARIADB_DATABASE": true, "MARIADB_USER": true, "MARIADB_PASSWORD": true, "MARIADB_ROOT_PASSWORD": true}
		if values["MARIADB_DATABASE"] != "" {
			databaseName = values["MARIADB_DATABASE"]
		}
		if values["MARIADB_USER"] != "" {
			username = values["MARIADB_USER"]
		}
		password = values["MARIADB_PASSWORD"]
		if password == "" {
			password = values["MARIADB_ROOT_PASSWORD"]
		}
	}
	extra := 0
	for key := range values {
		if !known[key] {
			extra++
		}
	}
	return databaseName, username, password, extra
}

func rewriteComposeServiceHosts(environment []string, hosts map[string]string) []string {
	rewritten := make([]string, 0, len(environment))
	for _, line := range environment {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		for name, host := range hosts {
			value = replaceComposeHost(value, name, host)
		}
		rewritten = append(rewritten, key+"="+value)
	}
	return rewritten
}

func replaceComposeHost(value, name, host string) string {
	if name == "" || !strings.Contains(value, name) {
		return value
	}
	var result strings.Builder
	for offset := 0; offset < len(value); {
		index := strings.Index(value[offset:], name)
		if index < 0 {
			result.WriteString(value[offset:])
			break
		}
		index += offset
		end := index + len(name)
		leftOK := index == 0 || !composeHostnameCharacter(value[index-1])
		rightOK := end == len(value) || !composeHostnameCharacter(value[end])
		result.WriteString(value[offset:index])
		if leftOK && rightOK {
			result.WriteString(host)
		} else {
			result.WriteString(name)
		}
		offset = end
	}
	return result.String()
}

func composeHostnameCharacter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '.' || value == '_' || value == '-'
}
