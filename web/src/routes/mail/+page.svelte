<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api, can, currentPermissions, readAPIJSON } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let loading = true;
  let saving = false;
  let verifying = '';
  let error = '';
  let domains = [];
  let apiKeys = [];
  let messages = [];
  let stalwartConnected = false;
  let deliveryConfigured = false;
  let mailSetup = false;
  let mailServerHostname = '';
  let setupHostname = '';
  let tab = 'domains';
  let selectedId = '';
  let domainDialog = false;
  let keyDialog = false;
  let deleteTarget = null;
  let deleteError = '';
  let newDomain = '';
  let keyDraft = { name: '', domainId: '' };
  let revealedSecret = '';

  $: selected = domains.find((domain) => domain.id === selectedId) || domains[0] || null;
  $: verifiedDomains = domains.filter((domain) => domain.status === 'verified');
  $: canWrite = can($currentPermissions, 'project:write');
  $: canWriteSecrets = can($currentPermissions, 'secret:write');
  $: canSetupMail = can($currentPermissions, 'platform:write');

  onMount(load);

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await api('/api/mail');
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not load mail');
      domains = payload.domains || [];
      apiKeys = payload.apiKeys || [];
      messages = payload.messages || [];
      stalwartConnected = Boolean(payload.stalwartConnected);
      deliveryConfigured = Boolean(payload.deliveryConfigured);
      mailSetup = Boolean(payload.mailSetup);
      mailServerHostname = payload.mailServerHostname || '';
      if (!setupHostname) setupHostname = mailServerHostname;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load mail';
    } finally {
      loading = false;
    }
  }

  async function setupMailServer() {
    saving = true;
    error = '';
    try {
      const response = await api('/api/mail/setup', {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ hostname: setupHostname })
      });
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not set up the mail server');
      toast.success(payload.refreshedDomains ? `Mail server ready. Refreshed ${payload.refreshedDomains} domain${payload.refreshedDomains === 1 ? '' : 's'}.` : 'Mail server is ready.');
      await load();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not set up the mail server';
    } finally {
      saving = false;
    }
  }

  async function createDomain() {
    saving = true;
    error = '';
    try {
      const response = await api('/api/mail/domains', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: newDomain })
      });
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not add domain');
      domains = [payload.domain, ...domains];
      selectedId = payload.domain.id;
      newDomain = '';
      domainDialog = false;
      toast.success('Domain added. Publish the ownership record to continue.');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not add domain';
    } finally {
      saving = false;
    }
  }

  async function verifyDomain(domain) {
    verifying = domain.id;
    error = '';
    try {
      const response = await api(`/api/mail/domains/${domain.id}/verify`, { method: 'POST' });
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not verify DNS');
      domains = domains.map((item) => item.id === domain.id ? payload.domain : item);
      toast.success(payload.domain.status === 'verified' ? 'Domain verified and ready to send.' : 'DNS check completed.');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not verify DNS';
      toast.error(error);
    } finally {
      verifying = '';
    }
  }

  async function removeDomain() {
    if (!deleteTarget) return;
    saving = true;
    deleteError = '';
    try {
      const response = await api(`/api/mail/domains/${deleteTarget.id}`, { method: 'DELETE' });
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not remove domain');
      domains = domains.filter((item) => item.id !== deleteTarget.id);
      apiKeys = apiKeys.filter((key) => key.domainId !== deleteTarget.id);
      deleteTarget = null;
      selectedId = domains[0]?.id || '';
      toast.success('Mail domain removed.');
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not remove domain';
    } finally {
      saving = false;
    }
  }

  function openKeyDialog() {
    keyDraft = { name: '', domainId: verifiedDomains[0]?.id || '' };
    revealedSecret = '';
    keyDialog = true;
  }

  async function createKey() {
    saving = true;
    error = '';
    try {
      const response = await api('/api/mail/api-keys', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(keyDraft)
      });
      const payload = await readAPIJSON(response);
      if (!response.ok) throw new Error(payload.error || 'Could not create API key');
      apiKeys = [payload.apiKey, ...apiKeys];
      revealedSecret = payload.secret;
      toast.success('API key created. Copy it now.');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not create API key';
    } finally {
      saving = false;
    }
  }

  async function revokeKey(key) {
    const response = await api(`/api/mail/api-keys/${key.id}`, { method: 'DELETE' });
    const payload = await readAPIJSON(response);
    if (!response.ok) { toast.error(payload.error || 'Could not revoke key'); return; }
    apiKeys = apiKeys.filter((item) => item.id !== key.id);
    toast.success('API key revoked.');
  }

  async function copy(value, label = 'Value') {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied.`);
  }

  function statusMeta(status) {
    return {
      verified: { label: 'Verified', icon: 'check-circle', tone: 'success' },
      pending_ownership: { label: 'Ownership required', icon: 'clock', tone: 'warning' },
      pending_dns: { label: 'DNS setup', icon: 'clock', tone: 'warning' },
      temporary_failure: { label: 'DNS issue', icon: 'alert', tone: 'danger' },
      failed: { label: 'Failed', icon: 'x-circle', tone: 'danger' }
    }[status] || { label: status, icon: 'clock', tone: 'neutral' };
  }

  function formatDate(value) {
    if (!value) return 'Never';
    return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
  }
</script>

<svelte:head><title>Mail · Dokyr</title></svelte:head>

<Shell eyebrow="Infrastructure" title="Mail" subtitle="Verify sending domains and issue scoped API keys for your applications.">
  <div slot="actions">{#if mailSetup && canWrite}<button class="btn btn-primary" type="button" onclick={() => (domainDialog = true)}><Icon name="plus" size={14} /> Add domain</button>{/if}</div>

  {#if error}<div class="alert alert-error page-alert"><Icon name="x-circle" size={15} /><div><strong>Mail action failed</strong><span>{error}</span></div></div>{/if}

  <section class="mail-readiness" aria-label="Mail readiness">
    <div class="signal-orbit"><span></span><i></i><b></b><Icon name="mail" size={22} /></div>
    <div class="readiness-copy">
      <span class="eyebrow">Developer email gateway</span>
      <strong>{!mailSetup ? 'Set up your mail server' : verifiedDomains.length > 0 ? `${verifiedDomains.length} verified sending domain${verifiedDomains.length === 1 ? '' : 's'}` : 'Connect your first sending domain'}</strong>
      <p>{mailSetup ? 'Dokyr proves ownership first, then checks every DKIM, SPF, and return-path record before enabling delivery.' : 'Choose one public hostname for SMTP identity and MX records. Developer domains can be added afterward.'}</p>
    </div>
    <div class="readiness-checks">
      <span class:ready={stalwartConnected}><i></i>Stalwart {stalwartConnected ? 'connected' : 'not connected'}</span>
      <span class:ready={deliveryConfigured}><i></i>Delivery {deliveryConfigured ? 'configured' : 'not configured'}</span>
    </div>
  </section>

  {#if !loading && !mailSetup}
    <section class="panel setup-panel">
      <div class="setup-copy"><span class="setup-icon"><Icon name="settings" size={20} /></span><div><span class="eyebrow">One-time platform setup</span><h2>Choose the server developers will send through</h2><p>This is your infrastructure hostname, not a developer’s sending domain. Every verified domain will point its MX and discovery records here.</p></div></div>
      {#if canSetupMail}
        <form onsubmit={(event) => { event.preventDefault(); setupMailServer(); }}>
          <label class="field"><span>Mail server hostname</span><input class="input input-mono" bind:value={setupHostname} placeholder="mail.example.com" autocomplete="off" spellcheck="false" required /><small>Create an A record pointing this hostname to the Dokyr server and configure matching reverse DNS.</small></label>
          <button class="btn btn-primary" type="submit" disabled={saving || !setupHostname.trim()}>{saving ? 'Setting up…' : 'Set up mail server'}</button>
        </form>
      {:else}
        <div class="safe-note warning"><Icon name="alert" size={15} /><span>Ask the platform owner to complete Mail setup before adding sending domains.</span></div>
      {/if}
    </section>
  {:else}
  <nav class="mail-tabs" aria-label="Mail sections">
    <button class:active={tab === 'domains'} type="button" onclick={() => (tab = 'domains')}><Icon name="globe" size={14} /> Domains <span>{domains.length}</span></button>
    <button class:active={tab === 'keys'} type="button" onclick={() => (tab = 'keys')}><Icon name="key" size={14} /> API keys <span>{apiKeys.length}</span></button>
    <button class:active={tab === 'activity'} type="button" onclick={() => (tab = 'activity')}><Icon name="logs" size={14} /> Activity <span>{messages.length}</span></button>
  </nav>

  {#if tab === 'domains'}
    {#if loading}
      <div class="loading-grid"><div></div><div></div><div></div></div>
    {:else if domains.length === 0}
      <section class="panel empty-panel">
        <EmptyState icon="mail" title="No sending domains" description="Add a domain to receive a unique ownership record and begin DNS verification.">
          {#if canWrite}<button class="btn btn-primary" type="button" onclick={() => (domainDialog = true)}><Icon name="plus" size={14} /> Add a domain</button>{/if}
        </EmptyState>
      </section>
    {:else}
      <div class="domain-workspace">
        <aside class="domain-list panel" aria-label="Sending domains">
          <header><span>Sending domains</span><small>{verifiedDomains.length}/{domains.length} ready</small></header>
          {#each domains as domain}
            {@const meta = statusMeta(domain.status)}
            <button class:selected={selected?.id === domain.id} type="button" onclick={() => (selectedId = domain.id)}>
              <span class="domain-mark" class:verified={domain.status === 'verified'}><Icon name={meta.icon} size={14} /></span>
              <span><strong>{domain.name}</strong><small class={meta.tone}>{meta.label}</small></span>
              <Icon name="chevron-right" size={13} />
            </button>
          {/each}
        </aside>

        {#if selected}
          {@const selectedMeta = statusMeta(selected.status)}
          <section class="domain-detail panel">
            <header class="domain-head">
              <div>
                <span class="status-pill {selectedMeta.tone}"><Icon name={selectedMeta.icon} size={12} />{selectedMeta.label}</span>
                <h2>{selected.name}</h2>
                <p>{selected.status === 'verified' ? 'This domain can send through the Dokyr API.' : 'Publish the records below, then ask Dokyr to check public DNS.'}</p>
              </div>
              <div class="domain-actions">
                {#if canWrite}<button class="btn" type="button" disabled={verifying === selected.id} onclick={() => verifyDomain(selected)}><Icon name="refresh" size={13} />{verifying === selected.id ? 'Checking…' : 'Verify DNS'}</button>{/if}
                {#if canWrite}<button class="icon-danger" type="button" aria-label="Remove domain" onclick={() => { deleteError = ''; deleteTarget = selected; }}><Icon name="trash" size={14} /></button>{/if}
              </div>
            </header>

            {#if selected.lastError}
              <div class="domain-note" class:warning={selected.status !== 'verified'}><Icon name="info" size={14} /><span>{selected.lastError}</span></div>
            {/if}

            <div class="record-guide">
              <div><b>1</b><span><strong>Copy records</strong><small>Use your DNS provider</small></span></div>
              <i></i>
              <div><b>2</b><span><strong>Wait for DNS</strong><small>Usually a few minutes</small></span></div>
              <i></i>
              <div><b>3</b><span><strong>Verify</strong><small>Dokyr checks publicly</small></span></div>
            </div>

            <div class="records-table" role="table" aria-label="DNS records">
              <div class="record-row record-header" role="row"><span>Status</span><span>Type</span><span>Name</span><span>Value</span><span></span></div>
              {#each selected.records || [] as record}
                <div class="record-row" role="row">
                  <span class="record-status {record.status}" title={record.lastError || record.status}><Icon name={record.status === 'verified' ? 'check' : record.status === 'incorrect' ? 'alert' : 'clock'} size={12} /></span>
                  <span><code class="record-type">{record.type}</code>{#if record.priority}<small>Priority {record.priority}</small>{/if}</span>
                  <span class="record-value"><small>{record.purpose}</small><code>{record.name}</code></span>
                  <span class="record-value"><code>{record.value}</code></span>
                  <button type="button" aria-label={`Copy ${record.purpose} record`} onclick={() => copy(record.value, record.purpose)}><Icon name="copy" size={13} /></button>
                </div>
              {/each}
            </div>
            <footer class="domain-foot"><Icon name="clock" size={12} /> Last checked {formatDate(selected.lastCheckedAt)}</footer>
          </section>
        {/if}
      </div>
    {/if}
  {:else if tab === 'keys'}
    <section class="panel keys-panel">
      <header class="section-head"><div><h2>Sending API keys</h2><p>Every key is locked to one verified domain.</p></div>{#if canWriteSecrets}<button class="btn btn-primary" type="button" disabled={verifiedDomains.length === 0} onclick={openKeyDialog}><Icon name="plus" size={13} /> Create API key</button>{/if}</header>
      {#if apiKeys.length === 0}
        <EmptyState icon="key" title="No mail API keys" description={verifiedDomains.length ? 'Create a key for an application that needs to send.' : 'Verify a domain before creating a sending key.'} />
      {:else}
        <div class="key-list">
          {#each apiKeys as key}
            <article><span class="key-icon"><Icon name="key" size={15} /></span><div><strong>{key.name}</strong><small>{key.domainName} · <code>{key.prefix}••••</code></small></div><span class="key-time">Last used<br><b>{formatDate(key.lastUsedAt)}</b></span>{#if canWriteSecrets}<button class="icon-danger" type="button" aria-label="Revoke API key" onclick={() => revokeKey(key)}><Icon name="trash" size={13} /></button>{/if}</article>
          {/each}
        </div>
      {/if}
    </section>
  {:else}
    <section class="panel activity-panel">
      <header class="section-head"><div><h2>Recent delivery activity</h2><p>The first 25 messages accepted by this Dokyr server.</p></div></header>
      {#if messages.length === 0}
        <EmptyState icon="logs" title="No messages yet" description="Messages sent through POST /v1/emails will appear here." />
      {:else}
        <div class="message-list">
          {#each messages as message}
            <article><span class="delivery-mark {message.status}"><Icon name={message.status === 'sent' ? 'check' : message.status === 'failed' ? 'x' : 'clock'} size={13} /></span><div><strong>{message.subject}</strong><small>{message.from} → {message.to.join(', ')}</small></div><span><b class={message.status}>{message.status}</b><small>{formatDate(message.createdAt)}</small></span></article>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
  {/if}
</Shell>

{#if domainDialog}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !saving) domainDialog = false; }}>
    <div class="modal mail-modal" role="dialog" aria-modal="true" aria-labelledby="add-domain-title">
      <header><div><span class="eyebrow">Sending identity</span><h2 id="add-domain-title">Add a mail domain</h2></div><button type="button" onclick={() => (domainDialog = false)} disabled={saving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createDomain(); }}>
        <div class="modal-body"><label class="field"><span>Sending domain</span><input class="input input-mono" bind:value={newDomain} placeholder="updates.example.com" autocomplete="off" spellcheck="false" required /><small>A dedicated subdomain keeps application mail separate from your existing inbox and MX records.</small></label><div class="safe-note"><Icon name="shield" size={15} /><span>Nothing is provisioned in Stalwart until ownership passes.</span></div></div>
        <footer><button class="btn" type="button" onclick={() => (domainDialog = false)} disabled={saving}>Cancel</button><button class="btn btn-primary" type="submit" disabled={saving || !newDomain.trim()}>{saving ? 'Adding…' : 'Add domain'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if keyDialog}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !saving) keyDialog = false; }}>
    <div class="modal mail-modal" role="dialog" aria-modal="true" aria-labelledby="key-title">
      <header><div><span class="eyebrow">Developer access</span><h2 id="key-title">{revealedSecret ? 'Copy your API key' : 'Create a sending key'}</h2></div><button type="button" onclick={() => (keyDialog = false)} disabled={saving}>×</button></header>
      {#if revealedSecret}
        <div class="modal-body"><div class="secret-box"><code>{revealedSecret}</code><button type="button" onclick={() => copy(revealedSecret, 'API key')}><Icon name="copy" size={14} /> Copy</button></div><div class="safe-note warning"><Icon name="alert" size={15} /><span>This secret is shown once. Dokyr stores only its hash.</span></div></div>
        <footer><button class="btn btn-primary" type="button" onclick={() => (keyDialog = false)}>I saved the key</button></footer>
      {:else}
        <form onsubmit={(event) => { event.preventDefault(); createKey(); }}><div class="modal-body"><label class="field"><span>Key name</span><input class="input" bind:value={keyDraft.name} placeholder="Production API" required /></label><label class="field"><span>Sending domain</span><select class="select" bind:value={keyDraft.domainId} required>{#each verifiedDomains as domain}<option value={domain.id}>{domain.name}</option>{/each}</select></label></div><footer><button class="btn" type="button" onclick={() => (keyDialog = false)}>Cancel</button><button class="btn btn-primary" type="submit" disabled={saving || !keyDraft.name.trim() || !keyDraft.domainId}>{saving ? 'Creating…' : 'Create key'}</button></footer></form>
      {/if}
    </div>
  </div>
{/if}

{#if deleteTarget}
  <ConfirmDialog title={`Remove ${deleteTarget.name}?`} message="This removes the domain from Stalwart, revokes its API keys, and deletes its local delivery history." confirmLabel="Remove domain" busy={saving} error={deleteError} requireText={deleteTarget.name} onConfirm={removeDomain} onClose={() => { if (!saving) deleteTarget = null; }} />
{/if}

<style>
  .mail-readiness { position: relative; min-height: 116px; margin-bottom: var(--space-4); padding: var(--space-5) var(--space-6); display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: var(--space-5); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: linear-gradient(118deg, var(--color-paper-raised) 0%, var(--color-accent-softer) 100%); box-shadow: var(--shadow-panel); }
  .mail-readiness::after { content: ''; position: absolute; width: 280px; height: 280px; right: 18%; top: -210px; border: 1px solid color-mix(in srgb, var(--color-accent) 16%, transparent); border-radius: 50%; }
  .signal-orbit { position: relative; width: 62px; height: 62px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--color-rule)); border-radius: 50%; background: var(--color-paper-raised); color: var(--color-accent); box-shadow: 0 0 0 7px color-mix(in srgb, var(--color-accent) 6%, transparent); }
  .signal-orbit span, .signal-orbit i, .signal-orbit b { position: absolute; width: 4px; height: 4px; border-radius: 50%; background: var(--color-accent); }
  .signal-orbit span { top: 3px; left: 18px; }.signal-orbit i { right: 1px; top: 29px; }.signal-orbit b { bottom: 5px; left: 13px; }
  .readiness-copy { min-width: 0; display: grid; gap: 3px; }.eyebrow { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 750; letter-spacing: .08em; text-transform: uppercase; }.readiness-copy strong { font-size: var(--text-lg); }.readiness-copy p { max-width: 650px; margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .readiness-checks { z-index: 1; min-width: 174px; display: grid; gap: var(--space-2); }.readiness-checks span { display: flex; align-items: center; gap: 8px; color: var(--color-muted); font-size: var(--text-xs); font-weight: 650; }.readiness-checks i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-warning); box-shadow: 0 0 0 3px var(--color-warning-soft); }.readiness-checks .ready i { background: var(--color-success); box-shadow: 0 0 0 3px var(--color-success-soft); }.readiness-checks .ready { color: var(--color-ink-secondary); }
  .mail-tabs { margin-bottom: var(--space-4); display: flex; gap: 2px; border-bottom: 1px solid var(--color-rule); }.mail-tabs button { min-height: 38px; padding: 0 var(--space-3); display: flex; align-items: center; gap: 7px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--color-muted); font-size: var(--text-sm); font-weight: 650; cursor: pointer; }.mail-tabs button:hover { color: var(--color-ink); }.mail-tabs button.active { border-color: var(--color-accent); color: var(--color-accent); }.mail-tabs span { min-width: 19px; padding: 1px 5px; border-radius: 10px; background: var(--color-paper-subtle); color: var(--color-muted); font-size: var(--text-2xs); text-align: center; }
  .domain-workspace { display: grid; grid-template-columns: minmax(220px, .32fr) minmax(0, 1fr); gap: var(--space-4); align-items: start; }.domain-list { padding-bottom: var(--space-2); }.domain-list > header { min-height: 46px; padding: 0 var(--space-4); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }.domain-list > header span { font-size: var(--text-sm); font-weight: 700; }.domain-list > header small { color: var(--color-muted); font-size: var(--text-2xs); }.domain-list > button { width: calc(100% - 12px); min-height: 60px; margin: 6px 6px 0; padding: 0 10px; display: grid; grid-template-columns: 30px 1fr auto; align-items: center; gap: 9px; border: 1px solid transparent; border-radius: var(--radius-md); background: transparent; color: var(--color-ink); text-align: left; cursor: pointer; }.domain-list > button:hover { background: var(--color-paper-subtle); }.domain-list > button.selected { border-color: color-mix(in srgb, var(--color-accent) 24%, var(--color-rule)); background: var(--color-accent-softer); }.domain-list button > span:nth-child(2) { min-width: 0; display: grid; }.domain-list strong { overflow: hidden; font-family: var(--font-mono); font-size: var(--text-xs); text-overflow: ellipsis; }.domain-list small { font-size: var(--text-2xs); }.domain-list small.success { color: var(--color-success); }.domain-list small.warning { color: var(--color-warning); }.domain-list small.danger { color: var(--color-danger); }.domain-mark { width: 28px; height: 28px; display: grid; place-items: center; border-radius: 50%; background: var(--color-warning-soft); color: var(--color-warning); }.domain-mark.verified { background: var(--color-success-soft); color: var(--color-success); }
  .domain-detail { min-width: 0; }.domain-head { min-height: 100px; padding: var(--space-5); display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }.domain-head h2 { margin: 8px 0 2px; font-family: var(--font-mono); font-size: var(--text-lg); }.domain-head p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); }.status-pill { width: fit-content; padding: 3px 7px; display: inline-flex; align-items: center; gap: 5px; border-radius: 12px; font-size: var(--text-2xs); font-weight: 700; }.status-pill.success { background: var(--color-success-soft); color: var(--color-success); }.status-pill.warning { background: var(--color-warning-soft); color: var(--color-warning); }.status-pill.danger { background: var(--color-danger-soft); color: var(--color-danger); }.domain-actions { display: flex; gap: var(--space-2); }.icon-danger { width: 34px; height: 34px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }.icon-danger:hover { border-color: color-mix(in srgb, var(--color-danger) 50%, var(--color-rule)); color: var(--color-danger); }
  .domain-note { margin: var(--space-4) var(--space-5) 0; padding: 10px 12px; display: flex; gap: 8px; border-radius: var(--radius-md); background: var(--color-info-soft); color: var(--color-info); font-size: var(--text-xs); }.domain-note.warning { background: var(--color-warning-soft); color: var(--color-warning); }
  .record-guide { padding: var(--space-5); display: flex; align-items: center; }.record-guide > div { display: flex; align-items: center; gap: 9px; }.record-guide b { width: 24px; height: 24px; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent-soft); color: var(--color-accent); font-size: var(--text-2xs); }.record-guide span { display: grid; }.record-guide strong { font-size: var(--text-xs); }.record-guide small { color: var(--color-muted); font-size: var(--text-2xs); }.record-guide > i { width: clamp(18px, 4vw, 56px); height: 1px; margin: 0 var(--space-3); background: var(--color-rule); }
  .records-table { margin: 0 var(--space-5) var(--space-4); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-md); }.record-row { min-height: 62px; padding: 8px 10px; display: grid; grid-template-columns: 38px 68px minmax(150px, .8fr) minmax(180px, 1.25fr) 28px; align-items: center; gap: 8px; border-top: 1px solid var(--color-rule); }.record-row:first-child { border-top: 0; }.record-header { min-height: 32px; background: var(--color-paper-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; }.record-row > span { min-width: 0; }.record-row > span:nth-child(2) { display: grid; gap: 3px; }.record-row button { width: 28px; height: 28px; display: grid; place-items: center; border: 0; border-radius: var(--radius-xs); background: transparent; color: var(--color-muted); cursor: pointer; }.record-row button:hover { background: var(--color-paper-subtle); color: var(--color-accent); }.record-status { width: 23px; height: 23px; display: grid; place-items: center; border-radius: 50%; background: var(--color-warning-soft); color: var(--color-warning); }.record-status.verified { background: var(--color-success-soft); color: var(--color-success); }.record-status.incorrect { background: var(--color-danger-soft); color: var(--color-danger); }.record-type { width: fit-content; padding: 2px 5px; border-radius: 3px; background: var(--color-paper-subtle); color: var(--color-ink-secondary); font-size: var(--text-2xs); font-weight: 700; }.record-row small { color: var(--color-muted); font-size: 9px; }.record-value { display: grid; gap: 2px; }.record-value code { overflow: hidden; color: var(--color-ink-secondary); font-size: var(--text-2xs); line-height: 1.45; text-overflow: ellipsis; white-space: nowrap; }.domain-foot { min-height: 34px; padding: 0 var(--space-5); display: flex; align-items: center; gap: 6px; border-top: 1px solid var(--color-rule); color: var(--color-muted); font-size: var(--text-2xs); }
  .section-head { min-height: 72px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }.section-head h2 { margin: 0; font-size: var(--text-md); }.section-head p { margin: 3px 0 0; color: var(--color-muted); font-size: var(--text-xs); }.key-list article, .message-list article { min-height: 66px; padding: 0 var(--space-5); display: grid; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); }.key-list article:first-child, .message-list article:first-child { border-top: 0; }.key-list article { grid-template-columns: 34px 1fr auto 34px; }.key-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }.key-list article > div, .message-list article > div { min-width: 0; display: grid; }.key-list strong, .message-list strong { font-size: var(--text-xs); }.key-list small, .message-list small { overflow: hidden; color: var(--color-muted); font-size: var(--text-2xs); text-overflow: ellipsis; white-space: nowrap; }.key-time { color: var(--color-muted); font-size: 9px; text-align: right; }.key-time b { color: var(--color-ink-secondary); font-size: var(--text-2xs); font-weight: 500; }.message-list article { grid-template-columns: 30px 1fr auto; }.message-list article > span:last-child { display: grid; text-align: right; }.message-list b { font-size: var(--text-2xs); text-transform: capitalize; }.message-list b.sent { color: var(--color-success); }.message-list b.failed { color: var(--color-danger); }.delivery-mark { width: 26px; height: 26px; display: grid; place-items: center; border-radius: 50%; background: var(--color-warning-soft); color: var(--color-warning); }.delivery-mark.sent { background: var(--color-success-soft); color: var(--color-success); }.delivery-mark.failed { background: var(--color-danger-soft); color: var(--color-danger); }
  .empty-panel { min-height: 320px; display: grid; place-items: center; }.loading-grid { display: grid; grid-template-columns: 220px 1fr; gap: var(--space-4); }.loading-grid div { min-height: 280px; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: linear-gradient(100deg, var(--color-paper-raised) 20%, var(--color-paper-subtle) 45%, var(--color-paper-raised) 70%); background-size: 300% 100%; animation: shimmer 1.4s infinite; }.loading-grid div:nth-child(2) { grid-column: 2; }.loading-grid div:nth-child(3) { display: none; }@keyframes shimmer { to { background-position: -200% 0; } }
  .setup-panel { padding: var(--space-6); display: grid; grid-template-columns: minmax(0, 1fr) minmax(300px, .7fr); align-items: center; gap: var(--space-7); }.setup-copy { display: flex; align-items: flex-start; gap: var(--space-4); }.setup-icon { width: 46px; height: 46px; flex: 0 0 auto; display: grid; place-items: center; border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }.setup-copy h2 { margin: 5px 0 5px; font-size: var(--text-lg); }.setup-copy p { max-width: 580px; margin: 0; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.55; }.setup-panel form { display: grid; gap: var(--space-3); }.setup-panel form .btn { justify-self: end; }
  .mail-modal { width: min(500px, 100%); }.modal-body { padding: var(--space-5); display: grid; gap: var(--space-4); }.safe-note { padding: 10px 12px; display: flex; align-items: center; gap: 9px; border-radius: var(--radius-md); background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-xs); }.safe-note.warning { background: var(--color-warning-soft); color: var(--color-warning); }.secret-box { padding: 8px; display: flex; align-items: center; gap: 8px; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-log-bg); }.secret-box code { min-width: 0; flex: 1; overflow: hidden; color: var(--color-log-text); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }.secret-box button { min-height: 30px; padding: 0 10px; display: flex; align-items: center; gap: 6px; border: 1px solid var(--color-log-rule); border-radius: var(--radius-sm); background: var(--color-log-surface); color: var(--color-log-text); cursor: pointer; }
  @media (max-width: 900px) { .mail-readiness { grid-template-columns: auto 1fr; }.readiness-checks { grid-column: 2; grid-template-columns: repeat(2, auto); }.setup-panel { grid-template-columns: 1fr; }.setup-panel form .btn { justify-self: start; }.domain-workspace { grid-template-columns: 1fr; }.domain-list { display: grid; grid-template-columns: repeat(2, 1fr); }.domain-list > header { grid-column: 1 / -1; }.record-row { grid-template-columns: 32px 56px minmax(120px, .8fr) minmax(160px, 1fr) 28px; } }
  @media (max-width: 650px) { .mail-readiness { padding: var(--space-4); grid-template-columns: 1fr; }.signal-orbit { display: none; }.readiness-checks { grid-column: 1; grid-template-columns: 1fr; }.mail-tabs { overflow-x: auto; }.domain-list { grid-template-columns: 1fr; }.domain-head { flex-direction: column; }.domain-actions { width: 100%; }.domain-actions .btn { flex: 1; }.record-guide { display: none; }.records-table { margin-top: var(--space-4); overflow-x: auto; }.record-row { width: 690px; }.key-list article { grid-template-columns: 34px 1fr 34px; }.key-time { display: none; }.section-head { align-items: flex-start; flex-direction: column; }.section-head .btn { width: 100%; }.message-list article { grid-template-columns: 30px minmax(0, 1fr); }.message-list article > span:last-child { grid-column: 2; text-align: left; }.loading-grid { grid-template-columns: 1fr; }.loading-grid div:nth-child(2) { display: none; } }
</style>
