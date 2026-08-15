<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import Status from '$lib/components/Status.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  const engines = {
    postgres: { label: 'PostgreSQL', version: '17', port: 5432, mark: 'PG' },
    mysql: { label: 'MySQL', version: '8.4', port: 3306, mark: 'MY' },
    mariadb: { label: 'MariaDB', version: '11.8', port: 3306, mark: 'MA' }
  };
  const blankForm = () => ({ engine: 'postgres', name: 'PostgreSQL', databaseName: 'app', username: 'app', password: '', publicEnabled: false, publicPort: 5432 });

  let clusters = [];
  let loading = true;
  let loadError = '';
  let query = '';
  let engineFilter = 'all';
  let createOpen = false;
  let creating = false;
  let createError = '';
  let form = blankForm();
  let lifecycleBusy = '';
  let credentialsTarget = null;
  let credentials = null;
  let credentialsLoading = false;
  let copied = '';
  let logsTarget = null;
  let logsView = 'runtime';
  let logs = [];
  let logEvents = [];
  let logsLoading = false;
  let logsError = '';
  let logsUpdated = '';
  let logLimit = 300;
  let logsLive = true;
  let logFollowing = true;
  let unseenLogCount = 0;
  let logRequest = 0;
  let logPollTimer;
  let clusterPollTimer;
  let logConsole;
  let deleteTarget = null;
  let deleteConfirmation = '';
  let deleteVolume = false;
  let deleting = false;
  let deleteError = '';

  $: filteredClusters = clusters.filter((cluster) => {
    const matchesEngine = engineFilter === 'all' || cluster.engine === engineFilter;
    const haystack = `${cluster.name} ${cluster.engine} ${(cluster.projects || []).map((project) => project.name).join(' ')}`.toLowerCase();
    return matchesEngine && haystack.includes(query.trim().toLowerCase());
  });
  $: attachmentCount = clusters.reduce((total, cluster) => total + (cluster.projectCount || 0), 0);
  $: runningCount = clusters.filter((cluster) => ['healthy', 'deploying', 'degraded'].includes(cluster.status)).length;
  $: logEntries = logsView === 'deployment'
    ? logEvents.map((event, index) => ({
        index: index + 1,
        timestamp: event.createdAt || '',
        time: event.createdAt ? new Date(event.createdAt).toLocaleTimeString() : '—',
        message: `[${event.stage || 'deploy'}] ${event.message}`,
        severity: event.type === 'error' ? 'error' : event.type === 'complete' ? 'info' : 'debug'
      }))
    : logs.map(parseLogLine);

  onMount(async () => {
    await load();
    if (page.url.searchParams.get('create') === '1') openCreate();
  });

  onDestroy(() => {
    stopLogPolling();
    clearTimeout(clusterPollTimer);
  });

  async function load(silent = false) {
    if (!silent) {
      loading = true;
      loadError = '';
    }
    try {
      const response = await api('/api/databases');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load database clusters');
      clusters = payload.clusters || [];
      if (logsTarget) {
        const currentTarget = clusters.find((cluster) => cluster.id === logsTarget.id);
        if (currentTarget) logsTarget = currentTarget;
      }
    } catch (cause) {
      if (!silent) loadError = cause instanceof Error ? cause.message : 'Could not load database clusters';
    } finally {
      if (!silent) loading = false;
      scheduleClusterPolling();
    }
  }

  function scheduleClusterPolling() {
    clearTimeout(clusterPollTimer);
    if (clusters.some((cluster) => cluster.status === 'deploying' || cluster.status === 'created')) {
      clusterPollTimer = setTimeout(() => load(true), 2500);
    }
  }

  function openCreate() {
    form = blankForm();
    createError = '';
    createOpen = true;
  }

  function chooseEngine(engine) {
    const previous = engines[form.engine];
    const next = engines[engine];
    form = { ...form, engine, name: form.name === previous.label ? next.label : form.name, publicPort: next.port };
  }

  async function createCluster() {
    creating = true;
    createError = '';
    try {
      const response = await api('/api/databases', { method: 'POST', body: JSON.stringify(form) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not create database cluster');
      createOpen = false;
      clusters = [payload.cluster, ...clusters.filter((cluster) => cluster.id !== payload.cluster.id)];
      scheduleClusterPolling();
      toast.success(payload.message || `${payload.cluster.name} is provisioning in the background`);
    } catch (cause) {
      createError = cause instanceof Error ? cause.message : 'Could not create database cluster';
    } finally {
      creating = false;
    }
  }

  async function control(cluster, action) {
    lifecycleBusy = `${cluster.id}:${action}`;
    try {
      const response = await api(`/api/databases/${cluster.id}/${action}`, { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || `Could not ${action} cluster`);
      toast.success(payload.message || `${cluster.name} updated`);
      await load();
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : `Could not ${action} cluster`);
    } finally {
      lifecycleBusy = '';
    }
  }

  async function revealCredentials(cluster) {
    credentialsTarget = cluster;
    credentials = null;
    credentialsLoading = true;
    copied = '';
    try {
      const response = await api(`/api/databases/${cluster.id}/credentials`);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not reveal credentials');
      credentials = payload;
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not reveal credentials');
      credentialsTarget = null;
    } finally {
      credentialsLoading = false;
    }
  }

  async function copy(field, value) {
    await navigator.clipboard.writeText(value);
    copied = field;
    setTimeout(() => { if (copied === field) copied = ''; }, 1400);
  }

  function parseLogLine(line, index) {
    const timestampMatch = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(.*)$/);
    const timestamp = timestampMatch?.[1] || '';
    const message = timestampMatch?.[2] || line;
    const normalized = message.toLowerCase();
    let severity = 'info';
    if (/\b(error|fatal|panic|critical|crit|emerg|alert)\b/.test(normalized)) severity = 'error';
    else if (/\b(warn|warning)\b/.test(normalized)) severity = 'warning';
    else if (/\b(debug|trace|verbose)\b/.test(normalized)) severity = 'debug';
    return { index: index + 1, timestamp, time: timestamp ? timestamp.slice(11, 23) : '—', message, severity };
  }

  async function openLogs(cluster, initialView = 'runtime') {
    logsTarget = cluster;
    logsView = initialView === 'runtime' && !cluster.container ? 'deployment' : initialView;
    logs = [];
    logEvents = [];
    logsError = '';
    logsUpdated = '';
    logsLive = true;
    logFollowing = true;
    unseenLogCount = 0;
    await tick();
    await loadClusterLogs();
    startLogPolling();
  }

  async function loadClusterLogs(silent = false) {
    if (!logsTarget) return;
    const target = logsTarget;
    const view = logsView;
    const request = ++logRequest;
    if (!silent) logsLoading = true;
    const previousCount = view === 'deployment' ? logEvents.length : logs.length;
    if (!silent) logsError = '';
    try {
      const endpoint = view === 'deployment'
        ? `/api/databases/${target.id}/events`
        : `/api/databases/${target.id}/logs?lines=${logLimit}`;
      const response = await api(endpoint);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || `Could not load ${view === 'deployment' ? 'deployment logs' : 'runtime logs'}`);
      if (request !== logRequest || logsTarget?.id !== target.id || logsView !== view) return;
      if (view === 'deployment') {
        logEvents = payload.events || [];
        logs = [];
      } else {
        logs = payload.lines || [];
        logEvents = [];
      }
      logsUpdated = new Date().toLocaleTimeString();
      await tick();
      const currentCount = view === 'deployment' ? logEvents.length : logs.length;
      if (logsLive && logFollowing && logConsole) {
        logConsole.scrollTop = logConsole.scrollHeight;
        unseenLogCount = 0;
      } else if (logsLive && currentCount > previousCount) {
        unseenLogCount += currentCount - previousCount;
      }
    } catch (cause) {
      if (request !== logRequest || logsTarget?.id !== target.id || logsView !== view) return;
      if (!silent) {
        logsError = cause instanceof Error ? cause.message : 'Could not load logs';
        logs = [];
        logEvents = [];
      }
    } finally {
      if (!silent && request === logRequest) logsLoading = false;
    }
  }

  async function selectLogsView(view) {
    if (!logsTarget || (view === 'runtime' && !logsTarget.container)) return;
    stopLogPolling();
    logsView = view;
    logs = [];
    logEvents = [];
    logsError = '';
    logsUpdated = '';
    logFollowing = true;
    unseenLogCount = 0;
    await tick();
    await loadClusterLogs();
    startLogPolling();
  }

  function startLogPolling() {
    stopLogPolling();
    if (!logsTarget || !logsLive) return;
    logPollTimer = setInterval(() => loadClusterLogs(true), 1000);
  }

  function stopLogPolling() {
    if (logPollTimer) clearInterval(logPollTimer);
    logPollTimer = undefined;
  }

  function closeLogs() {
    stopLogPolling();
    logRequest += 1;
    logsTarget = null;
  }

  function toggleLiveLogs() {
    logsLive = !logsLive;
    if (logsLive) {
      logFollowing = true;
      unseenLogCount = 0;
      loadClusterLogs();
      startLogPolling();
    } else {
      stopLogPolling();
    }
  }

  function changeLogLimit() {
    loadClusterLogs();
    startLogPolling();
  }

  function handleLogScroll() {
    if (!logConsole || !logsLive) return;
    const distanceFromBottom = logConsole.scrollHeight - logConsole.scrollTop - logConsole.clientHeight;
    logFollowing = distanceFromBottom < 36;
    if (logFollowing) unseenLogCount = 0;
  }

  async function followLatestLogs() {
    logFollowing = true;
    unseenLogCount = 0;
    await tick();
    if (logConsole) logConsole.scrollTop = logConsole.scrollHeight;
  }

  function confirmDelete(cluster) {
    deleteTarget = cluster;
    deleteConfirmation = '';
    deleteVolume = false;
    deleteError = '';
  }

  async function deleteCluster() {
    deleting = true;
    deleteError = '';
    try {
      const response = await api(`/api/databases/${deleteTarget.id}`, {
        method: 'DELETE',
        body: JSON.stringify({ confirmation: deleteConfirmation, removeVolume: deleteVolume })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not delete database cluster');
      toast.success(deleteVolume ? `${deleteTarget.name} and its data were deleted` : `${deleteTarget.name} removed; volume retained`);
      deleteTarget = null;
      await load();
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not delete database cluster';
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:window onkeydown={(event) => {
  if (event.key === 'Escape' && createOpen && !creating) createOpen = false;
  if (event.key === 'Escape' && credentialsTarget) credentialsTarget = null;
  if (event.key === 'Escape' && logsTarget) closeLogs();
  if (event.key === 'Escape' && deleteTarget && !deleting) deleteTarget = null;
}} />

<Shell eyebrow="Infrastructure" title="Databases" subtitle="Shared database clusters with project-scoped private network access.">
  <button slot="actions" class="btn btn-primary" type="button" onclick={openCreate}><Icon name="plus" size={14} /> Create cluster</button>

  <section class="database-ledger" aria-label="Database cluster summary">
    <div class="ledger-title">
      <span class="ledger-icon"><Icon name="database" size={23} /></span>
      <div><span>Shared infrastructure</span><h2>One cluster. Many private networks.</h2><p>Clusters live independently from projects. Attach only the database and DNS name each project needs.</p></div>
    </div>
    <dl>
      <div><dt>Clusters</dt><dd>{clusters.length}</dd></div>
      <div><dt>Running</dt><dd>{runningCount}</dd></div>
      <div><dt>Attachments</dt><dd>{attachmentCount}</dd></div>
    </dl>
  </section>

  {#if loadError}<div class="feedback error"><Icon name="x-circle" size={15} /><div><strong>Database clusters unavailable</strong><span>{loadError}</span></div><button class="btn btn-sm" onclick={load}>Retry</button></div>{/if}

  <section class="cluster-panel panel">
    <header class="cluster-toolbar">
      <div><span>Cluster inventory</span><h3>Database clusters</h3></div>
      <div class="filters">
        <label><Icon name="search" size={14} /><input bind:value={query} placeholder="Search clusters" aria-label="Search clusters" /></label>
        <select bind:value={engineFilter} aria-label="Filter by engine"><option value="all">All engines</option>{#each Object.entries(engines) as [id, engine]}<option value={id}>{engine.label}</option>{/each}</select>
      </div>
    </header>

    {#if loading}
      <div class="loading-list">{#each [1, 2, 3] as row}<div aria-hidden="true"></div>{/each}</div>
    {:else if clusters.length === 0}
      <div class="empty-clusters"><span><Icon name="database" size={26} /></span><div><small>NO CLUSTERS</small><h2>Provision shared data infrastructure</h2><p>Create PostgreSQL, MySQL, or MariaDB once, then attach it to as many project networks as you need.</p></div><button class="btn btn-primary" onclick={openCreate}><Icon name="plus" size={14} /> Create first cluster</button></div>
    {:else if filteredClusters.length === 0}
      <div class="no-results"><Icon name="search" size={20} /><strong>No matching clusters</strong><button onclick={() => { query = ''; engineFilter = 'all'; }}>Clear filters</button></div>
    {:else}
      <div class="cluster-columns" aria-hidden="true"><span>Cluster</span><span>Databases</span><span>Project networks</span><span>Access</span><span></span></div>
      <div class="cluster-list">
        {#each filteredClusters as cluster}
          <article class="cluster-row">
            <div class="cluster-identity"><span class:postgres={cluster.engine === 'postgres'} class:mysql={cluster.engine === 'mysql'}>{engines[cluster.engine]?.mark || 'DB'}</span><div><a href={`/databases/${cluster.id}`}>{cluster.name}</a><small>{engines[cluster.engine]?.label || cluster.engine} {engines[cluster.engine]?.version} · {cluster.image}</small>{#if cluster.status === 'failed' && cluster.lastError}<small class="cluster-error" title={cluster.lastError}>{cluster.lastError}</small>{/if}</div><Status value={cluster.status} /></div>
            <div class="database-cells">{#each cluster.databases || [] as database}<span><Icon name="database" size={12} /><b>{database.name}</b><small>{database.username}</small></span>{/each}</div>
            <div class="project-cells">{#if cluster.projects?.length}{#each cluster.projects.slice(0, 3) as project}<a href={`/projects/${project.id}#databases`}><i></i>{project.name}</a>{/each}{#if cluster.projects.length > 3}<small>+{cluster.projects.length - 3} more</small>{/if}{:else}<span class="unattached">Not attached</span>{/if}</div>
            <div class="access-cell"><Icon name={cluster.publicEnabled ? 'globe' : 'lock'} size={14} /><span><strong>{cluster.publicEnabled ? `Public · ${cluster.publicPort}` : 'Private only'}</strong><small>{cluster.projectCount || 0} project network{cluster.projectCount === 1 ? '' : 's'}</small></span></div>
            <div class="row-actions">
              <a class="btn btn-sm" href={`/databases/${cluster.id}`}><Icon name="settings" size={13} /> Manage</a>
              <button class="btn btn-sm" onclick={() => openLogs(cluster)} title={cluster.container ? 'View runtime and deployment logs' : 'View deployment logs'}><Icon name="logs" size={13} /> Logs</button>
              <button class="btn btn-sm" onclick={() => revealCredentials(cluster)}><Icon name="key" size={13} /> Credentials</button>
              {#if cluster.status === 'stopped'}<button class="btn btn-sm" onclick={() => control(cluster, 'restart')} disabled={lifecycleBusy !== ''}><Icon name="play" size={12} /> {lifecycleBusy === `${cluster.id}:restart` ? 'Starting…' : 'Start'}</button>{:else if cluster.container}<button class="btn btn-sm" onclick={() => control(cluster, 'stop')} disabled={lifecycleBusy !== ''}><Icon name="stop" size={11} /> {lifecycleBusy === `${cluster.id}:stop` ? 'Stopping…' : 'Stop'}</button>{:else if cluster.status === 'deploying'}<button class="btn btn-sm" disabled><span class="button-spinner"></span> Provisioning</button>{:else}<button class="btn btn-sm" onclick={() => control(cluster, 'deploy')} disabled={lifecycleBusy !== ''}><Icon name="rocket" size={12} /> {lifecycleBusy === `${cluster.id}:deploy` ? 'Queuing…' : (cluster.status === 'failed' ? 'Retry' : 'Deploy')}</button>{/if}
              <button class="icon-action danger" aria-label={`Delete ${cluster.name}`} title={cluster.projectCount ? 'Detach from every project before deleting' : cluster.status === 'deploying' ? 'Wait for provisioning to finish' : 'Delete cluster'} disabled={cluster.projectCount > 0 || cluster.status === 'deploying'} onclick={() => confirmDelete(cluster)}><Icon name="trash" size={14} /></button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</Shell>

{#if createOpen}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !creating) createOpen = false; }}>
    <div class="modal create-modal" role="dialog" aria-modal="true" aria-labelledby="create-cluster-title">
      <header><div><span>Global infrastructure</span><h2 id="create-cluster-title">Create database cluster</h2></div><button aria-label="Close" onclick={() => createOpen = false} disabled={creating}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createCluster(); }}>
        <div class="engine-picker">{#each Object.entries(engines) as [id, engine]}<button type="button" class:active={form.engine === id} onclick={() => chooseEngine(id)}><span>{engine.mark}</span><strong>{engine.label}</strong><small>{engine.version} · {engine.port}</small></button>{/each}</div>
        {#if createError}<div class="modal-error"><Icon name="x-circle" size={14} /><div><strong>Cluster not created</strong><span>{createError}</span></div></div>{/if}
        <div class="form-grid">
          <label class="field"><span>Cluster name</span><input class="input" bind:value={form.name} required maxlength="50" placeholder="Production data" /></label>
          <label class="field"><span>Initial database</span><input class="input input-mono" bind:value={form.databaseName} required maxlength="63" placeholder="app" /></label>
          <label class="field"><span>Database user</span><input class="input input-mono" bind:value={form.username} required maxlength="63" placeholder="app" /></label>
          <label class="field"><span>Password <em>optional</em></span><input class="input input-mono" bind:value={form.password} type="password" minlength="12" autocomplete="new-password" placeholder="Securely generated if empty" /></label>
        </div>
        <label class="public-choice"><input class="checkbox" bind:checked={form.publicEnabled} type="checkbox" /><span><strong>Enable public access</strong><small>Private project networking is recommended. Public access binds a host port on this server.</small></span></label>
        {#if form.publicEnabled}<label class="field public-port"><span>Public host port</span><input class="input input-mono" bind:value={form.publicPort} type="number" min="1" max="65535" required /></label>{/if}
        <footer><span>The row appears immediately while provisioning continues in the background.</span><button class="btn" type="button" onclick={() => createOpen = false} disabled={creating}>Cancel</button><button class="btn btn-primary" type="submit" disabled={creating}>{creating ? 'Creating…' : 'Create cluster'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if logsTarget}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeLogs(); }}>
    <div class="modal logs-modal" role="dialog" aria-modal="true" aria-labelledby="cluster-logs-title">
      <header>
        <div><span>Cluster observability</span><h2 id="cluster-logs-title">{logsTarget.name} logs</h2></div>
        <button aria-label="Close logs" onclick={closeLogs}>×</button>
      </header>
      <div class="logs-toolbar">
        <div class="log-view-tabs" aria-label="Choose database log type">
          <button class:active={logsView === 'runtime'} onclick={() => selectLogsView('runtime')} disabled={!logsTarget.container} title={!logsTarget.container ? 'Runtime logs are available while the cluster container exists' : 'View container output'}><Icon name="activity" size={13} /> Runtime</button>
          <button class:active={logsView === 'deployment'} onclick={() => selectLogsView('deployment')}><Icon name="rocket" size={13} /> Deployment</button>
        </div>
        <div class="logs-actions">
          {#if logsUpdated}<small>Updated {logsUpdated}</small>{/if}
          {#if logsView === 'runtime'}
            <label class="line-limit"><span>Lines</span><select bind:value={logLimit} onchange={changeLogLimit} aria-label="Number of runtime log lines"><option value={100}>100</option><option value={300}>300</option><option value={500}>500</option><option value={1000}>1,000</option></select></label>
          {/if}
          <button class="live-toggle" class:live={logsLive} onclick={toggleLiveLogs} aria-pressed={logsLive}><i></i>{logsLive ? 'Live · Pause' : 'Paused · Resume'}</button>
          <button onclick={() => loadClusterLogs()} disabled={logsLoading}><Icon name="refresh" size={13} /> {logsLoading ? 'Refreshing…' : 'Refresh'}</button>
        </div>
      </div>
      {#if logsLoading && logEntries.length === 0}
        <div class="logs-state"><span class="spinner"></span><div><strong>Reading {logsView} logs</strong><p>{logsView === 'runtime' ? 'Loading the latest output from the database container.' : 'Loading the cluster deployment history.'}</p></div></div>
      {:else if logsError}
        <div class="logs-state error-state"><span>!</span><div><strong>Logs unavailable</strong><p>{logsError}</p></div><button class="btn btn-sm" onclick={() => loadClusterLogs()}>Try again</button></div>
      {:else if logEntries.length === 0}
        <div class="logs-state"><span>LOG</span><div><strong>{logsView === 'deployment' ? 'No deployment events yet' : 'No runtime output yet'}</strong><p>{logsView === 'deployment' ? 'Events will appear the next time this cluster is deployed or restarted.' : 'The database container has not written anything to stdout or stderr.'}</p></div></div>
      {:else}
        <div class="terminal-head"><span class:paused={!logsLive}></span><strong>{logsView === 'deployment' ? `${logsTarget.name} deployment activity` : (logsTarget.container || logsTarget.name)}</strong><small>{logEntries.length} {logsView === 'deployment' ? 'events' : 'lines'} · {logsLive ? (logFollowing ? 'following live output' : 'live updates, scroll held') : 'stream paused'}</small></div>
        <div class="log-console-wrap">
          <div class="log-console" aria-label={logsView === 'deployment' ? 'Live database deployment logs' : 'Live database runtime logs'} bind:this={logConsole} onscroll={handleLogScroll}>
            {#each logEntries as entry}
              <div class="log-line {entry.severity}"><span class="line-number">{entry.index}</span><time datetime={entry.timestamp} title={entry.timestamp}>{entry.time}</time><span class="severity">{entry.severity}</span><code>{entry.message}</code></div>
            {/each}
          </div>
          {#if logsLive && !logFollowing}<button class="follow-latest" onclick={followLatestLogs}><Icon name="chevron-down" size={13} /> {unseenLogCount ? `${unseenLogCount} new · ` : ''}Jump to latest</button>{/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

{#if credentialsTarget}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) credentialsTarget = null; }}>
    <div class="modal credentials-modal" role="dialog" aria-modal="true" aria-labelledby="credentials-title">
      <header><div><span>Cluster secret</span><h2 id="credentials-title">{credentialsTarget.name} credentials</h2></div><button aria-label="Close" onclick={() => credentialsTarget = null}>×</button></header>
      {#if credentialsLoading}<div class="modal-loading"><span></span>Decrypting credentials…</div>{:else if credentials}
        <div class="credential-note"><Icon name="network" size={16} /><p>The internal hostname is assigned per project attachment. Use the service name shown in that project instead of the container identifier below.</p></div>
        <div class="credential-rows">{#each [['Database', 'database', credentials.database], ['Username', 'username', credentials.username], ['Password', 'password', credentials.password], ['Container', 'host', credentials.host]] as row}<div><span>{row[0]}</span><code>{row[2]}</code><button onclick={() => copy(row[1], row[2])}>{copied === row[1] ? 'Copied' : 'Copy'}</button></div>{/each}</div>
      {/if}
      <footer><button class="btn btn-primary" onclick={() => credentialsTarget = null}>Done</button></footer>
    </div>
  </div>
{/if}

{#if deleteTarget}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !deleting) deleteTarget = null; }}>
    <div class="modal delete-modal" role="dialog" aria-modal="true" aria-labelledby="delete-title">
      <header><div><span>Permanent operation</span><h2 id="delete-title">Delete {deleteTarget.name}?</h2></div><button aria-label="Close" onclick={() => deleteTarget = null} disabled={deleting}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); deleteCluster(); }}>
        <div class="danger-note"><Icon name="alert" size={18} /><div><strong>This removes the database container</strong><p>A cluster must be detached from every project before it can be deleted.</p></div></div>
        <label class="volume-option"><input class="checkbox" bind:checked={deleteVolume} type="checkbox" /><span><strong>Also delete the persistent volume</strong><small>This permanently deletes every database and record in the cluster.</small></span></label>
        {#if deleteError}<div class="modal-error"><Icon name="x-circle" size={14} /><div><strong>Cluster not deleted</strong><span>{deleteError}</span></div></div>{/if}
        <label class="field"><span>Type <code>{deleteTarget.name}</code> to confirm</span><input class="input" bind:value={deleteConfirmation} autocomplete="off" /></label>
        <footer><button class="btn" type="button" onclick={() => deleteTarget = null} disabled={deleting}>Cancel</button><button class="btn btn-danger-solid" type="submit" disabled={deleting || deleteConfirmation !== deleteTarget.name}>{deleting ? 'Deleting…' : deleteVolume ? 'Delete cluster and data' : 'Delete cluster'}</button></footer>
      </form>
    </div>
  </div>
{/if}

<style>
  .database-ledger { margin-bottom: var(--space-5); padding: var(--space-6); display: grid; gap: var(--space-6); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: linear-gradient(115deg, var(--color-paper-raised) 0 58%, var(--color-accent-softer)); box-shadow: var(--shadow-panel); }
  .ledger-title { display: grid; grid-template-columns: 48px minmax(0, 1fr); gap: var(--space-4); align-items: start; }
  .ledger-icon { width: 48px; height: 48px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 34%, transparent); border-radius: var(--radius-md); background: var(--color-accent); color: var(--color-accent-ink); box-shadow: 0 8px 20px color-mix(in srgb, var(--color-accent) 22%, transparent); }
  .ledger-title span:not(.ledger-icon), .cluster-toolbar > div > span { color: var(--color-accent); font: 700 var(--text-2xs)/1 var(--font-mono); letter-spacing: .12em; text-transform: uppercase; }
  .ledger-title h2 { margin: var(--space-2) 0 var(--space-1); font: 600 var(--text-xl)/1.2 var(--font-display); letter-spacing: -.04em; }
  .ledger-title p { margin: 0; max-width: 650px; color: var(--color-muted); font-size: var(--text-sm); }
  .database-ledger dl { margin: 0; display: grid; grid-template-columns: repeat(3, minmax(90px, 1fr)); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-paper-raised) 88%, transparent); }
  .database-ledger dl div { padding: var(--space-3) var(--space-4); display: grid; gap: 2px; border-right: 1px solid var(--color-rule); }
  .database-ledger dl div:last-child { border: 0; }
  .database-ledger dt { color: var(--color-muted); font: 600 var(--text-2xs)/1 var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
  .database-ledger dd { margin: 0; font: 600 var(--text-xl)/1.2 var(--font-display); }
  .feedback { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-danger-soft); color: var(--color-danger); }
  .feedback div { display: grid; } .feedback span { font-size: var(--text-xs); }
  .cluster-toolbar { min-height: 68px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .cluster-toolbar h3 { margin: var(--space-1) 0 0; font: 600 var(--text-lg)/1.2 var(--font-display); }
  .filters { display: flex; gap: var(--space-2); }
  .filters label { min-width: 220px; height: 34px; padding: 0 var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); }
  .filters input { width: 100%; border: 0; outline: 0; background: transparent; color: var(--color-ink); font-size: var(--text-sm); }
  .filters select { height: 34px; padding: 0 30px 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); }
  .cluster-columns, .cluster-row { display: grid; grid-template-columns: minmax(230px, 1.3fr) minmax(130px, .7fr) minmax(170px, .9fr) minmax(125px, .65fr) minmax(320px, auto); gap: var(--space-4); align-items: center; }
  .cluster-columns { padding: var(--space-2) var(--space-5); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font: 600 var(--text-2xs)/1 var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
  .cluster-row { min-height: 92px; padding: var(--space-4) var(--space-5); border-bottom: 1px solid var(--color-rule); transition: background var(--duration-fast) var(--ease-out); }
  .cluster-row:last-child { border-bottom: 0; } .cluster-row:hover { background: var(--color-surface-subtle); }
  .cluster-identity { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); }
  .cluster-identity > span:first-child { width: 38px; height: 38px; display: grid; place-items: center; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink-secondary); font: 700 var(--text-xs)/1 var(--font-mono); }
  .cluster-identity > span.postgres { border-color: color-mix(in srgb, #336791 35%, var(--color-rule)); background: color-mix(in srgb, #336791 10%, var(--color-paper-raised)); color: #336791; }
  .cluster-identity > span.mysql { border-color: color-mix(in srgb, #e48e00 35%, var(--color-rule)); background: color-mix(in srgb, #e48e00 10%, var(--color-paper-raised)); color: #b56f00; }
  .cluster-identity div { min-width: 0; display: grid; } .cluster-identity a { overflow: hidden; color: var(--color-ink); font-size: var(--text-sm); font-weight: 700; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; } .cluster-identity a:hover { color: var(--color-accent); }
  .cluster-identity small { overflow: hidden; color: var(--color-muted); font: var(--text-xs)/1.5 var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .cluster-identity .cluster-error { color: var(--color-danger); }
  .database-cells, .project-cells { display: flex; flex-wrap: wrap; gap: var(--space-1); }
  .database-cells > span { padding: 4px 7px; display: inline-flex; align-items: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-paper-raised); font-size: var(--text-xs); }
  .database-cells small { color: var(--color-muted); font-family: var(--font-mono); }
  .project-cells a { padding: 4px 7px; display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: 999px; color: var(--color-ink-secondary); font-size: var(--text-xs); text-decoration: none; }
  .project-cells i { width: 5px; height: 5px; border-radius: 50%; background: var(--color-success); }
  .project-cells > small, .unattached { align-self: center; color: var(--color-muted); font-size: var(--text-xs); }
  .access-cell { display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); }
  .access-cell span { display: grid; } .access-cell strong { color: var(--color-ink-secondary); font-size: var(--text-xs); } .access-cell small { font-size: var(--text-xs); }
  .row-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--space-2); }
  .button-spinner { width: 11px; height: 11px; border: 1.5px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .icon-action { width: 30px; height: 30px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); cursor: pointer; }
  .icon-action.danger:not(:disabled):hover { border-color: var(--color-danger); color: var(--color-danger); } .icon-action:disabled { opacity: .35; cursor: not-allowed; }
  .loading-list { padding: var(--space-4); display: grid; gap: var(--space-2); } .loading-list div { height: 72px; border-radius: var(--radius-md); background: linear-gradient(90deg, var(--color-paper-subtle), var(--color-surface-subtle), var(--color-paper-subtle)); background-size: 200% 100%; animation: shimmer 1.4s infinite; }
  .empty-clusters { min-height: 300px; padding: var(--space-8); display: grid; grid-template-columns: 52px minmax(0, 520px) auto; justify-content: center; align-items: center; gap: var(--space-5); }
  .empty-clusters > span { width: 52px; height: 52px; display: grid; place-items: center; border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); color: var(--color-muted); }
  .empty-clusters small { color: var(--color-accent); font: 700 var(--text-2xs)/1 var(--font-mono); letter-spacing: .12em; } .empty-clusters h2 { margin: var(--space-2) 0 var(--space-1); font: 600 var(--text-lg)/1.2 var(--font-display); } .empty-clusters p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .no-results { min-height: 220px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); } .no-results button { border: 0; background: transparent; color: var(--color-accent); cursor: pointer; }
  .modal-backdrop { position: fixed; z-index: 500; inset: 0; padding: var(--space-4); display: grid; place-items: center; overflow-y: auto; background: rgb(4 10 18 / .58); backdrop-filter: blur(3px); }
  .modal { width: min(680px, 100%); overflow: hidden; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-modal); }
  .modal > header { min-height: 65px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .modal > header span { color: var(--color-accent); font: 700 var(--text-2xs)/1 var(--font-mono); letter-spacing: .1em; text-transform: uppercase; } .modal > header h2 { margin: var(--space-1) 0 0; font: 600 var(--text-lg)/1.2 var(--font-display); }
  .modal > header > button { width: 30px; height: 30px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }
  .create-modal form, .delete-modal form { padding: var(--space-5); }
  .engine-picker { margin-bottom: var(--space-5); display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-2); }
  .engine-picker button { min-height: 84px; padding: var(--space-3); display: grid; grid-template-columns: 30px minmax(0, 1fr); grid-template-rows: auto auto; align-items: center; column-gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-ink); text-align: left; cursor: pointer; }
  .engine-picker button.active { border-color: var(--color-accent); background: var(--color-accent-softer); box-shadow: inset 0 0 0 1px var(--color-accent); }
  .engine-picker button > span { grid-row: 1 / 3; width: 30px; height: 30px; display: grid; place-items: center; border-radius: var(--radius-xs); background: var(--color-paper-subtle); font: 700 var(--text-xs)/1 var(--font-mono); }
  .engine-picker strong { align-self: end; font-size: var(--text-sm); } .engine-picker small { align-self: start; color: var(--color-muted); font: var(--text-xs)/1.4 var(--font-mono); }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
  .public-choice, .volume-option { margin-top: var(--space-5); padding: var(--space-4); display: grid; grid-template-columns: 18px minmax(0, 1fr); align-items: start; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); cursor: pointer; }
  .public-choice span, .volume-option span { display: grid; } .public-choice strong, .volume-option strong { font-size: var(--text-sm); } .public-choice small, .volume-option small { color: var(--color-muted); font-size: var(--text-xs); }
  .public-port { width: 200px; margin-top: var(--space-4); }
  .modal footer { margin: var(--space-5) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .modal footer > span { margin-right: auto; max-width: 340px; color: var(--color-muted); font-size: var(--text-xs); }
  .modal-error { margin-bottom: var(--space-4); padding: var(--space-3); display: flex; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-danger-soft); color: var(--color-danger); } .modal-error div { display: grid; } .modal-error span { font-size: var(--text-xs); }
  .credentials-modal { width: min(610px, 100%); } .credentials-modal > footer { margin: 0; }
  .logs-modal { width: min(980px, 100%); }
  .logs-toolbar { min-height: 58px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .log-view-tabs { padding: 3px; display: flex; gap: 2px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .log-view-tabs button { min-height: 30px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: 7px; border: 0; border-radius: var(--radius-xs); background: transparent; color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .log-view-tabs button.active { background: var(--color-accent); color: var(--color-accent-ink); box-shadow: var(--shadow-xs); }
  .log-view-tabs button:disabled { opacity: .4; cursor: not-allowed; }
  .logs-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: var(--space-2); }
  .logs-actions small { color: var(--color-muted); font-size: var(--text-xs); white-space: nowrap; }
  .logs-actions > button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: 7px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink-secondary); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .logs-actions > button:disabled { opacity: .5; cursor: wait; }
  .line-limit { min-height: 32px; padding-left: var(--space-2); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .line-limit span { color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .line-limit select { height: 30px; padding: 0 24px 0 2px; border: 0; outline: 0; background: transparent; color: var(--color-ink); font: var(--text-xs) var(--font-mono); cursor: pointer; }
  .live-toggle i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-faint); }
  .live-toggle.live { border-color: color-mix(in srgb, var(--color-success) 45%, var(--color-rule)); color: var(--color-success); }
  .live-toggle.live i { background: var(--color-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-success) 14%, transparent); animation: live-pulse 1.8s ease-out infinite; }
  .terminal-head { min-height: 40px; padding: 0 var(--space-4); display: grid; grid-template-columns: 9px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .terminal-head > span { width: 8px; height: 8px; border-radius: 50%; background: var(--color-success); }
  .terminal-head > span.paused { background: var(--color-muted); }
  .terminal-head strong, .terminal-head small { overflow: hidden; color: var(--color-muted); font: 500 var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .log-console-wrap { position: relative; background: var(--color-log-bg); }
  .log-console { min-height: 360px; max-height: 65vh; overflow: auto; background: var(--color-log-bg); color: var(--color-log-text); font-family: var(--font-mono); }
  .follow-latest { position: absolute; right: var(--space-4); bottom: var(--space-4); min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: 7px; border: 1px solid color-mix(in srgb, var(--color-accent) 55%, var(--color-log-rule)); border-radius: 999px; background: var(--color-accent); color: var(--color-accent-ink); box-shadow: 0 8px 22px rgb(0 0 0 / .32); font-size: var(--text-xs); font-weight: 700; cursor: pointer; }
  .log-line { --severity-color: var(--color-info); min-height: 32px; padding: var(--space-1) var(--space-3) var(--space-1) 0; display: grid; grid-template-columns: 44px 94px 66px minmax(0, 1fr); align-items: start; border-bottom: 1px solid var(--color-log-rule); box-shadow: inset 2px 0 var(--severity-color); }
  .log-line.debug { --severity-color: var(--color-debug); } .log-line.info { --severity-color: var(--color-info); }
  .log-line.warning { --severity-color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 7%, var(--color-log-bg)); }
  .log-line.error { --severity-color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 9%, var(--color-log-bg)); }
  .line-number { padding-top: 3px; color: var(--color-log-muted); text-align: right; font-size: var(--text-2xs); user-select: none; }
  .log-line time { padding: 3px var(--space-3) 0; color: var(--color-log-muted); font-size: var(--text-2xs); white-space: nowrap; }
  .severity { width: fit-content; margin-top: 1px; padding: 2px var(--space-2); border: 1px solid color-mix(in srgb, var(--severity-color) 45%, transparent); border-radius: var(--radius-xs); background: color-mix(in srgb, var(--severity-color) 12%, transparent); color: var(--severity-color); font-size: var(--text-2xs); font-weight: 500; line-height: 1.5; text-transform: uppercase; }
  .log-line code { padding-top: 1px; overflow-wrap: anywhere; color: var(--color-log-text); font: var(--text-xs)/1.7 var(--font-mono); }
  .logs-state { min-height: 400px; padding: var(--space-8); display: flex; align-items: center; justify-content: center; gap: var(--space-4); }
  .logs-state > span:not(.spinner) { width: 42px; height: 42px; display: grid; place-items: center; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); color: var(--color-muted); font: 700 var(--text-xs) var(--font-mono); }
  .logs-state div { max-width: 480px; } .logs-state p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-sm); }
  .logs-state.error-state > span { border-color: color-mix(in srgb, var(--color-danger) 40%, var(--color-rule)); background: var(--color-danger-soft); color: var(--color-danger); }
  .modal-loading { min-height: 220px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); } .modal-loading > span { width: 16px; height: 16px; border: 2px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .credential-note { margin: var(--space-5); padding: var(--space-3); display: flex; gap: var(--space-3); border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); } .credential-note p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); }
  .credential-rows { padding: 0 var(--space-5) var(--space-5); } .credential-rows > div { min-height: 52px; display: grid; grid-template-columns: 90px minmax(0, 1fr) 50px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); } .credential-rows span { color: var(--color-muted); font-size: var(--text-xs); } .credential-rows code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } .credential-rows button { border: 0; background: transparent; color: var(--color-accent); font-size: var(--text-xs); cursor: pointer; }
  .danger-note { padding: var(--space-4); display: flex; gap: var(--space-3); border-left: 3px solid var(--color-danger); background: var(--color-danger-soft); color: var(--color-danger); } .danger-note p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-xs); } .delete-modal .field { margin-top: var(--space-5); } .delete-modal code { color: var(--color-danger); }
  @keyframes shimmer { to { background-position: -200% 0; } } @keyframes spin { to { transform: rotate(360deg); } }
  @media (min-width: 54rem) { .database-ledger { grid-template-columns: minmax(0, 1.5fr) minmax(330px, .7fr); align-items: center; } }
  @media (max-width: 72rem) { .cluster-columns { display: none; } .cluster-row { grid-template-columns: minmax(240px, 1fr) minmax(150px, .7fr) auto; } .database-cells { display: none; } .access-cell { display: none; } }
  @media (max-width: 48rem) { .cluster-toolbar { align-items: stretch; flex-direction: column; } .filters { flex-direction: column; } .filters label { min-width: 0; } .cluster-row { grid-template-columns: 1fr; } .project-cells { grid-row: 2; } .row-actions { justify-content: flex-start; overflow-x: auto; } .empty-clusters { grid-template-columns: 52px minmax(0, 1fr); } .empty-clusters button { grid-column: 1 / -1; } .form-grid, .engine-picker { grid-template-columns: 1fr; } .modal footer { align-items: stretch; flex-direction: column; } .modal footer > span { margin: 0; } .logs-toolbar { align-items: stretch; flex-direction: column; } .logs-actions { justify-content: flex-start; } .log-line { grid-template-columns: 34px 62px minmax(0, 1fr); } .log-line time { display: none; } }
  @media (max-width: 34rem) { .database-ledger dl { grid-template-columns: 1fr; } .database-ledger dl div { border-right: 0; border-bottom: 1px solid var(--color-rule); } .cluster-identity { grid-template-columns: 38px minmax(0, 1fr); } .cluster-identity :global(.status) { grid-column: 2; } }
  @media (prefers-reduced-motion: reduce) { .loading-list div, .modal-loading > span, .live-toggle.live i, .button-spinner { animation: none; } }
</style>
