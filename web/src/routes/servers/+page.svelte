<script>
  import { onDestroy, onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  let activeView = 'monitoring';
  let metrics = { engineName: 'local-docker', global: { diskIo: {}, networkIo: {}, disk: {} } };
  let metricsLoading = true;
  let metricsRefreshing = false;
  let metricsError = '';
  let history = [];
  let pollTimer;
  let cleanup = { containers: {}, images: {}, buildCache: {}, networks: {}, volumes: {} };
  let cleanupLoading = false;
  let cleanupLoaded = false;
  let cleanupError = '';
  let cleanupSelection = { containers: true, images: true, buildCache: true, networks: true, volumes: false };
  let cleanupConfirmation = '';
  let cleanupRunning = false;
  let cleanupResult = null;

  $: selectedBytes = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.bytes || 0 : 0), 0);
  $: selectedItems = ['containers', 'images', 'buildCache', 'networks', 'volumes'].reduce((total, key) => total + (cleanupSelection[key] ? cleanup[key]?.count || 0 : 0), 0);
  $: hostDiskAvailable = (metrics.global.disk?.total || 0) > 0;

  onMount(async () => {
    await loadMetrics();
    pollTimer = setInterval(() => {
      if (activeView === 'monitoring' && !metricsRefreshing) loadMetrics(true);
    }, 5000);
  });

  onDestroy(() => clearInterval(pollTimer));

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

  function showView(view) {
    activeView = view;
    if (view === 'cleanup' && !cleanupLoaded && !cleanupLoading) loadCleanup();
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
  }
</style>
