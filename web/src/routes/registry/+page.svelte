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
    objectStorageId: '',
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
  let registryDomain = { domain: '', httpsEnabled: true, attached: false };
  let domainDraft = '';
  let domainSaving = false;
  let domainError = '';
  let detachDomainOpen = false;
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
  let objectStorages = [];
  let filter = '';
  let expandedRepositories = [];
  let gcResult = null;
  let deleteTarget = null;
  let deleteError = '';
  let relativeNow = Date.now();

  onMount(() => {
    load();
    const relativeTimeTimer = window.setInterval(() => {
      relativeNow = Date.now();
    }, 60_000);
    return () => window.clearInterval(relativeTimeTimer);
  });

  $: visibleRepositories = repositories.filter((item) => {
    if ((item.images || []).length === 0) return false;
    const query = filter.trim().toLowerCase();
    return item.name.toLowerCase().includes(query)
      || (item.tags || []).some((tag) => tag.toLowerCase().includes(query))
      || (item.images || []).some((image) => (image.digest || '').toLowerCase().includes(query));
  });
  $: tagCount = repositories.reduce((count, item) => count + (item.tags || []).length, 0);
  $: selectedObjectStorage = objectStorages.find((item) => item.id === settings.objectStorageId);
  $: storageLabel = settings.storage === 's3' ? (selectedObjectStorage?.name || 'Object storage') : 'Docker volume filesystem';
  $: registryHost = registryHosts[0] || 'registry.example.com';
  $: loginCommand = `docker login ${registryHost} --username ${createdCredential?.username || registryUsername || 'you@example.com'}`;

  async function load() {
    loading = true;
    error = '';
    warning = '';
    try {
      const [statusResponse, settingsResponse, tokensResponse, domainResponse, objectStorageResponse] = await Promise.all([
        api('/api/registry/status'),
        api('/api/registry/settings'),
        api('/api/registry/access-tokens'),
        api('/api/registry/domain'),
        api('/api/object-storage')
      ]);
      const statusPayload = await statusResponse.json();
      const settingsPayload = await settingsResponse.json();
      const tokensPayload = await tokensResponse.json();
      const domainPayload = await domainResponse.json();
      const objectStoragePayload = await objectStorageResponse.json();
      if (!statusResponse.ok) throw new Error(statusPayload.error || 'Could not load registry status');
      if (!settingsResponse.ok) throw new Error(settingsPayload.error || 'Could not load registry settings');
      if (!tokensResponse.ok) throw new Error(tokensPayload.error || 'Could not load registry tokens');
      if (!domainResponse.ok) throw new Error(domainPayload.error || 'Could not load registry domain');
      if (!objectStorageResponse.ok) throw new Error(objectStoragePayload.error || 'Could not load object storage');
      status = statusPayload;
      settings = { ...emptySettings(), ...settingsPayload, s3SecretKey: '' };
      accessTokens = tokensPayload.tokens || [];
      registryUsername = tokensPayload.username || '';
      registryDomain = { domain: '', httpsEnabled: true, attached: false, ...domainPayload };
      domainDraft = registryDomain.domain || '';
      registryHosts = domainPayload.registryHosts || tokensPayload.registryHosts || [];
      objectStorages = objectStoragePayload.connections || [];
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
      expandedRepositories = [
        ...expandedRepositories,
        ...repositories.map((repository) => repository.name).filter((name) => !expandedRepositories.includes(name))
      ];
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

  function formatRelativeDate(value) {
    if (!value) return 'Not recorded';
    const timestamp = new Date(value).getTime();
    if (!Number.isFinite(timestamp)) return 'Not recorded';
    const elapsed = timestamp - relativeNow;
    const absolute = Math.abs(elapsed);
    if (absolute < 60_000) return 'just now';
    const units = [
      ['year', 365 * 24 * 60 * 60 * 1000],
      ['month', 30 * 24 * 60 * 60 * 1000],
      ['day', 24 * 60 * 60 * 1000],
      ['hour', 60 * 60 * 1000],
      ['minute', 60 * 1000]
    ];
    const [unit, duration] = units.find(([, size]) => absolute >= size) || units[units.length - 1];
    return new Intl.RelativeTimeFormat(undefined, { numeric: 'always' }).format(Math.round(elapsed / duration), unit);
  }

  function imagesFor(repository) {
    if ((repository.images || []).length > 0) return repository.images;
    return (repository.tags || []).map((tag) => ({ digest: '', tags: [tag], size: 0 }));
  }

  function preferredTag(repository) {
    if ((repository.tags || []).includes('latest')) return 'latest';
    return (repository.tags || [])[0] || '';
  }

  function formatBytes(value) {
    if (!value || value < 1) return 'Unknown';
    const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
    const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    const amount = value / (1024 ** exponent);
    return `${amount >= 100 || exponent === 0 ? amount.toFixed(0) : amount.toFixed(2)} ${units[exponent]}`;
  }

  function shortDigest(value) {
    if (!value) return 'Digest unavailable';
    if (value.length <= 28) return value;
    return `${value.slice(0, 20)}…${value.slice(-8)}`;
  }

  function repositoryReference(name, tag) {
    return `${registryHost}/${name}:${tag}`;
  }

  function isRepositoryExpanded(name) {
    return expandedRepositories.includes(name);
  }

  function toggleRepository(name) {
    expandedRepositories = isRepositoryExpanded(name)
      ? expandedRepositories.filter((repositoryName) => repositoryName !== name)
      : [...expandedRepositories, name];
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
          objectStorageId: settings.objectStorageId,
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

  async function persistRegistryDomain(domain, httpsEnabled) {
    domainSaving = true;
    domainError = '';
    try {
      const response = await api('/api/registry/domain', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain, httpsEnabled })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not configure registry domain');
      registryDomain = { domain: '', httpsEnabled: true, attached: false, ...payload };
      domainDraft = registryDomain.domain || '';
      registryHosts = payload.registryHosts || registryHosts;
      createdCredential = null;
      toast.success(domain ? 'Registry domain attached' : 'Registry domain detached');
    } catch (cause) {
      domainError = cause instanceof Error ? cause.message : 'Could not configure registry domain';
      throw cause;
    } finally {
      domainSaving = false;
    }
  }

  async function saveRegistryDomain() {
    try {
      await persistRegistryDomain(domainDraft.trim(), registryDomain.httpsEnabled);
    } catch {
      // The inline form presents the API error.
    }
  }

  async function detachRegistryDomain() {
    try {
      await persistRegistryDomain('', true);
      detachDomainOpen = false;
    } catch {
      // Keep the confirmation open so the error remains visible.
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
      if (!response.ok) throw new Error(payload.error || 'Could not delete image');
      deleteTarget = null;
      toast.success('Registry image deleted');
      await loadRepositories();
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not delete image';
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
          <label class:active={settings.storage === 's3'}><input type="radio" bind:group={settings.storage} value="s3" /><Icon name="cloud" size={16} /><span><strong>Object storage</strong><small>Use a saved S3-compatible bucket.</small></span></label>
        </div>

        {#if settings.storage === 's3'}
          {#if objectStorages.length}
            <div class="object-storage-selection">
              <label>
                <span>Object storage connection</span>
                <select bind:value={settings.objectStorageId} required>
                  <option value="" disabled>Select a bucket…</option>
                  {#each objectStorages as storage}
                    <option value={storage.id}>{storage.name} · {storage.bucket}</option>
                  {/each}
                </select>
                <small>Connections are managed independently and their secret keys stay encrypted.</small>
              </label>
              {#if selectedObjectStorage}
                <div class="selected-storage">
                  <span class="selected-storage-mark"><Icon name="cloud" size={18} /></span>
                  <div>
                    <strong>{selectedObjectStorage.name}</strong>
                    <span><code>{selectedObjectStorage.bucket}</code> · {selectedObjectStorage.region}</span>
                    <small>{selectedObjectStorage.endpoint || 'Amazon S3 managed endpoint'}</small>
                  </div>
                  <a class="btn btn-sm" href="/object-storage">Manage</a>
                </div>
              {/if}
            </div>
          {:else}
            <div class="no-object-storage">
              <span><Icon name="cloud" size={20} /></span>
              <div><strong>No object storage connected</strong><small>Add an S3-compatible bucket first, then return here to select it.</small></div>
              <a class="btn btn-primary btn-sm" href="/object-storage"><Icon name="plus" size={13} /> Add object storage</a>
            </div>
          {/if}
        {:else}
          <div class="filesystem-note"><Icon name="hard-drive" size={18} /><div><strong>Registry data stays in the Compose volume.</strong><span>Changing from S3 back to filesystem does not migrate objects automatically.</span></div></div>
        {/if}

        <footer>
          <button type="submit" class="primary" disabled={saving || loading || (settings.storage === 's3' && !objectStorages.length)}><Icon name="check" size={14} />{saving ? 'Saving…' : 'Save and restart registry'}</button>
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

  <section class="panel domain-panel" aria-label="Registry domain">
    <header class="panel-header">
      <div><span class="eyebrow">Network endpoint</span><h2>Registry domain</h2></div>
      <span class:active={registryDomain.attached} class="domain-status"><i></i>{registryDomain.attached ? 'Attached' : 'Not attached'}</span>
    </header>
    <div class="domain-layout">
      <div class="domain-intro">
        <span class="domain-mark"><Icon name="globe" size={22} /></span>
        <div>
          <strong>{registryDomain.attached ? registryDomain.domain : 'Give the registry a public address'}</strong>
          <span>Dokyr will route this hostname to Docker Distribution and use it in every generated login and image reference.</span>
        </div>
        {#if registryDomain.attached}
          <code>{registryDomain.httpsEnabled ? 'https://' : 'http://'}{registryDomain.domain}</code>
        {/if}
      </div>
      <form class="domain-form" onsubmit={(event) => { event.preventDefault(); saveRegistryDomain(); }}>
        <label class="domain-name">
          <span>Domain name</span>
          <div><Icon name="globe" size={15} /><input bind:value={domainDraft} autocomplete="off" placeholder="registry.example.com" required /></div>
          <small>Enter only the hostname. Do not include <code>https://</code> or a path.</small>
        </label>
        <div class="domain-https">
          <label class="switch"><input type="checkbox" bind:checked={registryDomain.httpsEnabled} /><span></span><em>{registryDomain.httpsEnabled ? 'TLS' : 'HTTP'}</em></label>
          <div><strong>Automatic HTTPS</strong><small>Caddy obtains and renews the certificate after DNS points to this server.</small></div>
        </div>
        {#if domainError}<div class="domain-error"><Icon name="x-circle" size={14} />{domainError}</div>{/if}
        <footer>
          {#if registryDomain.attached}
            <button type="button" class="detach-button" disabled={domainSaving} onclick={() => { detachDomainOpen = true; domainError = ''; }}>Detach</button>
          {/if}
          <button type="submit" class="primary" disabled={domainSaving || !domainDraft.trim()}><Icon name="check" size={14} />{domainSaving ? 'Applying…' : registryDomain.attached ? 'Update domain' : 'Attach domain'}</button>
        </footer>
      </form>
      <aside class="dns-note">
        <Icon name="info" size={16} />
        <div><strong>DNS comes first</strong><span>Create an A or AAAA record for this hostname pointing to the Dokyr server, and allow inbound ports 80 and 443.</span></div>
      </aside>
    </div>
  </section>

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
      <div class="token-notice"><Icon name="alert" size={15} /><span>Attach a registry domain above before using these credentials outside Dokyr.</span></div>
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
          {@const images = imagesFor(repository)}
          {@const defaultTag = preferredTag(repository)}
          {@const expanded = expandedRepositories.includes(repository.name)}
          <article class="repository-card">
            <header class="repository-summary">
              <button
                type="button"
                class="disclosure"
                aria-label={`${expanded ? 'Collapse' : 'Expand'} ${repository.name}`}
                aria-expanded={expanded}
                onclick={() => toggleRepository(repository.name)}
              >
                <Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={15} />
              </button>
              <span class="repository-mark"><Icon name="box" size={18} /></span>
              <div class="repository-identity">
                <strong>{repository.name}</strong>
                <span>{images.length} image{images.length === 1 ? '' : 's'} · {(repository.tags || []).length} tag{(repository.tags || []).length === 1 ? '' : 's'}</span>
              </div>
              {#if defaultTag}
                <div class="default-tag">
                  <span>Quick pull</span>
                  <code><Icon name="tag" size={13} />{defaultTag}</code>
                  <button type="button" aria-label={`Copy ${repository.name}:${defaultTag}`} onclick={() => copyText(repositoryReference(repository.name, defaultTag), 'Image reference')}><Icon name="copy" size={14} /></button>
                </div>
              {/if}
            </header>

            {#if expanded}
              <div class="image-table" role="table" aria-label={`${repository.name} images`}>
                <div class="image-table-header" role="row">
                  <span role="columnheader">Digest</span>
                  <span role="columnheader">Tags</span>
                  <span role="columnheader">Pushed</span>
                  <span role="columnheader">Content size</span>
                  <span role="columnheader">Action</span>
                </div>
                {#if images.length === 0}
                  <div class="image-empty">This repository does not have any tagged images.</div>
                {:else}
                  {#each images as image}
                    <div class="image-row" role="row">
                      <div class="digest-cell" role="cell" data-label="Digest">
                        <span class="image-kind"><Icon name="layers" size={16} /></span>
                        <code title={image.digest || 'Digest unavailable'}>{shortDigest(image.digest)}</code>
                        {#if image.digest}
                          <button type="button" aria-label="Copy manifest digest" onclick={() => copyText(image.digest, 'Manifest digest')}><Icon name="copy" size={14} /></button>
                        {/if}
                      </div>
                      <div class="image-tags" role="cell" data-label="Tags">
                        {#each image.tags || [] as tag}
                          <span class="image-tag">
                            <Icon name="tag" size={13} />
                            <code>{tag}</code>
                            <button type="button" aria-label={`Copy ${repository.name}:${tag}`} onclick={() => copyText(repositoryReference(repository.name, tag), 'Image reference')}><Icon name="copy" size={13} /></button>
                          </span>
                        {/each}
                      </div>
                      <div
                        class:unknown={!image.pushedAt}
                        class="image-pushed"
                        role="cell"
                        data-label="Pushed"
                        title={image.pushedAt ? formatDate(image.pushedAt) : 'Push history starts after this Dokyr update'}
                      >
                        <time datetime={image.pushedAt || undefined}>{formatRelativeDate(image.pushedAt)}</time>
                      </div>
                      <span class:unknown={!image.size} class="image-size" role="cell" data-label="Content size">{formatBytes(image.size)}</span>
                      <div class="image-actions" role="cell" data-label="Action">
                        <button
                          type="button"
                          aria-label={`Delete image ${repository.name}:${(image.tags || [])[0] || ''}`}
                          disabled={(image.tags || []).length === 0}
                          onclick={() => {
                            deleteTarget = { name: repository.name, tag: image.tags[0], tags: image.tags, digest: image.digest };
                            deleteError = '';
                          }}
                        ><Icon name="trash" size={14} /><span>Delete</span></button>
                      </div>
                    </div>
                  {/each}
                {/if}
              </div>
            {/if}
          </article>
        {/each}
      </div>
    {/if}
  </section>
</Shell>

{#if deleteTarget}
  <ConfirmDialog
    title="Delete registry image?"
    message={`Delete the manifest for ${deleteTarget.name}. ${deleteTarget.tags.length === 1 ? `Tag ${deleteTarget.tags[0]} points` : `Tags ${deleteTarget.tags.join(', ')} point`} to this manifest and ${deleteTarget.tags.length === 1 ? 'will' : 'will all'} be removed. Layers remain until garbage collection removes unreferenced data.`}
    confirmLabel="Delete image"
    requireText={deleteTarget.tag}
    busy={deleting}
    error={deleteError}
    onConfirm={confirmDelete}
    onClose={() => { if (!deleting) deleteTarget = null; }}
  />
{/if}

{#if detachDomainOpen}
  <ConfirmDialog
    title="Detach registry domain?"
    message={`Detach ${registryDomain.domain}. Docker clients using this hostname will lose access until another registry domain is attached.`}
    confirmLabel="Detach domain"
    requireText={registryDomain.domain}
    busy={domainSaving}
    error={domainError}
    onConfirm={detachRegistryDomain}
    onClose={() => { if (!domainSaving) detachDomainOpen = false; }}
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
  .storage-picker span { display: grid; gap: 3px; }
  .storage-picker strong { font-size: var(--text-sm); }
  .storage-picker small, .filesystem-note span { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .object-storage-selection { display: grid; gap: var(--space-3); }
  .object-storage-selection > label { display: grid; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .object-storage-selection select { width: 100%; height: 40px; padding: 0 34px 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background-color: var(--color-paper-raised); background-image: linear-gradient(45deg, transparent 50%, var(--color-muted) 50%), linear-gradient(135deg, var(--color-muted) 50%, transparent 50%); background-position: calc(100% - 16px) 50%, calc(100% - 11px) 50%; background-repeat: no-repeat; background-size: 5px 5px; color: var(--color-ink); font-size: var(--text-sm); appearance: none; outline: 0; }
  .object-storage-selection select:focus { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 16%, transparent); }
  .object-storage-selection > label small { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 500; }
  .selected-storage { min-height: 82px; padding: var(--space-3); display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--color-rule)); border-radius: var(--radius-md); background: linear-gradient(120deg, var(--color-accent-softer), var(--color-paper-raised)); }
  .selected-storage-mark { width: 42px; height: 42px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-accent); }
  .selected-storage > div { min-width: 0; display: grid; gap: 2px; }
  .selected-storage strong { font-size: var(--text-sm); }
  .selected-storage span, .selected-storage small { min-width: 0; overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .selected-storage code { color: var(--color-ink-secondary); font-size: var(--text-xs); }
  .no-object-storage { min-height: 86px; padding: var(--space-4); display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .no-object-storage > span { width: 42px; height: 42px; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-paper-subtle); color: var(--color-muted); }
  .no-object-storage > div { display: grid; gap: 3px; }
  .no-object-storage strong { font-size: var(--text-sm); }
  .no-object-storage small { color: var(--color-muted); font-size: var(--text-xs); }
  .repo-filter input { width: 100%; min-width: 0; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-sm); outline: 0; }
  .repo-filter input:focus { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 16%, transparent); }
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
  .domain-panel, .access-panel, .repositories-panel { margin-top: var(--space-5); }
  .domain-panel { container-name: registry-domain; container-type: inline-size; }
  .domain-status { min-height: 28px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 7px; border: 1px solid var(--color-rule); border-radius: 999px; background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; }
  .domain-status i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-muted); }
  .domain-status.active { border-color: color-mix(in srgb, var(--color-success) 30%, var(--color-rule)); color: var(--color-success); }
  .domain-status.active i { background: var(--color-success); }
  .domain-layout { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(420px, 1.2fr); }
  .domain-intro { padding: var(--space-5); display: grid; grid-template-columns: 44px minmax(0, 1fr); align-content: center; gap: var(--space-3); border-right: 1px solid var(--color-rule); background: linear-gradient(135deg, color-mix(in srgb, var(--color-accent) 7%, var(--color-paper-raised)), var(--color-paper-raised) 65%); }
  .domain-mark { width: 42px; height: 42px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .domain-intro > div { min-width: 0; display: grid; gap: 4px; }
  .domain-intro strong { overflow-wrap: anywhere; font-size: var(--text-sm); }
  .domain-intro span { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .domain-intro > code { grid-column: 2; width: max-content; max-width: 100%; padding: 6px 9px; overflow: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); white-space: nowrap; }
  .domain-form { padding: var(--space-5); display: grid; grid-template-columns: minmax(0, 1fr) minmax(230px, .7fr); align-items: end; gap: var(--space-4); }
  .domain-name { min-width: 0; display: grid; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; }
  .domain-name > div { height: 40px; padding: 0 var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); }
  .domain-name > div:focus-within { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 16%, transparent); }
  .domain-name input { width: 100%; min-width: 0; height: 36px; border: 0; outline: 0; background: transparent; color: var(--color-ink); font-size: var(--text-sm); }
  .domain-name small, .domain-https small, .dns-note span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 500; line-height: 1.45; }
  .domain-name small code { color: var(--color-ink); }
  .domain-https { min-height: 66px; padding: var(--space-3); display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .domain-https > div { display: grid; gap: 3px; }
  .domain-https strong, .dns-note strong { font-size: var(--text-xs); }
  .domain-error { grid-column: 1 / -1; display: flex; align-items: center; gap: var(--space-2); color: var(--color-danger); font-size: var(--text-xs); }
  .domain-form footer { grid-column: 1 / -1; display: flex; justify-content: flex-end; gap: var(--space-2); }
  .detach-button { min-height: 36px; padding: 0 var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); font-size: var(--text-sm); font-weight: 600; cursor: pointer; }
  .detach-button:hover { border-color: var(--color-danger); color: var(--color-danger); }
  .dns-note { grid-column: 1 / -1; min-height: 50px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-accent); }
  .dns-note > div { display: grid; gap: 2px; }
  .access-panel { container-name: registry-access; container-type: inline-size; }
  .repositories-panel { container-name: registry-repositories; container-type: inline-size; }
  .token-create-button { min-height: 34px; }
  .access-intro { padding: var(--space-4) var(--space-5); display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); background: linear-gradient(90deg, color-mix(in srgb, var(--color-accent) 5%, var(--color-paper-raised)), var(--color-paper-raised)); }
  .access-mark { width: 38px; height: 38px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 25%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .access-intro > div:nth-child(2) { display: grid; gap: 3px; }
  .access-intro strong { font-size: var(--text-sm); }
  .access-intro span { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .access-intro > code { padding: 6px 9px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-surface-subtle); color: var(--color-ink); font-size: var(--text-xs); }
  .token-notice { margin: var(--space-4) var(--space-5) 0; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-rule)); border-radius: var(--radius-sm); background: color-mix(in srgb, var(--color-warning) 7%, var(--color-paper-raised)); color: var(--color-muted); font-size: var(--text-xs); }
  .token-notice :global(svg) { color: var(--color-warning); }
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
  .repository-card { display: grid; border-top: 1px solid var(--color-rule); }
  .repository-card:first-child { border-top: 0; }
  .repository-summary { min-height: 78px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 30px 38px minmax(180px, 1fr) minmax(220px, .75fr); align-items: center; gap: var(--space-3); background: linear-gradient(90deg, color-mix(in srgb, var(--color-accent) 3%, var(--color-paper-raised)), var(--color-paper-raised) 42%); }
  .disclosure { width: 28px; height: 28px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-paper-raised); color: var(--color-muted); cursor: pointer; }
  .disclosure:hover { border-color: var(--color-accent); color: var(--color-accent); }
  .repository-mark { width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 22%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); }
  .repository-identity { min-width: 0; display: grid; gap: 3px; }
  .repository-identity strong { overflow: hidden; color: var(--color-ink); font-family: var(--font-mono); font-size: var(--text-sm); text-overflow: ellipsis; white-space: nowrap; }
  .repository-identity span { color: var(--color-muted); font-size: var(--text-xs); }
  .default-tag { min-width: 0; justify-self: end; display: grid; grid-template-columns: auto minmax(0, auto) 30px; align-items: center; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
  .default-tag > span { padding: 0 var(--space-2); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
  .default-tag code { min-width: 0; height: 30px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 6px; border-left: 1px solid var(--color-rule); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); }
  .default-tag code :global(svg), .image-tag > :global(svg) { color: var(--color-accent); }
  .default-tag button, .digest-cell button, .image-tag button { width: 30px; height: 30px; display: grid; place-items: center; border: 0; border-left: 1px solid var(--color-rule); background: transparent; color: var(--color-muted); cursor: pointer; }
  .default-tag button:hover, .digest-cell button:hover, .image-tag button:hover { color: var(--color-accent); }
  .image-table { margin: 0 var(--space-5) var(--space-5); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); }
  .image-table-header, .image-row { display: grid; grid-template-columns: minmax(225px, 1.25fr) minmax(160px, .9fr) minmax(105px, .48fr) minmax(95px, .42fr) 86px; align-items: center; gap: var(--space-3); }
  .image-table-header { min-height: 34px; padding: 0 var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
  .image-row { min-height: 62px; padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .image-row:last-child { border-bottom: 0; }
  .digest-cell { min-width: 0; display: grid; grid-template-columns: 28px minmax(0, 1fr) 30px; align-items: center; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-surface-subtle); }
  .image-kind { width: 28px; height: 30px; display: grid; place-items: center; color: var(--color-accent); }
  .digest-cell code { min-width: 0; padding: 0 var(--space-2); overflow: hidden; color: var(--color-ink); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .image-tags { min-width: 0; display: flex; flex-wrap: wrap; gap: var(--space-2); }
  .image-tag { min-width: 0; height: 32px; display: grid; grid-template-columns: 25px minmax(0, auto) 30px; align-items: center; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-surface-subtle); }
  .image-tag > :global(svg) { margin-left: 8px; }
  .image-tag code { min-width: 0; padding: 0 var(--space-2); overflow: hidden; color: var(--color-ink); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .image-pushed { color: var(--color-ink); font-size: var(--text-xs); white-space: nowrap; }
  .image-pushed.unknown { color: var(--color-muted); }
  .image-size { color: var(--color-ink); font-family: var(--font-mono); font-size: var(--text-xs); }
  .image-size.unknown { color: var(--color-muted); font-family: inherit; }
  .image-actions { display: flex; justify-content: flex-end; }
  .image-actions button { min-height: 30px; padding: 0 var(--space-2); display: inline-flex; align-items: center; justify-content: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); background: var(--color-paper-raised); color: var(--color-muted); font-size: var(--text-xs); cursor: pointer; }
  .image-actions button:hover { border-color: color-mix(in srgb, var(--color-danger) 55%, var(--color-rule)); color: var(--color-danger); }
  .image-empty { min-height: 70px; padding: var(--space-4); display: grid; place-items: center; color: var(--color-muted); font-size: var(--text-sm); text-align: center; }
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
  @container registry-domain (max-width: 820px) {
    .domain-layout, .domain-form { grid-template-columns: 1fr; }
    .domain-intro { border-right: 0; border-bottom: 1px solid var(--color-rule); }
    .domain-form footer, .domain-error { grid-column: 1; }
  }
  @container registry-domain (max-width: 460px) {
    .domain-intro, .domain-form { padding: var(--space-4); }
    .domain-form footer { display: grid; grid-template-columns: 1fr; }
    .domain-form footer button { width: 100%; }
    .dns-note { align-items: flex-start; padding: var(--space-3) var(--space-4); }
  }
  @container registry-repositories (max-width: 760px) {
    .repository-summary { grid-template-columns: 30px 38px minmax(0, 1fr); }
    .default-tag { grid-column: 3; justify-self: start; }
    .image-table-header { display: none; }
    .image-table { border: 0; border-radius: 0; }
    .image-row { margin-top: var(--space-3); padding: var(--space-3); grid-template-columns: minmax(0, 1fr) auto; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); }
    .image-row:first-child { margin-top: 0; }
    .digest-cell, .image-tags { grid-column: 1 / -1; }
    .image-pushed::before { content: 'Pushed · '; color: var(--color-muted); }
    .image-size::before { content: 'Content size · '; color: var(--color-muted); font-family: inherit; }
  }
  @container registry-repositories (max-width: 460px) {
    .repository-summary { padding: var(--space-3); grid-template-columns: 28px 34px minmax(0, 1fr); gap: var(--space-2); }
    .repository-mark { width: 32px; height: 32px; }
    .default-tag { grid-column: 1 / -1; width: 100%; justify-self: stretch; grid-template-columns: auto minmax(0, 1fr) 30px; }
    .default-tag code { justify-content: flex-start; }
    .image-table { margin: 0 var(--space-3) var(--space-3); }
    .image-row { grid-template-columns: 1fr; }
    .digest-cell, .image-tags { grid-column: 1; }
    .image-pushed, .image-size { display: flex; justify-content: space-between; }
    .image-actions { justify-content: stretch; }
    .image-actions button { width: 100%; }
  }
  @media (max-width: 860px) { .registry-grid { grid-template-columns: 1fr; } .storage-picker, .gc-actions, .credential-fields { grid-template-columns: 1fr; } .credential-fields .login-command { grid-column: auto; } .settings-form footer, .primary { width: 100%; } .panel-header { align-items: flex-start; flex-direction: column; } .access-intro { grid-template-columns: 38px minmax(0, 1fr); } .access-intro > code { grid-column: 2; width: max-content; max-width: 100%; overflow: auto; } .repo-filter { width: 100%; } .repository-error { grid-template-columns: 1fr; justify-items: center; text-align: center; } .no-object-storage, .selected-storage { grid-template-columns: 42px minmax(0, 1fr); } .no-object-storage .btn, .selected-storage .btn { grid-column: 1 / -1; width: 100%; } }
</style>
