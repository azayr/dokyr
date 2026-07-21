<script>
  import { onDestroy, onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { api } from '$lib/auth.js';

  let data = { projects: [], deployments: [], docker: {} };
  let metrics = {
    checkedAt: '',
    engineName: '',
    global: {
      cpuPercent: 0,
      memoryUsage: 0,
      memoryLimit: 0,
      memoryPercent: 0,
      cpuCores: 0,
      containers: 0,
      running: 0,
      diskIo: { read: 0, write: 0 },
      networkIo: { receive: 0, transmit: 0 },
      disk: { total: 0, used: 0, available: 0, dockerUsed: 0, reclaimable: 0 }
    }
  };
  let loading = true;
  let metricsLoading = true;
  let metricsRefreshing = false;
  let metricsError = '';
  let pollTimer;

  $: engineOnline = Boolean(metrics.checkedAt) || Boolean(data.docker.connected);
  $: cpuPercent = clamp(metrics.global.cpuPercent);
  $: memoryPercent = clamp(metrics.global.memoryPercent);
  $: nodePressure = Math.max(cpuPercent, memoryPercent);
  $: diskTotal = metrics.global.disk?.total || 0;
  $: hostDiskAvailable = diskTotal > 0;
  $: diskUsed = hostDiskAvailable ? metrics.global.disk?.used || 0 : metrics.global.disk?.dockerUsed || 0;
  $: diskAvailable = hostDiskAvailable ? metrics.global.disk?.available || 0 : metrics.global.disk?.reclaimable || 0;
  $: displayedDiskTotal = hostDiskAvailable ? diskTotal : diskUsed + diskAvailable;
  $: diskPercent = displayedDiskTotal > 0 ? clamp((diskUsed / displayedDiskTotal) * 100) : 0;
  $: activeDeployments = data.deployments.filter((item) => ['queued', 'building', 'deploying', 'running'].includes(item.status));
  $: failedDeployment = data.deployments.find((item) => item.status === 'failed');

  onMount(async () => {
    await Promise.all([loadDashboard(), loadMetrics()]);
    loading = false;
    pollTimer = setInterval(() => loadMetrics(true), 10000);
  });

  onDestroy(() => clearInterval(pollTimer));

  async function loadDashboard() {
    try {
      const response = await api('/api/dashboard');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load the dashboard');
      data = payload;
    } catch {
      data = { projects: [], deployments: [], docker: {} };
    }
  }

  async function loadMetrics(silent = false) {
    if (silent) metricsRefreshing = true;
    else metricsLoading = true;
    metricsError = '';
    try {
      const response = await api('/api/infrastructure/metrics');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not read Docker metrics');
      metrics = payload;
    } catch (cause) {
      metricsError = cause instanceof Error ? cause.message : 'Could not read Docker metrics';
    } finally {
      metricsLoading = false;
      metricsRefreshing = false;
    }
  }

  function ago(value) {
    if (!value) return 'not yet';
    const minutes = Math.max(1, Math.round((Date.now() - new Date(value).getTime()) / 60000));
    if (minutes >= 60) return `${Math.floor(minutes / 60)}h ago`;
    return `${minutes}m ago`;
  }

  function formatBytes(value = 0) {
    if (!Number.isFinite(value) || value <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    const amount = value / Math.pow(1024, index);
    return `${amount >= 100 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
  }

  function formatPercent(value = 0) {
    return `${clamp(value).toFixed(value >= 10 ? 1 : 2)}%`;
  }

  function clamp(value = 0) {
    return Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  }
</script>

<Shell
  eyebrow="Workspace"
  title="Overview"
  subtitle="System health, deployment activity, and infrastructure signals for this control plane."
  meta={[metrics.engineName || 'local-docker', engineOnline ? 'live' : 'offline']}
>
  <a slot="actions" class="btn btn-primary" href="/projects?new=1"><Icon name="plus" size={14} /> New project</a>

  <section class="metrics" aria-label="Workspace metrics">
    <article>
      <span class="metric-label">Projects</span>
      {#if loading}<span class="skeleton metric-skeleton"></span>{:else}<strong>{data.projects.length}</strong>{/if}
      <small><a href="/projects">View all projects</a></small>
    </article>
    <article>
      <span class="metric-label">Running services</span>
      {#if metricsLoading}<span class="skeleton metric-skeleton"></span>{:else}<strong>{metrics.global.running}</strong>{/if}
      <small class={engineOnline ? 'tone-success' : 'tone-danger'}>{engineOnline ? 'All systems normal' : 'Monitoring unavailable'}</small>
    </article>
    <article>
      <span class="metric-label">Active deployments</span>
      {#if loading}<span class="skeleton metric-skeleton"></span>{:else}<strong>{activeDeployments.length}</strong>{/if}
      <small class={activeDeployments.length ? 'tone-info' : ''}>{activeDeployments.length ? 'Pipeline running' : 'Pipeline idle'}</small>
    </article>
    <article>
      <span class="metric-label">Node pressure</span>
      {#if metricsLoading}<span class="skeleton metric-skeleton"></span>{:else}<strong>{formatPercent(nodePressure)}</strong>{/if}
      <small>{metricsLoading ? 'Sampling host' : `${metrics.global.containers} containers on this node`}</small>
    </article>
  </section>

  {#if failedDeployment}
    <a class="failure-strip" href={'/deployments/' + failedDeployment.id}>
      <Icon name="alert" size={15} />
      <span><strong>Deployment failed</strong> · {failedDeployment.projectId}{failedDeployment.commit ? ` · ${failedDeployment.commit}` : ''}</span>
      <em>View deployment <Icon name="arrow-right" size={13} /></em>
    </a>
  {/if}

  <section class="panel resources" aria-label="Live host monitoring">
    <header class="panel-header">
      <div>
        <span class="eyebrow">Live monitoring</span>
        <h2>Host resources</h2>
      </div>
      <a class="resources-link" href="/servers">
        <i class:refreshing={metricsRefreshing}></i>{metricsRefreshing ? 'Refreshing' : `Updated ${ago(metrics.checkedAt)}`}
        <span class="resources-link-more">· Servers <Icon name="arrow-right" size={12} /></span>
      </a>
    </header>
    <div class="resource-grid">
      <article>
        <div class="resource-title"><span class="resource-icon tone-accent"><Icon name="cpu" size={14} /></span><span>CPU</span></div>
        <strong>{metricsLoading ? '—' : formatPercent(cpuPercent)}</strong>
        <div class="meter"><i style={'width:' + cpuPercent + '%'}></i></div>
        <small>{metrics.global.cpuCores || '—'} cores · host load</small>
      </article>
      <article>
        <div class="resource-title"><span class="resource-icon tone-info"><Icon name="activity" size={14} /></span><span>Memory</span></div>
        <strong>{metricsLoading ? '—' : formatPercent(memoryPercent)}</strong>
        <div class="meter"><i style={'width:' + memoryPercent + '%'}></i></div>
        <small>{formatBytes(metrics.global.memoryUsage)} / {formatBytes(metrics.global.memoryLimit)}</small>
      </article>
      <article>
        <div class="resource-title"><span class="resource-icon tone-warning"><Icon name="hard-drive" size={14} /></span><span>Disk</span></div>
        <strong>{metricsLoading ? '—' : formatPercent(diskPercent)}</strong>
        <div class="meter"><i style={'width:' + diskPercent + '%'}></i></div>
        <small>{formatBytes(diskUsed)} used · {formatBytes(diskAvailable)} free</small>
      </article>
      <article>
        <div class="resource-title"><span class="resource-icon tone-muted"><Icon name="layers" size={14} /></span><span>Disk I/O</span></div>
        <strong>{metricsLoading ? '—' : formatBytes((metrics.global.diskIo?.read || 0) + (metrics.global.diskIo?.write || 0))}</strong>
        <dl><div><dt>Read</dt><dd>{formatBytes(metrics.global.diskIo?.read || 0)}</dd></div><div><dt>Write</dt><dd>{formatBytes(metrics.global.diskIo?.write || 0)}</dd></div></dl>
      </article>
      <article>
        <div class="resource-title"><span class="resource-icon tone-success"><Icon name="network" size={14} /></span><span>Network</span></div>
        <strong>{metricsLoading ? '—' : formatBytes((metrics.global.networkIo?.receive || 0) + (metrics.global.networkIo?.transmit || 0))}</strong>
        <dl><div><dt>In</dt><dd>{formatBytes(metrics.global.networkIo?.receive || 0)}</dd></div><div><dt>Out</dt><dd>{formatBytes(metrics.global.networkIo?.transmit || 0)}</dd></div></dl>
      </article>
    </div>
  </section>

  <div class="dashboard-grid">
    <section class="panel" aria-label="Recent deployments">
      <header class="panel-header">
        <div>
          <span class="eyebrow">Delivery</span>
          <h2>Recent deployments</h2>
        </div>
        <a class="panel-more" href="/deployments">View all <Icon name="arrow-right" size={12} /></a>
      </header>
      {#if loading}
        <div class="rows-loading" aria-label="Loading deployments">
          {#each Array(4) as _}<div class="row-skeleton"><span class="skeleton" style="width:32px;height:32px"></span><span class="skeleton" style="height:14px;flex:1"></span><span class="skeleton" style="width:64px;height:22px"></span></div>{/each}
        </div>
      {:else if data.deployments.length === 0}
        <EmptyState icon="rocket" title="No deployments yet" description="Create a project and ship its first release to see the pipeline here.">
          <a class="btn btn-primary btn-sm" href="/projects?new=1"><Icon name="plus" size={13} /> Create a project</a>
        </EmptyState>
      {:else}
        <div class="table-scroll">
          <table class="data-table deployment-table">
            <thead><tr><th>Deployment</th><th>Commit</th><th>Duration</th><th>Time</th><th>Status</th></tr></thead>
            <tbody>
              {#each data.deployments.slice(0, 8) as deployment}
                <tr onclick={() => (location.href = '/deployments/' + deployment.id)} class="clickable">
                  <td>
                    <span class="deployment-name">
                      <span class="deployment-icon"><Icon name="box" size={13} /></span>
                      <strong>{deployment.message || deployment.projectId}</strong>
                    </span>
                  </td>
                  <td><code>{deployment.commit || 'image'}</code></td>
                  <td><span class="muted">{deployment.duration ? `${deployment.duration}s` : '—'}</span></td>
                  <td><time>{ago(deployment.createdAt)}</time></td>
                  <td><Status value={deployment.status} /></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <aside class="rail">
      <section class="panel" aria-label="Infrastructure health">
        <header class="panel-header">
          <div>
            <span class="eyebrow">Infrastructure</span>
            <h2>Node health</h2>
          </div>
          <span class="badge" class:badge-success={engineOnline} class:badge-danger={!engineOnline}><i></i>{engineOnline ? 'Online' : 'Offline'}</span>
        </header>
        {#if metricsLoading}
          <div class="rail-loading"><span class="spinner"></span><span>Sampling Docker host…</span></div>
        {:else if metricsError && !metrics.checkedAt}
          <div class="rail-error">
            <Icon name="alert" size={16} />
            <p>{metricsError}</p>
            <button class="btn btn-sm" onclick={() => loadMetrics()}>Retry</button>
          </div>
        {:else}
          <div class="health-rows">
            <div class="health-row">
              <div><strong>{metrics.engineName || 'local-docker'}</strong><small>CPU load</small></div>
              <b>{formatPercent(cpuPercent)}</b>
            </div>
            <div class="health-row">
              <div><strong>Host memory</strong><small>{formatBytes(metrics.global.memoryUsage)} used</small></div>
              <b>{formatPercent(memoryPercent)}</b>
            </div>
            <div class="health-row">
              <div><strong>Storage</strong><small>{formatBytes(diskAvailable)} available</small></div>
              <b>{formatPercent(diskPercent)}</b>
            </div>
          </div>
          <footer class="panel-footer">
            <span>{metrics.global.running} running · {metrics.global.containers} total</span>
            <a href="/servers">Details <Icon name="arrow-right" size={12} /></a>
          </footer>
        {/if}
      </section>

      <section class="panel" aria-label="Projects">
        <header class="panel-header">
          <div>
            <span class="eyebrow">Workspace</span>
            <h2>Projects</h2>
          </div>
          <a class="panel-more" href="/projects">View all <Icon name="arrow-right" size={12} /></a>
        </header>
        {#if loading}
          <div class="rows-loading">
            {#each Array(3) as _}<div class="row-skeleton"><span class="skeleton" style="width:28px;height:28px"></span><span class="skeleton" style="height:14px;flex:1"></span></div>{/each}
          </div>
        {:else if data.projects.length === 0}
          <div class="rail-empty">
            <p>No projects yet.</p>
            <a class="btn btn-sm" href="/projects?new=1"><Icon name="plus" size={13} /> New project</a>
          </div>
        {:else}
          <div class="project-rows">
            {#each data.projects.slice(0, 5) as project}
              <a class="project-row" href={'/projects/' + project.id}>
                <span class="deployment-icon"><Icon name="box" size={13} /></span>
                <span class="project-row-text"><strong>{project.name}</strong><small>{project.branch || 'main'} · {project.repository || 'container image'}</small></span>
                <code>{project.updatedAt ? ago(project.updatedAt) : '—'}</code>
              </a>
            {/each}
          </div>
        {/if}
      </section>
    </aside>
  </div>
</Shell>

<style>
  .metrics {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    overflow: hidden;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .metrics article {
    min-height: 104px;
    padding: var(--space-4) var(--space-5);
    display: grid;
    align-content: center;
    gap: var(--space-1);
    border-right: 1px solid var(--color-rule);
  }
  .metrics article:last-child {
    border-right: 0;
  }
  .metric-label {
    color: var(--color-muted);
    font-size: var(--text-xs);
    font-weight: 600;
  }
  .metrics strong {
    font-size: 26px;
    font-weight: 700;
    letter-spacing: -0.03em;
    line-height: 1.1;
  }
  .metrics small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .metrics small a {
    color: var(--color-accent);
    text-decoration: none;
  }
  .metrics small a:hover {
    text-decoration: underline;
  }
  .metric-skeleton {
    width: 56px;
    height: 26px;
    margin-block: 3px;
  }
  .tone-success { color: var(--color-success) !important; }
  .tone-danger { color: var(--color-danger) !important; }
  .tone-info { color: var(--color-info) !important; }

  .failure-strip {
    margin-bottom: var(--space-4);
    padding: var(--space-3) var(--space-4);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid color-mix(in srgb, var(--color-danger) 32%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-danger) 5%, var(--color-paper-raised));
    color: var(--color-danger);
    font-size: var(--text-sm);
    text-decoration: none;
  }
  .failure-strip span {
    min-width: 0;
    flex: 1;
    overflow: hidden;
    color: var(--color-ink);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .failure-strip strong {
    color: var(--color-danger);
  }
  .failure-strip em {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: var(--text-xs);
    font-style: normal;
    font-weight: 600;
    white-space: nowrap;
  }

  .resources {
    margin-bottom: var(--space-4);
  }
  .resources-link {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-decoration: none;
  }
  .resources-link:hover {
    color: var(--color-accent);
  }
  .resources-link i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-success);
  }
  .resources-link i.refreshing {
    animation: dokyr-status-pulse 1s ease-in-out infinite;
  }
  .resources-link-more {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .resource-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }
  .resource-grid article {
    min-width: 0;
    min-height: 128px;
    padding: var(--space-4);
    display: grid;
    align-content: start;
    gap: var(--space-2);
    border-right: 1px solid var(--color-rule);
  }
  .resource-grid article:last-child {
    border-right: 0;
  }
  .resource-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--color-muted);
    font-size: var(--text-xs);
    font-weight: 600;
  }
  .resource-icon {
    width: 26px;
    height: 26px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
  }
  .resource-icon.tone-accent { background: var(--color-accent-soft); color: var(--color-accent); }
  .resource-icon.tone-info { background: var(--color-info-soft); color: var(--color-info); }
  .resource-icon.tone-warning { background: var(--color-warning-soft); color: var(--color-warning); }
  .resource-icon.tone-muted { background: var(--color-paper-subtle); color: var(--color-muted); }
  .resource-icon.tone-success { background: var(--color-success-soft); color: var(--color-success); }
  .resource-grid strong {
    overflow: hidden;
    font-size: var(--text-xl);
    font-weight: 700;
    letter-spacing: -0.02em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resource-grid small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
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
  .resource-grid dl {
    margin: 2px 0 0;
    display: grid;
    gap: 4px;
  }
  .resource-grid dl div {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .resource-grid dt,
  .resource-grid dd {
    margin: 0;
  }
  .resource-grid dd {
    color: var(--color-ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .dashboard-grid {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(300px, 1fr);
    gap: var(--space-4);
    align-items: start;
  }
  .panel-more {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--color-accent);
    font-size: var(--text-xs);
    font-weight: 600;
    text-decoration: none;
  }
  .panel-more:hover {
    text-decoration: underline;
  }
  .deployment-table .deployment-name {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .deployment-icon {
    width: 28px;
    height: 28px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-surface-subtle);
    color: var(--color-muted);
  }
  .deployment-name strong {
    overflow: hidden;
    max-width: 300px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .deployment-table code {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .deployment-table .muted,
  .deployment-table time {
    color: var(--color-muted);
    font-size: var(--text-sm);
    white-space: nowrap;
  }
  .clickable {
    cursor: pointer;
  }

  .rail {
    display: grid;
    gap: var(--space-4);
  }
  .health-rows {
    display: grid;
  }
  .health-row {
    min-height: 52px;
    padding: var(--space-2) var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
  }
  .health-row div {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .health-row strong {
    overflow: hidden;
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .health-row small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .health-row b {
    color: var(--color-ink-secondary);
    font: 500 var(--text-sm) var(--font-mono);
  }
  .panel-footer {
    font-size: var(--text-xs);
    color: var(--color-muted);
  }
  .panel-footer a {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--color-accent);
    font-weight: 600;
    text-decoration: none;
  }
  .rail-loading,
  .rail-error {
    min-height: 150px;
    padding: var(--space-5);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .rail-error {
    flex-direction: column;
    gap: var(--space-2);
    text-align: center;
  }
  .rail-error :global(svg) {
    color: var(--color-danger);
  }
  .rail-error p {
    margin: 0;
    font-size: var(--text-sm);
  }
  .rail-empty {
    min-height: 120px;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: var(--space-3);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .rail-empty p {
    margin: 0;
  }

  .project-rows {
    display: grid;
  }
  .project-row {
    min-height: 54px;
    padding: var(--space-2) var(--space-5);
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
    color: inherit;
    text-decoration: none;
  }
  .project-row:last-child {
    border-bottom: 0;
  }
  .project-row:hover {
    background: var(--color-surface-subtle);
  }
  .project-row-text {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .project-row-text strong {
    overflow: hidden;
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .project-row-text small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .project-row code {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .rows-loading {
    padding: var(--space-3) var(--space-5);
    display: grid;
    gap: var(--space-3);
  }
  .row-skeleton {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  @media (max-width: 70rem) {
    .resource-grid {
      grid-template-columns: repeat(3, 1fr);
    }
    .resource-grid article:nth-child(3) {
      border-right: 0;
    }
    .resource-grid article:nth-child(-n + 3) {
      border-bottom: 1px solid var(--color-rule);
    }
    .dashboard-grid {
      grid-template-columns: 1fr;
    }
  }
  @media (max-width: 46rem) {
    .metrics {
      grid-template-columns: repeat(2, 1fr);
    }
    .metrics article:nth-child(2) {
      border-right: 0;
    }
    .metrics article:nth-child(-n + 2) {
      border-bottom: 1px solid var(--color-rule);
    }
    .resource-grid {
      grid-template-columns: repeat(2, 1fr);
    }
    .resource-grid article,
    .resource-grid article:nth-child(3) {
      border-right: 1px solid var(--color-rule);
      border-bottom: 1px solid var(--color-rule);
    }
    .resource-grid article:nth-child(even) {
      border-right: 0;
    }
    .resource-grid article:last-child {
      border-bottom: 0;
    }
    .failure-strip em {
      display: none;
    }
    .deployment-table th:nth-child(2),
    .deployment-table td:nth-child(2),
    .deployment-table th:nth-child(3),
    .deployment-table td:nth-child(3) {
      display: none;
    }
  }
  @media (max-width: 30rem) {
    .metrics {
      grid-template-columns: 1fr;
    }
    .metrics article {
      min-height: 88px;
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .metrics article:last-child {
      border-bottom: 0;
    }
    .resource-grid {
      grid-template-columns: 1fr;
    }
    .resource-grid article,
    .resource-grid article:nth-child(3),
    .resource-grid article:nth-child(even) {
      min-height: 112px;
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .resource-grid article:last-child {
      border-bottom: 0;
    }
  }
</style>
