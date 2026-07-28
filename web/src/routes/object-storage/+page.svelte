<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  const providers = [
    { id: 'aws', name: 'Amazon S3', mark: 'AWS', hint: 'Native AWS object storage' },
    { id: 'r2', name: 'Cloudflare R2', mark: 'R2', hint: 'S3 without egress fees' },
    { id: 'minio', name: 'MinIO', mark: 'MI', hint: 'Self-hosted object storage' },
    { id: 'digitalocean', name: 'DigitalOcean Spaces', mark: 'DO', hint: 'Managed S3-compatible storage' },
    { id: 'custom', name: 'S3 compatible', mark: 'S3', hint: 'Any compatible provider' }
  ];

  const blankConnection = () => ({
    id: '',
    name: '',
    provider: 'aws',
    region: 'us-east-1',
    bucket: '',
    endpoint: '',
    accessKey: '',
    secretKey: '',
    hasSecretKey: false,
    forcePathStyle: false,
    secure: true
  });

  let connections = [];
  let loading = true;
  let loadError = '';
  let editorOpen = false;
  let saving = false;
  let formError = '';
  let form = blankConnection();
  let removeTarget = null;
  let removing = false;
  let removeError = '';

  $: currentProvider = providers.find((provider) => provider.id === form.provider) || providers[4];
  $: endpointPlaceholder = form.provider === 'r2'
    ? 'https://<account-id>.r2.cloudflarestorage.com'
    : form.provider === 'minio'
      ? 'https://minio.example.com'
      : form.provider === 'digitalocean'
        ? `https://${form.region || 'nyc3'}.digitaloceanspaces.com`
        : 'https://s3.example.com';

  onMount(load);

  async function load() {
    loading = true;
    loadError = '';
    try {
      const response = await api('/api/object-storage');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load object storage');
      connections = payload.connections || [];
    } catch (cause) {
      loadError = cause instanceof Error ? cause.message : 'Could not load object storage';
    } finally {
      loading = false;
    }
  }

  function openCreate() {
    form = blankConnection();
    formError = '';
    editorOpen = true;
  }

  function openEdit(item) {
    form = { ...blankConnection(), ...item, secretKey: '' };
    formError = '';
    editorOpen = true;
  }

  function closeEditor() {
    if (!saving) editorOpen = false;
  }

  function chooseProvider(provider) {
    const next = { ...form, provider };
    if (provider === 'aws') {
      next.region = form.id ? form.region : 'us-east-1';
      next.endpoint = '';
      next.forcePathStyle = false;
      next.secure = true;
    } else if (provider === 'r2') {
      next.region = form.id ? form.region : 'auto';
      next.endpoint = form.id ? form.endpoint : '';
      next.forcePathStyle = true;
      next.secure = true;
    } else if (provider === 'minio') {
      next.region = form.id ? form.region : 'us-east-1';
      next.endpoint = form.id ? form.endpoint : '';
      next.forcePathStyle = true;
      next.secure = true;
    } else if (provider === 'digitalocean') {
      next.region = form.id ? form.region : 'nyc3';
      next.endpoint = form.id ? form.endpoint : 'https://nyc3.digitaloceanspaces.com';
      next.forcePathStyle = false;
      next.secure = true;
    } else {
      next.region = form.id ? form.region : 'us-east-1';
      next.endpoint = form.id ? form.endpoint : '';
      next.forcePathStyle = true;
      next.secure = true;
    }
    form = next;
  }

  async function save() {
    saving = true;
    formError = '';
    try {
      const response = await api(form.id ? `/api/object-storage/${encodeURIComponent(form.id)}` : '/api/object-storage', {
        method: form.id ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: form.name,
          provider: form.provider,
          region: form.region,
          bucket: form.bucket,
          endpoint: form.endpoint,
          accessKey: form.accessKey,
          secretKey: form.secretKey,
          forcePathStyle: form.forcePathStyle,
          secure: form.secure
        })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save object storage');
      toast.success(form.id ? (form.inUse ? 'Storage updated. Save Registry to apply it.' : 'Object storage updated') : 'Object storage connected');
      editorOpen = false;
      await load();
    } catch (cause) {
      formError = cause instanceof Error ? cause.message : 'Could not save object storage';
    } finally {
      saving = false;
    }
  }

  async function removeConnection() {
    if (!removeTarget) return;
    removing = true;
    removeError = '';
    try {
      const response = await api(`/api/object-storage/${encodeURIComponent(removeTarget.id)}`, { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not remove object storage');
      toast.success(`${removeTarget.name} removed`);
      removeTarget = null;
      await load();
    } catch (cause) {
      removeError = cause instanceof Error ? cause.message : 'Could not remove object storage';
    } finally {
      removing = false;
    }
  }

  function providerFor(id) {
    return providers.find((provider) => provider.id === id) || providers[4];
  }

  function endpointLabel(item) {
    if (!item.endpoint) return 'AWS managed endpoint';
    try {
      return new URL(item.endpoint).host;
    } catch {
      return item.endpoint;
    }
  }

  function formatDate(value) {
    if (!value) return 'Not recorded';
    return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(new Date(value));
  }
</script>

<svelte:window onkeydown={(event) => {
  if (event.key === 'Escape' && editorOpen) closeEditor();
}} />

<Shell eyebrow="Infrastructure" title="Object storage" subtitle="Reusable S3-compatible buckets for the Registry and future Dokyr services.">
  <button slot="actions" class="btn btn-primary" type="button" onclick={openCreate}>
    <Icon name="plus" size={14} /> Add object storage
  </button>

  <section class="storage-hero" aria-label="Object storage overview">
    <div class="hero-mark"><Icon name="cloud" size={24} /></div>
    <div>
      <span class="eyebrow">Shared infrastructure</span>
      <h2>Connect a bucket once. Reuse it safely.</h2>
      <p>Credentials are encrypted before they are stored. Dokyr never returns secret keys to the browser after saving.</p>
    </div>
    <dl>
      <div><dt>Connections</dt><dd>{connections.length}</dd></div>
      <div><dt>In use</dt><dd>{connections.filter((item) => item.inUse).length}</dd></div>
    </dl>
  </section>

  {#if loadError}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Object storage unavailable</strong><span>{loadError}</span></div></div>
  {/if}

  {#if loading}
    <div class="loading-grid" aria-label="Loading object storage connections">
      {#each [1, 2, 3] as item}<div class="loading-card" aria-hidden="true"></div>{/each}
    </div>
  {:else if connections.length === 0}
    <section class="panel empty-storage">
      <span class="empty-cloud"><Icon name="cloud" size={28} /></span>
      <div>
        <span class="eyebrow">No connections yet</span>
        <h2>Bring your own bucket</h2>
        <p>Connect Amazon S3, Cloudflare R2, MinIO, DigitalOcean Spaces, or another S3-compatible service.</p>
      </div>
      <button class="btn btn-primary" type="button" onclick={openCreate}><Icon name="plus" size={14} /> Add your first bucket</button>
    </section>
  {:else}
    <div class="section-heading">
      <div><span class="eyebrow">Connections</span><h2>Available buckets</h2></div>
      <p>Select any of these from Registry → Storage backend.</p>
    </div>
    <div class="connection-grid">
      {#each connections as item}
        {@const provider = providerFor(item.provider)}
        <article class="panel connection-card" class:active={item.inUse}>
          <header>
            <span class="provider-mark {item.provider}">{provider.mark}</span>
            <div class="connection-title">
              <h3>{item.name}</h3>
              <span>{provider.name}</span>
            </div>
            {#if item.inUse}<span class="usage-badge"><i></i>Registry</span>{/if}
          </header>
          <div class="bucket-name"><Icon name="database" size={16} /><strong>{item.bucket}</strong></div>
          <dl class="connection-meta">
            <div><dt>Endpoint</dt><dd title={item.endpoint || 'AWS managed endpoint'}>{endpointLabel(item)}</dd></div>
            <div><dt>Region</dt><dd>{item.region}</dd></div>
            <div><dt>URL style</dt><dd>{item.forcePathStyle ? 'Path style' : 'Virtual host'}</dd></div>
            <div><dt>Updated</dt><dd>{formatDate(item.updatedAt)}</dd></div>
          </dl>
          <footer>
            <span><Icon name="lock" size={12} /> Secret stored encrypted</span>
            <div>
              <button class="btn btn-sm" type="button" onclick={() => openEdit(item)}>Edit</button>
              <button class="btn btn-sm btn-danger" type="button" disabled={item.inUse} title={item.inUse ? 'Switch the Registry backend before removing this connection' : ''} onclick={() => { removeError = ''; removeTarget = item; }}>Remove</button>
            </div>
          </footer>
        </article>
      {/each}
    </div>
  {/if}
</Shell>

{#if editorOpen}
  <div class="editor-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeEditor(); }}>
    <div class="storage-editor" role="dialog" aria-modal="true" aria-labelledby="storage-editor-title">
      <header class="editor-header">
        <div>
          <span class="eyebrow">{form.id ? 'Edit connection' : 'New connection'}</span>
          <h2 id="storage-editor-title">{form.id ? form.name : 'Add object storage'}</h2>
        </div>
        <button class="icon-close" type="button" aria-label="Close" onclick={closeEditor}><Icon name="x" size={16} /></button>
      </header>

      <form onsubmit={(event) => { event.preventDefault(); save(); }}>
        <fieldset class="provider-fieldset">
          <legend>Provider</legend>
          <div class="provider-grid">
            {#each providers as provider}
              <button type="button" class:active={form.provider === provider.id} aria-pressed={form.provider === provider.id} onclick={() => chooseProvider(provider.id)}>
                <span class="provider-mark {provider.id}">{provider.mark}</span>
                <span><strong>{provider.name}</strong><small>{provider.hint}</small></span>
                {#if form.provider === provider.id}<i><Icon name="check" size={11} /></i>{/if}
              </button>
            {/each}
          </div>
        </fieldset>

        <div class="editor-fields">
          <label class="field wide">
            <span>Connection name</span>
            <input class="input" bind:value={form.name} autocomplete="off" placeholder="Production registry storage" maxlength="80" required />
            <small>A recognizable name shown when choosing storage elsewhere in Dokyr.</small>
          </label>
          <label class="field">
            <span>Bucket</span>
            <input class="input input-mono" bind:value={form.bucket} autocomplete="off" placeholder="dokyr-registry" required />
          </label>
          <label class="field">
            <span>Region</span>
            <input class="input input-mono" bind:value={form.region} autocomplete="off" placeholder={form.provider === 'r2' ? 'auto' : 'us-east-1'} required />
          </label>
          <label class="field wide">
            <span>Endpoint <em>{form.provider === 'aws' ? 'optional' : 'required'}</em></span>
            <input class="input input-mono" bind:value={form.endpoint} type="url" autocomplete="off" placeholder={endpointPlaceholder} required={form.provider !== 'aws'} />
            <small>Use only the origin, without the bucket name or a path.</small>
          </label>
          <label class="field">
            <span>Access key ID</span>
            <input class="input input-mono" bind:value={form.accessKey} autocomplete="off" spellcheck="false" required />
          </label>
          <label class="field">
            <span>Secret access key <em>{form.hasSecretKey ? 'leave blank to keep current' : 'required'}</em></span>
            <input class="input input-mono" bind:value={form.secretKey} type="password" autocomplete="new-password" spellcheck="false" placeholder={form.hasSecretKey ? 'Stored encrypted' : ''} required={!form.hasSecretKey} />
          </label>
        </div>

        <div class="behavior-grid">
          <label class="behavior">
            <span class="switch"><input type="checkbox" bind:checked={form.forcePathStyle} /><i></i></span>
            <span><strong>Force path-style URLs</strong><small>Recommended for R2, MinIO, and most custom endpoints.</small></span>
          </label>
          <label class="behavior">
            <span class="switch"><input type="checkbox" bind:checked={form.secure} /><i></i></span>
            <span><strong>Secure transport</strong><small>Keep TLS enabled unless the endpoint is HTTP-only.</small></span>
          </label>
        </div>

        {#if formError}
          <div class="form-error"><Icon name="alert" size={14} /><span>{formError}</span></div>
        {/if}

        <footer class="editor-footer">
          <span><Icon name="lock" size={13} /> Credentials are encrypted at rest.</span>
          <div>
            <button class="btn" type="button" onclick={closeEditor} disabled={saving}>Cancel</button>
            <button class="btn btn-primary" type="submit" disabled={saving}><Icon name={form.id ? 'check' : 'plus'} size={14} /> {saving ? 'Saving…' : form.id ? 'Save changes' : `Connect ${currentProvider.name}`}</button>
          </div>
        </footer>
      </form>
    </div>
  </div>
{/if}

{#if removeTarget}
  <ConfirmDialog
    title="Remove object storage?"
    message={`Remove ${removeTarget.name} from Dokyr. The bucket and its objects will not be deleted.`}
    confirmLabel="Remove connection"
    requireText={removeTarget.name}
    busy={removing}
    error={removeError}
    onConfirm={removeConnection}
    onClose={() => { if (!removing) removeTarget = null; }}
  />
{/if}

<style>
  .storage-hero { min-height: 124px; padding: var(--space-5); display: grid; grid-template-columns: 52px minmax(0, 1fr) auto; align-items: center; gap: var(--space-4); overflow: hidden; position: relative; border: 1px solid color-mix(in srgb, var(--color-accent) 22%, var(--color-rule)); border-radius: var(--radius-lg); background: linear-gradient(120deg, var(--color-accent-softer), var(--color-paper-raised) 60%); box-shadow: var(--shadow-panel); }
  .storage-hero::after { content: ''; width: 220px; height: 220px; position: absolute; right: 12%; top: -170px; border: 1px solid color-mix(in srgb, var(--color-accent) 14%, transparent); border-radius: 50%; box-shadow: 0 0 0 34px color-mix(in srgb, var(--color-accent) 3%, transparent), 0 0 0 68px color-mix(in srgb, var(--color-accent) 2%, transparent); pointer-events: none; }
  .hero-mark { width: 52px; height: 52px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-accent); box-shadow: var(--shadow-whisper); }
  .storage-hero > div:nth-child(2) { display: grid; gap: 4px; }
  .storage-hero h2, .storage-hero p, .section-heading h2, .empty-storage h2 { margin: 0; }
  .storage-hero h2 { font-size: var(--text-lg); letter-spacing: -.015em; }
  .storage-hero p { max-width: 620px; color: var(--color-muted); font-size: var(--text-sm); }
  .storage-hero dl { margin: 0; display: flex; position: relative; z-index: 1; }
  .storage-hero dl div { min-width: 88px; padding: 0 var(--space-4); display: grid; gap: 1px; border-left: 1px solid var(--color-rule); }
  .storage-hero dt { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; text-transform: uppercase; letter-spacing: .06em; }
  .storage-hero dd { margin: 0; font-size: var(--text-xl); font-weight: 650; }
  .alert { margin-top: var(--space-4); }
  .section-heading { margin: var(--space-6) 0 var(--space-3); display: flex; align-items: end; justify-content: space-between; gap: var(--space-4); }
  .section-heading > div { display: grid; gap: 2px; }
  .section-heading h2 { font-size: var(--text-lg); }
  .section-heading p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); }
  .connection-grid, .loading-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-4); }
  .loading-grid { margin-top: var(--space-5); }
  .loading-card { min-height: 300px; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: linear-gradient(100deg, var(--color-paper-raised) 25%, var(--color-paper-subtle) 40%, var(--color-paper-raised) 55%); background-size: 220% 100%; animation: shimmer 1.5s infinite linear; }
  @keyframes shimmer { to { background-position: -220% 0; } }
  .connection-card { min-width: 0; display: grid; grid-template-rows: auto auto 1fr auto; transition: border-color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out); }
  .connection-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-popover); }
  .connection-card.active { border-color: color-mix(in srgb, var(--color-success) 45%, var(--color-rule)); }
  .connection-card header { min-height: 72px; padding: var(--space-4); display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .provider-mark { width: 38px; height: 38px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-ink-secondary); font: 700 10px var(--font-mono); letter-spacing: -.02em; }
  .provider-mark.aws { color: #d97706; background: color-mix(in srgb, #f59e0b 9%, var(--color-paper-raised)); }
  .provider-mark.r2 { color: #f48120; background: color-mix(in srgb, #f48120 9%, var(--color-paper-raised)); }
  .provider-mark.minio { color: #c51a4a; background: color-mix(in srgb, #c51a4a 8%, var(--color-paper-raised)); }
  .provider-mark.digitalocean { color: #0080ff; background: color-mix(in srgb, #0080ff 8%, var(--color-paper-raised)); }
  .provider-mark.custom { color: var(--color-accent); background: var(--color-accent-softer); }
  .connection-title { min-width: 0; display: grid; gap: 2px; }
  .connection-title h3 { margin: 0; overflow: hidden; font-size: var(--text-md); text-overflow: ellipsis; white-space: nowrap; }
  .connection-title span { color: var(--color-muted); font-size: var(--text-xs); }
  .usage-badge { min-height: 24px; padding: 0 7px; display: inline-flex; align-items: center; gap: 6px; border: 1px solid color-mix(in srgb, var(--color-success) 32%, var(--color-rule)); border-radius: 999px; background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-2xs); font-weight: 700; }
  .usage-badge i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
  .bucket-name { margin: var(--space-4) var(--space-4) 0; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-muted); }
  .bucket-name strong { min-width: 0; overflow: hidden; color: var(--color-ink); font: 600 var(--text-sm) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .connection-meta { margin: 0; padding: var(--space-4); display: grid; gap: 9px; }
  .connection-meta div { min-width: 0; display: grid; grid-template-columns: 72px minmax(0, 1fr); gap: var(--space-2); }
  .connection-meta dt { color: var(--color-muted); font-size: var(--text-xs); }
  .connection-meta dd { min-width: 0; margin: 0; overflow: hidden; color: var(--color-ink-secondary); font: 500 var(--text-xs) var(--font-mono); text-align: right; text-overflow: ellipsis; white-space: nowrap; }
  .connection-card > footer { min-height: 54px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .connection-card > footer > span { display: inline-flex; align-items: center; gap: 5px; color: var(--color-muted); font-size: var(--text-2xs); }
  .connection-card > footer > div { display: flex; gap: var(--space-2); }
  .empty-storage { min-height: 260px; margin-top: var(--space-5); padding: var(--space-8); display: grid; justify-items: center; align-content: center; gap: var(--space-4); text-align: center; }
  .empty-cloud { width: 60px; height: 60px; display: grid; place-items: center; border: 1px dashed color-mix(in srgb, var(--color-accent) 40%, var(--color-rule)); border-radius: 50%; background: var(--color-accent-softer); color: var(--color-accent); }
  .empty-storage > div { max-width: 520px; display: grid; gap: 5px; }
  .empty-storage p { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .editor-backdrop { position: fixed; z-index: 160; inset: 0; padding: var(--space-5); display: grid; justify-items: end; background: rgb(6 12 20 / .55); backdrop-filter: blur(3px); }
  .storage-editor { width: min(680px, 100%); height: 100%; overflow-y: auto; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-modal); animation: editor-in var(--duration-base) var(--ease-out); }
  @keyframes editor-in { from { opacity: 0; transform: translateX(24px); } }
  .editor-header { min-height: 70px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .editor-header > div { display: grid; gap: 2px; }
  .editor-header h2 { margin: 0; font-size: var(--text-lg); }
  .icon-close { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); cursor: pointer; }
  .storage-editor form { padding: var(--space-5); display: grid; gap: var(--space-5); }
  .provider-fieldset { min-width: 0; padding: 0; margin: 0; border: 0; }
  .provider-fieldset legend { margin-bottom: var(--space-2); color: var(--color-ink-secondary); font-size: var(--text-xs); font-weight: 600; }
  .provider-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-2); }
  .provider-grid button { min-height: 58px; padding: var(--space-2); position: relative; display: grid; grid-template-columns: 38px minmax(0, 1fr); align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-ink); text-align: left; cursor: pointer; }
  .provider-grid button:last-child { grid-column: 1 / -1; }
  .provider-grid button.active { border-color: var(--color-accent); background: var(--color-accent-softer); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 25%, transparent); }
  .provider-grid button > span:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .provider-grid strong { font-size: var(--text-xs); }
  .provider-grid small { overflow: hidden; color: var(--color-muted); font-size: var(--text-2xs); text-overflow: ellipsis; white-space: nowrap; }
  .provider-grid button > i { width: 18px; height: 18px; position: absolute; right: 7px; top: 7px; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent); color: var(--color-accent-ink); }
  .editor-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4); }
  .wide { grid-column: 1 / -1; }
  .field > span em { color: var(--color-faint); font-style: normal; font-weight: 500; }
  .behavior-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-3); }
  .behavior { min-height: 72px; padding: var(--space-3); display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); cursor: pointer; }
  .behavior > span:last-child { display: grid; gap: 3px; }
  .behavior strong { font-size: var(--text-xs); }
  .behavior small { color: var(--color-muted); font-size: var(--text-2xs); line-height: 1.45; }
  .switch { display: block; }
  .switch input { position: absolute; opacity: 0; pointer-events: none; }
  .switch i { width: 36px; height: 20px; position: relative; display: block; border: 1px solid var(--color-rule-strong); border-radius: 999px; background: var(--color-paper-subtle); }
  .switch i::after { content: ''; width: 14px; height: 14px; position: absolute; left: 2px; top: 2px; border-radius: 50%; background: var(--color-muted); transition: transform var(--duration-fast) var(--ease-out); }
  .switch input:checked + i { border-color: var(--color-accent); background: var(--color-accent); }
  .switch input:checked + i::after { transform: translateX(16px); background: var(--color-accent-ink); }
  .form-error { min-height: 38px; padding: var(--space-2) var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-danger-soft); color: var(--color-danger); font-size: var(--text-xs); }
  .editor-footer { margin: 0 calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .editor-footer > span { display: inline-flex; align-items: center; gap: 6px; color: var(--color-muted); font-size: var(--text-xs); }
  .editor-footer > div { display: flex; gap: var(--space-2); }
  @media (max-width: 1050px) { .connection-grid, .loading-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
  @media (max-width: 700px) {
    .storage-hero { grid-template-columns: 44px minmax(0, 1fr); }
    .hero-mark { width: 44px; height: 44px; }
    .storage-hero dl { grid-column: 1 / -1; padding-top: var(--space-3); border-top: 1px solid var(--color-rule); }
    .storage-hero dl div:first-child { padding-left: 0; border-left: 0; }
    .connection-grid, .loading-grid, .editor-fields, .behavior-grid, .provider-grid { grid-template-columns: 1fr; }
    .provider-grid button:last-child, .wide { grid-column: auto; }
    .section-heading { align-items: start; flex-direction: column; }
    .editor-backdrop { padding: 0; }
    .storage-editor { border-radius: 0; border-width: 0; }
    .editor-footer { align-items: stretch; flex-direction: column; }
    .editor-footer > div, .editor-footer .btn { width: 100%; }
    .connection-card > footer { align-items: flex-start; flex-direction: column; }
  }
</style>
