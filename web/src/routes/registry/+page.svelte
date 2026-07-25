<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  const emptySettings = () => ({
    storage: 'filesystem',
    s3Region: '',
    s3Bucket: '',
    s3AccessKey: '',
    s3SecretKey: '',
    hasS3SecretKey: false,
    s3Endpoint: '',
    s3ForcePathStyle: false,
    s3Secure: true
  });

  let loading = true;
  let repositoriesLoading = true;
  let saving = false;
  let collecting = false;
  let deleting = false;
  let tokensLoading = true;
  let tokenCreating = false;
  let tokenFormOpen = false;
  let tokenName = '';
  let tokenPermission = 'read_only';
  let tokenError = '';
  let accessTokens = [];
  let registryUsername = '';
  let registryHosts = [];
  let createdCredential = null;
  let revokeTarget = null;
  let revoking = false;
  let revokeError = '';
  let error = '';
  let warning = '';
  let repositoriesError = '';
  let status = { available: false };
  let settings = emptySettings();
  let repositories = [];
  let filter = '';
  let gcResult = null;
  let deleteTarget = null;
  let deleteError = '';

  onMount(load);

  $: visibleRepositories = repositories.filter((item) => item.name.toLowerCase().includes(filter.trim().toLowerCase()) || (item.tags || []).some((tag) => tag.toLowerCase().includes(filter.trim().toLowerCase())));
  $: tagCount = repositories.reduce((count, item) => count + (item.tags || []).length, 0);
  $: storageLabel = settings.storage === 's3' ? 'S3-compatible object storage' : 'Docker volume filesystem';
  $: registryHost = registryHosts[0] || 'registry.example.com';
  $: loginCommand = `docker login ${registryHost} --username ${createdCredential?.username || registryUsername || 'you@example.com'}`;

  async function load() {
    loading = true;
    error = '';
    warning = '';
    try {
      const [statusResponse, settingsResponse, tokensResponse] = await Promise.all([
        api('/api/registry/status'),
        api('/api/registry/settings'),
        api('/api/registry/access-tokens')
      ]);
      const statusPayload = await statusResponse.json();
      const settingsPayload = await settingsResponse.json();
      const tokensPayload = await tokensResponse.json();
      if (!statusResponse.ok) throw new Error(statusPayload.error || 'Could not load registry status');
      if (!settingsResponse.ok) throw new Error(settingsPayload.error || 'Could not load registry settings');
      if (!tokensResponse.ok) throw new Error(tokensPayload.error || 'Could not load registry tokens');
      status = statusPayload;
      settings = { ...emptySettings(), ...settingsPayload, s3SecretKey: '' };
      accessTokens = tokensPayload.tokens || [];
      registryUsername = tokensPayload.username || '';
      registryHosts = tokensPayload.registryHosts || [];
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load registry';
    } finally {
      loading = false;
      tokensLoading = false;
    }
    await loadRepositories();
  }

  async function loadRepositories() {
    repositoriesLoading = true;
    repositoriesError = '';
    try {
      const response = await api('/api/registry/repositories');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load repositories');
      repositories = payload.repositories || [];
    } catch (cause) {
      repositoriesError = cause instanceof Error ? cause.message : 'Could not load repositories';
      repositories = [];
    } finally {
      repositoriesLoading = false;
    }
  }

  async function createAccessToken() {
    tokenCreating = true;
    tokenError = '';
    try {
      const response = await api('/api/registry/access-tokens', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: tokenName, permission: tokenPermission })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not create registry token');
      accessTokens = [payload.token, ...accessTokens];
      registryUsername = payload.username || registryUsername;
      registryHosts = payload.registryHosts || registryHosts;
      createdCredential = payload;
      tokenName = '';
      tokenPermission = 'read_only';
      tokenFormOpen = false;
      toast.success('Registry token created');
    } catch (cause) {
      tokenError = cause instanceof Error ? cause.message : 'Could not create registry token';
    } finally {
      tokenCreating = false;
    }
  }

  async function revokeAccessToken() {
    if (!revokeTarget) return;
    revoking = true;
    revokeError = '';
    try {
      const response = await api(`/api/registry/access-tokens/${encodeURIComponent(revokeTarget.id)}`, { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not revoke registry token');
      accessTokens = accessTokens.filter((token) => token.id !== revokeTarget.id);
      revokeTarget = null;
      toast.success('Registry token revoked');
    } catch (cause) {
      revokeError = cause instanceof Error ? cause.message : 'Could not revoke registry token';
    } finally {
      revoking = false;
    }
  }

  async function copyText(value, label) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`${label} copied`);
    } catch {
      toast.error(`Could not copy ${label.toLowerCase()}`);
    }
  }

  function formatDate(value) {
    if (!value) return 'Never';
    return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
  }

  async function saveSettings() {
    saving = true;
    error = '';
    warning = '';
    try {
      const response = await api('/api/registry/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          storage: settings.storage,
          s3Region: settings.s3Region,
          s3Bucket: settings.s3Bucket,
          s3AccessKey: settings.s3AccessKey,
          s3SecretKey: settings.s3SecretKey,
          s3Endpoint: settings.s3Endpoint,
          s3ForcePathStyle: settings.s3ForcePathStyle,
          s3Secure: settings.s3Secure
        })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save registry settings');
      settings = { ...emptySettings(), ...payload.settings, s3SecretKey: '' };
      warning = payload.warning || '';
      toast.success(warning ? 'Registry settings saved' : 'Registry settings applied');
      await load();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not save registry settings';
    } finally {
      saving = false;
    }
  }

  async function runGarbageCollection(dryRun) {
    collecting = true;
    error = '';
    gcResult = null;
    try {
      const response = await api('/api/registry/garbage-collection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dryRun })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Garbage collection failed');
      gcResult = payload;
      toast.success(dryRun ? 'Dry-run complete' : 'Garbage collection complete');
      if (!dryRun) await loadRepositories();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Garbage collection failed';
    } finally {
      collecting = false;
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    deleting = true;
    deleteError = '';
    try {
      const params = new URLSearchParams({ name: deleteTarget.name, tag: deleteTarget.tag });
      const response = await api('/api/registry/tags?' + params.toString(), { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not delete tag');
      deleteTarget = null;
      toast.success('Registry tag deleted');
      await loadRepositories();
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not delete tag';
    } finally {
      deleting = false;
    }
  }
</script>

<Shell eyebrow="Infrastructure" title="Registry" subtitle="Docker Distribution storage, repository catalog, tag deletion, and garbage collection.">
  <div slot="actions" class="registry-status" class:online={status.available}>
    <i></i>
    <span><strong>{status.available ? 'Available' : 'Unavailable'}</strong><small>{status.container || 'registry:5000'}</small></span>
  </div>

  {#if error}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Registry action failed</strong><span>{error}</span></div></div>
  {/if}
  {#if warning}
    <div class="alert alert-warning"><Icon name="alert" size={15} /><div><strong>Settings saved with warning</strong><span>{warning}</span></div></div>
  {/if}
  {#if status.error}
    <div class="alert alert-warning"><Icon name="alert" size={15} /><div><strong>Registry is not healthy</strong><span>{status.error}</span></div></div>
  {/if}

  <div class="registry-grid">
    <section class="panel settings-panel" aria-label="Registry storage settings">
      <header class="panel-header">
        <div><span class="eyebrow">Storage backend</span><h2>Settings</h2></div>
        <b>{storageLabel}</b>
      </header>
      <form class="settings-form" onsubmit={(event) => { event.preventDefault(); saveSettings(); }}>
        <div class="storage-picker" role="radiogroup" aria-label="Storage backend">
          <label class:active={settings.storage === 'filesystem'}><input type="radio" bind:group={settings.storage} value="filesystem" /><Icon name="hard-drive" size={16} /><span><strong>Filesystem</strong><small>Use the registry Docker volume.</small></span></label>
          <label class:active={settings.storage === 's3'}><input type="radio" bind:group={settings.storage} value="s3" /><Icon name="database" size={16} /><span><strong>S3</strong><small>Use an object storage bucket.</small></span></label>
        </div>

        {#if settings.storage === 's3'}
          <div class="form-grid">
            <label><span>Region</span><input bind:value={settings.s3Region} autocomplete="off" placeholder="us-east-1" required /></label>
            <label><span>Bucket</span><input bind:value={settings.s3Bucket} autocomplete="off" placeholder="dokyr-registry" required /></label>
            <label><span>Access key</span><input bind:value={settings.s3AccessKey} autocomplete="off" required /></label>
            <label><span>Secret key <em>{settings.hasS3SecretKey ? 'leave blank to keep current' : 'required'}</em></span><input bind:value={settings.s3SecretKey} type="password" autocomplete="new-password" placeholder={settings.hasS3SecretKey ? 'Stored encrypted' : ''} /></label>
            <label class="wide"><span>Endpoint <em>optional</em></span><input bind:value={settings.s3Endpoint} autocomplete="off" placeholder="https://minio.example.com" /></label>
            <div class="toggle-row wide">
              <label class="switch"><input type="checkbox" bind:checked={settings.s3ForcePathStyle} /><span></span><em>{settings.s3ForcePathStyle ? 'On' : 'Off'}</em></label>
              <div><strong>Force path-style URLs</strong><small>Enable for MinIO and other S3-compatible services that do not support bucket hostnames.</small></div>
            </div>
            <div class="toggle-row wide">
              <label class="switch"><input type="checkbox" bind:checked={settings.s3Secure} /><span></span><em>{settings.s3Secure ? 'TLS' : 'HTTP'}</em></label>
              <div><strong>Secure transport</strong><small>Keep enabled unless the endpoint is only available over plain HTTP.</small></div>
            </div>
          </div>
        {:else}
          <div class="filesystem-note"><Icon name="hard-drive" size={18} /><div><strong>Registry data stays in the Compose volume.</strong><span>Changing from S3 back to filesystem does not migrate objects automatically.</span></div></div>
        {/if}

        <footer>
          <button type="submit" class="primary" disabled={saving || loading}><Icon name="check" size={14} />{saving ? 'Saving…' : 'Save and restart registry'}</button>
        </footer>
      </form>
    </section>

    <section class="panel summary-panel" aria-label="Registry summary">
      <header class="panel-header">
        <div><span class="eyebrow">Catalog</span><h2>Overview</h2></div>
        <button type="button" class="icon-action" aria-label="Refresh registry" onclick={load} disabled={loading || repositoriesLoading}><Icon name="refresh" size={15} /></button>
      </header>
      <div class="summary-metrics">
        <div><span>Repositories</span><strong>{repositories.length}</strong></div>
        <div><span>Tags</span><strong>{tagCount}</strong></div>
        <div><span>Container state</span><strong>{status.state || (status.available ? 'running' : 'unknown')}</strong></div>
      </div>
      <div class="gc-actions">
        <button type="button" onclick={() => runGarbageCollection(true)} disabled={collecting || !status.container}><Icon name="search" size={14} />Dry-run GC</button>
        <button type="button" class="danger" onclick={() => runGarbageCollection(false)} disabled={collecting || !status.container}><Icon name="trash" size={14} />Run GC</button>
      </div>
      {#if gcResult}
        <div class="gc-output">
          <header><strong>{gcResult.dryRun ? 'Dry-run output' : 'Garbage collection output'}</strong><span>exit {gcResult.exitCode}{gcResult.truncated ? ' · truncated' : ''}</span></header>
          <pre>{gcResult.output || 'Command completed without output.'}</pre>
        </div>
      {/if}
    </section>
  </div>

  <section class="panel access-panel" aria-label="Registry access tokens">
    <header class="panel-header">
      <div><span class="eyebrow">Authentication</span><h2>Access tokens</h2></div>
      <button type="button" class="primary token-create-button" onclick={() => { tokenFormOpen = !tokenFormOpen; tokenError = ''; }} disabled={tokenCreating}>
        <Icon name={tokenFormOpen ? 'x' : 'plus'} size={14} />{tokenFormOpen ? 'Cancel' : 'Generate token'}
      </button>
    </header>

    <div class="access-intro">
      <div class="access-mark"><Icon name="key" size={20} /></div>
      <div><strong>Personal credentials for Docker</strong><span>Generate a separate secret for each machine or CI runner. Your Dokyr password is never accepted by the registry.</span></div>
      <code>{registryHost}</code>
    </div>

    {#if registryHost === 'registry.invalid'}
      <div class="token-notice"><Icon name="alert" size={15} /><span>Set <code>REGISTRY_HOSTS</code> to a real registry hostname before using these credentials outside Dokyr.</span></div>
    {/if}

    {#if tokenFormOpen}
      <form class="token-form" onsubmit={(event) => { event.preventDefault(); createAccessToken(); }}>
        <label class="token-name"><span>Token name</span><input bind:value={tokenName} maxlength="80" autocomplete="off" placeholder="Production CI" required /></label>
        <fieldset>
          <legend>Permission</legend>
          <label class:active={tokenPermission === 'read_only'}>
            <input type="radio" bind:group={tokenPermission} value="read_only" />
            <Icon name="eye" size={16} />
            <span><strong>Read only</strong><small>Pull images from any repository.</small></span>
          </label>
          <label class:active={tokenPermission === 'read_write'}>
            <input type="radio" bind:group={tokenPermission} value="read_write" />
            <Icon name="shield" size={16} />
            <span><strong>Read & write</strong><small>Pull and push images.</small></span>
          </label>
        </fieldset>
        {#if tokenError}<div class="token-form-error"><Icon name="x-circle" size={14} />{tokenError}</div>{/if}
        <footer><button type="submit" class="primary" disabled={tokenCreating || !tokenName.trim()}>{tokenCreating ? 'Generating…' : 'Generate credential'}</button></footer>
      </form>
    {/if}

    {#if createdCredential}
      <div class="credential-reveal">
        <header>
          <div class="reveal-icon"><Icon name="check" size={16} /></div>
          <div><strong>Token ready — copy it now</strong><span>This secret is shown once. Dokyr cannot recover it after you close this card.</span></div>
          <button type="button" aria-label="Dismiss generated token" onclick={() => { createdCredential = null; }}>×</button>
        </header>
        <div class="credential-fields">
          <div><span>Username</span><code>{createdCredential.username}</code><button type="button" onclick={() => copyText(createdCredential.username, 'Username')}><Icon name="copy" size={14} />Copy</button></div>
          <div><span>Token</span><code>{createdCredential.secret}</code><button type="button" onclick={() => copyText(createdCredential.secret, 'Token')}><Icon name="copy" size={14} />Copy</button></div>
          <div class="login-command"><span>Login command</span><code>{loginCommand}</code><button type="button" onclick={() => copyText(loginCommand, 'Login command')}><Icon name="copy" size={14} />Copy</button></div>
        </div>
        <p>Run the login command, then paste the token when Docker asks for the password.</p>
      </div>
    {/if}

    {#if tokensLoading}
      <div class="token-empty"><span class="spinner"></span><p>Loading access tokens…</p></div>
    {:else if accessTokens.length === 0}
      <div class="token-empty"><Icon name="lock" size={22} /><div><strong>No access tokens</strong><span>Generate one to authenticate Docker, CI, or another registry client.</span></div></div>
    {:else}
      <div class="token-list">
        <div class="token-list-header"><span>Name</span><span>Permission</span><span>Last used</span><span>Created</span><span></span></div>
        {#each accessTokens as token}
          <article>
            <div class="token-identity"><span class="token-key"><Icon name="key" size={15} /></span><div><strong>{token.name}</strong><code>{token.prefix}…</code></div></div>
            <span class:write={token.permission === 'read_write'} class="permission-badge">{token.permission === 'read_write' ? 'Read & write' : 'Read only'}</span>
            <time>{formatDate(token.lastUsedAt)}</time>
            <time>{formatDate(token.createdAt)}</time>
            <button type="button" class="revoke-button" aria-label={`Revoke ${token.name}`} onclick={() => { revokeTarget = token; revokeError = ''; }}><Icon name="trash" size={14} />Revoke</button>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="panel repositories-panel" aria-label="Registry repositories">
    <header class="panel-header">
      <div><span class="eyebrow">Images</span><h2>Repositories</h2></div>
      <div class="repo-filter"><Icon name="search" size={14} /><input bind:value={filter} placeholder="Filter repositories or tags" /></div>
    </header>
    {#if repositoriesError}
      <div class="repository-error"><Icon name="x-circle" size={16} /><div><strong>Repositories unavailable</strong><span>{repositoriesError}</span></div><button type="button" onclick={loadRepositories}>Retry</button></div>
    {:else if repositoriesLoading}
      <div class="loading-state"><span class="spinner"></span><p>Loading registry catalog…</p></div>
    {:else if repositories.length === 0}
      <EmptyState icon="layers" title="No images pushed" description="Push an image to a configured registry host and repositories will appear here." />
    {:else if visibleRepositories.length === 0}
      <EmptyState icon="search" title="No matching images" description="Change the filter to inspect more repositories or tags." />
    {:else}
      <div class="repository-list">
        {#each visibleRepositories as repository}
          <article>
            <header><Icon name="box" size={17} /><strong>{repository.name}</strong><span>{(repository.tags || []).length} tag{(repository.tags || []).length === 1 ? '' : 's'}</span></header>
            <div class="tag-list">
              {#if (repository.tags || []).length === 0}
                <span class="tag-empty">No tags</span>
              {:else}
                {#each repository.tags as tag}
                  <span class="tag"><code>{tag}</code><button type="button" aria-label={`Delete ${repository.name}:${tag}`} onclick={() => { deleteTarget = { name: repository.name, tag }; deleteError = ''; }}><Icon name="trash" size={13} /></button></span>
                {/each}
              {/if}
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</Shell>

{#if deleteTarget}
  <ConfirmDialog
    title="Delete registry tag?"
    message={`Delete ${deleteTarget.name}:${deleteTarget.tag}. Layers remain on disk until garbage collection removes unreferenced data.`}
    confirmLabel="Delete tag"
    requireText={deleteTarget.tag}
    busy={deleting}
    error={deleteError}
    onConfirm={confirmDelete}
    onClose={() => { if (!deleting) deleteTarget = null; }}
  />
{/if}

{#if revokeTarget}
  <ConfirmDialog
    title="Revoke registry token?"
    message={`Revoke ${revokeTarget.name}. Any Docker client or CI runner using it will immediately lose registry access.`}
    confirmLabel="Revoke token"
    requireText={revokeTarget.name}
    busy={revoking}
    error={revokeError}
    onConfirm={revokeAccessToken}
    onClose={() => { if (!revoking) revokeTarget = null; }}
  />
{/if}

<style>
  .registry-status { min-height: 42px; padding: 0 var(--space-3); display: inline-flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); }
  .registry-status i { width: 9px; height: 9px; border-radius: 50%; background: var(--color-warning); }
  .registry-status.online i { background: var(--color-success); }
  .registry-status span { display: grid; gap: 1px; }
  .registry-status strong { font-size: var(--text-sm); }
  .registry-status small { color: var(--color-muted); font-size: var(--text-xs); }
  .registry-grid { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(320px, 0.75fr); gap: var(--space-5); }
  .panel-header b { padding: 4px 8px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .settings-form { padding: var(--space-5); display: grid; gap: var(--space-5); }
  .storage-picker { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-3); }
  .storage-picker label { min-height: 76px; padding: var(--space-3); display: grid; grid-template-columns: 28px minmax(0, 1fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); cursor: pointer; }
  .storage-picker label.active { border-color: var(--color-accent); background: var(--color-accent-softer); }
  .storage-picker input { position: absolute; opacity: 0; pointer-events: none; }
  .storage-picker span, .toggle-row div { display: grid; gap: 3px; }
  .storage-picker strong, .toggle-row strong { font-size: var(--text-sm); }
  .storage-picker small, .toggle-row small, .filesystem-note span { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4); }
  .form-grid label { display: grid; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .form-grid label em { color: var(--color-faint); font-style: normal; font-weight: 500; }
  .form-grid input, .repo-filter input { width: 100%; min-width: 0; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); outline: 0; }
  .form-grid input { height: 38px; padding: 0 var(--space-3); }
  .form-grid input:focus, .repo-filter input:focus { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 16%, transparent); }
  .wide { grid-column: 1 / -1; }
  .toggle-row { min-height: 54px; padding: var(--space-3); display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .switch { display: grid; grid-template-columns: 36px 28px; align-items: center; gap: 7px; cursor: pointer; }
  .switch input { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }
  .switch > span { width: 36px; height: 20px; position: relative; border: 1px solid var(--color-rule-strong); border-radius: 999px; background: var(--color-paper-subtle); }
  .switch > span::after { content: ''; width: 14px; height: 14px; position: absolute; top: 2px; left: 2px; border-radius: 50%; background: var(--color-muted); transition: transform 0.16s ease; }
  .switch input:checked + span { border-color: var(--color-accent); background: var(--color-accent); }
  .switch input:checked + span::after { transform: translateX(16px); background: var(--color-accent-ink); }
  .switch em { color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 700; }
  .filesystem-note { min-height: 72px; padding: var(--space-4); display: flex; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .filesystem-note div { display: grid; gap: 3px; }
  .settings-form footer { display: flex; justify-content: flex-end; }
  button { font: inherit; }
  .primary, .gc-actions button, .repository-error button { min-height: 36px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  button:disabled { opacity: 0.6; cursor: not-allowed; }
  .icon-action { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); cursor: pointer; }
  .summary-metrics { padding: var(--space-5); display: grid; gap: var(--space-3); }
  .summary-metrics div { padding: var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .summary-metrics span { color: var(--color-muted); font-size: var(--text-xs); }
  .summary-metrics strong { font-size: var(--text-md); }
  .gc-actions { padding: 0 var(--space-5) var(--space-5); display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-2); }
  .gc-actions .danger { border-color: var(--color-danger); background: var(--color-danger); color: white; }
  .gc-output { margin: 0 var(--space-5) var(--space-5); overflow: hidden; border: 1px solid var(--color-log-rule); border-radius: var(--radius-md); background: var(--color-log-bg); }
  .gc-output header { min-height: 36px; padding: 0 var(--space-3); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-log-rule); color: var(--color-log-text); }
  .gc-output header span { color: var(--color-log-muted); font-size: var(--text-xs); }
  .gc-output pre { max-height: 280px; margin: 0; padding: var(--space-3); overflow: auto; color: var(--color-log-text); font-size: var(--text-xs); line-height: 1.5; white-space: pre-wrap; }
  .access-panel, .repositories-panel { margin-top: var(--space-5); }
  .access-panel { container-name: registry-access; container-type: inline-size; }
  .token-create-button { min-height: 34px; }
  .access-intro { padding: var(--space-4) var(--space-5); display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: linear-gradient(90deg, color-mix(in srgb, var(--color-accent) 5%, var(--color-paper-raised)), var(--color-paper-raised)); }
  .access-mark { width: 38px; height: 38px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 25%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .access-intro > div:nth-child(2) { display: grid; gap: 3px; }
  .access-intro strong { font-size: var(--text-sm); }
  .access-intro span { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .access-intro > code { padding: 6px 9px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-surface-subtle); color: var(--color-ink); font-size: var(--text-xs); }
  .token-notice { margin: var(--space-4) var(--space-5) 0; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-rule)); border-radius: var(--radius-sm); background: color-mix(in srgb, var(--color-warning) 7%, var(--color-paper-raised)); color: var(--color-muted); font-size: var(--text-xs); }
  .token-notice :global(svg) { color: var(--color-warning); }
  .token-notice code { color: var(--color-ink); }
  .token-form { margin: var(--space-5); padding: var(--space-4); display: grid; grid-template-columns: minmax(190px, .75fr) minmax(0, 1.25fr) auto; align-items: end; gap: var(--space-4); border: 1px solid var(--color-accent); border-radius: var(--radius-md); background: var(--color-accent-softer); box-shadow: 0 8px 24px color-mix(in srgb, var(--color-accent) 8%, transparent); }
  .token-name { display: grid; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .token-name input { width: 100%; height: 40px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); outline: 0; }
  .token-name input:focus { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 16%, transparent); }
  .token-form fieldset { min-width: 0; margin: 0; padding: 0; display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-2); border: 0; }
  .token-form legend { margin-bottom: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .token-form fieldset label { min-height: 40px; padding: 7px var(--space-3); display: grid; grid-template-columns: 20px minmax(0, 1fr); align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); cursor: pointer; }
  .token-form fieldset label.active { border-color: var(--color-accent); }
  .token-form fieldset input { position: absolute; opacity: 0; pointer-events: none; }
  .token-form fieldset label > span { display: grid; }
  .token-form fieldset strong { font-size: var(--text-xs); }
  .token-form fieldset small { color: var(--color-muted); font-size: var(--text-2xs); }
  .token-form footer { display: flex; }
  .token-form-error { grid-column: 1 / -1; display: flex; align-items: center; gap: var(--space-2); color: var(--color-danger); font-size: var(--text-xs); }
  .credential-reveal { margin: var(--space-5); overflow: hidden; border: 1px solid color-mix(in srgb, var(--color-success) 45%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-success) 4%, var(--color-paper-raised)); box-shadow: 0 10px 28px color-mix(in srgb, var(--color-success) 8%, transparent); }
  .credential-reveal > header { min-height: 64px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 36px minmax(0, 1fr) 28px; align-items: center; gap: var(--space-3); border-bottom: 1px solid color-mix(in srgb, var(--color-success) 22%, var(--color-rule)); }
  .reveal-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 50%; background: var(--color-success); color: white; }
  .credential-reveal header > div:nth-child(2) { display: grid; gap: 2px; }
  .credential-reveal header strong { font-size: var(--text-sm); }
  .credential-reveal header span, .credential-reveal > p { color: var(--color-muted); font-size: var(--text-xs); }
  .credential-reveal header > button { width: 28px; height: 28px; border: 0; background: transparent; color: var(--color-muted); font-size: 20px; cursor: pointer; }
  .credential-fields { padding: var(--space-4); display: grid; grid-template-columns: 1fr 1.7fr; gap: var(--space-3); }
  .credential-fields > div { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) auto; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-log-bg); }
  .credential-fields > div > span { grid-column: 1 / -1; padding: 7px var(--space-3) 0; color: var(--color-log-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
  .credential-fields code { min-width: 0; padding: 7px var(--space-3) 10px; overflow: auto; color: var(--color-log-text); font-size: var(--text-xs); white-space: nowrap; }
  .credential-fields button { padding: 0 var(--space-3); border: 0; border-left: 1px solid var(--color-log-rule); background: transparent; color: var(--color-log-muted); display: flex; align-items: center; gap: 5px; font-size: var(--text-xs); cursor: pointer; }
  .credential-fields button:hover { color: var(--color-log-text); }
  .credential-fields .login-command { grid-column: 1 / -1; }
  .credential-reveal > p { margin: 0; padding: 0 var(--space-4) var(--space-4); }
  .token-empty { min-height: 128px; padding: var(--space-5); display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); text-align: left; }
  .token-empty > div { display: grid; gap: 2px; }
  .token-empty strong { color: var(--color-ink); font-size: var(--text-sm); }
  .token-empty span { font-size: var(--text-xs); }
  .token-list { display: grid; }
  .token-list-header, .token-list article { padding: 0 var(--space-5); display: grid; grid-template-columns: minmax(210px, 1.4fr) minmax(110px, .65fr) minmax(140px, .85fr) minmax(140px, .85fr) 92px; align-items: center; gap: var(--space-3); }
  .token-list-header { min-height: 34px; border-top: 1px solid var(--color-rule); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
  .token-list article { min-height: 68px; border-bottom: 1px solid var(--color-rule); }
  .token-list article:last-child { border-bottom: 0; }
  .token-identity { min-width: 0; display: flex; align-items: center; gap: var(--space-3); }
  .token-key { width: 32px; height: 32px; flex: 0 0 auto; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-muted); }
  .token-identity > div { min-width: 0; display: grid; gap: 2px; }
  .token-identity strong { overflow: hidden; font-size: var(--text-sm); text-overflow: ellipsis; white-space: nowrap; }
  .token-identity code { color: var(--color-muted); font-size: var(--text-2xs); }
  .permission-badge { width: max-content; padding: 4px 7px; border: 1px solid var(--color-rule); border-radius: 999px; background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; }
  .permission-badge.write { border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); background: var(--color-accent-softer); color: var(--color-accent); }
  .token-list time { color: var(--color-muted); font-size: var(--text-xs); }
  .revoke-button { min-height: 30px; padding: 0 var(--space-2); display: flex; align-items: center; justify-content: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); font-size: var(--text-xs); cursor: pointer; }
  .revoke-button:hover { border-color: var(--color-danger); color: var(--color-danger); }
  .repo-filter { width: min(320px, 100%); height: 34px; padding: 0 var(--space-2); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); }
  .repo-filter input { height: 30px; padding: 0; border: 0; background: transparent; }
  .repository-list { display: grid; }
  .repository-list article { padding: var(--space-4) var(--space-5); display: grid; gap: var(--space-3); border-top: 1px solid var(--color-rule); }
  .repository-list article:first-child { border-top: 0; }
  .repository-list article > header { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: var(--space-2); }
  .repository-list article strong { min-width: 0; overflow-wrap: anywhere; font-family: var(--font-mono); font-size: var(--text-sm); }
  .repository-list article header span { color: var(--color-muted); font-size: var(--text-xs); }
  .tag-list { display: flex; flex-wrap: wrap; gap: var(--space-2); padding-left: 32px; }
  .tag { min-height: 30px; display: inline-flex; align-items: center; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .tag code { padding: 0 var(--space-2); color: var(--color-ink); font-size: var(--text-xs); }
  .tag button { width: 30px; height: 28px; display: grid; place-items: center; border: 0; border-left: 1px solid var(--color-rule); background: transparent; color: var(--color-muted); cursor: pointer; }
  .tag button:hover { color: var(--color-danger); }
  .tag-empty { color: var(--color-muted); font-size: var(--text-sm); }
  .loading-state, .repository-error { min-height: 170px; padding: var(--space-5); display: grid; place-content: center; justify-items: center; gap: var(--space-3); text-align: center; }
  .repository-error { grid-template-columns: 28px minmax(0, 1fr) auto; justify-items: start; text-align: left; }
  .repository-error span { color: var(--color-muted); font-size: var(--text-sm); }
  .spinner { width: 22px; height: 22px; border: 2px solid var(--color-rule); border-top-color: var(--color-accent); border-radius: 50%; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @container registry-access (max-width: 820px) {
    .token-form { grid-template-columns: minmax(180px, .7fr) minmax(0, 1.3fr); }
    .token-form footer { grid-column: 1 / -1; justify-content: flex-end; }
    .token-form footer .primary { min-width: 180px; }
    .token-list-header { display: none; }
    .token-list article { padding: var(--space-4) var(--space-5); grid-template-columns: minmax(0, 1fr) auto; }
    .token-list time { display: none; }
  }
  @container registry-access (max-width: 620px) {
    .access-intro { grid-template-columns: 38px minmax(0, 1fr); }
    .access-intro > code { grid-column: 2; width: max-content; max-width: 100%; overflow: auto; }
    .token-form { grid-template-columns: 1fr; align-items: stretch; }
    .token-form footer, .token-form-error { grid-column: 1; }
    .token-form footer .primary { width: 100%; }
  }
  @container registry-access (max-width: 440px) {
    .token-form { margin: var(--space-3); padding: var(--space-3); }
    .token-form fieldset, .token-list article { grid-template-columns: 1fr; }
    .revoke-button { width: 100%; }
  }
  @media (max-width: 860px) { .registry-grid { grid-template-columns: 1fr; } .form-grid, .storage-picker, .gc-actions, .credential-fields { grid-template-columns: 1fr; } .credential-fields .login-command { grid-column: auto; } .settings-form footer, .primary { width: 100%; } .panel-header { align-items: flex-start; flex-direction: column; } .access-intro { grid-template-columns: 38px minmax(0, 1fr); } .access-intro > code { grid-column: 2; width: max-content; max-width: 100%; overflow: auto; } .repo-filter { width: 100%; } .repository-error { grid-template-columns: 1fr; justify-items: center; text-align: center; } .tag-list { padding-left: 0; } }
</style>
