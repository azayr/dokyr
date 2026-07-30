<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  let project = { id: '', name: 'Project', sourceType: 'empty' };
  let workloads = [];
  let loading = true;
  let loadError = '';
  let submitting = false;
  let submitError = '';
  let name = '';

  $: applications = workloads.filter((item) => item.kind === 'application');
  $: databases = workloads.filter((item) => item.kind === 'database');
  $: selectedApplications = applications.filter((item) => item.selected);
  $: selectedDatabases = databases.filter((item) => item.selected);
  $: selectedVariableCount = selectedApplications.reduce((total, item) => total + (item.variables?.length || 0), 0);
  $: sameName = name.trim().toLowerCase() === project.name.trim().toLowerCase();
  $: canSubmit = name.trim() && !sameName && !submitting;

  onMount(loadCloneSource);

  function cloneName(value) {
    return `${value.trim()}-clone`;
  }

  async function loadCloneSource() {
    loading = true;
    loadError = '';
    try {
      const response = await api('/api/projects/' + page.params.id);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load the project');
      project = payload.project;
      name = cloneName(project.name);

      const next = [];
      if (project.sourceType !== 'empty') {
        next.push({
          id: 'main',
          kind: 'application',
          legacy: true,
          name: project.name,
          source: project.sourceType === 'repository' ? `${project.repository}@${project.branch}` : project.imageUrl,
          selected: true,
          expanded: false,
          variables: null,
          environmentError: ''
        });
      }
      for (const service of payload.applicationServices || []) {
        next.push({
          id: service.id,
          kind: 'application',
          legacy: false,
          name: service.name,
          source: service.sourceType === 'repository' ? `${service.repository}@${service.branch}` : service.imageUrl,
          selected: true,
          expanded: false,
          variables: null,
          environmentError: ''
        });
      }
      for (const service of payload.databaseServices || []) {
        next.push({
          id: service.id,
          kind: 'database',
          name: service.name,
          source: `${service.engine} · ${service.databaseName}`,
          engine: service.engine,
          selected: true
        });
      }
      workloads = next;
      await Promise.all(next.filter((item) => item.kind === 'application').map(loadEnvironment));
    } catch (cause) {
      loadError = cause instanceof Error ? cause.message : 'Could not load the project';
    } finally {
      loading = false;
    }
  }

  async function loadEnvironment(item) {
    item.environmentError = '';
    workloads = [...workloads];
    try {
      const endpoint = item.legacy ? `/api/projects/${project.id}/environment` : `/api/services/${item.id}/environment`;
      const response = await api(endpoint);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Environment could not be loaded');
      item.variables = (payload.variables || []).map((variable) => ({ ...variable, revealed: false }));
    } catch (cause) {
      item.environmentError = cause instanceof Error ? cause.message : 'Environment could not be loaded';
    }
    workloads = [...workloads];
  }

  function toggleWorkload(item) {
    item.selected = !item.selected;
    if (!item.selected) item.expanded = false;
    workloads = [...workloads];
  }

  function toggleAll(kind, selected) {
    workloads = workloads.map((item) => item.kind === kind ? { ...item, selected, expanded: selected ? item.expanded : false } : item);
  }

  function toggleEnvironment(item) {
    item.expanded = !item.expanded;
    workloads = [...workloads];
  }

  function addVariable(item) {
    item.variables = [...(item.variables || []), { key: '', value: '', secret: false, revealed: false }];
    workloads = [...workloads];
  }

  function removeVariable(item, index) {
    item.variables = item.variables.filter((_, current) => current !== index);
    workloads = [...workloads];
  }

  function updateVariable(item, index, field, value) {
    item.variables[index] = { ...item.variables[index], [field]: value };
    item.variables = [...item.variables];
    workloads = [...workloads];
  }

  function toggleReveal(item, index) {
    updateVariable(item, index, 'revealed', !item.variables[index].revealed);
  }

  async function cloneProject() {
    if (!canSubmit) return;
    submitting = true;
    submitError = '';
    try {
      const legacy = selectedApplications.find((item) => item.legacy);
      const body = {
        name: name.trim(),
        cloneLegacy: Boolean(legacy),
        legacyVariables: legacy?.variables?.map(({ key, value, secret }) => ({ key, value, secret })),
        applicationServices: selectedApplications
          .filter((item) => !item.legacy)
          .map((item) => ({
            id: item.id,
            variables: item.variables?.map(({ key, value, secret }) => ({ key, value, secret }))
          })),
        databaseServiceIds: selectedDatabases.map((item) => item.id)
      };
      const response = await api(`/api/projects/${project.id}/clone`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not clone the project');
      await goto(`/projects/${payload.project.id}`);
    } catch (cause) {
      submitError = cause instanceof Error ? cause.message : 'Could not clone the project';
      submitting = false;
    }
  }
</script>

<svelte:head><title>Clone {project.name} · Dokyr</title></svelte:head>

<Shell eyebrow="Projects" title={`Clone ${project.name}`} subtitle="Create a production-ready copy from the configuration you already trust.">
  <a slot="actions" class="btn" href={`/projects/${page.params.id}`}><Icon name="arrow-left" size={13} /> Back to project</a>

  {#if loading}
    <section class="panel loading-state" aria-busy="true">
      <span class="spinner"></span>
      <div><strong>Preparing the clone</strong><small>Reading services and environment configuration.</small></div>
    </section>
  {:else if loadError}
    <section class="panel error-state">
      <span><Icon name="alert" size={20} /></span>
      <div><strong>Clone configuration unavailable</strong><p>{loadError}</p></div>
      <button class="btn btn-sm" onclick={loadCloneSource}><Icon name="refresh" size={13} /> Try again</button>
    </section>
  {:else}
    <form class="clone-layout" onsubmit={(event) => { event.preventDefault(); cloneProject(); }}>
      <div class="clone-main">
        {#if submitError}
          <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Project not cloned</strong><span>{submitError}</span></div></div>
        {/if}

        <section class="panel identity-panel">
          <header class="panel-header">
            <div><span class="eyebrow">1 · Identity</span><h2>Name the new environment</h2></div>
            <span class="step-status"><Icon name="check" size={12} /> Required</span>
          </header>
          <div class="panel-body identity-grid">
            <label class="field">
              <span>New project name</span>
              <input class="input input-mono" bind:value={name} maxlength="100" required aria-invalid={sameName} />
              {#if sameName}<small class="field-error">Use a different name from {project.name}.</small>{:else}<small>This becomes a separate project and private service network.</small>{/if}
            </label>
            <div class="clone-direction" aria-label={`Clone ${project.name} as ${name || 'new project'}`}>
              <span><Icon name="box" size={15} /></span>
              <div><small>Source</small><strong>{project.name}</strong></div>
              <Icon name="arrow-right" size={17} />
              <span class="target"><Icon name="layers" size={15} /></span>
              <div><small>New environment</small><strong>{name || 'Choose a name'}</strong></div>
            </div>
          </div>
        </section>

        <section class="panel service-panel">
          <header class="panel-header">
            <div><span class="eyebrow">2 · Services</span><h2>Choose what to carry over</h2></div>
            {#if applications.length}
              <div class="selection-actions">
                <button type="button" onclick={() => toggleAll('application', true)}>Select all</button>
                <button type="button" onclick={() => toggleAll('application', false)}>Clear</button>
              </div>
            {/if}
          </header>
          {#if applications.length === 0}
            <div class="empty-group"><Icon name="box" size={19} /><div><strong>No application services</strong><span>The clone will start as an empty project unless you include a database.</span></div></div>
          {:else}
            <div class="workload-list">
              {#each applications as item}
                <article class="workload" class:selected={item.selected}>
                  <div class="workload-row">
                    <label class="workload-select">
                      <input type="checkbox" checked={item.selected} onchange={() => toggleWorkload(item)} />
                      <span class="workload-icon"><Icon name={item.legacy ? 'layers' : 'box'} size={17} /></span>
                      <span class="workload-copy">
                        <strong>{item.name}{#if item.legacy}<em>Main</em>{/if}</strong>
                        <small>{item.source || 'No source configured'}</small>
                      </span>
                    </label>
                    <span class="variable-count">{item.variables === null ? 'Reading variables…' : `${item.variables.length} variable${item.variables.length === 1 ? '' : 's'}`}</span>
                    <button class="environment-toggle" type="button" disabled={!item.selected || item.variables === null} onclick={() => toggleEnvironment(item)}>
                      <Icon name="key" size={13} /> {item.expanded ? 'Close editor' : 'Review environment'}
                    </button>
                  </div>

                  {#if item.environmentError}
                    <div class="environment-warning"><Icon name="alert" size={14} /><span>{item.environmentError}. The original encrypted values will still be copied.</span><button type="button" onclick={() => loadEnvironment(item)}>Retry</button></div>
                  {/if}

                  {#if item.selected && item.expanded && item.variables !== null}
                    <div class="environment-editor">
                      <header>
                        <div><strong>Environment variables</strong><span>Adjust staging-specific values before the copy is created.</span></div>
                        <button type="button" onclick={() => addVariable(item)}><Icon name="plus" size={12} /> Add variable</button>
                      </header>
                      {#if item.variables.length === 0}
                        <button class="empty-variables" type="button" onclick={() => addVariable(item)}><Icon name="plus" size={14} /> Add the first environment variable</button>
                      {:else}
                        <div class="variable-head"><span>Key</span><span>Value</span><span>Secret</span><span></span></div>
                        {#each item.variables as variable, index}
                          <div class="variable-row">
                            <input value={variable.key} oninput={(event) => updateVariable(item, index, 'key', event.currentTarget.value)} maxlength="128" placeholder="VARIABLE_NAME" aria-label="Variable key" />
                            <div class="value-input">
                              <input type={variable.secret && !variable.revealed ? 'password' : 'text'} value={variable.value} oninput={(event) => updateVariable(item, index, 'value', event.currentTarget.value)} placeholder="value" aria-label={`${variable.key || 'Variable'} value`} />
                              {#if variable.secret}<button type="button" onclick={() => toggleReveal(item, index)} aria-label={variable.revealed ? 'Hide secret value' : 'Reveal secret value'}><Icon name={variable.revealed ? 'eye-off' : 'eye'} size={13} /></button>{/if}
                            </div>
                            <label class="secret-check"><input type="checkbox" checked={variable.secret} onchange={(event) => updateVariable(item, index, 'secret', event.currentTarget.checked)} /><span>{variable.secret ? 'Secret' : 'Plain'}</span></label>
                            <button class="remove-variable" type="button" onclick={() => removeVariable(item, index)} aria-label={`Remove ${variable.key || 'variable'}`}><Icon name="trash" size={13} /></button>
                          </div>
                        {/each}
                      {/if}
                      <footer><Icon name="lock" size={13} /><span>Secret values stay encrypted in the new project.</span></footer>
                    </div>
                  {/if}
                </article>
              {/each}
            </div>
          {/if}
        </section>

        <section class="panel database-panel">
          <header class="panel-header">
            <div><span class="eyebrow">3 · Data services</span><h2>Choose database definitions</h2></div>
            {#if databases.length}
              <div class="selection-actions">
                <button type="button" onclick={() => toggleAll('database', true)}>Select all</button>
                <button type="button" onclick={() => toggleAll('database', false)}>Clear</button>
              </div>
            {/if}
          </header>
          {#if databases.length === 0}
            <div class="empty-group"><Icon name="database" size={19} /><div><strong>No databases to clone</strong><span>This project has no managed database definitions.</span></div></div>
          {:else}
            <div class="database-list">
              {#each databases as item}
                <label class="database-row" class:selected={item.selected}>
                  <input type="checkbox" checked={item.selected} onchange={() => toggleWorkload(item)} />
                  <span class="workload-icon database"><Icon name="database" size={17} /></span>
                  <span><strong>{item.name}</strong><small>{item.source}</small></span>
                  <em>Fresh volume</em>
                </label>
              {/each}
            </div>
            <div class="fresh-data-note"><Icon name="info" size={15} /><span>Credentials and settings are copied, and selected app variables are remapped to the new database hostname. Each database gets a fresh empty volume and no public port.</span></div>
          {/if}
        </section>
      </div>

      <aside class="clone-summary">
        <section class="panel">
          <header class="panel-header"><div><span class="eyebrow">Clone summary</span><h2>{name || 'New project'}</h2></div></header>
          <div class="summary-body">
            <dl>
              <div><dt>Applications</dt><dd>{selectedApplications.length}</dd></div>
              <div><dt>Databases</dt><dd>{selectedDatabases.length}</dd></div>
              <div><dt>Environment values</dt><dd>{selectedVariableCount}</dd></div>
            </dl>
            <div class="not-copied">
              <strong>Starts safely paused</strong>
              <ul>
                <li><Icon name="check" size={12} /> No containers or deployments</li>
                <li><Icon name="check" size={12} /> No domains or public ports</li>
                <li><Icon name="check" size={12} /> Auto-deploy disabled</li>
                <li><Icon name="check" size={12} /> No deployment history</li>
              </ul>
            </div>
          </div>
          <footer class="panel-footer summary-footer">
            <span>Review, clone, then deploy when ready.</span>
            <button class="btn btn-primary" type="submit" disabled={!canSubmit}><Icon name="copy" size={14} /> {submitting ? 'Cloning project…' : 'Clone project'}</button>
          </footer>
        </section>
      </aside>
    </form>
  {/if}
</Shell>

<style>
  .clone-layout { display: grid; grid-template-columns: minmax(0, 1fr) 320px; gap: var(--space-4); align-items: start; }
  .clone-main { min-width: 0; display: grid; gap: var(--space-4); }
  .loading-state, .error-state { min-height: 160px; padding: var(--space-6); display: flex; align-items: center; justify-content: center; gap: var(--space-3); }
  .loading-state div, .error-state div { display: grid; gap: 3px; }
  .loading-state small, .error-state p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .error-state > span { width: 38px; height: 38px; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-danger-soft); color: var(--color-danger); }
  .error-state button { margin-left: var(--space-4); }
  .spinner { width: 18px; height: 18px; border: 2px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .identity-panel, .service-panel, .database-panel { overflow: visible; }
  .step-status { min-height: 24px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 5px; border-radius: 999px; background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
  .identity-grid { display: grid; grid-template-columns: minmax(220px, .9fr) minmax(340px, 1.1fr); gap: var(--space-5); align-items: end; }
  .field-error { color: var(--color-danger) !important; }
  .clone-direction { min-height: 66px; padding: var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) 18px 34px minmax(0, 1fr); align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-muted); }
  .clone-direction > span { width: 34px; height: 34px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .clone-direction > span.target { border-color: color-mix(in srgb, var(--color-accent) 30%, var(--color-rule)); background: var(--color-accent-soft); color: var(--color-accent); }
  .clone-direction div { min-width: 0; display: grid; }
  .clone-direction small { color: var(--color-muted); font-size: var(--text-2xs); text-transform: uppercase; }
  .clone-direction strong { overflow: hidden; color: var(--color-ink); font: 600 var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .selection-actions { display: flex; gap: var(--space-1); }
  .selection-actions button { min-height: 28px; padding: 0 var(--space-2); border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .selection-actions button:hover { background: var(--color-accent-soft); }
  .workload-list { display: grid; }
  .workload { border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); transition: background var(--duration-fast) var(--ease-out); }
  .workload:last-child { border-bottom: 0; }
  .workload.selected { background: color-mix(in srgb, var(--color-accent) 2.5%, var(--color-paper-raised)); }
  .workload-row { min-height: 72px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: var(--space-4); }
  .workload-select { min-width: 0; display: grid; grid-template-columns: 16px 38px minmax(0, 1fr); align-items: center; gap: var(--space-3); cursor: pointer; }
  .workload-select > input, .database-row > input { width: 15px; height: 15px; accent-color: var(--color-accent); }
  .workload-icon { width: 38px; height: 38px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-muted); }
  .selected .workload-icon { border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); background: var(--color-accent-soft); color: var(--color-accent); }
  .workload-copy { min-width: 0; display: grid; gap: 3px; }
  .workload-copy strong { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); }
  .workload-copy strong em { padding: 2px 5px; border-radius: var(--radius-xs); background: var(--color-paper-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 700; text-transform: uppercase; }
  .workload-copy small { overflow: hidden; color: var(--color-muted); font: var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .variable-count { color: var(--color-muted); font-size: var(--text-xs); white-space: nowrap; }
  .environment-toggle { min-height: 30px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .environment-toggle:disabled { opacity: .5; cursor: not-allowed; }
  .environment-warning { margin: 0 var(--space-5) var(--space-3) 70px; padding: var(--space-2) var(--space-3); display: flex; align-items: center; gap: var(--space-2); border-radius: var(--radius-sm); background: var(--color-warning-soft); color: var(--color-warning); font-size: var(--text-xs); }
  .environment-warning span { flex: 1; }
  .environment-warning button { border: 0; background: transparent; color: inherit; font-weight: 700; cursor: pointer; }
  .environment-editor { margin: 0 var(--space-5) var(--space-4) 70px; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .environment-editor > header { min-height: 54px; padding: var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .environment-editor > header div { display: grid; gap: 2px; }
  .environment-editor > header strong { font-size: var(--text-xs); }
  .environment-editor > header span { color: var(--color-muted); font-size: var(--text-xs); }
  .environment-editor > header button, .empty-variables { min-height: 28px; padding: 0 var(--space-2); display: inline-flex; align-items: center; justify-content: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-accent); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .variable-head, .variable-row { display: grid; grid-template-columns: minmax(130px, .8fr) minmax(180px, 1.2fr) 76px 30px; align-items: center; gap: var(--space-2); }
  .variable-head { padding: var(--space-2) var(--space-3) 0; color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .05em; text-transform: uppercase; }
  .variable-row { padding: var(--space-2) var(--space-3); }
  .variable-row input[type='text'], .variable-row input[type='password'] { width: 100%; height: 32px; padding: 0 var(--space-2); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font: var(--text-xs) var(--font-mono); outline: 0; }
  .variable-row input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-accent) 12%, transparent); }
  .value-input { position: relative; }
  .value-input input { padding-right: 34px !important; }
  .value-input button { position: absolute; top: 1px; right: 1px; width: 30px; height: 30px; display: grid; place-items: center; border: 0; background: transparent; color: var(--color-muted); cursor: pointer; }
  .secret-check { display: grid; grid-template-columns: 14px 1fr; align-items: center; gap: 5px; color: var(--color-muted); font-size: var(--text-2xs); cursor: pointer; }
  .secret-check input { accent-color: var(--color-accent); }
  .remove-variable { width: 28px; height: 28px; display: grid; place-items: center; border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }
  .remove-variable:hover { background: var(--color-danger-soft); color: var(--color-danger); }
  .empty-variables { margin: var(--space-4); border-style: dashed; }
  .environment-editor > footer { min-height: 36px; padding: 0 var(--space-3); display: flex; align-items: center; gap: 6px; border-top: 1px solid var(--color-rule); color: var(--color-muted); font-size: var(--text-xs); }
  .database-list { display: grid; }
  .database-row { min-height: 68px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 16px 38px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); cursor: pointer; }
  .database-row > span:not(.workload-icon) { display: grid; gap: 2px; }
  .database-row strong { font-size: var(--text-sm); }
  .database-row small { color: var(--color-muted); font-size: var(--text-xs); }
  .database-row em { padding: 3px 7px; border-radius: 999px; background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-2xs); font-style: normal; font-weight: 700; text-transform: uppercase; }
  .fresh-data-note { min-height: 46px; padding: var(--space-2) var(--space-5); display: flex; align-items: center; gap: var(--space-2); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .empty-group { min-height: 92px; padding: var(--space-5); display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); }
  .empty-group div { display: grid; gap: 2px; }
  .empty-group strong { color: var(--color-ink); font-size: var(--text-sm); }
  .empty-group span { font-size: var(--text-xs); }
  .clone-summary { position: sticky; top: 76px; }
  .summary-body { padding: var(--space-4); display: grid; gap: var(--space-4); }
  .summary-body dl { margin: 0; display: grid; border: 1px solid var(--color-rule); border-radius: var(--radius-md); }
  .summary-body dl div { min-height: 42px; padding: 0 var(--space-3); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .summary-body dl div:last-child { border-bottom: 0; }
  .summary-body dt { color: var(--color-muted); font-size: var(--text-xs); }
  .summary-body dd { margin: 0; font: 600 var(--text-sm) var(--font-mono); }
  .not-copied { padding: var(--space-3); border-left: 3px solid var(--color-success); border-radius: 0 var(--radius-sm) var(--radius-sm) 0; background: var(--color-success-soft); }
  .not-copied strong { color: var(--color-success); font-size: var(--text-xs); }
  .not-copied ul { margin: var(--space-2) 0 0; padding: 0; display: grid; gap: var(--space-2); list-style: none; }
  .not-copied li { display: flex; align-items: center; gap: 6px; color: var(--color-ink-secondary); font-size: var(--text-xs); }
  .summary-footer { align-items: stretch; flex-direction: column; }
  .summary-footer > span { color: var(--color-muted); font-size: var(--text-xs); text-align: center; }
  .summary-footer button { width: 100%; min-height: 38px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (max-width: 68rem) { .clone-layout { grid-template-columns: 1fr; } .clone-summary { position: static; } .summary-body { grid-template-columns: 1fr 1fr; } }
  @media (max-width: 48rem) { .identity-grid { grid-template-columns: 1fr; } .workload-row { grid-template-columns: minmax(0, 1fr) auto; } .variable-count { display: none; } .environment-toggle { grid-column: 1 / -1; margin-left: 70px; width: max-content; } .environment-editor, .environment-warning { margin-left: var(--space-5); } .variable-head { display: none; } .variable-row { grid-template-columns: 1fr 72px 30px; padding-block: var(--space-3); border-bottom: 1px solid var(--color-rule); } .variable-row > input:first-child, .value-input { grid-column: 1 / -1; } .summary-body { grid-template-columns: 1fr; } }
  @media (max-width: 34rem) { .clone-direction { grid-template-columns: 30px minmax(0, 1fr); } .clone-direction > :global(svg), .clone-direction > span.target, .clone-direction > span.target + div { display: none; } .panel-header { align-items: flex-start; flex-direction: column; } .selection-actions { align-self: flex-end; } .workload-row { gap: var(--space-2); } .workload-select { grid-template-columns: 16px 32px minmax(0, 1fr); gap: var(--space-2); } .workload-icon { width: 32px; height: 32px; } .environment-toggle { margin-left: 56px; } .database-row { grid-template-columns: 16px 32px minmax(0, 1fr); } .database-row em { grid-column: 3; width: max-content; } }
</style>
