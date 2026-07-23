<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Status from '$lib/components/Status.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api } from '$lib/auth.js';

  const imageStages = [
    { id: 'prepare', label: 'Prepare release', detail: 'Validate source and credentials' },
    { id: 'pull', label: 'Pull image', detail: 'Stream layers from the registry' },
    { id: 'replace', label: 'Reserve release', detail: 'Keep the current container serving traffic' },
    { id: 'create', label: 'Create container', detail: 'Attach runtime settings and network' },
    { id: 'start', label: 'Start container', detail: 'Launch the new process' },
    { id: 'verify', label: 'Verify candidate', detail: 'Check readiness before receiving traffic' },
    { id: 'promote', label: 'Promote release', detail: 'Switch traffic and retire the previous container' }
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
  let cancelError = '';
  let cancelling = false;
  let cancelRequested = false;
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
    const complete = events.some((event) => event.type === 'complete') || (['verify', 'promote'].includes(definition.id) && data.deployment.status === 'healthy');
    const started = events.length > 0;
    const errorEvent = events.find((event) => event.type === 'error');
    const failed = Boolean(errorEvent) || (['failed', 'degraded'].includes(data.deployment.status) && started && !complete);
    const cancelled = data.deployment.status === 'cancelled' && started && !complete && !failed;
    const startedAt = events[0]?.createdAt;
    const endedAt = events[events.length - 1]?.createdAt;
    const duration = startedAt && endedAt ? Math.max(0, Math.round((new Date(endedAt) - new Date(startedAt)) / 1000)) : 0;
    const failureMessage = errorEvent?.message || (failed ? data.deployment.message : '');
    return { ...definition, state: failed ? 'failed' : complete ? 'complete' : cancelled ? 'cancelled' : started ? 'active' : 'pending', duration, failureMessage };
  });
  $: visibleEvents = data.events.filter((event) => !filter || event.message.toLowerCase().includes(filter.toLowerCase()) || event.stage.includes(filter.toLowerCase()));
  $: isLive = ['deploying', 'building'].includes(data.deployment.status);
  $: isCancelled = data.deployment.status === 'cancelled';
  $: isFailed = ['failed', 'degraded'].includes(data.deployment.status);
  $: wasInterrupted = isFailed && data.deployment.message?.toLowerCase().includes('interrupted');
  $: canCancel = isLive && !data.events.some((event) => event.stage === 'promote');

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
      if (!['deploying', 'building'].includes(payload.deployment.status)) cancelRequested = false;
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

  async function cancelDeployment() {
    if (!canCancel || cancelling || cancelRequested) return;
    cancelling = true;
    cancelError = '';
    try {
      const response = await api('/api/deployments/' + page.params.id + '/cancel', { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not stop deployment');
      cancelRequested = true;
      await loadDeployment();
    } catch (cause) {
      cancelRequested = false;
      cancelError = cause instanceof Error ? cause.message : 'Could not stop deployment';
    } finally {
      cancelling = false;
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
    copyTimer = setTimeout(() => (outputCopied = false), 1600);
  }
</script>

<Shell eyebrow="Deployment" title={data.deployment.id}>
  <svelte:fragment slot="actions">
    {#if canCancel}
      <button class="btn btn-danger" onclick={cancelDeployment} disabled={cancelling || cancelRequested}>
        <Icon name="stop" size={13} /> {cancelling || cancelRequested ? 'Stopping…' : 'Stop deployment'}
      </button>
    {/if}
    <a class="btn" href={'/projects/' + data.project.id}><Icon name="arrow-left" size={14} /> Back to project</a>
  </svelte:fragment>

  <section class="release-head" aria-busy={loading}>
    <div class="release-copy">
      <div class="status-line">
        <Status value={data.deployment.status} />
        <span class="badge" class:badge-info={isLive}><i></i>{isLive ? 'Live' : 'Recorded'}</span>
      </div>
      <h2>{data.deployment.message || 'Preparing deployment'}</h2>
      <p>{data.project.name} <i>·</i> {data.deployment.serviceName || data.project.name} <i>·</i> <code>{data.deployment.commit}</code></p>
    </div>
    <div class="timer">
      <small>{isLive ? 'Elapsed' : 'Duration'}</small>
      <strong>{wasInterrupted && elapsed === 0 ? 'Interrupted' : formatDuration(elapsed)}</strong>
    </div>
  </section>

  {#if error}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Live updates interrupted</strong><span>{error}. Retrying automatically…</span></div></div>
  {/if}
  {#if cancelError}
    <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Deployment was not stopped</strong><span>{cancelError}</span></div></div>
  {/if}

  <div class="workbench">
    <section class="panel pipeline" aria-label="Deployment pipeline">
      <header class="panel-header">
        <div>
          <span class="eyebrow">Execution path</span>
          <h2>Deployment progress</h2>
        </div>
        <span class="badge">{steps.filter((step) => step.state === 'complete').length}/{steps.length}</span>
      </header>
      <div class="stage-list">
        {#each steps as step, index}
          <article class:active={step.state === 'active'} class:complete={step.state === 'complete'} class:failed={step.state === 'failed'} class:cancelled={step.state === 'cancelled'}>
            <div class="rail">
              <span>
                {#if step.state === 'complete'}<Icon name="check" size={12} />
                {:else if step.state === 'failed'}<Icon name="x" size={12} />
                {:else if step.state === 'cancelled'}<Icon name="stop" size={10} />
                {:else}{index + 1}{/if}
              </span>
              <i></i>
            </div>
            <div class="stage-copy">
              <strong>{step.label}</strong>
              <small>{step.state === 'active' ? data.events.filter((event) => event.stage === step.id).at(-1)?.message : step.state === 'failed' ? step.failureMessage : step.detail}</small>
            </div>
            <div class="stage-state">
              {#if step.state === 'active'}<i class="spinner"></i><b>Running</b>
              {:else if step.state === 'complete'}<b>{step.duration ? step.duration + 's' : 'Done'}</b>
              {:else if step.state === 'failed'}<b>Failed</b>
              {:else if step.state === 'cancelled'}<b>Stopped</b>
              {:else}<b>Waiting</b>{/if}
            </div>
          </article>
        {/each}
      </div>
    </section>

    <aside class="panel release-meta">
      <header class="panel-header">
        <div>
          <span class="eyebrow">Release</span>
          <h2>Runtime metadata</h2>
        </div>
      </header>
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
      <div class="terminal-title">
        <span class="lights"><i></i><i></i><i></i></span>
        <strong>Deployment output</strong>
        <em class:live={isLive} class:cancelled={isCancelled} class:failed={isFailed}>{isLive ? 'streaming' : isCancelled ? 'stopped' : isFailed ? 'failed' : 'complete'}</em>
      </div>
      <div class="terminal-actions">
        <button class="terminal-copy" class:copied={outputCopied} onclick={copyVisibleOutput} disabled={visibleEvents.length === 0} aria-live="polite">
          <Icon name={outputCopied ? 'check' : 'copy'} size={12} />{outputCopied ? 'Copied' : 'Copy output'}
        </button>
        <label><input type="checkbox" bind:checked={autoScroll} /> Auto-scroll</label>
      </div>
    </header>
    <div class="terminal-body" bind:this={terminalBody}>
      {#if visibleEvents.length === 0}
        <div class="empty-output">
          {#if isLive || loading}<i class="spinner"></i>{/if}
          <span>{loading ? 'Connecting to deployment stream…' : isCancelled ? 'Deployment stopped before runtime output.' : isFailed ? data.deployment.message : 'Waiting for the first runtime event…'}</span>
        </div>
      {:else}
        {#each visibleEvents as event}
          <div class="log-line" class:error={event.type === 'error'} class:success={event.type === 'complete'} class:command={event.type === 'start'} class:cancelled={event.type === 'cancelled'}>
            <time>{formatTime(event.createdAt)}</time>
            <span class="stage-tag">{event.stage}</span>
            <i class="log-glyph">{event.type === 'start' ? '›' : event.type === 'complete' ? '✓' : event.type === 'error' ? '×' : event.type === 'cancelled' ? '■' : '·'}</i>
            <code>{event.message}</code>
          </div>
        {/each}
      {/if}
    </div>
    <footer>
      <Icon name="search" size={13} />
      <input aria-label="Filter deployment output" bind:value={filter} placeholder="Filter output by layer, stage, or message…" />
      <code>{visibleEvents.length} events</code>
    </footer>
  </section>
</Shell>

<style>
  .release-head {
    margin-bottom: var(--space-4);
    padding: var(--space-5);
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: var(--space-5);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .release-copy {
    min-width: 0;
  }
  .status-line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .release-copy h2 {
    margin: var(--space-3) 0 var(--space-1);
    font-size: var(--text-xl);
    font-weight: 700;
    letter-spacing: -0.02em;
  }
  .release-copy p {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .release-copy p i {
    margin: 0 var(--space-1);
    font-style: normal;
  }
  .release-copy code {
    font-size: var(--text-sm);
  }
  .timer {
    flex: 0 0 auto;
    text-align: right;
  }
  .timer small {
    display: block;
    color: var(--color-muted);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .timer strong {
    font: 600 var(--text-xl) var(--font-mono);
  }

  .workbench {
    margin-bottom: var(--space-4);
    display: grid;
    grid-template-columns: minmax(0, 1fr) 300px;
    gap: var(--space-4);
    align-items: start;
  }
  .stage-list {
    padding: var(--space-2) 0;
  }
  .stage-list article {
    min-height: 62px;
    padding: 0 var(--space-5);
    display: grid;
    grid-template-columns: 40px minmax(0, 1fr) 84px;
    align-items: center;
    position: relative;
  }
  .stage-list article.active {
    background: linear-gradient(90deg, var(--color-accent-soft), transparent 72%);
  }
  .rail {
    align-self: stretch;
    display: flex;
    align-items: center;
    position: relative;
  }
  .rail > span {
    position: relative;
    z-index: 1;
    width: 26px;
    height: 26px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule-strong);
    border-radius: 50%;
    background: var(--color-paper-raised);
    color: var(--color-muted);
    font: 600 var(--text-2xs) var(--font-mono);
  }
  .rail > i {
    position: absolute;
    left: 12px;
    top: 0;
    bottom: 0;
    width: 1px;
    background: var(--color-rule);
  }
  .stage-list article:first-child .rail > i {
    top: 50%;
  }
  .stage-list article:last-child .rail > i {
    bottom: 50%;
  }
  .complete .rail > span {
    border-color: var(--color-success);
    background: var(--color-success);
    color: #fff;
  }
  .active .rail > span {
    border-color: var(--color-info);
    color: var(--color-info);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-info) 14%, transparent);
  }
  .failed .rail > span {
    border-color: var(--color-danger);
    background: var(--color-danger);
    color: #fff;
  }
  .cancelled .rail > span {
    border-color: var(--color-warning);
    background: var(--color-warning);
    color: #fff;
  }
  .complete .rail > i {
    background: var(--color-success);
  }
  .stage-copy {
    min-width: 0;
    display: grid;
    gap: 2px;
  }
  .stage-copy strong {
    font-size: var(--text-sm);
  }
  .stage-copy small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .stage-state {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 7px;
  }
  .stage-state b {
    color: var(--color-muted);
    font: 600 var(--text-2xs) var(--font-mono);
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }
  .active .stage-state b {
    color: var(--color-info);
  }
  .complete .stage-state b {
    color: var(--color-success);
  }
  .failed .stage-state b {
    color: var(--color-danger);
  }
  .cancelled .stage-state b {
    color: var(--color-warning);
  }
  .stage-state .spinner {
    width: 12px;
    height: 12px;
    border-color: color-mix(in srgb, var(--color-info) 26%, transparent);
    border-top-color: var(--color-info);
  }

  .release-meta dl {
    margin: 0;
    padding: var(--space-2) var(--space-5);
  }
  .release-meta dl > div {
    min-height: 46px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
  }
  .release-meta dl > div:last-child {
    border-bottom: 0;
  }
  .release-meta dt {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .release-meta dd {
    margin: 0;
    max-width: 170px;
    overflow: hidden;
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .terminal {
    overflow: hidden;
    border: 1px solid var(--color-log-rule);
    border-radius: var(--radius-lg);
    background: var(--color-log-bg);
    color: var(--color-log-text);
  }
  .terminal > header {
    min-height: 48px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-log-rule);
    background: var(--color-log-surface);
  }
  .terminal-title {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .lights {
    display: flex;
    gap: 5px;
  }
  .lights i {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-log-rule);
  }
  .lights i:first-child {
    background: #d9665d;
  }
  .lights i:nth-child(2) {
    background: #d9aa4d;
  }
  .lights i:last-child {
    background: #5fb879;
  }
  .terminal-title strong {
    font: 500 var(--text-xs) var(--font-mono);
    white-space: nowrap;
  }
  .terminal-title em {
    padding: 2px 7px;
    border-radius: var(--radius-xs);
    background: var(--color-log-rule);
    color: var(--color-log-muted);
    font: 500 var(--text-2xs) var(--font-mono);
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }
  .terminal-title em.live {
    background: color-mix(in srgb, var(--color-success) 20%, transparent);
    color: var(--color-success);
  }
  .terminal-title em.cancelled {
    background: color-mix(in srgb, var(--color-warning) 20%, transparent);
    color: var(--color-warning);
  }
  .terminal-title em.failed {
    background: color-mix(in srgb, var(--color-danger) 20%, transparent);
    color: var(--color-danger);
  }
  .terminal label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--color-log-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .terminal label input {
    accent-color: var(--color-accent);
  }
  .terminal-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .terminal-copy {
    height: 28px;
    padding: 0 var(--space-2);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    border: 1px solid var(--color-log-rule);
    border-radius: var(--radius-xs);
    background: transparent;
    color: var(--color-log-muted);
    font: 500 var(--text-2xs) var(--font-mono);
    cursor: pointer;
  }
  .terminal-copy:hover:not(:disabled) {
    border-color: var(--color-log-muted);
    color: var(--color-log-text);
  }
  .terminal-copy.copied {
    border-color: color-mix(in srgb, var(--color-success) 45%, transparent);
    color: var(--color-success);
  }
  .terminal-copy:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .terminal-body {
    height: 320px;
    padding: var(--space-3) 0;
    overflow: auto;
    scrollbar-color: var(--color-log-rule) transparent;
  }
  .log-line {
    min-height: 26px;
    padding: 3px var(--space-4);
    display: grid;
    grid-template-columns: 70px 68px 14px minmax(0, 1fr);
    align-items: start;
    font: var(--text-xs)/1.7 var(--font-mono);
  }
  .log-line:hover {
    background: color-mix(in srgb, var(--color-log-surface) 70%, transparent);
  }
  .log-line time {
    color: var(--color-log-muted);
  }
  .stage-tag {
    width: max-content;
    max-width: 62px;
    padding: 1px 5px;
    overflow: hidden;
    border: 1px solid var(--color-log-rule);
    border-radius: var(--radius-xs);
    color: var(--color-log-muted);
    font-size: 9px;
    letter-spacing: 0.05em;
    text-overflow: ellipsis;
    text-transform: uppercase;
  }
  .log-glyph {
    color: var(--color-log-muted);
    font-style: normal;
  }
  .log-line > code {
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    color: var(--color-log-text);
  }
  .log-line.command > .log-glyph,
  .log-line.command > code {
    color: var(--color-info);
  }
  .log-line.success > .log-glyph,
  .log-line.success > code {
    color: var(--color-success);
  }
  .log-line.error > .log-glyph,
  .log-line.error > code {
    color: var(--color-danger);
  }
  .log-line.cancelled > .log-glyph,
  .log-line.cancelled > code {
    color: var(--color-warning);
  }
  .empty-output {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    color: var(--color-log-muted);
    font-size: var(--text-sm);
  }
  .empty-output .spinner {
    border-color: var(--color-log-rule);
    border-top-color: var(--color-accent);
  }
  .terminal > footer {
    height: 42px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border-top: 1px solid var(--color-log-rule);
    background: var(--color-log-surface);
    color: var(--color-log-muted);
  }
  .terminal > footer input {
    min-width: 0;
    flex: 1;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--color-log-text);
    font: var(--text-xs) var(--font-mono);
  }
  .terminal > footer input::placeholder {
    color: var(--color-log-muted);
  }
  .terminal > footer code {
    color: var(--color-log-muted);
    font-size: var(--text-2xs);
    white-space: nowrap;
  }

  @media (max-width: 56rem) {
    .workbench {
      grid-template-columns: 1fr;
    }
    .release-meta {
      display: none;
    }
  }
  @media (max-width: 42rem) {
    .release-head {
      align-items: flex-start;
      flex-direction: column;
    }
    .timer {
      text-align: left;
    }
    .stage-list article {
      padding: 0 var(--space-3);
    }
    .log-line {
      grid-template-columns: 58px 14px minmax(0, 1fr);
    }
    .stage-tag {
      display: none;
    }
    .terminal > header {
      align-items: flex-start;
      flex-direction: column;
      padding-block: var(--space-2);
      gap: var(--space-2);
    }
  }
</style>
