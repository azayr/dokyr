<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let projects = [];
  let loading = true;
  let loadError = '';
  let open = false;
  let busy = false;
  let error = '';
  let query = '';
  let form = { name: '', sourceType: 'empty' };

  $: filtered = projects.filter((project) =>
    `${project.name} ${project.domain || ''} ${project.repository || ''} ${project.imageUrl || ''}`.toLowerCase().includes(query.trim().toLowerCase())
  );

  onMount(async () => {
    open = page.url.searchParams.get('new') === '1';
    await loadProjects();
  });

  async function loadProjects() {
    loading = true;
    loadError = '';
    try {
      const response = await api('/api/projects');
      if (!response.ok) throw new Error('Could not load projects');
      projects = await response.json();
    } catch (cause) {
      loadError = cause instanceof Error ? cause.message : 'Could not load projects';
    } finally {
      loading = false;
    }
  }

  async function create() {
    busy = true;
    error = '';
    const response = await api('/api/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form)
    });
    const data = await response.json();
    if (!response.ok) {
      error = data.error;
      busy = false;
      return;
    }
    toast.success(`Project ${data.name || form.name} created`);
    location.href = '/projects/' + data.id;
  }

  const sourceLabel = (project) => (project.sourceType === 'empty' ? 'No services yet' : project.sourceType === 'image' ? project.imageUrl : project.repository);
  const sourceIcon = (project) => (project.sourceType === 'empty' ? 'grid' : project.sourceType === 'image' ? 'box' : 'git');
</script>

<Shell eyebrow="Workspace" title="Projects" subtitle="Applications composed from independent services and databases.">
  <button slot="actions" class="btn btn-primary" onclick={() => (open = true)}><Icon name="plus" size={14} /> New project</button>

  {#if open}
    <section class="panel creator" aria-label="Create a project">
      <header class="panel-header">
        <div>
          <span class="eyebrow">New project</span>
          <h2>Create an empty workspace</h2>
        </div>
        <button class="icon-close" onclick={() => (open = false)} aria-label="Close project form"><Icon name="x" size={15} /></button>
      </header>
      <form onsubmit={(event) => { event.preventDefault(); create(); }}>
        {#if error}<div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Project not created</strong><span>{error}</span></div></div>{/if}
        <div class="creator-grid">
          <label class="field">
            <span>Project name</span>
            <input class="input input-mono" bind:value={form.name} placeholder="customer-api" required />
          </label>
          <div class="creator-note">
            <Icon name="grid" size={16} />
            <div>
              <b>Every application is a service</b>
              <span>Add the frontend, API, workers, and databases independently after creation.</span>
            </div>
          </div>
        </div>
        <footer class="creator-actions">
          <button type="button" class="btn" onclick={() => (open = false)}>Cancel</button>
          <button class="btn btn-primary" type="submit" disabled={busy}><Icon name="plus" size={13} />{busy ? 'Creating…' : 'Create project'}</button>
        </footer>
      </form>
    </section>
  {/if}

  <section class="panel" aria-busy={loading}>
    <header class="panel-header list-toolbar">
      <div>
        <span class="eyebrow">Applications</span>
        <h2>All projects {#if !loading}<span class="count">{filtered.length}</span>{/if}</h2>
      </div>
      {#if projects.length > 0}
        <label class="search-field">
          <Icon name="search" size={14} />
          <input bind:value={query} type="search" placeholder="Search projects" aria-label="Search projects" />
        </label>
      {/if}
    </header>

    {#if loading}
      <div class="rows-loading">
        {#each Array(3) as _}
          <div class="row-skeleton"><span class="skeleton" style="width:34px;height:34px"></span><span class="skeleton" style="height:14px;flex:1"></span><span class="skeleton" style="width:80px;height:22px"></span></div>
        {/each}
      </div>
    {:else if loadError}
      <div class="empty-state">
        <span class="empty-icon"><Icon name="alert" size={20} /></span>
        <h3>Projects are unavailable</h3>
        <p>{loadError}</p>
        <button class="btn" onclick={loadProjects}><Icon name="refresh" size={13} /> Try again</button>
      </div>
    {:else if projects.length === 0}
      <EmptyState icon="box" title="No projects yet" description="Create a workspace, then add only the services it needs.">
        <button class="btn btn-primary btn-sm" onclick={() => (open = true)}><Icon name="plus" size={13} /> Create first project</button>
      </EmptyState>
    {:else if filtered.length === 0}
      <EmptyState icon="search" title="No matching projects" description="Try a different search query.">
        <button class="btn btn-sm" onclick={() => (query = '')}>Clear search</button>
      </EmptyState>
    {:else}
      <div class="table-scroll">
        <table class="data-table project-table">
          <thead>
            <tr><th>Project</th><th>Status</th><th>Source</th><th>Updated</th><th><span class="sr-only">Open</span></th></tr>
          </thead>
          <tbody>
            {#each filtered as project}
              <tr class="clickable" onclick={() => (location.href = '/projects/' + project.id)}>
                <td>
                  <span class="project-cell">
                    <span class="project-icon"><Icon name={sourceIcon(project)} size={14} /></span>
                    <span class="project-cell-text"><strong>{project.name}</strong><small>{project.domain || 'No public domain'}</small></span>
                  </span>
                </td>
                <td><Status value={project.status} /></td>
                <td><code class="source-cell">{sourceLabel(project)}</code></td>
                <td><time>{new Date(project.updatedAt).toLocaleDateString()}</time></td>
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
  .creator {
    margin-bottom: var(--space-4);
  }
  .creator form {
    padding: var(--space-5);
  }
  .creator .alert {
    margin-bottom: var(--space-4);
  }
  .icon-close {
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-muted);
    cursor: pointer;
  }
  .icon-close:hover {
    background: var(--color-paper-subtle);
    color: var(--color-ink);
  }
  .creator-grid {
    display: grid;
    grid-template-columns: minmax(220px, 0.8fr) minmax(280px, 1.2fr);
    gap: var(--space-4);
    align-items: start;
  }
  .creator-note {
    min-height: 58px;
    padding: var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-surface-subtle);
    color: var(--color-accent);
  }
  .creator-note div {
    display: grid;
    gap: 2px;
  }
  .creator-note b {
    color: var(--color-ink);
    font-size: var(--text-sm);
  }
  .creator-note span {
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.45;
  }
  .creator-actions {
    margin-top: var(--space-4);
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .list-toolbar {
    gap: var(--space-3);
  }
  .count {
    margin-left: 2px;
    color: var(--color-muted);
    font-weight: 500;
  }
  .search-field {
    width: min(240px, 42vw);
    height: 32px;
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
  .project-cell {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .project-icon {
    width: 32px;
    height: 32px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-surface-subtle);
    color: var(--color-muted);
  }
  .project-cell-text {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .project-cell-text strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .project-cell-text small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .source-cell {
    display: block;
    max-width: 260px;
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .project-table time {
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
  @media (max-width: 46rem) {
    .creator-grid {
      grid-template-columns: 1fr;
    }
    .project-table th:nth-child(3),
    .project-table td:nth-child(3),
    .project-table th:nth-child(4),
    .project-table td:nth-child(4),
    .project-table th:nth-child(5),
    .project-table td:nth-child(5) {
      display: none;
    }
  }
</style>
