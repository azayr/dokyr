<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api } from '$lib/auth.js';

  const infrastructureViews = ['monitoring', 'backups', 'cleanup'];
  const controlServices = [
    { key: 'dokyr', name: 'Dokyr', role: 'Control plane', icon: 'box' },
    { key: 'postgres', name: 'PostgreSQL', role: 'State database', icon: 'database' },
    { key: 'caddy', name: 'Caddy', role: 'Ingress proxy', icon: 'globe' },
    { key: 'registry', name: 'Registry', role: 'Container image storage', icon: 'layers' },
    { key: 'stalwart', name: 'Stalwart', role: 'Mail server', icon: 'mail' }
  ];
  const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
  const defaultCleanupSchedule = {
    configured: false,
    enabled: false,
    frequency: 'weekly',
    weekday: 0,
    hour: 3,
    minute: 0,
    timezone: 'UTC',
    containers: true,
    images: true,
    buildCache: true,
    networks: true,
    lastStatus: 'never'
  };
  const defaultBackupSchedule = {
    configured: false,
    enabled: false,
    objectStorageId: '',
    objectStorageName: '',
    frequency: 'daily',
    weekday: 0,
    hour: 2,
    minute: 0,
    timezone: 'UTC',
    lastStatus: 'never'
  };

  let activeView = 'monitoring';
  let metrics = { engineName: 'local-docker', global: { diskIo: {}, networkIo: {}, disk: {} } };
  let metricsLoading = true;
  let metricsRefreshing = false;
  let metricsError = '';
  let history = [];
  let pollTimer;
  let hashListener;
  let cleanup = { containers: {}, images: {}, buildCache: {}, networks: {}, volumes: {} };
  let cleanupLoading = false;
  let cleanupLoaded = false;
  let cleanupError = '';
  let cleanupSelection = { containers: true, images: true, buildCache: true, networks: true, volumes: false };
  let cleanupConfirmation = '';
  let cleanupRunning = false;
  let cleanupResult = null;
  let cleanupSchedule = { ...defaultCleanupSchedule };
  let cleanupScheduleTime = '03:00';
  let cleanupScheduleLoading = false;
  let cleanupScheduleLoaded = false;
  let cleanupScheduleSaving = false;
  let cleanupScheduleError = '';
  let cleanupScheduleSaved = false;
  let controlPlane = { global: { diskIo: {}, networkIo: {} }, containers: [] };
  let controlPlaneLoading = false;
  let controlPlaneRefreshing = false;
  let controlPlaneLoaded = false;
  let controlPlaneError = '';
  let selectedControlService = 'dokyr';
  let controlLogs = [];
  let controlLogsLoading = false;
  let controlLogsLoaded = false;
  let controlLogsError = '';
  let controlLogsAutoRefresh = true;
  let controlLogViewport;
  let controlLogsCopied = false;
  let controlLogsRequest = 0;
  let backupJobs = [];
  let backupDestinations = [];
  let backupSchedule = { ...defaultBackupSchedule };
  let backupScheduleTime = '02:00';
  let backupLoading = false;
  let backupLoaded = false;
  let backupError = '';
  let backupScheduleSaving = false;
  let backupScheduleSaved = false;
  let backupCreating = false;
  let selectedBackupStorage = '';
  let restoreTarget = null;
  let restoreRunning = false;
  let restoreError = '';

  $: selectedBytes = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.bytes || 0 : 0), 0);
  $: selectedItems = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.count || 0 : 0), 0);
  $: hostDiskAvailable = (metrics.global.disk?.total || 0) > 0;
  $: automaticResourceCount = ['containers', 'images', 'buildCache', 'networks'].filter((key) => cleanupSchedule[key]).length;
  $: selectedControlContainer = controlPlane.containers?.find((container) => container.controlPlaneService === selectedControlService);
  $: activeBackupJobs = backupJobs.filter((job) => job.status === 'queued' || job.status === 'running');
  $: successfulBackups = backupJobs.filter((job) => job.kind === 'backup' && job.status === 'succeeded');

  onMount(async () => {
    hashListener = syncViewFromURL;
    syncViewFromURL();
    window.addEventListener('hashchange', hashListener);

    await loadMetrics();
    pollTimer = setInterval(() => {
      if (activeView === 'monitoring') {
        if (!metricsRefreshing) loadMetrics(true);
        if (!controlPlaneRefreshing) loadControlPlane(true);
        if (controlLogsAutoRefresh && !controlLogsLoading) loadControlLogs(true);
      } else if (activeView === 'backups' && activeBackupJobs.length && !backupLoading) {
        loadBackups(true);
      }
    }, 5000);
  });

  onDestroy(() => {
    clearInterval(pollTimer);
    if (hashListener) window.removeEventListener('hashchange', hashListener);
  });

  async function loadMetrics(silent = false) {
    if (silent) metricsRefreshing = true;
    else metricsLoading = true;
    metricsError = '';
    try {
      const response = await api('/api/infrastructure/metrics');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not read Docker metrics');
      metrics = payload;
      history = [...history.slice(-23), { cpu: payload.global.cpuPercent || 0, memory: payload.global.memoryPercent || 0 }];
    } catch (cause) {
      metricsError = cause instanceof Error ? cause.message : 'Could not read Docker metrics';
    } finally {
      metricsLoading = false;
      metricsRefreshing = false;
    }
  }

  async function loadControlPlane(silent = false) {
    if (silent) controlPlaneRefreshing = true;
    else controlPlaneLoading = true;
    controlPlaneError = '';
    try {
      const response = await api('/api/infrastructure/control-plane/metrics');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not read Dokyr control-plane metrics');
      controlPlane = payload;
      controlPlaneLoaded = true;
    } catch (cause) {
      controlPlaneError = cause instanceof Error ? cause.message : 'Could not read Dokyr control-plane metrics';
    } finally {
      controlPlaneLoading = false;
      controlPlaneRefreshing = false;
    }
  }

  async function loadControlLogs(silent = false) {
    const request = ++controlLogsRequest;
    controlLogsLoading = true;
    controlLogsError = '';
    try {
      const response = await api(`/api/infrastructure/control-plane/logs?service=${selectedControlService}&lines=300`);
      const payload = await response.json();
      if (request !== controlLogsRequest) return;
      if (!response.ok) throw new Error(payload.error || 'Could not read control-plane logs');
      controlLogs = payload.lines || [];
      controlLogsLoaded = true;
      await tick();
      if (controlLogsAutoRefresh && controlLogViewport) {
        controlLogViewport.scrollTop = controlLogViewport.scrollHeight;
      }
    } catch (cause) {
      if (request === controlLogsRequest) {
        controlLogsError = cause instanceof Error ? cause.message : 'Could not read control-plane logs';
      }
    } finally {
      if (request === controlLogsRequest) controlLogsLoading = false;
    }
  }

  function selectControlService(service) {
    if (selectedControlService === service && controlLogsLoaded) return;
    selectedControlService = service;
    controlLogs = [];
    controlLogsLoaded = false;
    controlLogsCopied = false;
    loadControlLogs();
  }

  async function copyControlLogs() {
    if (!controlLogs.length) return;
    await navigator.clipboard.writeText(controlLogs.join('\n'));
    controlLogsCopied = true;
    setTimeout(() => (controlLogsCopied = false), 1500);
  }

  async function loadCleanup() {
    cleanupLoading = true;
    cleanupError = '';
    try {
      const response = await api('/api/infrastructure/cleanup');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not inspect Docker storage');
      cleanup = payload;
      cleanupLoaded = true;
    } catch (cause) {
      cleanupError = cause instanceof Error ? cause.message : 'Could not inspect Docker storage';
    } finally {
      cleanupLoading = false;
    }
  }

  async function loadCleanupSchedule() {
    cleanupScheduleLoading = true;
    cleanupScheduleError = '';
    try {
      const response = await api('/api/infrastructure/cleanup/schedule');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load the cleanup schedule');
      if (!payload.configured) {
        payload.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
      }
      cleanupSchedule = { ...defaultCleanupSchedule, ...payload };
      cleanupScheduleTime = `${String(cleanupSchedule.hour).padStart(2, '0')}:${String(cleanupSchedule.minute).padStart(2, '0')}`;
      cleanupScheduleLoaded = true;
    } catch (cause) {
      cleanupScheduleError = cause instanceof Error ? cause.message : 'Could not load the cleanup schedule';
    } finally {
      cleanupScheduleLoading = false;
    }
  }

  async function loadBackups(silent = false) {
    if (!silent) backupLoading = true;
    backupError = '';
    try {
      const response = await api('/api/infrastructure/backups');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load server backups');
      backupJobs = payload.jobs || [];
      backupDestinations = payload.destinations || [];
      const nextSchedule = { ...defaultBackupSchedule, ...(payload.schedule || {}) };
      if (!nextSchedule.configured) {
        nextSchedule.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
      }
      backupSchedule = nextSchedule;
      backupScheduleTime = `${String(backupSchedule.hour).padStart(2, '0')}:${String(backupSchedule.minute).padStart(2, '0')}`;
      if (!selectedBackupStorage || !backupDestinations.some((item) => item.id === selectedBackupStorage)) {
        selectedBackupStorage = backupSchedule.objectStorageId || backupDestinations[0]?.id || '';
      }
      backupLoaded = true;
    } catch (cause) {
      backupError = cause instanceof Error ? cause.message : 'Could not load server backups';
    } finally {
      backupLoading = false;
    }
  }

  async function createBackup() {
    if (!selectedBackupStorage) return;
    backupCreating = true;
    backupError = '';
    try {
      const response = await api('/api/infrastructure/backups', {
        method: 'POST',
        body: JSON.stringify({ objectStorageId: selectedBackupStorage })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not queue the server backup');
      backupJobs = [payload.job, ...backupJobs];
      setTimeout(() => loadBackups(true), 700);
    } catch (cause) {
      backupError = cause instanceof Error ? cause.message : 'Could not queue the server backup';
    } finally {
      backupCreating = false;
    }
  }

  async function saveBackupSchedule() {
    backupScheduleSaving = true;
    backupScheduleSaved = false;
    backupError = '';
    const [hour, minute] = backupScheduleTime.split(':').map(Number);
    try {
      const response = await api('/api/infrastructure/backups/schedule', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: backupSchedule.enabled,
          objectStorageId: backupSchedule.objectStorageId,
          frequency: backupSchedule.frequency,
          weekday: Number(backupSchedule.weekday),
          hour,
          minute,
          timezone: backupSchedule.timezone
        })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save the backup schedule');
      backupSchedule = { ...defaultBackupSchedule, ...payload };
      backupScheduleTime = `${String(backupSchedule.hour).padStart(2, '0')}:${String(backupSchedule.minute).padStart(2, '0')}`;
      selectedBackupStorage = backupSchedule.objectStorageId;
      backupScheduleSaved = true;
    } catch (cause) {
      backupError = cause instanceof Error ? cause.message : 'Could not save the backup schedule';
    } finally {
      backupScheduleSaving = false;
    }
  }

  async function restoreBackup() {
    if (!restoreTarget) return;
    restoreRunning = true;
    restoreError = '';
    try {
      const response = await api(`/api/infrastructure/backups/${encodeURIComponent(restoreTarget.id)}/restore`, {
        method: 'POST',
        body: JSON.stringify({ confirmation: 'RESTORE SERVER' })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not queue the restore');
      backupJobs = [payload.job, ...backupJobs];
      restoreTarget = null;
      setTimeout(() => loadBackups(true), 700);
    } catch (cause) {
      restoreError = cause instanceof Error ? cause.message : 'Could not queue the restore';
    } finally {
      restoreRunning = false;
    }
  }

  function activateView(view) {
    activeView = view;
    if (view === 'monitoring') {
      if (!controlPlaneLoaded && !controlPlaneLoading) loadControlPlane();
      if (!controlLogsLoaded && !controlLogsLoading) loadControlLogs();
    } else if (view === 'cleanup') {
      if (!cleanupLoaded && !cleanupLoading) loadCleanup();
      if (!cleanupScheduleLoaded && !cleanupScheduleLoading) loadCleanupSchedule();
    } else if (view === 'backups') {
      if (!backupLoaded && !backupLoading) loadBackups();
    }
  }

  function viewFromHash() {
    const view = window.location.hash.slice(1).toLowerCase();
    return infrastructureViews.includes(view) ? view : 'monitoring';
  }

  function syncViewFromURL() {
    const view = viewFromHash();
    activateView(view);

    if (window.location.hash !== `#${view}`) {
      window.history.replaceState(null, '', `#${view}`);
    }
  }

  function showView(view) {
    if (!infrastructureViews.includes(view)) return;

    activateView(view);
    if (window.location.hash !== `#${view}`) {
      window.location.hash = view;
    }
  }

  async function runCleanup() {
    cleanupRunning = true;
    cleanupError = '';
    cleanupResult = null;
    try {
      const response = await api('/api/infrastructure/cleanup', {
        method: 'POST',
        body: JSON.stringify({ ...cleanupSelection, confirmation: cleanupConfirmation })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Docker cleanup failed');
      cleanupResult = payload;
      cleanup = payload.after;
      cleanupConfirmation = '';
      await loadMetrics(true);
    } catch (cause) {
      cleanupError = cause instanceof Error ? cause.message : 'Docker cleanup failed';
    } finally {
      cleanupRunning = false;
    }
  }

  async function saveCleanupSchedule() {
    cleanupScheduleSaving = true;
    cleanupScheduleError = '';
    cleanupScheduleSaved = false;
    const [hour, minute] = cleanupScheduleTime.split(':').map(Number);
    try {
      const response = await api('/api/infrastructure/cleanup/schedule', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: cleanupSchedule.enabled,
          frequency: cleanupSchedule.frequency,
          weekday: Number(cleanupSchedule.weekday),
          hour,
          minute,
          timezone: cleanupSchedule.timezone,
          containers: cleanupSchedule.containers,
          images: cleanupSchedule.images,
          buildCache: cleanupSchedule.buildCache,
          networks: cleanupSchedule.networks
        })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save the cleanup schedule');
      cleanupSchedule = { ...defaultCleanupSchedule, ...payload };
      cleanupScheduleTime = `${String(cleanupSchedule.hour).padStart(2, '0')}:${String(cleanupSchedule.minute).padStart(2, '0')}`;
      cleanupScheduleSaved = true;
    } catch (cause) {
      cleanupScheduleError = cause instanceof Error ? cause.message : 'Could not save the cleanup schedule';
    } finally {
      cleanupScheduleSaving = false;
    }
  }

  function formatScheduleDate(value) {
    if (!value) return 'Not scheduled';
    try {
      return new Date(value).toLocaleString([], {
        dateStyle: 'medium',
        timeStyle: 'short',
        timeZone: cleanupSchedule.timezone
      });
    } catch {
      return new Date(value).toLocaleString();
    }
  }

  function formatBackupDate(value, withTime = true) {
    if (!value) return 'Not yet';
    try {
      return new Date(value).toLocaleString([], {
        dateStyle: 'medium',
        ...(withTime ? { timeStyle: 'short' } : {}),
        timeZone: backupSchedule.timezone
      });
    } catch {
      return new Date(value).toLocaleString();
    }
  }

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

  function width(value = 0, maximum = 100) {
    return `${Math.max(2, Math.min(100, (value / maximum) * 100))}%`;
  }
</script>

<Shell eyebrow="Infrastructure" title="Servers" subtitle="Docker node health, resource usage, and storage maintenance.">
  <div slot="actions" class="view-switch" role="tablist" aria-label="Infrastructure view">
    <button role="tab" aria-selected={activeView === 'monitoring'} class:active={activeView === 'monitoring'} onclick={() => showView('monitoring')}>
      <Icon name="activity" size={14} /> Monitoring
    </button>
    <button role="tab" aria-selected={activeView === 'backups'} class:active={activeView === 'backups'} onclick={() => showView('backups')}>
      <Icon name="hard-drive" size={14} /> Backups
    </button>
    <button role="tab" aria-selected={activeView === 'cleanup'} class:active={activeView === 'cleanup'} onclick={() => showView('cleanup')}>
      <Icon name="trash" size={14} /> Cleanup
    </button>
  </div>

  <section class="node-signal">
    <div class="node-identity">
      <span class="engine-light" class:offline={Boolean(metricsError)}></span>
      <div>
        <strong>{metrics.engineName || 'local-docker'}</strong>
        <small>Docker socket · single-node control plane</small>
      </div>
    </div>
    <div class="refresh-state">
      {#if metricsRefreshing}<span class="spinner small"></span>{/if}
      <span>{metrics.checkedAt ? `Updated ${new Date(metrics.checkedAt).toLocaleTimeString()}` : 'Connecting…'}</span>
    </div>
  </section>

  {#if activeView === 'monitoring'}
    {#if metricsError}
      <div class="alert alert-error">
        <Icon name="x-circle" size={15} />
        <div><strong>Monitoring unavailable</strong><span>{metricsError}</span></div>
        <button class="btn btn-sm alert-action" onclick={() => loadMetrics()}>Retry</button>
      </div>
    {/if}
    {#if metricsLoading}
      <section class="panel loading-state">
        <span class="spinner"></span>
        <div><strong>Sampling Docker workloads</strong><span>Reading CPU, memory, storage, and network counters…</span></div>
      </section>
    {:else}
      <section class="metric-grid" aria-label="Node metrics">
        <article class="metric-card">
          <header><span>CPU load</span><em>{metrics.global.cpuCores} cores</em></header>
          <strong>{percent(metrics.global.cpuPercent)}</strong>
          <div class="meter"><i style={'width:' + width(metrics.global.cpuPercent)}></i></div>
          <small>Normalized across the Docker host</small>
        </article>
        <article class="metric-card">
          <header><span>Memory</span><em>{formatBytes(metrics.global.memoryLimit)}</em></header>
          <strong>{formatBytes(metrics.global.memoryUsage)}</strong>
          <div class="meter"><i style={'width:' + width(metrics.global.memoryPercent)}></i></div>
          <small>{percent(metrics.global.memoryPercent)} of host memory</small>
        </article>
        <article class="metric-card">
          <header><span>Disk I/O</span><em>host devices</em></header>
          <strong>{formatBytes(metrics.global.diskIo.read)}</strong>
          <div class="io-pair"><span>Read</span><b>{formatBytes(metrics.global.diskIo.read)}</b><span>Written</span><b>{formatBytes(metrics.global.diskIo.write)}</b></div>
        </article>
        <article class="metric-card">
          <header><span>{hostDiskAvailable ? 'Disk space' : 'Docker storage'}</span><em>{hostDiskAvailable ? 'host filesystem' : 'Docker Desktop'}</em></header>
          <strong>{formatBytes(hostDiskAvailable ? metrics.global.disk.used : metrics.global.disk.dockerUsed)}</strong>
          <div class="io-pair">
            <span>{hostDiskAvailable ? 'Available' : 'Reclaimable'}</span><b>{formatBytes(hostDiskAvailable ? metrics.global.disk.available : metrics.global.disk.reclaimable)}</b>
            <span>{hostDiskAvailable ? 'Total' : 'Allocated'}</span><b>{formatBytes(hostDiskAvailable ? metrics.global.disk.total : metrics.global.disk.dockerUsed)}</b>
          </div>
        </article>
        <article class="metric-card">
          <header><span>Network I/O</span><em>all interfaces</em></header>
          <strong>{formatBytes(metrics.global.networkIo.receive)}</strong>
          <div class="io-pair"><span>Received</span><b>{formatBytes(metrics.global.networkIo.receive)}</b><span>Sent</span><b>{formatBytes(metrics.global.networkIo.transmit)}</b></div>
        </article>
      </section>

      <section class="panel control-plane-panel" aria-labelledby="control-plane-title">
        <header class="control-plane-header">
          <div>
            <span class="eyebrow">Platform internals</span>
            <h2 id="control-plane-title">Dokyr control plane</h2>
            <p>Health, resource pressure, and logs for Dokyr and the services it depends on.</p>
          </div>
          <div class="control-plane-total">
            {#if controlPlaneRefreshing}<span class="spinner small"></span>{/if}
            <strong>{controlPlane.global?.running || 0}/{controlServices.length}</strong>
            <span>services online · {formatBytes(controlPlane.global?.memoryUsage)}</span>
          </div>
        </header>

        {#if controlPlaneError}
          <div class="alert alert-error control-plane-alert">
            <Icon name="x-circle" size={15} />
            <div><strong>Control-plane monitoring unavailable</strong><span>{controlPlaneError}</span></div>
            <button class="btn btn-sm alert-action" onclick={() => loadControlPlane()}>Retry</button>
          </div>
        {/if}

        <div class="control-service-grid" role="tablist" aria-label="Control-plane service logs">
          {#each controlServices as service}
            {@const container = controlPlane.containers?.find((item) => item.controlPlaneService === service.key)}
            <button
              type="button"
              role="tab"
              aria-selected={selectedControlService === service.key}
              class:selected={selectedControlService === service.key}
              onclick={() => selectControlService(service.key)}
            >
              <span class="service-card-head">
                <i><Icon name={service.icon} size={17} /></i>
                <span>
                  <strong>{service.name}</strong>
                  <small>{service.role}</small>
                </span>
                <em class:online={container?.state === 'running'}>
                  <b></b>{container?.state === 'running' ? 'Online' : controlPlaneLoading ? 'Checking' : 'Offline'}
                </em>
              </span>
              <code title={container?.image || 'Container unavailable'}>{container?.image || 'Container unavailable'}</code>
              <dl>
                <div><dt>CPU</dt><dd>{percent(container?.cpuPercent || 0)}</dd></div>
                <div><dt>Memory</dt><dd>{formatBytes(container?.memoryUsage)}</dd></div>
                <div><dt>Network</dt><dd>↓ {formatBytes(container?.networkIo?.receive)}</dd></div>
              </dl>
              <span class="service-card-foot">
                <span>{container?.status || 'Not detected'}</span>
                <strong>View logs <Icon name="arrow-right" size={13} /></strong>
              </span>
            </button>
          {/each}
        </div>

        <section class="control-log-console" aria-label={`${selectedControlService} logs`}>
          <header>
            <div class="console-title">
              <span class="console-lights"><i></i><i></i><i></i></span>
              <span>
                <strong>{controlServices.find((service) => service.key === selectedControlService)?.name} logs</strong>
                <code>{selectedControlContainer?.name || 'container unavailable'}</code>
              </span>
              {#if controlLogsAutoRefresh}<em><i></i>LIVE</em>{/if}
            </div>
            <div class="console-actions">
              <label><input type="checkbox" bind:checked={controlLogsAutoRefresh} /> Auto-refresh</label>
              <button type="button" onclick={() => loadControlLogs()} disabled={controlLogsLoading}><Icon name="refresh" size={13} /> Refresh</button>
              <button type="button" onclick={copyControlLogs} disabled={!controlLogs.length}><Icon name="copy" size={13} /> {controlLogsCopied ? 'Copied' : 'Copy'}</button>
            </div>
          </header>
          <div class="control-log-output" bind:this={controlLogViewport}>
            {#if controlLogsError}
              <div class="console-message error"><Icon name="x-circle" size={16} /><span><strong>Logs unavailable</strong>{controlLogsError}</span></div>
            {:else if controlLogsLoading && !controlLogsLoaded}
              <div class="console-message"><span class="spinner small"></span><span><strong>Reading container output</strong>Loading the latest 300 lines…</span></div>
            {:else if !controlLogs.length}
              <div class="console-message"><Icon name="logs" size={16} /><span><strong>No output yet</strong>This container has not written any recent logs.</span></div>
            {:else}
              {#each controlLogs as line, index}
                <div class="control-log-line"><span>{String(index + 1).padStart(3, '0')}</span><code>{line}</code></div>
              {/each}
            {/if}
          </div>
          <footer>
            <span>{controlLogs.length} line{controlLogs.length === 1 ? '' : 's'} · last 300</span>
            <span>{controlPlane.checkedAt ? `Metrics sampled ${new Date(controlPlane.checkedAt).toLocaleTimeString()}` : 'Metrics warming up'}</span>
          </footer>
        </section>
      </section>

      <section class="panel pressure-panel">
        <header class="panel-header">
          <div>
            <span class="eyebrow">Live pressure</span>
            <h2>Node activity</h2>
          </div>
          <div class="legend"><span><i class="cpu"></i>CPU</span><span><i class="memory"></i>Memory</span></div>
        </header>
        <div class="pressure-chart" aria-label="Recent CPU and memory samples">
          {#each history as sample}
            <div class="sample"><i class="memory" style={'height:' + width(sample.memory)}></i><i class="cpu" style={'height:' + width(sample.cpu)}></i></div>
          {/each}
          {#if history.length < 24}{#each Array(24 - history.length) as _}<div class="sample empty"></div>{/each}{/if}
        </div>
        <footer class="panel-footer">
          <span>Last {history.length} sample{history.length === 1 ? '' : 's'} · refreshes every 5 seconds</span>
          <b>{metrics.global.running} running / {metrics.global.containers} total</b>
        </footer>
      </section>
    {/if}
  {:else if activeView === 'cleanup'}
    <section class="cleanup-intro panel">
      <div>
        <span class="eyebrow accent-eyebrow">Storage maintenance</span>
        <h2>Docker cleanup</h2>
        <p>Review unused resources before removing them from this node. Running containers and resources they reference are never selected by Docker's prune operations.</p>
      </div>
      <div class="reclaim-stat">
        <strong>{formatBytes(cleanup.totalReclaimable)}</strong>
        <span>potentially reclaimable</span>
      </div>
    </section>

    {#if cleanupError}
      <div class="alert alert-error">
        <Icon name="x-circle" size={15} />
        <div><strong>Cleanup unavailable</strong><span>{cleanupError}</span></div>
        <button class="btn btn-sm alert-action" onclick={loadCleanup}>Retry</button>
      </div>
    {/if}
    {#if cleanupResult}
      <div class="alert alert-success">
        <Icon name="check-circle" size={15} />
        <div><strong>Cleanup complete</strong><span>Removed {cleanupResult.deleted} resource{cleanupResult.deleted === 1 ? '' : 's'} and reclaimed {formatBytes(cleanupResult.spaceReclaimed)}.</span></div>
      </div>
    {/if}

    <form class="cleanup-layout" onsubmit={(event) => { event.preventDefault(); runCleanup(); }}>
      <section class="panel cleanup-options">
        <header class="panel-header">
          <div>
            <span class="eyebrow">Cleanup plan</span>
            <h2>Select resources</h2>
          </div>
          <button type="button" class="btn btn-sm" onclick={loadCleanup} disabled={cleanupLoading}>{cleanupLoading ? 'Inspecting…' : 'Refresh preview'}</button>
        </header>
        <div class="cleanup-list">
          <label><input class="checkbox" type="checkbox" bind:checked={cleanupSelection.containers} /><i><Icon name="box" size={15} /></i><span><strong>Stopped containers</strong><small>Containers that are no longer running.</small></span><em>{cleanup.containers?.count || 0}<small>{formatBytes(cleanup.containers?.bytes)}</small></em></label>
          <label><input class="checkbox" type="checkbox" bind:checked={cleanupSelection.images} /><i><Icon name="grid" size={15} /></i><span><strong>Unused images</strong><small>Images not referenced by any container.</small></span><em>{cleanup.images?.count || 0}<small>{formatBytes(cleanup.images?.bytes)}</small></em></label>
          <label><input class="checkbox" type="checkbox" bind:checked={cleanupSelection.buildCache} /><i><Icon name="activity" size={15} /></i><span><strong>Build cache</strong><small>Build layers that are not currently in use.</small></span><em>{cleanup.buildCache?.count || 0}<small>{formatBytes(cleanup.buildCache?.bytes)}</small></em></label>
          <label><input class="checkbox" type="checkbox" bind:checked={cleanupSelection.networks} /><i><Icon name="globe" size={15} /></i><span><strong>Unused networks</strong><small>Custom networks with no attached containers.</small></span><em>{cleanup.networks?.count || 0}<small>metadata</small></em></label>
          <label class="volume-option"><input class="checkbox" type="checkbox" bind:checked={cleanupSelection.volumes} /><i><Icon name="database" size={15} /></i><span><strong>Unused volumes</strong><small>Persistent data not attached to a container. Review carefully.</small></span><em>{cleanup.volumes?.count || 0}<small>{formatBytes(cleanup.volumes?.bytes)}</small></em></label>
        </div>
      </section>

      <aside class="panel cleanup-confirm">
        <div class="impact">
          <span>Selected impact</span>
          <strong>{formatBytes(selectedBytes)}</strong>
          <small>{selectedItems} resource{selectedItems === 1 ? '' : 's'} in the current preview</small>
        </div>
        {#if cleanupSelection.volumes}
          <div class="alert alert-warning volume-warning">
            <Icon name="alert" size={15} />
            <div><strong>Persistent data selected</strong><span>Unused named volumes may contain data you still need. This cannot be undone.</span></div>
          </div>
        {/if}
        <label class="field">
          <span>Type <code class="confirm-code">CLEAN DOCKER</code> to confirm</span>
          <input class="input input-mono" bind:value={cleanupConfirmation} placeholder="CLEAN DOCKER" autocomplete="off" spellcheck="false" />
        </label>
        <button class="btn btn-danger-solid cleanup-button" type="submit" disabled={cleanupRunning || cleanupConfirmation !== 'CLEAN DOCKER' || selectedItems === 0}>
          {cleanupRunning ? 'Cleaning Docker…' : `Run cleanup · ${formatBytes(selectedBytes)}`}
        </button>
        <p class="cleanup-note">Docker only removes resources that are unused at execution time. The preview is refreshed after cleanup.</p>
      </aside>
    </form>

    <section class="panel schedule-panel">
      <header class="schedule-header">
        <div class="schedule-heading">
          <span class="schedule-icon"><Icon name="clock" size={18} /></span>
          <div>
            <span class="eyebrow">Automation</span>
            <h2>Scheduled cleanup</h2>
            <p>Let Dokyr remove safe, unused Docker resources on a recurring schedule.</p>
          </div>
        </div>
        <span class:enabled={cleanupSchedule.enabled} class="schedule-state">
          <i></i>{cleanupSchedule.enabled ? 'Active' : 'Paused'}
        </span>
      </header>

      {#if cleanupScheduleError}
        <div class="alert alert-error schedule-alert">
          <Icon name="x-circle" size={15} />
          <div><strong>Schedule unavailable</strong><span>{cleanupScheduleError}</span></div>
        </div>
      {:else if cleanupScheduleSaved}
        <div class="alert alert-success schedule-alert">
          <Icon name="check-circle" size={15} />
          <div><strong>Schedule saved</strong><span>{cleanupSchedule.enabled ? `Next cleanup is ${formatScheduleDate(cleanupSchedule.nextRunAt)}.` : 'Automatic cleanup is paused.'}</span></div>
        </div>
      {/if}

      <div class="schedule-body">
        <aside class="schedule-summary">
          <div class="next-run">
            <span>{cleanupSchedule.enabled ? 'Next automatic cleanup' : 'Automation status'}</span>
            <strong>{cleanupScheduleLoading ? 'Loading…' : cleanupSchedule.enabled ? formatScheduleDate(cleanupSchedule.nextRunAt) : 'Paused'}</strong>
            <small>{cleanupSchedule.enabled ? cleanupSchedule.timezone : 'Enable the schedule when you are ready.'}</small>
          </div>
          <dl>
            <div><dt>Last run</dt><dd>{formatScheduleDate(cleanupSchedule.lastRunAt)}</dd></div>
            <div>
              <dt>Last result</dt>
              <dd class:success={cleanupSchedule.lastStatus === 'succeeded'} class:failed={cleanupSchedule.lastStatus === 'failed'}>
                {cleanupSchedule.lastStatus === 'never' ? 'No runs yet' : cleanupSchedule.lastStatus}
              </dd>
            </div>
            {#if cleanupSchedule.lastStatus === 'succeeded'}
              <div><dt>Reclaimed</dt><dd>{formatBytes(cleanupSchedule.lastReclaimed)} · {cleanupSchedule.lastDeleted} removed</dd></div>
            {/if}
          </dl>
          {#if cleanupSchedule.lastStatus === 'failed' && cleanupSchedule.lastMessage}
            <p class="last-error">{cleanupSchedule.lastMessage}</p>
          {/if}
        </aside>

        <form class="schedule-form" onsubmit={(event) => { event.preventDefault(); saveCleanupSchedule(); }}>
          <label class="automation-toggle">
            <span>
              <strong>Enable automatic cleanup</strong>
              <small>Runs even when no one has the dashboard open.</small>
            </span>
            <input type="checkbox" bind:checked={cleanupSchedule.enabled} />
            <i></i>
          </label>

          <div class="schedule-fields">
            <label class="field">
              <span>Frequency</span>
              <select class="select" bind:value={cleanupSchedule.frequency}>
                <option value="daily">Every day</option>
                <option value="weekly">Every week</option>
              </select>
            </label>
            {#if cleanupSchedule.frequency === 'weekly'}
              <label class="field">
                <span>Day</span>
                <select class="select" bind:value={cleanupSchedule.weekday}>
                  {#each weekdays as day, index}<option value={index}>{day}</option>{/each}
                </select>
              </label>
            {/if}
            <label class="field">
              <span>Run at</span>
              <input class="input input-mono" type="time" bind:value={cleanupScheduleTime} />
            </label>
          </div>
          <p class="timezone-note"><Icon name="globe" size={14} /> Timezone: <code>{cleanupSchedule.timezone}</code></p>

          <fieldset class="automatic-resources">
            <legend>Resources to remove</legend>
            <div>
              <label><input class="checkbox" type="checkbox" bind:checked={cleanupSchedule.containers} /><span>Stopped containers</span></label>
              <label><input class="checkbox" type="checkbox" bind:checked={cleanupSchedule.images} /><span>Unused images</span></label>
              <label><input class="checkbox" type="checkbox" bind:checked={cleanupSchedule.buildCache} /><span>Build cache</span></label>
              <label><input class="checkbox" type="checkbox" bind:checked={cleanupSchedule.networks} /><span>Unused networks</span></label>
            </div>
          </fieldset>

          <div class="schedule-safety">
            <Icon name="shield" size={16} />
            <p><strong>Volumes stay manual.</strong> Scheduled cleanup never deletes volumes or resources attached to running containers.</p>
          </div>

          <footer>
            <span>{automaticResourceCount} resource categor{automaticResourceCount === 1 ? 'y' : 'ies'} selected</span>
            <button class="btn btn-primary" type="submit" disabled={cleanupScheduleLoading || cleanupScheduleSaving || (cleanupSchedule.enabled && automaticResourceCount === 0)}>
              {cleanupScheduleSaving ? 'Saving schedule…' : 'Save schedule'}
            </button>
          </footer>
        </form>
      </div>
    </section>
  {:else}
    <section class="backup-hero panel">
      <div class="backup-hero-copy">
        <span class="backup-vault"><Icon name="hard-drive" size={22} /></span>
        <div>
          <span class="eyebrow accent-eyebrow">Server recovery</span>
          <h2>Portable control-plane snapshots</h2>
          <p>Project configuration, Dokyr settings, and the complete PostgreSQL database travel together in one restorable <code>.tar.gz</code>.</p>
        </div>
      </div>
      <dl>
        <div><dt>Available</dt><dd>{successfulBackups.length}</dd><small>restorable archives</small></div>
        <div><dt>Automation</dt><dd class:online={backupSchedule.enabled}>{backupSchedule.enabled ? 'On' : 'Off'}</dd><small>{backupSchedule.enabled ? formatBackupDate(backupSchedule.nextRunAt) : 'schedule paused'}</small></div>
      </dl>
    </section>

    {#if backupError}
      <div class="alert alert-error backup-alert">
        <Icon name="x-circle" size={15} />
        <div><strong>Backups unavailable</strong><span>{backupError}</span></div>
        <button class="btn btn-sm alert-action" type="button" onclick={() => loadBackups()}>Retry</button>
      </div>
    {:else if backupScheduleSaved}
      <div class="alert alert-success backup-alert">
        <Icon name="check-circle" size={15} />
        <div><strong>Backup schedule saved</strong><span>{backupSchedule.enabled ? `Next backup is ${formatBackupDate(backupSchedule.nextRunAt)}.` : 'Automatic backups are paused.'}</span></div>
      </div>
    {/if}

    {#if backupLoading && !backupLoaded}
      <section class="panel loading-state">
        <span class="spinner"></span>
        <div><strong>Opening the backup vault</strong><span>Loading destinations, schedule, and archive history…</span></div>
      </section>
    {:else if backupDestinations.length === 0}
      <section class="panel backup-empty">
        <span><Icon name="cloud" size={27} /></span>
        <div>
          <span class="eyebrow">Destination required</span>
          <h2>Connect an external S3 bucket first</h2>
          <p>Backups only use reusable object storage connections. Add Amazon S3, Cloudflare R2, MinIO, DigitalOcean Spaces, or another S3-compatible bucket.</p>
        </div>
        <a class="btn btn-primary" href="/object-storage"><Icon name="plus" size={14} /> Add object storage</a>
      </section>
    {:else}
      <div class="backup-command-grid">
        <section class="panel backup-now-card">
          <header>
            <div>
              <span class="eyebrow">On demand</span>
              <h2>Backup this server now</h2>
              <p>The job runs in the background. You can leave this page after it enters the queue.</p>
            </div>
            <span class="queue-mark" class:busy={activeBackupJobs.length > 0}>
              {#if activeBackupJobs.length}<i class="spinner small"></i>{:else}<i></i>{/if}
              {activeBackupJobs.length ? `${activeBackupJobs.length} active` : 'Queue ready'}
            </span>
          </header>
          <div class="backup-destination-row">
            <label class="field">
              <span>Upload destination</span>
              <select class="select" bind:value={selectedBackupStorage}>
                {#each backupDestinations as destination}
                  <option value={destination.id}>{destination.name} · {destination.bucket}</option>
                {/each}
              </select>
            </label>
            <button class="btn btn-primary backup-now-button" type="button" onclick={createBackup} disabled={backupCreating || !selectedBackupStorage}>
              <Icon name="hard-drive" size={15} /> {backupCreating ? 'Adding to queue…' : 'Backup now'}
            </button>
          </div>
          <footer><Icon name="lock" size={13} /> Credentials remain encrypted; only the background worker can decrypt the selected connection.</footer>
        </section>

        <aside class="panel backup-contents">
          <header><span class="eyebrow">Every archive</span><strong>Recovery set</strong></header>
          <ul>
            <li><i><Icon name="folder" size={15} /></i><span><strong>Project configuration</strong><small>Services, domains, environment, and deployment settings</small></span><b><Icon name="check" size={13} /></b></li>
            <li><i><Icon name="settings" size={15} /></i><span><strong>Dokyr configuration</strong><small>Users, integrations, Registry, SMTP, and platform policy</small></span><b><Icon name="check" size={13} /></b></li>
            <li><i><Icon name="database" size={15} /></i><span><strong>PostgreSQL database</strong><small>Consistent plain-SQL dump for transactional restore</small></span><b><Icon name="check" size={13} /></b></li>
          </ul>
          <footer><code>dokyr-server-YYYYMMDDTHHMMSSZ.tar.gz</code></footer>
        </aside>
      </div>

      <section class="panel backup-schedule-card">
        <header class="schedule-header">
          <div class="schedule-heading">
            <span class="schedule-icon"><Icon name="clock" size={18} /></span>
            <div>
              <span class="eyebrow">Automation</span>
              <h2>Scheduled server backups</h2>
              <p>Create the same complete archive on a daily or weekly cadence.</p>
            </div>
          </div>
          <span class:enabled={backupSchedule.enabled} class="schedule-state"><i></i>{backupSchedule.enabled ? 'Active' : 'Paused'}</span>
        </header>
        <form class="backup-schedule-form" onsubmit={(event) => { event.preventDefault(); saveBackupSchedule(); }}>
          <label class="automation-toggle">
            <span><strong>Enable automatic backups</strong><small>Queued by Dokyr even when no one is signed in.</small></span>
            <input type="checkbox" bind:checked={backupSchedule.enabled} />
            <i></i>
          </label>
          <div class="backup-schedule-fields">
            <label class="field destination-field">
              <span>Object storage</span>
              <select class="select" bind:value={backupSchedule.objectStorageId} required>
                <option value="" disabled>Select a bucket</option>
                {#each backupDestinations as destination}
                  <option value={destination.id}>{destination.name} · {destination.bucket}</option>
                {/each}
              </select>
            </label>
            <label class="field">
              <span>Frequency</span>
              <select class="select" bind:value={backupSchedule.frequency}>
                <option value="daily">Every day</option>
                <option value="weekly">Every week</option>
              </select>
            </label>
            {#if backupSchedule.frequency === 'weekly'}
              <label class="field">
                <span>Day</span>
                <select class="select" bind:value={backupSchedule.weekday}>
                  {#each weekdays as day, index}<option value={index}>{day}</option>{/each}
                </select>
              </label>
            {/if}
            <label class="field">
              <span>Run at</span>
              <input class="input input-mono" type="time" bind:value={backupScheduleTime} />
            </label>
          </div>
          <div class="backup-schedule-foot">
            <span><Icon name="globe" size={13} /> {backupSchedule.timezone} · Last run {formatBackupDate(backupSchedule.lastRunAt)}</span>
            <button class="btn btn-primary" type="submit" disabled={backupScheduleSaving || !backupSchedule.objectStorageId}>
              {backupScheduleSaving ? 'Saving schedule…' : 'Save schedule'}
            </button>
          </div>
        </form>
      </section>

      <section class="panel backup-history">
        <header class="panel-header">
          <div><span class="eyebrow">Archive ledger</span><h2>Backup and restore history</h2></div>
          <button class="btn btn-sm" type="button" onclick={() => loadBackups()} disabled={backupLoading}><Icon name="refresh" size={13} /> Refresh</button>
        </header>
        {#if backupJobs.length === 0}
          <div class="backup-history-empty">
            <Icon name="hard-drive" size={22} />
            <strong>No backups yet</strong>
            <span>Run the first backup or enable the schedule above.</span>
          </div>
        {:else}
          <div class="backup-job-list">
            {#each backupJobs as job}
              <article class="backup-job" class:restore={job.kind === 'restore'}>
                <span class="job-kind"><Icon name={job.kind === 'restore' ? 'refresh' : 'hard-drive'} size={16} /></span>
                <div class="job-main">
                  <strong>{job.kind === 'restore' ? `Restore from ${job.filename || 'server backup'}` : job.filename || 'Server backup'}</strong>
                  <span>{job.objectStorageName} · {job.trigger === 'scheduled' ? 'Scheduled' : job.kind === 'restore' ? 'Recovery' : 'Manual'} · {formatBackupDate(job.createdAt)}</span>
                  {#if job.status === 'failed' && job.message}<small>{job.message}</small>{/if}
                </div>
                <div class="job-size">
                  <strong>{job.sizeBytes ? formatBytes(job.sizeBytes) : '—'}</strong>
                  <span>.tar.gz</span>
                </div>
                <span class="job-status {job.status}">
                  {#if job.status === 'queued' || job.status === 'running'}<i class="spinner small"></i>{:else}<i></i>{/if}
                  {job.status}
                </span>
                <div class="job-action">
                  {#if job.kind === 'backup' && job.status === 'succeeded'}
                    <button class="btn btn-sm" type="button" onclick={() => { restoreError = ''; restoreTarget = job; }}>Restore</button>
                  {:else}
                    <span>{job.finishedAt ? formatBackupDate(job.finishedAt, false) : 'Background job'}</span>
                  {/if}
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <div class="backup-recovery-note">
        <Icon name="shield" size={15} />
        <p><strong>Keep the installation encryption key.</strong> The archive contains encrypted credentials. Restoring on another server requires the same <code>DOKYR_ENCRYPTION_KEY</code>.</p>
      </div>
    {/if}
  {/if}
</Shell>

{#if restoreTarget}
  <ConfirmDialog
    title="Restore this server backup?"
    message={`Dokyr will replace its project configuration, platform settings, and PostgreSQL database with ${restoreTarget.filename}. The database restore is transactional, but changes made after the backup will be lost.`}
    confirmLabel="Queue server restore"
    requireText="RESTORE SERVER"
    busy={restoreRunning}
    error={restoreError}
    onConfirm={restoreBackup}
    onClose={() => { if (!restoreRunning) restoreTarget = null; }}
  />
{/if}

<style>
  .node-signal {
    min-height: 58px;
    margin-bottom: var(--space-4);
    padding: var(--space-3) var(--space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .node-identity {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .node-identity > div {
    display: grid;
    gap: 1px;
  }
  .node-identity strong {
    font-size: var(--text-md);
  }
  .node-identity small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .engine-light {
    width: 9px;
    height: 9px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: var(--color-success);
    box-shadow: 0 0 0 4px var(--color-success-soft);
  }
  .engine-light.offline {
    background: var(--color-danger);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-danger) 12%, transparent);
  }
  .refresh-state {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--color-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .spinner.small {
    width: 12px;
    height: 12px;
    border-width: 1.5px;
  }
  .alert-action {
    margin-left: auto;
  }
  .view-switch {
    padding: 3px;
    display: flex;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-paper-subtle);
  }
  .view-switch button {
    min-height: 30px;
    padding: 0 var(--space-3);
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-muted);
    font-size: var(--text-sm);
    font-weight: 500;
    cursor: pointer;
  }
  .view-switch button.active {
    background: var(--color-paper-raised);
    color: var(--color-ink);
    box-shadow: var(--shadow-whisper);
  }

  .loading-state {
    min-height: 300px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
  }
  .loading-state div {
    display: grid;
    gap: var(--space-1);
  }
  .loading-state strong {
    font-size: var(--text-md);
  }
  .loading-state span {
    color: var(--color-muted);
    font-size: var(--text-sm);
  }

  .metric-grid {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    overflow: hidden;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .metric-card {
    min-width: 0;
    padding: var(--space-4);
    display: grid;
    align-content: start;
    gap: var(--space-3);
    border-right: 1px solid var(--color-rule);
  }
  .metric-card:last-child {
    border-right: 0;
  }
  .metric-card header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }
  .metric-card header span {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .metric-card header em {
    overflow: hidden;
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-style: normal;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .metric-card > strong {
    overflow: hidden;
    font-size: var(--text-xl);
    font-weight: 700;
    letter-spacing: -0.02em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .metric-card > small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .meter {
    height: 4px;
    overflow: hidden;
    border-radius: 4px;
    background: var(--color-paper-subtle);
  }
  .meter i {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--color-accent);
    transition: width var(--duration-base) var(--ease-out);
  }
  .io-pair {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 5px var(--space-2);
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .io-pair b {
    color: var(--color-ink-secondary);
    font: 500 var(--text-xs) var(--font-mono);
  }

  .control-plane-panel {
    margin-bottom: var(--space-4);
    overflow: hidden;
  }
  .control-plane-header {
    min-height: 86px;
    padding: var(--space-4) var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-5);
    border-bottom: 1px solid var(--color-rule);
    background:
      linear-gradient(105deg, color-mix(in srgb, var(--color-accent) 7%, transparent), transparent 38%),
      var(--color-paper-raised);
  }
  .control-plane-header h2 {
    margin: 2px 0 0;
    font-size: var(--text-lg);
  }
  .control-plane-header p {
    margin: 3px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .control-plane-total {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: auto auto;
    align-items: center;
    justify-items: end;
    gap: 0 var(--space-2);
  }
  .control-plane-total strong {
    font: 700 var(--text-xl) var(--font-mono);
  }
  .control-plane-total > span:last-child {
    grid-column: 1 / -1;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .control-plane-alert {
    margin: var(--space-4) var(--space-5) 0;
  }
  .control-service-grid {
    padding: var(--space-4) var(--space-5);
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-3);
  }
  .control-service-grid > button {
    min-width: 0;
    padding: var(--space-4);
    display: grid;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    color: var(--color-ink);
    text-align: left;
    cursor: pointer;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out),
      transform var(--duration-fast) var(--ease-out);
  }
  .control-service-grid > button:hover {
    border-color: var(--color-rule-strong);
    transform: translateY(-1px);
    box-shadow: var(--shadow-whisper);
  }
  .control-service-grid > button.selected {
    border-color: var(--color-accent);
    background: linear-gradient(145deg, var(--color-accent-softer), var(--color-paper-raised) 55%);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--color-accent) 8%, transparent);
  }
  .service-card-head {
    min-width: 0;
    display: grid;
    grid-template-columns: 36px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-2);
  }
  .service-card-head > i {
    width: 36px;
    height: 36px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-subtle);
    color: var(--color-accent);
    font-style: normal;
  }
  .service-card-head > span {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .service-card-head strong {
    font-size: var(--text-sm);
  }
  .service-card-head small {
    color: var(--color-muted);
    font-size: var(--text-2xs);
  }
  .service-card-head em {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-style: normal;
    font-weight: 700;
    text-transform: uppercase;
  }
  .service-card-head em b {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-faint);
  }
  .service-card-head em.online {
    color: var(--color-success);
  }
  .service-card-head em.online b {
    background: var(--color-success);
    box-shadow: 0 0 0 3px var(--color-success-soft);
  }
  .control-service-grid > button > code {
    overflow: hidden;
    color: var(--color-muted);
    font: 500 var(--text-2xs) var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .control-service-grid dl {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-2);
  }
  .control-service-grid dl > div {
    min-width: 0;
    display: grid;
    gap: 2px;
  }
  .control-service-grid dt {
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-weight: 700;
    text-transform: uppercase;
  }
  .control-service-grid dd {
    margin: 0;
    overflow: hidden;
    color: var(--color-ink-secondary);
    font: 600 var(--text-xs) var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .service-card-foot {
    padding-top: var(--space-2);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    border-top: 1px solid var(--color-rule);
  }
  .service-card-foot > span {
    min-width: 0;
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-2xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .service-card-foot > strong {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--color-accent);
    font-size: var(--text-2xs);
  }
  .control-log-console {
    margin: 0 var(--space-5) var(--space-5);
    overflow: hidden;
    border: 1px solid #29384a;
    border-radius: var(--radius-md);
    background: #0d1723;
    box-shadow: 0 14px 32px color-mix(in srgb, #07101a 22%, transparent);
    color: #d5deea;
  }
  .control-log-console > header {
    min-height: 54px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid #29384a;
    background: #111e2c;
  }
  .console-title,
  .console-title > span:nth-child(2) {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .console-title > span:nth-child(2) {
    gap: 7px;
  }
  .console-title strong {
    color: #e8eef7;
    font-size: var(--text-xs);
  }
  .console-title code {
    max-width: 260px;
    overflow: hidden;
    color: #8292a7;
    font: 500 var(--text-2xs) var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .console-lights {
    display: flex;
    gap: 5px;
  }
  .console-lights i {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #ef6b67;
  }
  .console-lights i:nth-child(2) {
    background: #e7b34c;
  }
  .console-lights i:nth-child(3) {
    background: #5dbc83;
  }
  .console-title em {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: #65d994;
    font-size: 9px;
    font-style: normal;
    font-weight: 800;
    letter-spacing: 0.08em;
  }
  .console-title em i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 15%, transparent);
  }
  .console-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .console-actions label,
  .console-actions button {
    min-height: 29px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #91a0b3;
    font-size: var(--text-2xs);
  }
  .console-actions label {
    cursor: pointer;
  }
  .console-actions input {
    accent-color: #2c8cff;
  }
  .console-actions button {
    padding: 0 var(--space-2);
    border: 1px solid #334459;
    border-radius: var(--radius-xs);
    background: #152334;
    cursor: pointer;
  }
  .console-actions button:hover:not(:disabled) {
    border-color: #4a6380;
    color: #d5deea;
  }
  .console-actions button:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .control-log-output {
    height: 330px;
    overflow: auto;
    padding: var(--space-3) 0;
    scrollbar-color: #34475e #0d1723;
  }
  .control-log-line {
    min-width: max-content;
    padding: 2px var(--space-4);
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr);
    gap: var(--space-3);
    color: #c5d0dd;
    font-size: 11px;
    line-height: 1.55;
  }
  .control-log-line:hover {
    background: #132132;
  }
  .control-log-line > span {
    color: #4e6178;
    font-family: var(--font-mono);
    text-align: right;
    user-select: none;
  }
  .control-log-line code {
    color: inherit;
    font-family: var(--font-mono);
    white-space: pre;
  }
  .console-message {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    color: #72849a;
  }
  .console-message > span:last-child {
    display: grid;
    gap: 2px;
    font-size: var(--text-xs);
  }
  .console-message strong {
    color: #b9c5d4;
  }
  .console-message.error,
  .console-message.error strong {
    color: #f1918d;
  }
  .control-log-console > footer {
    min-height: 35px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-top: 1px solid #29384a;
    background: #111e2c;
    color: #6f8095;
    font: 500 10px var(--font-mono);
  }

  .legend {
    display: flex;
    gap: var(--space-3);
  }
  .legend span {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .legend i {
    width: 8px;
    height: 8px;
    border-radius: 2px;
  }
  .legend i.cpu {
    background: var(--color-accent);
  }
  .legend i.memory {
    background: var(--color-info);
  }
  .pressure-chart {
    height: 132px;
    padding: var(--space-5) var(--space-4) var(--space-3);
    display: grid;
    grid-template-columns: repeat(24, 1fr);
    align-items: end;
    gap: 4px;
    background-image: linear-gradient(to bottom, transparent 24%, var(--color-rule) 25%, transparent 26%, transparent 49%, var(--color-rule) 50%, transparent 51%, transparent 74%, var(--color-rule) 75%, transparent 76%);
  }
  .sample {
    height: 100%;
    position: relative;
    display: flex;
    align-items: end;
    gap: 1px;
  }
  .sample i {
    min-height: 2px;
    flex: 1;
    border-radius: 2px 2px 0 0;
    transition: height var(--duration-base) var(--ease-out);
  }
  .sample i.cpu {
    background: var(--color-accent);
  }
  .sample i.memory {
    background: var(--color-info);
    opacity: 0.7;
  }
  .sample.empty {
    height: 1px;
    background: var(--color-rule);
  }
  .pressure-panel .panel-footer {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .pressure-panel .panel-footer b {
    color: var(--color-ink-secondary);
    font-weight: 600;
  }

  .cleanup-intro {
    margin-bottom: var(--space-4);
    padding: var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
  }
  .cleanup-intro > div:first-child {
    max-width: 640px;
  }
  .accent-eyebrow {
    color: var(--color-accent);
  }
  .cleanup-intro h2 {
    margin: var(--space-1) 0;
    font-size: var(--text-xl);
    letter-spacing: -0.02em;
  }
  .cleanup-intro p {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.6;
  }
  .reclaim-stat {
    flex: 0 0 auto;
    padding: var(--space-3) var(--space-5);
    display: grid;
    gap: 2px;
    border-left: 1px solid var(--color-rule);
    text-align: right;
  }
  .reclaim-stat strong {
    font-size: var(--text-2xl);
    font-weight: 700;
    letter-spacing: -0.03em;
  }
  .reclaim-stat span {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .cleanup-layout {
    display: grid;
    grid-template-columns: minmax(0, 1.35fr) minmax(300px, 0.65fr);
    align-items: start;
    gap: var(--space-4);
  }
  .cleanup-list label {
    min-height: 68px;
    padding: var(--space-3) var(--space-5);
    display: grid;
    grid-template-columns: 18px 36px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
    cursor: pointer;
  }
  .cleanup-list label:last-child {
    border-bottom: 0;
  }
  .cleanup-list label:has(input:checked) {
    background: var(--color-accent-softer);
  }
  .cleanup-list label > i {
    width: 36px;
    height: 36px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-muted);
    font-style: normal;
  }
  .cleanup-list label > span {
    display: grid;
    gap: 2px;
  }
  .cleanup-list label > span strong {
    font-size: var(--text-sm);
  }
  .cleanup-list label > span small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .cleanup-list em {
    min-width: 72px;
    display: grid;
    gap: 2px;
    color: var(--color-ink);
    font: 600 var(--text-md) var(--font-mono);
    font-style: normal;
    text-align: right;
  }
  .cleanup-list em small {
    color: var(--color-muted);
    font: 500 var(--text-2xs) var(--font-mono);
  }
  .cleanup-list .volume-option {
    border-top: 1px dashed color-mix(in srgb, var(--color-danger) 32%, var(--color-rule));
  }
  .cleanup-list .volume-option:has(input:checked) {
    background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised));
  }

  .cleanup-confirm {
    padding: var(--space-5);
    display: grid;
    gap: var(--space-4);
    position: sticky;
    top: 72px;
  }
  .impact {
    padding-bottom: var(--space-4);
    display: grid;
    gap: var(--space-1);
    border-bottom: 1px solid var(--color-rule);
  }
  .impact span {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }
  .impact strong {
    font-size: 26px;
    font-weight: 700;
    letter-spacing: -0.03em;
  }
  .impact small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .volume-warning {
    margin-bottom: 0;
  }
  .confirm-code {
    padding: 1px 5px;
    border-radius: var(--radius-xs);
    background: var(--color-paper-subtle);
    color: var(--color-danger);
    font-family: var(--font-mono);
  }
  .cleanup-button {
    width: 100%;
    min-height: 38px;
  }
  .cleanup-note {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
  }

  .schedule-panel {
    margin-top: var(--space-5);
    overflow: hidden;
  }
  .schedule-header {
    min-height: 82px;
    padding: var(--space-4) var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    border-bottom: 1px solid var(--color-rule);
    background:
      linear-gradient(90deg, color-mix(in srgb, var(--color-info) 6%, transparent), transparent 42%),
      var(--color-paper-raised);
  }
  .schedule-heading {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .schedule-icon {
    width: 42px;
    height: 42px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid color-mix(in srgb, var(--color-info) 25%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-info) 8%, var(--color-paper-raised));
    color: var(--color-info);
  }
  .schedule-heading h2 {
    margin: 2px 0 0;
    font-size: var(--text-lg);
  }
  .schedule-heading p {
    margin: 2px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .schedule-state {
    padding: 5px 9px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--color-rule);
    border-radius: 999px;
    background: var(--color-paper-subtle);
    color: var(--color-muted);
    font-size: var(--text-xs);
    font-weight: 700;
  }
  .schedule-state i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-faint);
  }
  .schedule-state.enabled {
    border-color: color-mix(in srgb, var(--color-success) 30%, var(--color-rule));
    background: color-mix(in srgb, var(--color-success) 9%, var(--color-paper-raised));
    color: var(--color-success);
  }
  .schedule-state.enabled i {
    background: var(--color-success);
  }
  .schedule-alert {
    margin: var(--space-4) var(--space-5) 0;
  }
  .schedule-body {
    display: grid;
    grid-template-columns: minmax(260px, 0.62fr) minmax(0, 1.38fr);
  }
  .schedule-summary {
    padding: var(--space-5);
    border-right: 1px solid var(--color-rule);
    background: var(--color-paper-subtle);
  }
  .next-run {
    padding-bottom: var(--space-5);
    display: grid;
    gap: var(--space-1);
    border-bottom: 1px solid var(--color-rule);
  }
  .next-run span,
  .schedule-summary dt {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .next-run strong {
    font-size: var(--text-lg);
    letter-spacing: -0.02em;
  }
  .next-run small {
    color: var(--color-muted);
    font: 500 var(--text-xs) var(--font-mono);
  }
  .schedule-summary dl {
    margin: var(--space-4) 0 0;
    display: grid;
    gap: var(--space-3);
  }
  .schedule-summary dl > div {
    display: grid;
    gap: 3px;
  }
  .schedule-summary dd {
    margin: 0;
    color: var(--color-ink-secondary);
    font-size: var(--text-sm);
    text-transform: capitalize;
  }
  .schedule-summary dd.success {
    color: var(--color-success);
  }
  .schedule-summary dd.failed,
  .last-error {
    color: var(--color-danger);
  }
  .last-error {
    margin: var(--space-4) 0 0;
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .schedule-form {
    padding: var(--space-5);
    display: grid;
    gap: var(--space-4);
  }
  .automation-toggle {
    min-height: 58px;
    padding: var(--space-3);
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    cursor: pointer;
  }
  .automation-toggle > span {
    display: grid;
    gap: 2px;
  }
  .automation-toggle strong {
    font-size: var(--text-sm);
  }
  .automation-toggle small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .automation-toggle input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .automation-toggle > i {
    width: 38px;
    height: 22px;
    position: relative;
    border: 1px solid var(--color-rule-strong);
    border-radius: 999px;
    background: var(--color-paper-subtle);
    transition: background var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }
  .automation-toggle > i::after {
    content: '';
    width: 16px;
    height: 16px;
    position: absolute;
    top: 2px;
    left: 2px;
    border-radius: 50%;
    background: var(--color-muted);
    transition: transform var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out);
  }
  .automation-toggle input:checked + i {
    border-color: var(--color-accent);
    background: var(--color-accent);
  }
  .automation-toggle input:checked + i::after {
    transform: translateX(16px);
    background: var(--color-accent-ink);
  }
  .automation-toggle input:focus-visible + i {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  .schedule-fields {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-3);
  }
  .timezone-note {
    margin: calc(var(--space-2) * -1) 0 0;
    display: flex;
    align-items: center;
    gap: var(--space-1);
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .timezone-note code {
    color: var(--color-ink-secondary);
    font-family: var(--font-mono);
  }
  .automatic-resources {
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  .automatic-resources legend {
    margin-bottom: var(--space-2);
    color: var(--color-ink-secondary);
    font-size: var(--text-xs);
    font-weight: 700;
  }
  .automatic-resources > div {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .automatic-resources label {
    min-height: 44px;
    padding: 0 var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border-bottom: 1px solid var(--color-rule);
    cursor: pointer;
    font-size: var(--text-xs);
  }
  .automatic-resources label:nth-child(odd) {
    border-right: 1px solid var(--color-rule);
  }
  .automatic-resources label:nth-last-child(-n + 2) {
    border-bottom: 0;
  }
  .automatic-resources label:has(input:checked) {
    background: var(--color-accent-softer);
    color: var(--color-accent-strong);
  }
  .schedule-safety {
    padding: var(--space-3);
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    border: 1px solid color-mix(in srgb, var(--color-info) 25%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-info) 5%, var(--color-paper-raised));
    color: var(--color-info);
  }
  .schedule-safety p {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .schedule-safety strong {
    color: var(--color-ink-secondary);
  }
  .schedule-form footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .schedule-form footer > span {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .backup-hero {
    min-height: 146px;
    padding: var(--space-5);
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    overflow: hidden;
    border-color: color-mix(in srgb, var(--color-accent) 24%, var(--color-rule));
    background:
      linear-gradient(115deg, color-mix(in srgb, var(--color-accent) 9%, var(--color-paper-raised)), var(--color-paper-raised) 58%),
      var(--color-paper-raised);
  }
  .backup-hero::after {
    content: '';
    width: 260px;
    height: 260px;
    position: absolute;
    right: 12%;
    top: -210px;
    border: 1px solid color-mix(in srgb, var(--color-accent) 16%, transparent);
    border-radius: 50%;
    box-shadow: 0 0 0 38px color-mix(in srgb, var(--color-accent) 3%, transparent), 0 0 0 76px color-mix(in srgb, var(--color-accent) 2%, transparent);
    pointer-events: none;
  }
  .backup-hero-copy {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .backup-hero-copy > div {
    display: grid;
    gap: 4px;
  }
  .backup-vault {
    width: 52px;
    height: 52px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid color-mix(in srgb, var(--color-accent) 28%, var(--color-rule));
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    color: var(--color-accent);
    box-shadow: var(--shadow-whisper);
  }
  .backup-hero h2,
  .backup-hero p {
    margin: 0;
  }
  .backup-hero h2 {
    font-size: var(--text-xl);
    letter-spacing: -.02em;
  }
  .backup-hero p {
    max-width: 680px;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.5;
  }
  .backup-hero code,
  .backup-recovery-note code {
    color: var(--color-ink-secondary);
    font-family: var(--font-mono);
    font-size: .92em;
  }
  .backup-hero dl {
    margin: 0;
    position: relative;
    z-index: 1;
    display: flex;
  }
  .backup-hero dl div {
    min-width: 124px;
    padding: 0 var(--space-4);
    display: grid;
    gap: 1px;
    border-left: 1px solid var(--color-rule);
  }
  .backup-hero dt {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: .06em;
    text-transform: uppercase;
  }
  .backup-hero dd {
    margin: 0;
    font-size: var(--text-xl);
    font-weight: 680;
    line-height: 1.2;
  }
  .backup-hero dd.online {
    color: var(--color-success);
  }
  .backup-hero dl small {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    white-space: nowrap;
  }
  .backup-alert {
    margin-top: var(--space-4);
  }
  .backup-empty {
    min-height: 250px;
    display: grid;
    grid-template-columns: 54px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-6);
  }
  .backup-empty > span {
    width: 54px;
    height: 54px;
    display: grid;
    place-items: center;
    border: 1px dashed color-mix(in srgb, var(--color-accent) 35%, var(--color-rule));
    border-radius: var(--radius-md);
    background: var(--color-accent-softer);
    color: var(--color-accent);
  }
  .backup-empty > div {
    display: grid;
    gap: 3px;
  }
  .backup-empty h2,
  .backup-empty p {
    margin: 0;
  }
  .backup-empty h2 {
    font-size: var(--text-lg);
  }
  .backup-empty p {
    max-width: 660px;
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .backup-command-grid {
    margin-top: var(--space-4);
    display: grid;
    grid-template-columns: minmax(0, 1.55fr) minmax(300px, .75fr);
    gap: var(--space-4);
  }
  .backup-now-card {
    min-width: 0;
    padding: var(--space-5);
    display: grid;
    gap: var(--space-5);
  }
  .backup-now-card > header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
  }
  .backup-now-card > header > div {
    display: grid;
    gap: 3px;
  }
  .backup-now-card h2,
  .backup-now-card p {
    margin: 0;
  }
  .backup-now-card h2 {
    font-size: var(--text-lg);
  }
  .backup-now-card p {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .queue-mark {
    min-height: 25px;
    padding: 0 8px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid color-mix(in srgb, var(--color-success) 25%, var(--color-rule));
    border-radius: 999px;
    background: var(--color-success-soft);
    color: var(--color-success);
    font-size: var(--text-2xs);
    font-weight: 700;
    white-space: nowrap;
  }
  .queue-mark > i:not(.spinner) {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
  }
  .queue-mark.busy {
    border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-rule));
    background: var(--color-accent-softer);
    color: var(--color-accent);
  }
  .backup-destination-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: var(--space-3);
  }
  .backup-now-button {
    min-height: 38px;
    padding-inline: var(--space-4);
  }
  .backup-now-card > footer {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--color-muted);
    font-size: var(--text-2xs);
  }
  .backup-contents {
    min-width: 0;
    overflow: hidden;
  }
  .backup-contents > header {
    padding: var(--space-4);
    display: grid;
    gap: 2px;
    border-bottom: 1px solid var(--color-rule);
  }
  .backup-contents > header strong {
    font-size: var(--text-md);
  }
  .backup-contents ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .backup-contents li {
    min-height: 64px;
    padding: var(--space-3) var(--space-4);
    display: grid;
    grid-template-columns: 30px minmax(0, 1fr) 20px;
    align-items: center;
    gap: var(--space-2);
    border-bottom: 1px solid var(--color-rule);
  }
  .backup-contents li > i {
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    background: var(--color-paper-subtle);
    color: var(--color-ink-secondary);
  }
  .backup-contents li > span {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .backup-contents li strong {
    font-size: var(--text-xs);
  }
  .backup-contents li small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-2xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .backup-contents li > b {
    color: var(--color-success);
  }
  .backup-contents > footer {
    min-height: 43px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    background: var(--color-paper-subtle);
  }
  .backup-contents > footer code {
    color: var(--color-muted);
    font-family: var(--font-mono);
    font-size: var(--text-2xs);
  }
  .backup-schedule-card {
    margin-top: var(--space-4);
    overflow: hidden;
  }
  .backup-schedule-form {
    padding: var(--space-4) var(--space-5) var(--space-5);
    display: grid;
    gap: var(--space-4);
  }
  .backup-schedule-fields {
    display: grid;
    grid-template-columns: minmax(230px, 1.4fr) repeat(3, minmax(130px, .65fr));
    gap: var(--space-3);
  }
  .backup-schedule-fields .destination-field {
    grid-column: auto;
  }
  .backup-schedule-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }
  .backup-schedule-foot > span {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .backup-history {
    margin-top: var(--space-4);
    overflow: hidden;
  }
  .backup-history > .panel-header {
    min-height: 76px;
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--color-rule);
  }
  .backup-history-empty {
    min-height: 170px;
    display: grid;
    place-items: center;
    align-content: center;
    gap: var(--space-1);
    color: var(--color-muted);
  }
  .backup-history-empty strong {
    color: var(--color-ink-secondary);
    font-size: var(--text-sm);
  }
  .backup-history-empty span {
    font-size: var(--text-xs);
  }
  .backup-job-list {
    display: grid;
  }
  .backup-job {
    min-height: 76px;
    padding: var(--space-3) var(--space-5);
    display: grid;
    grid-template-columns: 38px minmax(240px, 1fr) 90px 112px 110px;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
    transition: background var(--duration-fast) var(--ease-out);
  }
  .backup-job:last-child {
    border-bottom: 0;
  }
  .backup-job:hover {
    background: var(--color-paper-subtle);
  }
  .job-kind {
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-subtle);
    color: var(--color-accent);
  }
  .backup-job.restore .job-kind {
    color: var(--color-warning);
  }
  .job-main {
    min-width: 0;
    display: grid;
    gap: 2px;
  }
  .job-main > strong {
    overflow: hidden;
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .job-main > span,
  .job-main > small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-2xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .job-main > small {
    color: var(--color-danger);
  }
  .job-size {
    display: grid;
    justify-items: end;
    gap: 1px;
  }
  .job-size strong {
    font: 600 var(--text-xs) var(--font-mono);
  }
  .job-size span {
    color: var(--color-muted);
    font-size: var(--text-2xs);
  }
  .job-status {
    min-height: 24px;
    padding: 0 8px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: 1px solid var(--color-rule);
    border-radius: 999px;
    background: var(--color-paper-subtle);
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 700;
    text-transform: capitalize;
  }
  .job-status.succeeded {
    border-color: color-mix(in srgb, var(--color-success) 28%, var(--color-rule));
    background: var(--color-success-soft);
    color: var(--color-success);
  }
  .job-status.failed {
    border-color: color-mix(in srgb, var(--color-danger) 28%, var(--color-rule));
    background: color-mix(in srgb, var(--color-danger) 7%, var(--color-paper-raised));
    color: var(--color-danger);
  }
  .job-status.running,
  .job-status.queued {
    border-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-rule));
    background: var(--color-accent-softer);
    color: var(--color-accent);
  }
  .job-status > i:not(.spinner) {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
  }
  .job-action {
    display: flex;
    justify-content: flex-end;
  }
  .job-action > span {
    color: var(--color-muted);
    font-size: var(--text-2xs);
    text-align: right;
  }
  .backup-recovery-note {
    margin-top: var(--space-4);
    padding: var(--space-3) var(--space-4);
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    border: 1px solid color-mix(in srgb, var(--color-warning) 24%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-warning) 5%, var(--color-paper-raised));
    color: var(--color-warning);
  }
  .backup-recovery-note p {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .backup-recovery-note strong {
    color: var(--color-ink-secondary);
  }

  @media (max-width: 76rem) {
    .metric-grid {
      grid-template-columns: repeat(3, 1fr);
    }
    .metric-card:nth-child(3) {
      border-right: 0;
    }
    .metric-card:nth-child(-n + 3) {
      border-bottom: 1px solid var(--color-rule);
    }
    .backup-command-grid {
      grid-template-columns: 1fr;
    }
    .backup-schedule-fields {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
    .backup-schedule-fields .destination-field {
      grid-column: 1 / -1;
    }
  }
  @media (max-width: 58rem) {
    .control-service-grid {
      grid-template-columns: 1fr;
    }
    .control-log-console > header {
      min-height: 0;
      padding-block: var(--space-3);
      align-items: flex-start;
      flex-direction: column;
    }
    .console-actions {
      width: 100%;
    }
    .cleanup-layout {
      grid-template-columns: 1fr;
    }
    .cleanup-confirm {
      position: static;
    }
    .schedule-body {
      grid-template-columns: 1fr;
    }
    .schedule-summary {
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .backup-hero {
      align-items: flex-start;
      flex-direction: column;
    }
    .backup-hero dl {
      width: 100%;
    }
    .backup-hero dl div:first-child {
      padding-left: 0;
      border-left: 0;
    }
    .backup-job {
      grid-template-columns: 38px minmax(0, 1fr) 105px;
    }
    .job-size {
      display: none;
    }
    .job-action {
      grid-column: 3;
    }
  }
  @media (max-width: 40rem) {
    .node-signal {
      align-items: flex-start;
      flex-direction: column;
    }
    .metric-grid {
      grid-template-columns: 1fr;
    }
    .metric-card,
    .metric-card:nth-child(3) {
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .metric-card:last-child {
      border-bottom: 0;
    }
    .pressure-chart {
      gap: 2px;
    }
    .control-plane-header {
      align-items: flex-start;
      flex-direction: column;
    }
    .control-plane-total {
      justify-items: start;
    }
    .control-service-grid {
      padding-inline: var(--space-3);
    }
    .control-log-console {
      margin-inline: var(--space-3);
    }
    .console-title {
      max-width: 100%;
      flex-wrap: wrap;
    }
    .console-title code {
      max-width: 170px;
    }
    .console-actions {
      flex-wrap: wrap;
    }
    .control-log-console > footer {
      min-height: 46px;
      padding-block: var(--space-2);
      align-items: flex-start;
      flex-direction: column;
      justify-content: center;
      gap: 2px;
    }
    .cleanup-intro {
      align-items: flex-start;
      flex-direction: column;
    }
    .reclaim-stat {
      padding: var(--space-3) 0 0;
      border-left: 0;
      border-top: 1px solid var(--color-rule);
      width: 100%;
      text-align: left;
    }
    .cleanup-list label {
      grid-template-columns: 18px minmax(0, 1fr) auto;
    }
    .cleanup-list label > i {
      display: none;
    }
    .schedule-header,
    .schedule-form footer {
      align-items: flex-start;
      flex-direction: column;
    }
    .schedule-state {
      margin-left: 54px;
    }
    .schedule-fields,
    .automatic-resources > div {
      grid-template-columns: 1fr;
    }
    .automatic-resources label,
    .automatic-resources label:nth-child(odd),
    .automatic-resources label:nth-last-child(-n + 2) {
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .automatic-resources label:last-child {
      border-bottom: 0;
    }
    .schedule-form footer button {
      width: 100%;
    }
    .view-switch button {
      padding-inline: var(--space-2);
      font-size: var(--text-xs);
    }
    .backup-hero-copy {
      align-items: flex-start;
    }
    .backup-vault {
      width: 42px;
      height: 42px;
    }
    .backup-hero dl {
      flex-direction: column;
      gap: var(--space-3);
    }
    .backup-hero dl div,
    .backup-hero dl div:first-child {
      padding: 0;
      border: 0;
    }
    .backup-empty {
      grid-template-columns: 1fr;
    }
    .backup-empty .btn {
      width: 100%;
    }
    .backup-now-card > header,
    .backup-destination-row,
    .backup-schedule-foot {
      align-items: stretch;
      grid-template-columns: 1fr;
      flex-direction: column;
    }
    .backup-destination-row {
      display: grid;
    }
    .backup-now-button,
    .backup-schedule-foot .btn {
      width: 100%;
    }
    .backup-schedule-fields {
      grid-template-columns: 1fr;
    }
    .backup-schedule-fields .destination-field {
      grid-column: auto;
    }
    .backup-job {
      padding-inline: var(--space-3);
      grid-template-columns: 34px minmax(0, 1fr);
      gap: var(--space-2);
    }
    .job-status,
    .job-action {
      grid-column: 2;
      justify-self: start;
    }
    .job-action {
      justify-content: flex-start;
    }
  }
</style>
