<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  const tabs = ['overview', 'metrics', 'deployments', 'logs', 'environment', 'databases', 'domains', 'settings'];
  let activeTab = 'overview';
  let data = { project: { name: 'Loading…', status: 'deploying' }, deployments: [], services: [], applicationServices: [], databaseServices: [] };
  let loading = true;
  let deploying = false;
  let error = '';
  let notice = '';
  let platformProtocol = 'http:';
  let platformPort = '';
  let platformHost = 'localhost';
  let domainBindings = [];
  let domainSaving = false;
  let domainError = '';
  let domainNotice = '';
  let serviceModal = false;
  let serviceSaving = false;
  let serviceError = '';
  let serviceForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', environment: '' };
  let serviceRepositories = [];
  let serviceRepositoriesLoading = false;
  let serviceRepositoriesError = '';
  let serviceRepositoryQuery = '';
  let serviceRepositoryPickerOpen = false;
  let serviceSettingsService = null;
  let serviceSettingsForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '' };
  let serviceSettingsSaving = false;
  let serviceSettingsError = '';
  let runtimeSettingsServiceId = '';
  let runtimeSettingsForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '' };
  let runtimeSettingsBusy = '';
  let runtimeSettingsError = '';
  let runtimeSettingsNotice = '';
  let runtimeTriggers = { autoDeploy: false, registryWebhookEnabled: false, registryWebhookTag: '', webhookUrl: '', webhookConfigured: false };
  let runtimeTriggersLoading = false;
  let runtimeTriggersSaving = false;
  let runtimeTriggersError = '';
  let runtimeTriggersNotice = '';
  let applicationLogService = null;
  let applicationRuntimeLogs = [];
  let applicationLogLimit = 300;
  let applicationLogsLoading = false;
  let applicationLogsError = '';
  let applicationLogTimer;
  let applicationLogsCopied = false;
  let applicationDeleteService = null;
  let applicationDeleteBusy = false;
  let applicationDeleteError = '';
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
  let databaseLogService = null;
  let databaseLogTab = 'runtime';
  let databaseRuntimeLogs = [];
  let databaseDeployEvents = [];
  let databaseLogLimit = 300;
  let databaseLogsLoading = false;
  let databaseLogsError = '';
  let databaseLogTimer;
  let databaseLogsCopied = false;
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
  let deploymentFilter = 'all';
  let bulkEnvironmentModal = false;
  let bulkEnvironmentText = '';
  let bulkEnvironmentError = '';
  let projectMetrics = { global: { diskIo: {}, networkIo: {}, disk: {} }, containers: [] };
  let metricsLoading = false;
  let metricsRefreshing = false;
  let metricsError = '';
  let metricsTimer;

  $: project = data.project;
  $: service = data.services[0];
  $: databaseServices = data.databaseServices || [];
  $: applicationServices = data.applicationServices || [];
  $: legacyService = project.sourceType === 'empty' ? null : { id: 'main', name: project.name, imageUrl: project.sourceType === 'image' ? project.imageUrl : project.repository, containerPort: project.containerPort || 80, status: service?.status || project.status, container: service?.container || '', legacy: true };
  $: displayApplicationServices = [...(legacyService ? [legacyService] : []), ...applicationServices];
  $: routeTargets = displayApplicationServices.map((item) => ({ ...item, id: item.legacy ? '' : item.id }));
  $: environmentTargets = displayApplicationServices;
  $: runtimeSettingsService = applicationServices.find((item) => item.id === runtimeSettingsServiceId) || applicationServices[0] || null;
  $: activeEnvironmentTarget = environmentTargets.find((item) => item.id === environmentTargetId) || environmentTargets[0];
  $: filteredDeployments = deploymentFilter === 'all' ? data.deployments : data.deployments.filter((item) => (item.serviceId || 'main') === deploymentFilter);
  $: source = project.sourceType === 'empty' ? '' : project.sourceType === 'image' ? project.imageUrl : project.repository;
  $: filteredServiceRepositories = serviceRepositories.filter((item) => item.fullName.toLowerCase().includes(serviceRepositoryQuery.trim().toLowerCase()));
  $: domainURL = project.domain ? `${project.httpsEnabled ? 'https:' : platformProtocol}//${project.domain}${!project.httpsEnabled && platformPort ? ':' + platformPort : ''}` : '';
  $: parsedLogs = logs.map(parseLogLine);
  $: logCounts = parsedLogs.reduce((counts, entry) => ({ ...counts, [entry.severity]: (counts[entry.severity] || 0) + 1 }), { debug: 0, info: 0, warning: 0, error: 0 });
  $: visibleLogs = parsedLogs.filter((entry) => (logLevel === 'all' || entry.severity === logLevel) && entry.message.toLowerCase().includes(logQuery.trim().toLowerCase()));
  $: parsedDatabaseLogs = databaseRuntimeLogs.map(parseLogLine);
  $: parsedApplicationLogs = applicationRuntimeLogs.map(parseLogLine);

  onMount(async () => {
    platformProtocol = location.protocol || 'http:';
    platformPort = location.port;
    platformHost = location.hostname || 'localhost';
    const requestedTab = location.hash.slice(1);
    if (tabs.includes(requestedTab)) activeTab = requestedTab;
    await Promise.all([loadProject(), loadIntegrations()]);
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
    stopDatabaseLogPolling();
    stopApplicationLogPolling();
    stopMetricsPolling();
    clearTimeout(logsCopyTimer);
  });

  async function loadProject() {
    loading = true;
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
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load project';
    } finally {
      loading = false;
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
      settingsNotice = 'Project configuration saved. Deploy again when you are ready to apply the new source.';
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
      loadLogs();
      startLogPolling();
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

  async function loadEnvironment() {
    if (!activeEnvironmentTarget) {
      environmentVariables = [emptyEnvironmentVariable()];
      return;
    }
    if (environmentLoading) return;
    environmentLoading = true;
    environmentError = '';
    try {
      const endpoint = activeEnvironmentTarget.legacy ? '/api/projects/' + page.params.id + '/environment' : '/api/services/' + activeEnvironmentTarget.id + '/environment';
      const response = await api(endpoint);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load environment variables');
      environmentVariables = (payload.variables || []).map((variable) => ({ ...variable, revealed: false }));
      if (environmentVariables.length === 0) environmentVariables = [emptyEnvironmentVariable()];
    } catch (cause) {
      environmentError = cause instanceof Error ? cause.message : 'Could not load environment variables';
    } finally {
      environmentLoading = false;
    }
  }

  async function selectEnvironmentTarget(id) {
    environmentTargetId = id;
    environmentVariables = [];
    environmentNotice = '';
    environmentError = '';
    await loadEnvironment();
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
    if (logsLoading) return;
    logsLoading = true;
    logsError = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/logs?lines=' + logLimit);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load container logs');
      logs = payload.lines || [];
      logsUpdated = new Date().toLocaleTimeString();
      await tick();
      if (logsLive && logConsole) logConsole.scrollTop = logConsole.scrollHeight;
    } catch (cause) {
      logsError = cause instanceof Error ? cause.message : 'Could not load container logs';
      logs = [];
    } finally {
      logsLoading = false;
    }
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

  async function saveDomain() {
    domainSaving = true;
    domainError = '';
    domainNotice = '';
    try {
      const response = await api('/api/projects/' + page.params.id + '/domain', {
        method: 'PUT',
        body: JSON.stringify({ domains: domainBindings.map((binding) => ({ domain: binding.domain.trim(), httpsEnabled: binding.httpsEnabled, rules: binding.rules.map((rule) => ({ path: rule.path.trim(), port: Number(rule.port), serviceId: rule.serviceId || '' })) })) })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not update domain');
      data = { ...data, project: payload.project };
      domainBindings = (payload.domainBindings || []).map((binding) => ({ domain: binding.domain, httpsEnabled: binding.httpsEnabled || false, rules: (binding.rules || []).map((rule) => ({ path: rule.path, port: rule.port, serviceId: rule.serviceId || '' })) }));
      domainNotice = payload.active ? `Caddy activated ${domainBindings.length} domain${domainBindings.length === 1 ? '' : 's'} and all path rules.` : 'All domain routes were removed.';
    } catch (cause) {
      domainError = cause instanceof Error ? cause.message : 'Could not update domain';
    } finally {
      domainSaving = false;
    }
  }

  function addDomainBinding() {
    const target = routeTargets[0];
    domainBindings = [...domainBindings, { domain: '', httpsEnabled: false, rules: [{ path: '/*', port: target?.containerPort || 80, serviceId: target?.id || '' }] }];
  }

  function removeDomainBinding(index) {
    domainBindings = domainBindings.filter((_, bindingIndex) => bindingIndex !== index);
  }

  function addDomainRule(bindingIndex) {
    const target = routeTargets[0];
    domainBindings = domainBindings.map((binding, index) => index === bindingIndex ? { ...binding, rules: [...binding.rules, { path: '/api/*', port: target?.containerPort || 8080, serviceId: target?.id || '' }] } : binding);
  }

  function removeDomainRule(bindingIndex, ruleIndex) {
    domainBindings = domainBindings.map((binding, index) => index === bindingIndex ? { ...binding, rules: binding.rules.filter((_, index) => index !== ruleIndex) } : binding);
  }

  function routeTargetName(serviceId) {
    return routeTargets.find((target) => target.id === (serviceId || ''))?.name || 'application';
  }

  function setRuleService(bindingIndex, ruleIndex, serviceId) {
    const target = routeTargets.find((item) => item.id === serviceId) || routeTargets[0];
    domainBindings = domainBindings.map((binding, index) => index === bindingIndex ? {
      ...binding,
      rules: binding.rules.map((rule, index) => index === ruleIndex ? { ...rule, serviceId, port: target.containerPort || 80 } : rule)
    } : binding);
  }

  function openServiceModal() {
    serviceError = '';
    serviceRepositories = [];
    serviceRepositoriesError = '';
    serviceRepositoryQuery = '';
    serviceRepositoryPickerOpen = false;
    serviceForm = { name: '', sourceType: 'image', imageUrl: '', containerPort: 80, registryId: '', connectionId: '', repository: '', branch: 'main', dockerfilePath: 'Dockerfile', buildContext: '.', buildStrategy: 'dockerfile', command: '', environment: '' };
    serviceModal = true;
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
    serviceSettingsForm = { name: item.name, sourceType: item.sourceType || 'image', imageUrl: item.imageUrl || '', containerPort: item.containerPort || 80, registryId: item.registryId || '', connectionId: item.connectionId || '', repository: item.repository || '', branch: item.branch || 'main', dockerfilePath: item.dockerfilePath || 'Dockerfile', buildContext: item.buildContext || '.', buildStrategy: item.buildStrategy || 'dockerfile', command: item.command || '' };
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
      command: item.command || ''
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
        command: payload.service.command || ''
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
      await loadProject();
      setTimeout(loadProject, 2500);
      setTimeout(loadProject, 6000);
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
    await loadProject();
    setTimeout(loadProject, 2500);
    setTimeout(loadProject, 6000);
  }

  function openApplicationLogs(item) {
    applicationLogService = item;
    applicationRuntimeLogs = [];
    applicationLogsError = '';
    loadApplicationLogs();
    stopApplicationLogPolling();
    applicationLogTimer = setInterval(loadApplicationLogs, 2500);
  }

  function stopApplicationLogPolling() {
    if (applicationLogTimer) clearInterval(applicationLogTimer);
    applicationLogTimer = null;
  }

  function closeApplicationLogs() {
    stopApplicationLogPolling();
    applicationLogService = null;
  }

  async function loadApplicationLogs() {
    if (!applicationLogService || applicationLogsLoading) return;
    applicationLogsLoading = true;
    try {
      const response = await api(`/api/services/${applicationLogService.id}/logs?lines=${applicationLogLimit}`);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load service logs');
      applicationRuntimeLogs = payload.lines || [];
      applicationLogsError = '';
    } catch (cause) {
      applicationLogsError = cause instanceof Error ? cause.message : 'Could not load service logs';
    } finally {
      applicationLogsLoading = false;
    }
  }

  async function copyApplicationLogs() {
    await writeClipboard(applicationRuntimeLogs.join('\n'));
    applicationLogsCopied = true;
    setTimeout(() => applicationLogsCopied = false, 1600);
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

  function openDatabaseLogs(item, initialTab = 'runtime') {
    databaseLogService = item;
    databaseLogTab = initialTab;
    databaseRuntimeLogs = [];
    databaseDeployEvents = [];
    databaseLogsError = '';
    loadDatabaseLogs();
    stopDatabaseLogPolling();
    databaseLogTimer = setInterval(loadDatabaseLogs, 2500);
  }

  function stopDatabaseLogPolling() {
    if (databaseLogTimer) clearInterval(databaseLogTimer);
    databaseLogTimer = null;
  }

  function closeDatabaseLogs() {
    stopDatabaseLogPolling();
    databaseLogService = null;
  }

  async function loadDatabaseLogs() {
    if (!databaseLogService || databaseLogsLoading) return;
    databaseLogsLoading = true;
    try {
      const [runtimeResponse, eventsResponse] = await Promise.all([
        api(`/api/databases/${databaseLogService.id}/logs?lines=${databaseLogLimit}`),
        api(`/api/databases/${databaseLogService.id}/events`)
      ]);
      const runtimePayload = await runtimeResponse.json();
      const eventsPayload = await eventsResponse.json();
      if (!runtimeResponse.ok) throw new Error(runtimePayload.error || 'Could not load runtime logs');
      if (!eventsResponse.ok) throw new Error(eventsPayload.error || 'Could not load deployment logs');
      databaseRuntimeLogs = runtimePayload.lines || [];
      databaseDeployEvents = eventsPayload.events || [];
      databaseLogsError = '';
    } catch (cause) {
      databaseLogsError = cause instanceof Error ? cause.message : 'Could not load database logs';
    } finally {
      databaseLogsLoading = false;
    }
  }

  async function copyDatabaseLogs() {
    const text = databaseLogTab === 'runtime'
      ? databaseRuntimeLogs.join('\n')
      : databaseDeployEvents.map((event) => `${event.createdAt} [${event.stage.toUpperCase()}] ${event.message}`).join('\n');
    await writeClipboard(text);
    databaseLogsCopied = true;
    setTimeout(() => databaseLogsCopied = false, 1600);
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

<Shell eyebrow="Project" title={project.name}>
  <div class="crumb"><a href="/projects">Projects</a><span>/</span><b>{project.name}</b></div>

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
        <header><div><span>Runtime</span><h3>Services</h3></div><div class="service-head-actions"><b>{displayApplicationServices.length + databaseServices.length}</b><button class="service-add-primary" onclick={openServiceModal}><Icon name="plus" size={14}/> Add service</button><button onclick={openDatabaseModal}><Icon name="database" size={14}/> Add database</button></div></header>
        {#if displayApplicationServices.length === 0 && databaseServices.length === 0}
          <div class="empty"><div class="empty-icon"><Icon name="box" size={22} /></div><div><h4>No services yet</h4><p>Add a containerized frontend, API, admin tool, or database. Every service stays private until it receives a domain route.</p></div></div>
        {/if}
        {#each displayApplicationServices as item}
          <article class="application-service-row">
            <span class="service-icon application"><Icon name="box" size={18} /></span>
            <div><strong>{item.name}</strong><small>{item.sourceType === 'repository' ? `${item.repository}@${item.branch}` : (item.imageUrl || 'No image configured')} · :{item.containerPort}{item.command ? ` · command ${item.command}` : ''} · {item.container || 'container not created'}</small></div>
            <Status value={item.status} />
            <div class="application-service-actions">
              <button onclick={() => item.legacy ? selectTab('logs') : openApplicationLogs(item)} disabled={!item.container}><Icon name="logs" size={13}/> Logs</button>
              <button onclick={() => openEnvironmentFor(item.id)}><Icon name="settings" size={13}/> Env</button>
              {#if !item.legacy}<button class="icon-only" title="Configure service" aria-label={'Configure ' + item.name} onclick={() => openServiceSettings(item)}><Icon name="settings" size={14}/></button>{/if}
              <button onclick={() => item.legacy ? deploy() : deployApplicationService(item)} disabled={item.status === 'deploying' || item.legacy && project.sourceType !== 'image'}><Icon name="play" size={13}/>{item.status === 'deploying' ? 'Pulling…' : 'Deploy'}</button>
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
              <button onclick={() => openDatabaseLogs(item)}><Icon name="logs" size={13}/> Logs</button>
              <button onclick={() => showCredentials(item)}><Icon name="key" size={13}/> Credentials</button>
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
                <button onclick={() => openDatabaseLogs(item, 'runtime')}><Icon name="activity" size={14} /> Runtime logs</button>
                <button onclick={() => openDatabaseLogs(item, 'deploy')}><Icon name="rocket" size={14} /> Deployment logs</button>
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
        <div><span>Container output</span><h3>Runtime logs</h3></div>
        <div class="log-actions">
          {#if logsUpdated}<small>Updated {logsUpdated}</small>{/if}
          <label class="line-limit">
            <span>Lines</span>
            <select bind:value={logLimit} onchange={changeLogLimit} aria-label="Number of log lines">
              <option value={100}>100</option>
              <option value={300}>300</option>
              <option value={500}>500</option>
              <option value={1000}>1,000</option>
            </select>
          </label>
          <button class="live-toggle" class:live={logsLive} onclick={toggleLiveLogs} aria-pressed={logsLive}>
            <i></i>{logsLive ? 'Live · Pause' : 'Paused · Resume'}
          </button>
          <button class:copied={logsCopied} onclick={copyVisibleLogs} disabled={visibleLogs.length === 0} aria-live="polite">{logsCopied ? 'Copied ✓' : 'Copy output'}</button>
          <button onclick={loadLogs} disabled={logsLoading}>{logsLoading ? 'Refreshing…' : 'Refresh'}</button>
        </div>
      </header>
      <div class="log-toolbar">
        <div class="severity-filters" aria-label="Filter logs by severity">
          <button class:active={logLevel === 'all'} onclick={() => logLevel = 'all'}>All <span>{logs.length}</span></button>
          <button class="debug" class:active={logLevel === 'debug'} onclick={() => logLevel = 'debug'}>Debug <span>{logCounts.debug}</span></button>
          <button class="info" class:active={logLevel === 'info'} onclick={() => logLevel = 'info'}>Info <span>{logCounts.info}</span></button>
          <button class="warning" class:active={logLevel === 'warning'} onclick={() => logLevel = 'warning'}>Warning <span>{logCounts.warning}</span></button>
          <button class="error" class:active={logLevel === 'error'} onclick={() => logLevel = 'error'}>Error <span>{logCounts.error}</span></button>
        </div>
        <label><span class="sr-only">Search logs</span><input bind:value={logQuery} type="search" placeholder="Search log output" /></label>
      </div>
      {#if logsLoading && logs.length === 0}
        <div class="log-state"><span class="spinner"></span><div><h4>Reading container logs</h4><p>Loading the latest output from Docker.</p></div></div>
      {:else if logsError}
        <div class="log-state"><div class="empty-icon">!</div><div><h4>Logs unavailable</h4><p>{logsError}</p></div></div>
      {:else if logs.length === 0}
        <div class="log-state"><div class="empty-icon">LOG</div><div><h4>No output yet</h4><p>The container has not written anything to stdout or stderr.</p></div></div>
      {:else}
        <div class="terminal-head"><span></span><strong>{service?.container || 'project container'}</strong><small>{logs.length} lines</small></div>
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
      <div class="environment-intro">
        <div class="restart-mark"><Icon name="refresh" size={16}/></div>
        <div><strong>Restart {activeEnvironmentTarget.name} only—no rebuild</strong><p>Saving recreates only this service from its current local image. Other project services keep running.</p></div>
      </div>
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
                <label><span class="sr-only">Variable key</span><input bind:value={variable.key} placeholder="APP_ENV" autocomplete="off" spellcheck="false" /></label>
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
      <header><div><span>Traffic</span><h3>Domain routing</h3></div>{#if domainBindings.length}<span class="route-state"><i></i> {domainBindings.length} active</span>{/if}</header>
      <div class="domain-layout">
        <form class="domain-form" onsubmit={(event) => { event.preventDefault(); saveDomain(); }}>
          <div class="domain-editor-head"><div class="form-copy"><h4>Domains, paths, and services</h4><p>Send each hostname and path to a specific application service and its internal port.</p></div><button class="add-domain" type="button" onclick={addDomainBinding}>＋ Add domain</button></div>
          {#if domainError}<div class="domain-feedback error"><strong>Route not changed</strong><span>{domainError}</span></div>{/if}
          {#if domainNotice}<div class="domain-feedback success"><strong>Route updated</strong><span>{domainNotice}</span></div>{/if}
          {#if domainBindings.length === 0}
            <button class="domain-empty" type="button" onclick={addDomainBinding}><span>＋</span><strong>Add your first domain</strong><small>Then choose which paths and container ports it should serve.</small></button>
          {:else}
            <div class="domain-binding-list">
              {#each domainBindings as binding, bindingIndex}
                <section class="domain-binding">
                  <header class="binding-head">
                    <div class="binding-domain"><label for={'domain-' + bindingIndex}>Domain name</label><input id={'domain-' + bindingIndex} bind:value={binding.domain} placeholder={bindingIndex === 0 ? 'domain.local' : 'domain2.local'} autocomplete="off" spellcheck="false" required /></div>
                    <label class="binding-https"><input type="checkbox" bind:checked={binding.httpsEnabled} /><span>HTTPS</span></label>
                    <button class="remove-domain" type="button" onclick={() => removeDomainBinding(bindingIndex)} aria-label={'Remove ' + (binding.domain || 'domain')}>Remove</button>
                  </header>
                  <div class="binding-rules-head"><div><strong>Path forwarding</strong><small>Unmatched requests return 404.</small></div><button type="button" onclick={() => addDomainRule(bindingIndex)}>＋ Add path</button></div>
                  <div class="binding-rules">
                    {#each binding.rules as rule, ruleIndex}
                      <div class="route-rule editable-rule">
                        <input aria-label={'Path pattern for ' + (binding.domain || 'domain')} bind:value={rule.path} placeholder={ruleIndex === 0 ? '/*' : '/api/**'} required />
                        <span>→</span>
                        <label class="service-target"><small>Service</small><select aria-label="Target application service" bind:value={rule.serviceId} onchange={(event) => setRuleService(bindingIndex, ruleIndex, event.currentTarget.value)} required>{#if routeTargets.length === 0}<option value="">Add a service first</option>{/if}{#each routeTargets as item}<option value={item.id}>{item.name}{item.legacy ? ' (legacy)' : ''}</option>{/each}</select></label>
                        <div class="port-input"><small>Container port</small><input aria-label="Internal container port" bind:value={rule.port} type="number" min="1" max="65535" required /></div>
                        <button type="button" aria-label="Remove path" onclick={() => removeDomainRule(bindingIndex, ruleIndex)} disabled={binding.rules.length === 1}>×</button>
                      </div>
                    {/each}
                  </div>
                  <footer class="binding-preview"><span>{binding.domain || 'domain.local'}</span><code>{binding.rules.map((rule) => `${rule.path || '/*'} → ${routeTargetName(rule.serviceId)}:${rule.port || '—'}`).join(' · ')}</code>{#if binding.domain}<a href={domainEndpoint(binding)} target="_blank" rel="noreferrer">Open ↗</a>{/if}</footer>
                </section>
              {/each}
            </div>
          {/if}
          <p class="route-hint">Use <code>/api/**</code> or <code>/static/**</code> for prefixes. Caddy preserves the complete request path when forwarding.</p>
          <div class="route-rules-footer domain-save-footer"><span>One save updates every hostname and path as a single Caddy configuration.</span><button type="submit" disabled={domainSaving}>{domainSaving ? 'Applying routing…' : 'Save domain routing'}</button></div>
        </form>
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
        <header><div><span>Configuration</span><h3>Project settings</h3></div><code>{project.id}</code></header>
        <form onsubmit={(event) => { event.preventDefault(); saveProjectSettings(); }}>
          <div class="settings-intro"><h4>Identity and deployment source</h4><p>Changes affect the next deployment. The currently running container continues serving traffic until you deploy again.</p></div>
          {#if settingsError}<div class="domain-feedback error"><strong>Project not updated</strong><span>{settingsError}</span></div>{/if}
          {#if settingsNotice}<div class="domain-feedback success"><strong>Settings saved</strong><span>{settingsNotice}</span></div>{/if}
          <label class="settings-field"><span>Project name</span><input bind:value={settingsForm.name} required maxlength="100" /></label>
          <fieldset class="source-choice">
            <legend>Deployment source</legend>
            <button type="button" class:active={settingsForm.sourceType === 'empty'} onclick={() => settingsForm.sourceType = 'empty'}><Icon name="grid" size={17}/><span><strong>Service-first</strong><small>No legacy default container</small></span></button>
            <button type="button" class:active={settingsForm.sourceType === 'image'} onclick={() => settingsForm.sourceType = 'image'}><Icon name="box" size={17}/><span><strong>Container image</strong><small>Pull from a Docker registry</small></span></button>
            <button type="button" class:active={settingsForm.sourceType === 'repository'} onclick={() => settingsForm.sourceType = 'repository'}><Icon name="git" size={17}/><span><strong>Git repository</strong><small>GitHub, GitLab, or Git URL</small></span></button>
          </fieldset>
          {#if settingsForm.sourceType === 'image'}
            <div class="settings-grid">
              <label class="settings-field wide"><span>Container image</span><input bind:value={settingsForm.imageUrl} required placeholder="ghcr.io/acme/customer-api:latest" spellcheck="false" /></label>
              <label class="settings-field"><span>Registry credential</span><select bind:value={settingsForm.registryId}><option value="">Public image / no credential</option>{#each integrations.registries || [] as item}<option value={item.id}>{item.name} · {item.registryUrl}</option>{/each}</select></label>
			  <label class="settings-field"><span>Container port</span><input type="number" min="1" max="65535" bind:value={settingsForm.containerPort} required /></label>
              <div class="field-note"><strong>Private registry?</strong><span>Select a saved credential or <a href="/integrations">add one in Sources</a>.</span></div>
            </div>
          {:else if settingsForm.sourceType === 'repository'}
            <div class="settings-grid">
              <label class="settings-field wide"><span>Repository URL</span><input bind:value={settingsForm.repository} required placeholder="https://github.com/acme/customer-api.git" spellcheck="false" /></label>
              <label class="settings-field"><span>Branch</span><input bind:value={settingsForm.branch} required placeholder="main" spellcheck="false" /></label>
              <label class="settings-field"><span>Connected account <em>optional</em></span><select bind:value={settingsForm.connectionId}><option value="">Public repository</option>{#each integrations.connections || [] as item}<option value={item.id}>{item.provider} · {item.accountName}</option>{/each}</select></label>
              <div class="repository-note"><strong>Repository source can be saved now.</strong><span>The Git clone and image-build deployment worker is the next backend feature; current deployments run prebuilt images.</span></div>
            </div>
          {:else}
            <div class="service-first-settings"><Icon name="grid" size={18}/><div><strong>This project has no default application.</strong><span>Add, deploy, route, and configure every workload independently from the Services panel.</span></div></div>
          {/if}
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

{#if databaseLogService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeDatabaseLogs(); }}>
    <div class="modal database-logs-modal" role="dialog" aria-modal="true" aria-labelledby="database-logs-title">
      <header><div><span>Database observability</span><h2 id="database-logs-title">{databaseLogService.name} logs</h2></div><button aria-label="Close" onclick={closeDatabaseLogs}>×</button></header>
      <div class="database-log-toolbar">
        <div class="database-log-tabs"><button class:active={databaseLogTab === 'runtime'} onclick={() => databaseLogTab = 'runtime'}>Runtime</button><button class:active={databaseLogTab === 'deploy'} onclick={() => databaseLogTab = 'deploy'}>Deployment <span>{databaseDeployEvents.length}</span></button></div>
        <div class="database-log-actions"><label>Lines <select bind:value={databaseLogLimit} onchange={loadDatabaseLogs}><option value={100}>100</option><option value={300}>300</option><option value={500}>500</option><option value={1000}>1,000</option></select></label><button onclick={loadDatabaseLogs} disabled={databaseLogsLoading}>{databaseLogsLoading ? 'Refreshing…' : 'Refresh'}</button><button onclick={copyDatabaseLogs}>{databaseLogsCopied ? 'Copied' : 'Copy'}</button></div>
      </div>
      {#if databaseLogsError}<div class="database-log-error">{databaseLogsError}</div>{/if}
      <div class="database-log-console">
        {#if databaseLogTab === 'runtime'}
          {#if parsedDatabaseLogs.length === 0}<div class="log-empty">No runtime output yet.</div>{:else}{#each parsedDatabaseLogs as entry}<div class="database-log-line {entry.severity}"><time>{entry.timestamp || '—'}</time><b>{entry.severity}</b><span>{entry.message}</span></div>{/each}{/if}
        {:else}
          {#if databaseDeployEvents.length === 0}<div class="log-empty">Deployment events will appear after the next database apply.</div>{:else}{#each databaseDeployEvents as event}<div class="database-log-line {event.type === 'complete' ? 'info' : event.type === 'error' ? 'error' : 'debug'}"><time>{new Date(event.createdAt).toLocaleTimeString()}</time><b>{event.stage}</b><span>{event.message}</span></div>{/each}{/if}
        {/if}
      </div>
      <footer><span class="live-indicator"><i></i> Live · refreshes every 2.5s</span><button class="primary" onclick={closeDatabaseLogs}>Done</button></footer>
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

{#if applicationLogService}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeApplicationLogs(); }}>
    <div class="modal database-logs-modal" role="dialog" aria-modal="true" aria-labelledby="application-logs-title">
      <header><div><span>Application observability</span><h2 id="application-logs-title">{applicationLogService.name} logs</h2></div><button aria-label="Close" onclick={closeApplicationLogs}>×</button></header>
      <div class="database-log-toolbar"><span class="live-indicator"><i></i> Live runtime output</span><div class="database-log-actions"><label>Lines <select bind:value={applicationLogLimit} onchange={loadApplicationLogs}><option value={100}>100</option><option value={300}>300</option><option value={500}>500</option><option value={1000}>1,000</option></select></label><button onclick={loadApplicationLogs} disabled={applicationLogsLoading}>{applicationLogsLoading ? 'Refreshing…' : 'Refresh'}</button><button onclick={copyApplicationLogs}>{applicationLogsCopied ? 'Copied' : 'Copy'}</button></div></div>
      {#if applicationLogsError}<div class="database-log-error">{applicationLogsError}</div>{/if}
      <div class="database-log-console">
        {#if parsedApplicationLogs.length === 0}<div class="log-empty">No runtime output yet.</div>{:else}{#each parsedApplicationLogs as entry}<div class="database-log-line {entry.severity}"><time>{entry.timestamp || '—'}</time><b>{entry.severity}</b><span>{entry.message}</span></div>{/each}{/if}
      </div>
      <footer><span class="live-indicator"><i></i> Refreshes every 2.5s</span><button class="primary" onclick={closeApplicationLogs}>Done</button></footer>
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
        </div>
        <footer><span>Environment variables are managed separately in the Environment tab.</span><button type="button" onclick={() => serviceSettingsService = null} disabled={serviceSettingsSaving}>Cancel</button><button class="primary" type="submit" disabled={serviceSettingsSaving}>{serviceSettingsSaving ? 'Saving…' : 'Save configuration'}</button></footer>
      </form>
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
  .crumb { margin-bottom: var(--space-4); display: flex; gap: var(--space-2); color: var(--color-muted); font: 12px var(--font-mono); }
  .crumb a { color: var(--color-muted); text-decoration: none; }
  .crumb b { overflow: hidden; color: var(--color-ink); text-overflow: ellipsis; white-space: nowrap; }
  .feedback { min-height: 52px; margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); font-size: 13px; }
  .feedback span { color: var(--color-muted); }
  .feedback button { border: 0; background: transparent; color: var(--color-muted); cursor: pointer; font-size: 18px; }
  .feedback.error { border-color: color-mix(in oklch, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in oklch, var(--color-danger) 8%, var(--color-paper-raised)); }
  .feedback.success { border-color: color-mix(in oklch, var(--color-accent) 35%, var(--color-rule)); background: var(--color-accent-soft); }
  .project-hero { min-height: 160px; padding: var(--space-6); display: flex; flex-direction: column; align-items: flex-start; justify-content: space-between; gap: var(--space-6); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-whisper); }
  .project-hero h2 { margin: var(--space-3) 0 var(--space-1); font-size: 24px; letter-spacing: -.035em; }
  .project-hero p { max-width: 70ch; margin: 0; overflow-wrap: anywhere; color: var(--color-muted); font: 12px var(--font-mono); }
  .endpoint { margin-top: var(--space-3); display: inline-flex; color: var(--color-accent); font-size: 13px; font-weight: 600; text-decoration: none; }
  .hero-actions { display: flex; gap: var(--space-2); }
  .hero-actions button, .deploy-small, .recent header button { min-height: 42px; padding: 0 var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 13px; font-weight: 600; white-space: nowrap; cursor: pointer; }
  .hero-actions .deploy, .deploy-small { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  button:disabled { opacity: .55; cursor: not-allowed; }
  .tabs { margin-bottom: var(--space-5); display: flex; gap: var(--space-6); overflow-x: auto; border-bottom: 1px solid var(--color-rule); }
  .tabs button { min-height: 58px; padding: var(--space-1) 0 0; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--color-muted); font-size: 13px; font-weight: 600; white-space: nowrap; cursor: pointer; }
  .tabs button.active { border-bottom-color: var(--color-accent); color: var(--color-ink); }
  .overview-grid { margin-bottom: var(--space-4); display: grid; gap: var(--space-4); }
  .panel { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-whisper); }
  .panel > header { min-height: 72px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .panel header div { display: grid; gap: var(--space-1); }
  .panel header span { color: var(--color-muted); font-size: 12px; }
  .panel h3 { margin: 0; font-size: 17px; letter-spacing: -.02em; }
  .project-metrics-head { min-height: 82px; margin-bottom: var(--space-4); padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-5); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
  .project-metrics-head > div:first-child { display: grid; gap: var(--space-1); }
  .project-metrics-head > div:first-child > span { color: var(--color-accent); font: 700 9px var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
  .project-metrics-head h3 { margin: 0; font-size: 17px; letter-spacing: -.02em; }
  .project-metrics-head p { margin: 0; color: var(--color-muted); font-size: 11px; }
  .metrics-freshness { display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); font: 9px var(--font-mono); white-space: nowrap; }
  .metrics-freshness > i { width: 8px; height: 8px; border: 1px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; }
  .metrics-freshness > i.spinning { animation: spin .7s linear infinite; }
  .metrics-freshness button, .metrics-feedback button { min-height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 10px; font-weight: 600; cursor: pointer; }
  .metrics-feedback { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in oklch, var(--color-danger) 36%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in oklch, var(--color-danger) 7%, var(--color-paper-raised)); font-size: 11px; }
  .metrics-feedback span { color: var(--color-muted); }
  .metrics-loading { min-height: 280px; display: flex; align-items: center; justify-content: center; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
  .metrics-loading h3, .metrics-loading p { margin: 0; } .metrics-loading h3 { font-size: 14px; } .metrics-loading p { margin-top: var(--space-1); color: var(--color-muted); font-size: 11px; }
  .project-metric-grid { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
  .project-metric-grid article { min-width: 0; min-height: 126px; padding: var(--space-4); display: grid; align-content: start; gap: var(--space-2); border-right: 1px solid var(--color-rule); }
  .project-metric-grid article:last-child { border-right: 0; }
  .project-metric-grid article > span { color: var(--color-muted); font: 700 9px var(--font-mono); letter-spacing: .07em; text-transform: uppercase; }
  .project-metric-grid strong { overflow: hidden; font: 600 22px var(--font-sans); letter-spacing: -.04em; text-overflow: ellipsis; white-space: nowrap; }
  .project-metric-grid strong small { color: var(--color-muted); font-size: 13px; }
  .project-metric-grid p { margin: 0; color: var(--color-muted); font: 9px var(--font-mono); }
  .project-meter { height: 4px; overflow: hidden; border-radius: 4px; background: var(--color-paper-subtle); }
  .project-meter i { display: block; height: 100%; border-radius: inherit; background: var(--color-accent); transition: width var(--duration-base) var(--ease-out); }
  .project-workloads { min-height: 260px; }
  .project-workloads > header > b { width: 29px; height: 29px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; font: 9px var(--font-mono); }
  .workload-columns, .workload-row { display: grid; grid-template-columns: minmax(220px, 1.45fr) minmax(80px, .55fr) minmax(110px, .7fr) minmax(100px, .65fr) minmax(100px, .65fr) minmax(100px, .65fr); align-items: center; gap: var(--space-4); }
  .workload-columns { min-height: 36px; padding: 0 var(--space-5); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-subtle); color: var(--color-faint); font: 8px var(--font-mono); letter-spacing: .06em; text-transform: uppercase; }
  .workload-row { min-height: 72px; padding: var(--space-3) var(--space-5); border-bottom: 1px solid var(--color-rule); }
  .workload-row:last-child { border-bottom: 0; }
  .workload-name { min-width: 0; display: flex; align-items: center; gap: var(--space-3); }
  .workload-name > i { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: var(--color-accent); box-shadow: 0 0 0 4px var(--color-accent-soft); }
  .workload-name > i.stopped { background: var(--color-faint); box-shadow: none; }
  .workload-name > span { min-width: 0; display: grid; gap: 4px; }
  .workload-name strong, .workload-name small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .workload-name strong { font-size: 11px; } .workload-name small { color: var(--color-muted); font: 8px var(--font-mono); }
  .workload-value { min-width: 0; display: grid; gap: 5px; }
  .workload-value strong { font: 10px var(--font-mono); font-weight: 500; }
  .workload-value > i { height: 3px; overflow: hidden; border-radius: 3px; background: var(--color-paper-subtle); }
  .workload-value u { display: block; height: 100%; background: var(--color-accent); }
  .workload-value small, .workload-pair { color: var(--color-muted); font: 8px var(--font-mono); }
  .workload-pair { min-width: 0; display: grid; gap: 4px; }
  .service-head-actions { display: flex !important; grid-auto-flow: column; align-items: center; gap: var(--space-2) !important; }
  .service-head-actions b { min-width: 30px; height: 30px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); font: 12px var(--font-mono); }
  .service-head-actions button { min-height: 36px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: 12px; font-weight: 600; cursor: pointer; }
  .service-head-actions .service-add-primary { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .services article { min-height: 76px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto auto; align-items: center; gap: var(--space-3); }
  .service-icon, .empty-icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); }
  .services article > div { min-width: 0; display: grid; gap: var(--space-1); }
  .services article strong { font-size: 14px; }
  .services article small { overflow: hidden; color: var(--color-muted); font: 11px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .service-icon.database { background: color-mix(in oklch, var(--color-info) 10%, var(--color-paper-subtle)); color: var(--color-info); }
  .service-icon.application { background: var(--color-accent-soft); color: var(--color-accent); }
  .application-service-row { border-top: 1px solid var(--color-rule); }
  .application-service-actions { display: flex !important; grid-auto-flow: column; gap: var(--space-1) !important; }
  .application-service-actions button { min-height: 34px; padding: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-ink); font-size: 11px; font-weight: 600; cursor: pointer; }
  .application-service-actions .danger-text { color: var(--color-danger); }
  .database-row { border-top: 1px solid var(--color-rule); }
  .database-state { justify-items: end; gap: var(--space-1) !important; }
  .database-state em { color: var(--color-muted); font: 9px var(--font-mono); font-style: normal; text-transform: uppercase; letter-spacing: .05em; }
  .database-state em.public { color: var(--color-warning); }
  .database-actions { display: flex !important; grid-auto-flow: column; gap: var(--space-1) !important; }
  .database-actions button { min-height: 34px; padding: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-ink); font-size: 11px; font-weight: 600; cursor: pointer; }
  .database-actions .danger-text { color: var(--color-danger); }
  .database-manager > header { min-height: 74px; }
  .database-manager-list { padding: var(--space-4); display: grid; gap: var(--space-4); }
  .database-manager-card { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); }
  .database-card-heading { min-height: 68px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .database-card-heading > div { display: grid; gap: 3px; } .database-card-heading strong { font-size: 13px; } .database-card-heading small { color: var(--color-muted); font: 10px var(--font-mono); }
  .database-manager-card dl { margin: 0; padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--space-6); }
  .database-manager-card dl > div { min-height: 48px; display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .database-manager-card dt { color: var(--color-muted); font-size: 10px; } .database-manager-card dd { margin: 0; min-width: 0; font-size: 11px; }
  .database-manager-card dd code { display: block; overflow: hidden; font: 10px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .database-manager-card dd span { color: var(--color-accent); font: 9px var(--font-mono); text-transform: uppercase; } .database-manager-card dd span.public { color: var(--color-warning); }
  .database-card-actions { padding: var(--space-3) var(--space-4); display: flex; flex-wrap: wrap; gap: var(--space-2); }
  .database-card-actions button { min-height: 36px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: 11px; font-weight: 600; cursor: pointer; }
  .database-card-actions .delete-database { margin-left: auto; border-color: color-mix(in oklch, var(--color-danger) 35%, var(--color-rule)); color: var(--color-danger); }
  @media (max-width: 46rem) { .database-manager-card dl { grid-template-columns: 1fr; } .database-card-actions .delete-database { margin-left: 0; } }
  .database-logs-modal { width: min(1040px, calc(100vw - 32px)); }
  .database-log-toolbar { padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .database-log-tabs, .database-log-actions { display: flex; align-items: center; gap: var(--space-2); }
  .database-log-tabs button, .database-log-actions button, .database-log-actions select { min-height: 34px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); cursor: pointer; }
  .database-log-tabs button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); }
  .database-log-tabs span { margin-left: 4px; color: var(--color-muted); font: 10px var(--font-mono); }
  .database-log-actions label { display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); font-size: 11px; }
  .database-log-console { height: min(55vh, 560px); padding: var(--space-3) 0; overflow: auto; background: #0d1117; color: #d6deeb; font: 12px/1.65 var(--font-mono); }
  .database-log-line { padding: 2px var(--space-4); display: grid; grid-template-columns: 82px 72px minmax(0, 1fr); gap: var(--space-3); border-left: 2px solid transparent; }
  .database-log-line:hover { background: #ffffff0a; }
  .database-log-line time { color: #768390; }
  .database-log-line b { color: #8b949e; text-transform: uppercase; }
  .database-log-line.info { border-color: #3fb950; } .database-log-line.info b { color: #56d364; }
  .database-log-line.warning { border-color: #d29922; } .database-log-line.warning b { color: #e3b341; }
  .database-log-line.error { border-color: #f85149; background: #f8514908; } .database-log-line.error b { color: #ff7b72; }
  .database-log-line.debug { border-color: #58a6ff; } .database-log-line.debug b { color: #79c0ff; }
  .database-log-error { padding: var(--space-3) var(--space-5); background: color-mix(in oklch, var(--color-danger) 10%, var(--color-paper-raised)); color: var(--color-danger); font-size: 12px; }
  .log-empty { padding: var(--space-6); color: #8b949e; text-align: center; }
  .live-indicator { margin-right: auto; color: var(--color-muted); font: 11px var(--font-mono); } .live-indicator i { width: 7px; height: 7px; margin-right: 6px; display: inline-block; border-radius: 50%; background: var(--color-accent); box-shadow: 0 0 0 4px var(--color-accent-soft); }
  .database-delete-modal form { padding: var(--space-5); }
  .volume-choice { margin: var(--space-4) 0; padding: var(--space-4); display: flex; align-items: flex-start; gap: var(--space-3); border: 1px solid color-mix(in oklch, var(--color-danger) 30%, var(--color-rule)); border-radius: var(--radius-md); }
  .volume-choice span { display: grid; gap: var(--space-1); } .volume-choice small { color: var(--color-muted); line-height: 1.5; }
  dl { margin: 0; }
  .runtime-facts dl, .detail-list { padding: var(--space-2) var(--space-5); }
  dl > div { min-height: 50px; display: grid; grid-template-columns: minmax(120px, .7fr) minmax(0, 1fr); align-items: center; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  dl > div:last-child { border-bottom: 0; }
  dt { color: var(--color-muted); font-size: 13px; }
  dd { margin: 0; overflow-wrap: anywhere; text-align: right; font: 12px var(--font-mono); }
  .empty { min-height: 190px; padding: var(--space-8) var(--space-5); display: flex; align-items: center; gap: var(--space-4); }
  .empty-icon { flex: 0 0 auto; font: 600 9px var(--font-mono); }
  .empty h4 { margin: 0 0 var(--space-2); font-size: 15px; }
  .empty p { max-width: 60ch; margin: 0; color: var(--color-muted); font-size: 13px; line-height: 1.55; }
  .recent > a, .deployment-row { min-height: 68px; padding: 0 var(--space-5); display: grid; grid-template-columns: 110px minmax(0, 1fr) 60px 170px 20px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; }
  .recent > a:last-child, .deployment-row:last-child { border-bottom: 0; }
  .recent a div, .deployment-row div { min-width: 0; display: grid; gap: var(--space-1); }
  .recent a strong, .deployment-row strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
  .recent a small, .deployment-row small, .recent code, .deployment-row code, .recent time, .deployment-row time { overflow: hidden; color: var(--color-muted); font: 11px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .compact-empty { padding: var(--space-8) var(--space-5); color: var(--color-muted); font-size: 13px; }
  .settings-panel, .deployment-panel { min-height: 330px; }
  .environment-panel { min-height: 430px; }
  .environment-header-actions { display: flex !important; grid-auto-flow: column; align-items: center; gap: var(--space-2) !important; }
  .add-variable, .bulk-variable { min-height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .bulk-variable { background: transparent; color: var(--color-accent); }
  .add-variable:hover, .bulk-variable:hover { border-color: var(--color-accent); color: var(--color-accent); }
  .environment-intro { margin: var(--space-5); padding: var(--space-4); display: grid; grid-template-columns: 34px minmax(0, 1fr); gap: var(--space-3); border: 1px solid color-mix(in oklch, var(--color-info) 26%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in oklch, var(--color-info) 6%, var(--color-paper-raised)); }
  .restart-mark { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 50%; background: color-mix(in oklch, var(--color-info) 13%, var(--color-paper-raised)); color: var(--color-info); font: 18px var(--font-mono); }
  .environment-intro strong { font-size: 12px; }
  .environment-intro p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: 11px; line-height: 1.55; }
  .environment-feedback { margin: 0 var(--space-5) var(--space-4); padding: var(--space-3); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); font-size: 11px; }
  .environment-feedback span { color: var(--color-muted); }
  .environment-feedback.error { border-color: color-mix(in oklch, var(--color-danger) 36%, var(--color-rule)); background: color-mix(in oklch, var(--color-danger) 7%, var(--color-paper-raised)); }
  .environment-feedback.success { border-color: color-mix(in oklch, var(--color-accent) 36%, var(--color-rule)); background: var(--color-accent-soft); }
  .environment-loading { min-height: 240px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); font-size: 12px; }
  .environment-columns, .variable-row { display: grid; grid-template-columns: minmax(180px, .8fr) minmax(260px, 1.5fr) 72px 34px; gap: var(--space-2); }
  .environment-columns { padding: 0 var(--space-5) var(--space-2); color: var(--color-muted); font: 9px var(--font-mono); text-transform: uppercase; letter-spacing: .09em; }
  .variable-list { padding: 0 var(--space-5) var(--space-5); display: grid; gap: var(--space-2); }
  .variable-row { align-items: center; }
  .variable-row input[type='text'], .variable-row input[type='password'] { width: 100%; height: 42px; padding: 0 var(--space-3); outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-log-bg); color: var(--color-log-text); font: 11px var(--font-mono); }
  .variable-row input:focus-visible { outline-color: var(--color-focus); }
  .value-field { position: relative; }
  .value-field input { padding-right: 52px !important; }
  .value-field > button { position: absolute; top: 7px; right: 7px; height: 28px; padding: 0 var(--space-2); border: 1px solid var(--color-log-rule); border-radius: 4px; background: var(--color-log-surface); color: var(--color-log-muted); font: 9px var(--font-mono); cursor: pointer; }
  .secret-toggle { min-height: 42px; display: flex; align-items: center; justify-content: center; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); color: var(--color-muted); font-size: 10px; cursor: pointer; }
  .secret-toggle input { accent-color: var(--color-accent); }
  .remove-variable { width: 34px; height: 34px; border: 1px solid transparent; border-radius: 50%; background: transparent; color: var(--color-muted); font-size: 18px; cursor: pointer; }
  .remove-variable:hover { border-color: color-mix(in oklch, var(--color-danger) 36%, var(--color-rule)); color: var(--color-danger); }
  .environment-editor > footer { min-height: 74px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .environment-editor > footer > div { display: grid; gap: 3px; }
  .environment-editor > footer strong { font-size: 11px; }
  .environment-editor > footer span { color: var(--color-muted); font-size: 10px; }
  .environment-editor > footer button { min-height: 40px; padding: 0 var(--space-4); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .environment-editor > footer button:disabled { opacity: .55; cursor: wait; }
  .bulk-environment-modal { width: min(760px, 100%); }
  .bulk-environment-body { padding: var(--space-5); }
  .bulk-environment-body > p { margin: 0 0 var(--space-5); color: var(--color-muted); font-size: 12px; line-height: 1.65; }
  .bulk-environment-body code { font-family: var(--font-mono); }
  .bulk-environment-body > label { display: block; margin-bottom: var(--space-2); font-size: 11px; font-weight: 700; }
  .bulk-environment-body textarea { width: 100%; min-height: 330px; padding: var(--space-4); resize: vertical; outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-log-rule); border-radius: var(--radius-md); background: var(--color-log-bg); color: var(--color-log-text); caret-color: var(--color-info); font: 11px/1.75 var(--font-mono); tab-size: 2; }
  .bulk-environment-body textarea:focus-visible { outline-color: var(--color-focus); }
  .bulk-environment-body textarea::placeholder { color: var(--color-log-muted); }
  .bulk-environment-body > small { margin-top: var(--space-2); display: block; color: var(--color-muted); font-size: 10px; }
  .domain-panel { min-height: 430px; }
  .modal-backdrop { position: fixed; z-index: 100; inset: 0; padding: var(--space-5); display: grid; place-items: center; overflow-y: auto; background: color-mix(in oklch, #07100b 68%, transparent); backdrop-filter: blur(5px); }
  .modal { width: min(680px, 100%); max-height: calc(100vh - 40px); overflow: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); color: var(--color-ink); box-shadow: 0 28px 90px color-mix(in oklch, #07100b 38%, transparent); }
  .modal > header { min-height: 78px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .modal > header div { display: grid; gap: var(--space-1); }
  .modal > header span { color: var(--color-accent); font: 10px var(--font-mono); text-transform: uppercase; letter-spacing: .1em; }
  .modal h2 { margin: 0; font-size: 19px; letter-spacing: -.025em; }
  .modal > header > button { width: 34px; height: 34px; border: 1px solid var(--color-rule); border-radius: 50%; background: transparent; color: var(--color-muted); cursor: pointer; font-size: 20px; }
  .database-modal form { padding: var(--space-5); }
  .engine-picker { margin-bottom: var(--space-5); display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-2); }
  .engine-picker button { min-height: 92px; padding: var(--space-3); display: grid; grid-template-columns: 32px minmax(0, 1fr); grid-template-rows: auto auto; align-items: center; gap: 0 var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-ink); text-align: left; cursor: pointer; }
  .engine-picker button.active { border-color: color-mix(in oklch, var(--color-accent) 52%, var(--color-rule)); background: var(--color-accent-soft); box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-accent) 16%, transparent); }
  .engine-mark { width: 32px; height: 32px; grid-row: 1 / 3; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-accent); font: 10px var(--font-mono); }
  .engine-picker strong { font-size: 12px; }
  .engine-picker small { color: var(--color-muted); font: 9px var(--font-mono); }
  .form-grid { display: grid; gap: var(--space-4); }
  .form-grid .wide { grid-column: 1 / -1; }
  .form-grid label, .port-field { display: grid; gap: var(--space-2); }
  .form-grid label > span, .port-field > span { font-size: 11px; font-weight: 600; }
  .form-grid label em { color: var(--color-muted); font-size: 10px; font-style: normal; font-weight: 400; }
  .form-grid input, .form-grid select, .port-field input { width: 100%; height: 42px; padding: 0 var(--space-3); outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 12px; }
  .form-grid input:focus-visible, .form-grid select:focus-visible, .port-field input:focus-visible { outline-color: var(--color-focus); }
  .repository-search { position: relative; }
  .repository-combobox { position: relative; }
  .repository-combobox input { padding-right: 34px; }
  .repository-combobox::after { position: absolute; top: 14px; right: 13px; width: 7px; height: 7px; border-right: 1.5px solid var(--color-muted); border-bottom: 1.5px solid var(--color-muted); content: ''; pointer-events: none; transform: rotate(45deg); transition: transform .15s ease; }
  .repository-combobox.open::after { top: 18px; transform: rotate(225deg); }
  .repository-results { position: absolute; z-index: 10; top: calc(100% + 6px); right: 0; left: 0; max-height: 238px; overflow: auto; padding: var(--space-1); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); box-shadow: var(--shadow-popover); }
  .repository-results button { width: 100%; min-height: 36px; padding: 0 var(--space-2); display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); border: 0; border-radius: calc(var(--radius-sm) - 2px); background: transparent; color: var(--color-ink); font: 11px var(--font-mono); text-align: left; cursor: pointer; }
  .repository-results button:hover, .repository-results button[aria-selected="true"] { background: var(--color-accent-soft); }
  .repository-results button span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .repository-results button small { flex: none; color: var(--color-muted); font: 700 9px var(--font-mono); letter-spacing: .05em; text-transform: uppercase; }
  .repository-results p { margin: 0; padding: var(--space-3); color: var(--color-muted); font-size: 11px; }
  .application-service-modal { width: min(720px, calc(100vw - 32px)); }
  .application-service-modal form { padding: var(--space-5); }
  .application-service-modal form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .application-service-modal form > footer > span { margin-right: auto; color: var(--color-muted); font-size: 10px; }
  .service-source-picker { margin: 0 0 var(--space-5); padding: 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); border: 0; }
  .service-source-picker legend { margin-bottom: var(--space-2); color: var(--color-muted); font: 10px var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
  .service-source-picker button { min-height: 76px; padding: var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) 14px; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-ink); text-align: left; cursor: pointer; }
  .service-source-picker button:hover { border-color: color-mix(in oklch, var(--color-accent) 36%, var(--color-rule)); }
  .service-source-picker button.active { border-color: var(--color-accent); background: var(--color-accent-soft); box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-accent) 20%, transparent); }
  .service-source-picker .source-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); }
  .service-source-picker button.active .source-icon { color: var(--color-accent); }
  .service-source-picker button > span:nth-child(2) { min-width: 0; display: grid; gap: 3px; }
  .service-source-picker strong { font-size: 12px; }
  .service-source-picker small { overflow: hidden; color: var(--color-muted); font-size: 10px; line-height: 1.35; text-overflow: ellipsis; }
  .service-source-picker i { width: 12px; height: 12px; border: 1px solid var(--color-rule); border-radius: 50%; background: var(--color-paper-raised); }
  .service-source-picker button.active i { border: 3px solid var(--color-paper-raised); background: var(--color-accent); box-shadow: 0 0 0 1px var(--color-accent); }
  .source-empty-note { min-height: 58px; padding: var(--space-3); display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); }
  .source-empty-note > span { display: grid; gap: 2px; }
  .source-empty-note strong { color: var(--color-ink); font-size: 11px; }
  .source-empty-note small { font-size: 10px; }
  .source-empty-note a { color: var(--color-accent); font-size: 10px; font-weight: 700; }
  .source-empty-note.permission-note { border-color: color-mix(in oklch, var(--color-warning) 46%, var(--color-rule)); background: color-mix(in oklch, var(--color-warning) 8%, var(--color-paper-raised)); color: var(--color-warning); }
  .source-empty-note.permission-note a { color: var(--color-warning); }
  .field-help { color: var(--color-muted); font-size: 10px; }
  .service-template-note { margin-bottom: var(--space-4); padding: var(--space-4); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); }
  .service-template-note strong { font-size: 12px; } .service-template-note span { color: var(--color-muted); font-size: 11px; }
  .service-environment textarea { min-height: 112px; padding: var(--space-3); resize: vertical; outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-log-bg); color: var(--color-log-text); font: 11px/1.6 var(--font-mono); }
  .service-environment textarea:focus-visible { outline-color: var(--color-focus); } .service-environment small { color: var(--color-muted); font-size: 10px; }
  .command-field input { background: var(--color-log-bg); color: var(--color-log-text); font-family: var(--font-mono); }
  .command-field small { color: var(--color-muted); font-size: 10px; line-height: 1.5; }
  .command-field code { color: var(--color-accent); font-family: var(--font-mono); }
  .exposure-choice { margin-top: var(--space-5); padding: var(--space-4); display: grid; grid-template-columns: 18px minmax(0, 1fr); align-items: start; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); cursor: pointer; }
  .exposure-choice input { margin-top: 2px; accent-color: var(--color-accent); }
  .exposure-choice span { display: grid; gap: var(--space-1); }
  .exposure-choice strong { font-size: 12px; }
  .exposure-choice small { color: var(--color-muted); font-size: 11px; line-height: 1.5; }
  .exposure-choice code { font-family: var(--font-mono); }
  .database-modal .port-field { margin-top: var(--space-4); }
  .port-field small { color: var(--color-muted); font: 10px var(--font-mono); }
  .modal footer { padding: var(--space-4) var(--space-5); display: flex; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); }
  .database-modal form > footer { margin: var(--space-5) calc(-1 * var(--space-5)) calc(-1 * var(--space-5)); }
  .modal footer button { min-height: 40px; padding: 0 var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 12px; font-weight: 600; cursor: pointer; }
  .modal footer .primary { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .modal-body { padding: var(--space-5); }
  .warning-note { margin-bottom: var(--space-5); padding: var(--space-4); border: 1px solid color-mix(in oklch, var(--color-warning) 35%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in oklch, var(--color-warning) 7%, var(--color-paper-raised)); }
  .warning-note strong { font-size: 12px; }
  .warning-note p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: 11px; line-height: 1.55; }
  .exposure-modal { width: min(480px, 100%); }
  .credentials-modal { width: min(760px, 100%); }
  .credential-loading { min-height: 220px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); font-size: 12px; }
  .credential-list { padding: var(--space-2) var(--space-5); }
  .credential-list > div { min-height: 58px; display: grid; grid-template-columns: 112px minmax(0, 1fr) 54px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .credential-list > div:last-child { border-bottom: 0; }
  .credential-list span { color: var(--color-muted); font-size: 11px; }
  .credential-list code { overflow: hidden; font: 11px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .credential-list button { min-height: 30px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-accent); font-size: 10px; cursor: pointer; }
  .secret-note { margin: var(--space-2) var(--space-5) var(--space-5); padding: var(--space-3); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); font-size: 10px; line-height: 1.5; }
  .route-state { display: inline-flex !important; grid-auto-flow: column; align-items: center; gap: var(--space-2) !important; color: var(--color-accent) !important; font-weight: 600; }
  .route-state i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-accent); box-shadow: 0 0 0 4px var(--color-accent-soft); }
  .domain-layout { display: grid; }
  .domain-form { padding: var(--space-6) var(--space-5); }
  .domain-editor-head { margin-bottom: var(--space-5); display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); }
  .domain-editor-head .form-copy { min-width: 0; }
  .domain-editor-head .form-copy p { margin-bottom: 0; }
  .add-domain { min-height: 38px; padding: 0 var(--space-4); flex: 0 0 auto; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .domain-empty { width: 100%; min-height: 160px; display: grid; place-items: center; align-content: center; gap: var(--space-2); border: 1px dashed var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-ink); cursor: pointer; }
  .domain-empty > span { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent-soft); color: var(--color-accent); font-size: 20px; }
  .domain-empty strong { font-size: 13px; } .domain-empty small { color: var(--color-muted); font-size: 11px; }
  .domain-binding-list { display: grid; gap: var(--space-4); }
  .domain-binding { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); }
  .domain-binding > .binding-head { min-height: auto; padding: var(--space-4); display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: end; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .binding-domain { min-width: 0; display: grid; gap: var(--space-2); }
  .binding-domain label { color: var(--color-muted); font-size: 10px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; }
  .binding-domain input { width: 100%; min-width: 0; height: 40px; padding: 0 var(--space-3); outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font: 12px var(--font-mono); }
  .binding-domain input:focus-visible { outline-color: var(--color-focus); }
  .binding-https { height: 40px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); color: var(--color-muted); font-size: 11px; font-weight: 600; cursor: pointer; }
  .binding-https input { accent-color: var(--color-accent); }
  .remove-domain { height: 40px; padding: 0 var(--space-3); border: 1px solid color-mix(in oklch, var(--color-danger) 25%, var(--color-rule)); border-radius: var(--radius-sm); background: transparent; color: var(--color-danger); font-size: 11px; font-weight: 600; cursor: pointer; }
  .binding-rules-head { padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); }
  .binding-rules-head > div { display: grid; gap: 2px; } .binding-rules-head strong { font-size: 11px; } .binding-rules-head small { color: var(--color-muted); font-size: 10px; }
  .binding-rules-head button { min-height: 30px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-accent); font-size: 10px; font-weight: 700; cursor: pointer; }
  .binding-rules { padding: 0 var(--space-4) var(--space-4); }
  .binding-rules .route-rule { background: var(--color-paper-raised); }
  .editable-rule > button:disabled { color: var(--color-muted); opacity: .35; cursor: not-allowed; }
  .binding-preview { padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: minmax(110px, auto) minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .binding-preview > span { overflow: hidden; font: 10px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .binding-preview code { overflow: hidden; color: var(--color-muted); font: 10px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .binding-preview a { color: var(--color-accent); font-size: 10px; font-weight: 700; text-decoration: none; }
  .form-copy h4 { margin: 0 0 var(--space-2); font-size: 16px; }
  .form-copy p { max-width: 62ch; margin: 0 0 var(--space-6); color: var(--color-muted); font-size: 13px; line-height: 1.6; }
  .route-rule { min-height: 50px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: minmax(110px, 1fr) 24px minmax(150px, 1.1fr) minmax(150px, .9fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-bottom: 0; background: var(--color-paper-subtle); }
  .route-rule:first-of-type { border-radius: var(--radius-sm) var(--radius-sm) 0 0; } .route-rule:last-of-type { border-bottom: 1px solid var(--color-rule); border-radius: 0 0 var(--radius-sm) var(--radius-sm); }
  .route-rule > span { color: var(--color-muted); text-align: center; } .route-rule code { overflow: hidden; font: 11px var(--font-mono); text-overflow: ellipsis; }
  .editable-rule input { min-width: 0; height: 34px; padding: 0 var(--space-2); outline: none; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: 11px var(--font-mono); }
  .port-input { display: grid; grid-template-columns: auto 1fr; align-items: center; gap: var(--space-2); } .port-input small { color: var(--color-muted); font-size: 9px; text-transform: uppercase; }
  .service-target { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: var(--space-2); } .service-target small { color: var(--color-muted); font-size: 9px; text-transform: uppercase; }
  .service-target select { min-width: 0; height: 34px; padding: 0 var(--space-2); outline: none; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: 11px var(--font-mono); }
  .editable-rule > button { width: 30px; height: 30px; border: 0; background: transparent; color: var(--color-danger); font-size: 18px; cursor: pointer; }
  .route-hint { margin: var(--space-2) 0 0; color: var(--color-muted); font-size: 10px; }
  .route-rules-footer { margin-top: var(--space-4); padding-top: var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px dashed var(--color-rule); }
  .route-rules-footer span { color: var(--color-muted); font-size: 10px; }
  .route-rules-footer button { min-height: 38px; padding: 0 var(--space-4); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: 12px; font-weight: 700; cursor: pointer; }
  .domain-save-footer { margin-top: var(--space-5); padding-top: var(--space-4); border-top-style: solid; }
  .domain-form code, .route-guide code { font-family: var(--font-mono); }
  .domain-feedback { margin-bottom: var(--space-4); padding: var(--space-3); display: grid; gap: var(--space-1); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); font-size: 12px; }
  .domain-feedback span { color: var(--color-muted); }
  .domain-feedback.error { border-color: color-mix(in oklch, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in oklch, var(--color-danger) 7%, var(--color-paper-raised)); }
  .domain-feedback.success { border-color: color-mix(in oklch, var(--color-accent) 35%, var(--color-rule)); background: var(--color-accent-soft); }
  .route-guide { padding: var(--space-6) var(--space-5); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .guide-label { color: var(--color-muted); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: .08em; }
  .route-guide ol { margin: var(--space-5) 0 0; padding: 0; display: grid; gap: var(--space-5); list-style: none; }
  .route-guide li { display: grid; grid-template-columns: 28px minmax(0, 1fr); gap: var(--space-3); }
  .route-guide li > b { width: 28px; height: 28px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; background: var(--color-paper-raised); font: 11px var(--font-mono); }
  .route-guide strong { font-size: 13px; }
  .route-guide p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: 12px; line-height: 1.55; }
  .routing-example { margin-top: var(--space-6); padding: var(--space-4); display: grid; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .routing-example span { color: var(--color-muted); font-size: 9px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
  .routing-example code { overflow: hidden; font: 10px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .log-panel { min-height: 430px; }
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
  .log-actions { display: flex !important; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: var(--space-2) !important; }
  .log-actions small { color: var(--color-muted); font: 11px var(--font-mono); white-space: nowrap; }
  .log-actions button { min-height: 38px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 12px; font-weight: 600; cursor: pointer; }
  .log-actions button.copied { border-color: color-mix(in oklch, var(--color-accent) 55%, var(--color-rule)); color: var(--color-accent); }
  .line-limit { min-height: 38px; padding-left: var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); }
  .line-limit span { font-size: 11px !important; font-weight: 600; }
  .line-limit select { height: 36px; padding: 0 28px 0 var(--space-1); border: 0; outline: 0; background: transparent; color: var(--color-ink); font: 11px var(--font-mono); cursor: pointer; }
  .live-toggle { display: inline-flex; align-items: center; gap: var(--space-2); }
  .live-toggle i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-faint); }
  .live-toggle.live { border-color: color-mix(in oklch, var(--color-accent) 38%, var(--color-rule)); color: var(--color-accent); }
  .live-toggle.live i { background: var(--color-accent); box-shadow: 0 0 0 4px color-mix(in oklch, var(--color-accent) 14%, transparent); animation: live-pulse 1.8s ease-out infinite; }
  .log-toolbar { min-height: 58px; padding: var(--space-2) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .severity-filters { display: flex; align-items: center; gap: var(--space-1); overflow-x: auto; }
  .severity-filters button { min-height: 34px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: var(--space-1); border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: 11px; font-weight: 600; white-space: nowrap; cursor: pointer; }
  .severity-filters button span { min-width: 19px; height: 19px; padding: 0 var(--space-1); display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); font: 10px var(--font-mono); }
  .severity-filters button.active { border-color: var(--color-rule); background: var(--color-paper-subtle); color: var(--color-ink); }
  .severity-filters .debug.active { color: var(--color-debug); }
  .severity-filters .info.active { color: var(--color-info); }
  .severity-filters .warning.active { color: var(--color-warning); }
  .severity-filters .error.active { color: var(--color-danger); }
  .log-toolbar label { width: min(230px, 32vw); flex: 0 0 auto; }
  .log-toolbar input { width: 100%; height: 36px; padding: 0 var(--space-3); outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 12px; }
  .log-toolbar input:focus-visible { outline-color: var(--color-focus); }
  .terminal-head { min-height: 42px; padding: 0 var(--space-4); display: grid; grid-template-columns: 9px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .terminal-head > span { width: 8px; height: 8px; border-radius: 50%; background: var(--color-accent); }
  .terminal-head strong, .terminal-head small { font: 11px var(--font-mono); }
  .terminal-head small { color: var(--color-muted); }
  .log-console { min-height: 330px; max-height: 62vh; overflow: auto; background: var(--color-log-bg); color: var(--color-log-text); font-family: var(--font-mono); }
  .log-line { --severity-color: var(--color-info); min-height: 34px; padding: var(--space-1) var(--space-3) var(--space-1) 0; display: grid; grid-template-columns: 44px 94px 66px minmax(0, 1fr); align-items: start; border-bottom: 1px solid var(--color-log-rule); box-shadow: inset 2px 0 var(--severity-color); }
  .log-line.debug { --severity-color: var(--color-debug); }
  .log-line.info { --severity-color: var(--color-info); }
  .log-line.warning { --severity-color: var(--color-warning); background: color-mix(in oklch, var(--color-warning) 7%, var(--color-log-bg)); }
  .log-line.error { --severity-color: var(--color-danger); background: color-mix(in oklch, var(--color-danger) 9%, var(--color-log-bg)); }
  .line-number { padding-top: 3px; color: var(--color-log-muted); text-align: right; font-size: 10px; user-select: none; }
  .log-line time { padding: 3px var(--space-3) 0; color: var(--color-log-muted); font-size: 10px; white-space: nowrap; }
  .severity { width: fit-content; margin-top: 1px; padding: 2px var(--space-2); border: 1px solid color-mix(in oklch, var(--severity-color) 45%, transparent); border-radius: var(--radius-sm); background: color-mix(in oklch, var(--severity-color) 12%, transparent); color: var(--severity-color); font-size: 9px; font-weight: 500; line-height: 1.5; text-transform: uppercase; }
  .log-line code { padding-top: 1px; overflow-wrap: anywhere; color: var(--color-log-text); font: 11px/1.7 var(--font-mono); }
  .filtered-empty { min-height: 330px; display: grid; place-content: center; gap: var(--space-1); background: var(--color-log-bg); color: var(--color-log-text); text-align: center; }
  .filtered-empty span { color: var(--color-log-muted); font-size: 12px; }
  .log-state { min-height: 330px; padding: var(--space-8); display: flex; align-items: center; justify-content: center; gap: var(--space-4); }
  .log-state h4, .log-state p { margin: 0; }
  .log-state h4 { font-size: 15px; }
  .log-state p { margin-top: var(--space-1); color: var(--color-muted); font-size: 13px; }
  .settings-stack { display: grid; gap: var(--space-4); }
  .project-editor { min-height: 0; }
  .project-editor > header code { color: var(--color-muted); font-size: 10px; }
  .project-editor > form { padding: var(--space-6) var(--space-5); }
  .settings-intro { margin-bottom: var(--space-5); }
  .settings-intro h4 { margin: 0 0 var(--space-2); font-size: 16px; }
  .settings-intro p { max-width: 68ch; margin: 0; color: var(--color-muted); font-size: 12px; line-height: 1.6; }
  .settings-field { display: grid; gap: var(--space-2); color: var(--color-ink); font-size: 11px; font-weight: 700; }
  .settings-field span { display: flex; align-items: center; gap: var(--space-1); }
  .settings-field em { color: var(--color-muted); font-style: normal; font-weight: 500; }
  .settings-field input, .settings-field select, .confirm-field input { width: 100%; height: 42px; padding: 0 var(--space-3); outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font: 11px var(--font-mono); }
  .settings-field input:focus-visible, .settings-field select:focus-visible, .confirm-field input:focus-visible { outline-color: var(--color-focus); border-color: var(--color-focus); }
  .build-pack-help { min-height: 78px; padding: var(--space-3); display: grid; align-content: center; gap: 5px; border: 1px dashed color-mix(in srgb, var(--color-accent) 45%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-ink); }
  .build-pack-help strong { font-size: 10px; }
  .build-pack-help small { color: var(--color-muted); font-size: 9px; line-height: 1.5; }
  .build-pack-help code { color: var(--color-accent); font: 9px var(--font-mono); }
  .source-choice { margin: var(--space-5) 0; padding: 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-2); border: 0; }
  .source-choice legend { margin-bottom: var(--space-2); color: var(--color-ink); font-size: 11px; font-weight: 700; }
  .source-choice button { min-height: 64px; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); text-align: left; cursor: pointer; }
  .source-choice button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-accent) 18%, transparent); }
  .source-choice button span { display: grid; gap: 3px; }
  .source-choice strong { color: var(--color-ink); font-size: 11px; }
  .source-choice small { font-size: 10px; }
  .settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
  .settings-grid .wide { grid-column: 1 / -1; }
  .field-note, .repository-note { min-height: 42px; align-self: end; display: grid; align-content: center; gap: 2px; color: var(--color-muted); font-size: 10px; }
  .field-note strong, .repository-note strong { color: var(--color-ink); }
  .field-note a { color: var(--color-accent); text-decoration: none; }
  .repository-note { grid-column: 1 / -1; padding: var(--space-3); border: 1px solid color-mix(in oklch, var(--color-warning) 34%, var(--color-rule)); border-radius: var(--radius-sm); background: color-mix(in oklch, var(--color-warning) 8%, var(--color-paper-raised)); }
  .project-editor form > footer { margin: var(--space-6) calc(var(--space-5) * -1) calc(var(--space-6) * -1); padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .project-editor form > footer > span { color: var(--color-muted); font: 10px var(--font-mono); }
  .save-settings { min-height: 38px; padding: 0 var(--space-4); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .save-settings:disabled { opacity: .55; cursor: wait; }
  .runtime-settings-panel > header { align-items: center; }
  .runtime-settings-panel > header > p { max-width: 52ch; margin: 0 0 0 auto; color: var(--color-muted); font-size: 10px; line-height: 1.55; text-align: right; }
  .runtime-service-tabs { padding: var(--space-3) var(--space-5); display: flex; align-items: stretch; gap: var(--space-2); overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .runtime-service-tabs > button { min-width: 210px; min-height: 58px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-muted); text-align: left; cursor: pointer; }
  .runtime-service-tabs > button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-accent) 18%, transparent); }
  .runtime-service-tabs button > span:nth-child(2) { min-width: 0; display: grid; gap: 3px; }
  .runtime-service-tabs strong, .runtime-service-tabs small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .runtime-service-tabs strong { color: var(--color-ink); font-size: 11px; }
  .runtime-service-tabs small { font: 9px var(--font-mono); }
  .runtime-settings-form { padding: var(--space-5); }
  .runtime-settings-copy { margin-bottom: var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); }
  .runtime-settings-copy span { color: var(--color-accent); font: 700 9px var(--font-mono); text-transform: uppercase; letter-spacing: .1em; }
  .runtime-settings-copy h4 { margin: 4px 0 0; font-size: 16px; }
  .runtime-settings-copy code { color: var(--color-muted); font-size: 9px; }
  .runtime-settings-grid { margin-top: var(--space-4); }
  .runtime-command-field input { border-color: color-mix(in oklch, var(--color-accent) 28%, var(--color-rule-strong)); background: var(--color-log-bg); color: var(--color-log-text); }
  .runtime-command-field small { color: var(--color-muted); font-size: 10px; font-weight: 500; line-height: 1.55; }
  .runtime-command-field code { color: var(--color-accent); }
  .runtime-apply-note { margin-top: var(--space-4); padding: var(--space-3); display: flex; align-items: flex-start; gap: var(--space-3); border: 1px solid color-mix(in oklch, var(--color-accent) 24%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-soft); color: var(--color-accent); }
  .runtime-apply-note div { display: grid; gap: 3px; }
  .runtime-apply-note strong { color: var(--color-ink); font-size: 10px; }
  .runtime-apply-note span { color: var(--color-muted); font-size: 10px; line-height: 1.5; }
  .deployment-triggers { margin-top: var(--space-4); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .deployment-triggers > header { min-height: 58px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .deployment-triggers > header > div { display: grid; gap: 3px; }
  .deployment-triggers > header span { color: var(--color-accent); font: 700 8px var(--font-mono); text-transform: uppercase; letter-spacing: .12em; }
  .deployment-triggers > header h4 { margin: 0; color: var(--color-ink); font-size: 13px; }
  .deployment-triggers > header b { padding: 5px 8px; border: 1px solid var(--color-rule-strong); border-radius: 999px; color: var(--color-muted); font: 700 8px var(--font-mono); text-transform: uppercase; letter-spacing: .07em; }
  .trigger-loading { min-height: 92px; padding: var(--space-4); display: flex; align-items: center; justify-content: center; gap: var(--space-2); color: var(--color-muted); font-size: 10px; }
  .trigger-row { min-height: 78px; padding: var(--space-4); display: grid; grid-template-columns: 36px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); }
  .trigger-icon { width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid color-mix(in oklch, var(--color-accent) 22%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); }
  .trigger-row > div { min-width: 0; display: grid; gap: 4px; }
  .trigger-row strong { color: var(--color-ink); font-size: 11px; }
  .trigger-row strong code { color: var(--color-accent); font: 10px var(--font-mono); }
  .trigger-row small { color: var(--color-muted); font-size: 10px; line-height: 1.5; }
  .switch { display: grid; grid-template-columns: 34px 20px; align-items: center; gap: 7px; cursor: pointer; }
  .switch input { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }
  .switch > span { width: 34px; height: 19px; position: relative; border: 1px solid var(--color-rule-strong); border-radius: 999px; background: var(--color-paper-subtle); transition: background .16s ease, border-color .16s ease; }
  .switch > span::after { content: ''; width: 13px; height: 13px; position: absolute; top: 2px; left: 2px; border-radius: 50%; background: var(--color-muted); transition: transform .16s ease, background .16s ease; }
  .switch input:checked + span { border-color: var(--color-accent); background: var(--color-accent); }
  .switch input:checked + span::after { transform: translateX(15px); background: var(--color-accent-ink); }
  .switch input:focus-visible + span { outline: 2px solid var(--color-focus); outline-offset: 2px; }
  .switch em { color: var(--color-muted); font: 700 9px var(--font-mono); font-style: normal; }
  .webhook-endpoint { margin: 0 var(--space-4) var(--space-4); padding: var(--space-3); display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-log-bg); }
  .webhook-endpoint > div { min-width: 0; display: grid; gap: 5px; }
  .webhook-endpoint small { color: var(--color-log-muted); font: 8px var(--font-mono); text-transform: uppercase; letter-spacing: .08em; }
  .webhook-endpoint code { overflow: hidden; color: var(--color-log-text); font: 9px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .webhook-endpoint button, .webhook-endpoint a, .deployment-triggers > footer button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: 10px; font-weight: 700; text-decoration: none; cursor: pointer; }
  .webhook-endpoint.webhook-warning { border-color: color-mix(in oklch, var(--color-warning) 58%, var(--color-rule)); background: color-mix(in oklch, var(--color-warning) 9%, var(--color-log-bg)); }
  .registry-trigger-config { border-top: 1px solid var(--color-rule); }
  .registry-trigger-config > .settings-field { padding: var(--space-4); }
  .deployment-triggers > footer { min-height: 52px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .deployment-triggers > footer > span { color: var(--color-muted); font: 9px var(--font-mono); }
  .deployment-triggers > footer button { border-color: var(--color-accent); background: var(--color-accent); color: var(--color-accent-ink); }
  .deployment-triggers > footer button:disabled { opacity: .55; cursor: wait; }
  .runtime-settings-form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .runtime-settings-form > footer > span { color: var(--color-muted); font: 9px var(--font-mono); }
  .runtime-settings-form > footer > div { display: flex; align-items: center; gap: var(--space-2); }
  .runtime-settings-form .save-settings { display: inline-flex; align-items: center; justify-content: center; gap: 7px; }
  .secondary-runtime { min-height: 38px; padding: 0 var(--space-4); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .secondary-runtime:disabled { opacity: .55; cursor: wait; }
  .runtime-settings-empty { min-height: 150px; padding: var(--space-5); display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; align-items: center; gap: var(--space-4); }
  .runtime-settings-empty h4 { margin: 0 0 4px; font-size: 13px; }
  .runtime-settings-empty p { margin: 0; color: var(--color-muted); font-size: 11px; }
  .runtime-settings-empty button { min-height: 38px; padding: 0 var(--space-4); display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .danger-zone { min-height: 112px; padding: var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-5); border: 1px solid color-mix(in oklch, var(--color-danger) 38%, var(--color-rule)); border-radius: var(--radius-lg); background: color-mix(in oklch, var(--color-danger) 5%, var(--color-paper-raised)); }
  .danger-zone span { color: var(--color-danger); font: 700 9px var(--font-mono); text-transform: uppercase; letter-spacing: .1em; }
  .danger-zone h3 { margin: var(--space-1) 0; font-size: 14px; }
  .danger-zone p { margin: 0; color: var(--color-muted); font-size: 11px; }
  .danger-zone button { min-height: 38px; padding: 0 var(--space-4); flex: 0 0 auto; border: 1px solid color-mix(in oklch, var(--color-danger) 55%, var(--color-rule)); border-radius: var(--radius-sm); background: transparent; color: var(--color-danger); font-size: 11px; font-weight: 700; cursor: pointer; }
  .danger-zone button:hover { background: var(--color-danger); color: white; }
  .delete-project-modal form { padding: var(--space-5); }
  .deletion-warning { margin-bottom: var(--space-5); padding: var(--space-4); border-left: 3px solid var(--color-danger); border-radius: 0 var(--radius-sm) var(--radius-sm) 0; background: color-mix(in oklch, var(--color-danger) 8%, var(--color-paper-raised)); }
  .deletion-warning strong { color: var(--color-danger); font-size: 13px; }
  .deletion-warning p { margin: var(--space-2) 0 0; color: var(--color-muted); font-size: 11px; line-height: 1.6; }
  .confirm-field { display: grid; gap: var(--space-2); color: var(--color-ink); font-size: 11px; font-weight: 600; }
  .confirm-field code { padding: 2px 5px; border-radius: 4px; background: var(--color-paper-subtle); color: var(--color-danger); }
  .delete-project-modal form > footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-4) var(--space-5); display: flex; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); }
  .delete-project-modal footer button { min-height: 38px; padding: 0 var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 11px; font-weight: 700; cursor: pointer; }
  .delete-project-modal footer .destructive { border-color: var(--color-danger); background: var(--color-danger); color: white; }
  .delete-project-modal footer button:disabled { opacity: .45; cursor: not-allowed; }
  .state { min-height: 300px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); }
  .state h3, .state p { margin: 0; }
  .state h3 { color: var(--color-ink); font-size: 15px; }
  .state p { margin-top: var(--space-1); font-size: 13px; }
  .spinner { width: 22px; height: 22px; border: 2px solid var(--color-rule); border-top-color: var(--color-accent); border-radius: 50%; animation: spin 800ms linear infinite; }
  .hero-actions button, .deploy-small, .service-head-actions button, .application-service-actions button, .database-actions button, .environment-header-actions button, .environment-editor > footer button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; }
  .application-service-actions .icon-only, .database-actions .icon-only { width: 34px; padding: 0; }
  .deployment-service-tabs, .environment-service-tabs { padding: var(--space-3) var(--space-5); display: flex; align-items: center; gap: var(--space-2); overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-paper-subtle); }
  .deployment-service-tabs button { min-height: 34px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: 11px; font-weight: 650; white-space: nowrap; cursor: pointer; }
  .deployment-service-tabs button span { min-width: 20px; height: 20px; padding: 0 5px; display: grid; place-items: center; border-radius: 10px; background: var(--color-paper-raised); font: 9px var(--font-mono); }
  .deployment-service-tabs button.active { border-color: var(--color-rule-strong); background: var(--color-paper-raised); color: var(--color-ink); }
  .environment-service-tabs { align-items: stretch; }
  .environment-service-tabs > button { min-width: 190px; min-height: 58px; padding: var(--space-2) var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-muted); text-align: left; cursor: pointer; }
  .environment-service-tabs > button.active { border-color: var(--color-accent); background: var(--color-accent-soft); color: var(--color-accent); box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-accent) 18%, transparent); }
  .service-tab-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); }
  .environment-service-tabs button > span:nth-child(2) { min-width: 0; display: grid; gap: 3px; }
  .environment-service-tabs strong, .environment-service-tabs small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .environment-service-tabs strong { color: var(--color-ink); font-size: 11px; } .environment-service-tabs small { font: 9px var(--font-mono); }
  .source-choice { grid-template-columns: repeat(3, 1fr); }
  .service-first-settings { padding: var(--space-4); display: flex; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-accent); }
  .service-first-settings div { display: grid; gap: 3px; } .service-first-settings strong { color: var(--color-ink); font-size: 12px; } .service-first-settings span { color: var(--color-muted); font-size: 10px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes live-pulse { 50% { box-shadow: 0 0 0 7px transparent; } }
  @media (min-width: 42rem) { .form-grid { grid-template-columns: 1fr 1fr; } }
  @media (max-width: 41.99rem) { .service-source-picker { grid-template-columns: 1fr; } }
  @media (min-width: 50rem) { .project-hero { flex-direction: row; align-items: center; } .overview-grid { grid-template-columns: minmax(0, 1.4fr) minmax(280px, .8fr); } .domain-layout { grid-template-columns: minmax(0, 1.35fr) minmax(300px, .65fr); } .route-guide { border-top: 0; border-left: 1px solid var(--color-rule); } }
  @media (max-width: 48rem) { .recent > a, .deployment-row { grid-template-columns: 100px minmax(0, 1fr) 20px; } .recent code, .deployment-row code, .recent time, .deployment-row time { display: none; } .services article { grid-template-columns: 40px minmax(0, 1fr) auto; } }
  @media (max-width: 32rem) { .hero-actions { width: 100%; } .hero-actions button { flex: 1; padding-inline: var(--space-3); } .services article { grid-template-columns: 40px minmax(0, 1fr); } .services article :global(.good), .services article :global(.busy), .services article :global(.bad) { grid-column: 2; } .database-state, .database-actions { grid-column: 2; justify-items: start; } .feedback { grid-template-columns: 1fr auto; } .feedback span { grid-row: 2; grid-column: 1 / -1; } .engine-picker { grid-template-columns: 1fr; } .credential-list > div { grid-template-columns: 1fr 54px; padding: var(--space-2) 0; } .credential-list span { grid-column: 1 / -1; } .source-choice, .settings-grid { grid-template-columns: 1fr; } .settings-grid .wide, .repository-note { grid-column: auto; } .danger-zone, .project-editor form > footer, .runtime-settings-form > footer, .deployment-triggers > footer { align-items: flex-start; flex-direction: column; } .runtime-settings-panel > header { align-items: flex-start; flex-direction: column; } .runtime-settings-panel > header > p { margin-left: 0; text-align: left; } .runtime-settings-form > footer > div, .runtime-settings-form > footer button, .deployment-triggers > footer button { width: 100%; } .runtime-settings-empty { grid-template-columns: 42px minmax(0, 1fr); } .runtime-settings-empty button { grid-column: 1 / -1; width: 100%; } .trigger-row { grid-template-columns: 36px minmax(0, 1fr); } .trigger-row .switch { grid-column: 2; } .webhook-endpoint { grid-template-columns: 1fr; } .webhook-endpoint button { width: 100%; } }
  @media (max-width: 44rem) { .log-panel > header { align-items: flex-start; flex-direction: column; } .log-actions { width: 100%; justify-content: flex-start; } .log-actions small { width: 100%; } .log-toolbar { align-items: stretch; flex-direction: column; } .log-toolbar label { width: 100%; } .log-line { grid-template-columns: 34px 62px minmax(0, 1fr); } .log-line time { display: none; } .environment-panel > header { align-items: flex-start; flex-direction: column; gap: var(--space-3); } .environment-columns { display: none; } .variable-row { grid-template-columns: 1fr 64px 34px; padding: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); } .variable-row > label:first-child, .value-field { grid-column: 1 / -1; } .environment-editor > footer { align-items: stretch; flex-direction: column; } .environment-editor > footer button { width: 100%; } .domain-editor-head { align-items: stretch; flex-direction: column; } .add-domain { width: 100%; } .domain-binding > .binding-head { grid-template-columns: minmax(0, 1fr) auto; } .binding-domain { grid-column: 1 / -1; } .binding-preview { grid-template-columns: minmax(0, 1fr) auto; } .binding-preview code { grid-row: 2; grid-column: 1 / -1; } .route-rules-footer { align-items: stretch; flex-direction: column; } .route-rules-footer button { width: 100%; } }
  @media (max-width: 78rem) { .project-metric-grid { grid-template-columns: repeat(3, 1fr); } .project-metric-grid article:nth-child(3) { border-right: 0; } .project-metric-grid article:nth-child(-n+3) { border-bottom: 1px solid var(--color-rule); } .workload-columns, .workload-row { grid-template-columns: minmax(210px, 1.35fr) minmax(74px, .5fr) minmax(105px, .7fr) minmax(100px, .65fr); } .workload-columns span:nth-child(n+5), .workload-row > :nth-child(n+5) { display: none; } }
  @media (max-width: 52rem) { .project-metrics-head { align-items: flex-start; flex-direction: column; } .metrics-freshness { width: 100%; } .metrics-freshness button { margin-left: auto; } .project-metric-grid { grid-template-columns: 1fr 1fr; } .project-metric-grid article, .project-metric-grid article:nth-child(3) { border-right: 1px solid var(--color-rule); border-bottom: 1px solid var(--color-rule); } .project-metric-grid article:nth-child(even) { border-right: 0; } .project-metric-grid article:last-child { border-bottom: 0; } .workload-columns { display: none; } .workload-row { grid-template-columns: minmax(0, 1fr) 80px 105px; } .workload-row > :nth-child(n+4) { display: none; } }
  @media (max-width: 34rem) { .project-metric-grid { grid-template-columns: 1fr; } .project-metric-grid article, .project-metric-grid article:nth-child(3), .project-metric-grid article:nth-child(even) { border-right: 0; } .workload-row { grid-template-columns: minmax(0, 1fr) 74px; } .workload-row > :nth-child(n+3) { display: none; } }
  @media (prefers-reduced-motion: reduce) { .spinner, .live-toggle.live i, .metrics-freshness > i.spinning { animation: none; } }
</style>
