<script>
  import {onMount} from 'svelte';
  import {page} from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import {api} from '$lib/auth.js';
  let projects=[];
  let open=false,busy=false,error='';
  let form={name:'',sourceType:'empty'};
  onMount(async()=>{open=page.url.searchParams.get('new')==='1';const response=await api('/api/projects');projects=await response.json()});
  async function create(){busy=true;error='';const r=await api('/api/projects',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(form)});const data=await r.json();if(!r.ok){error=data.error;busy=false;return}location.href='/projects/'+data.id}
  const sourceLabel=(p)=>p.sourceType==='empty'?'No services yet':p.sourceType==='image'?p.imageUrl:p.repository;
</script>

<Shell eyebrow="Workspace" title="Projects">
  <section class="intro"><div><small>Applications</small><h2>Projects</h2><p>Create an empty project, then compose it from independent services and databases.</p></div><button onclick={()=>open=true}><Icon name="plus" size={14}/> New project</button></section>
  {#if open}
    <section class="creator">
      <div class="creator-head"><div><small>New project</small><h3>Create an empty workspace</h3></div><button onclick={()=>open=false} aria-label="Close project form">×</button></div>
      {#if error}<p class="error">{error}</p>{/if}
      <form onsubmit={(event)=>{event.preventDefault();create()}}>
        <label>Project name<input bind:value={form.name} placeholder="customer-api" required /></label>
        <div class="empty-project-note"><Icon name="grid" size={18}/><div><b>Every application is a service</b><span>After creation, add the frontend, API, Adminer, workers, and databases independently from the project overview.</span></div></div>
        <div class="formactions"><button type="button" onclick={()=>open=false}>Cancel</button><button class="primary" disabled={busy}><Icon name="plus" size={13}/>{busy?'Creating…':'Create project'}</button></div>
      </form>
    </section>
  {/if}
  <section class="list">
    <div class="table-head"><span>Project</span><span>Status</span><span>Source</span><span>Updated</span><span></span></div>
    {#if projects.length===0}<div class="empty"><Icon name="box" size={25}/><h3>No projects yet</h3><p>Create a workspace, then add only the services it needs.</p><button onclick={()=>open=true}>Create first project</button></div>{:else}{#each projects as project}<a href={'/projects/'+project.id}><span class="icon"><Icon name={project.sourceType==='empty'?'grid':project.sourceType==='image'?'box':'git'} size={16}/></span><div><strong>{project.name}</strong><small>{project.domain||'No public domain'}</small></div><Status value={project.status}/><code>{sourceLabel(project)}</code><time>{new Date(project.updatedAt).toLocaleDateString()}</time><b>→</b></a>{/each}{/if}
  </section>
</Shell>

<style>
  .intro{display:flex;justify-content:space-between;align-items:center;margin-bottom:15px}.intro small{font:8px var(--font-mono);color:var(--accent);text-transform:uppercase;letter-spacing:.1em}.intro h2{font-size:16px;margin:4px 0}.intro p{font-size:10px;color:var(--muted);margin:0}.intro button,.empty button{height:34px;border:1px solid var(--accent);background:var(--accent);color:var(--color-accent-ink);border-radius:6px;padding:0 12px;font-weight:700;font-size:9px}
  .creator,.list{border:1px solid var(--line);background:var(--surface);border-radius:8px;margin-bottom:15px;overflow:hidden}.creator{padding:18px}.creator-head{display:flex;justify-content:space-between}.creator-head>div small{font:8px var(--font-mono);color:var(--accent);text-transform:uppercase}.creator-head h3{font-size:15px;margin:5px 0 14px}.creator-head>button{width:28px;height:28px;border:1px solid var(--line);border-radius:5px;background:var(--surface2);color:var(--muted);cursor:pointer}
  form{display:grid;grid-template-columns:minmax(220px,.65fr) minmax(300px,1.35fr);gap:11px}label{font-size:8px;font-weight:700;color:var(--muted);display:grid;gap:6px}input{height:36px;border:1px solid var(--line2);border-radius:5px;background:var(--surface);color:var(--ink);padding:0 9px;outline:none;font:9px var(--font-mono)}input:focus{border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-soft)}.formactions{grid-column:1/-1;display:flex;justify-content:flex-end;gap:7px;margin-top:4px}.formactions button{height:32px;border:1px solid var(--line);border-radius:5px;background:var(--surface2);color:var(--ink);padding:0 11px;font-size:9px;cursor:pointer}.formactions .primary{background:var(--accent);color:var(--color-accent-ink);border-color:var(--accent)}.formactions .primary:disabled{opacity:.45}.error{grid-column:1/-1;color:var(--red);font-size:9px;background:var(--color-danger-soft);border:1px solid color-mix(in srgb,var(--red) 28%,var(--line));padding:8px 10px;border-radius:5px}
  .table-head{height:36px;display:grid;grid-template-columns:1.2fr 90px 1.2fr 90px 20px;gap:10px;align-items:center;padding:0 17px;background:var(--surface2);border-bottom:1px solid var(--line);font:8px var(--font-mono);text-transform:uppercase;color:var(--faint)}.table-head span:first-child{padding-left:42px}.list>a{min-height:63px;display:grid;grid-template-columns:35px 1.2fr 90px 1.2fr 90px 20px;align-items:center;gap:10px;padding:0 17px;border-bottom:1px solid var(--line);text-decoration:none;color:inherit}.list>a:hover{background:var(--surface2)}.icon{width:30px;height:30px;border:1px solid var(--line);border-radius:6px;display:grid;place-items:center;color:var(--muted);background:var(--surface2)}.list a div{display:grid;gap:3px}.list strong{font-size:10px}.list small,.list code,.list time{font:8px var(--font-mono);color:var(--muted)}.list code{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.empty{text-align:center;padding:60px 20px;color:var(--muted)}.empty h3{color:var(--ink);font-size:13px;margin:12px 0 5px}.empty p{font-size:9px;margin:0 0 15px}
  .intro button,.empty button,.formactions button{display:inline-flex;align-items:center;justify-content:center;gap:7px}.empty-project-note{min-height:58px;padding:11px 12px;display:flex;align-items:center;gap:10px;border:1px solid var(--line);border-radius:6px;background:var(--surface2);color:var(--accent)}.empty-project-note div{display:grid;gap:3px}.empty-project-note b{font-size:9px;color:var(--ink)}.empty-project-note span{font-size:8px;color:var(--muted)}
  @media(max-width:1000px){.table-head{display:none}.list>a{grid-template-columns:35px 1fr 90px}.list a code,.list a time,.list a>b{display:none}}@media(max-width:600px){form{grid-template-columns:1fr}.empty-project-note{grid-row:2}.list>a{grid-template-columns:35px 1fr}}
</style>
