<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let data = { providers: { github: {}, gitlab: {} }, connections: [], registries: [] };
  let loading = true;
  let saving = false;
  let syncing = false;
  let syncAttempted = false;
  let error = '';
  let sourceError = '';
  let warning = '';
  let registry = { name: '', registryUrl: '', username: '', password: '' };
  let registryToRemove = null;
  let removeRegistryBusy = false;
  let accountToUnlink = null;
  let unlinkBusy = false;
  let unlinkError = '';

  const providerInfo = {
    github: { name: 'GitHub', mark: 'GH', hint: 'Organizations and private repositories' },
    gitlab: { name: 'GitLab', mark: 'GL', hint: 'Groups and private repositories' }
  };

  async function load(trySync = true) {
    loading = true;
    const response = await api('/api/integrations');
    data = await response.json();
    loading = false;
    if (trySync && !syncAttempted && data.providers?.github?.linked && data.providers?.github?.managed) {
      syncAttempted = true;
      await syncGitHubInstallations(false);
    }
  }

  async function syncGitHubInstallations(showEmpty = true) {
    syncing = true;
    sourceError = '';
    warning = '';
    const response = await api('/api/integrations/github/installations/sync', { method: 'POST' });
    const body = await response.json();
    if (!response.ok) {
      sourceError = body.error || 'Could not synchronize GitHub installations';
      syncing = false;
      return;
    }
    warning = body.warning || '';
    if (body.synced > 0) await load(false);
    else if (showEmpty) sourceError = body.message || 'No GitHub App installation was found for this account.';
    syncing = false;
  }

  onMount(load);

  async function saveRegistry() {
    saving = true;
    error = '';
    const response = await api('/api/integrations/registries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(registry)
    });
    const body = await response.json();
    if (!response.ok) {
      error = body.error;
      saving = false;
      return;
    }
    registry = { name: '', registryUrl: '', username: '', password: '' };
    toast.success('Registry credential saved');
    await load();
    saving = false;
  }

  async function removeRegistry() {
    if (!registryToRemove) return;
    removeRegistryBusy = true;
    await api('/api/integrations/registries/' + registryToRemove.id, { method: 'DELETE' });
    toast.success(`Registry ${registryToRemove.name} removed`);
    registryToRemove = null;
    removeRegistryBusy = false;
    await load();
  }

  async function unlinkSource() {
    if (!accountToUnlink) return;
    unlinkBusy = true;
    unlinkError = '';
    const response = await api('/api/integrations/sources/' + accountToUnlink.id, { method: 'DELETE' });
    const body = await response.json();
    if (!response.ok) {
      unlinkError = body.error || 'Could not unlink Git source';
      unlinkBusy = false;
      return;
    }
    toast.success(`${accountToUnlink.provider} account ${accountToUnlink.accountName} unlinked`);
    accountToUnlink = null;
    unlinkBusy = false;
    await load();
  }
</script>

<Shell eyebrow="Infrastructure" title="Sources" subtitle="Git providers and private registries used to build and pull application images.">
  {#if page.url.searchParams.get('connected')}
    <div class="alert alert-success"><Icon name="check-circle" size={15} /><div><strong>Account connected</strong><span>Private repositories are now available when creating a project.</span></div></div>
  {/if}
  {#if page.url.searchParams.get('error')}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Connection failed</strong><span>{page.url.searchParams.get('error')}</span></div></div>
  {/if}
  {#if sourceError}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Source error</strong><span>{sourceError}</span></div></div>
  {/if}
  {#if warning}
    <div class="alert alert-warning"><Icon name="alert" size={15} /><div><strong>Permission update required</strong><span>{warning} Open the GitHub App settings, enable read-only Contents permission, then approve the permission update.</span></div></div>
  {/if}

  <div class="providers">
    {#each Object.entries(providerInfo) as [key, provider]}
      {@const state = data.providers[key] || {}}
      {@const connections = data.connections.filter((item) => item.provider === key)}
      <article class="panel provider-card">
        <header class="provider-head">
          <span class="provider-mark {key}">
            {#if key === 'github'}<Icon name="github" size={18} />{:else}<Icon name="gitlab" size={18} />{/if}
          </span>
          <div>
            <h3>{provider.name}</h3>
            <p>{provider.hint}</p>
          </div>
          <span class="badge" class:badge-success={connections.length || (key === 'github' && state.linked)}>
            <i></i>{connections.length ? 'Connected' : key === 'github' && state.linked ? 'Linked' : state.configured ? 'Ready' : 'Setup required'}
          </span>
        </header>

        {#if connections.length}
          <div class="accounts">
            {#each connections as account}
              <div class="account-row">
                {#if account.accountAvatar}
                  <img src={account.accountAvatar} alt="" />
                {:else}
                  <i class="account-fallback">{provider.mark}</i>
                {/if}
                <span class="account-text">
                  <strong>{account.accountName}</strong>
                  <small>{account.repositorySelection === 'selected' ? 'Selected repositories' : account.repositorySelection === 'all' ? 'All repositories' : account.baseUrl}</small>
                  {#if key === 'github' && account.contentsPermission !== 'read' && account.contentsPermission !== 'write'}
                    <small class="permission-missing">Contents permission required for private deploys</small>
                  {/if}
                </span>
                <span class="account-actions">
                  {#if account.manageUrl}<a class="btn btn-sm" href={account.manageUrl}>Change access <Icon name="external" size={12} /></a>{/if}
                  <button class="btn btn-sm btn-danger" onclick={() => { unlinkError = ''; accountToUnlink = account; }}>Unlink</button>
                </span>
              </div>
            {/each}
          </div>
        {:else if key === 'github' && state.linked}
          <div class="linked-account">
            <span class="github-avatar"><Icon name="github" size={15} /></span>
            <span class="account-text"><strong>@{state.login}</strong><small>Linked in Settings · default GitHub account</small></span>
            <span class="badge badge-success"><i></i>Ready</span>
          </div>
        {:else}
          <div class="empty-account">No {provider.name} account connected yet.</div>
        {/if}

        <footer class="provider-footer">
          <div>
            {#if key === 'github'}
              <small>Repository access</small>
              <code>{connections.length ? 'Access can be changed any time on GitHub' : state.linked ? 'Choose all repositories or only selected repositories' : 'Uses the GitHub account linked in Settings'}</code>
            {:else}
              <small>OAuth callback</small>
              <code>{state.callbackUrl || '—'}</code>
            {/if}
          </div>
          <div class="provider-actions">
            {#if key === 'github'}
              {#if connections.length}
                <a class="btn btn-sm btn-primary" href="/api/integrations/github/install/start">Add installation <Icon name="arrow-right" size={12} /></a>
              {:else if state.linked && state.managed}
                <button class="btn btn-sm" type="button" onclick={() => syncGitHubInstallations(true)} disabled={syncing}>{syncing ? 'Checking…' : 'Refresh installation'}</button>
                <a class="btn btn-sm btn-primary" href="/api/integrations/github/install/start">Select repositories <Icon name="arrow-right" size={12} /></a>
              {:else}
                <a class="btn btn-sm btn-primary" href="/api/account/github/start">Link GitHub <Icon name="arrow-right" size={12} /></a>
              {/if}
            {:else if state.configured}
              <a class="btn btn-sm btn-primary" href={'/api/integrations/oauth/' + key + '/start'}>Connect {provider.name} <Icon name="arrow-right" size={12} /></a>
            {:else}
              <button class="btn btn-sm" disabled>Configure {provider.name}</button>
            {/if}
          </div>
        </footer>

        {#if key === 'github' && !state.managed}
          <div class="config-note accent">
            <Icon name="info" size={14} />
            <span>Link GitHub in <a href="/settings?section=security">Settings → Security</a>. Dokyr creates a private GitHub App and stores its credentials encrypted.</span>
          </div>
        {/if}
        {#if key === 'gitlab' && !state.configured}
          <div class="config-note">
            <Icon name="info" size={14} />
            <span>GitLab OAuth requires <code>GITLAB_CLIENT_ID</code> and <code>GITLAB_CLIENT_SECRET</code> in this server's environment.</span>
          </div>
        {/if}
      </article>
    {/each}
  </div>

  <section class="panel registry-panel">
    <header class="panel-header">
      <div>
        <span class="eyebrow">Container sources</span>
        <h2>Private Docker registries</h2>
      </div>
      <span class="badge">{data.registries.length} saved</span>
    </header>
    <div class="registry-grid">
      <form onsubmit={(event) => { event.preventDefault(); saveRegistry(); }}>
        <div class="form-title"><b>Add registry</b><small>Credentials are encrypted at rest</small></div>
        {#if error}<div class="alert alert-error registry-error"><Icon name="x-circle" size={14} /><div><span>{error}</span></div></div>{/if}
        <label class="field"><span>Display name</span><input class="input" bind:value={registry.name} placeholder="GitHub Container Registry" required /></label>
        <label class="field"><span>Registry host</span><input class="input input-mono" bind:value={registry.registryUrl} placeholder="ghcr.io" required /></label>
        <div class="split">
          <label class="field"><span>Username</span><input class="input input-mono" bind:value={registry.username} placeholder="octocat" /></label>
          <label class="field"><span>Password or token</span><input class="input" bind:value={registry.password} type="password" placeholder="••••••••••••" /></label>
        </div>
        <button class="btn btn-primary" disabled={saving}>{saving ? 'Saving…' : 'Save registry'}</button>
      </form>
      <div class="registry-list">
        {#if loading}
          <div class="rows-loading">
            {#each Array(2) as _}
              <div class="row-skeleton"><span class="skeleton" style="width:30px;height:30px"></span><span class="skeleton" style="height:14px;flex:1"></span></div>
            {/each}
          </div>
        {:else if !data.registries.length}
          <EmptyState icon="database" title="No private registries" description="Public images can still be deployed without a saved credential." />
        {:else}
          <div class="registry-rows">
            {#each data.registries as item}
              <div class="registry-row">
                <span class="registry-icon"><Icon name="database" size={14} /></span>
                <span class="registry-text"><strong>{item.name}</strong><small>{item.registryUrl}</small></span>
                <code>{item.username || 'anonymous'} / ••••••••</code>
                <button class="btn btn-sm btn-danger" onclick={() => (registryToRemove = item)}>Remove</button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </section>
</Shell>

{#if registryToRemove}
  <ConfirmDialog
    title={'Remove ' + registryToRemove.name + '?'}
    message="Existing projects keep their image reference, but new deployments that pull private images from this registry will fail."
    confirmLabel="Remove registry"
    busy={removeRegistryBusy}
    onConfirm={removeRegistry}
    onClose={() => (registryToRemove = null)}
  />
{/if}

{#if accountToUnlink}
  <ConfirmDialog
    title={'Unlink ' + accountToUnlink.accountName + '?'}
    message="Existing containers keep running, but Git services must be connected again before their next deployment."
    confirmLabel="Unlink account"
    busy={unlinkBusy}
    error={unlinkError}
    onConfirm={unlinkSource}
    onClose={() => (accountToUnlink = null)}
  />
{/if}

<style>
  .providers {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-4);
  }
  .provider-card {
    display: flex;
    flex-direction: column;
  }
  .provider-head {
    min-height: 72px;
    padding: var(--space-4) var(--space-5);
    display: grid;
    grid-template-columns: 40px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
  }
  .provider-mark {
    width: 40px;
    height: 40px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-md);
    background: var(--color-log-bg);
    color: var(--color-log-text);
  }
  .provider-mark.gitlab {
    background: #5c2d1e;
    color: #fc6d26;
  }
  :global(.theme-dark) .provider-mark.gitlab {
    background: #3a2118;
    color: #fc8a51;
  }
  .provider-head h3 {
    margin: 0;
    font-size: var(--text-md);
  }
  .provider-head p {
    margin: 2px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .accounts {
    border-top: 1px solid var(--color-rule);
  }
  .account-row {
    min-height: 64px;
    padding: var(--space-3) var(--space-5);
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
    background: var(--color-surface-subtle);
  }
  .account-row:last-child {
    border-bottom: 0;
  }
  .account-row img,
  .account-fallback,
  .github-avatar {
    width: 32px;
    height: 32px;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--color-paper-subtle);
    color: var(--color-muted);
    font: 600 var(--text-2xs) var(--font-mono);
    font-style: normal;
  }
  .github-avatar {
    background: var(--color-log-bg);
    color: var(--color-log-text);
  }
  .account-text {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .account-text strong {
    overflow: hidden;
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .account-text small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .account-text .permission-missing {
    color: var(--color-warning);
    font-weight: 600;
  }
  .account-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .linked-account {
    min-height: 64px;
    padding: var(--space-3) var(--space-5);
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border-top: 1px solid var(--color-rule);
    background: var(--color-surface-subtle);
  }
  .empty-account {
    padding: var(--space-5);
    border-top: 1px solid var(--color-rule);
    background: var(--color-surface-subtle);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .provider-footer {
    min-height: 64px;
    margin-top: auto;
    padding: var(--space-3) var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-top: 1px solid var(--color-rule);
  }
  .provider-footer > div:first-child {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .provider-footer small {
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .provider-footer code {
    overflow: hidden;
    max-width: 300px;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .config-note {
    padding: var(--space-3) var(--space-5);
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    border-top: 1px solid color-mix(in srgb, var(--color-warning) 30%, var(--color-rule));
    background: color-mix(in srgb, var(--color-warning) 6%, var(--color-paper-raised));
    color: var(--color-warning);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .config-note.accent {
    border-top-color: color-mix(in srgb, var(--color-accent) 26%, var(--color-rule));
    background: var(--color-accent-softer);
    color: var(--color-accent);
  }
  .config-note span {
    color: var(--color-ink-secondary);
  }
  .config-note a {
    color: var(--color-accent);
    font-weight: 600;
  }
  .config-note code {
    font-size: var(--text-xs);
  }

  .registry-grid {
    display: grid;
    grid-template-columns: 340px minmax(0, 1fr);
  }
  .registry-grid form {
    padding: var(--space-5);
    display: grid;
    align-content: start;
    gap: var(--space-3);
    border-right: 1px solid var(--color-rule);
    background: var(--color-surface-subtle);
  }
  .form-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .form-title b {
    font-size: var(--text-md);
  }
  .form-title small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .registry-error {
    margin-bottom: 0;
    padding: var(--space-2) var(--space-3);
  }
  .split {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-3);
  }
  .registry-list {
    min-width: 0;
  }
  .registry-rows {
    display: grid;
  }
  .registry-row {
    min-height: 60px;
    padding: var(--space-2) var(--space-5);
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) auto auto;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
  }
  .registry-row:last-child {
    border-bottom: 0;
  }
  .registry-icon {
    width: 32px;
    height: 32px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    background: var(--color-accent-soft);
    color: var(--color-accent);
  }
  .registry-text {
    min-width: 0;
    display: grid;
    gap: 1px;
  }
  .registry-text strong {
    overflow: hidden;
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .registry-text small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .registry-row code {
    color: var(--color-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
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

  @media (max-width: 62rem) {
    .providers {
      grid-template-columns: 1fr;
    }
    .registry-grid {
      grid-template-columns: 1fr;
    }
    .registry-grid form {
      border-right: 0;
      border-bottom: 1px solid var(--color-rule);
    }
  }
  @media (max-width: 40rem) {
    .split {
      grid-template-columns: 1fr;
    }
    .account-row {
      grid-template-columns: 32px minmax(0, 1fr);
    }
    .account-actions {
      grid-column: 2;
      justify-content: flex-start;
    }
    .provider-footer {
      align-items: flex-start;
      flex-direction: column;
    }
    .registry-row {
      grid-template-columns: 32px minmax(0, 1fr) auto;
    }
    .registry-row code {
      display: none;
    }
  }
</style>
