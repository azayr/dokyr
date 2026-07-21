<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
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

  const formatDuration = (seconds) => (Number(seconds) > 0 ? `${seconds}s` : '—');
  const formatDate = (value) => (value ? new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : '—');
</script>

<Shell eyebrow="Workspace" title="Deployments" subtitle="Builds and releases from every project in this workspace.">
  <button slot="actions" class="btn" onclick={loadDeployments} disabled={loading}>
    <Icon name="refresh" size={14} />{loading ? 'Refreshing' : 'Refresh'}
  </button>

  <section class="metrics" aria-label="Deployment summary">
    <article>
      <span class="metric-label">Total releases</span>
      {#if loading}<span class="skeleton metric-skeleton"></span>{:else}<strong>{items.length}</strong>{/if}
      <small>Recorded across this workspace</small>
    </article>
    <article>
      <span class="metric-label">Running now</span>
      {#if loading}<span class="skeleton metric-skeleton"></span>{:else}<strong class="tone-info">{activeCount}</strong>{/if}
      <small>{activeCount ? 'Pipeline active' : 'Pipeline idle'}</small>
    </article>
    <article>
      <span class="metric-label">Needs attention</span>
      {#if loading}<span class="skeleton metric-skeleton"></span>{:else}<strong class={failedCount ? 'tone-danger' : ''}>{failedCount}</strong>{/if}
      <small>{failedCount ? 'Failed deployments' : 'No failures'}</small>
    </article>
  </section>

  <section class="panel" aria-busy={loading}>
    <header class="panel-header list-toolbar">
      <div>
        <span class="eyebrow">History</span>
        <h2>All deployments</h2>
      </div>
      <div class="controls">
        <label class="search-field">
          <Icon name="search" size={14} />
          <input bind:value={query} type="search" placeholder="Search commit or project" aria-label="Search deployments" />
        </label>
        <select class="select status-filter" bind:value={status} aria-label="Filter by status">
          <option value="all">All statuses</option>
          <option value="building">Building</option>
          <option value="deploying">Deploying</option>
          <option value="healthy">Healthy</option>
          <option value="failed">Failed</option>
        </select>
      </div>
    </header>

    {#if loading && items.length === 0}
      <div class="rows-loading">
        {#each Array(4) as _}
          <div class="row-skeleton"><span class="skeleton" style="width:90px;height:22px"></span><span class="skeleton" style="height:14px;flex:1"></span><span class="skeleton" style="width:70px;height:14px"></span></div>
        {/each}
      </div>
    {:else if error}
      <div class="empty-state">
        <span class="empty-icon"><Icon name="alert" size={20} /></span>
        <h3>Deployment history is unavailable</h3>
        <p>{error}. Check the service and try again.</p>
        <button class="btn" onclick={loadDeployments}><Icon name="refresh" size={13} /> Try again</button>
      </div>
    {:else if items.length === 0}
      <EmptyState icon="rocket" title="No deployments yet" description="Connect a repository or registry, create a project, and ship its first revision.">
        <a class="btn btn-primary btn-sm" href="/projects?new=1"><Icon name="plus" size={13} /> Create a project</a>
      </EmptyState>
    {:else if filteredItems.length === 0}
      <EmptyState icon="search" title="No matching deployments" description="Change the search or status filter to see more results.">
        <button class="btn btn-sm" onclick={() => { query = ''; status = 'all'; }}>Clear filters</button>
      </EmptyState>
    {:else}
      <div class="table-scroll">
        <table class="data-table deployment-table">
          <thead>
            <tr><th>Status</th><th>Revision</th><th>Duration</th><th>Started</th><th><span class="sr-only">Open</span></th></tr>
          </thead>
          <tbody>
            {#each filteredItems as deployment}
              <tr class="clickable" onclick={() => (location.href = '/deployments/' + deployment.id)}>
                <td><Status value={deployment.status} /></td>
                <td>
                  <span class="revision-cell">
                    <strong>{deployment.message || 'Deployment'}</strong>
                    <small>{deployment.projectId || 'Project'} · {deployment.commit || 'No commit'}</small>
                  </span>
                </td>
                <td><code>{formatDuration(deployment.duration)}</code></td>
                <td><time datetime={deployment.createdAt}>{formatDate(deployment.createdAt)}</time></td>
                <td class="arrow-cell"><Icon name="chevron-right" size={14} /></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
</Shell>

<style>
  .metrics {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    overflow: hidden;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .metrics article {
    min-height: 96px;
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
    font-size: 24px;
    font-weight: 700;
    letter-spacing: -0.03em;
    line-height: 1.1;
  }
  .metrics small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .metric-skeleton {
    width: 48px;
    height: 24px;
    margin-block: 2px;
  }
  .tone-info { color: var(--color-info); }
  .tone-danger { color: var(--color-danger); }

  .list-toolbar {
    flex-wrap: wrap;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .search-field {
    width: min(230px, 44vw);
    height: 34px;
    padding: 0 var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-muted);
  }
  .search-field input {
    min-width: 0;
    flex: 1;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--color-ink);
    font-size: var(--text-sm);
  }
  .status-filter {
    width: auto;
    height: 34px;
  }
  .revision-cell {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .revision-cell strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .revision-cell small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .deployment-table code {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .deployment-table time {
    color: var(--color-muted);
    font-size: var(--text-sm);
    white-space: nowrap;
  }
  .arrow-cell {
    color: var(--color-faint);
    text-align: right;
  }
  .clickable {
    cursor: pointer;
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
  @media (max-width: 40rem) {
    .metrics {
      grid-template-columns: 1fr;
    }
    .metrics article {
      min-height: 76px;
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
    .metrics article:last-child {
      border-bottom: 0;
    }
    .controls {
      width: 100%;
    }
    .search-field {
      flex: 1;
      width: auto;
    }
    .deployment-table th:nth-child(3),
    .deployment-table td:nth-child(3),
    .deployment-table th:nth-child(4),
    .deployment-table td:nth-child(4),
    .deployment-table th:nth-child(5),
    .deployment-table td:nth-child(5) {
      display: none;
    }
  }
</style>
