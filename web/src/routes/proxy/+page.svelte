<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

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
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not restore managed routes';
    } finally {
      resetting = false;
    }
  }

  async function copyConfiguration() {
    await navigator.clipboard.writeText(configuration);
    copied = true;
    setTimeout(() => copied = false, 1500);
  }
</script>

<Shell eyebrow="Edge network" title="Proxy & Caddy">
  <section class="proxy-hero">
    <div class="proxy-mark"><Icon name="globe" size={22} /></div>
    <div><span>Ingress controller</span><h2>One public edge, private workloads.</h2><p>Caddy owns ports 80 and 443. Applications remain on the private proxy network and receive traffic on their configured container port.</p></div>
    <div class:offline={!connected} class="connection"><i></i><span><strong>{connected ? 'Connected' : 'Unavailable'}</strong><small>Unix admin socket</small></span></div>
  </section>

  {#if error}<div class="feedback error"><strong>Configuration not applied</strong><span>{error}</span></div>{/if}
  {#if notice}<div class="feedback success"><strong>Proxy updated</strong><span>{notice}</span></div>{/if}
  {#if connectionError}<div class="feedback warning"><strong>Caddy admin API is unavailable</strong><span>{connectionError}</span></div>{/if}

  <div class="proxy-grid">
    <section class="routes panel">
      <header><div><span>Managed state</span><h3>Domain routes</h3></div><b>{routes.length}</b></header>
      <div class="route-head"><span>Domain</span><span>Upstream</span><span>TLS</span></div>
      {#if loading}
        <div class="empty">Reading Caddy state…</div>
      {:else if routes.length === 0}
        <div class="empty"><Icon name="globe" size={22}/><strong>No domains assigned</strong><span>Add a hostname from a project's Domains tab.</span></div>
      {:else}
        {#each routes as route}
          <div class="route-row"><div><i></i><code>{route.domain}</code></div><code>{route.upstream}</code><span class:secure={route.https}>{route.https ? 'Automatic' : 'HTTP only'}</span></div>
        {/each}
      {/if}
    </section>

    <aside class="edge-notes">
      <article><span>01</span><div><strong>No host ports</strong><p>Application ports are reachable only through <code>selfhost-proxy</code>.</p></div></article>
      <article><span>02</span><div><strong>Automatic TLS</strong><p>Real domains can obtain and renew certificates when ports 80 and 443 reach Caddy.</p></div></article>
      <article><span>03</span><div><strong>Safe reload</strong><p>Caddy rejects invalid edits and keeps the last working configuration active.</p></div></article>
    </aside>
  </div>

  <section class="editor panel">
    <header>
      <div><span>Advanced runtime editor</span><h3>Generated Caddyfile</h3><p>Project domain changes regenerate this file. Manual edits are runtime overrides and may be replaced by the next managed route update.</p></div>
      <div class="editor-actions"><button onclick={copyConfiguration}>{copied ? 'Copied' : 'Copy'}</button><button onclick={resetManaged} disabled={resetting}>{resetting ? 'Restoring…' : 'Restore managed'}</button><button class="apply" onclick={applyConfiguration} disabled={saving || !dirty || !connected}>{saving ? 'Applying…' : 'Validate & apply'}</button></div>
    </header>
    <div class="editor-state"><span><i class:changed={dirty}></i>{dirty ? 'Unsaved runtime override' : 'Matches managed state'}</span><code>text/caddyfile · /run/caddy-admin/admin.sock</code></div>
    <textarea value={configuration} oninput={changeConfiguration} spellcheck="false" aria-label="Caddy configuration"></textarea>
  </section>
</Shell>

<style>
  .proxy-hero{margin-bottom:var(--space-5);padding:var(--space-6);display:grid;grid-template-columns:48px minmax(0,1fr) auto;align-items:center;gap:var(--space-4);border:1px solid var(--color-rule);border-radius:var(--radius-md);background:linear-gradient(135deg,var(--color-paper-raised),var(--color-paper-subtle));box-shadow:var(--shadow-whisper)}
  .proxy-mark{width:48px;height:48px;display:grid;place-items:center;border:1px solid color-mix(in oklch,var(--color-accent) 30%,var(--color-rule));border-radius:50%;background:var(--color-accent-soft);color:var(--color-accent)}
  .proxy-hero>div:nth-child(2)>span,.panel header>div>span{color:var(--color-accent);font:10px var(--font-mono);letter-spacing:.08em;text-transform:uppercase}.proxy-hero h2{margin:4px 0 5px;font-size:20px;letter-spacing:-.025em}.proxy-hero p{max-width:68ch;margin:0;color:var(--color-muted);font-size:12px;line-height:1.55}
  .connection{min-width:150px;padding:10px 12px;display:flex;align-items:center;gap:10px;border:1px solid color-mix(in oklch,var(--color-accent) 32%,var(--color-rule));border-radius:var(--radius-sm);background:var(--color-paper-raised)}.connection>i{width:8px;height:8px;border-radius:50%;background:var(--color-accent);box-shadow:0 0 0 4px var(--color-accent-soft)}.connection.offline{border-color:color-mix(in oklch,var(--color-danger) 35%,var(--color-rule))}.connection.offline>i{background:var(--color-danger);box-shadow:none}.connection>span{display:grid;gap:2px}.connection strong{font-size:11px}.connection small{color:var(--color-muted);font:9px var(--font-mono)}
  .feedback{margin-bottom:var(--space-4);padding:var(--space-3) var(--space-4);display:grid;grid-template-columns:180px 1fr;gap:var(--space-3);border:1px solid var(--color-rule);border-radius:var(--radius-sm);font-size:12px}.feedback span{color:var(--color-muted)}.feedback.error{border-color:color-mix(in oklch,var(--color-danger) 35%,var(--color-rule));background:color-mix(in oklch,var(--color-danger) 7%,var(--color-paper-raised))}.feedback.success{background:var(--color-accent-soft)}.feedback.warning{border-color:color-mix(in oklch,var(--color-warning) 35%,var(--color-rule));background:color-mix(in oklch,var(--color-warning) 7%,var(--color-paper-raised))}
  .proxy-grid{margin-bottom:var(--space-5);display:grid;grid-template-columns:minmax(0,1.6fr) minmax(260px,.6fr);gap:var(--space-4)}.panel{overflow:hidden;border:1px solid var(--color-rule);border-radius:var(--radius-md);background:var(--color-paper-raised);box-shadow:var(--shadow-whisper)}.panel>header{min-height:68px;padding:var(--space-4) var(--space-5);display:flex;align-items:center;justify-content:space-between;gap:var(--space-4);border-bottom:1px solid var(--color-rule)}.panel h3{margin:4px 0 0;font-size:15px}.routes header>b{min-width:32px;height:26px;display:grid;place-items:center;border-radius:20px;background:var(--color-paper-subtle);font:11px var(--font-mono)}
  .route-head,.route-row{display:grid;grid-template-columns:minmax(170px,1fr) minmax(190px,1fr) 100px;gap:var(--space-3);align-items:center}.route-head{height:34px;padding:0 var(--space-5);border-bottom:1px solid var(--color-rule);background:var(--color-paper-subtle);color:var(--color-muted);font:9px var(--font-mono);text-transform:uppercase}.route-row{min-height:54px;padding:0 var(--space-5);border-bottom:1px solid var(--color-rule)}.route-row:last-child{border-bottom:0}.route-row>div{min-width:0;display:flex;align-items:center;gap:9px}.route-row>div i{width:7px;height:7px;flex:none;border-radius:50%;background:var(--color-accent)}.route-row code{overflow:hidden;color:var(--color-ink-secondary);font:10px var(--font-mono);text-overflow:ellipsis;white-space:nowrap}.route-row>div code{color:var(--color-ink)}.route-row>span{width:max-content;padding:5px 7px;border-radius:4px;background:var(--color-paper-subtle);color:var(--color-muted);font:9px var(--font-mono)}.route-row>span.secure{background:var(--color-accent-soft);color:var(--color-accent)}.empty{min-height:170px;display:grid;place-items:center;align-content:center;gap:7px;color:var(--color-muted);font-size:11px}.empty strong{color:var(--color-ink);font-size:12px}
  .edge-notes{display:grid;align-content:start;gap:var(--space-2)}.edge-notes article{padding:var(--space-4);display:grid;grid-template-columns:30px 1fr;gap:var(--space-3);border:1px solid var(--color-rule);border-radius:var(--radius-md);background:var(--color-paper-raised)}.edge-notes article>span{color:var(--color-accent);font:10px var(--font-mono)}.edge-notes strong{font-size:12px}.edge-notes p{margin:4px 0 0;color:var(--color-muted);font-size:11px;line-height:1.5}.edge-notes code{font:10px var(--font-mono)}
  .editor>header{align-items:flex-start}.editor header p{max-width:76ch;margin:5px 0 0;color:var(--color-muted);font-size:11px;line-height:1.5}.editor-actions{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:var(--space-2)}.editor-actions button{min-height:36px;padding:0 var(--space-3);border:1px solid var(--color-rule);border-radius:var(--radius-sm);background:var(--color-paper-subtle);color:var(--color-ink);font-size:11px;font-weight:600;cursor:pointer}.editor-actions button.apply{border-color:var(--color-accent);background:var(--color-accent);color:var(--color-accent-ink)}.editor-actions button:disabled{opacity:.45;cursor:not-allowed}.editor-state{height:38px;padding:0 var(--space-4);display:flex;align-items:center;justify-content:space-between;gap:var(--space-3);border-bottom:1px solid var(--color-log-rule);background:var(--color-log-surface);color:var(--color-log-muted)}.editor-state span{display:flex;align-items:center;gap:8px;font:10px var(--font-mono)}.editor-state i{width:7px;height:7px;border-radius:50%;background:var(--color-accent)}.editor-state i.changed{background:var(--color-warning)}.editor-state code{font:9px var(--font-mono)}.editor textarea{width:100%;min-height:390px;padding:var(--space-5);display:block;resize:vertical;border:0;outline:0;background:var(--color-log-bg);color:var(--color-log-text);font:12px/1.65 var(--font-mono);tab-size:2}
  @media(max-width:850px){.proxy-hero{grid-template-columns:48px 1fr}.connection{grid-column:1/-1}.proxy-grid{grid-template-columns:1fr}.edge-notes{grid-template-columns:repeat(3,1fr)}.editor>header{align-items:stretch;flex-direction:column}.editor-actions{justify-content:flex-start}}
  @media(max-width:620px){.proxy-hero{grid-template-columns:1fr}.proxy-mark{display:none}.route-head{display:none}.route-row{grid-template-columns:1fr;gap:7px;padding-block:12px}.edge-notes{grid-template-columns:1fr}.editor-actions button{flex:1}.editor-state code{display:none}.feedback{grid-template-columns:1fr}}
</style>
