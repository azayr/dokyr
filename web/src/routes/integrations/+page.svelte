<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';
  let data={providers:{github:{},gitlab:{}},connections:[],registries:[]};
  let loading=true, saving=false, syncing=false, syncAttempted=false, error='', sourceError='', warning='';
  let registry={name:'',registryUrl:'',username:'',password:''};
  const providerInfo={github:{name:'GitHub',mark:'GH',hint:'Organizations and private repositories'},gitlab:{name:'GitLab',mark:'GL',hint:'Groups and private repositories'}};
  async function load(trySync=true){loading=true;const r=await api('/api/integrations');data=await r.json();loading=false;if(trySync&&!syncAttempted&&data.providers?.github?.linked&&data.providers?.github?.managed){syncAttempted=true;await syncGitHubInstallations(false)}}
  async function syncGitHubInstallations(showEmpty=true){syncing=true;sourceError='';warning='';const r=await api('/api/integrations/github/installations/sync',{method:'POST'});const body=await r.json();if(!r.ok){sourceError=body.error||'Could not synchronize GitHub installations';syncing=false;return}warning=body.warning||'';if(body.synced>0){await load(false)}else if(showEmpty){sourceError=body.message||'No GitHub App installation was found for this account.'}syncing=false}
  onMount(load);
  async function saveRegistry(){saving=true;error='';const r=await api('/api/integrations/registries',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(registry)});const body=await r.json();if(!r.ok){error=body.error;saving=false;return}registry={name:'',registryUrl:'',username:'',password:''};await load();saving=false}
  async function removeRegistry(id){if(!confirm('Remove this registry credential? Existing projects keep their image reference.'))return;await api('/api/integrations/registries/'+id,{method:'DELETE'});await load()}
  async function unlinkSource(account){if(!confirm(`Unlink ${account.provider} account ${account.accountName}? Existing containers keep running, but Git services must be connected again before their next deployment.`))return;error='';const r=await api('/api/integrations/sources/'+account.id,{method:'DELETE'});const body=await r.json();if(!r.ok){error=body.error||'Could not unlink Git source';return}await load()}
</script>

<Shell eyebrow="Workspace" title="Sources & registries">
  {#if page.url.searchParams.get('connected')}<div class="notice success">Account connected. Private repositories are now available when creating a project.</div>{/if}
  {#if page.url.searchParams.get('error')}<div class="notice failure">{page.url.searchParams.get('error')}</div>{/if}
  {#if sourceError}<div class="notice failure">{sourceError}</div>{/if}
  {#if warning}<div class="notice caution">{warning} Open the GitHub App settings, enable read-only Contents permission, then approve the permission update.</div>{/if}
  <section class="hero"><div><small>Source control</small><h2>Connect a Git provider</h2><p>Authorize Selfhost to discover repositories you can access. Provider tokens are encrypted before they are stored.</p></div><span>{data.connections.length} connected</span></section>
  <div class="providers">
    {#each Object.entries(providerInfo) as [key, provider]}
      {@const state=data.providers[key]||{}}
      {@const connections=data.connections.filter((item)=>item.provider===key)}
      <article>
        <div class="provider-head"><b class={key}>{provider.mark}</b><div><h3>{provider.name}</h3><p>{provider.hint}</p></div><span class:ready={connections.length || (key==='github' && state.linked)}>{connections.length?'Connected':key==='github'&&state.linked?'Linked':state.configured?'Ready':'Setup required'}</span></div>
        {#if connections.length}
          <div class="accounts">{#each connections as account}<div>{#if account.accountAvatar}<img src={account.accountAvatar} alt="" />{:else}<i>{provider.mark}</i>{/if}<span><strong>{account.accountName}</strong><small>{account.repositorySelection==='selected'?'Selected repositories':account.repositorySelection==='all'?'All repositories':account.baseUrl}</small>{#if key==='github' && account.contentsPermission!=='read' && account.contentsPermission!=='write'}<small class="permission-missing">Contents permission required for private deploys</small>{/if}</span><div class="account-actions">{#if account.manageUrl}<a href={account.manageUrl}>Change access <Icon name="link" size={12}/></a>{/if}<button onclick={()=>unlinkSource(account)}>Unlink</button></div></div>{/each}</div>
        {:else if key==='github' && state.linked}
          <div class="linked-account"><span class="github-avatar"><Icon name="github" size={15}/></span><span><strong>@{state.login}</strong><small>Linked in Settings · default GitHub account</small></span><em>Ready</em></div>
        {:else}<div class="empty-account">No {provider.name} account connected yet.</div>{/if}
        <footer>
          {#if key==='github'}
            <div><small>Repository access</small><code>{connections.length?'Access can be changed any time on GitHub':state.linked?'Choose all repositories or only selected repositories':'Uses the GitHub account linked in Settings'}</code></div>
            {#if connections.length}<a href="/api/integrations/github/install/start">Add installation →</a>{:else if state.linked && state.managed}<div class="github-source-actions"><button type="button" onclick={()=>syncGitHubInstallations(true)} disabled={syncing}>{syncing?'Checking…':'Refresh installation'}</button><a href="/api/integrations/github/install/start">Select repositories →</a></div>{:else}<a href="/api/account/github/start">Link GitHub →</a>{/if}
          {:else}
            <div><small>OAuth callback</small><code>{state.callbackUrl||'—'}</code></div>
            {#if state.configured}<a href={'/api/integrations/oauth/'+key+'/start'}>Connect {provider.name} →</a>{:else}<button disabled>Configure {provider.name}</button>{/if}
          {/if}
        </footer>
        {#if key==='github' && !state.managed}<div class="config github-help"><span>Link GitHub in <a href="/settings?section=security">Settings → Security</a>. DeployForge creates a private GitHub App and stores its credentials securely.</span></div>{/if}
        {#if key==='gitlab' && !state.configured}<div class="config"><span>GitLab OAuth still requires <code>GITLAB_CLIENT_ID + GITLAB_CLIENT_SECRET</code> for this self-hosted server.</span></div>{/if}
      </article>
    {/each}
  </div>

  <section class="registry-panel">
    <div class="section-head"><div><small>Container sources</small><h2>Private Docker registries</h2><p>Save credentials for GHCR, Docker Hub, GitLab Registry, or any Registry V2 endpoint.</p></div><span>{data.registries.length} saved</span></div>
    <div class="registry-grid">
      <form onsubmit={(event)=>{event.preventDefault();saveRegistry()}}>
        <div class="form-title"><b>Add registry</b><small>Credentials are encrypted at rest</small></div>
        {#if error}<p class="error">{error}</p>{/if}
        <label>Display name<input bind:value={registry.name} placeholder="GitHub Container Registry" required /></label>
        <label>Registry host<input bind:value={registry.registryUrl} placeholder="ghcr.io" required /></label>
        <div class="split"><label>Username<input bind:value={registry.username} placeholder="octocat" /></label><label>Password or token<input bind:value={registry.password} type="password" placeholder="••••••••••••" /></label></div>
        <button disabled={saving}>{saving?'Saving…':'Save registry'}</button>
      </form>
      <div class="registry-list">
        <div class="table-head"><span>Registry</span><span>Credential</span><span></span></div>
        {#if loading}<p class="empty">Loading sources…</p>{:else if !data.registries.length}<div class="empty"><b>No private registries</b><span>Public images can still be deployed without a saved credential.</span></div>{:else}{#each data.registries as item}<div class="registry-row"><div><i>◈</i><span><strong>{item.name}</strong><small>{item.registryUrl}</small></span></div><code>{item.username||'anonymous'} / ••••••••</code><button onclick={()=>removeRegistry(item.id)} aria-label="Remove registry">Remove</button></div>{/each}{/if}
      </div>
    </div>
  </section>
</Shell>

<style>
  .notice{padding:11px 14px;border-radius:7px;margin-bottom:14px;font-size:10px;border:1px solid}.success{background:var(--accent-soft);color:var(--green);border-color:color-mix(in srgb,var(--green) 24%,var(--line))}.failure{background:var(--color-danger-soft);color:var(--red);border-color:color-mix(in srgb,var(--red) 30%,var(--line))}.caution{background:var(--color-warning-soft);color:var(--amber);border-color:color-mix(in srgb,var(--amber) 30%,var(--line))}
  .hero,.section-head{display:flex;justify-content:space-between;align-items:start}.hero{margin-bottom:15px}.hero small,.section-head small{font:8px var(--font-mono);text-transform:uppercase;letter-spacing:.1em;color:var(--accent)}h2{font-size:16px;margin:5px 0 4px}.hero p,.section-head p{margin:0;color:var(--muted);font-size:10px}.hero>span,.section-head>span{font:9px var(--font-mono);color:var(--muted);border:1px solid var(--line);background:var(--surface);padding:6px 8px;border-radius:5px}
  .providers{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-bottom:16px}.providers article,.registry-panel{background:var(--surface);border:1px solid var(--line);border-radius:8px;overflow:hidden}.provider-head{display:grid;grid-template-columns:38px 1fr auto;gap:11px;align-items:center;padding:16px}.provider-head>b{width:36px;height:36px;border-radius:7px;display:grid;place-items:center;background:var(--color-log-bg);color:var(--color-log-text);font:600 9px var(--font-mono)}.provider-head>b.gitlab{background:color-mix(in srgb,#f06a3b 82%,var(--color-log-bg))}.provider-head h3{font-size:12px;margin:0 0 3px}.provider-head p{font-size:9px;color:var(--muted);margin:0}.provider-head>span{font:8px var(--font-mono);padding:5px 7px;border-radius:12px;background:var(--color-warning-soft);color:var(--amber)}.provider-head>span.ready{background:var(--accent-soft);color:var(--green)}
  .accounts,.empty-account,.linked-account{border-top:1px solid var(--line);border-bottom:1px solid var(--line);background:var(--surface2)}.empty-account{padding:17px;color:var(--muted);font-size:9px}.accounts>div,.linked-account{min-height:58px;display:grid;grid-template-columns:29px 1fr auto;align-items:center;gap:9px;padding:10px 16px}.accounts img,.accounts i,.github-avatar{width:28px;height:28px;border-radius:50%;display:grid;place-items:center;background:var(--line);font:8px var(--font-mono);font-style:normal}.accounts span,.linked-account>span:nth-child(2){display:grid;gap:2px}.accounts strong,.linked-account strong{font-size:10px}.accounts small,.linked-account small{font:8px var(--font-mono);color:var(--muted)}.linked-account em{font-style:normal;font:8px var(--font-mono);color:var(--green)}.account-actions{display:flex;align-items:center;gap:9px}.accounts a{display:inline-flex;align-items:center;gap:5px;color:var(--accent);font-size:8px;font-weight:700;text-decoration:none}.account-actions button{border:0;background:transparent;color:var(--red);font-size:8px;font-weight:700;cursor:pointer}.github-avatar{background:var(--color-log-bg);color:var(--color-log-text)}
  .accounts small.permission-missing{color:var(--amber);font-weight:700}
  .providers footer{min-height:66px;padding:13px 16px;display:flex;align-items:center;justify-content:space-between;gap:10px}.providers footer>div{display:grid;min-width:0;gap:3px}.providers footer small{font-size:8px;color:var(--faint)}.providers footer code{font:8px var(--font-mono);color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:310px}.providers footer a,.providers footer button{height:31px;border-radius:5px;padding:0 10px;display:flex;align-items:center;border:1px solid var(--accent);background:var(--accent);color:var(--color-accent-ink);text-decoration:none;font-size:9px;font-weight:700;white-space:nowrap}.providers footer button:disabled{background:var(--surface2);border-color:var(--line);color:var(--muted)}.config{padding:9px 16px;background:var(--color-warning-soft);border-top:1px solid color-mix(in srgb,var(--amber) 26%,var(--line));color:var(--amber);font-size:8px;line-height:1.6}.config code{font:8px var(--font-mono)}.config.github-help{background:var(--accent-soft);border-color:color-mix(in srgb,var(--accent) 22%,var(--line));color:var(--muted)}.config a{color:var(--accent);font-weight:700}
  .github-source-actions{display:flex!important;grid-auto-flow:column;align-items:center;gap:7px!important}.github-source-actions button{background:var(--surface)!important;color:var(--accent)!important}
  .registry-panel{padding:18px}.section-head{margin-bottom:16px}.registry-grid{display:grid;grid-template-columns:340px 1fr;border:1px solid var(--line);border-radius:7px;overflow:hidden}.registry-grid form{padding:16px;background:var(--surface2);border-right:1px solid var(--line);display:grid;gap:11px}.form-title{display:flex;justify-content:space-between;align-items:center}.form-title b{font-size:11px}.form-title small{font-size:8px;color:var(--muted)}label{display:grid;gap:5px;font-size:8px;font-weight:700;color:var(--muted)}input{height:34px;border:1px solid var(--line2);background:var(--surface);color:var(--ink);border-radius:5px;padding:0 9px;font:9px var(--font-mono);outline:none}input:focus{border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-soft)}.split{display:grid;grid-template-columns:1fr 1fr;gap:9px}form>button{height:33px;border:1px solid var(--accent);background:var(--accent);color:#fff;border-radius:5px;font-size:9px;font-weight:700}.error{margin:0;color:var(--red);font-size:9px}
  .registry-list{min-width:0}.table-head,.registry-row{display:grid;grid-template-columns:1fr 180px 65px;align-items:center}.table-head{height:35px;padding:0 14px;border-bottom:1px solid var(--line);font:8px var(--font-mono);text-transform:uppercase;color:var(--faint)}.registry-row{min-height:57px;padding:0 14px;border-bottom:1px solid var(--line)}.registry-row:last-child{border:0}.registry-row>div{display:flex;align-items:center;gap:9px}.registry-row i{width:29px;height:29px;border-radius:6px;background:var(--accent-soft);color:var(--accent);display:grid;place-items:center;font-style:normal}.registry-row span{display:grid}.registry-row strong{font-size:10px}.registry-row small,.registry-row code{font:8px var(--font-mono);color:var(--muted)}.registry-row button{border:0;background:transparent;color:var(--red);font-size:8px}.empty{padding:36px;text-align:center;color:var(--muted);display:grid;gap:5px;font-size:9px}.empty b{color:var(--ink);font-size:10px}
  @media(max-width:1000px){.providers{grid-template-columns:1fr}.registry-grid{grid-template-columns:1fr}.registry-grid form{border-right:0;border-bottom:1px solid var(--line)}}@media(max-width:620px){.registry-grid{border:0}.split{grid-template-columns:1fr}.table-head{display:none}.registry-row{grid-template-columns:1fr auto}.registry-row code{display:none}}
</style>
