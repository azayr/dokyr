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
    return `${Math.max(2, Math.min(100, value / maximum * 100))}%`;
  }

</script>

<Shell eyebrow="Infrastructure" title="Docker node">
  <section class="node-signal">
    <div class="node-identity"><span class="engine-light" class:offline={Boolean(metricsError)}></span><div><strong>{metrics.engineName || 'local-docker'}</strong><small>Docker socket · single-node control plane</small></div></div>
    <div class="view-switch" aria-label="Infrastructure view">
      <button class:active={activeView === 'monitoring'} onclick={() => showView('monitoring')}><Icon name="activity" size={15}/> Monitoring</button>
      <button class:active={activeView === 'cleanup'} onclick={() => showView('cleanup')}><Icon name="settings" size={15}/> Cleanup</button>
    </div>
    <div class="refresh-state"><i class:spinning={metricsRefreshing}></i><span>{metrics.checkedAt ? `Updated ${new Date(metrics.checkedAt).toLocaleTimeString()}` : 'Connecting…'}</span></div>
  </section>

  {#if activeView === 'monitoring'}
    {#if metricsError}<div class="feedback error"><strong>Monitoring unavailable</strong><span>{metricsError}</span><button onclick={() => loadMetrics()}>Retry</button></div>{/if}
    {#if metricsLoading}
      <section class="loading-state"><i></i><div><strong>Sampling Docker workloads</strong><span>Reading CPU, memory, storage and network counters…</span></div></section>
    {:else}
      <section class="metric-grid">
        <article class="metric-card cpu-card"><header><span>CPU load</span><em>{metrics.global.cpuCores} cores</em></header><strong>{percent(metrics.global.cpuPercent)}</strong><div class="meter"><i style={'width:' + width(metrics.global.cpuPercent)}></i></div><small>Normalized across the Docker host</small></article>
        <article class="metric-card"><header><span>Memory</span><em>{formatBytes(metrics.global.memoryLimit)}</em></header><strong>{formatBytes(metrics.global.memoryUsage)}</strong><div class="meter"><i style={'width:' + width(metrics.global.memoryPercent)}></i></div><small>{percent(metrics.global.memoryPercent)} of host memory</small></article>
        <article class="metric-card"><header><span>Disk I/O</span><em>host devices</em></header><strong>{formatBytes(metrics.global.diskIo.read)}</strong><div class="io-pair"><span>Read</span><b>{formatBytes(metrics.global.diskIo.read)}</b><span>Written</span><b>{formatBytes(metrics.global.diskIo.write)}</b></div></article>
        <article class="metric-card"><header><span>{hostDiskAvailable ? 'Disk space' : 'Docker storage'}</span><em>{hostDiskAvailable ? 'host filesystem' : 'Docker Desktop'}</em></header><strong>{formatBytes(hostDiskAvailable ? metrics.global.disk.used : metrics.global.disk.dockerUsed)}</strong><div class="io-pair"><span>{hostDiskAvailable ? 'Available' : 'Reclaimable'}</span><b>{formatBytes(hostDiskAvailable ? metrics.global.disk.available : metrics.global.disk.reclaimable)}</b><span>{hostDiskAvailable ? 'Total' : 'Allocated'}</span><b>{formatBytes(hostDiskAvailable ? metrics.global.disk.total : metrics.global.disk.dockerUsed)}</b></div></article>
        <article class="metric-card"><header><span>Network I/O</span><em>all interfaces</em></header><strong>{formatBytes(metrics.global.networkIo.receive)}</strong><div class="io-pair"><span>Received</span><b>{formatBytes(metrics.global.networkIo.receive)}</b><span>Sent</span><b>{formatBytes(metrics.global.networkIo.transmit)}</b></div></article>
      </section>

      <section class="pressure-panel">
        <header><div><span>Live pressure</span><h2>Node activity</h2></div><div class="legend"><span><i class="cpu"></i>CPU</span><span><i class="memory"></i>Memory</span></div></header>
        <div class="pressure-chart" aria-label="Recent CPU and memory samples">
          {#each history as sample}
            <div class="sample"><i class="memory" style={'height:' + width(sample.memory)}></i><i class="cpu" style={'height:' + width(sample.cpu)}></i></div>
          {/each}
          {#if history.length < 24}{#each Array(24 - history.length) as _}<div class="sample empty"></div>{/each}{/if}
        </div>
        <footer><span>Last {history.length} sample{history.length === 1 ? '' : 's'} · refreshes every 5 seconds</span><b>{metrics.global.running} running / {metrics.global.containers} total</b></footer>
      </section>

    {/if}
  {:else}
    <section class="cleanup-intro">
      <div><span>Storage maintenance</span><h2>Docker cleanup</h2><p>Review unused resources before removing them from this node. Running containers and resources they reference are never selected by Docker's prune operations.</p></div>
      <div class="reclaim-orbit"><strong>{formatBytes(cleanup.totalReclaimable)}</strong><span>potentially reclaimable</span></div>
    </section>

    {#if cleanupError}<div class="feedback error"><strong>Cleanup unavailable</strong><span>{cleanupError}</span><button onclick={loadCleanup}>Retry</button></div>{/if}
    {#if cleanupResult}<div class="feedback success"><strong>Cleanup complete</strong><span>Removed {cleanupResult.deleted} resource{cleanupResult.deleted === 1 ? '' : 's'} and reclaimed {formatBytes(cleanupResult.spaceReclaimed)}.</span></div>{/if}

    <form class="cleanup-layout" onsubmit={(event) => { event.preventDefault(); runCleanup(); }}>
      <section class="cleanup-options">
        <header><div><span>Cleanup plan</span><h3>Select resources</h3></div><button type="button" onclick={loadCleanup} disabled={cleanupLoading}>{cleanupLoading ? 'Inspecting…' : 'Refresh preview'}</button></header>
        <div class="cleanup-list">
          <label><input type="checkbox" bind:checked={cleanupSelection.containers}/><i><Icon name="box" size={16}/></i><span><strong>Stopped containers</strong><small>Containers that are no longer running.</small></span><em>{cleanup.containers?.count || 0}<small>{formatBytes(cleanup.containers?.bytes)}</small></em></label>
          <label><input type="checkbox" bind:checked={cleanupSelection.images}/><i><Icon name="grid" size={16}/></i><span><strong>Unused images</strong><small>Images not referenced by any container.</small></span><em>{cleanup.images?.count || 0}<small>{formatBytes(cleanup.images?.bytes)}</small></em></label>
          <label><input type="checkbox" bind:checked={cleanupSelection.buildCache}/><i><Icon name="activity" size={16}/></i><span><strong>Build cache</strong><small>Build layers that are not currently in use.</small></span><em>{cleanup.buildCache?.count || 0}<small>{formatBytes(cleanup.buildCache?.bytes)}</small></em></label>
          <label><input type="checkbox" bind:checked={cleanupSelection.networks}/><i><Icon name="globe" size={16}/></i><span><strong>Unused networks</strong><small>Custom networks with no attached containers.</small></span><em>{cleanup.networks?.count || 0}<small>metadata</small></em></label>
          <label class="volume-option"><input type="checkbox" bind:checked={cleanupSelection.volumes}/><i><Icon name="database" size={16}/></i><span><strong>Unused volumes</strong><small>Persistent data not attached to a container. Review carefully.</small></span><em>{cleanup.volumes?.count || 0}<small>{formatBytes(cleanup.volumes?.bytes)}</small></em></label>
        </div>
      </section>

      <aside class="cleanup-confirm">
        <div class="impact"><span>Selected impact</span><strong>{formatBytes(selectedBytes)}</strong><small>{selectedItems} resource{selectedItems === 1 ? '' : 's'} in the current preview</small></div>
        {#if cleanupSelection.volumes}<div class="volume-warning"><strong>Persistent data selected</strong><p>Unused named volumes may contain data you still need. This cannot be undone.</p></div>{/if}
        <label><span>Type <code>CLEAN DOCKER</code> to confirm</span><input bind:value={cleanupConfirmation} placeholder="CLEAN DOCKER" autocomplete="off" spellcheck="false" /></label>
        <button class="cleanup-button" type="submit" disabled={cleanupRunning || cleanupConfirmation !== 'CLEAN DOCKER' || selectedItems === 0}>{cleanupRunning ? 'Cleaning Docker…' : `Run cleanup · ${formatBytes(selectedBytes)}`}</button>
        <p>Docker only removes resources that are unused at execution time. The preview is refreshed after cleanup.</p>
      </aside>
    </form>
  {/if}
</Shell>

<style>
  .node-signal { min-height: 64px; margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: minmax(220px, 1fr) auto minmax(190px, 1fr); align-items: center; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .node-identity { display: flex; align-items: center; gap: var(--space-3); }
  .node-identity > div { display: grid; gap: 2px; }
  .node-identity strong { font-size: 12px; } .node-identity small { color: var(--color-muted); font: 9px var(--font-mono); }
  .engine-light { width: 8px; height: 8px; border-radius: 50%; background: var(--color-accent); box-shadow: 0 0 0 5px var(--color-accent-soft); }
  .engine-light.offline { background: var(--color-danger); box-shadow: 0 0 0 5px color-mix(in oklch, var(--color-danger) 10%, transparent); }
  .view-switch { padding: 3px; display: flex; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); }
  .view-switch button { min-height: 34px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 0; border-radius: 4px; background: transparent; color: var(--color-muted); font-size: 11px; font-weight: 600; cursor: pointer; }
  .view-switch button.active { background: var(--color-paper-raised); color: var(--color-ink); box-shadow: var(--shadow-whisper); }
  .refresh-state { display: flex; align-items: center; justify-content: flex-end; gap: var(--space-2); color: var(--color-muted); font: 9px var(--font-mono); }
  .refresh-state i { width: 8px; height: 8px; border: 1px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; }
  .refresh-state i.spinning { animation: spin .7s linear infinite; }
  .feedback { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); font-size: 11px; }
  .feedback span { color: var(--color-muted); } .feedback button { border: 0; background: transparent; color: var(--color-accent); font-weight: 700; cursor: pointer; }
  .feedback.error { border-color: color-mix(in oklch, var(--color-danger) 35%, var(--color-rule)); background: color-mix(in oklch, var(--color-danger) 6%, var(--color-paper-raised)); }
  .feedback.success { border-color: color-mix(in oklch, var(--color-accent) 35%, var(--color-rule)); background: var(--color-accent-soft); }
  .loading-state { min-height: 320px; display: flex; align-items: center; justify-content: center; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .loading-state > i { width: 28px; height: 28px; border: 2px solid var(--color-rule); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .loading-state div { display: grid; gap: var(--space-1); } .loading-state strong { font-size: 13px; } .loading-state span { color: var(--color-muted); font-size: 10px; }
  .metric-grid { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .metric-card { min-width: 0; padding: var(--space-5); display: grid; align-content: start; gap: var(--space-3); border-right: 1px solid var(--color-rule); }
  .metric-card:last-child { border-right: 0; }
  .metric-card header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
  .metric-card header span { color: var(--color-muted); font: 700 9px var(--font-mono); letter-spacing: .06em; text-transform: uppercase; }
  .metric-card header em { overflow: hidden; color: var(--color-faint); font: 8px var(--font-mono); font-style: normal; text-overflow: ellipsis; white-space: nowrap; }
  .metric-card > strong { overflow: hidden; font: 600 23px var(--font-sans); letter-spacing: -.04em; text-overflow: ellipsis; white-space: nowrap; }
  .metric-card > small { color: var(--color-muted); font-size: 9px; }
  .meter { height: 4px; overflow: hidden; border-radius: 4px; background: var(--color-paper-subtle); }
  .meter i { display: block; height: 100%; border-radius: inherit; background: var(--color-accent); transition: width var(--duration-base) var(--ease-out); }
  .io-pair { display: grid; grid-template-columns: 1fr auto; gap: 5px var(--space-2); color: var(--color-muted); font-size: 9px; }
  .io-pair b { color: var(--color-ink-secondary); font: 9px var(--font-mono); font-weight: 500; }
  .pressure-panel, .cleanup-options, .cleanup-confirm { border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .pressure-panel { margin-bottom: var(--space-4); overflow: hidden; }
  .pressure-panel > header, .cleanup-options > header { min-height: 62px; padding: 0 var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .pressure-panel header > div:first-child, .cleanup-options header > div { display: grid; gap: 3px; }
  .pressure-panel header span:first-child, .cleanup-options header span { color: var(--color-muted); font: 8px var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
  .pressure-panel h2, .cleanup-options h3 { margin: 0; font-size: 13px; }
  .legend { display: flex !important; grid-auto-flow: column; gap: var(--space-3) !important; }
  .legend span { display: inline-flex; align-items: center; gap: 5px; color: var(--color-muted) !important; font: 9px var(--font-sans) !important; text-transform: none !important; }
  .legend i { width: 7px; height: 7px; border-radius: 2px; } .legend i.cpu { background: var(--color-accent); } .legend i.memory { background: var(--color-info); }
  .pressure-chart { height: 120px; padding: var(--space-5) var(--space-4) var(--space-3); display: grid; grid-template-columns: repeat(24, 1fr); align-items: end; gap: 4px; background-image: linear-gradient(to bottom, transparent 24%, var(--color-rule) 25%, transparent 26%, transparent 49%, var(--color-rule) 50%, transparent 51%, transparent 74%, var(--color-rule) 75%, transparent 76%); }
  .sample { height: 100%; position: relative; display: flex; align-items: end; gap: 1px; }
  .sample i { min-height: 2px; flex: 1; border-radius: 2px 2px 0 0; transition: height var(--duration-base) var(--ease-out); }
  .sample i.cpu { background: var(--color-accent); } .sample i.memory { background: var(--color-info); opacity: .7; } .sample.empty { height: 1px; background: var(--color-rule); }
  .pressure-panel footer { min-height: 38px; padding: 0 var(--space-4); display: flex; align-items: center; justify-content: space-between; color: var(--color-muted); font: 8px var(--font-mono); border-top: 1px solid var(--color-rule); }
  .pressure-panel footer b { color: var(--color-ink-secondary); font-weight: 500; }
  .cleanup-intro { min-height: 146px; margin-bottom: var(--space-4); padding: var(--space-6); display: flex; align-items: center; justify-content: space-between; gap: var(--space-8); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: linear-gradient(100deg, var(--color-paper-raised), var(--color-paper-subtle)); }
  .cleanup-intro > div:first-child { max-width: 650px; } .cleanup-intro span { color: var(--color-accent); font: 700 8px var(--font-mono); letter-spacing: .09em; text-transform: uppercase; } .cleanup-intro h2 { margin: var(--space-2) 0; font-size: 22px; letter-spacing: -.03em; } .cleanup-intro p { margin: 0; color: var(--color-muted); font-size: 11px; line-height: 1.6; }
  .reclaim-orbit { width: 128px; height: 128px; flex: 0 0 auto; display: grid; place-content: center; text-align: center; border: 1px solid var(--color-rule); border-radius: 50%; background: radial-gradient(circle, var(--color-paper-raised) 56%, transparent 57%), conic-gradient(var(--color-accent) 68%, var(--color-rule) 0); }
  .reclaim-orbit strong { font: 600 17px var(--font-sans); } .reclaim-orbit span { max-width: 80px; margin: 3px auto 0; color: var(--color-muted); font: 7px var(--font-mono); line-height: 1.4; }
  .cleanup-layout { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(300px, .65fr); align-items: start; gap: var(--space-4); }
  .cleanup-options { overflow: hidden; }
  .cleanup-options > header button { min-height: 32px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-muted); font-size: 9px; font-weight: 600; cursor: pointer; }
  .cleanup-list label { min-height: 72px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 18px 36px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); cursor: pointer; }
  .cleanup-list label:last-child { border-bottom: 0; } .cleanup-list label:has(input:checked) { background: var(--color-accent-soft); }
  .cleanup-list input { accent-color: var(--color-accent); }
  .cleanup-list label > i { width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); color: var(--color-muted); background: var(--color-paper-raised); }
  .cleanup-list label > span { display: grid; gap: 4px; } .cleanup-list label > span strong { font-size: 11px; } .cleanup-list label > span small { color: var(--color-muted); font-size: 9px; }
  .cleanup-list em { min-width: 70px; display: grid; gap: 3px; color: var(--color-ink); font: 600 14px var(--font-mono); font-style: normal; text-align: right; } .cleanup-list em small { color: var(--color-muted); font: 8px var(--font-mono); }
  .cleanup-list .volume-option { border-top: 1px dashed color-mix(in oklch, var(--color-danger) 32%, var(--color-rule)); }
  .cleanup-list .volume-option:has(input:checked) { background: color-mix(in oklch, var(--color-danger) 8%, var(--color-paper-raised)); }
  .cleanup-confirm { padding: var(--space-5); position: sticky; top: var(--space-4); }
  .impact { padding-bottom: var(--space-5); display: grid; gap: var(--space-2); border-bottom: 1px solid var(--color-rule); } .impact span { color: var(--color-muted); font: 8px var(--font-mono); text-transform: uppercase; } .impact strong { font: 600 28px var(--font-sans); letter-spacing: -.04em; } .impact small { color: var(--color-muted); font-size: 9px; }
  .volume-warning { margin-top: var(--space-4); padding: var(--space-3); border-left: 3px solid var(--color-danger); background: color-mix(in oklch, var(--color-danger) 7%, var(--color-paper-raised)); } .volume-warning strong { color: var(--color-danger); font-size: 10px; } .volume-warning p { margin: 4px 0 0; color: var(--color-muted); font-size: 9px; line-height: 1.5; }
  .cleanup-confirm > label { margin-top: var(--space-5); display: grid; gap: var(--space-2); } .cleanup-confirm label span { font-size: 9px; font-weight: 600; } .cleanup-confirm label code { color: var(--color-danger); }
  .cleanup-confirm input { height: 42px; padding: 0 var(--space-3); outline: 2px solid transparent; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font: 11px var(--font-mono); } .cleanup-confirm input:focus-visible { outline-color: var(--color-focus); }
  .cleanup-button { width: 100%; min-height: 42px; margin-top: var(--space-3); border: 1px solid var(--color-danger); border-radius: var(--radius-sm); background: var(--color-danger); color: white; font-size: 10px; font-weight: 700; cursor: pointer; } .cleanup-button:disabled { border-color: var(--color-rule); background: var(--color-paper-subtle); color: var(--color-faint); cursor: not-allowed; }
  .cleanup-confirm > p { margin: var(--space-3) 0 0; color: var(--color-muted); font-size: 8px; line-height: 1.5; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (max-width: 76rem) { .metric-grid { grid-template-columns: repeat(3, 1fr); } .metric-card:nth-child(3) { border-right: 0; } .metric-card:nth-child(-n+3) { border-bottom: 1px solid var(--color-rule); } }
  @media (max-width: 58rem) { .node-signal { grid-template-columns: 1fr auto; } .refresh-state { grid-row: 2; grid-column: 1 / -1; justify-content: flex-start; } .cleanup-layout { grid-template-columns: 1fr; } .cleanup-confirm { position: static; } }
  @media (max-width: 40rem) { .node-signal { grid-template-columns: 1fr; } .view-switch { width: 100%; } .view-switch button { flex: 1; justify-content: center; } .refresh-state { grid-row: auto; grid-column: auto; } .metric-grid { grid-template-columns: 1fr; } .metric-card { border-right: 0; border-bottom: 1px solid var(--color-rule); } .metric-card:nth-child(3) { border-right: 0; } .metric-card:last-child { border-bottom: 0; } .pressure-chart { gap: 2px; } .cleanup-intro { align-items: flex-start; flex-direction: column; } .reclaim-orbit { width: 104px; height: 104px; } .cleanup-list label { grid-template-columns: 18px minmax(0, 1fr) auto; } .cleanup-list label > i { display: none; } }
  @media (prefers-reduced-motion: reduce) { .refresh-state i.spinning, .loading-state > i { animation: none; } }
</style>
