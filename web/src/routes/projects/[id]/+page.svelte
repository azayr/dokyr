<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api, currentUser } from '$lib/auth.js';

  const tabs = ['overview', 'metrics', 'deployments', 'logs', 'environment', 'databases', 'domains', 'settings'];
  let activeTab = 'overview';
  let data = { project: { name: 'Loading…', status: 'deploying' }, deployments: [], services: [], applicationServices: [], databaseServices: [] };
  let loading = true;
  let deploying = false;
  let lifecycleBusy = '';
  let error = '';
  let notice = '';
  let platformProtocol = 'http:';
  let platformPort = '';
  let platformHost = 'localhost';
  let domainBindings = [];
  let domainSaving = false;
  let domainError = '';
  let domainNotice = '';
  let domainModal = false;
  let domainDraft = null;
  let domainEditingIndex = -1;
  let serviceModal = false;
  let serviceSaving = false;
  let serviceError = '';
  let serviceForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', healthCheckType: 'none', healthCheckPath: '/', healthCheckCommand: '', healthCheckTimeoutSeconds: 60, environment: '' };
  let composeModal = false;
  let composeText = '';
  let composeFileName = '';
  let composeValidation = null;
  let composeValidating = false;
  let composeImporting = false;
  let composeError = '';
  let serviceRepositories = [];
  let serviceRepositoriesLoading = false;
  let serviceRepositoriesError = '';
  let serviceRepositoryQuery = '';
  let serviceRepositoryPickerOpen = false;
  let serviceSettingsService = null;
  let serviceSettingsForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', healthCheckType: 'none', healthCheckPath: '/', healthCheckCommand: '', healthCheckTimeoutSeconds: 60 };
  let serviceSettingsSaving = false;
  let serviceSettingsError = '';
  let runtimeSettingsServiceId = '';
  let runtimeSettingsForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', healthCheckType: 'none', healthCheckPath: '/', healthCheckCommand: '', healthCheckTimeoutSeconds: 60 };
  let runtimeSettingsBusy = '';
  let runtimeSettingsError = '';
  let runtimeSettingsNotice = '';
  let runtimeTriggers = { autoDeploy: false, registryWebhookEnabled: false, registryWebhookTag: '', webhookUrl: '', webhookConfigured: false };
  let runtimeTriggersLoading = false;
  let runtimeTriggersSaving = false;
  let runtimeTriggersError = '';
  let runtimeTriggersNotice = '';
  let applicationDeleteService = null;
  let applicationDeleteBusy = false;
  let applicationDeleteError = '';
  let logTargetId = '';
  let logView = 'runtime';
  let logEvents = [];
  let logRequest = 0;
  let logs = [];
  let logsLoading = false;
  let logsError = '';
  let logsUpdated = '';
  let logLevel = 'all';
  let logQuery = '';
  let logLimit = 300;
  let logsLive = true;
  let logPollTimer;
  let logsCopyTimer;
  let logsCopied = false;
  let logConsole;
  let terminalService = null;
  let terminalCommand = '';
  let terminalWorkingDir = '';
  let terminalEntries = [];
  let terminalHistory = [];
  let terminalHistoryIndex = -1;
  let terminalRunning = false;
  let terminalReturnFocus;
  let terminalInput;
  let terminalOutput;
  const databasePresets = {
    mysql: { label: 'MySQL', version: '8.4', port: 3306 },
    postgres: { label: 'PostgreSQL', version: '17', port: 5432 },
    mariadb: { label: 'MariaDB', version: '11.8', port: 3306 }
  };
  let databaseModal = false;
  let databaseSaving = false;
  let databaseError = '';
  let databaseForm = { engine: 'mysql', name: 'MySQL', databaseName: 'app', username: 'app', password: '', publicEnabled: false, publicPort: 3306 };
  let credentialsModal = false;
  let credentialsLoading = false;
  let credentials = null;
  let credentialsService = null;
  let copiedField = '';
  let exposureService = null;
  let exposurePort = 3306;
  let exposureSaving = false;
  let databaseDeleteService = null;
  let databaseDeleteConfirmation = '';
  let databaseDeleteVolume = false;
  let databaseDeleteBusy = false;
  let databaseDeleteError = '';
  let integrations = { connections: [], registries: [] };
  let settingsForm = { name: '', sourceType: 'image', repository: '', branch: 'main', connectionId: '', imageUrl: '', registryId: '', containerPort: 80 };
  let settingsSaving = false;
  let settingsError = '';
  let settingsNotice = '';
  let deleteModal = false;
  let deleteConfirmation = '';
  let deleteBusy = false;
  let deleteError = '';
  let environmentVariables = [];
  let environmentLoading = false;
  let environmentSaving = false;
  let environmentError = '';
  let environmentNotice = '';
  let environmentTargetId = 'main';
  let environmentLoadRequest = 0;
  let deploymentFilter = 'all';
  let bulkEnvironmentModal = false;
  let bulkEnvironmentText = '';
  let bulkEnvironmentError = '';
  let projectMetrics = { global: { diskIo: {}, networkIo: {}, disk: {} }, containers: [] };
  let metricsLoading = false;
  let metricsRefreshing = false;
  let metricsError = '';
  let metricsTimer;
  let projectPollTimer;

  $: project = data.project;
  $: service = data.services[0];
  $: databaseServices = data.databaseServices || [];
  $: applicationServices = data.applicationServices || [];
  $: legacyService = project.sourceType === 'empty' ? null : { id: 'main', name: project.name, imageUrl: project.sourceType === 'image' ? project.imageUrl : project.repository, containerPort: project.containerPort || 80, status: service?.status || project.status, container: service?.container || '', legacy: true };
  $: displayApplicationServices = [...(legacyService ? [legacyService] : []), ...applicationServices];
  $: logTargets = [
    ...displayApplicationServices.map((item) => ({ key: `application:${item.id}`, kind: 'application', ...item })),
    ...databaseServices.map((item) => ({ key: `database:${item.id}`, kind: 'database', ...item }))
  ];
  $: activeLogTarget = logTargets.find((item) => item.key === logTargetId) || logTargets[0] || null;
  $: if (activeLogTarget?.kind !== 'database' && logView !== 'runtime') logView = 'runtime';
  $: routeTargets = displayApplicationServices.map((item) => ({ ...item, id: item.legacy ? '' : item.id }));
  $: environmentTargets = displayApplicationServices;
  $: runtimeSettingsService = applicationServices.find((item) => item.id === runtimeSettingsServiceId) || applicationServices[0] || null;
  $: activeEnvironmentTarget = environmentTargets.find((item) => item.id === environmentTargetId) || environmentTargets[0];
  $: filteredDeployments = deploymentFilter === 'all' ? data.deployments : data.deployments.filter((item) => (item.serviceId || 'main') === deploymentFilter);
  $: source = project.sourceType === 'empty' ? '' : project.sourceType === 'image' ? project.imageUrl : project.repository;
  $: filteredServiceRepositories = serviceRepositories.filter((item) => item.fullName.toLowerCase().includes(serviceRepositoryQuery.trim().toLowerCase()));
  $: domainURL = project.domain ? `${project.httpsEnabled ? 'https:' : platformProtocol}//${project.domain}${!project.httpsEnabled && platformPort ? ':' + platformPort : ''}` : '';
  $: parsedLogs = logView === 'deployment'
    ? logEvents.map((event, index) => ({
        index: index + 1,
        timestamp: event.createdAt || '',
        time: event.createdAt ? new Date(event.createdAt).toLocaleTimeString() : '—',
        message: `[${event.stage || 'deploy'}] ${event.message}`,
        severity: event.type === 'error' ? 'error' : event.type === 'complete' ? 'info' : 'debug'
      }))
    : logs.map(parseLogLine);
  $: logCounts = parsedLogs.reduce((counts, entry) => ({ ...counts, [entry.severity]: (counts[entry.severity] || 0) + 1 }), { debug: 0, info: 0, warning: 0, error: 0 });
  $: visibleLogs = parsedLogs.filter((entry) => (logLevel === 'all' || entry.severity === logLevel) && entry.message.toLowerCase().includes(logQuery.trim().toLowerCase()));

  onMount(async () => {
    platformProtocol = location.protocol || 'http:';
    platformPort = location.port;
    platformHost = location.hostname || 'localhost';
    const requestedTab = location.hash.slice(1);
    if (tabs.includes(requestedTab)) activeTab = requestedTab;
    await Promise.all([loadProject(), loadIntegrations()]);
    await tick();
    if (activeTab === 'logs') {
      await loadLogs();
      startLogPolling();
    } else if (activeTab === 'environment') {
      await loadEnvironment();
    } else if (activeTab === 'metrics') {
      await loadProjectMetrics();
      startMetricsPolling();
    }
  });

  onDestroy(() => {
    stopLogPolling();
    stopMetricsPolling();
    clearTimeout(projectPollTimer);
    clearTimeout(logsCopyTimer);
  });

  async function loadProject(silent = false) {
    if (!silent) loading = true;
    error = '';
    try {
      const response = await api('/api/projects/' + page.params.id);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load project');
      data = payload;
      domainBindings = (payload.domainBindings || []).map((binding) => ({
        domain: binding.domain,
        httpsEnabled: binding.httpsEnabled || false,
        rules: (binding.rules || []).map((rule) => ({ path: rule.path, port: rule.port, serviceId: rule.serviceId || '' }))
      }));
      settingsForm = {
        name: payload.project.name || '',
        sourceType: payload.project.sourceType || 'image',
        repository: payload.project.repository || '',
        branch: payload.project.branch || 'main',
        connectionId: payload.project.connectionId || '',
        imageUrl: payload.project.imageUrl || '',
        registryId: payload.project.registryId || '',
		containerPort: payload.project.containerPort || 80
      };
      const runtimeServices = payload.applicationServices || [];
      if (runtimeServices.length > 0 && !runtimeServices.some((item) => item.id === runtimeSettingsServiceId)) {
        selectRuntimeSettingsService(runtimeServices[0]);
      }
      clearTimeout(projectPollTimer);
      if ((payload.deployments || []).some((item) => ['deploying', 'building'].includes(item.status))) {
        projectPollTimer = setTimeout(() => loadProject(true), 1200);
      }
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load project';
    } finally {
      if (!silent) loading = false;
    }
  }

  async function loadIntegrations() {
    try {
      const response = await api('/api/integrations');
      if (response.ok) integrations = await response.json();
    } catch {
      integrations = { connections: [], registries: [] };
    }
  }

  function selectedServiceConnection() {
    return (integrations.connections || []).find((item) => item.id === serviceForm.connectionId);
  }

  function githubContentsPermissionMissing() {
    const connection = selectedServiceConnection();
    if (!connection || connection.provider !== 'github') return false;
    return connection.contentsPermission !== 'read' && connection.contentsPermission !== 'write';
  }

  async function saveProjectSettings() {
    settingsSaving = true;
    settingsError = '';
    settingsNotice = '';
    try {
      const response = await api('/api/projects/' + page.params.id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settingsForm)
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not update project');
      data = { ...data, project: payload };
      settingsForm = {
        name: payload.name,
        sourceType: payload.sourceType,
        repository: payload.repository || '',
        branch: payload.branch || 'main',
        connectionId: payload.connectionId || '',
        imageUrl: payload.imageUrl || '',
        registryId: payload.registryId || '',
		containerPort: payload.containerPort || 80
      };
      settingsNotice = 'Project details saved.';
    } catch (cause) {
      settingsError = cause instanceof Error ? cause.message : 'Could not update project';
    } finally {
      settingsSaving = false;
    }
  }

  function openDeleteModal() {
    deleteConfirmation = '';
    deleteError = '';
    deleteModal = true;
  }

  async function deleteProject() {
    if (deleteConfirmation !== project.name) return;
    deleteBusy = true;
    deleteError = '';
    try {
      const response = await api('/api/projects/' + page.params.id, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ confirmation: deleteConfirmation })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not delete project');
      await goto('/projects');
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not delete project';
      deleteBusy = false;
    }
  }

  function selectTab(tab) {
    activeTab = tab;
    history.replaceState(null, '', '#' + tab);
    if (tab === 'logs') {
      tick().then(async () => {
        if (activeTab !== 'logs') return;
        await loadLogs();
        startLogPolling();
      });
    } else {
      stopLogPolling();
      if (tab === 'environment' && environmentVariables.length === 0) loadEnvironment();
    }
    if (tab === 'metrics') {
      loadProjectMetrics();
      startMetricsPolling();
    } else {
      stopMetricsPolling();
    }
  }

  async function loadProjectMetrics(silent = false) {
    if (silent) metricsRefreshing = true;
    else metricsLoading = true;
    metricsError = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/metrics');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load project metrics');
      projectMetrics = { ...payload, containers: payload.containers || [] };
    } catch (cause) {
      metricsError = cause instanceof Error ? cause.message : 'Could not load project metrics';
    } finally {
      metricsLoading = false;
      metricsRefreshing = false;
    }
  }

  function startMetricsPolling() {
    stopMetricsPolling();
    metricsTimer = setInterval(() => {
      if (activeTab === 'metrics' && !metricsRefreshing) loadProjectMetrics(true);
    }, 5000);
  }

  function stopMetricsPolling() {
    if (metricsTimer) clearInterval(metricsTimer);
    metricsTimer = null;
  }

  const emptyEnvironmentVariable = () => ({ key: '', value: '', secret: false, revealed: false });

  async function loadEnvironment(targetId = environmentTargetId) {
    const target = environmentTargets.find((item) => item.id === targetId) || environmentTargets[0];
    const requestId = ++environmentLoadRequest;
    if (!target) {
      environmentVariables = [emptyEnvironmentVariable()];
      environmentLoading = false;
      return;
    }
    environmentLoading = true;
    environmentError = '';
    try {
      const endpoint = target.legacy ? '/api/projects/' + page.params.id + '/environment' : '/api/services/' + target.id + '/environment';
      const response = await api(endpoint);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load environment variables');
      if (requestId !== environmentLoadRequest) return;
      environmentVariables = (payload.variables || []).map((variable) => ({ ...variable, revealed: false }));
      if (environmentVariables.length === 0) environmentVariables = [emptyEnvironmentVariable()];
    } catch (cause) {
      if (requestId !== environmentLoadRequest) return;
      environmentError = cause instanceof Error ? cause.message : 'Could not load environment variables';
    } finally {
      if (requestId === environmentLoadRequest) environmentLoading = false;
    }
  }

  async function selectEnvironmentTarget(id) {
    environmentTargetId = id;
    environmentVariables = [];
    environmentNotice = '';
    environmentError = '';
    await loadEnvironment(id);
  }

  function openEnvironmentFor(id) {
    environmentTargetId = id;
    environmentVariables = [];
    selectTab('environment');
  }

  function addEnvironmentVariable() {
    environmentVariables = [...environmentVariables, emptyEnvironmentVariable()];
    environmentNotice = '';
  }

  function removeEnvironmentVariable(index) {
    environmentVariables = environmentVariables.filter((_, current) => current !== index);
    if (environmentVariables.length === 0) environmentVariables = [emptyEnvironmentVariable()];
    environmentNotice = '';
  }

  function toggleEnvironmentValue(index) {
    environmentVariables = environmentVariables.map((variable, current) => current === index ? { ...variable, revealed: !variable.revealed } : variable);
  }

  function formatDotEnvValue(value) {
    if (value === '') return '';
    if (/^[A-Za-z0-9_./:@%+,\-]+$/.test(value)) return value;
    return JSON.stringify(value);
  }

  function openBulkEnvironment() {
    bulkEnvironmentText = environmentVariables
      .filter((variable) => variable.key.trim())
      .map((variable) => `${variable.key.trim()}=${formatDotEnvValue(variable.value)}`)
      .join('\n');
    bulkEnvironmentError = '';
    bulkEnvironmentModal = true;
  }

  function parseDotEnvValue(rawValue, lineNumber) {
    let value = rawValue.trim();
    if (value.startsWith('"') || value.startsWith("'")) {
      const quote = value[0];
      let closing = -1;
      for (let index = 1; index < value.length; index += 1) {
        if (value[index] !== quote) continue;
        let backslashes = 0;
        for (let previous = index - 1; previous >= 0 && value[previous] === '\\'; previous -= 1) backslashes += 1;
        if (backslashes % 2 === 0) {
          closing = index;
          break;
        }
      }
      if (closing < 0) throw new Error(`Line ${lineNumber}: quoted value is not closed.`);
      const trailing = value.slice(closing + 1).trim();
      if (trailing && !trailing.startsWith('#')) throw new Error(`Line ${lineNumber}: unexpected text after the quoted value.`);
      const inner = value.slice(1, closing);
      if (quote === "'") return inner;
      return inner.replace(/\\(.)/g, (match, escaped) => {
        if (escaped === 'n') return '\n';
        if (escaped === 'r') return '\r';
        if (escaped === 't') return '\t';
        if (escaped === '"') return '"';
        if (escaped === '\\') return '\\';
        return match;
      });
    }
    const comment = value.search(/\s+#/);
    if (comment >= 0) value = value.slice(0, comment);
    return value.trim();
  }

  function parseDotEnv(text) {
    const currentSecrets = new Map(environmentVariables.filter((variable) => variable.key.trim()).map((variable) => [variable.key.trim(), variable.secret]));
    const seen = new Set();
    const variables = [];
    const lines = text.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n').split('\n');
    lines.forEach((rawLine, index) => {
      let line = rawLine.trim();
      const lineNumber = index + 1;
      if (!line || line.startsWith('#')) return;
      if (line.startsWith('export ')) line = line.slice(7).trimStart();
      const separator = line.indexOf('=');
      if (separator < 1) throw new Error(`Line ${lineNumber}: expected KEY=value.`);
      const key = line.slice(0, separator).trim();
      if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) throw new Error(`Line ${lineNumber}: “${key}” is not a valid environment variable key.`);
      if (seen.has(key)) throw new Error(`Line ${lineNumber}: “${key}” is already defined.`);
      seen.add(key);
      const secret = currentSecrets.get(key) ?? /(SECRET|TOKEN|PASSWORD|PASS|PRIVATE|CREDENTIAL|AUTH|API_KEY)/i.test(key);
      variables.push({ key, value: parseDotEnvValue(line.slice(separator + 1), lineNumber), secret, revealed: false });
    });
    return variables;
  }

  function applyBulkEnvironment() {
    bulkEnvironmentError = '';
    try {
      const variables = parseDotEnv(bulkEnvironmentText);
      environmentVariables = variables.length > 0 ? variables : [emptyEnvironmentVariable()];
      environmentError = '';
      environmentNotice = '';
      bulkEnvironmentModal = false;
    } catch (cause) {
      bulkEnvironmentError = cause instanceof Error ? cause.message : 'Could not parse the .env content.';
    }
  }

  async function saveEnvironment() {
    environmentError = '';
    environmentNotice = '';
    const variables = environmentVariables
      .filter((variable) => variable.key.trim() || variable.value)
      .map(({ key, value, secret }) => ({ key: key.trim(), value, secret }));
    if (variables.some((variable) => !variable.key)) {
      environmentError = 'Every value needs an environment variable key.';
      return;
    }
    if (variables.some((variable) => !/^[A-Za-z_][A-Za-z0-9_]*$/.test(variable.key))) {
      environmentError = 'Keys must start with a letter or underscore and use only letters, numbers, and underscores.';
      return;
    }
    if (new Set(variables.map((variable) => variable.key)).size !== variables.length) {
      environmentError = 'Each environment variable key must be unique.';
      return;
    }
    environmentSaving = true;
    try {
      const endpoint = activeEnvironmentTarget?.legacy ? '/api/projects/' + page.params.id + '/environment' : '/api/services/' + activeEnvironmentTarget.id + '/environment';
      const response = await api(endpoint, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ variables })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not apply environment variables');
      environmentVariables = (payload.variables || []).map((variable) => ({ ...variable, revealed: false }));
      if (environmentVariables.length === 0) environmentVariables = [emptyEnvironmentVariable()];
      environmentNotice = payload.message || 'Environment saved and application restarted.';
      await loadProject();
    } catch (cause) {
      environmentError = cause instanceof Error ? cause.message : 'Could not apply environment variables';
    } finally {
      environmentSaving = false;
    }
  }

  async function loadLogs() {
    const target = activeLogTarget;
    if (!target) {
      logs = [];
      logEvents = [];
      logsError = '';
      return;
    }
    const request = ++logRequest;
    logsLoading = true;
    logsError = '';
    try {
      let endpoint;
      if (target.kind === 'database' && logView === 'deployment') endpoint = `/api/databases/${target.id}/events`;
      else if (target.kind === 'database') endpoint = `/api/databases/${target.id}/logs?lines=${logLimit}`;
      else if (target.legacy) endpoint = `/api/projects/${page.params.id}/logs?lines=${logLimit}`;
      else endpoint = `/api/services/${target.id}/logs?lines=${logLimit}`;
      const response = await api(endpoint);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || `Could not load ${logView === 'deployment' ? 'deployment events' : 'container logs'}`);
      if (request !== logRequest) return;
      if (logView === 'deployment') {
        logEvents = payload.events || [];
        logs = [];
      } else {
        logs = payload.lines || [];
        logEvents = [];
      }
      logsUpdated = new Date().toLocaleTimeString();
      await tick();
      if (logsLive && logConsole) logConsole.scrollTop = logConsole.scrollHeight;
    } catch (cause) {
      if (request !== logRequest) return;
      logsError = cause instanceof Error ? cause.message : 'Could not load logs';
      logs = [];
      logEvents = [];
    } finally {
      if (request === logRequest) logsLoading = false;
    }
  }

  function openWorkloadLogs(item, kind, initialView = 'runtime') {
    logTargetId = `${kind}:${item.id}`;
    logView = kind === 'database' ? initialView : 'runtime';
    logs = [];
    logEvents = [];
    logsError = '';
    logsUpdated = '';
    selectTab('logs');
  }

  async function openServiceTerminal(item, trigger) {
    if (terminalService?.id !== item.id) {
      terminalEntries = [];
      terminalHistory = [];
      terminalWorkingDir = '';
    }
    terminalService = item;
    terminalReturnFocus = trigger;
    terminalHistoryIndex = -1;
    await tick();
    terminalInput?.focus();
  }

  async function closeServiceTerminal() {
    if (terminalRunning) return;
    terminalService = null;
    terminalCommand = '';
    terminalHistoryIndex = -1;
    await tick();
    terminalReturnFocus?.focus();
  }

  function clearTerminal() {
    if (terminalRunning) return;
    terminalEntries = [];
    terminalInput?.focus();
  }

  function terminalKeydown(event) {
    if (event.key === 'ArrowUp') {
      if (terminalHistory.length === 0) return;
      event.preventDefault();
      terminalHistoryIndex = Math.min(terminalHistory.length - 1, terminalHistoryIndex + 1);
      terminalCommand = terminalHistory[terminalHistory.length - 1 - terminalHistoryIndex];
    } else if (event.key === 'ArrowDown') {
      if (terminalHistoryIndex < 0) return;
      event.preventDefault();
      terminalHistoryIndex -= 1;
      terminalCommand = terminalHistoryIndex < 0 ? '' : terminalHistory[terminalHistory.length - 1 - terminalHistoryIndex];
    }
  }

  async function runTerminalCommand() {
    const command = terminalCommand.trim();
    if (!terminalService || !command || terminalRunning) return;
    const entry = {
      id: `${Date.now()}-${terminalEntries.length}`,
      command,
      workingDir: terminalWorkingDir.trim(),
      status: 'running',
      startedAt: new Date().toLocaleTimeString()
    };
    terminalEntries = [...terminalEntries, entry];
    if (terminalHistory.at(-1) !== command) terminalHistory = [...terminalHistory, command];
    terminalHistoryIndex = -1;
    terminalCommand = '';
    terminalRunning = true;
    await tick();
    if (terminalOutput) terminalOutput.scrollTop = terminalOutput.scrollHeight;
    try {
      const response = await api(`/api/services/${terminalService.id}/exec`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command, workingDir: entry.workingDir })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not execute command');
      terminalEntries = terminalEntries.map((item) => item.id === entry.id
        ? { ...item, status: 'complete', ...payload.result }
        : item);
    } catch (cause) {
      terminalEntries = terminalEntries.map((item) => item.id === entry.id
        ? { ...item, status: 'error', error: cause instanceof Error ? cause.message : 'Could not execute command' }
        : item);
    } finally {
      terminalRunning = false;
      await tick();
      if (terminalOutput) terminalOutput.scrollTop = terminalOutput.scrollHeight;
      terminalInput?.focus();
    }
  }

  async function selectLogTarget(key) {
    stopLogPolling();
    logTargetId = key;
    logView = 'runtime';
    logs = [];
    logEvents = [];
    logsError = '';
    logsUpdated = '';
    await tick();
    await loadLogs();
    startLogPolling();
  }

  async function selectLogView(view) {
    if (!activeLogTarget || (view === 'deployment' && activeLogTarget.kind !== 'database')) return;
    stopLogPolling();
    logView = view;
    logs = [];
    logEvents = [];
    logsError = '';
    logsUpdated = '';
    await tick();
    await loadLogs();
    startLogPolling();
  }

  function startLogPolling() {
    stopLogPolling();
    if (!logsLive || activeTab !== 'logs') return;
    logPollTimer = setInterval(loadLogs, 2000);
  }

  function stopLogPolling() {
    if (logPollTimer) clearInterval(logPollTimer);
    logPollTimer = undefined;
  }

  function toggleLiveLogs() {
    logsLive = !logsLive;
    if (logsLive) {
      loadLogs();
      startLogPolling();
    } else {
      stopLogPolling();
    }
  }

  function changeLogLimit() {
    loadLogs();
    startLogPolling();
  }

  function parseLogLine(line, index) {
    const timestampMatch = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(.*)$/);
    const timestamp = timestampMatch?.[1] || '';
    const message = timestampMatch?.[2] || line;
    const normalized = message.toLowerCase();
    let severity = 'info';
    if (/\b(error|fatal|panic|critical|crit|emerg|alert)\b/.test(normalized) || /"\s5\d{2}\s/.test(message)) severity = 'error';
    else if (/\b(warn|warning)\b/.test(normalized) || /"\s4\d{2}\s/.test(message)) severity = 'warning';
    else if (/\b(debug|trace|verbose)\b/.test(normalized)) severity = 'debug';
    return { index: index + 1, timestamp, time: timestamp ? timestamp.slice(11, 23) : '—', message, severity };
  }

  async function deploy() {
    deploying = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/deploy', { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Deployment failed');
      await goto('/deployments/' + payload.deployment.id);
    } catch (cause) {
      const failure = cause instanceof Error ? cause.message : 'Deployment failed';
      await loadProject();
      error = failure;
    } finally {
      deploying = false;
    }
  }

  function newDomainBinding() {
    const target = routeTargets[0];
    return { domain: '', httpsEnabled: false, rules: [{ path: '/*', port: target?.containerPort || 80, serviceId: target?.id || '' }] };
  }

  function copyDomainBinding(binding) {
    return { ...binding, rules: binding.rules.map((rule) => ({ ...rule })) };
  }

  function openDomainModal(index = -1) {
    domainEditingIndex = index;
    domainDraft = index === -1 ? newDomainBinding() : copyDomainBinding(domainBindings[index]);
    domainError = '';
    domainModal = true;
  }

  function closeDomainModal() {
    if (domainSaving) return;
    domainModal = false;
    domainDraft = null;
    domainEditingIndex = -1;
  }

  async function persistDomainBindings(bindings) {
    domainSaving = true;
    domainError = '';
    domainNotice = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/domain', {
        method: 'PUT',
        body: JSON.stringify({ domains: bindings.map((binding) => ({ domain: binding.domain.trim(), httpsEnabled: binding.httpsEnabled, rules: binding.rules.map((rule) => ({ path: rule.path.trim(), port: Number(rule.port), serviceId: rule.serviceId || '' })) })) })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not update domain');
      data = { ...data, project: payload.project };
      domainBindings = (payload.domainBindings || []).map((binding) => ({ domain: binding.domain, httpsEnabled: binding.httpsEnabled || false, rules: (binding.rules || []).map((rule) => ({ path: rule.path, port: rule.port, serviceId: rule.serviceId || '' })) }));
      domainNotice = payload.active ? `Caddy activated ${domainBindings.length} domain${domainBindings.length === 1 ? '' : 's'} and all path rules.` : 'All domain routes were removed.';
      return true;
    } catch (cause) {
      domainError = cause instanceof Error ? cause.message : 'Could not update domain';
      return false;
    } finally {
      domainSaving = false;
    }
  }

  async function saveDomainDraft() {
    if (!domainDraft) return;
    const bindings = domainEditingIndex === -1
      ? [...domainBindings, domainDraft]
      : domainBindings.map((binding, index) => index === domainEditingIndex ? domainDraft : binding);
    if (await persistDomainBindings(bindings)) closeDomainModal();
  }

  async function deleteDomainDraft() {
    if (domainEditingIndex === -1) return;
    if (await persistDomainBindings(domainBindings.filter((_, index) => index !== domainEditingIndex))) closeDomainModal();
  }

  function addDomainRule() {
    if (!domainDraft) return;
    const target = routeTargets[0];
    domainDraft = { ...domainDraft, rules: [...domainDraft.rules, { path: '/api/*', port: target?.containerPort || 8080, serviceId: target?.id || '' }] };
  }

  function removeDomainRule(ruleIndex) {
    if (!domainDraft) return;
    domainDraft = { ...domainDraft, rules: domainDraft.rules.filter((_, index) => index !== ruleIndex) };
  }

  function routeTargetName(serviceId) {
    return routeTargets.find((target) => target.id === (serviceId || ''))?.name || 'application';
  }

  function setRuleService(ruleIndex, serviceId) {
    if (!domainDraft) return;
    const target = routeTargets.find((item) => item.id === serviceId) || routeTargets[0];
    domainDraft = { ...domainDraft, rules: domainDraft.rules.map((rule, index) => index === ruleIndex ? { ...rule, serviceId, port: target?.containerPort || 80 } : rule) };
  }

  function openServiceModal() {
    serviceError = '';
    serviceRepositories = [];
    serviceRepositoriesError = '';
    serviceRepositoryQuery = '';
    serviceRepositoryPickerOpen = false;
    serviceForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', healthCheckType: 'none', healthCheckPath: '/', healthCheckCommand: '', healthCheckTimeoutSeconds: 60, environment: '' };
    serviceModal = true;
  }

  function openComposeModal() {
    composeText = '';
    composeFileName = '';
    composeValidation = null;
    composeError = '';
    composeModal = true;
  }

  function updateComposeText(value) {
    composeText = value;
    composeValidation = null;
    composeError = '';
  }

  async function chooseComposeFile(event) {
    const file = event.currentTarget.files?.[0];
    if (!file) return;
    composeFileName = file.name;
    composeValidation = null;
    composeError = '';
    try {
      composeText = await file.text();
    } catch {
      composeError = 'Could not read this file.';
    }
    event.currentTarget.value = '';
  }

  async function validateCompose() {
    composeValidating = true;
    composeError = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/compose/validate', {
        method: 'POST',
        body: JSON.stringify({ compose: composeText })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not validate Compose file');
      composeValidation = payload;
    } catch (cause) {
      composeError = cause instanceof Error ? cause.message : 'Could not validate Compose file';
    } finally {
      composeValidating = false;
    }
  }

  async function importCompose() {
    if (!composeValidation?.valid) return;
    composeImporting = true;
    composeError = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/compose', {
        method: 'POST',
        body: JSON.stringify({ compose: composeText })
      });
      const payload = await response.json();
      if (!response.ok) {
        if (payload.preview) composeValidation = payload.preview;
        throw new Error(payload.error || 'Could not import Compose file');
      }
      const applications = payload.services?.length || 0;
      const databases = payload.databases?.length || 0;
      const deploymentErrors = payload.deploymentErrors?.length || 0;
      composeModal = false;
      notice = `Imported ${applications + databases} service${applications + databases === 1 ? '' : 's'} from Compose. ${applications - deploymentErrors} application deployment${applications - deploymentErrors === 1 ? '' : 's'} started${databases ? ` and ${databases} database${databases === 1 ? '' : 's'} created` : ''}.`;
      if (deploymentErrors) error = `${deploymentErrors} imported application deployment${deploymentErrors === 1 ? '' : 's'} could not be started. Review the service rows for details.`;
      await loadProject(true);
    } catch (cause) {
      composeError = cause instanceof Error ? cause.message : 'Could not import Compose file';
    } finally {
      composeImporting = false;
    }
  }

  async function chooseServiceSource(sourceType) {
    serviceForm = { ...serviceForm, sourceType, imageUrl: sourceType === 'image' ? serviceForm.imageUrl : '', registryId: sourceType === 'image' ? serviceForm.registryId : '', connectionId: sourceType === 'repository' ? serviceForm.connectionId : '', repository: sourceType === 'repository' ? serviceForm.repository : '' };
    serviceRepositoriesError = '';
    if (sourceType === 'repository' && !serviceForm.connectionId && (integrations.connections || []).length === 1) {
      serviceForm.connectionId = integrations.connections[0].id;
    }
    if (sourceType === 'repository' && serviceForm.connectionId) await loadServiceRepositories(serviceForm.connectionId);
  }

  async function loadServiceRepositories(connectionId) {
    serviceRepositories = [];
    serviceRepositoriesError = '';
    serviceRepositoriesLoading = true;
    if (!connectionId) {
      serviceRepositoriesLoading = false;
      return;
    }
    try {
      const response = await api('/api/integrations/sources/' + connectionId + '/repositories');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load repositories');
      // Accept the documented object response and the older array response so
      // a control-plane upgrade never leaves the picker unexpectedly empty.
      serviceRepositories = Array.isArray(payload) ? payload : (payload.repositories || []);
      const selected = serviceRepositories.find((item) => item.fullName === serviceForm.repository);
      serviceRepositoryQuery = selected?.fullName || '';
      if (selected?.defaultBranch) serviceForm.branch = selected.defaultBranch;
    } catch (cause) {
      serviceRepositoriesError = cause instanceof Error ? cause.message : 'Could not load repositories';
    } finally {
      serviceRepositoriesLoading = false;
    }
  }

  async function changeServiceConnection() {
    serviceForm.repository = '';
    serviceForm.branch = 'main';
    serviceRepositoryQuery = '';
    serviceRepositoryPickerOpen = false;
    await loadServiceRepositories(serviceForm.connectionId);
  }

  function searchServiceRepositories() {
    serviceForm.repository = '';
    serviceRepositoryPickerOpen = true;
  }

  function selectServiceRepository(repository) {
    serviceForm.repository = repository.fullName;
    serviceRepositoryQuery = repository.fullName;
    serviceRepositoryPickerOpen = false;
    const selected = repository;
    if (selected?.defaultBranch) serviceForm.branch = selected.defaultBranch;
  }

  function openServiceSettings(item) {
    serviceSettingsService = item;
    serviceSettingsError = '';
    serviceSettingsForm = { name: item.name, sourceType: item.sourceType || 'image', imageUrl: item.imageUrl || '', containerPort: item.containerPort || 80, registryId: item.registryId || '', connectionId: item.connectionId || '', repository: item.repository || '', branch: item.branch || 'main', dockerfilePath: item.dockerfilePath || 'Dockerfile', buildContext: item.buildContext || '.', buildStrategy: item.buildStrategy || 'dockerfile', command: item.command || '', healthCheckType: item.healthCheckType || 'none', healthCheckPath: item.healthCheckPath || '/', healthCheckCommand: item.healthCheckCommand || '', healthCheckTimeoutSeconds: item.healthCheckTimeoutSeconds || 60 };
  }

  async function saveServiceSettings() {
    if (!serviceSettingsService) return;
    serviceSettingsSaving = true;
    serviceSettingsError = '';
    try {
      const response = await api('/api/services/' + serviceSettingsService.id, { method: 'PUT', body: JSON.stringify({ ...serviceSettingsForm, containerPort: Number(serviceSettingsForm.containerPort) }) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save service configuration');
      const serviceName = payload.service?.name || serviceSettingsForm.name;
      serviceSettingsService = null;
      notice = `${serviceName} configuration was saved. Deploy it to apply the image, port, or command.`;
      await loadProject();
    } catch (cause) {
      serviceSettingsError = cause instanceof Error ? cause.message : 'Could not save service configuration';
    } finally {
      serviceSettingsSaving = false;
    }
  }

  async function selectRuntimeSettingsService(item) {
    runtimeSettingsServiceId = item.id;
    runtimeSettingsForm = {
      name: item.name || '',
      sourceType: item.sourceType || 'image',
      imageUrl: item.imageUrl || '',
      containerPort: item.containerPort || 80,
      registryId: item.registryId || '',
      connectionId: item.connectionId || '',
      repository: item.repository || '',
      branch: item.branch || 'main',
      dockerfilePath: item.dockerfilePath || 'Dockerfile',
      buildContext: item.buildContext || '.',
      buildStrategy: item.buildStrategy || 'dockerfile',
      command: item.command || '',
      healthCheckType: item.healthCheckType || 'none',
      healthCheckPath: item.healthCheckPath || '/',
      healthCheckCommand: item.healthCheckCommand || '',
      healthCheckTimeoutSeconds: item.healthCheckTimeoutSeconds || 60
    };
    runtimeSettingsError = '';
    runtimeSettingsNotice = '';
    await loadDeploymentTriggers(item.id);
  }

  function imageTag(image) {
    const clean = (image || '').split('@')[0];
    const slash = clean.lastIndexOf('/');
    const colon = clean.lastIndexOf(':');
    return colon > slash ? clean.slice(colon + 1) : 'latest';
  }

  async function loadDeploymentTriggers(serviceId) {
    runtimeTriggersLoading = true;
    runtimeTriggersError = '';
    runtimeTriggersNotice = '';
    try {
      const response = await api('/api/services/' + serviceId + '/deployment-triggers');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load deployment triggers');
      runtimeTriggers = {
        autoDeploy: payload.autoDeploy || false,
        registryWebhookEnabled: payload.registryWebhookEnabled || false,
        registryWebhookTag: payload.registryWebhookTag || (runtimeSettingsForm.sourceType === 'image' ? imageTag(runtimeSettingsForm.imageUrl) : ''),
        webhookUrl: payload.webhookUrl || '',
        webhookConfigured: payload.webhookConfigured || false
      };
    } catch (cause) {
      runtimeTriggersError = cause instanceof Error ? cause.message : 'Could not load deployment triggers';
    } finally {
      runtimeTriggersLoading = false;
    }
  }

  async function saveDeploymentTriggers() {
    if (!runtimeSettingsService) return;
    runtimeTriggersSaving = true;
    runtimeTriggersError = '';
    runtimeTriggersNotice = '';
    try {
      const response = await api('/api/services/' + runtimeSettingsService.id + '/deployment-triggers', {
        method: 'PUT',
        body: JSON.stringify({
          autoDeploy: runtimeTriggers.autoDeploy,
          registryWebhookEnabled: runtimeTriggers.registryWebhookEnabled,
          registryWebhookTag: runtimeTriggers.registryWebhookTag
        })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save deployment triggers');
      runtimeTriggers = {
        autoDeploy: payload.autoDeploy || false,
        registryWebhookEnabled: payload.registryWebhookEnabled || false,
        registryWebhookTag: payload.registryWebhookTag || '',
        webhookUrl: payload.webhookUrl || '',
        webhookConfigured: payload.webhookConfigured || false
      };
      runtimeTriggersNotice = runtimeSettingsForm.sourceType === 'repository' ? 'Git push auto-deploy settings saved.' : 'Registry webhook settings saved.';
      await loadProject();
    } catch (cause) {
      runtimeTriggersError = cause instanceof Error ? cause.message : 'Could not save deployment triggers';
    } finally {
      runtimeTriggersSaving = false;
    }
  }

  async function copyWebhookUrl() {
    if (!runtimeTriggers.webhookUrl) return;
    await writeClipboard(runtimeTriggers.webhookUrl);
    copiedField = 'webhook-url';
    setTimeout(() => { if (copiedField === 'webhook-url') copiedField = ''; }, 1600);
  }

  async function saveRuntimeSettings(deployAfterSave = false) {
    if (!runtimeSettingsService) return;
    runtimeSettingsBusy = deployAfterSave ? 'deploy' : 'save';
    runtimeSettingsError = '';
    runtimeSettingsNotice = '';
    try {
      const response = await api('/api/services/' + runtimeSettingsService.id, {
        method: 'PUT',
        body: JSON.stringify({ ...runtimeSettingsForm, containerPort: Number(runtimeSettingsForm.containerPort) })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save service configuration');
      runtimeSettingsForm = {
        name: payload.service.name,
        sourceType: payload.service.sourceType || 'image',
        imageUrl: payload.service.imageUrl,
        containerPort: payload.service.containerPort || 80,
        registryId: payload.service.registryId || '',
        connectionId: payload.service.connectionId || '',
        repository: payload.service.repository || '',
        branch: payload.service.branch || 'main',
        dockerfilePath: payload.service.dockerfilePath || 'Dockerfile',
        buildContext: payload.service.buildContext || '.',
        buildStrategy: payload.service.buildStrategy || 'dockerfile',
        command: payload.service.command || '',
        healthCheckType: payload.service.healthCheckType || 'none',
        healthCheckPath: payload.service.healthCheckPath || '/',
        healthCheckCommand: payload.service.healthCheckCommand || '',
        healthCheckTimeoutSeconds: payload.service.healthCheckTimeoutSeconds || 60
      };
      if (deployAfterSave) {
        const deployResponse = await api('/api/services/' + runtimeSettingsService.id + '/deploy', { method: 'POST' });
        const deployed = await deployResponse.json();
        if (!deployResponse.ok) throw new Error(deployed.error || 'Configuration was saved, but deployment could not start');
        runtimeSettingsNotice = `${payload.service.name} was saved and is being redeployed with the new runtime command.`;
      } else {
        runtimeSettingsNotice = `${payload.service.name} was saved. Deploy it when you are ready to apply the new command.`;
      }
      await loadProject();
    } catch (cause) {
      runtimeSettingsError = cause instanceof Error ? cause.message : 'Could not save service configuration';
    } finally {
      runtimeSettingsBusy = '';
    }
  }

  async function createApplicationService() {
    serviceSaving = true;
    serviceError = '';
    try {
      const createResponse = await api('/api/projects/' + page.params.id + '/services', { method: 'POST', body: JSON.stringify({ ...serviceForm, containerPort: Number(serviceForm.containerPort) }) });
      const created = await createResponse.json();
      if (!createResponse.ok) throw new Error(created.error || 'Could not create service');
      const deployResponse = await api('/api/services/' + created.service.id + '/deploy', { method: 'POST' });
      const deployed = await deployResponse.json();
      if (!deployResponse.ok) throw new Error(deployed.error || 'Service was created but could not start deployment');
      serviceModal = false;
      notice = created.service.sourceType === 'repository' ? `${created.service.name} was added. Its repository is being cloned and built in the background.` : `${created.service.name} was added. Its image is being pulled in the background.`;
      await loadProject(true);
    } catch (cause) {
      serviceError = cause instanceof Error ? cause.message : 'Could not create service';
    } finally {
      serviceSaving = false;
    }
  }

  async function deployApplicationService(item) {
    error = '';
    const response = await api('/api/services/' + item.id + '/deploy', { method: 'POST' });
    const payload = await response.json();
    if (!response.ok) {
      error = payload.error || 'Could not deploy service';
      return;
    }
    notice = item.sourceType === 'repository' ? `${item.name} is cloning ${item.repository} and building a new image.` : `${item.name} is pulling ${item.imageUrl}.`;
    await loadProject(true);
  }

  async function controlWorkload(item, kind, action) {
    if (lifecycleBusy) return;
    const key = `${kind}:${item.id}:${action}`;
    lifecycleBusy = key;
    error = '';
    notice = '';
    try {
      let endpoint;
      if (kind === 'database') endpoint = `/api/databases/${item.id}/${action}`;
      else if (item.legacy) endpoint = `/api/projects/${page.params.id}/${action}`;
      else endpoint = `/api/services/${item.id}/${action}`;
      const response = await api(endpoint, { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || `Could not ${action} ${item.name}`);
      notice = payload.message || `${item.name} ${action === 'stop' ? 'stopped' : 'restarted'}.`;
      await loadProject(true);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : `Could not ${action} ${item.name}`;
    } finally {
      lifecycleBusy = '';
    }
  }

  function workloadActionBusy(item, kind, action) {
    return lifecycleBusy === `${kind}:${item.id}:${action}`;
  }

  function openApplicationDelete(item) {
    applicationDeleteService = item;
    applicationDeleteError = '';
  }

  async function deleteApplicationService() {
    if (!applicationDeleteService) return;
    applicationDeleteBusy = true;
    applicationDeleteError = '';
    try {
      const response = await api('/api/services/' + applicationDeleteService.id, { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not remove service');
      notice = `${applicationDeleteService.name} was removed.`;
      applicationDeleteService = null;
      await loadProject();
    } catch (cause) {
      applicationDeleteError = cause instanceof Error ? cause.message : 'Could not remove service';
    } finally {
      applicationDeleteBusy = false;
    }
  }

  function domainEndpoint(binding) {
    const protocol = binding.httpsEnabled ? 'https:' : platformProtocol;
    const port = !binding.httpsEnabled && platformPort ? ':' + platformPort : '';
    return `${protocol}//${binding.domain}${port}`;
  }

  function openDatabaseModal() {
    databaseError = '';
    databaseForm = { engine: 'mysql', name: 'MySQL', databaseName: 'app', username: 'app', password: '', publicEnabled: false, publicPort: 3306 };
    databaseModal = true;
  }

  function selectDatabaseEngine(engine) {
    const previousLabel = databasePresets[databaseForm.engine].label;
    const next = databasePresets[engine];
    databaseForm = { ...databaseForm, engine, name: databaseForm.name === previousLabel ? next.label : databaseForm.name, publicPort: next.port };
  }

  async function createDatabase() {
    databaseSaving = true;
    databaseError = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/databases', {
        method: 'POST',
        body: JSON.stringify(databaseForm)
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not deploy database');
      databaseModal = false;
      credentials = payload.credentials;
      credentialsService = payload.service;
      credentialsModal = true;
      notice = `${payload.service.name} is starting with private networking${payload.service.publicEnabled ? ` and public port ${payload.service.publicPort}` : ''}.`;
      await loadProject();
    } catch (cause) {
      databaseError = cause instanceof Error ? cause.message : 'Could not deploy database';
    } finally {
      databaseSaving = false;
    }
  }

  async function showCredentials(item) {
    credentialsModal = true;
    credentialsLoading = true;
    credentialsService = item;
    credentials = null;
    copiedField = '';
    try {
      const response = await api('/api/databases/' + item.id + '/credentials');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not reveal credentials');
      credentials = payload;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not reveal credentials';
      credentialsModal = false;
    } finally {
      credentialsLoading = false;
    }
  }

  function openExposure(item) {
    exposureService = item;
    exposurePort = item.publicPort || item.internalPort;
    databaseError = '';
  }

  async function saveExposure(enabled) {
    const item = exposureService || databaseServices.find((database) => database.publicEnabled);
    if (!item) return;
    exposureSaving = true;
    databaseError = '';
    try {
      const response = await api('/api/databases/' + item.id + '/exposure', {
        method: 'PUT',
        body: JSON.stringify({ enabled, port: enabled ? Number(exposurePort) : 0 })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not update database exposure');
      exposureService = null;
      notice = enabled ? `${item.name} is now available on public port ${exposurePort}.` : `${item.name} is private again.`;
      await loadProject();
    } catch (cause) {
      databaseError = cause instanceof Error ? cause.message : 'Could not update database exposure';
      error = databaseError;
    } finally {
      exposureSaving = false;
    }
  }

  async function makePrivate(item) {
    exposureService = item;
    await saveExposure(false);
  }

  function openDatabaseDelete(item) {
    databaseDeleteService = item;
    databaseDeleteConfirmation = '';
    databaseDeleteVolume = false;
    databaseDeleteError = '';
  }

  async function deleteDatabase() {
    if (!databaseDeleteService) return;
    databaseDeleteBusy = true;
    databaseDeleteError = '';
    try {
      const response = await api(`/api/databases/${databaseDeleteService.id}`, {
        method: 'DELETE',
        body: JSON.stringify({ confirmation: databaseDeleteConfirmation, removeVolume: databaseDeleteVolume })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not remove database');
      notice = payload.volumeRemoved ? `${databaseDeleteService.name} and its persistent data were removed.` : `${databaseDeleteService.name} was removed. Volume ${payload.retainedVolume} was retained.`;
      databaseDeleteService = null;
      await loadProject();
    } catch (cause) {
      databaseDeleteError = cause instanceof Error ? cause.message : 'Could not remove database';
    } finally {
      databaseDeleteBusy = false;
    }
  }

  async function writeClipboard(value) {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return;
    }
    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    textarea.remove();
  }

  async function copyValue(field, value) {
    await writeClipboard(value);
    copiedField = field;
    setTimeout(() => { if (copiedField === field) copiedField = ''; }, 1600);
  }

  async function copyVisibleLogs() {
    if (visibleLogs.length === 0) return;
    const output = visibleLogs.map((entry) => `${entry.timestamp ? entry.timestamp + ' ' : ''}[${entry.severity.toUpperCase()}] ${entry.message}`).join('\n');
    await writeClipboard(output);
    logsCopied = true;
    clearTimeout(logsCopyTimer);
    logsCopyTimer = setTimeout(() => logsCopied = false, 1600);
  }

  const duration = (seconds) => Number(seconds) > 0 ? `${seconds}s` : '—';

  function formatBytes(value = 0) {
    if (!Number.isFinite(value) || value <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    const amount = value / Math.pow(1024, index);
    return `${amount >= 100 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
  }

  function percent(value = 0) {
    return `${Math.max(0, value).toFixed(value >= 10 ? 1 : 2)}%`;
  }

  function metricWidth(value = 0, maximum = 100) {
    return `${Math.max(2, Math.min(100, value / maximum * 100))}%`;
  }
</script>

<Shell eyebrow="Projects" title={project.name}>
  {#if error}<div class="feedback error"><strong>Action failed</strong><span>{error}</span><button onclick={() => error = ''}>×</button></div>{/if}
  {#if notice}<div class="feedback success"><strong>Action complete</strong><span>{notice}</span><button onclick={() => notice = ''}>×</button></div>{/if}

  <section class="project-hero">
    <div>
      <Status value={project.status} />
      <h2>{project.domain || (displayApplicationServices.length ? `${displayApplicationServices.length} application service${displayApplicationServices.length === 1 ? '' : 's'}` : 'Add your first service')}</h2>
      <p>{source || 'Empty project · services are added independently'}{#if project.sourceType === 'repository'} · {project.branch}{/if}</p>
      {#if service && domainURL}<a class="endpoint" href={domainURL} target="_blank" rel="noreferrer">Open {domainURL} ↗</a>{/if}
    </div>
    <div class="hero-actions">
      <button onclick={() => selectTab('deployments')}>View activity</button>
      {#if project.sourceType !== 'empty'}<button class="deploy" onclick={deploy} disabled={deploying || loading || project.sourceType !== 'image'}><Icon name="rocket" size={14}/>{deploying ? 'Deploying…' : 'Deploy main'}</button>{:else}<button class="deploy" onclick={openServiceModal}><Icon name="plus" size={14}/> Add first service</button>{/if}
    </div>
  </section>

  <nav class="tabs" aria-label="Project sections">
    {#each tabs as tab}
      <button class:active={activeTab === tab} onclick={() => selectTab(tab)}>{tab[0].toUpperCase() + tab.slice(1)}</button>
    {/each}
  </nav>

  {#if loading}
    <section class="state"><span class="spinner"></span><div><h3>Loading project</h3><p>Reading service and deployment state.</p></div></section>
  {:else if activeTab === 'overview'}
    <div class="overview-grid">
      <section class="panel services">
        <header><div><span>Runtime</span><h3>Services</h3></div><div class="service-head-actions"><b>{displayApplicationServices.length + databaseServices.length}</b><button class="compose-add" onclick={openComposeModal}><Icon name="file-text" size={14}/> Add Compose</button><button class="service-add-primary" onclick={openServiceModal}><Icon name="plus" size={14}/> Add service</button><button onclick={openDatabaseModal}><Icon name="database" size={14}/> Add database</button></div></header>
        {#if displayApplicationServices.length === 0 && databaseServices.length === 0}
          <div class="empty"><div class="empty-icon"><Icon name="box" size={22} /></div><div><h4>No services yet</h4><p>Add a containerized frontend, API, admin tool, or database. Every service stays private until it receives a domain route.</p></div></div>
        {/if}
        {#each displayApplicationServices as item}
          <article class="application-service-row">
            <span class="service-icon application"><Icon name="box" size={18} /></span>
            <div><strong>{item.name}</strong><small>{item.sourceType === 'repository' ? `${item.repository}@${item.branch}` : (item.imageUrl || 'No image configured')} · :{item.containerPort}{item.command ? ` · command ${item.command}` : ''} · {item.container || 'container not created'}</small></div>
            <Status value={item.status} />
            <div class="application-service-actions">
              {#if !item.legacy && $currentUser && $currentUser.role !== 'viewer'}<button class="terminal-action" title={'Open terminal for ' + item.name} onclick={(event) => openServiceTerminal(item, event.currentTarget)} disabled={!item.container || item.status === 'stopped' || item.status === 'deploying'}><Icon name="terminal" size={13}/> Terminal</button>{/if}
              {#if item.status !== 'stopped'}<button class="lifecycle-stop" onclick={() => controlWorkload(item, 'application', 'stop')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name="stop" size={12}/>{workloadActionBusy(item, 'application', 'stop') ? 'Stopping…' : 'Stop'}</button>{/if}
              <button class="lifecycle-restart" onclick={() => controlWorkload(item, 'application', 'restart')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name={item.status === 'stopped' ? 'play' : 'refresh'} size={13}/>{workloadActionBusy(item, 'application', 'restart') ? (item.status === 'stopped' ? 'Starting…' : 'Restarting…') : (item.status === 'stopped' ? 'Start' : 'Restart')}</button>
              <button onclick={() => item.legacy ? deploy() : deployApplicationService(item)} disabled={item.status === 'deploying' || lifecycleBusy !== '' || item.legacy && project.sourceType !== 'image'}><Icon name="rocket" size={13}/>{item.status === 'deploying' ? 'Pulling…' : 'Deploy'}</button>
              {#if !item.legacy}<button class="danger-text icon-only" title="Remove service" aria-label={'Remove ' + item.name} onclick={() => openApplicationDelete(item)}><Icon name="trash" size={14}/></button>{/if}
            </div>
          </article>
        {/each}
        {#each databaseServices as item}
          <article class="database-row">
            <span class="service-icon database"><Icon name="database" size={18} /></span>
            <div><strong>{item.name}</strong><small>{databasePresets[item.engine]?.label || item.engine} · {item.internalAddress}</small></div>
            <div class="database-state"><Status value={item.status} /><em class:public={item.publicEnabled}>{item.publicEnabled ? `Public · ${item.publicPort}` : 'Private'}</em></div>
            <div class="database-actions">
              <button onclick={() => showCredentials(item)}><Icon name="key" size={13}/> Credentials</button>
              {#if item.status !== 'stopped'}<button class="lifecycle-stop" onclick={() => controlWorkload(item, 'database', 'stop')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name="stop" size={12}/>{workloadActionBusy(item, 'database', 'stop') ? 'Stopping…' : 'Stop'}</button>{/if}
              <button class="lifecycle-restart" onclick={() => controlWorkload(item, 'database', 'restart')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name={item.status === 'stopped' ? 'play' : 'refresh'} size={13}/>{workloadActionBusy(item, 'database', 'restart') ? (item.status === 'stopped' ? 'Starting…' : 'Restarting…') : (item.status === 'stopped' ? 'Start' : 'Restart')}</button>
              {#if item.publicEnabled}<button class="danger-text" onclick={() => makePrivate(item)} disabled={exposureSaving}><Icon name="network" size={13}/> Private</button>{:else}<button onclick={() => openExposure(item)}><Icon name="network" size={13}/> Expose</button>{/if}
              <button class="danger-text icon-only" title="Remove database" aria-label={'Remove ' + item.name} onclick={() => openDatabaseDelete(item)}><Icon name="trash" size={14}/></button>
            </div>
          </article>
        {/each}
      </section>

      <section class="panel runtime-facts">
        <header><div><span>Container</span><h3>Runtime details</h3></div></header>
        <dl>
          <div><dt>Engine</dt><dd>Docker</dd></div>
          <div><dt>Network</dt><dd>selfhost-proxy</dd></div>
          <div><dt>Containers</dt><dd>{data.services.length + applicationServices.filter((item) => item.container).length} application</dd></div>
          <div><dt>Exposure</dt><dd>{project.domain ? 'Caddy ingress' : 'Internal only'}</dd></div>
        </dl>
      </section>
    </div>

    <section class="panel recent">
      <header><div><span>Delivery</span><h3>Recent deployments</h3></div><button onclick={() => selectTab('deployments')}>View all</button></header>
      {#if data.deployments.length === 0}<div class="compact-empty">No deployments recorded for this project.</div>{:else}
        {#each data.deployments.slice(0, 3) as item}
          <a href={'/deployments/' + item.id}><Status value={item.status} /><div><strong>{item.message}</strong><small>{item.serviceName || project.name} · {item.commit}</small></div><code>{duration(item.duration)}</code><time>{new Date(item.createdAt).toLocaleString()}</time><b>→</b></a>
        {/each}
      {/if}
    </section>
  {:else if activeTab === 'metrics'}
    <section class="project-metrics-head">
      <div><span>Project workloads</span><h3>Container metrics</h3><p>Only the application and database containers attached to this project are included.</p></div>
      <div class="metrics-freshness"><i class:spinning={metricsRefreshing}></i><span>{projectMetrics.checkedAt ? `Updated ${new Date(projectMetrics.checkedAt).toLocaleTimeString()}` : 'Waiting for first sample'}</span><button onclick={() => loadProjectMetrics()} disabled={metricsRefreshing}>Refresh</button></div>
    </section>

    {#if metricsError}<div class="metrics-feedback"><strong>Metrics unavailable</strong><span>{metricsError}</span><button onclick={() => loadProjectMetrics()}>Retry</button></div>{/if}
    {#if metricsLoading && !projectMetrics.checkedAt}
      <section class="metrics-loading"><span class="spinner"></span><div><h3>Loading project metrics</h3><p>Reading the latest background sample.</p></div></section>
    {:else}
      <section class="project-metric-grid">
        <article><span>Containers</span><strong>{projectMetrics.global.running || 0}<small> / {projectMetrics.global.containers || 0}</small></strong><p>running in this project</p></article>
        <article><span>CPU</span><strong>{percent(projectMetrics.global.cpuPercent)}</strong><div class="project-meter"><i style={'width:' + metricWidth(projectMetrics.global.cpuPercent, Math.max(100, (projectMetrics.global.cpuCores || 1) * 100))}></i></div><p>combined current sample</p></article>
        <article><span>Memory</span><strong>{formatBytes(projectMetrics.global.memoryUsage)}</strong><div class="project-meter"><i style={'width:' + metricWidth(projectMetrics.global.memoryPercent)}></i></div><p>{percent(projectMetrics.global.memoryPercent)} of host RAM</p></article>
        <article><span>Disk I/O</span><strong>{formatBytes(projectMetrics.global.diskIo?.read)}</strong><p>R {formatBytes(projectMetrics.global.diskIo?.read)} · W {formatBytes(projectMetrics.global.diskIo?.write)}</p></article>
        <article><span>Network I/O</span><strong>{formatBytes(projectMetrics.global.networkIo?.receive)}</strong><p>↓ {formatBytes(projectMetrics.global.networkIo?.receive)} · ↑ {formatBytes(projectMetrics.global.networkIo?.transmit)}</p></article>
      </section>

      <section class="panel project-workloads">
        <header><div><span>Live workload sample</span><h3>Project containers</h3></div><b>{projectMetrics.containers.length}</b></header>
        {#if projectMetrics.containers.length === 0}
          <div class="empty"><div class="empty-icon"><Icon name="activity" size={22} /></div><div><h4>No project containers found</h4><p>Deploy the application or add a database. Metrics appear here when Docker creates a container with this project’s label.</p></div></div>
        {:else}
          <div class="workload-columns" aria-hidden="true"><span>Container</span><span>CPU</span><span>Memory</span><span>Disk I/O</span><span>Writable</span><span>Network I/O</span></div>
          <div class="workload-list">
            {#each projectMetrics.containers as container}
              <article class="workload-row">
                <div class="workload-name"><i class:stopped={container.state !== 'running'}></i><span><strong>{container.name}</strong><small>{container.serviceKind === 'database' ? 'Database' : 'Application'} · {container.image}</small></span></div>
                <div class="workload-value"><strong>{percent(container.cpuPercent)}</strong><i><u style={'width:' + metricWidth(container.cpuPercent, Math.max(100, (projectMetrics.global.cpuCores || 1) * 100))}></u></i></div>
                <div class="workload-value"><strong>{formatBytes(container.memoryUsage)}</strong><i><u style={'width:' + metricWidth(container.memoryPercent)}></u></i><small>{percent(container.memoryPercent)} of limit</small></div>
                <div class="workload-pair"><span>R {formatBytes(container.diskIo?.read)}</span><span>W {formatBytes(container.diskIo?.write)}</span></div>
                <div class="workload-pair"><span>RW {formatBytes(container.disk?.writable)}</span><span>FS {formatBytes(container.disk?.rootFs)}</span></div>
                <div class="workload-pair"><span>↓ {formatBytes(container.networkIo?.receive)}</span><span>↑ {formatBytes(container.networkIo?.transmit)}</span></div>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  {:else if activeTab === 'databases'}
    <section class="panel database-manager">
      <header><div><span>Persistent services</span><h3>Databases</h3></div><button class="deploy-small" onclick={openDatabaseModal}><Icon name="plus" size={14}/> Add database</button></header>
      {#if databaseServices.length === 0}
        <div class="empty"><div class="empty-icon"><Icon name="database" size={22} /></div><div><h4>No databases deployed</h4><p>Add PostgreSQL, MySQL, or MariaDB with private networking and persistent storage.</p></div></div>
      {:else}
        <div class="database-manager-list">
          {#each databaseServices as item}
            <article class="database-manager-card">
              <div class="database-card-heading"><span class="service-icon database"><Icon name="database" size={18} /></span><div><strong>{item.name}</strong><small>{databasePresets[item.engine]?.label || item.engine} · {item.image}</small></div><Status value={item.status} /></div>
              <dl>
                <div><dt>Internal address</dt><dd><code>{item.internalAddress}</code></dd></div>
                <div><dt>Container</dt><dd><code>{item.container}</code></dd></div>
                <div><dt>Network access</dt><dd><span class:public={item.publicEnabled}>{item.publicEnabled ? `Public on ${item.publicPort}` : 'Private only'}</span></dd></div>
                <div><dt>Persistent volume</dt><dd><code>{item.volumeName}</code></dd></div>
              </dl>
              <div class="database-card-actions">
                <button onclick={() => openWorkloadLogs(item, 'database', 'runtime')}><Icon name="activity" size={14} /> Runtime logs</button>
                <button onclick={() => openWorkloadLogs(item, 'database', 'deployment')}><Icon name="rocket" size={14} /> Deployment logs</button>
                {#if item.status !== 'stopped'}<button class="lifecycle-stop" onclick={() => controlWorkload(item, 'database', 'stop')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name="stop" size={12}/>{workloadActionBusy(item, 'database', 'stop') ? 'Stopping…' : 'Stop'}</button>{/if}
                <button class="lifecycle-restart" onclick={() => controlWorkload(item, 'database', 'restart')} disabled={!item.container || item.status === 'deploying' || lifecycleBusy !== ''}><Icon name={item.status === 'stopped' ? 'play' : 'refresh'} size={13}/>{workloadActionBusy(item, 'database', 'restart') ? (item.status === 'stopped' ? 'Starting…' : 'Restarting…') : (item.status === 'stopped' ? 'Start' : 'Restart')}</button>
                <button onclick={() => showCredentials(item)}><Icon name="key" size={14}/> Credentials</button>
                {#if item.publicEnabled}<button onclick={() => makePrivate(item)} disabled={exposureSaving}><Icon name="network" size={14}/> Make private</button>{:else}<button onclick={() => openExposure(item)}><Icon name="network" size={14}/> Networking</button>{/if}
                <button class="delete-database" onclick={() => openDatabaseDelete(item)}><Icon name="trash" size={14}/> Delete database</button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  {:else if activeTab === 'deployments'}
    <section class="panel deployment-panel">
      <header><div><span>Delivery</span><h3>All service deployments</h3></div><button class="deploy-small" onclick={openServiceModal}><Icon name="plus" size={14}/> Add service</button></header>
      <div class="deployment-service-tabs" aria-label="Filter deployments by service"><button class:active={deploymentFilter === 'all'} onclick={() => deploymentFilter = 'all'}>All <span>{data.deployments.length}</span></button>{#each displayApplicationServices as item}<button class:active={deploymentFilter === item.id} onclick={() => deploymentFilter = item.id}>{item.name}<span>{data.deployments.filter((deployment) => (deployment.serviceId || 'main') === item.id).length}</span></button>{/each}</div>
      {#if filteredDeployments.length === 0}<div class="empty"><div class="empty-icon"><Icon name="rocket" size={22} /></div><div><h4>No deployments for this selection</h4><p>Deploy any application service and its pull, create, start, and verification events will appear here.</p></div></div>{:else}
        {#each filteredDeployments as item}
          <a class="deployment-row" href={'/deployments/' + item.id}><Status value={item.status} /><div><strong>{item.message}</strong><small>{item.serviceName || project.name} · {item.commit}</small></div><code>{duration(item.duration)}</code><time>{new Date(item.createdAt).toLocaleString()}</time><b>→</b></a>
        {/each}
      {/if}
    </section>
  {:else if activeTab === 'logs'}
    <section class="panel log-panel">
      <header>
        <div><span>{activeLogTarget?.kind === 'database' ? 'Database observability' : 'Container output'}</span><h3>{activeLogTarget ? `${activeLogTarget.name} logs` : 'Runtime logs'}</h3></div>
        <div class="log-actions">
          {#if logsUpdated}<small>Updated {logsUpdated}</small>{/if}
          {#if logView === 'runtime'}
            <label class="line-limit">
              <span>Lines</span>
              <select bind:value={logLimit} onchange={changeLogLimit} aria-label="Number of log lines">
                <option value={100}>100</option>
                <option value={300}>300</option>
                <option value={500}>500</option>
                <option value={1000}>1,000</option>
              </select>
            </label>
          {/if}
          <button class="live-toggle" class:live={logsLive} onclick={toggleLiveLogs} aria-pressed={logsLive}>
            <i></i>{logsLive ? 'Live · Pause' : 'Paused · Resume'}
          </button>
          <button class:copied={logsCopied} onclick={copyVisibleLogs} disabled={visibleLogs.length === 0} aria-live="polite">{logsCopied ? 'Copied ✓' : 'Copy output'}</button>
          <button onclick={loadLogs} disabled={logsLoading}>{logsLoading ? 'Refreshing…' : 'Refresh'}</button>
        </div>
      </header>
      {#if logTargets.length > 0}
        <div class="log-source-strip" aria-label="Choose log source">
          {#each logTargets as item}
            <button class:active={activeLogTarget?.key === item.key} onclick={() => selectLogTarget(item.key)}>
              <span class:database={item.kind === 'database'} class="log-source-icon"><Icon name={item.kind === 'database' ? 'database' : 'box'} size={14}/></span>
              <span><strong>{item.name}</strong><small>{item.kind === 'database' ? (databasePresets[item.engine]?.label || item.engine) : (item.legacy ? 'Main application' : `Application · :${item.containerPort}`)}</small></span>
              <Status value={item.status}/>
            </button>
          {/each}
        </div>
        <div class="log-toolbar">
          <div class="log-filter-group">
            {#if activeLogTarget?.kind === 'database'}
              <div class="log-view-tabs" aria-label="Choose database log type">
                <button class:active={logView === 'runtime'} onclick={() => selectLogView('runtime')}>Runtime</button>
                <button class:active={logView === 'deployment'} onclick={() => selectLogView('deployment')}>Deployment</button>
              </div>
            {/if}
            <div class="severity-filters" aria-label="Filter logs by severity">
              <button class:active={logLevel === 'all'} onclick={() => logLevel = 'all'}>All <span>{parsedLogs.length}</span></button>
              <button class="debug" class:active={logLevel === 'debug'} onclick={() => logLevel = 'debug'}>Debug <span>{logCounts.debug}</span></button>
              <button class="info" class:active={logLevel === 'info'} onclick={() => logLevel = 'info'}>Info <span>{logCounts.info}</span></button>
              <button class="warning" class:active={logLevel === 'warning'} onclick={() => logLevel = 'warning'}>Warning <span>{logCounts.warning}</span></button>
              <button class="error" class:active={logLevel === 'error'} onclick={() => logLevel = 'error'}>Error <span>{logCounts.error}</span></button>
            </div>
          </div>
          <label><span class="sr-only">Search logs</span><input bind:value={logQuery} type="search" placeholder="Search log output" /></label>
        </div>
      {/if}
      {#if !activeLogTarget}
        <div class="log-state"><div class="empty-icon"><Icon name="logs" size={22}/></div><div><h4>No workloads to inspect</h4><p>Add an application service or database and its logs will appear here.</p></div></div>
      {:else if logsLoading && parsedLogs.length === 0}
        <div class="log-state"><span class="spinner"></span><div><h4>Reading container logs</h4><p>Loading the latest output from Docker.</p></div></div>
      {:else if logsError}
        <div class="log-state"><div class="empty-icon">!</div><div><h4>Logs unavailable</h4><p>{logsError}</p></div></div>
      {:else if parsedLogs.length === 0}
        <div class="log-state"><div class="empty-icon">LOG</div><div><h4>{logView === 'deployment' ? 'No deployment events yet' : 'No output yet'}</h4><p>{logView === 'deployment' ? 'Deployment events will appear after the next database apply.' : 'The container has not written anything to stdout or stderr.'}</p></div></div>
      {:else}
        <div class="terminal-head"><span></span><strong>{logView === 'deployment' ? `${activeLogTarget.name} deployment activity` : (activeLogTarget.container || activeLogTarget.name)}</strong><small>{parsedLogs.length} {logView === 'deployment' ? 'events' : 'lines'}</small></div>
        {#if visibleLogs.length === 0}
          <div class="filtered-empty"><strong>No matching log lines</strong><span>Change the severity filter or search query.</span></div>
        {:else}
          <div class="log-console" aria-label="Container logs" bind:this={logConsole}>
            {#each visibleLogs as entry}
              <div class="log-line {entry.severity}">
                <span class="line-number">{entry.index}</span>
                <time datetime={entry.timestamp} title={entry.timestamp}>{entry.time}</time>
                <span class="severity">{entry.severity}</span>
                <code>{entry.message}</code>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </section>
  {:else if activeTab === 'environment'}
    <section class="panel environment-panel">
      <header>
        <div><span>Runtime configuration</span><h3>Service environment</h3></div>
        <div class="environment-header-actions"><button class="bulk-variable" type="button" onclick={openBulkEnvironment} disabled={!activeEnvironmentTarget}><Icon name="logs" size={13}/> Bulk edit</button><button class="add-variable" type="button" onclick={addEnvironmentVariable} disabled={!activeEnvironmentTarget}><Icon name="plus" size={13}/> Add variable</button></div>
      </header>
      <div class="environment-service-tabs" aria-label="Choose application service">{#each environmentTargets as item}<button class:active={activeEnvironmentTarget?.id === item.id} onclick={() => selectEnvironmentTarget(item.id)}><span class="service-tab-icon"><Icon name="box" size={14}/></span><span><strong>{item.name}</strong><small>{item.legacy ? 'Legacy main service' : `:${item.containerPort}`}</small></span><Status value={item.status}/></button>{/each}</div>
      {#if !activeEnvironmentTarget}
        <div class="empty"><div class="empty-icon"><Icon name="settings" size={22}/></div><div><h4>No application services</h4><p>Add a service first. Its isolated environment editor will appear here automatically.</p><button class="deploy-small" onclick={openServiceModal}><Icon name="plus" size={14}/> Add service</button></div></div>
      {:else}
      {#if environmentError}<div class="environment-feedback error"><strong>Variables not applied</strong><span>{environmentError}</span></div>{/if}
      {#if environmentNotice}<div class="environment-feedback success" aria-live="polite"><strong>Runtime updated</strong><span>{environmentNotice}</span></div>{/if}
      {#if environmentLoading}
        <div class="environment-loading"><span class="spinner"></span><span>Reading runtime configuration…</span></div>
      {:else}
        <form class="environment-editor" onsubmit={(event) => { event.preventDefault(); saveEnvironment(); }}>
          <div class="environment-columns" aria-hidden="true"><span>Key</span><span>Value</span><span>Secret</span><span></span></div>
          <div class="variable-list">
            {#each environmentVariables as variable, index}
              <div class="variable-row">
                <label class="variable-key-field"><span class="sr-only">Variable key</span><input type="text" bind:value={variable.key} title={variable.key || 'Variable key'} placeholder="APP_ENV" autocomplete="off" spellcheck="false" /></label>
                <label class="value-field"><span class="sr-only">Variable value</span><input type={variable.secret && !variable.revealed ? 'password' : 'text'} bind:value={variable.value} placeholder="production" autocomplete="off" spellcheck="false" />{#if variable.secret}<button type="button" onclick={() => toggleEnvironmentValue(index)} aria-label={variable.revealed ? 'Hide value' : 'Reveal value'}>{variable.revealed ? 'Hide' : 'Show'}</button>{/if}</label>
                <label class="secret-toggle"><input type="checkbox" bind:checked={variable.secret} /><span>Mask</span></label>
                <button class="remove-variable" type="button" onclick={() => removeEnvironmentVariable(index)} aria-label={'Remove ' + (variable.key || 'variable')}>×</button>
              </div>
            {/each}
          </div>
          <footer>
            <div><strong>{environmentVariables.filter((variable) => variable.key.trim()).length} configured</strong><span>Values marked as secrets are encrypted in PostgreSQL.</span></div>
            <button type="submit" disabled={environmentSaving || environmentLoading}><Icon name="refresh" size={14}/>{environmentSaving ? `Restarting ${activeEnvironmentTarget.name}…` : 'Save & restart'}</button>
          </footer>
        </form>
      {/if}
      {/if}
    </section>
  {:else if activeTab === 'domains'}
    <section class="panel domain-panel">
      <header><div><span>Traffic</span><h3>Domain routing</h3></div><div class="domain-header-actions">{#if domainBindings.length}<span class="route-state"><i></i> {domainBindings.length} active</span>{/if}<button class="add-domain" type="button" onclick={() => openDomainModal()}>＋ Add domain</button></div></header>
      <div class="domain-layout">
        <div class="domain-form">
          <div class="domain-editor-head"><div class="form-copy"><h4>Domains, paths, and services</h4><p>Review every hostname and its routing rules. Add or edit a domain in its own focused workspace.</p></div></div>
          {#if domainError}<div class="domain-feedback error"><strong>Route not changed</strong><span>{domainError}</span></div>{/if}
          {#if domainNotice}<div class="domain-feedback success"><strong>Route updated</strong><span>{domainNotice}</span></div>{/if}
          {#if domainBindings.length === 0}
            <button class="domain-empty" type="button" onclick={() => openDomainModal()}><span>＋</span><strong>Add your first domain</strong><small>Then choose which paths and container ports it should serve.</small></button>
          {:else}
            <div class="domain-binding-list">
              {#each domainBindings as binding, bindingIndex}
                <section class="domain-binding">
                  <div class="domain-list-row">
                    <div class="domain-list-copy"><strong>{binding.domain || 'Untitled domain'}</strong><span>{binding.rules.length} path{binding.rules.length === 1 ? '' : 's'} · {binding.rules.map((rule) => `${rule.path || '/*'} → ${routeTargetName(rule.serviceId)}:${rule.port || '—'}`).join(' · ')}</span></div>
                    {#if binding.httpsEnabled}<span class="https-badge">HTTPS</span>{/if}
                    {#if binding.domain}<a href={domainEndpoint(binding)} target="_blank" rel="noreferrer">Open ↗</a>{/if}
                    <button type="button" onclick={() => openDomainModal(bindingIndex)}>Edit</button>
                  </div>
                </section>
              {/each}
            </div>
          {/if}
        </div>
        <aside class="route-guide">
          <span class="guide-label">How traffic reaches it</span>
          <ol>
            <li><b>1</b><div><strong>Point every domain</strong><p>Create an A record to your VPS IP. For local domains, point each name to <code>127.0.0.1</code>.</p></div></li>
            <li><b>2</b><div><strong>Match by host and path</strong><p>Caddy selects the hostname first, then evaluates its path rules in the order shown.</p></div></li>
            <li><b>3</b><div><strong>Forward privately</strong><p>Every match reaches the service selected in that rule on <code>selfhost-proxy</code>. No random public app port is needed.</p></div></li>
          </ol>
          <div class="routing-example"><span>Example</span><code>domain.local/api/** → :8080</code><code>domain.local/static/** → :8080</code><code>domain2.local/api/** → :8080</code></div>
        </aside>
      </div>
    </section>
  {:else if activeTab === 'settings'}
    <div class="settings-stack">
      <section class="panel project-editor">
        <header><div><span>Project</span><h3>Project details</h3></div><code>{project.id}</code></header>
        <form onsubmit={(event) => { event.preventDefault(); saveProjectSettings(); }}>
          {#if settingsError}<div class="domain-feedback error"><strong>Project not updated</strong><span>{settingsError}</span></div>{/if}
          {#if settingsNotice}<div class="domain-feedback success"><strong>Project updated</strong><span>{settingsNotice}</span></div>{/if}
          <div class="project-identity-layout">
            <label class="settings-field"><span>Project name</span><input bind:value={settingsForm.name} required maxlength="100" /></label>
            <div class="project-boundary-note">
              <span><Icon name="layers" size={17}/></span>
              <div><strong>Shared service boundary</strong><p>{applicationServices.length} application service{applicationServices.length === 1 ? '' : 's'} · {databaseServices.length} database{databaseServices.length === 1 ? '' : 's'} · one private Docker network</p></div>
            </div>
          </div>
          <footer><span>Last updated {new Date(project.updatedAt).toLocaleString()}</span><button class="save-settings" type="submit" disabled={settingsSaving}>{settingsSaving ? 'Saving changes…' : 'Save changes'}</button></footer>
        </form>
      </section>

      <section class="panel runtime-settings-panel">
        <header>
          <div><span>Runtime definitions</span><h3>Application services</h3></div>
          <p>Edit the image, internal port, registry, and command used to create each service container.</p>
        </header>
        {#if applicationServices.length === 0}
          <div class="runtime-settings-empty"><div class="empty-icon"><Icon name="settings" size={21}/></div><div><h4>No independently managed services</h4><p>Add an application service from Overview. Its runtime definition will appear here.</p></div><button type="button" onclick={() => { selectTab('overview'); openServiceModal(); }}><Icon name="plus" size={14}/> Add service</button></div>
        {:else}
          <div class="runtime-service-tabs" role="tablist" aria-label="Application service settings">
            {#each applicationServices as item}
              <button type="button" role="tab" aria-selected={runtimeSettingsService?.id === item.id} class:active={runtimeSettingsService?.id === item.id} onclick={() => selectRuntimeSettingsService(item)}>
                <span class="service-tab-icon"><Icon name="box" size={14}/></span>
                <span><strong>{item.name}</strong><small>{item.sourceType === 'repository' ? `${item.repository}@${item.branch}` : item.imageUrl} · :{item.containerPort}</small></span>
                <Status value={item.status}/>
              </button>
            {/each}
          </div>
          {#if runtimeSettingsService}
            <form class="runtime-settings-form" onsubmit={(event) => { event.preventDefault(); saveRuntimeSettings(false); }}>
              <div class="runtime-settings-copy"><div><span>Service definition</span><h4>{runtimeSettingsService.name}</h4></div><code>{runtimeSettingsService.id}</code></div>
              {#if runtimeSettingsError}<div class="domain-feedback error"><strong>Service not updated</strong><span>{runtimeSettingsError}</span></div>{/if}
              {#if runtimeSettingsNotice}<div class="domain-feedback success"><strong>Runtime definition saved</strong><span>{runtimeSettingsNotice}</span></div>{/if}
              <div class="settings-grid runtime-settings-grid">
                <label class="settings-field"><span>Service name</span><input bind:value={runtimeSettingsForm.name} required maxlength="63" /></label>
                <label class="settings-field"><span>Container port</span><input bind:value={runtimeSettingsForm.containerPort} type="number" min="1" max="65535" required /></label>
                {#if runtimeSettingsForm.sourceType === 'repository'}
                  <label class="settings-field"><span>Git account</span><select bind:value={runtimeSettingsForm.connectionId} required>{#each integrations.connections || [] as connection}<option value={connection.id}>{connection.provider} · {connection.accountName}</option>{/each}</select></label>
                  <label class="settings-field"><span>Repository</span><input bind:value={runtimeSettingsForm.repository} required spellcheck="false" /></label>
                  <label class="settings-field"><span>Branch</span><input bind:value={runtimeSettingsForm.branch} required spellcheck="false" /></label>
                  <label class="settings-field"><span>Build strategy</span><select bind:value={runtimeSettingsForm.buildStrategy}><option value="dockerfile">Dockerfile</option><option value="railpack">Railpack</option><option value="nixpacks">Nixpacks</option></select></label>
                  {#if runtimeSettingsForm.buildStrategy === 'dockerfile'}
                    <label class="settings-field"><span>Dockerfile path</span><input bind:value={runtimeSettingsForm.dockerfilePath} required spellcheck="false" /></label>
                    <label class="settings-field"><span>Build context</span><input bind:value={runtimeSettingsForm.buildContext} required spellcheck="false" /></label>
                  {:else}
                    <div class="settings-field build-pack-help"><strong>{runtimeSettingsForm.buildStrategy === 'railpack' ? 'Railpack' : 'Nixpacks'} will detect the language and build plan.</strong><small>No Dockerfile is required. Commit <code>railpack.json</code> or <code>nixpacks.toml</code> to customize the build.</small></div>
                  {/if}
                {:else}
                  <label class="settings-field"><span>Registry credential <em>optional</em></span><select bind:value={runtimeSettingsForm.registryId}><option value="">Docker Hub / public registry</option>{#each integrations.registries || [] as registry}<option value={registry.id}>{registry.name} · {registry.registryUrl}</option>{/each}</select></label>
                  <label class="settings-field"><span>Container image</span><input bind:value={runtimeSettingsForm.imageUrl} required spellcheck="false" placeholder="quay.io/keycloak/keycloak:26.7.0" /></label>
                {/if}
                <label class="settings-field wide runtime-command-field"><span>Container command <em>optional</em></span><input bind:value={runtimeSettingsForm.command} maxlength="4096" placeholder="start-dev" spellcheck="false" /><small>Arguments passed to the image ENTRYPOINT. For Keycloak use <code>start-dev</code>. Quoted arguments are supported.</small></label>
                <label class="settings-field"><span>Health verification</span><select bind:value={runtimeSettingsForm.healthCheckType}><option value="none">Container running</option><option value="http">HTTP path</option><option value="command">Command</option></select></label>
                <label class="settings-field"><span>Health timeout</span><input bind:value={runtimeSettingsForm.healthCheckTimeoutSeconds} type="number" min="5" max="600" required /><small>Seconds to wait before deployment fails.</small></label>
                {#if runtimeSettingsForm.healthCheckType === 'http'}
                  <label class="settings-field wide"><span>Health path</span><input bind:value={runtimeSettingsForm.healthCheckPath} required maxlength="2048" placeholder="/health" spellcheck="false" /><small>Requested on private port <code>{runtimeSettingsForm.containerPort}</code> until it returns HTTP 2xx or 3xx.</small></label>
                {:else if runtimeSettingsForm.healthCheckType === 'command'}
                  <label class="settings-field wide runtime-command-field"><span>Health command</span><input bind:value={runtimeSettingsForm.healthCheckCommand} required maxlength="4096" placeholder="wget -qO- http://127.0.0.1:8080/health >/dev/null" spellcheck="false" /><small>Executed inside the container with <code>/bin/sh -c</code>. Exit code 0 means healthy.</small></label>
                {:else}
                  <div class="settings-field wide health-check-note"><strong>Container-state check</strong><small>The deployment succeeds when Docker confirms that the main process is running.</small></div>
                {/if}
              </div>
              <div class="runtime-apply-note"><Icon name="settings" size={16}/><div><strong>Saving does not interrupt the running container.</strong><span>Choose Save & deploy to recreate this service immediately with the updated command. Docker reuses cached image layers when available.</span></div></div>
              <section class="deployment-triggers" aria-labelledby="deployment-triggers-title">
                <header><div><span>Automation</span><h4 id="deployment-triggers-title">Deployment triggers</h4></div><b>{runtimeSettingsForm.sourceType === 'repository' ? 'Git push' : 'Registry webhook'}</b></header>
                {#if runtimeTriggersError}<div class="domain-feedback error"><strong>Triggers unavailable</strong><span>{runtimeTriggersError}</span></div>{/if}
                {#if runtimeTriggersNotice}<div class="domain-feedback success"><strong>Automation updated</strong><span>{runtimeTriggersNotice}</span></div>{/if}
                {#if runtimeTriggersLoading}
                  <div class="trigger-loading"><span class="spinner"></span>Loading deployment triggers…</div>
                {:else if runtimeSettingsForm.sourceType === 'repository'}
                  <div class="trigger-row">
                    <span class="trigger-icon"><Icon name="git" size={17}/></span>
                    <div><strong>Deploy pushes to <code>{runtimeSettingsForm.branch}</code></strong><small>A verified GitHub push rebuilds this service. Duplicate webhook deliveries are ignored.</small></div>
                    <label class="switch"><input type="checkbox" bind:checked={runtimeTriggers.autoDeploy}/><span></span><em>{runtimeTriggers.autoDeploy ? 'On' : 'Off'}</em></label>
                  </div>
                  <div class:webhook-warning={!runtimeTriggers.webhookConfigured} class="webhook-endpoint"><div><small>GitHub App webhook</small><code>{runtimeTriggers.webhookUrl || 'Reconnect the GitHub App in Sources to register its signed push webhook'}</code></div>{#if runtimeTriggers.webhookUrl}<button type="button" onclick={copyWebhookUrl}>{copiedField === 'webhook-url' ? 'Copied' : 'Copy URL'}</button>{:else}<a href="/integrations">Open Sources</a>{/if}</div>
                {:else}
                  <div class="trigger-row">
                    <span class="trigger-icon"><Icon name="box" size={17}/></span>
                    <div><strong>Deploy after an image push</strong><small>Docker Hub, GHCR, GitLab Registry, and compatible registry webhooks can call the generated URL.</small></div>
                    <label class="switch"><input type="checkbox" bind:checked={runtimeTriggers.registryWebhookEnabled}/><span></span><em>{runtimeTriggers.registryWebhookEnabled ? 'On' : 'Off'}</em></label>
                  </div>
                  {#if runtimeTriggers.registryWebhookEnabled}
                    <div class="registry-trigger-config">
                      <label class="settings-field"><span>Watched tag</span><input bind:value={runtimeTriggers.registryWebhookTag} maxlength="128" placeholder="latest" spellcheck="false"/><small>Only a push containing this tag starts deployment. Leave empty to accept any pushed tag.</small></label>
                      <div class="webhook-endpoint"><div><small>Registry webhook URL</small><code>{runtimeTriggers.webhookUrl || 'Save triggers to generate the private URL'}</code></div>{#if runtimeTriggers.webhookUrl}<button type="button" onclick={copyWebhookUrl}>{copiedField === 'webhook-url' ? 'Copied' : 'Copy URL'}</button>{/if}</div>
                    </div>
                  {/if}
                {/if}
                <footer><span>{runtimeSettingsForm.sourceType === 'repository' ? (runtimeTriggers.webhookConfigured ? 'Signed with the GitHub App webhook secret' : 'One-time GitHub App reconnect required') : 'The URL contains a private service token'}</span><button type="button" onclick={saveDeploymentTriggers} disabled={runtimeTriggersSaving || runtimeTriggersLoading || (runtimeSettingsForm.sourceType === 'repository' && runtimeTriggers.autoDeploy && !runtimeTriggers.webhookConfigured)}>{runtimeTriggersSaving ? 'Saving triggers…' : 'Save triggers'}</button></footer>
              </section>
              <footer><span>Environment variables remain managed in the Environment tab.</span><div><button type="submit" class="secondary-runtime" disabled={runtimeSettingsBusy !== ''}>{runtimeSettingsBusy === 'save' ? 'Saving…' : 'Save definition'}</button><button type="button" class="save-settings" onclick={() => saveRuntimeSettings(true)} disabled={runtimeSettingsBusy !== ''}><Icon name="play" size={13}/>{runtimeSettingsBusy === 'deploy' ? 'Starting deployment…' : 'Save & deploy'}</button></div></footer>
            </form>
          {/if}
        {/if}
      </section>

      <section class="danger-zone">
        <div><span>Danger zone</span><h3>Delete this project</h3><p>Permanently remove its containers, databases, persistent volumes, deployment history, and domain route.</p></div>
        <button onclick={openDeleteModal}>Delete project</button>
      </section>
    </div>
  {/if}
</Shell>

{#if terminalService}
  <div class="modal-backdrop terminal-modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeServiceTerminal(); }}>
    <div class="modal service-terminal-modal" role="dialog" aria-modal="true" aria-labelledby="service-terminal-title">
      <header>
        <div><span>Container terminal</span><h2 id="service-terminal-title">{terminalService.name}</h2></div>
        <button aria-label="Close terminal" onclick={closeServiceTerminal} disabled={terminalRunning}>×</button>
      </header>
      <div class="terminal-modal-toolbar">
        <div><span><i></i> Connected</span><code>{terminalService.container}</code></div>
        <small>/bin/sh · 30 second response limit · runs as the container user</small>
        <label><span>Working directory</span><input bind:value={terminalWorkingDir} placeholder="Image default" disabled={terminalRunning} aria-label="Container working directory"/></label>
        <button type="button" onclick={clearTerminal} disabled={terminalRunning || terminalEntries.length === 0}>Clear output</button>
      </div>
      <div class="terminal-session">
        <div class="terminal-command-output" bind:this={terminalOutput} aria-live="polite">
          {#if terminalEntries.length === 0}
            <div class="terminal-welcome"><Icon name="terminal" size={22}/><strong>Ready for a command</strong><span>Run diagnostics, framework CLIs, or maintenance commands inside this service container. Output is limited to 2 MB.</span></div>
          {:else}
            {#each terminalEntries as entry}
              <article class="terminal-entry" class:failed={entry.status === 'error' || entry.exitCode > 0}>
                <div class="terminal-prompt"><span>{entry.workingDir || '~'} $</span><code>{entry.command}</code><time>{entry.startedAt}</time></div>
                {#if entry.status === 'running'}
                  <div class="terminal-running"><span class="spinner"></span> Executing…</div>
                {:else if entry.status === 'error'}
                  <pre class="terminal-stderr">{entry.error}</pre>
                {:else}
                  {#if entry.stdout}<pre>{entry.stdout}</pre>{/if}
                  {#if entry.stderr}<pre class="terminal-stderr">{entry.stderr}</pre>{/if}
                  {#if !entry.stdout && !entry.stderr}<span class="terminal-empty-output">Command completed without output.</span>{/if}
                  <footer><span class:failed={entry.exitCode > 0}>exit {entry.exitCode}</span><span>{entry.durationMs} ms</span>{#if entry.truncated}<strong>Output truncated at 2 MB</strong>{/if}</footer>
                {/if}
              </article>
            {/each}
          {/if}
        </div>
        <form class="terminal-command-form" onsubmit={(event) => { event.preventDefault(); runTerminalCommand(); }}>
          <span aria-hidden="true">$</span>
          <input bind:this={terminalInput} bind:value={terminalCommand} onkeydown={terminalKeydown} placeholder="Type a command, for example: php artisan about" autocomplete="off" spellcheck="false" disabled={terminalRunning} aria-label={'Command for ' + terminalService.name}/>
          <button type="submit" disabled={terminalRunning || !terminalCommand.trim()}>{terminalRunning ? 'Running…' : 'Run'} <Icon name="arrow-right" size={14}/></button>
        </form>
      </div>
    </div>
  </div>
{/if}

{#if domainModal && domainDraft}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeDomainModal(); }}>
    <div class="modal domain-editor-modal" role="dialog" aria-modal="true" aria-labelledby="domain-editor-title">
      <header><div><span>Traffic routing</span><h2 id="domain-editor-title">{domainEditingIndex === -1 ? 'Add domain' : 'Edit domain'}</h2></div><button aria-label="Close" onclick={closeDomainModal} disabled={domainSaving}>×</button></header>
      <form class="domain-editor-form" onsubmit={(event) => { event.preventDefault(); saveDomainDraft(); }}>
        <div class="domain-editor-body">
          <p>Send requests for this hostname to one or more application paths on the private Docker network.</p>
          {#if domainError}<div class="domain-feedback error"><strong>Route not changed</strong><span>{domainError}</span></div>{/if}
          <div class="domain-draft-head">
            <label class="binding-domain"><span>Domain name</span><input bind:value={domainDraft.domain} placeholder="domain.local" autocomplete="off" spellcheck="false" required /></label>
            <label class="binding-https"><input type="checkbox" bind:checked={domainDraft.httpsEnabled} /><span>HTTPS</span></label>
          </div>
          <div class="binding-rules-head"><div><strong>Path forwarding</strong><small>Unmatched requests return 404.</small></div><button type="button" onclick={addDomainRule}>＋ Add path</button></div>
          <div class="binding-rules">
            {#each domainDraft.rules as rule, ruleIndex}
              <div class="route-rule editable-rule">
                <input aria-label={'Path pattern for ' + (domainDraft.domain || 'domain')} bind:value={rule.path} placeholder={ruleIndex === 0 ? '/*' : '/api/**'} required />
                <span>→</span>
                <label class="service-target"><small>Service</small><select aria-label="Target application service" bind:value={rule.serviceId} onchange={(event) => setRuleService(ruleIndex, event.currentTarget.value)} required>{#if routeTargets.length === 0}<option value="">Add a service first</option>{/if}{#each routeTargets as item}<option value={item.id}>{item.name}{item.legacy ? ' (legacy)' : ''}</option>{/each}</select></label>
                <div class="port-input"><small>Container port</small><input aria-label="Internal container port" bind:value={rule.port} type="number" min="1" max="65535" required /></div>
                <button type="button" aria-label="Remove path" onclick={() => removeDomainRule(ruleIndex)} disabled={domainDraft.rules.length === 1}>×</button>
              </div>
            {/each}
          </div>
          <p class="route-hint">Use <code>/api/**</code> or <code>/static/**</code> for prefixes. Caddy preserves the complete request path when forwarding.</p>
        </div>
        <footer><button type="button" onclick={closeDomainModal} disabled={domainSaving}>Cancel</button>{#if domainEditingIndex !== -1}<button class="danger" type="button" onclick={deleteDomainDraft} disabled={domainSaving}>Remove domain</button>{/if}<button class="primary" type="submit" disabled={domainSaving}>{domainSaving ? 'Applying routing…' : 'Save domain'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if bulkEnvironmentModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) bulkEnvironmentModal = false; }}>
    <div class="modal bulk-environment-modal" role="dialog" aria-modal="true" aria-labelledby="bulk-environment-title">
      <header><div><span>Paste configuration</span><h2 id="bulk-environment-title">Bulk edit environment</h2></div><button aria-label="Close" onclick={() => bulkEnvironmentModal = false}>×</button></header>
      <div class="bulk-environment-body">
        <p>Paste the contents of an existing <code>.env</code> file. Applying replaces the rows in the editor, but nothing restarts until you choose <strong>Save & restart</strong>.</p>
        {#if bulkEnvironmentError}<div class="domain-feedback error"><strong>Could not read .env</strong><span>{bulkEnvironmentError}</span></div>{/if}
        <label for="bulk-environment-value">Environment file</label>
        <textarea id="bulk-environment-value" bind:value={bulkEnvironmentText} placeholder={'# Application\nAPP_ENV=production\nAPP_DEBUG=false\nAPI_TOKEN="replace-me"'} spellcheck="false" wrap="off"></textarea>
        <small>Supports comments, blank lines, quoted values, inline comments, and <code>export KEY=value</code>.</small>
      </div>
      <footer><button onclick={() => bulkEnvironmentModal = false}>Cancel</button><button class="primary" onclick={applyBulkEnvironment}>Apply to editor</button></footer>
    </div>
  </div>
{/if}

{#if databaseDeleteService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !databaseDeleteBusy) databaseDeleteService = null; }}>
    <div class="modal database-delete-modal" role="dialog" aria-modal="true" aria-labelledby="database-delete-title">
      <header><div><span>Database removal</span><h2 id="database-delete-title">Remove {databaseDeleteService.name}?</h2></div><button aria-label="Close" onclick={() => databaseDeleteService = null} disabled={databaseDeleteBusy}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); deleteDatabase(); }}>
        <div class="deletion-warning"><strong>The container will be removed immediately</strong><p>You can retain the Docker volume for manual recovery, or permanently delete all database data.</p></div>
        <label class="volume-choice"><input type="checkbox" bind:checked={databaseDeleteVolume} /><span><strong>Delete persistent volume and all data</strong><small>Leave unchecked to retain <code>{databaseDeleteService.volumeName}</code> as an unmanaged Docker volume.</small></span></label>
        {#if databaseDeleteError}<div class="domain-feedback error"><strong>Database not removed</strong><span>{databaseDeleteError}</span></div>{/if}
        <label class="confirm-field"><span>Type <code>{databaseDeleteService.name}</code> to confirm</span><input bind:value={databaseDeleteConfirmation} autocomplete="off" spellcheck="false" placeholder={databaseDeleteService.name} /></label>
        <footer><button type="button" onclick={() => databaseDeleteService = null} disabled={databaseDeleteBusy}>Cancel</button><button class="destructive" type="submit" disabled={databaseDeleteBusy || databaseDeleteConfirmation !== databaseDeleteService.name}>{databaseDeleteBusy ? 'Removing…' : databaseDeleteVolume ? 'Delete database and data' : 'Remove and retain volume'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if applicationDeleteService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !applicationDeleteBusy) applicationDeleteService = null; }}>
    <div class="modal database-delete-modal" role="dialog" aria-modal="true" aria-labelledby="application-delete-title">
      <header><div><span>Application service removal</span><h2 id="application-delete-title">Remove {applicationDeleteService.name}?</h2></div><button aria-label="Close" onclick={() => applicationDeleteService = null} disabled={applicationDeleteBusy}>×</button></header>
      <div class="modal-body"><div class="deletion-warning"><strong>The service container will be removed</strong><p>Routes that target this service must be removed first. The main application and databases are not affected.</p></div>{#if applicationDeleteError}<div class="domain-feedback error"><strong>Service not removed</strong><span>{applicationDeleteError}</span></div>{/if}</div>
      <footer><button onclick={() => applicationDeleteService = null} disabled={applicationDeleteBusy}>Cancel</button><button class="destructive" onclick={deleteApplicationService} disabled={applicationDeleteBusy}>{applicationDeleteBusy ? 'Removing…' : 'Remove service'}</button></footer>
    </div>
  </div>
{/if}

{#if serviceSettingsService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !serviceSettingsSaving) serviceSettingsService = null; }}>
    <div class="modal application-service-modal" role="dialog" aria-modal="true" aria-labelledby="service-settings-title">
      <header><div><span>Runtime definition</span><h2 id="service-settings-title">Configure {serviceSettingsService.name}</h2></div><button aria-label="Close" onclick={() => serviceSettingsService = null} disabled={serviceSettingsSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); saveServiceSettings(); }}>
        <div class="service-template-note"><strong>Configuration applies on the next deployment</strong><span>The current container keeps running until you deploy this service again.</span></div>
        {#if serviceSettingsError}<div class="domain-feedback error"><strong>Configuration not saved</strong><span>{serviceSettingsError}</span></div>{/if}
        <div class="form-grid">
          <label><span>Service name</span><input bind:value={serviceSettingsForm.name} required maxlength="63" /></label>
          <label><span>Container port</span><input bind:value={serviceSettingsForm.containerPort} type="number" min="1" max="65535" required /></label>
          {#if serviceSettingsForm.sourceType === 'repository'}
            <label><span>Git account</span><select bind:value={serviceSettingsForm.connectionId} required>{#each integrations.connections || [] as connection}<option value={connection.id}>{connection.provider} · {connection.accountName}</option>{/each}</select></label>
            <label><span>Repository</span><input bind:value={serviceSettingsForm.repository} required spellcheck="false" /></label>
            <label><span>Branch</span><input bind:value={serviceSettingsForm.branch} required spellcheck="false" /></label>
            <label><span>Build strategy</span><select bind:value={serviceSettingsForm.buildStrategy}><option value="dockerfile">Dockerfile</option><option value="railpack">Railpack</option><option value="nixpacks">Nixpacks</option></select></label>
            {#if serviceSettingsForm.buildStrategy === 'dockerfile'}
              <label><span>Dockerfile path</span><input bind:value={serviceSettingsForm.dockerfilePath} required spellcheck="false" /></label>
              <label><span>Build context</span><input bind:value={serviceSettingsForm.buildContext} required spellcheck="false" /></label>
            {:else}
              <div class="wide build-pack-help"><strong>{serviceSettingsForm.buildStrategy === 'railpack' ? 'Railpack' : 'Nixpacks'} automatically detects and builds the repository.</strong><small>No Dockerfile is required. Add <code>railpack.json</code> or <code>nixpacks.toml</code> to control the build.</small></div>
            {/if}
          {:else}
            <label><span>Private registry <em>optional</em></span><select bind:value={serviceSettingsForm.registryId}><option value="">Docker Hub / public registry</option>{#each integrations.registries || [] as registry}<option value={registry.id}>{registry.name} · {registry.registryUrl}</option>{/each}</select></label>
            <label><span>Container image</span><input bind:value={serviceSettingsForm.imageUrl} required spellcheck="false" /></label>
          {/if}
          <label class="wide command-field"><span>Container command <em>optional</em></span><input bind:value={serviceSettingsForm.command} maxlength="4096" placeholder="start-dev" spellcheck="false" /><small>Arguments passed to the image ENTRYPOINT. Quotes are supported, for example <code>serve --message "hello world"</code>.</small></label>
          <label><span>Health verification</span><select bind:value={serviceSettingsForm.healthCheckType}><option value="none">Container running</option><option value="http">HTTP path</option><option value="command">Command</option></select></label>
          <label><span>Health timeout</span><input bind:value={serviceSettingsForm.healthCheckTimeoutSeconds} type="number" min="5" max="600" required /><small>Seconds before verification fails.</small></label>
          {#if serviceSettingsForm.healthCheckType === 'http'}
            <label class="wide"><span>Health path</span><input bind:value={serviceSettingsForm.healthCheckPath} required maxlength="2048" placeholder="/health" spellcheck="false" /><small>Checked over the private network on container port <code>{serviceSettingsForm.containerPort}</code>.</small></label>
          {:else if serviceSettingsForm.healthCheckType === 'command'}
            <label class="wide command-field"><span>Health command</span><input bind:value={serviceSettingsForm.healthCheckCommand} required maxlength="4096" placeholder="wget -qO- http://127.0.0.1:8080/health >/dev/null" spellcheck="false" /><small>Runs inside the container. Exit code 0 means healthy.</small></label>
          {:else}
            <div class="wide health-check-note"><strong>Container-state check</strong><small>Docker only verifies that the main process remains running.</small></div>
          {/if}
        </div>
        <footer><span>Environment variables are managed separately in the Environment tab.</span><button type="button" onclick={() => serviceSettingsService = null} disabled={serviceSettingsSaving}>Cancel</button><button class="primary" type="submit" disabled={serviceSettingsSaving}>{serviceSettingsSaving ? 'Saving…' : 'Save configuration'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if composeModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !composeImporting) composeModal = false; }}>
    <div class="modal compose-modal" role="dialog" aria-modal="true" aria-labelledby="compose-modal-title">
      <header><div><span>Bulk service import</span><h2 id="compose-modal-title">Add a Compose file</h2></div><button aria-label="Close" onclick={() => composeModal = false} disabled={composeImporting}>×</button></header>
      <div class="compose-workspace">
        <div class="compose-intro">
          <span class="compose-mark"><Icon name="layers" size={20}/></span>
          <div><strong>Turn one Compose definition into Dokyr services</strong><p>Images become application services. Official PostgreSQL, MySQL, and MariaDB images become managed databases with persistent storage.</p></div>
          <label class="compose-upload"><input type="file" accept=".yaml,.yml,application/yaml,text/yaml,text/x-yaml" onchange={chooseComposeFile}/><Icon name="file-text" size={14}/>{composeFileName || 'Choose file'}</label>
        </div>
        {#if composeError}<div class="domain-feedback error"><strong>Compose import failed</strong><span>{composeError}</span></div>{/if}
        <label class="compose-editor">
          <span>compose.yaml</span>
          <textarea value={composeText} oninput={(event) => updateComposeText(event.currentTarget.value)} spellcheck="false" placeholder={'services:\n  web:\n    image: nginx:alpine\n    ports:\n      - "80"\n  db:\n    image: postgres:17-alpine\n    environment:\n      POSTGRES_PASSWORD: change-this-password'}></textarea>
        </label>
        {#if composeValidation}
          <section class:invalid={!composeValidation.valid} class="compose-result">
            <header>
              <span class="validation-icon"><Icon name={composeValidation.valid ? 'check-circle' : 'x-circle'} size={18}/></span>
              <div><strong>{composeValidation.valid ? 'Ready to import' : 'Changes required'}</strong><small>{composeValidation.applications} application{composeValidation.applications === 1 ? '' : 's'} · {composeValidation.databases} database{composeValidation.databases === 1 ? '' : 's'} · {composeValidation.warnings.length} warning{composeValidation.warnings.length === 1 ? '' : 's'}</small></div>
            </header>
            {#if composeValidation.errors.length}
              <div class="compose-issues errors">
                {#each composeValidation.errors as issue}<p><Icon name="x-circle" size={14}/><span>{issue.service ? `${issue.service}: ` : ''}{issue.message}</span></p>{/each}
              </div>
            {/if}
            <div class="compose-service-list">
              {#each composeValidation.services as item}
                <article>
                  <span class:database={item.kind === 'database'} class="compose-service-icon"><Icon name={item.kind === 'database' ? 'database' : 'box'} size={16}/></span>
                  <div><strong>{item.name}</strong><small>{item.image} · :{item.containerPort}{item.publicPort ? ` → host ${item.publicPort}` : ''}</small></div>
                  <em>{item.kind === 'database' ? item.engine : 'application'}</em>
                </article>
              {/each}
            </div>
            {#if composeValidation.warnings.length}
              <details class="compose-warnings">
                <summary>{composeValidation.warnings.length} mapping note{composeValidation.warnings.length === 1 ? '' : 's'}</summary>
                <div>{#each composeValidation.warnings as issue}<p><Icon name="alert" size={14}/><span>{issue.service ? `${issue.service}: ` : ''}{issue.message}</span></p>{/each}</div>
              </details>
            {/if}
          </section>
        {/if}
      </div>
      <footer>
        <span>Nothing is created until validation passes.</span>
        <button type="button" onclick={() => composeModal = false} disabled={composeImporting}>Cancel</button>
        <button type="button" onclick={validateCompose} disabled={composeValidating || composeImporting || !composeText.trim()}>{composeValidating ? 'Validating…' : composeValidation ? 'Validate again' : 'Validate file'}</button>
        <button class="primary" type="button" onclick={importCompose} disabled={composeImporting || !composeValidation?.valid}>{composeImporting ? 'Creating services…' : 'Create & deploy all'}</button>
      </footer>
    </div>
  </div>
{/if}

{#if serviceModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !serviceSaving) serviceModal = false; }}>
    <div class="modal application-service-modal" role="dialog" aria-modal="true" aria-labelledby="service-modal-title">
      <header><div><span>Application workload</span><h2 id="service-modal-title">Add an application service</h2></div><button aria-label="Close" onclick={() => serviceModal = false} disabled={serviceSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createApplicationService(); }}>
        <fieldset class="service-source-picker">
          <legend>Choose a source</legend>
          <button type="button" class:active={serviceForm.sourceType === 'image'} onclick={() => chooseServiceSource('image')}>
            <span class="source-icon"><Icon name="box" size={18}/></span><span><strong>Docker image</strong><small>Pull a public or private registry image</small></span><i></i>
          </button>
          <button type="button" class:active={serviceForm.sourceType === 'repository'} onclick={() => chooseServiceSource('repository')}>
            <span class="source-icon"><Icon name="git" size={18}/></span><span><strong>Git repository</strong><small>Clone from a connected GitHub or GitLab account</small></span><i></i>
          </button>
        </fieldset>
        {#if serviceError}<div class="domain-feedback error"><strong>Service not added</strong><span>{serviceError}</span></div>{/if}
        <div class="form-grid">
          <label><span>Service name</span><input bind:value={serviceForm.name} required maxlength="63" placeholder="api, front, adminer…" /></label>
          <label><span>Container port</span><input bind:value={serviceForm.containerPort} type="number" min="1" max="65535" required /></label>
          {#if serviceForm.sourceType === 'image'}
            <label><span>Container image</span><input bind:value={serviceForm.imageUrl} required placeholder="adminer:latest" spellcheck="false" /></label>
            <label><span>Private registry <em>optional</em></span><select bind:value={serviceForm.registryId}><option value="">Docker Hub / public registry</option>{#each integrations.registries || [] as registry}<option value={registry.id}>{registry.name} · {registry.registryUrl}</option>{/each}</select></label>
          {:else}
            <label><span>Git account</span><select bind:value={serviceForm.connectionId} onchange={changeServiceConnection} required><option value="">Select GitHub or GitLab</option>{#each integrations.connections || [] as connection}<option value={connection.id}>{connection.provider === 'github' ? 'GitHub' : 'GitLab'} · {connection.accountName}</option>{/each}</select></label>
            <label class="repository-search"><span>Repository</span>
              <div class:open={serviceRepositoryPickerOpen} class="repository-combobox">
                <input
                  bind:value={serviceRepositoryQuery}
                  oninput={searchServiceRepositories}
                  onfocus={() => serviceRepositoryPickerOpen = true}
                  onkeydown={(event) => { if (event.key === 'Escape') serviceRepositoryPickerOpen = false; }}
                  placeholder={serviceRepositoriesLoading ? 'Loading repositories…' : 'Search repositories…'}
                  role="combobox"
                  aria-autocomplete="list"
                  aria-controls="service-repository-results"
                  aria-expanded={serviceRepositoryPickerOpen}
                  disabled={!serviceForm.connectionId || serviceRepositoriesLoading}
                  autocomplete="off"
                  spellcheck="false"
                />
                {#if serviceRepositoryPickerOpen && !serviceRepositoriesLoading}
                  <div id="service-repository-results" class="repository-results" role="listbox">
                    {#if filteredServiceRepositories.length}
                      {#each filteredServiceRepositories as repository}
                        <button type="button" role="option" aria-selected={serviceForm.repository === repository.fullName} onclick={() => selectServiceRepository(repository)}>
                          <span>{repository.fullName}</span>{#if repository.private}<small>Private</small>{/if}
                        </button>
                      {/each}
                    {:else}
                      <p>No matching repositories</p>
                    {/if}
                  </div>
                {/if}
              </div>
              <small class="field-help">{serviceForm.repository ? `Selected: ${serviceForm.repository}` : 'Search by repository or owner name.'}</small>
            </label>
            {#if (integrations.connections || []).length === 0}<div class="wide source-empty-note"><Icon name="link" size={16}/><span><strong>No Git account connected</strong><small>Link GitHub or GitLab under Sources before deploying a repository.</small></span><a href="/integrations">Open Sources</a></div>{/if}
            {#if serviceForm.connectionId && !serviceRepositoriesLoading && !serviceRepositoriesError && serviceRepositories.length === 0}<div class="wide source-empty-note"><Icon name="git" size={16}/><span><strong>No repositories authorized</strong><small>Choose which repositories this Git account can deploy.</small></span><a href="/integrations">Manage access</a></div>{/if}
            {#if serviceForm.connectionId && githubContentsPermissionMissing()}<div class="wide source-empty-note permission-note"><Icon name="lock" size={16}/><span><strong>GitHub Contents permission is missing</strong><small>Repository names are visible, but private clone and deployment will fail until the App has read-only Contents access.</small></span><a href={selectedServiceConnection()?.manageUrl || '/integrations'}>Fix permission</a></div>{/if}
            {#if serviceRepositoriesError}<div class="wide domain-feedback error"><strong>Repositories unavailable</strong><span>{serviceRepositoriesError}</span></div>{/if}
            <label><span>Branch</span><input bind:value={serviceForm.branch} required placeholder="main" spellcheck="false" /></label>
            <label><span>Build strategy</span><select bind:value={serviceForm.buildStrategy}><option value="dockerfile">Dockerfile</option><option value="railpack">Railpack</option><option value="nixpacks">Nixpacks</option></select><small class="field-help">Use a build pack when the repository has no Dockerfile.</small></label>
            {#if serviceForm.buildStrategy === 'dockerfile'}
              <label><span>Dockerfile path</span><input bind:value={serviceForm.dockerfilePath} required placeholder="Dockerfile" spellcheck="false" /></label>
              <label><span>Build context</span><input bind:value={serviceForm.buildContext} required placeholder="." spellcheck="false" /><small class="field-help">Directory sent to Docker, relative to the repository root.</small></label>
            {:else}
              <div class="wide build-pack-help"><strong>{serviceForm.buildStrategy === 'railpack' ? 'Railpack' : 'Nixpacks'} will inspect this repository and create the container image.</strong><small>No Dockerfile is required. You can configure it in the repository with <code>{serviceForm.buildStrategy === 'railpack' ? 'railpack.json' : 'nixpacks.toml'}</code>.</small></div>
            {/if}
          {/if}
          <label class="wide command-field"><span>Container command <em>optional</em></span><input bind:value={serviceForm.command} maxlength="4096" placeholder="start-dev" spellcheck="false" /><small>Arguments passed to the image ENTRYPOINT. For Keycloak, enter <code>start-dev</code>.</small></label>
          <label><span>Health verification</span><select bind:value={serviceForm.healthCheckType}><option value="none">Container running</option><option value="http">HTTP path</option><option value="command">Command</option></select></label>
          <label><span>Health timeout</span><input bind:value={serviceForm.healthCheckTimeoutSeconds} type="number" min="5" max="600" required /><small>Seconds before the deployment fails.</small></label>
          {#if serviceForm.healthCheckType === 'http'}
            <label class="wide"><span>Health path</span><input bind:value={serviceForm.healthCheckPath} required maxlength="2048" placeholder="/health" spellcheck="false" /><small>Dokyr calls this path on private port <code>{serviceForm.containerPort}</code>. HTTP 2xx or 3xx is healthy.</small></label>
          {:else if serviceForm.healthCheckType === 'command'}
            <label class="wide command-field"><span>Health command</span><input bind:value={serviceForm.healthCheckCommand} required maxlength="4096" placeholder="wget -qO- http://127.0.0.1:8080/health >/dev/null" spellcheck="false" /><small>Runs inside the container with <code>/bin/sh -c</code>. Exit code 0 is healthy.</small></label>
          {:else}
            <div class="wide health-check-note"><strong>Container-state check</strong><small>Fastest option. Docker only confirms that the main process is running.</small></div>
          {/if}
          <label class="wide service-environment"><span>Environment variables <em>optional</em></span><textarea bind:value={serviceForm.environment} placeholder={'# One KEY=value per line\nPMA_ARBITRARY=1\nAPI_ENV=production'} spellcheck="false"></textarea><small>Use the private container names shown by database credentials when a service needs a database host.</small></label>
        </div>
        <footer><span>The service stays private until you assign it in Domain routing.</span><button type="button" onclick={() => serviceModal = false} disabled={serviceSaving}>Cancel</button><button class="primary" type="submit" disabled={serviceSaving}>{serviceSaving ? (serviceForm.sourceType === 'repository' ? 'Starting build…' : 'Adding service…') : 'Add & deploy'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if databaseModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !databaseSaving) databaseModal = false; }}>
    <div class="modal database-modal" role="dialog" aria-modal="true" aria-labelledby="database-modal-title">
      <header><div><span>Persistent service</span><h2 id="database-modal-title">Add a database</h2></div><button aria-label="Close" onclick={() => databaseModal = false} disabled={databaseSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createDatabase(); }}>
        <div class="engine-picker" aria-label="Database engine">
          {#each Object.entries(databasePresets) as [key, preset]}
            <button type="button" class:active={databaseForm.engine === key} onclick={() => selectDatabaseEngine(key)}>
              <span class="engine-mark">{preset.label.slice(0, 2).toUpperCase()}</span><strong>{preset.label}</strong><small>{preset.version} · {preset.port}</small>
            </button>
          {/each}
        </div>
        {#if databaseError}<div class="domain-feedback error"><strong>Database not created</strong><span>{databaseError}</span></div>{/if}
        <div class="form-grid">
          <label><span>Service name</span><input bind:value={databaseForm.name} required maxlength="50" placeholder="Primary database" /></label>
          <label><span>Database name</span><input bind:value={databaseForm.databaseName} required maxlength="63" placeholder="app" /></label>
          <label><span>Application user</span><input bind:value={databaseForm.username} required maxlength="63" placeholder="app" /></label>
          <label><span>Password <em>optional</em></span><input bind:value={databaseForm.password} type="password" minlength="12" autocomplete="new-password" placeholder="Generated securely if empty" /></label>
        </div>
        <label class="exposure-choice">
          <input bind:checked={databaseForm.publicEnabled} type="checkbox" />
          <span><strong>Expose publicly</strong><small>Off by default. Private services are reachable only by containers on <code>selfhost-proxy</code>.</small></span>
        </label>
        {#if databaseForm.publicEnabled}
          <label class="port-field"><span>Public host port</span><input bind:value={databaseForm.publicPort} type="number" min="1" max="65535" required /><small>Default for {databasePresets[databaseForm.engine].label}: {databasePresets[databaseForm.engine].port}</small></label>
        {/if}
        <footer><button type="button" onclick={() => databaseModal = false} disabled={databaseSaving}>Cancel</button><button class="primary" type="submit" disabled={databaseSaving}>{databaseSaving ? 'Pulling image…' : 'Create database'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if exposureService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !exposureSaving) exposureService = null; }}>
    <div class="modal exposure-modal" role="dialog" aria-modal="true" aria-labelledby="exposure-modal-title">
      <header><div><span>Network exposure</span><h2 id="exposure-modal-title">Expose {exposureService.name}</h2></div><button aria-label="Close" onclick={() => exposureService = null}>×</button></header>
      <div class="modal-body"><div class="warning-note"><strong>This opens the database to the network</strong><p>Use firewall rules and a strong password. Private container access will continue to work.</p></div><label class="port-field"><span>Public host port</span><input bind:value={exposurePort} type="number" min="1" max="65535" /><small>Recommended default: {exposureService.internalPort}</small></label></div>
      <footer><button onclick={() => exposureService = null} disabled={exposureSaving}>Cancel</button><button class="primary" onclick={() => saveExposure(true)} disabled={exposureSaving}>{exposureSaving ? 'Applying…' : 'Expose database'}</button></footer>
    </div>
  </div>
{/if}

{#if credentialsModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) credentialsModal = false; }}>
    <div class="modal credentials-modal" role="dialog" aria-modal="true" aria-labelledby="credentials-modal-title">
      <header><div><span>Connection details</span><h2 id="credentials-modal-title">{credentialsService?.name || 'Database credentials'}</h2></div><button aria-label="Close" onclick={() => credentialsModal = false}>×</button></header>
      {#if credentialsLoading}<div class="credential-loading"><span class="spinner"></span>Decrypting credentials…</div>{:else if credentials}
        <div class="credential-list">
          <div><span>Internal host</span><code>{credentials.host}:{credentials.port}</code><button onclick={() => copyValue('host', `${credentials.host}:${credentials.port}`)}>{copiedField === 'host' ? 'Copied' : 'Copy'}</button></div>
          <div><span>Database</span><code>{credentials.database}</code><button onclick={() => copyValue('database', credentials.database)}>{copiedField === 'database' ? 'Copied' : 'Copy'}</button></div>
          <div><span>Username</span><code>{credentials.username}</code><button onclick={() => copyValue('username', credentials.username)}>{copiedField === 'username' ? 'Copied' : 'Copy'}</button></div>
          <div><span>Password</span><code>{credentials.password}</code><button onclick={() => copyValue('password', credentials.password)}>{copiedField === 'password' ? 'Copied' : 'Copy'}</button></div>
          <div class="connection-row"><span>Connection URL</span><code>{credentials.connectionUrl}</code><button onclick={() => copyValue('url', credentials.connectionUrl)}>{copiedField === 'url' ? 'Copied' : 'Copy'}</button></div>
          {#if credentials.publicEnabled}<div><span>Public address</span><code>{platformHost}:{credentials.publicPort}</code><button onclick={() => copyValue('public', `${platformHost}:${credentials.publicPort}`)}>{copiedField === 'public' ? 'Copied' : 'Copy'}</button></div>{/if}
        </div>
        <div class="secret-note">Credentials are encrypted in PostgreSQL. Reveal and copy them only when needed.</div>
      {/if}
      <footer><button class="primary" onclick={() => credentialsModal = false}>Done</button></footer>
    </div>
  </div>
{/if}

{#if deleteModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !deleteBusy) deleteModal = false; }}>
    <div class="modal delete-project-modal" role="dialog" aria-modal="true" aria-labelledby="delete-project-title">
      <header><div><span>Permanent action</span><h2 id="delete-project-title">Delete {project.name}?</h2></div><button aria-label="Close" onclick={() => deleteModal = false} disabled={deleteBusy}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); deleteProject(); }}>
        <div class="deletion-warning"><strong>This cannot be undone</strong><p>The application container, {databaseServices.length} database service{databaseServices.length === 1 ? '' : 's'} and their persistent data, deployment history, and Caddy route will be removed.</p></div>
        {#if deleteError}<div class="domain-feedback error"><strong>Project not deleted</strong><span>{deleteError}</span></div>{/if}
        <label class="confirm-field"><span>Type <code>{project.name}</code> to confirm</span><input bind:value={deleteConfirmation} autocomplete="off" spellcheck="false" placeholder={project.name} /></label>
        <footer><button type="button" onclick={() => deleteModal = false} disabled={deleteBusy}>Cancel</button><button class="destructive" type="submit" disabled={deleteBusy || deleteConfirmation !== project.name}>{deleteBusy ? 'Deleting resources…' : 'Delete project permanently'}</button></footer>
      </form>
    </div>
  </div>
{/if}

<style>
  /* ---------- Page feedback ---------- */
  .feedback { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); font-size: var(--text-sm); }
  .feedback span { color: var(--color-muted); }
  .feedback button { border: 0; background: transparent; color: var(--color-muted); cursor: pointer; font-size: var(--text-lg); line-height: 1; }
  .feedback.error { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised)); }
  .feedback.error strong { color: var(--color-danger); }
  .feedback.success { border-color: color-mix(in srgb, var(--color-success) 35%, var(--color-rule)); background: var(--color-success-soft); }
  .feedback.success strong { color: var(--color-success); }

  /* ---------- Hero ---------- */
  .project-hero { min-height: 120px; margin-bottom: var(--space-4); padding: var(--space-5); display: flex; flex-direction: column; align-items: flex-start; justify-content: space-between; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .project-hero h2 { margin: var(--space-3) 0 var(--space-1); font-size: var(--text-xl); font-weight: 700; letter-spacing: -0.02em; }
  .project-hero p { max-width: 76ch; margin: 0; overflow-wrap: anywhere; color: var(--color-muted); font-size: var(--text-sm); }
  .endpoint { margin-top: var(--space-2); display: inline-flex; align-items: center; gap: 5px; color: var(--color-accent); font-size: var(--text-sm); font-weight: 600; text-decoration: none; }
  .endpoint:hover { text-decoration: underline; }
  .hero-actions { display: flex; gap: var(--space-2); }
  .hero-actions button, .deploy-small, .recent header button { min-height: 36px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .hero-actions button:hover:not(:disabled) { background: var(--color-paper-subtle); }
  .hero-actions .deploy, .deploy-small { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .hero-actions .deploy:hover:not(:disabled), .deploy-small:hover:not(:disabled) { border-color: var(--color-accent-hover); background: var(--color-accent-hover); }
  button:disabled { opacity: 0.55; cursor: not-allowed; }

  /* ---------- Tabs ---------- */
  .tabs { margin-bottom: var(--space-4); display: flex; gap: var(--space-5); overflow-x: auto; border-bottom: 1px solid var(--color-rule); scrollbar-width: none; }
  .tabs::-webkit-scrollbar { display: none; }
  .tabs button { min-height: 40px; padding: 0 1px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--color-muted); font-size: var(--text-sm); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .tabs button:hover { color: var(--color-ink); }
  .tabs button.active { border-bottom-color: var(--color-accent); color: var(--color-ink); }

  /* ---------- Shared panel pieces ---------- */
  .overview-grid { margin-bottom: var(--space-4); display: grid; gap: var(--space-4); }
  .panel > header { min-height: 58px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .panel header div { display: grid; gap: 1px; }
  .panel header span { color: var(--color-muted); font-size: var(--text-xs); }
  .panel h3 { margin: 0; font-size: var(--text-md); font-weight: 600; letter-spacing: -0.01em; }
  dl { margin: 0; }
  .runtime-facts dl, .detail-list { padding: var(--space-2) var(--space-5); }
  dl > div { min-height: 46px; display: grid; grid-template-columns: minmax(110px, 0.7fr) minmax(0, 1fr); align-items: center; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  dl > div:last-child { border-bottom: 0; }
  dt { color: var(--color-muted); font-size: var(--text-sm); }
  dd { margin: 0; overflow-wrap: anywhere; text-align: right; font: var(--text-xs) var(--font-mono); }
  .empty { min-height: 180px; padding: var(--space-8) var(--space-5); display: flex; align-items: center; gap: var(--space-4); }
  .empty-icon { width: 40px; height: 40px; flex: 0 0 auto; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-muted); font: 600 var(--text-2xs) var(--font-mono); }
  .empty h4 { margin: 0 0 var(--space-1); font-size: var(--text-md); }
  .empty p { max-width: 60ch; margin: 0; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.55; }
  .state { min-height: 280px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); }
  .state h3, .state p { margin: 0; }
  .state h3 { color: var(--color-ink); font-size: var(--text-md); }
  .state p { margin-top: var(--space-1); font-size: var(--text-sm); }

  /* ---------- Services ---------- */
  .services { container-type: inline-size; }
  .service-head-actions { display: flex !important; grid-auto-flow: column; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: var(--space-2) !important; }
  .service-head-actions b { min-width: 30px; height: 30px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); font: 600 var(--text-sm) var(--font-mono); }
  .service-head-actions button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: 6px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .service-head-actions .service-add-primary { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .service-head-actions .compose-add { border-color: color-mix(in srgb, var(--color-info) 40%, var(--color-rule)); color: var(--color-info); }
  .services article { min-height: 72px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto auto; align-items: center; gap: var(--space-3); }
  .service-icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); }
  .service-icon.database { background: var(--color-info-soft); color: var(--color-info); }
  .service-icon.application { background: var(--color-accent-soft); color: var(--color-accent); }
  .services article > div { min-width: 0; display: grid; gap: 2px; }
  .services article strong { font-size: var(--text-md); }
  .services article small { overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .application-service-row, .database-row { border-top: 1px solid var(--color-rule); }
  .application-service-actions, .database-actions { display: flex !important; flex-wrap: wrap; justify-content: flex-end; gap: var(--space-1) !important; }
  .application-service-actions button, .database-actions button { min-height: 32px; padding: 0 var(--space-2); display: inline-flex; align-items: center; justify-content: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .application-service-actions button:hover:not(:disabled), .database-actions button:hover:not(:disabled) { background: var(--color-paper-subtle); }
  .application-service-actions .danger-text, .database-actions .danger-text { color: var(--color-danger); }
  .application-service-actions .lifecycle-stop, .database-actions .lifecycle-stop, .database-card-actions .lifecycle-stop { border-color: color-mix(in srgb, var(--color-warning) 36%, var(--color-rule)); color: var(--color-warning); }
  .application-service-actions .lifecycle-restart, .database-actions .lifecycle-restart, .database-card-actions .lifecycle-restart { border-color: color-mix(in srgb, var(--color-info) 34%, var(--color-rule)); color: var(--color-info); }
  .application-service-actions .terminal-action { border-color: color-mix(in srgb, var(--color-accent) 34%, var(--color-rule)); color: var(--color-accent); }
  .application-service-actions .icon-only, .database-actions .icon-only { width: 32px; padding: 0; }
  .database-state { justify-items: end; gap: var(--space-1) !important; }
  .database-state em { color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase; }
  .database-state em.public { color: var(--color-warning); }

  /* ---------- Service terminal ---------- */
  .terminal-modal-backdrop { z-index: 220; }
  .service-terminal-modal { width: min(1080px, 100%); height: min(760px, calc(100vh - 32px)); display: grid; grid-template-rows: auto auto minmax(0, 1fr); overflow: hidden; }
  .service-terminal-modal > header { background: var(--color-paper-raised); }
  .terminal-modal-toolbar { min-height: 58px; padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: minmax(220px, 1fr) minmax(260px, auto) 190px auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .terminal-modal-toolbar > div { min-width: 0; display: flex; align-items: center; gap: var(--space-3); font: var(--text-2xs) var(--font-mono); }
  .terminal-modal-toolbar > div span { display: inline-flex; align-items: center; gap: 7px; flex: 0 0 auto; color: var(--color-success); }
  .terminal-modal-toolbar i { width: 7px; height: 7px; border-radius: 50%; background: currentColor; box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-success) 12%, transparent); }
  .terminal-modal-toolbar code { min-width: 0; overflow: hidden; color: var(--color-ink); text-overflow: ellipsis; white-space: nowrap; }
  .terminal-modal-toolbar > small { color: var(--color-muted); font: var(--text-2xs) var(--font-mono); white-space: nowrap; }
  .terminal-modal-toolbar label { min-width: 0; display: grid; gap: 3px; }
  .terminal-modal-toolbar label span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase; }
  .terminal-modal-toolbar input { width: 100%; height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-xs) var(--font-mono); outline: none; }
  .terminal-modal-toolbar input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .terminal-modal-toolbar > button { min-height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .terminal-session { min-height: 0; display: grid; grid-template-rows: minmax(0, 1fr) auto; background: var(--color-log-bg); color: var(--color-log-text); }
  .terminal-command-output { min-height: 0; overflow: auto; font-family: var(--font-mono); }
  .terminal-welcome { min-height: 100%; padding: var(--space-8); display: grid; place-content: center; justify-items: center; gap: var(--space-2); color: var(--color-log-muted); text-align: center; }
  .terminal-welcome strong { color: var(--color-log-text); font-size: var(--text-sm); }
  .terminal-welcome span { max-width: 62ch; font-size: var(--text-xs); line-height: 1.6; }
  .terminal-entry { border-bottom: 1px solid var(--color-log-rule); }
  .terminal-entry.failed { box-shadow: inset 2px 0 var(--color-danger); }
  .terminal-prompt { min-height: 38px; padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: start; gap: var(--space-2); background: color-mix(in srgb, var(--color-log-surface) 78%, transparent); }
  .terminal-prompt span { color: var(--color-success); font-size: var(--text-xs); white-space: nowrap; }
  .terminal-prompt code { overflow-wrap: anywhere; color: var(--color-log-text); font: var(--text-xs)/1.55 var(--font-mono); }
  .terminal-prompt time { color: var(--color-log-muted); font-size: var(--text-2xs); }
  .terminal-entry pre { margin: 0; padding: var(--space-3) var(--space-4); overflow-x: auto; color: var(--color-log-text); font: var(--text-xs)/1.65 var(--font-mono); white-space: pre-wrap; overflow-wrap: anywhere; }
  .terminal-entry pre.terminal-stderr { color: color-mix(in srgb, var(--color-danger) 75%, var(--color-log-text)); }
  .terminal-running { min-height: 48px; padding: 0 var(--space-4); display: flex; align-items: center; gap: var(--space-2); color: var(--color-log-muted); font-size: var(--text-xs); }
  .terminal-running .spinner { width: 14px; height: 14px; }
  .terminal-empty-output { display: block; padding: var(--space-3) var(--space-4); color: var(--color-log-muted); font-size: var(--text-xs); font-style: italic; }
  .terminal-entry footer { min-height: 30px; padding: 0 var(--space-4); display: flex; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-log-rule); color: var(--color-log-muted); font-size: var(--text-2xs); }
  .terminal-entry footer span:first-child { color: var(--color-success); }
  .terminal-entry footer span.failed, .terminal-entry footer strong { color: var(--color-danger); }
  .terminal-entry footer strong { margin-left: auto; font-weight: 500; }
  .terminal-command-form { min-height: 54px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 18px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border-top: 1px solid var(--color-log-rule); background: var(--color-log-surface); }
  .terminal-command-form > span { color: var(--color-success); font: 600 var(--text-sm) var(--font-mono); text-align: center; }
  .terminal-command-form input { width: 100%; height: 36px; border: 0; outline: 0; background: transparent; color: var(--color-log-text); font: var(--text-sm) var(--font-mono); }
  .terminal-command-form input::placeholder { color: var(--color-log-muted); }
  .terminal-command-form button { min-height: 34px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }

  /* ---------- Recent / deployment rows ---------- */
  .recent { margin-bottom: var(--space-4); }
  .recent > a, .deployment-row { min-height: 64px; padding: 0 var(--space-5); display: grid; grid-template-columns: 116px minmax(0, 1fr) 60px 170px 20px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; }
  .recent > a:hover, .deployment-row:hover { background: var(--color-surface-subtle); }
  .recent > a:last-child, .deployment-row:last-child { border-bottom: 0; }
  .recent a div, .deployment-row div { min-width: 0; display: grid; gap: 1px; }
  .recent a strong, .deployment-row strong { overflow: hidden; font-size: var(--text-sm); text-overflow: ellipsis; white-space: nowrap; }
  .recent a small, .deployment-row small, .recent code, .deployment-row code, .recent time, .deployment-row time { overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .deployment-row > b, .recent > a > b { color: var(--color-faint); font-weight: 400; }
  .compact-empty { padding: var(--space-8) var(--space-5); color: var(--color-muted); font-size: var(--text-sm); }
  .deployment-panel { min-height: 300px; }
  .deployment-service-tabs, .environment-service-tabs { padding: var(--space-3) var(--space-5); display: flex; align-items: center; gap: var(--space-2); overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .deployment-service-tabs button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: var(--text-sm); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .deployment-service-tabs button span { min-width: 20px; height: 20px; padding: 0 5px; display: grid; place-items: center; border-radius: 10px; background: var(--color-paper-raised); font: 500 var(--text-2xs) var(--font-mono); }
  .deployment-service-tabs button.active { border-color: var(--color-rule-strong); background: var(--color-paper-raised); color: var(--color-ink); }

  /* ---------- Metrics tab ---------- */
  .project-metrics-head { min-height: 72px; margin-bottom: var(--space-4); padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-5); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .project-metrics-head > div:first-child { display: grid; gap: 1px; }
  .project-metrics-head > div:first-child > span { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
  .project-metrics-head h3 { margin: 0; font-size: var(--text-md); }
  .project-metrics-head p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .metrics-freshness { display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); white-space: nowrap; }
  .metrics-freshness > i { width: 8px; height: 8px; border: 1.5px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; }
  .metrics-freshness > i.spinning { animation: spin 0.7s linear infinite; }
  .metrics-freshness button, .metrics-feedback button { min-height: 30px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .metrics-feedback { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised)); font-size: var(--text-sm); }
  .metrics-feedback span { color: var(--color-muted); }
  .metrics-loading { min-height: 280px; display: flex; align-items: center; justify-content: center; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
  .metrics-loading h3, .metrics-loading p { margin: 0; }
  .metrics-loading h3 { font-size: var(--text-md); }
  .metrics-loading p { margin-top: var(--space-1); color: var(--color-muted); font-size: var(--text-sm); }
  .project-metric-grid { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .project-metric-grid article { min-width: 0; min-height: 124px; padding: var(--space-4); display: grid; align-content: start; gap: var(--space-2); border-right: 1px solid var(--color-rule); }
  .project-metric-grid article:last-child { border-right: 0; }
  .project-metric-grid article > span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; }
  .project-metric-grid strong { overflow: hidden; font-size: var(--text-xl); font-weight: 700; letter-spacing: -0.02em; text-overflow: ellipsis; white-space: nowrap; }
  .project-metric-grid strong small { color: var(--color-muted); font-size: var(--text-sm); }
  .project-metric-grid p { margin: 0; overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .project-meter { height: 4px; overflow: hidden; border-radius: 4px; background: var(--color-paper-subtle); }
  .project-meter i { display: block; height: 100%; border-radius: inherit; background: var(--color-accent); transition: width var(--duration-base) var(--ease-out); }
  .project-workloads { min-height: 260px; }
  .project-workloads > header > b { width: 28px; height: 28px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; font: 500 var(--text-xs) var(--font-mono); }
  .workload-columns, .workload-row { display: grid; grid-template-columns: minmax(220px, 1.45fr) minmax(80px, 0.55fr) minmax(110px, 0.7fr) minmax(100px, 0.65fr) minmax(100px, 0.65fr) minmax(100px, 0.65fr); align-items: center; gap: var(--space-4); }
  .workload-columns { min-height: 36px; padding: 0 var(--space-5); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-faint); font-size: var(--text-2xs); font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; }
  .workload-row { min-height: 68px; padding: var(--space-2) var(--space-5); border-bottom: 1px solid var(--color-rule); }
  .workload-row:last-child { border-bottom: 0; }
  .workload-name { min-width: 0; display: flex; align-items: center; gap: var(--space-3); }
  .workload-name > i { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: var(--color-success); box-shadow: 0 0 0 4px var(--color-success-soft); }
  .workload-name > i.stopped { background: var(--color-faint); box-shadow: none; }
  .workload-name > span { min-width: 0; display: grid; gap: 2px; }
  .workload-name strong, .workload-name small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .workload-name strong { font-size: var(--text-sm); }
  .workload-name small { color: var(--color-muted); font-size: var(--text-xs); }
  .workload-value { min-width: 0; display: grid; gap: 5px; }
  .workload-value strong { font: 500 var(--text-xs) var(--font-mono); }
  .workload-value > i { height: 3px; overflow: hidden; border-radius: 3px; background: var(--color-paper-subtle); }
  .workload-value u { display: block; height: 100%; background: var(--color-accent); }
  .workload-value small, .workload-pair { color: var(--color-muted); font-size: var(--text-2xs); }
  .workload-pair { min-width: 0; display: grid; gap: 3px; }

  /* ---------- Databases tab ---------- */
  .database-manager > header { min-height: 58px; }
  .database-manager-list { padding: var(--space-4); display: grid; gap: var(--space-4); }
  .database-manager-card { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .database-card-heading { min-height: 64px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .database-card-heading > div { display: grid; gap: 2px; }
  .database-card-heading strong { font-size: var(--text-md); }
  .database-card-heading small { color: var(--color-muted); font-size: var(--text-xs); }
  .database-manager-card dl { margin: 0; padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--space-6); }
  .database-manager-card dl > div { min-height: 44px; display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .database-manager-card dl > div:nth-last-child(-n+2) { border-bottom: 0; }
  .database-manager-card dt { color: var(--color-muted); font-size: var(--text-xs); }
  .database-manager-card dd { margin: 0; min-width: 0; font-size: var(--text-xs); }
  .database-manager-card dd code { display: block; overflow: hidden; font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .database-manager-card dd span { color: var(--color-success); font-size: var(--text-2xs); font-weight: 600; text-transform: uppercase; }
  .database-manager-card dd span.public { color: var(--color-warning); }
  .database-card-actions { padding: var(--space-3) var(--space-4); display: flex; flex-wrap: wrap; gap: var(--space-2); }
  .database-card-actions button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .database-card-actions button:hover:not(:disabled) { background: var(--color-paper-subtle); }
  .database-card-actions .delete-database { margin-left: auto; border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); color: var(--color-danger); }

  /* ---------- Logs tab ---------- */
  .log-panel { min-height: 430px; }
  .log-source-strip { padding: var(--space-3) var(--space-4); display: flex; align-items: stretch; gap: var(--space-2); overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .log-source-strip > button { min-width: 210px; min-height: 58px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-muted); text-align: left; cursor: pointer; transition: border-color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out); }
  .log-source-strip > button:hover { border-color: var(--color-rule-strong); }
  .log-source-strip > button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 18%, transparent); }
  .log-source-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-info); }
  .log-source-icon.database { color: var(--color-warning); }
  .log-source-strip button > span:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .log-source-strip strong, .log-source-strip small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .log-source-strip strong { color: var(--color-ink); font-size: var(--text-sm); }
  .log-source-strip small { font-size: var(--text-xs); }
  .log-actions { display: flex !important; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: var(--space-2) !important; }
  .log-actions small { color: var(--color-muted); font-size: var(--text-xs); white-space: nowrap; }
  .log-actions button { min-height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .log-actions button.copied { border-color: color-mix(in srgb, var(--color-success) 50%, var(--color-rule)); color: var(--color-success); }
  .line-limit { min-height: 32px; padding-left: var(--space-2); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .line-limit span { font-size: var(--text-xs) !important; font-weight: 600; }
  .line-limit select { height: 30px; padding: 0 24px 0 2px; border: 0; outline: 0; background: transparent; color: var(--color-ink); font: var(--text-xs) var(--font-mono); cursor: pointer; }
  .live-toggle { display: inline-flex; align-items: center; gap: var(--space-2); }
  .live-toggle i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-faint); }
  .live-toggle.live { border-color: color-mix(in srgb, var(--color-success) 40%, var(--color-rule)); color: var(--color-success); }
  .live-toggle.live i { background: var(--color-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-success) 14%, transparent); animation: live-pulse 1.8s ease-out infinite; }
  .log-toolbar { min-height: 54px; padding: var(--space-2) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .log-filter-group { min-width: 0; display: flex; align-items: center; gap: var(--space-2); }
  .log-view-tabs { padding: 3px; display: flex; align-items: center; gap: 2px; flex: 0 0 auto; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .log-view-tabs button { min-height: 24px; padding: 0 var(--space-2); border: 0; border-radius: var(--radius-xs); background: transparent; color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .log-view-tabs button.active { background: var(--color-paper-raised); color: var(--color-ink); box-shadow: var(--shadow-xs); }
  .severity-filters { display: flex; align-items: center; gap: var(--space-1); overflow-x: auto; }
  .severity-filters button { min-height: 30px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 6px; border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .severity-filters button span { min-width: 19px; height: 19px; padding: 0 var(--space-1); display: grid; place-items: center; border-radius: var(--radius-xs); background: var(--color-paper-subtle); color: var(--color-muted); font: 500 var(--text-2xs) var(--font-mono); }
  .severity-filters button.active { border-color: var(--color-rule); background: var(--color-paper-subtle); color: var(--color-ink); }
  .severity-filters .debug.active { color: var(--color-debug); }
  .severity-filters .info.active { color: var(--color-info); }
  .severity-filters .warning.active { color: var(--color-warning); }
  .severity-filters .error.active { color: var(--color-danger); }
  .log-toolbar label { width: min(230px, 32vw); flex: 0 0 auto; }
  .log-toolbar input { width: 100%; height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); outline: none; }
  .log-toolbar input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .terminal-head { min-height: 40px; padding: 0 var(--space-4); display: grid; grid-template-columns: 9px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .terminal-head > span { width: 8px; height: 8px; border-radius: 50%; background: var(--color-success); }
  .terminal-head strong, .terminal-head small { font: 500 var(--text-xs) var(--font-mono); }
  .terminal-head small { color: var(--color-muted); }
  .log-console { min-height: 330px; max-height: 62vh; overflow: auto; background: var(--color-log-bg); color: var(--color-log-text); font-family: var(--font-mono); }
  .log-line { --severity-color: var(--color-info); min-height: 32px; padding: var(--space-1) var(--space-3) var(--space-1) 0; display: grid; grid-template-columns: 44px 94px 66px minmax(0, 1fr); align-items: start; border-bottom: 1px solid var(--color-log-rule); box-shadow: inset 2px 0 var(--severity-color); }
  .log-line.debug { --severity-color: var(--color-debug); }
  .log-line.info { --severity-color: var(--color-info); }
  .log-line.warning { --severity-color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 7%, var(--color-log-bg)); }
  .log-line.error { --severity-color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 9%, var(--color-log-bg)); }
  .line-number { padding-top: 3px; color: var(--color-log-muted); text-align: right; font-size: var(--text-2xs); user-select: none; }
  .log-line time { padding: 3px var(--space-3) 0; color: var(--color-log-muted); font-size: var(--text-2xs); white-space: nowrap; }
  .severity { width: fit-content; margin-top: 1px; padding: 2px var(--space-2); border: 1px solid color-mix(in srgb, var(--severity-color) 45%, transparent); border-radius: var(--radius-xs); background: color-mix(in srgb, var(--severity-color) 12%, transparent); color: var(--severity-color); font-size: var(--text-2xs); font-weight: 500; line-height: 1.5; text-transform: uppercase; }
  .log-line code { padding-top: 1px; overflow-wrap: anywhere; color: var(--color-log-text); font: var(--text-xs)/1.7 var(--font-mono); }
  .filtered-empty { min-height: 330px; display: grid; place-content: center; gap: var(--space-1); background: var(--color-log-bg); color: var(--color-log-text); text-align: center; }
  .filtered-empty span { color: var(--color-log-muted); font-size: var(--text-sm); }
  .log-state { min-height: 330px; padding: var(--space-8); display: flex; align-items: center; justify-content: center; gap: var(--space-4); }
  .log-state h4, .log-state p { margin: 0; }
  .log-state h4 { font-size: var(--text-md); }
  .log-state p { margin-top: var(--space-1); color: var(--color-muted); font-size: var(--text-sm); }

  /* ---------- Environment tab ---------- */
  .environment-panel { min-height: 430px; }
  .environment-header-actions { display: flex !important; grid-auto-flow: column; align-items: center; gap: var(--space-2) !important; }
  .add-variable, .bulk-variable { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: 6px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .add-variable:hover:not(:disabled), .bulk-variable:hover:not(:disabled) { border-color: var(--color-accent); color: var(--color-accent); }
  .environment-service-tabs { align-items: stretch; }
  .environment-service-tabs > button { min-width: 190px; min-height: 56px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-muted); text-align: left; cursor: pointer; }
  .environment-service-tabs > button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 18%, transparent); }
  .service-tab-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); }
  .environment-service-tabs button > span:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .environment-service-tabs strong, .environment-service-tabs small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .environment-service-tabs strong { color: var(--color-ink); font-size: var(--text-sm); }
  .environment-service-tabs small { font-size: var(--text-xs); }
  .environment-feedback { margin: 0 var(--space-5) var(--space-4); padding: var(--space-3); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); font-size: var(--text-sm); }
  .environment-feedback span { color: var(--color-muted); }
  .environment-feedback.error { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised)); }
  .environment-feedback.success { border-color: color-mix(in srgb, var(--color-success) 35%, var(--color-rule)); background: var(--color-success-soft); }
  .environment-loading { min-height: 240px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); font-size: var(--text-sm); }
  .environment-columns, .variable-row { display: grid; grid-template-columns: minmax(220px, 0.8fr) minmax(320px, 1.7fr) 72px 34px; gap: var(--space-2); }
  .environment-columns { margin-top: var(--space-5); padding: 0 var(--space-5) var(--space-2); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; }
  .variable-list { padding: 0 var(--space-5) var(--space-5); display: grid; gap: var(--space-2); }
  .variable-row { align-items: center; }
  .variable-key-field { min-width: 0; }
  .variable-key-field input { letter-spacing: 0.01em; }
  .variable-row input[type='text'], .variable-row input[type='password'] { width: 100%; height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-log-bg); color: var(--color-log-text); font: var(--text-sm) var(--font-mono); outline: none; }
  .variable-row input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 18%, transparent); }
  .value-field { position: relative; }
  .value-field input { padding-right: 52px !important; }
  .value-field > button { position: absolute; top: 5px; right: 5px; height: 28px; padding: 0 var(--space-2); border: 1px solid var(--color-log-rule); border-radius: var(--radius-xs); background: var(--color-log-surface); color: var(--color-log-muted); font: 500 var(--text-2xs) var(--font-mono); cursor: pointer; }
  .secret-toggle { min-height: 38px; display: flex; align-items: center; justify-content: center; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); color: var(--color-muted); font-size: var(--text-xs); cursor: pointer; }
  .secret-toggle input { accent-color: var(--color-accent); }
  .remove-variable { width: 32px; height: 32px; border: 1px solid transparent; border-radius: 50%; background: transparent; color: var(--color-muted); font-size: var(--text-lg); cursor: pointer; }
  .remove-variable:hover { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); color: var(--color-danger); }
  .environment-editor > footer { min-height: 64px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .environment-editor > footer > div { display: grid; gap: 2px; }
  .environment-editor > footer strong { font-size: var(--text-sm); }
  .environment-editor > footer span { color: var(--color-muted); font-size: var(--text-xs); }
  .environment-editor > footer button { min-height: 36px; padding: 0 var(--space-4); display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .environment-editor > footer button:disabled { opacity: 0.55; cursor: wait; }
  .bulk-environment-modal { width: min(760px, 100%); }
  .bulk-environment-body { padding: var(--space-5); }
  .bulk-environment-body > p { margin: 0 0 var(--space-4); color: var(--color-muted); font-size: var(--text-sm); line-height: 1.6; }
  .bulk-environment-body > label { display: block; margin-bottom: var(--space-2); font-size: var(--text-xs); font-weight: 600; }
  .bulk-environment-body textarea { width: 100%; min-height: 320px; padding: var(--space-4); resize: vertical; border: 1px solid var(--color-log-rule); border-radius: var(--radius-md); background: var(--color-log-bg); color: var(--color-log-text); caret-color: var(--color-accent); font: var(--text-sm)/1.75 var(--font-mono); tab-size: 2; outline: none; }
  .bulk-environment-body textarea:focus { border-color: var(--color-accent); }
  .bulk-environment-body textarea::placeholder { color: var(--color-log-muted); }
  .bulk-environment-body > small { margin-top: var(--space-2); display: block; color: var(--color-muted); font-size: var(--text-xs); }

  /* ---------- Domains tab ---------- */
  .domain-panel { min-height: 430px; }
  .route-state { display: inline-flex !important; grid-auto-flow: column; align-items: center; gap: var(--space-2) !important; color: var(--color-success) !important; font-weight: 600; }
  .route-state i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-success); box-shadow: 0 0 0 4px var(--color-success-soft); }
  .domain-layout { display: grid; }
  .domain-form { padding: var(--space-5); }
  .domain-header-actions { display: flex; align-items: center; gap: var(--space-3); }
  .domain-editor-head { margin-bottom: var(--space-4); display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); }
  .domain-editor-head .form-copy { min-width: 0; }
  .domain-editor-head .form-copy p { margin-bottom: 0; }
  .add-domain { min-height: 34px; padding: 0 var(--space-3); flex: 0 0 auto; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .domain-empty { width: 100%; min-height: 150px; display: grid; place-items: center; align-content: center; gap: var(--space-2); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-ink); cursor: pointer; }
  .domain-empty > span { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent-soft); color: var(--color-accent); font-size: var(--text-xl); }
  .domain-empty strong { font-size: var(--text-md); }
  .domain-empty small { color: var(--color-muted); font-size: var(--text-sm); }
  .domain-binding-list { display: grid; gap: var(--space-4); }
  .domain-binding { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .domain-list-row { min-height: 82px; padding: var(--space-4); display: grid; grid-template-columns: minmax(0, 1fr) auto auto auto; align-items: center; gap: var(--space-3); }
  .domain-list-copy { min-width: 0; display: grid; gap: var(--space-1); }
  .domain-list-copy strong { overflow: hidden; font: 600 var(--text-sm) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .domain-list-copy span { overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .domain-list-row > a, .domain-list-row > button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; text-decoration: none; cursor: pointer; }
  .domain-list-row > button { color: var(--color-ink); }
  .https-badge { padding: 4px 7px; border-radius: var(--radius-xs); background: var(--color-success-soft); color: var(--color-success); font: 700 var(--text-2xs) var(--font-mono); letter-spacing: 0.04em; }
  .binding-domain { min-width: 0; display: grid; gap: var(--space-2); }
  .binding-domain > span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; }
  .binding-domain input { width: 100%; min-width: 0; height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-sm) var(--font-mono); outline: none; }
  .binding-domain input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .binding-https { height: 38px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); color: var(--color-muted); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .binding-https input { accent-color: var(--color-accent); }
  .remove-domain { height: 38px; padding: 0 var(--space-3); border: 1px solid color-mix(in srgb, var(--color-danger) 28%, var(--color-rule)); border-radius: var(--radius-sm); background: transparent; color: var(--color-danger); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .binding-rules-head { padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); }
  .binding-rules-head > div { display: grid; gap: 1px; }
  .binding-rules-head strong { font-size: var(--text-sm); }
  .binding-rules-head small { color: var(--color-muted); font-size: var(--text-xs); }
  .binding-rules-head button { min-height: 30px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .binding-rules { padding: 0 var(--space-4) var(--space-4); }
  .binding-rules .route-rule { background: var(--color-paper-raised); }
  .route-rule { min-height: 48px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: minmax(110px, 1fr) 24px minmax(150px, 1.1fr) minmax(150px, 0.9fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-bottom: 0; background: var(--color-surface-subtle); }
  .route-rule:first-of-type { border-radius: var(--radius-sm) var(--radius-sm) 0 0; }
  .route-rule:last-of-type { border-bottom: 1px solid var(--color-rule); border-radius: 0 0 var(--radius-sm) var(--radius-sm); }
  .route-rule > span { color: var(--color-muted); text-align: center; }
  .route-rule code { overflow: hidden; font-size: var(--text-xs); text-overflow: ellipsis; }
  .editable-rule input { min-width: 0; height: 34px; padding: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-xs) var(--font-mono); outline: none; }
  .editable-rule input:focus { border-color: var(--color-accent); }
  .editable-rule > button { width: 30px; height: 30px; border: 0; background: transparent; color: var(--color-danger); font-size: var(--text-lg); cursor: pointer; }
  .editable-rule > button:disabled { color: var(--color-muted); opacity: 0.35; cursor: not-allowed; }
  .port-input { display: grid; grid-template-columns: auto 1fr; align-items: center; gap: var(--space-2); }
  .port-input small, .service-target small { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; text-transform: uppercase; }
  .service-target { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: var(--space-2); }
  .service-target select { min-width: 0; height: 34px; padding: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-xs) var(--font-mono); outline: none; }
  .binding-preview { padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: minmax(110px, auto) minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .binding-preview > span { overflow: hidden; font: 500 var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .binding-preview code { overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .form-copy h4 { margin: 0 0 var(--space-2); font-size: var(--text-lg); }
  .form-copy p { max-width: 62ch; margin: 0 0 var(--space-5); color: var(--color-muted); font-size: var(--text-sm); line-height: 1.6; }
  .route-hint { margin: var(--space-2) 0 0; color: var(--color-muted); font-size: var(--text-xs); }
  .route-hint code, .domain-form code, .route-guide code { font-family: var(--font-mono); }
  .route-rules-footer { margin-top: var(--space-4); padding-top: var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px dashed var(--color-rule); }
  .route-rules-footer span { color: var(--color-muted); font-size: var(--text-xs); }
  .domain-save-footer { margin-top: var(--space-4); padding-top: var(--space-4); border-top-style: solid; }
  .domain-feedback { margin-bottom: var(--space-4); padding: var(--space-3); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); font-size: var(--text-sm); }
  .domain-feedback span { color: var(--color-muted); }
  .domain-feedback.error { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised)); }
  .domain-feedback.success { border-color: color-mix(in srgb, var(--color-success) 35%, var(--color-rule)); background: var(--color-success-soft); }
  .domain-editor-modal { width: min(780px, calc(100vw - 32px)); }
  .domain-editor-form { display: grid; }
  .domain-editor-body { padding: var(--space-5); }
  .domain-editor-body > p { margin: 0 0 var(--space-5); color: var(--color-muted); font-size: var(--text-sm); line-height: 1.6; }
  .domain-draft-head { margin-bottom: var(--space-4); display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: var(--space-3); }
  .domain-editor-form > footer { padding: var(--space-4) var(--space-5); display: flex; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); }
  .domain-editor-form > footer button { min-height: 36px; padding: 0 var(--space-4); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .domain-editor-form > footer .primary { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .domain-editor-form > footer .danger { margin-right: auto; border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); color: var(--color-danger); }
  .domain-editor-form > footer button:disabled { opacity: 0.55; cursor: wait; }
  .route-guide { padding: var(--space-5); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .guide-label { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
  .route-guide ol { margin: var(--space-4) 0 0; padding: 0; display: grid; gap: var(--space-4); list-style: none; }
  .route-guide li { display: grid; grid-template-columns: 28px minmax(0, 1fr); gap: var(--space-3); }
  .route-guide li > b { width: 28px; height: 28px; display: grid; place-items: center; border: 1px solid var(--color-rule-strong); border-radius: 50%; background: var(--color-paper-raised); font: 500 var(--text-xs) var(--font-mono); }
  .route-guide strong { font-size: var(--text-sm); }
  .route-guide p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.55; }
  .routing-example { margin-top: var(--space-5); padding: var(--space-4); display: grid; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .routing-example span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
  .routing-example code { overflow: hidden; font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }

  /* ---------- Settings tab ---------- */
  .settings-stack { display: grid; gap: var(--space-4); }
  .project-editor { min-height: 0; }
  .project-editor > header code { color: var(--color-muted); font-size: var(--text-xs); }
  .project-editor > form { padding: var(--space-5); }
  .project-identity-layout { display: grid; grid-template-columns: minmax(240px, 0.8fr) minmax(320px, 1.2fr); align-items: end; gap: var(--space-4); }
  .project-boundary-note { min-height: 62px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 34px minmax(0, 1fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .project-boundary-note > span { width: 34px; height: 34px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); }
  .project-boundary-note strong { font-size: var(--text-sm); }
  .project-boundary-note p { margin: 3px 0 0; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .settings-field { display: grid; gap: var(--space-2); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; }
  .settings-field span { display: flex; align-items: center; gap: var(--space-1); }
  .settings-field em { color: var(--color-muted); font-style: normal; font-weight: 500; }
  .settings-field input, .settings-field select, .confirm-field input { width: 100%; height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-sm) var(--font-mono); outline: none; }
  .settings-field input:focus, .settings-field select:focus, .confirm-field input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .build-pack-help { min-height: 72px; padding: var(--space-3); display: grid; align-content: center; gap: 5px; border: 1px dashed color-mix(in srgb, var(--color-accent) 40%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-ink); }
  .build-pack-help strong { font-size: var(--text-xs); }
  .build-pack-help small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .build-pack-help code { color: var(--color-accent); font-size: var(--text-xs); }
  .health-check-note { min-height: 60px; padding: var(--space-3); display: grid; align-content: center; gap: 4px; border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .health-check-note strong { color: var(--color-ink); font-size: var(--text-xs); }
  .health-check-note small { color: var(--color-muted); font-size: var(--text-xs); font-weight: 500; line-height: 1.5; }
  .settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
  .settings-grid .wide { grid-column: 1 / -1; }
  .project-editor form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .project-editor form > footer > span { color: var(--color-muted); font: 500 var(--text-xs) var(--font-mono); }
  .save-settings { min-height: 36px; padding: 0 var(--space-4); display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .save-settings:disabled { opacity: 0.55; cursor: wait; }
  .runtime-settings-panel > header { align-items: center; }
  .runtime-settings-panel > header > p { max-width: 52ch; margin: 0 0 0 auto; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.55; text-align: right; }
  .runtime-service-tabs { padding: var(--space-3) var(--space-5); display: flex; align-items: stretch; gap: var(--space-2); overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .runtime-service-tabs > button { min-width: 210px; min-height: 56px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-muted); text-align: left; cursor: pointer; }
  .runtime-service-tabs > button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 18%, transparent); }
  .runtime-service-tabs button > span:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .runtime-service-tabs strong, .runtime-service-tabs small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .runtime-service-tabs strong { color: var(--color-ink); font-size: var(--text-sm); }
  .runtime-service-tabs small { font-size: var(--text-xs); }
  .runtime-settings-form { padding: var(--space-5); }
  .runtime-settings-copy { margin-bottom: var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); }
  .runtime-settings-copy span { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; }
  .runtime-settings-copy h4 { margin: 4px 0 0; font-size: var(--text-lg); }
  .runtime-settings-copy code { color: var(--color-muted); font-size: var(--text-xs); }
  .runtime-settings-grid { margin-top: var(--space-4); }
  .runtime-command-field input { border-color: color-mix(in srgb, var(--color-accent) 26%, var(--color-rule-strong)); background: var(--color-log-bg); color: var(--color-log-text); }
  .runtime-command-field small { color: var(--color-muted); font-size: var(--text-xs); font-weight: 500; line-height: 1.55; }
  .runtime-command-field code { color: var(--color-accent); }
  .runtime-apply-note { margin-top: var(--space-4); padding: var(--space-3); display: flex; align-items: flex-start; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-accent) 24%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-soft); color: var(--color-accent); }
  .runtime-apply-note div { display: grid; gap: 2px; }
  .runtime-apply-note strong { color: var(--color-ink); font-size: var(--text-sm); }
  .runtime-apply-note span { color: var(--color-muted); font-size: var(--text-sm); line-height: 1.5; }
  .deployment-triggers { margin-top: var(--space-4); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .deployment-triggers > header { min-height: 54px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .deployment-triggers > header > div { display: grid; gap: 2px; }
  .deployment-triggers > header span { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; }
  .deployment-triggers > header h4 { margin: 0; color: var(--color-ink); font-size: var(--text-md); }
  .deployment-triggers > header b { padding: 4px 8px; border: 1px solid var(--color-rule-strong); border-radius: 999px; color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; }
  .trigger-loading { min-height: 92px; padding: var(--space-4); display: flex; align-items: center; justify-content: center; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-sm); }
  .trigger-row { min-height: 72px; padding: var(--space-4); display: grid; grid-template-columns: 36px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); }
  .trigger-icon { width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 22%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); }
  .trigger-row > div { min-width: 0; display: grid; gap: 3px; }
  .trigger-row strong { color: var(--color-ink); font-size: var(--text-sm); }
  .trigger-row strong code { color: var(--color-accent); font-size: var(--text-sm); }
  .trigger-row small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .switch { display: grid; grid-template-columns: 36px 24px; align-items: center; gap: 7px; cursor: pointer; }
  .switch input { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }
  .switch > span { width: 36px; height: 20px; position: relative; border: 1px solid var(--color-rule-strong); border-radius: 999px; background: var(--color-paper-subtle); transition: background 0.16s ease, border-color 0.16s ease; }
  .switch > span::after { content: ''; width: 14px; height: 14px; position: absolute; top: 2px; left: 2px; border-radius: 50%; background: var(--color-muted); transition: transform 0.16s ease, background 0.16s ease; }
  .switch input:checked + span { border-color: var(--color-accent); background: var(--color-accent); }
  .switch input:checked + span::after { transform: translateX(16px); background: var(--color-accent-ink); }
  .switch input:focus-visible + span { outline: 2px solid var(--color-focus); outline-offset: 2px; }
  .switch em { color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 700; }
  .webhook-endpoint { margin: 0 var(--space-4) var(--space-4); padding: var(--space-3); display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-log-bg); }
  .webhook-endpoint > div { min-width: 0; display: grid; gap: 4px; }
  .webhook-endpoint small { color: var(--color-log-muted); font-size: var(--text-2xs); letter-spacing: 0.08em; text-transform: uppercase; }
  .webhook-endpoint code { overflow: hidden; color: var(--color-log-text); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .webhook-endpoint button, .webhook-endpoint a, .deployment-triggers > footer button { min-height: 30px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; text-decoration: none; cursor: pointer; }
  .webhook-endpoint.webhook-warning { border-color: color-mix(in srgb, var(--color-warning) 55%, var(--color-rule)); background: color-mix(in srgb, var(--color-warning) 9%, var(--color-log-bg)); }
  .registry-trigger-config { border-top: 1px solid var(--color-rule); }
  .registry-trigger-config > .settings-field { padding: var(--space-4); }
  .deployment-triggers > footer { min-height: 52px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .deployment-triggers > footer > span { color: var(--color-muted); font-size: var(--text-xs); }
  .deployment-triggers > footer button { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .deployment-triggers > footer button:disabled { opacity: 0.55; cursor: wait; }
  .runtime-settings-form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .runtime-settings-form > footer > span { color: var(--color-muted); font-size: var(--text-xs); }
  .runtime-settings-form > footer > div { display: flex; align-items: center; gap: var(--space-2); }
  .secondary-runtime { min-height: 36px; padding: 0 var(--space-4); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .secondary-runtime:disabled { opacity: 0.55; cursor: wait; }
  .runtime-settings-empty { min-height: 150px; padding: var(--space-5); display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; align-items: center; gap: var(--space-4); }
  .runtime-settings-empty h4 { margin: 0 0 4px; font-size: var(--text-md); }
  .runtime-settings-empty p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .runtime-settings-empty button { min-height: 36px; padding: 0 var(--space-4); display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .danger-zone { min-height: 100px; padding: var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-5); border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); border-radius: var(--radius-lg); background: color-mix(in srgb, var(--color-danger) 4%, var(--color-paper-raised)); }
  .danger-zone span { color: var(--color-danger); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; }
  .danger-zone h3 { margin: var(--space-1) 0; font-size: var(--text-md); }
  .danger-zone p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .danger-zone button { min-height: 36px; padding: 0 var(--space-4); flex: 0 0 auto; border: 1px solid color-mix(in srgb, var(--color-danger) 50%, var(--color-rule)); border-radius: var(--radius-sm); background: transparent; color: var(--color-danger); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .danger-zone button:hover { background: var(--color-danger); color: #fff; }

  /* ---------- Modals (page-specific layouts) ---------- */
  .database-modal form { padding: var(--space-5); }
  .compose-modal { width: min(820px, calc(100vw - 32px)); }
  .compose-workspace { max-height: min(72vh, 760px); padding: var(--space-5); display: grid; gap: var(--space-4); overflow-y: auto; }
  .compose-intro { padding: var(--space-4); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-info) 30%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-info) 5%, var(--color-paper-raised)); }
  .compose-mark { width: 40px; height: 40px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-info-soft); color: var(--color-info); }
  .compose-intro > div { min-width: 0; }
  .compose-intro strong { font-size: var(--text-sm); }
  .compose-intro p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .compose-upload { min-height: 34px; max-width: 180px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: 7px; overflow: hidden; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
  .compose-upload input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; pointer-events: none; }
  .compose-editor { display: grid; gap: var(--space-2); }
  .compose-editor > span { color: var(--color-muted); font: 600 var(--text-xs) var(--font-mono); }
  .compose-editor textarea { width: 100%; min-height: 250px; padding: var(--space-4); resize: vertical; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-log-bg); color: var(--color-log-text); font: var(--text-sm)/1.6 var(--font-mono); outline: none; tab-size: 2; }
  .compose-editor textarea:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .compose-result { overflow: hidden; border: 1px solid color-mix(in srgb, var(--color-success) 36%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-success) 3%, var(--color-paper-raised)); }
  .compose-result.invalid { border-color: color-mix(in srgb, var(--color-danger) 36%, var(--color-rule)); background: color-mix(in srgb, var(--color-danger) 3%, var(--color-paper-raised)); }
  .compose-result > header { min-height: 58px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 32px minmax(0, 1fr); align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .validation-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: 50%; background: var(--color-success-soft); color: var(--color-success); }
  .compose-result.invalid .validation-icon { background: color-mix(in srgb, var(--color-danger) 10%, var(--color-paper-raised)); color: var(--color-danger); }
  .compose-result > header div { display: grid; gap: 2px; }
  .compose-result > header strong { font-size: var(--text-sm); }
  .compose-result > header small { color: var(--color-muted); font-size: var(--text-xs); }
  .compose-issues { padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .compose-issues p, .compose-warnings p { margin: 0; padding: 4px 0; display: grid; grid-template-columns: 18px minmax(0, 1fr); gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .compose-issues.errors p { color: var(--color-danger); }
  .compose-service-list article { min-height: 52px; padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .compose-service-list article:last-child { border-bottom: 0; }
  .compose-service-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); }
  .compose-service-icon.database { background: var(--color-info-soft); color: var(--color-info); }
  .compose-service-list article > div { min-width: 0; display: grid; gap: 2px; }
  .compose-service-list strong { font-size: var(--text-sm); }
  .compose-service-list small { overflow: hidden; color: var(--color-muted); font: var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .compose-service-list em { color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; }
  .compose-warnings { border-top: 1px solid var(--color-rule); }
  .compose-warnings summary { padding: var(--space-3) var(--space-4); color: var(--color-warning); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .compose-warnings > div { padding: 0 var(--space-4) var(--space-3); }
  .compose-modal > footer { min-height: 62px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .compose-modal > footer > span { margin-right: auto; color: var(--color-muted); font-size: var(--text-xs); }
  .engine-picker { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-2); }
  .engine-picker button { min-height: 84px; padding: var(--space-3); display: grid; grid-template-columns: 32px minmax(0, 1fr); grid-template-rows: auto auto; align-items: center; gap: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-ink); text-align: left; cursor: pointer; }
  .engine-picker button.active { border-color: color-mix(in srgb, var(--color-accent) 50%, var(--color-rule)); background: var(--color-accent-soft); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 16%, transparent); }
  .engine-mark { width: 32px; height: 32px; grid-row: 1 / 3; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-accent); font: 600 var(--text-2xs) var(--font-mono); }
  .engine-picker strong { font-size: var(--text-sm); }
  .engine-picker small { color: var(--color-muted); font-size: var(--text-xs); }
  .form-grid { display: grid; gap: var(--space-4); }
  .form-grid .wide { grid-column: 1 / -1; }
  .form-grid label, .port-field { display: grid; gap: var(--space-2); }
  .form-grid label > span, .port-field > span { font-size: var(--text-xs); font-weight: 600; }
  .form-grid label em { color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 500; }
  .form-grid input, .form-grid select, .port-field input { width: 100%; height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); outline: none; }
  .form-grid input:focus, .form-grid select:focus, .port-field input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 14%, transparent); }
  .repository-search { position: relative; }
  .repository-combobox { position: relative; }
  .repository-combobox input { padding-right: 34px; }
  .repository-combobox::after { position: absolute; top: 15px; right: 13px; width: 7px; height: 7px; border-right: 1.5px solid var(--color-muted); border-bottom: 1.5px solid var(--color-muted); content: ''; pointer-events: none; transform: rotate(45deg); transition: transform 0.15s ease; }
  .repository-combobox.open::after { top: 18px; transform: rotate(225deg); }
  .repository-results { position: absolute; z-index: 10; top: calc(100% + 6px); right: 0; left: 0; max-height: 238px; overflow: auto; padding: var(--space-1); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); box-shadow: var(--shadow-popover); }
  .repository-results button { width: 100%; min-height: 34px; padding: 0 var(--space-2); display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); border: 0; border-radius: var(--radius-xs); background: transparent; color: var(--color-ink); font: var(--text-xs) var(--font-mono); text-align: left; cursor: pointer; }
  .repository-results button:hover, .repository-results button[aria-selected='true'] { background: var(--color-accent-soft); }
  .repository-results button span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .repository-results button small { flex: none; color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; }
  .repository-results p { margin: 0; padding: var(--space-3); color: var(--color-muted); font-size: var(--text-sm); }
  .application-service-modal { width: min(720px, calc(100vw - 32px)); }
  .application-service-modal form { padding: var(--space-5); }
  .application-service-modal form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .application-service-modal form > footer > span { margin-right: auto; color: var(--color-muted); font-size: var(--text-xs); }
  .service-source-picker { margin: 0 0 var(--space-4); padding: 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); border: 0; }
  .service-source-picker legend { margin-bottom: var(--space-2); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; }
  .service-source-picker button { min-height: 72px; padding: var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) 14px; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-ink); text-align: left; cursor: pointer; }
  .service-source-picker button:hover { border-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-rule)); }
  .service-source-picker button.active { border-color: var(--color-accent); background: var(--color-accent-soft); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 20%, transparent); }
  .service-source-picker .source-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); }
  .service-source-picker button.active .source-icon { color: var(--color-accent); }
  .service-source-picker button > span:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .service-source-picker strong { font-size: var(--text-sm); }
  .service-source-picker small { overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.35; text-overflow: ellipsis; }
  .service-source-picker i { width: 12px; height: 12px; border: 1px solid var(--color-rule-strong); border-radius: 50%; background: var(--color-paper-raised); }
  .service-source-picker button.active i { border: 3px solid var(--color-paper-raised); background: var(--color-accent); box-shadow: 0 0 0 1px var(--color-accent); }
  .source-empty-note { min-height: 54px; padding: var(--space-3); display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-muted); }
  .source-empty-note > span { display: grid; gap: 1px; }
  .source-empty-note strong { color: var(--color-ink); font-size: var(--text-sm); }
  .source-empty-note small { font-size: var(--text-xs); }
  .source-empty-note a { color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; }
  .source-empty-note.permission-note { border-color: color-mix(in srgb, var(--color-warning) 45%, var(--color-rule)); background: color-mix(in srgb, var(--color-warning) 7%, var(--color-paper-raised)); color: var(--color-warning); }
  .source-empty-note.permission-note a { color: var(--color-warning); }
  .field-help { color: var(--color-muted); font-size: var(--text-xs); }
  .service-template-note { margin-bottom: var(--space-4); padding: var(--space-4); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .service-template-note strong { font-size: var(--text-sm); }
  .service-template-note span { color: var(--color-muted); font-size: var(--text-sm); }
  .service-environment textarea { min-height: 110px; padding: var(--space-3); resize: vertical; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-log-bg); color: var(--color-log-text); font: var(--text-sm)/1.6 var(--font-mono); outline: none; }
  .service-environment textarea:focus { border-color: var(--color-accent); }
  .service-environment small { color: var(--color-muted); font-size: var(--text-xs); }
  .command-field input { background: var(--color-log-bg); color: var(--color-log-text); font-family: var(--font-mono); }
  .command-field small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .command-field code { color: var(--color-accent); font-family: var(--font-mono); }
  .exposure-choice { margin-top: var(--space-4); padding: var(--space-4); display: grid; grid-template-columns: 18px minmax(0, 1fr); align-items: start; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); cursor: pointer; }
  .exposure-choice input { margin-top: 2px; accent-color: var(--color-accent); }
  .exposure-choice span { display: grid; gap: var(--space-1); }
  .exposure-choice strong { font-size: var(--text-sm); }
  .exposure-choice small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .database-modal .port-field { margin-top: var(--space-4); }
  .port-field small { color: var(--color-muted); font-size: var(--text-xs); }
  .modal footer button { min-height: 36px; padding: 0 var(--space-4); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .modal footer .primary { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .modal-body { padding: var(--space-5); }
  .warning-note { margin-bottom: var(--space-4); padding: var(--space-4); border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-warning) 6%, var(--color-paper-raised)); }
  .warning-note strong { font-size: var(--text-sm); }
  .warning-note p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.55; }
  .exposure-modal { width: min(480px, 100%); }
  .credentials-modal { width: min(720px, 100%); }
  .credential-loading { min-height: 220px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); font-size: var(--text-sm); }
  .credential-list { padding: var(--space-2) var(--space-5); }
  .credential-list > div { min-height: 54px; display: grid; grid-template-columns: 110px minmax(0, 1fr) 54px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .credential-list > div:last-child { border-bottom: 0; }
  .credential-list span { color: var(--color-muted); font-size: var(--text-xs); }
  .credential-list code { overflow: hidden; font-size: var(--text-sm); text-overflow: ellipsis; white-space: nowrap; }
  .credential-list button { min-height: 28px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .secret-note { margin: var(--space-2) var(--space-5) var(--space-5); padding: var(--space-3); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }

  /* ---------- Delete dialogs ---------- */
  .database-delete-modal form, .delete-project-modal form { padding: var(--space-5); }
  .database-delete-modal form > footer, .delete-project-modal form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .volume-choice { margin: var(--space-4) 0; padding: var(--space-4); display: flex; align-items: flex-start; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-danger) 28%, var(--color-rule)); border-radius: var(--radius-md); }
  .volume-choice input { margin-top: 2px; accent-color: var(--color-danger); }
  .volume-choice span { display: grid; gap: var(--space-1); }
  .volume-choice small { color: var(--color-muted); line-height: 1.5; }
  .deletion-warning { margin-bottom: var(--space-4); padding: var(--space-4); border-left: 3px solid var(--color-danger); border-radius: 0 var(--radius-sm) var(--radius-sm) 0; background: color-mix(in srgb, var(--color-danger) 7%, var(--color-paper-raised)); }
  .deletion-warning strong { color: var(--color-danger); font-size: var(--text-sm); }
  .deletion-warning p { margin: var(--space-2) 0 0; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.6; }
  .confirm-field { display: grid; gap: var(--space-2); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; }
  .confirm-field code { padding: 2px 5px; border-radius: var(--radius-xs); background: var(--color-paper-subtle); color: var(--color-danger); }
  .modal footer .destructive { border-color: var(--color-danger); background: var(--color-danger); color: #fff; }
  .modal footer button:disabled { opacity: 0.45; cursor: not-allowed; }

  /* ---------- Animations & responsive ---------- */
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes live-pulse { 50% { box-shadow: 0 0 0 7px transparent; } }
  @media (min-width: 42rem) { .form-grid { grid-template-columns: 1fr 1fr; } }
  @media (max-width: 41.99rem) { .service-source-picker { grid-template-columns: 1fr; } .compose-intro { grid-template-columns: 40px minmax(0, 1fr); } .compose-upload { grid-column: 1 / -1; max-width: none; } .compose-modal > footer { align-items: stretch; flex-direction: column; } .compose-modal > footer > span { margin-right: 0; } }
  @media (min-width: 50rem) { .project-hero { flex-direction: row; align-items: center; } .overview-grid { grid-template-columns: minmax(0, 1.4fr) minmax(280px, 0.8fr); } .domain-layout { grid-template-columns: minmax(0, 1.35fr) minmax(300px, 0.65fr); } .route-guide { border-top: 0; border-left: 1px solid var(--color-rule); } }
  @media (max-width: 48rem) { .recent > a, .deployment-row { grid-template-columns: 104px minmax(0, 1fr) 20px; } .recent code, .deployment-row code, .recent time, .deployment-row time { display: none; } .services article { grid-template-columns: 40px minmax(0, 1fr) auto; } }
  @media (max-width: 32rem) { .hero-actions { width: 100%; } .hero-actions button { flex: 1; padding-inline: var(--space-3); } .services article { grid-template-columns: 40px minmax(0, 1fr); } .services article :global(.status) { grid-column: 2; } .database-state, .database-actions { grid-column: 2; justify-items: start; } .feedback { grid-template-columns: 1fr auto; } .feedback span { grid-row: 2; grid-column: 1 / -1; } .engine-picker { grid-template-columns: 1fr; } .credential-list > div { grid-template-columns: 1fr 54px; padding: var(--space-2) 0; } .credential-list span { grid-column: 1 / -1; } .settings-grid { grid-template-columns: 1fr; } .settings-grid .wide { grid-column: auto; } .danger-zone, .project-editor form > footer, .runtime-settings-form > footer, .deployment-triggers > footer { align-items: flex-start; flex-direction: column; } .runtime-settings-panel > header { align-items: flex-start; flex-direction: column; } .runtime-settings-panel > header > p { margin-left: 0; text-align: left; } .runtime-settings-form > footer > div, .runtime-settings-form > footer button, .deployment-triggers > footer button { width: 100%; } .runtime-settings-empty { grid-template-columns: 42px minmax(0, 1fr); } .runtime-settings-empty button { grid-column: 1 / -1; width: 100%; } .trigger-row { grid-template-columns: 36px minmax(0, 1fr); } .trigger-row .switch { grid-column: 2; } .webhook-endpoint { grid-template-columns: 1fr; } .webhook-endpoint button { width: 100%; } }
  @media (max-width: 48rem) { .project-identity-layout { grid-template-columns: 1fr; } }
  @media (max-width: 44rem) { .log-panel > header { align-items: flex-start; flex-direction: column; } .log-actions { width: 100%; justify-content: flex-start; } .log-actions small { width: 100%; } .log-toolbar { align-items: stretch; flex-direction: column; } .log-filter-group { align-items: stretch; flex-direction: column; } .log-toolbar label { width: 100%; } .log-line { grid-template-columns: 34px 62px minmax(0, 1fr); } .log-line time { display: none; } .terminal-modal-backdrop { padding: var(--space-2); } .service-terminal-modal { height: calc(100vh - 16px); } .terminal-modal-toolbar { grid-template-columns: minmax(0, 1fr) auto; align-items: stretch; } .terminal-modal-toolbar > div, .terminal-modal-toolbar > small { grid-column: 1 / -1; } .terminal-modal-toolbar > small { white-space: normal; } .terminal-prompt { grid-template-columns: auto minmax(0, 1fr); } .terminal-prompt time { display: none; } .terminal-command-form { grid-template-columns: 18px minmax(0, 1fr); } .terminal-command-form button { grid-column: 1 / -1; justify-content: center; } .environment-panel > header { align-items: flex-start; flex-direction: column; gap: var(--space-3); } .environment-columns { display: none; } .variable-row { grid-template-columns: 1fr 64px 34px; padding: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); } .variable-row > label:first-child, .value-field { grid-column: 1 / -1; } .environment-editor > footer { align-items: stretch; flex-direction: column; } .environment-editor > footer button { width: 100%; } .domain-editor-head { align-items: stretch; flex-direction: column; } .add-domain { width: 100%; } .binding-domain { grid-column: 1 / -1; } .binding-preview { grid-template-columns: minmax(0, 1fr) auto; } .binding-preview code { grid-row: 2; grid-column: 1 / -1; } .route-rules-footer { align-items: stretch; flex-direction: column; } }
  @media (max-width: 78rem) { .project-metric-grid { grid-template-columns: repeat(3, 1fr); } .project-metric-grid article:nth-child(3) { border-right: 0; } .project-metric-grid article:nth-child(-n+3) { border-bottom: 1px solid var(--color-rule); } .workload-columns, .workload-row { grid-template-columns: minmax(210px, 1.35fr) minmax(74px, 0.5fr) minmax(105px, 0.7fr) minmax(100px, 0.65fr); } .workload-columns span:nth-child(n+5), .workload-row > :nth-child(n+5) { display: none; } }
  @media (max-width: 52rem) { .project-metrics-head { align-items: flex-start; flex-direction: column; } .metrics-freshness { width: 100%; } .metrics-freshness button { margin-left: auto; } .project-metric-grid { grid-template-columns: 1fr 1fr; } .project-metric-grid article, .project-metric-grid article:nth-child(3) { border-right: 1px solid var(--color-rule); border-bottom: 1px solid var(--color-rule); } .project-metric-grid article:nth-child(even) { border-right: 0; } .project-metric-grid article:last-child { border-bottom: 0; } .workload-columns { display: none; } .workload-row { grid-template-columns: minmax(0, 1fr) 80px 105px; } .workload-row > :nth-child(n+4) { display: none; } }
  @media (max-width: 34rem) { .project-metric-grid { grid-template-columns: 1fr; } .project-metric-grid article, .project-metric-grid article:nth-child(3), .project-metric-grid article:nth-child(even) { border-right: 0; } .workload-row { grid-template-columns: minmax(0, 1fr) 74px; } .workload-row > :nth-child(n+3) { display: none; } }
  @media (max-width: 46rem) { .database-manager-card dl { grid-template-columns: 1fr; } .database-manager-card dl > div:nth-last-child(-n+2) { border-bottom: 1px solid var(--color-rule); } .database-manager-card dl > div:last-child { border-bottom: 0; } .database-card-actions .delete-database { margin-left: 0; } }
  @container (max-width: 48rem) {
    .services article {
      grid-template-columns: 40px minmax(0, 1fr) auto;
      align-items: center;
    }
    .application-service-actions,
    .database-actions {
      grid-column: 1 / -1;
      width: 100%;
      padding-top: var(--space-2);
      overflow-x: auto;
      flex-wrap: nowrap;
      justify-content: flex-start;
      border-top: 1px solid var(--color-rule);
      scrollbar-width: thin;
    }
    .application-service-actions button,
    .database-actions button {
      flex: 0 0 auto;
    }
  }
  @media (max-width: 44rem) { .domain-header-actions { width: 100%; justify-content: space-between; } .domain-header-actions .add-domain { width: auto; } .domain-list-row { grid-template-columns: minmax(0, 1fr) auto; } .domain-list-copy { grid-column: 1 / -1; } .domain-draft-head { grid-template-columns: 1fr; } .domain-editor-form > footer { flex-wrap: wrap; } }
  @media (prefers-reduced-motion: reduce) { .spinner, .live-toggle.live i, .metrics-freshness > i.spinning { animation: none; } }
</style>
