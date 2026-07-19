<script>
  import { onDestroy, onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
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
  eyebrow="Northstar Labs"
  title="Main Dashboard"
  subtitle="Production health, deployment velocity, and infrastructure signals for this control plane."
  meta={['production', 'main', metrics.engineName || 'local', 'live']}
>
  <section class="metrics" aria-label="Workspace metrics">
    <article><small>Projects</small><b>{loading ? '—' : data.projects.length}</b><em class="positive">↗ live workspace</em></article>
    <article><small>Running services</small><b>{metricsLoading ? '—' : metrics.global.running}</b><em class:positive={engineOnline}>{engineOnline ? 'All healthy' : 'Monitoring unavailable'}</em></article>
    <article><small>Active deployments</small><b>{loading ? '—' : data.deployments.filter((item) => ['queued', 'building', 'deploying', 'running'].includes(item.status)).length}</b><em class="info">{data.deployments.some((item) => ['queued', 'building', 'deploying', 'running'].includes(item.status)) ? '1 building' : 'Pipeline idle'}</em></article>
    <article><small>Node pressure</small><b>{metricsLoading ? '—' : formatPercent(nodePressure)}</b><em>{metricsLoading ? 'Sampling host' : `${metrics.global.running} containers running`}</em></article>
  </section>

  {#if data.deployments.some((item) => item.status === 'failed')}
    <a class="alert" href={'/deployments/' + data.deployments.find((item) => item.status === 'failed')?.id}>
      <span aria-hidden="true">△</span>
      <strong>{data.deployments.find((item) => item.status === 'failed')?.message || 'A recent deployment failed.'}</strong>
      <b>View deployment</b>
    </a>
  {/if}

  <section class="monitoring" aria-label="Live host monitoring">
    <div class="monitoring-head">
      <div><small>Live monitoring</small><h2>Host resources</h2></div>
      <a href="/servers"><span class:refreshing={metricsRefreshing}>●</span>{metricsRefreshing ? 'Refreshing' : `Updated ${ago(metrics.checkedAt)}`} · View infrastructure →</a>
    </div>
    <div class="monitor-cards">
      <article>
        <div class="monitor-title"><span class="monitor-icon cpu">CPU</span><small>Processor</small></div>
        <strong>{metricsLoading ? '—' : formatPercent(cpuPercent)}</strong>
        <div class="progress"><i style={'width:' + cpuPercent + '%'}></i></div>
        <p>{metrics.global.cpuCores || '—'} cores · host load</p>
      </article>
      <article>
        <div class="monitor-title"><span class="monitor-icon memory">RAM</span><small>Memory</small></div>
        <strong>{metricsLoading ? '—' : formatPercent(memoryPercent)}</strong>
        <div class="progress"><i style={'width:' + memoryPercent + '%'}></i></div>
        <p>{formatBytes(metrics.global.memoryUsage)} / {formatBytes(metrics.global.memoryLimit)}</p>
      </article>
      <article>
        <div class="monitor-title"><span class="monitor-icon disk">DSK</span><small>Disk space</small></div>
        <strong>{metricsLoading ? '—' : formatPercent(diskPercent)}</strong>
        <div class="progress"><i style={'width:' + diskPercent + '%'}></i></div>
        <p>{formatBytes(diskUsed)} used · {formatBytes(diskAvailable)} free</p>
      </article>
      <article>
        <div class="monitor-title"><span class="monitor-icon io">I/O</span><small>Disk I/O</small></div>
        <strong>{metricsLoading ? '—' : formatBytes((metrics.global.diskIo?.read || 0) + (metrics.global.diskIo?.write || 0))}</strong>
        <dl><div><dt>Read</dt><dd>{formatBytes(metrics.global.diskIo?.read || 0)}</dd></div><div><dt>Write</dt><dd>{formatBytes(metrics.global.diskIo?.write || 0)}</dd></div></dl>
      </article>
      <article>
        <div class="monitor-title"><span class="monitor-icon network">NET</span><small>Network I/O</small></div>
        <strong>{metricsLoading ? '—' : formatBytes((metrics.global.networkIo?.receive || 0) + (metrics.global.networkIo?.transmit || 0))}</strong>
        <dl><div><dt>Down</dt><dd>{formatBytes(metrics.global.networkIo?.receive || 0)}</dd></div><div><dt>Up</dt><dd>{formatBytes(metrics.global.networkIo?.transmit || 0)}</dd></div></dl>
      </article>
    </div>
  </section>

  <div class="dashboard-grid">
    <section class="panel deployments" id="deployments">
      <div class="panelhead"><h2>Recent deployments</h2><a href="/deployments">View all</a></div>
      <div class="table-head"><span>Project</span><span>Commit</span><span>Duration</span><span>Time</span><span>Status</span></div>
      {#if loading}
        <p class="empty">Contacting control plane…</p>
      {:else if data.deployments.length === 0}
        <p class="empty">No deployments yet. Create a project to start the pipeline.</p>
      {:else}
        {#each data.deployments.slice(0, 8) as deployment}
          <a href={'/deployments/' + deployment.id} class="deployment">
            <span class="deployment-project"><i><Icon name="box" size={12} /></i><strong>{deployment.message || deployment.projectId}</strong></span>
            <code>{deployment.commit || 'image'}</code>
            <span>{deployment.duration ? `${deployment.duration}s` : '—'}</span>
            <time>{ago(deployment.createdAt)}</time>
            <Status value={deployment.status} />
          </a>
        {/each}
      {/if}
    </section>

    <aside class="rail">
      <section class="panel health" id="servers">
        <div class="panelhead"><h2>Infrastructure health</h2><span class:offline={!engineOnline}>{engineOnline ? '● 1/1 online' : '● offline'}</span></div>
        {#if metricsLoading}
          <div class="server-loading"><i></i><span>Sampling Docker host…</span></div>
        {:else if metricsError && !metrics.checkedAt}
          <div class="server-error"><span>{metricsError}</span><button onclick={() => loadMetrics()}>Retry</button></div>
        {:else}
          <div class="health-row"><div><strong>{metrics.engineName || 'local-docker'}</strong><small>CPU</small></div><b>{formatPercent(cpuPercent)}</b><span>● Online</span></div>
          <div class="health-row"><div><strong>Host memory</strong><small>{formatBytes(metrics.global.memoryUsage)} used</small></div><b>{formatPercent(memoryPercent)}</b><span>● Healthy</span></div>
          <div class="health-row"><div><strong>Docker storage</strong><small>{formatBytes(diskAvailable)} available</small></div><b>{formatPercent(diskPercent)}</b><span>● Healthy</span></div>
        {/if}
      </section>

      <section class="panel project-list" id="projects">
        <div class="panelhead"><h2>Projects</h2><a href="/projects">View all</a></div>
        {#if loading}
          <p class="empty">Loading projects…</p>
        {:else if data.projects.length === 0}
          <p class="empty">No projects yet.</p>
        {:else}
          {#each data.projects.slice(0, 5) as project}
            <a class="project" href={'/projects/' + project.id}>
              <div><strong>{project.name}</strong><span>{project.status === 'healthy' ? '1 service' : project.status}</span></div>
              <small>{project.branch || 'main'} · {project.repository || 'container image'}</small>
              <code>latest {project.updatedAt ? ago(project.updatedAt) : 'not deployed'}</code>
            </a>
          {/each}
        {/if}
      </section>
    </aside>
  </div>
</Shell>

<style>
  :global(.panel) { background: var(--surface); border: 1px solid var(--line); border-radius: 10px; box-shadow: var(--shadow-panel); }
  .metrics { display: grid; grid-template-columns: repeat(4, 1fr); border: 1px solid var(--line); border-radius: 10px; background: var(--surface); margin-bottom: 12px; box-shadow: var(--shadow-panel); }
  .metrics article { min-height: 88px; padding: 16px; border-right: 1px solid var(--line); display: grid; align-content: center; gap: 5px; }
  .metrics article:last-child { border: 0; }
  .metrics small { font: 9px var(--font-mono); color: var(--muted); }
  .metrics b { font: 600 25px var(--font-sans); letter-spacing: -.04em; }
  .metrics em { font-style: normal; font-size: 9px; color: var(--muted); }
  .metrics .positive { color: var(--green); }
  .metrics .info { color: var(--color-info); }
  .alert { min-height: 42px; margin-bottom: 12px; padding: 0 13px; display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 9px; border: 1px solid color-mix(in srgb, var(--red) 30%, var(--line)); border-radius: 8px; background: color-mix(in srgb, var(--red) 4%, var(--surface)); color: inherit; text-decoration: none; }
  .alert > span { color: var(--red); }
  .alert strong { overflow: hidden; font-size: 10px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
  .alert b { padding: 6px 9px; border: 1px solid var(--line); border-radius: 6px; background: var(--surface); color: var(--red); font-size: 9px; }
  .monitoring { margin-bottom: 12px; overflow: hidden; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); box-shadow: var(--shadow-panel); }
  .monitoring-head { min-height: 48px; padding: 0 14px; display: flex; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--line); }
  .monitoring-head > div { display: grid; gap: 2px; }
  .monitoring-head small { color: var(--muted); font: 8px var(--font-mono); text-transform: uppercase; letter-spacing: .08em; }
  .monitoring-head h2 { margin: 0; font-size: 12px; letter-spacing: -.02em; }
  .monitoring-head a { color: var(--muted); text-decoration: none; font: 8px var(--font-mono); }
  .monitoring-head a:hover { color: var(--green); }
  .monitoring-head a span { margin-right: 5px; color: var(--green); }
  .monitoring-head a span.refreshing { animation: pulse 1s ease-in-out infinite; }
  .monitor-cards { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); }
  .monitor-cards article { min-width: 0; min-height: 116px; padding: 13px; display: grid; align-content: start; gap: 7px; border-right: 1px solid var(--line); }
  .monitor-cards article:last-child { border-right: 0; }
  .monitor-title { display: flex; align-items: center; gap: 7px; }
  .monitor-title small { overflow: hidden; color: var(--muted); font: 8px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .monitor-icon { width: 28px; height: 22px; flex: 0 0 auto; display: grid; place-items: center; border-radius: 5px; background: var(--accent-soft); color: var(--green); font: 600 7px var(--font-mono); letter-spacing: .03em; }
  .monitor-icon.memory { background: var(--color-info-soft); color: var(--color-info); }
  .monitor-icon.disk { background: var(--color-warning-soft); color: var(--amber); }
  .monitor-icon.io { background: color-mix(in srgb, var(--color-debug) 14%, transparent); color: var(--color-debug); }
  .monitor-icon.network { background: color-mix(in srgb, var(--green) 12%, transparent); color: var(--green); }
  .monitor-cards strong { overflow: hidden; font-size: 17px; line-height: 1.1; letter-spacing: -.035em; text-overflow: ellipsis; white-space: nowrap; }
  .monitor-cards p { margin: 0; overflow: hidden; color: var(--muted); font: 7px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .progress { height: 3px; overflow: hidden; border-radius: 3px; background: var(--line); }
  .progress i { display: block; height: 100%; border-radius: inherit; background: var(--green); transition: width .3s ease; }
  .monitor-cards dl { margin: 2px 0 0; display: grid; gap: 4px; }
  .monitor-cards dl div { display: flex; justify-content: space-between; gap: 8px; color: var(--muted); font: 7px var(--font-mono); }
  .monitor-cards dt, .monitor-cards dd { margin: 0; }
  .monitor-cards dd { overflow: hidden; color: var(--ink); text-overflow: ellipsis; white-space: nowrap; }
  .dashboard-grid { display: grid; grid-template-columns: minmax(0, 2.15fr) minmax(260px, .95fr); gap: 12px; align-items: stretch; }
  .panel { overflow: hidden; }
  .panelhead { height: 48px; padding: 0 14px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--line); }
  .panelhead h2 { margin: 0; font-size: 12px; letter-spacing: -.02em; }
  .panelhead a { color: var(--green); text-decoration: none; font-size: 9px; font-weight: 600; }
  .table-head, .deployment { display: grid; grid-template-columns: minmax(150px,1.5fr) minmax(70px,.8fr) 64px 64px 82px; gap: 9px; align-items: center; }
  .table-head { height: 32px; padding: 0 13px; border-bottom: 1px solid var(--line); color: var(--muted); font: 8px var(--font-mono); }
  .deployment { min-height: 48px; padding: 0 13px; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
  .deployment:last-child { border: 0; }
  .deployment:hover, .project:hover { background: var(--surface2); }
  .deployment-project { min-width: 0; display: flex; align-items: center; gap: 8px; }
  .deployment-project i { width: 25px; height: 25px; flex: 0 0 auto; display: grid; place-items: center; border: 1px solid var(--line); border-radius: 6px; color: var(--muted); font-style: normal; }
  .deployment-project strong { overflow: hidden; font-size: 9px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
  .deployment code, .deployment > span, .deployment time { overflow: hidden; color: var(--muted); font: 8px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .rail { display: grid; grid-template-rows: auto 1fr; gap: 12px; }
  .panelhead > span { color: var(--green); font: 8px var(--font-mono); }
  .panelhead > span.offline { color: var(--red); }
  .health-row { min-height: 48px; padding: 8px 12px; display: grid; grid-template-columns: minmax(0,1fr) 44px 61px; gap: 7px; align-items: center; border-bottom: 1px solid var(--line); }
  .health-row:last-child { border-bottom: 0; }
  .health-row div { min-width: 0; display: grid; gap: 2px; }
  .health-row strong { overflow: hidden; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
  .health-row small, .health-row b { color: var(--muted); font: 8px var(--font-mono); }
  .health-row b { font-weight: 500; }
  .health-row > span { color: var(--green); font: 7px var(--font-mono); }
  .project { padding: 10px 12px; display: grid; gap: 4px; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
  .project:last-child { border: 0; }
  .project div { display: flex; justify-content: space-between; gap: 10px; }
  .project strong { overflow: hidden; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
  .project div span { color: var(--muted); font: 8px var(--font-mono); }
  .project small { overflow: hidden; color: var(--muted); font: 8px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .project code { color: var(--green); font-size: 8px; }
  .server-loading, .server-error { min-height: 144px; padding: 20px; display: grid; place-content: center; justify-items: center; gap: 8px; text-align: center; }
  .server-loading i { width: 22px; height: 22px; border: 2px solid var(--line); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .server-loading span, .server-error span { color: var(--muted); font: 10px var(--font-mono); }
  .server-error button { border: 0; background: transparent; color: var(--accent); font-size: 11px; font-weight: 700; cursor: pointer; }
  .empty { padding: 28px 14px; color: var(--muted); font: 9px var(--font-mono); }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes pulse { 50% { opacity: .35; } }
  @media(max-width: 1120px) { .monitor-cards { grid-template-columns: repeat(3, 1fr); } .monitor-cards article:nth-child(3) { border-right: 0; } .monitor-cards article:nth-child(-n+3) { border-bottom: 1px solid var(--line); } .dashboard-grid { grid-template-columns: 1fr; } .rail { grid-template-columns: repeat(2,minmax(0,1fr)); grid-template-rows: none; } }
  @media(max-width: 760px) { .metrics { grid-template-columns: repeat(2,1fr); } .metrics article:nth-child(2) { border-right: 0; } .metrics article:nth-child(-n+2) { border-bottom: 1px solid var(--line); } .monitor-cards { grid-template-columns: repeat(2, 1fr); } .monitor-cards article, .monitor-cards article:nth-child(3) { border-right: 1px solid var(--line); border-bottom: 1px solid var(--line); } .monitor-cards article:nth-child(even) { border-right: 0; } .monitor-cards article:last-child { border-bottom: 0; } .monitoring-head a { font-size: 0; } .monitoring-head a::after { content: 'View infrastructure →'; font-size: 8px; } .rail { grid-template-columns: 1fr; } .table-head { display:none; } .deployment { grid-template-columns: minmax(130px,1fr) 65px 82px; } .deployment code, .deployment time { display:none; } }
  @media(max-width: 480px) { .metrics { grid-template-columns: 1fr; } .metrics article { min-height: 75px; border-right: 0; border-bottom: 1px solid var(--line); } .monitor-cards { grid-template-columns: 1fr; } .monitor-cards article, .monitor-cards article:nth-child(3), .monitor-cards article:nth-child(even) { min-height: 98px; border-right: 0; border-bottom: 1px solid var(--line); } .monitor-cards article:last-child { border-bottom: 0; } .alert { grid-template-columns: auto 1fr; } .alert b { display:none; } .deployment { grid-template-columns: minmax(120px,1fr) 78px; } .deployment > span { display:none; } }
</style>
