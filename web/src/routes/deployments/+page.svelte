<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  let items = [];
  let loading = true;
  let error = '';
  let query = '';
  let status = 'all';

  $: activeCount = items.filter((item) => ['building', 'deploying'].includes(item.status)).length;
  $: failedCount = items.filter((item) => item.status === 'failed').length;
  $: filteredItems = items.filter((item) => {
    const matchesStatus = status === 'all' || item.status === status;
    const haystack = `${item.message || ''} ${item.projectId || ''} ${item.commit || ''}`.toLowerCase();
    return matchesStatus && haystack.includes(query.trim().toLowerCase());
  });

  onMount(loadDeployments);

  async function loadDeployments() {
    loading = true;
    error = '';
    try {
      const response = await api('/api/deployments');
      if (!response.ok) throw new Error('Could not load deployments');
      items = await response.json();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load deployments';
    } finally {
      loading = false;
    }
  }

  const formatDuration = (seconds) => Number(seconds) > 0 ? `${seconds}s` : '—';
  const formatDate = (value) => value ? new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

<Shell eyebrow="Delivery activity" title="Deployments">
  <section class="operational-summary" aria-label="Deployment summary">
    <div class="primary-metric">
      <div class="metric-icon"><Icon name="rocket" size={22} /></div>
      <div>
        <span>Total releases</span>
        <strong>{items.length}</strong>
        <p>{items.length === 0 ? 'Your deployment history will appear here.' : 'Deployments recorded across this workspace.'}</p>
      </div>
    </div>
    <div class="metric">
      <span class="signal active" aria-hidden="true"></span>
      <div><span>Running now</span><strong>{activeCount}</strong></div>
    </div>
    <div class="metric">
      <span class="signal failed" aria-hidden="true"></span>
      <div><span>Needs attention</span><strong>{failedCount}</strong></div>
    </div>
  </section>

  <section class="deployment-list" aria-busy={loading}>
    <div class="list-toolbar">
      <div>
        <h2>Deployment history</h2>
        <p>Builds and releases from every project.</p>
      </div>
      <div class="controls">
        <label class="search">
          <span class="sr-only">Search deployments</span>
          <input bind:value={query} type="search" placeholder="Search commit or project" />
        </label>
        <label>
          <span class="sr-only">Filter by status</span>
          <select bind:value={status} aria-label="Filter by status">
            <option value="all">All statuses</option>
            <option value="building">Building</option>
            <option value="deploying">Deploying</option>
            <option value="healthy">Healthy</option>
            <option value="failed">Failed</option>
          </select>
        </label>
        <button class="refresh" onclick={loadDeployments} disabled={loading}>
          <span aria-hidden="true">↻</span>{loading ? 'Refreshing' : 'Refresh'}
        </button>
      </div>
    </div>

    {#if loading && items.length === 0}
      <div class="state loading-state">
        <span class="spinner" aria-hidden="true"></span>
        <div><h3>Loading deployments</h3><p>Reading activity from the control plane.</p></div>
      </div>
    {:else if error}
      <div class="state error-state">
        <div class="state-icon">!</div>
        <div><h3>Deployment history is unavailable</h3><p>{error}. Check the service and try again.</p></div>
        <button onclick={loadDeployments}>Try again</button>
      </div>
    {:else if items.length === 0}
      <div class="state empty-state">
        <div class="state-icon"><Icon name="rocket" size={24} /></div>
        <div>
          <h3>No deployments yet</h3>
          <p>Connect a repository or registry, create a project, and ship its first revision.</p>
        </div>
        <a href="/projects?new=1">Create a project <span aria-hidden="true">→</span></a>
      </div>
    {:else if filteredItems.length === 0}
      <div class="state empty-state">
        <div class="state-icon"><Icon name="activity" size={24} /></div>
        <div><h3>No matching deployments</h3><p>Change the search or status filter to see more results.</p></div>
        <button onclick={() => { query = ''; status = 'all'; }}>Clear filters</button>
      </div>
    {:else}
      <div class="table" role="table" aria-label="Deployments">
        <div class="table-head" role="row">
          <span role="columnheader">Status</span><span role="columnheader">Revision</span><span role="columnheader">Duration</span><span role="columnheader">Started</span><span></span>
        </div>
        {#each filteredItems as deployment}
          <a class="deployment-row" href={'/deployments/' + deployment.id} role="row">
            <span role="cell"><Status value={deployment.status} /></span>
            <span class="revision" role="cell"><strong>{deployment.message || 'Deployment'}</strong><small>{deployment.projectId || 'Project'} · {deployment.commit || 'No commit'}</small></span>
            <span role="cell"><code>{formatDuration(deployment.duration)}</code></span>
            <span role="cell"><time datetime={deployment.createdAt}>{formatDate(deployment.createdAt)}</time></span>
            <span class="row-arrow" aria-hidden="true">→</span>
          </a>
        {/each}
      </div>
    {/if}
  </section>
</Shell>

<style>
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
  .operational-summary { margin-bottom: var(--space-6); display: grid; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); overflow: hidden; background: var(--color-paper-raised); box-shadow: var(--shadow-whisper); }
  .primary-metric { min-height: 154px; padding: var(--space-6); display: flex; align-items: flex-start; gap: var(--space-4); }
  .metric-icon { width: 44px; height: 44px; flex: 0 0 auto; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-accent-soft); color: var(--color-accent); }
  .primary-metric > div:last-child { display: grid; gap: var(--space-1); }
  .primary-metric span, .metric span:not(.signal) { color: var(--color-muted); font-size: 13px; font-weight: 600; }
  .primary-metric strong { font-family: var(--font-display); font-size: 36px; line-height: 1.05; letter-spacing: -0.05em; }
  .primary-metric p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: 14px; line-height: 1.5; }
  .metric { min-height: 92px; padding: var(--space-5) var(--space-6); display: flex; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); }
  .metric div { display: flex; flex: 1; align-items: center; justify-content: space-between; gap: var(--space-4); }
  .metric strong { font: 600 24px var(--font-mono); letter-spacing: -0.04em; }
  .signal { width: 9px; height: 9px; flex: 0 0 auto; border-radius: 50%; background: var(--color-muted); }
  .signal.active { background: var(--color-warning); }
  .signal.failed { background: var(--color-danger); }

  .deployment-list { border: 1px solid var(--color-rule); border-radius: var(--radius-lg); overflow: hidden; background: var(--color-paper-raised); box-shadow: var(--shadow-whisper); }
  .list-toolbar { padding: var(--space-5); display: flex; flex-direction: column; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .list-toolbar h2 { margin: 0; font-family: var(--font-display); font-size: 19px; letter-spacing: -0.025em; }
  .list-toolbar p { margin: var(--space-1) 0 0; color: var(--color-muted); font-size: 14px; }
  .controls { display: grid; gap: var(--space-2); }
  .controls input, .controls select, .refresh { width: 100%; height: 44px; outline: 2px solid transparent; outline-offset: 1px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-ink); font-size: 13px; }
  .controls input { padding: 0 var(--space-3); }
  .controls select { padding: 0 34px 0 var(--space-3); cursor: pointer; }
  .controls input::placeholder { color: var(--color-faint); }
  .controls input:hover, .controls select:hover, .refresh:hover:not(:disabled) { border-color: var(--color-rule-strong); }
  .controls input:focus-visible, .controls select:focus-visible, .refresh:focus-visible { outline-color: var(--color-focus); }
  .refresh { padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); cursor: pointer; font-weight: 600; }
  .refresh:active:not(:disabled) { background: var(--color-accent-soft); }
  .refresh:disabled { color: var(--color-faint); background: var(--color-paper-subtle); opacity: .55; cursor: not-allowed; }

  .state { min-height: 280px; padding: var(--space-8) var(--space-6); display: grid; align-content: center; justify-items: start; gap: var(--space-4); }
  .state-icon { width: 48px; height: 48px; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-muted); font: 600 18px var(--font-mono); }
  .state h3 { margin: 0 0 var(--space-2); font-family: var(--font-display); font-size: 18px; letter-spacing: -0.02em; }
  .state p { max-width: 560px; margin: 0; color: var(--color-muted); font-size: 14px; line-height: 1.55; }
  .state a, .state button { min-height: 44px; padding: 0 var(--space-4); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); text-decoration: none; white-space: nowrap; font-size: 14px; font-weight: 600; cursor: pointer; }
  .state a:hover, .state button:hover { background: var(--color-accent-hover); border-color: var(--color-accent-hover); }
  .state a:active, .state button:active { background: var(--color-accent); }
  .error-state .state-icon { background: color-mix(in oklch, var(--color-danger) 12%, var(--color-paper-raised)); color: var(--color-danger); }
  .loading-state { grid-template-columns: auto 1fr; align-items: center; }
  .spinner { width: 24px; height: 24px; border: 2px solid var(--color-rule); border-top-color: var(--color-accent); border-radius: 50%; animation: spin 800ms linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .table-head { display: none; }
  .deployment-row { min-height: 92px; padding: var(--space-4) var(--space-5); display: grid; grid-template-columns: 1fr auto; gap: var(--space-3); align-items: center; border-bottom: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; transition: background var(--duration-fast) var(--ease-out); }
  .deployment-row:last-child { border-bottom: 0; }
  .deployment-row:hover { background: var(--color-paper-subtle); }
  .revision { grid-column: 1 / -1; display: grid; gap: var(--space-1); }
  .revision strong { font-size: 14px; }
  .revision small, .deployment-row code, .deployment-row time { color: var(--color-muted); font: 12px var(--font-mono); }
  .deployment-row > span:nth-child(3), .deployment-row > span:nth-child(4) { display: none; }
  .row-arrow { grid-column: 2; grid-row: 1; color: var(--color-muted); }

  @media (min-width: 42rem) {
    .operational-summary { grid-template-columns: minmax(0, 1.8fr) minmax(160px, 1fr) minmax(160px, 1fr); }
    .primary-metric { min-height: 142px; }
    .metric { min-height: auto; padding: var(--space-6); border-top: 0; border-left: 1px solid var(--color-rule); align-items: flex-start; }
    .metric div { display: grid; justify-content: initial; gap: var(--space-2); }
    .list-toolbar { flex-direction: row; align-items: center; justify-content: space-between; }
    .controls { grid-template-columns: minmax(190px, 1fr) 140px auto; }
    .refresh { width: auto; }
    .state { min-height: 320px; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; justify-items: initial; }
  }

  @media (min-width: 64rem) {
    .table-head, .deployment-row { display: grid; grid-template-columns: 118px minmax(260px, 1fr) 100px 180px 24px; gap: var(--space-4); align-items: center; }
    .table-head { min-height: 44px; padding: 0 var(--space-5); background: var(--color-paper-subtle); color: var(--color-muted); font: 11px var(--font-mono); }
    .deployment-row { min-height: 76px; }
    .revision { grid-column: auto; }
    .deployment-row > span:nth-child(3), .deployment-row > span:nth-child(4) { display: block; }
    .row-arrow { grid-column: auto; grid-row: auto; }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner { animation: none; }
    .deployment-row { transition: none; }
  }
</style>
