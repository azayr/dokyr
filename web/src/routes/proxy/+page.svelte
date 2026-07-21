<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let loading = true;
  let saving = false;
  let resetting = false;
  let copied = false;
  let error = '';
  let notice = '';
  let connected = false;
  let connectionError = '';
  let routes = [];
  let configuration = '';
  let managedConfiguration = '';
  let dirty = false;

  onMount(load);

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await api('/api/caddy/config');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load Caddy configuration');
      connected = payload.connected;
      connectionError = payload.connectionError || '';
      routes = payload.routes || [];
      configuration = payload.configuration || '';
      managedConfiguration = configuration;
      dirty = false;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load Caddy configuration';
    } finally {
      loading = false;
    }
  }

  function changeConfiguration(event) {
    configuration = event.currentTarget.value;
    dirty = configuration !== managedConfiguration;
    notice = '';
  }

  async function applyConfiguration() {
    saving = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/caddy/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ configuration })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Caddy rejected the configuration');
      dirty = configuration !== managedConfiguration;
      connected = true;
      notice = 'Caddy accepted the runtime configuration without downtime.';
      toast.success('Proxy configuration applied');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Caddy rejected the configuration';
    } finally {
      saving = false;
    }
  }

  async function resetManaged() {
    resetting = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/caddy/reset', { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not restore managed routes');
      routes = payload.routes || [];
      configuration = payload.configuration || '';
      managedConfiguration = configuration;
      dirty = false;
      connected = true;
      notice = 'Database-managed project routes have been restored.';
      toast.success('Managed routes restored');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not restore managed routes';
    } finally {
      resetting = false;
    }
  }

  async function copyConfiguration() {
    await navigator.clipboard.writeText(configuration);
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }
</script>

<Shell eyebrow="Infrastructure" title="Proxy" subtitle="Caddy edge routing, domain assignments, and runtime configuration.">
  <div slot="actions" class="connection" class:offline={!connected}>
    <i></i>
    <span><strong>{connected ? 'Connected' : 'Unavailable'}</strong><small>Caddy admin socket</small></span>
  </div>

  {#if error}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Configuration not applied</strong><span>{error}</span></div></div>
  {/if}
  {#if notice}
    <div class="alert alert-success"><Icon name="check-circle" size={15} /><div><strong>Proxy updated</strong><span>{notice}</span></div></div>
  {/if}
  {#if connectionError}
    <div class="alert alert-warning"><Icon name="alert" size={15} /><div><strong>Caddy admin API is unavailable</strong><span>{connectionError}</span></div></div>
  {/if}

  <div class="proxy-grid">
    <section class="panel" aria-label="Domain routes">
      <header class="panel-header">
        <div>
          <span class="eyebrow">Managed state</span>
          <h2>Domain routes</h2>
        </div>
        <span class="badge">{routes.length}</span>
      </header>
      {#if loading}
        <div class="rows-loading">
          {#each Array(3) as _}
            <div class="row-skeleton"><span class="skeleton" style="height:14px;width:40%"></span><span class="skeleton" style="height:14px;flex:1"></span><span class="skeleton" style="width:70px;height:22px"></span></div>
          {/each}
        </div>
      {:else if routes.length === 0}
        <EmptyState icon="globe" title="No domains assigned" description="Add a hostname from a project's Domains tab to route traffic here." />
      {:else}
        <div class="table-scroll">
          <table class="data-table route-table">
            <thead><tr><th>Domain</th><th>Upstream</th><th>TLS</th></tr></thead>
            <tbody>
              {#each routes as route}
                <tr>
                  <td><span class="route-domain"><i></i><code>{route.domain}</code></span></td>
                  <td><code class="route-upstream">{route.upstream}</code></td>
                  <td><span class="badge" class:badge-success={route.https}>{route.https ? 'Automatic' : 'HTTP only'}</span></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <aside class="edge-notes">
      <article>
        <span class="edge-icon"><Icon name="lock" size={14} /></span>
        <div><strong>No host ports</strong><p>Application ports are reachable only through the private <code>selfhost-proxy</code> network.</p></div>
      </article>
      <article>
        <span class="edge-icon"><Icon name="shield" size={14} /></span>
        <div><strong>Automatic TLS</strong><p>Real domains can obtain and renew certificates when ports 80 and 443 reach Caddy.</p></div>
      </article>
      <article>
        <span class="edge-icon"><Icon name="check-circle" size={14} /></span>
        <div><strong>Safe reload</strong><p>Caddy rejects invalid edits and keeps the last working configuration active.</p></div>
      </article>
    </aside>
  </div>

  <section class="panel editor" aria-label="Caddy configuration editor">
    <header class="panel-header editor-head">
      <div>
        <span class="eyebrow">Advanced runtime editor</span>
        <h2>Generated Caddyfile</h2>
        <p>Project domain changes regenerate this file. Manual edits are runtime overrides and may be replaced by the next managed route update.</p>
      </div>
      <div class="editor-actions">
        <button class="btn btn-sm" onclick={copyConfiguration}><Icon name={copied ? 'check' : 'copy'} size={13} />{copied ? 'Copied' : 'Copy'}</button>
        <button class="btn btn-sm" onclick={resetManaged} disabled={resetting}><Icon name="refresh" size={13} />{resetting ? 'Restoring…' : 'Restore managed'}</button>
        <button class="btn btn-sm btn-primary" onclick={applyConfiguration} disabled={saving || !dirty || !connected}>{saving ? 'Applying…' : 'Validate & apply'}</button>
      </div>
    </header>
    <div class="editor-state">
      <span><i class:changed={dirty}></i>{dirty ? 'Unsaved runtime override' : 'Matches managed state'}</span>
      <code>text/caddyfile · /run/caddy-admin/admin.sock</code>
    </div>
    <textarea value={configuration} oninput={changeConfiguration} spellcheck="false" aria-label="Caddy configuration"></textarea>
  </section>
</Shell>

<style>
  .connection {
    min-width: 150px;
    padding: 7px var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border: 1px solid color-mix(in srgb, var(--color-success) 32%, var(--color-rule));
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
  }
  .connection > i {
    width: 8px;
    height: 8px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: var(--color-success);
    box-shadow: 0 0 0 4px var(--color-success-soft);
  }
  .connection.offline {
    border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule));
  }
  .connection.offline > i {
    background: var(--color-danger);
    box-shadow: none;
  }
  .connection > span {
    display: grid;
    gap: 1px;
  }
  .connection strong {
    font-size: var(--text-xs);
  }
  .connection small {
    color: var(--color-muted);
    font-size: var(--text-2xs);
  }

  .proxy-grid {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: minmax(0, 1.6fr) minmax(260px, 0.6fr);
    gap: var(--space-4);
    align-items: start;
  }
  .route-domain {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .route-domain i {
    width: 7px;
    height: 7px;
    flex: none;
    border-radius: 50%;
    background: var(--color-success);
  }
  .route-domain code {
    overflow: hidden;
    color: var(--color-ink);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .route-upstream {
    display: block;
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .edge-notes {
    display: grid;
    gap: var(--space-3);
  }
  .edge-notes article {
    padding: var(--space-4);
    display: grid;
    grid-template-columns: 30px minmax(0, 1fr);
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .edge-icon {
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-accent-soft);
    color: var(--color-accent);
  }
  .edge-notes strong {
    font-size: var(--text-sm);
  }
  .edge-notes p {
    margin: 3px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.55;
  }
  .edge-notes code {
    font-size: var(--text-xs);
  }

  .editor-head {
    align-items: flex-start;
  }
  .editor-head p {
    max-width: 70ch;
    margin: var(--space-1) 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .editor-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .editor-state {
    height: 38px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-log-rule);
    background: var(--color-log-surface);
    color: var(--color-log-muted);
  }
  .editor-state span {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font: 500 var(--text-xs) var(--font-mono);
  }
  .editor-state i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-success);
  }
  .editor-state i.changed {
    background: var(--color-warning);
  }
  .editor-state code {
    font-size: var(--text-2xs);
  }
  .editor textarea {
    width: 100%;
    min-height: 400px;
    padding: var(--space-4);
    display: block;
    resize: vertical;
    border: 0;
    outline: 0;
    background: var(--color-log-bg);
    color: var(--color-log-text);
    caret-color: var(--color-accent);
    font: var(--text-sm)/1.65 var(--font-mono);
    tab-size: 2;
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

  @media (max-width: 54rem) {
    .proxy-grid {
      grid-template-columns: 1fr;
    }
    .edge-notes {
      grid-template-columns: repeat(3, 1fr);
    }
    .editor-head {
      align-items: stretch;
      flex-direction: column;
    }
    .editor-actions {
      justify-content: flex-start;
    }
  }
  @media (max-width: 38rem) {
    .edge-notes {
      grid-template-columns: 1fr;
    }
    .editor-actions .btn {
      flex: 1;
    }
    .editor-state code {
      display: none;
    }
    .route-table th:nth-child(3),
    .route-table td:nth-child(3) {
      display: none;
    }
  }
</style>
