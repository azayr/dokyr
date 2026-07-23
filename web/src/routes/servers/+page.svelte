<script>
  import { onDestroy, onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  const infrastructureViews = ['monitoring', 'cleanup'];
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

  $: selectedBytes = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.bytes || 0 : 0), 0);
  $: selectedItems = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.count || 0 : 0), 0);
  $: hostDiskAvailable = (metrics.global.disk?.total || 0) > 0;
  $: automaticResourceCount = ['containers', 'images', 'buildCache', 'networks'].filter((key) => cleanupSchedule[key]).length;

  onMount(async () => {
    hashListener = syncViewFromURL;
    syncViewFromURL();
    window.addEventListener('hashchange', hashListener);

    await loadMetrics();
    pollTimer = setInterval(() => {
      if (activeView === 'monitoring' && !metricsRefreshing) loadMetrics(true);
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

  function activateView(view) {
    activeView = view;
    if (view === 'cleanup') {
      if (!cleanupLoaded && !cleanupLoading) loadCleanup();
      if (!cleanupScheduleLoaded && !cleanupScheduleLoading) loadCleanupSchedule();
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
  {:else}
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
  {/if}
</Shell>

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
  }
  @media (max-width: 58rem) {
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
  }
</style>
