<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import { api } from '$lib/auth.js';

  const imageStages = [
    { id: 'prepare', label: 'Prepare release', detail: 'Validate source and credentials' },
    { id: 'pull', label: 'Pull image', detail: 'Stream layers from the registry' },
    { id: 'replace', label: 'Replace container', detail: 'Clear the previous runtime slot' },
    { id: 'create', label: 'Create container', detail: 'Attach runtime settings and network' },
    { id: 'start', label: 'Start container', detail: 'Launch the new process' },
    { id: 'verify', label: 'Verify runtime', detail: 'Inspect container state' }
  ];
  const repositoryStages = [
    { id: 'prepare', label: 'Prepare release', detail: 'Validate source and credentials' },
    { id: 'clone', label: 'Clone repository', detail: 'Stream checkout progress from Git' },
    { id: 'build', label: 'Build image', detail: 'Create the application image' },
    ...imageStages.slice(2)
  ];

  let data = { deployment: { id: 'Loading', status: 'deploying', duration: 0 }, project: { name: 'Loading…', sourceType: 'image' }, events: [] };
  let loading = true;
  let error = '';
  let filter = '';
  let autoScroll = true;
  let terminalBody;
  let pollTimer;
  let clockTimer;
  let copyTimer;
  let outputCopied = false;
  let elapsed = 0;
  let lastEventID = 0;

  $: stageDefinitions = data.deployment?.serviceId || data.project?.sourceType === 'image' || data.project?.sourceType === 'empty' ? imageStages : repositoryStages;
  $: steps = stageDefinitions.map((definition) => {
    const events = data.events.filter((event) => event.stage === definition.id);
    const failed = events.some((event) => event.type === 'error');
    const complete = events.some((event) => event.type === 'complete') || (definition.id === 'verify' && data.deployment.status === 'healthy');
    const started = events.length > 0;
    const startedAt = events[0]?.createdAt;
    const endedAt = events[events.length - 1]?.createdAt;
    const duration = startedAt && endedAt ? Math.max(0, Math.round((new Date(endedAt) - new Date(startedAt)) / 1000)) : 0;
    return { ...definition, state: failed ? 'failed' : complete ? 'complete' : started ? 'active' : 'pending', duration };
  });
  $: visibleEvents = data.events.filter((event) => !filter || event.message.toLowerCase().includes(filter.toLowerCase()) || event.stage.includes(filter.toLowerCase()));
  $: isLive = ['deploying', 'building'].includes(data.deployment.status);

  onMount(() => {
    loadDeployment();
    clockTimer = setInterval(updateElapsed, 1000);
  });

  onDestroy(() => {
    clearTimeout(pollTimer);
    clearInterval(clockTimer);
    clearTimeout(copyTimer);
  });

  function updateElapsed() {
    if (!data.deployment.createdAt) return;
    elapsed = isLive ? Math.max(0, Math.floor((Date.now() - new Date(data.deployment.createdAt).getTime()) / 1000)) : data.deployment.duration;
  }

  async function loadDeployment() {
    try {
      const response = await api('/api/deployments/' + page.params.id);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load deployment');
      const incomingLastID = payload.events.at(-1)?.id || 0;
      const hasNewEvents = incomingLastID !== lastEventID;
      data = payload;
      lastEventID = incomingLastID;
      loading = false;
      error = '';
      updateElapsed();
      if (hasNewEvents && autoScroll) {
        await tick();
        if (terminalBody) terminalBody.scrollTop = terminalBody.scrollHeight;
      }
      clearTimeout(pollTimer);
      if (['deploying', 'building'].includes(payload.deployment.status)) pollTimer = setTimeout(loadDeployment, 700);
    } catch (cause) {
      loading = false;
      error = cause instanceof Error ? cause.message : 'Could not load deployment';
      clearTimeout(pollTimer);
      pollTimer = setTimeout(loadDeployment, 2500);
    }
  }

  function formatTime(value) {
    if (!value) return '--:--:--';
    return new Date(value).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function formatDuration(value) {
    if (value < 60) return value + 's';
    return Math.floor(value / 60) + 'm ' + (value % 60) + 's';
  }

  async function writeClipboard(value) {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return;
    }
    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    textarea.remove();
  }

  async function copyVisibleOutput() {
    if (visibleEvents.length === 0) return;
    const output = visibleEvents.map((event) => `${formatTime(event.createdAt)} [${event.stage.toUpperCase()}] ${event.message}`).join('\n');
    await writeClipboard(output);
    outputCopied = true;
    clearTimeout(copyTimer);
    copyTimer = setTimeout(() => outputCopied = false, 1600);
  }
</script>

<Shell eyebrow="Deployment" title={data.deployment.id}>
  <section class="release-head" aria-busy={loading}>
    <div class="release-copy">
      <div class="status-line"><Status value={data.deployment.status} /><span class:live={isLive}>{isLive ? 'LIVE' : 'RECORDED'}</span></div>
      <h2>{data.deployment.message || 'Preparing deployment'}</h2>
      <p>{data.project.name} <i>·</i> {data.deployment.serviceName || data.project.name} <i>·</i> <code>{data.deployment.commit}</code></p>
    </div>
    <div class="release-actions">
      <div class="timer"><small>{isLive ? 'ELAPSED' : 'DURATION'}</small><strong>{formatDuration(elapsed)}</strong></div>
      <a href={'/projects/' + data.project.id}>Back to project</a>
    </div>
  </section>

  {#if error}<div class="error-banner"><strong>Live updates interrupted</strong><span>{error}. Retrying automatically…</span></div>{/if}

  <div class="workbench">
    <section class="pipeline" aria-label="Deployment pipeline">
      <header><div><span>Execution path</span><h3>Deployment progress</h3></div><code>{steps.filter((step) => step.state === 'complete').length}/{steps.length}</code></header>
      <div class="stage-list">
        {#each steps as step, index}
          <article class:active={step.state === 'active'} class:complete={step.state === 'complete'} class:failed={step.state === 'failed'}>
            <div class="rail"><span>{step.state === 'complete' ? '✓' : step.state === 'failed' ? '!' : index + 1}</span><i></i></div>
            <div class="stage-copy"><strong>{step.label}</strong><small>{step.state === 'active' ? data.events.filter((event) => event.stage === step.id).at(-1)?.message : step.detail}</small></div>
            <div class="stage-state">
              {#if step.state === 'active'}<i class="spinner"></i><b>Running</b>
              {:else if step.state === 'complete'}<b>{step.duration ? step.duration + 's' : 'Done'}</b>
              {:else if step.state === 'failed'}<b>Failed</b>
              {:else}<b>Waiting</b>{/if}
            </div>
          </article>
        {/each}
      </div>
    </section>

    <aside class="release-meta">
      <header><span>Release</span><h3>Runtime metadata</h3></header>
      <dl>
        <div><dt>Project</dt><dd>{data.project.name}</dd></div>
        <div><dt>Service</dt><dd>{data.deployment.serviceName || data.project.name}</dd></div>
        <div><dt>Source</dt><dd>{data.deployment.serviceId || data.project.sourceType === 'image' ? 'Container image' : 'Git repository'}</dd></div>
        <div><dt>Branch</dt><dd>{data.project.branch || '—'}</dd></div>
        <div><dt>Runtime</dt><dd>Docker Engine</dd></div>
        <div><dt>Network</dt><dd>selfhost-proxy</dd></div>
        <div><dt>Started</dt><dd>{formatTime(data.deployment.createdAt)}</dd></div>
      </dl>
    </aside>
  </div>

  <section class="terminal" aria-label="Live deployment output">
    <header>
      <div class="terminal-title"><span class="lights"><i></i><i></i><i></i></span><strong>Deployment output</strong><em class:live={isLive}>{isLive ? 'streaming' : 'complete'}</em></div>
      <div class="terminal-actions">
        <button class="terminal-copy" class:copied={outputCopied} onclick={copyVisibleOutput} disabled={visibleEvents.length === 0} aria-live="polite">{outputCopied ? 'Copied ✓' : 'Copy output'}</button>
        <label><input type="checkbox" bind:checked={autoScroll}/> Auto-scroll</label>
      </div>
    </header>
    <div class="terminal-body" bind:this={terminalBody}>
      {#if visibleEvents.length === 0}
        <div class="empty-output"><i class="spinner"></i><span>{loading ? 'Connecting to deployment stream…' : 'Waiting for the first runtime event…'}</span></div>
      {:else}
        {#each visibleEvents as event}
          <div class="log-line" class:error={event.type === 'error'} class:success={event.type === 'complete'} class:command={event.type === 'start'}>
            <time>{formatTime(event.createdAt)}</time><span class="stage-tag">{event.stage}</span><i>{event.type === 'start' ? '›' : event.type === 'complete' ? '✓' : event.type === 'error' ? '×' : '·'}</i><code>{event.message}</code>
          </div>
        {/each}
      {/if}
    </div>
    <footer><span>⌕</span><input aria-label="Filter deployment output" bind:value={filter} placeholder="Filter output by layer, stage, or message…"/><code>{visibleEvents.length} events</code></footer>
  </section>
</Shell>

<style>
  .release-head{display:flex;justify-content:space-between;align-items:flex-end;gap:var(--space-6);padding:0 0 var(--space-6);border-bottom:1px solid var(--color-rule);margin-bottom:var(--space-5)}
  .status-line{display:flex;align-items:center;gap:var(--space-3)}.status-line>span{padding:3px 6px;border:1px solid var(--color-rule);border-radius:4px;color:var(--color-muted);font:700 9px var(--font-mono);letter-spacing:.1em}.status-line>span.live{color:var(--color-warning);border-color:color-mix(in oklch,var(--color-warning) 38%,var(--color-rule));background:color-mix(in oklch,var(--color-warning) 9%,transparent)}
  .release-copy h2{margin:var(--space-3) 0 var(--space-2);font-size:21px;letter-spacing:-.025em}.release-copy p{margin:0;color:var(--color-muted);font-size:12px}.release-copy p i{margin:0 var(--space-2);font-style:normal}.release-copy code{font-size:11px}
  .release-actions{display:flex;align-items:center;gap:var(--space-4)}.release-actions>a{height:36px;display:flex;align-items:center;padding:0 var(--space-4);border:1px solid var(--color-rule-strong);border-radius:var(--radius-sm);color:var(--color-ink);text-decoration:none;font-size:11px;font-weight:700;background:var(--color-paper-raised)}.release-actions>a:hover{border-color:var(--color-accent);color:var(--color-accent)}
  .timer{min-width:76px;text-align:right}.timer small{display:block;color:var(--color-muted);font:700 9px var(--font-mono);letter-spacing:.1em}.timer strong{font:500 18px var(--font-mono)}
  .error-banner{display:flex;gap:var(--space-3);padding:var(--space-3) var(--space-4);margin-bottom:var(--space-4);border:1px solid color-mix(in oklch,var(--color-danger) 42%,var(--color-rule));border-radius:var(--radius-sm);background:color-mix(in oklch,var(--color-danger) 8%,var(--color-paper-raised));font-size:11px}.error-banner strong{color:var(--color-danger)}.error-banner span{color:var(--color-muted)}
  .workbench{display:grid;grid-template-columns:minmax(0,1fr) 300px;gap:var(--space-4);margin-bottom:var(--space-4)}
  .pipeline,.release-meta{border:1px solid var(--color-rule);border-radius:var(--radius-lg);background:var(--color-paper-raised);box-shadow:var(--shadow-whisper);overflow:hidden}.pipeline>header,.release-meta>header{min-height:68px;padding:0 var(--space-5);display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid var(--color-rule)}.pipeline header span,.release-meta header span{display:block;color:var(--color-muted);font:700 9px var(--font-mono);text-transform:uppercase;letter-spacing:.1em}.pipeline h3,.release-meta h3{margin:4px 0 0;font-size:14px}.pipeline>header>code{color:var(--color-muted);font-size:11px}
  .stage-list{padding:var(--space-2) 0}.stage-list article{min-height:66px;display:grid;grid-template-columns:42px minmax(0,1fr) 76px;align-items:center;padding:0 var(--space-5);position:relative}.stage-list article.active{background:linear-gradient(90deg,var(--color-accent-soft),transparent 72%)}.rail{align-self:stretch;display:flex;align-items:center;position:relative}.rail>span{position:relative;z-index:1;width:26px;height:26px;border-radius:50%;display:grid;place-items:center;border:1px solid var(--color-rule-strong);background:var(--color-paper-raised);color:var(--color-muted);font:700 10px var(--font-mono)}.rail>i{position:absolute;left:12px;top:0;bottom:0;width:1px;background:var(--color-rule)}.stage-list article:first-child .rail>i{top:50%}.stage-list article:last-child .rail>i{bottom:50%}.complete .rail>span{border-color:var(--color-accent);background:var(--color-accent);color:var(--color-accent-ink)}.active .rail>span{border-color:var(--color-warning);color:var(--color-warning);box-shadow:0 0 0 4px color-mix(in oklch,var(--color-warning) 12%,transparent)}.failed .rail>span{border-color:var(--color-danger);background:var(--color-danger);color:white}.complete .rail>i{background:var(--color-accent)}
  .stage-copy{min-width:0;display:grid;gap:4px}.stage-copy strong{font-size:11px}.stage-copy small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--color-muted);font:10px var(--font-mono)}.stage-state{display:flex;justify-content:flex-end;align-items:center;gap:7px}.stage-state b{color:var(--color-muted);font:700 9px var(--font-mono);text-transform:uppercase;letter-spacing:.04em}.active .stage-state b{color:var(--color-warning)}.complete .stage-state b{color:var(--color-accent)}.failed .stage-state b{color:var(--color-danger)}
  .spinner{width:11px;height:11px;border:2px solid color-mix(in oklch,var(--color-warning) 26%,transparent);border-top-color:var(--color-warning);border-radius:50%;animation:spin .75s linear infinite}
  .release-meta dl{margin:0;padding:var(--space-2) var(--space-5)}.release-meta dl>div{min-height:47px;display:flex;align-items:center;justify-content:space-between;gap:var(--space-3);border-bottom:1px solid var(--color-rule)}.release-meta dl>div:last-child{border:0}.release-meta dt{color:var(--color-muted);font-size:10px}.release-meta dd{margin:0;max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;text-align:right;font:10px var(--font-mono)}
  .terminal{border:1px solid var(--color-log-rule);border-radius:var(--radius-lg);background:var(--color-log-bg);color:var(--color-log-text);overflow:hidden;box-shadow:0 12px 30px oklch(10% .01 150/.12)}.terminal>header{height:50px;display:flex;align-items:center;justify-content:space-between;padding:0 var(--space-4);border-bottom:1px solid var(--color-log-rule);background:var(--color-log-surface)}.terminal-title{display:flex;align-items:center;gap:var(--space-3)}.lights{display:flex;gap:5px}.lights i{width:7px;height:7px;border-radius:50%;background:#55605a}.lights i:first-child{background:#d9665d}.lights i:nth-child(2){background:#d9aa4d}.lights i:last-child{background:#5fb879}.terminal-title strong{font:500 10px var(--font-mono)}.terminal-title em{padding:3px 6px;border-radius:4px;background:var(--color-log-rule);color:var(--color-log-muted);font:normal 8px var(--font-mono);text-transform:uppercase;letter-spacing:.08em}.terminal-title em.live{color:#7ee29a;background:#193823}.terminal label{display:flex;align-items:center;gap:7px;color:var(--color-log-muted);font:9px var(--font-mono)}.terminal label input{accent-color:var(--color-accent)}
  .terminal-actions{display:flex;align-items:center;gap:var(--space-3)}.terminal-copy{height:28px;padding:0 9px;border:1px solid var(--color-log-rule);border-radius:4px;background:transparent;color:var(--color-log-muted);font:500 9px var(--font-mono);cursor:pointer}.terminal-copy:hover:not(:disabled){border-color:#68736b;color:var(--color-log-text)}.terminal-copy.copied{border-color:#356847;color:#7ee29a}.terminal-copy:disabled{opacity:.45;cursor:not-allowed}
  .terminal-body{height:310px;overflow:auto;padding:var(--space-3) 0;scrollbar-color:var(--color-log-rule) transparent}.log-line{min-height:26px;display:grid;grid-template-columns:70px 68px 14px minmax(0,1fr);align-items:start;padding:4px var(--space-4);font:10px/1.7 var(--font-mono)}.log-line:hover{background:color-mix(in oklch,var(--color-log-surface) 70%,transparent)}.log-line time{color:var(--color-log-muted)}.stage-tag{width:max-content;max-width:62px;overflow:hidden;text-overflow:ellipsis;padding:1px 5px;border:1px solid var(--color-log-rule);border-radius:3px;color:#8c9a90;text-transform:uppercase;font-size:8px;letter-spacing:.05em}.log-line>i{color:#68736b;font-style:normal}.log-line>code{white-space:pre-wrap;overflow-wrap:anywhere;color:var(--color-log-text)}.log-line.command>i,.log-line.command>code{color:#8bc7ff}.log-line.success>i,.log-line.success>code{color:#7ee29a}.log-line.error>i,.log-line.error>code{color:#ff8178}.empty-output{height:100%;display:flex;align-items:center;justify-content:center;gap:var(--space-3);color:var(--color-log-muted);font:10px var(--font-mono)}
  .terminal>footer{height:43px;display:flex;align-items:center;gap:var(--space-3);padding:0 var(--space-4);border-top:1px solid var(--color-log-rule);background:var(--color-log-surface);color:#7ee29a}.terminal>footer input{min-width:0;flex:1;border:0;outline:0;background:transparent;color:var(--color-log-text);font:10px var(--font-mono)}.terminal>footer input::placeholder{color:var(--color-log-muted)}.terminal>footer code{color:var(--color-log-muted);font-size:9px}
  @keyframes spin{to{transform:rotate(360deg)}}
  @media(max-width:900px){.workbench{grid-template-columns:1fr}.release-meta{display:none}}
  @media(max-width:680px){.release-head{align-items:flex-start;flex-direction:column}.release-actions{width:100%;justify-content:space-between}.stage-list article{padding:0 var(--space-3)}.log-line{grid-template-columns:58px 14px minmax(0,1fr)}.stage-tag{display:none}}
  @media(prefers-reduced-motion:reduce){.spinner{animation:none}}
</style>
