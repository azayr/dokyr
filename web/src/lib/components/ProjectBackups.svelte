<script>
  import { onDestroy, onMount } from 'svelte';
  import { api } from '$lib/auth.js';
  import Icon from './Icon.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  export let project;
  export let volumeCount = 0;

  const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
  const defaults = { configured: false, enabled: false, objectStorageId: '', frequency: 'daily', weekday: 0, hour: 2, minute: 0, timezone: 'UTC', retentionCount: 7, lastStatus: 'never' };
  let jobs = [];
  let destinations = [];
  let schedule = { ...defaults };
  let scheduleTime = '02:00';
  let selectedStorage = '';
  let loading = true;
  let creating = false;
  let saving = false;
  let error = '';
  let saved = false;
  let restoreTarget = null;
  let restoring = false;
  let restoreError = '';
  let logJob = null;
  let pollTimer;

  $: activeJobs = jobs.filter((job) => job.status === 'queued' || job.status === 'running');

  onMount(() => { load(); pollTimer = setInterval(() => { if (activeJobs.length) load(true); }, 2500); });
  onDestroy(() => clearInterval(pollTimer));

  async function load(silent = false) {
    if (!silent) loading = true;
    error = '';
    try {
      const response = await api(`/api/projects/${encodeURIComponent(project.id)}/backups`);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load project backups');
      jobs = payload.jobs || [];
      destinations = payload.destinations || [];
      const next = { ...defaults, ...(payload.schedule || {}) };
      if (!next.configured) next.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
      schedule = next;
      scheduleTime = `${String(schedule.hour).padStart(2, '0')}:${String(schedule.minute).padStart(2, '0')}`;
      if (!selectedStorage || !destinations.some((item) => item.id === selectedStorage)) selectedStorage = schedule.objectStorageId || destinations[0]?.id || '';
    } catch (cause) { error = cause instanceof Error ? cause.message : 'Could not load project backups'; }
    finally { loading = false; }
  }

  async function backupNow() {
    creating = true; error = '';
    try {
      const response = await api(`/api/projects/${encodeURIComponent(project.id)}/backups`, { method: 'POST', body: JSON.stringify({ objectStorageId: selectedStorage }) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not queue the project backup');
      jobs = [payload.job, ...jobs];
    } catch (cause) { error = cause instanceof Error ? cause.message : 'Could not queue the project backup'; }
    finally { creating = false; }
  }

  async function saveSchedule() {
    saving = true; saved = false; error = '';
    const [hour, minute] = scheduleTime.split(':').map(Number);
    try {
      const response = await api(`/api/projects/${encodeURIComponent(project.id)}/backups/schedule`, { method: 'PUT', body: JSON.stringify({ ...schedule, weekday: Number(schedule.weekday), retentionCount: Number(schedule.retentionCount), hour, minute }) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not save the backup schedule');
      schedule = { ...defaults, ...payload }; selectedStorage = schedule.objectStorageId; saved = true;
    } catch (cause) { error = cause instanceof Error ? cause.message : 'Could not save the backup schedule'; }
    finally { saving = false; }
  }

  async function restore() {
    restoring = true; restoreError = '';
    try {
      const response = await api(`/api/projects/${encodeURIComponent(project.id)}/backups/${encodeURIComponent(restoreTarget.id)}/restore`, { method: 'POST', body: JSON.stringify({ confirmation: `RESTORE ${project.name}` }) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not queue the project restore');
      jobs = [payload.job, ...jobs]; restoreTarget = null;
    } catch (cause) { restoreError = cause instanceof Error ? cause.message : 'Could not queue the project restore'; }
    finally { restoring = false; }
  }

  function date(value) { return value ? new Date(value).toLocaleString() : 'Never'; }
  function bytes(value) { if (!value) return '—'; const u = ['B','KB','MB','GB','TB']; const i = Math.min(Math.floor(Math.log(value) / Math.log(1024)), u.length - 1); return `${(value / Math.pow(1024, i)).toFixed(i && value / Math.pow(1024, i) < 100 ? 1 : 0)} ${u[i]}`; }
  function duration(job) {
    if (!job?.startedAt) return '—';
    const end = job.finishedAt ? new Date(job.finishedAt) : new Date();
    const seconds = Math.max(0, Math.round((end - new Date(job.startedAt)) / 1000));
    return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  }
  function closeLog() { logJob = null; }
</script>

<svelte:window onkeydown={(event) => { if (event.key === 'Escape' && logJob) closeLog(); }} />

{#if error}<div class="backup-alert"><Icon name="alert" size={15}/><span>{error}</span><button onclick={() => error = ''}>×</button></div>{/if}
{#if loading}
  <section class="backup-loading"><span class="spinner"></span><div><strong>Reading recovery points</strong><small>Checking this project’s backup schedule and archive history.</small></div></section>
{:else}
  <section class="backup-hero">
    <div class="hero-mark"><Icon name="hard-drive" size={21}/></div>
    <div><span>Project recovery</span><h3>Portable service & volume snapshots</h3><p>Every application definition, secret, domain, database service, and persistent Docker volume travels together in one restorable <code>.tar.gz</code>.</p></div>
    <dl><div><dt>Volumes</dt><dd>{volumeCount}</dd></div><div><dt>Available</dt><dd>{jobs.filter((j) => j.kind === 'backup' && j.status === 'succeeded' && j.objectKey).length}</dd></div><div><dt>Automation</dt><dd>{schedule.enabled ? 'On' : 'Off'}</dd></div></dl>
  </section>

  {#if destinations.length === 0}
    <section class="panel no-storage"><Icon name="hard-drive" size={20}/><div><strong>Connect an S3 bucket first</strong><p>Add an object-storage connection, then return here to create or schedule project backups.</p></div><a href="/object-storage">Open object storage →</a></section>
  {:else}
    <div class="backup-grid">
      <section class="panel backup-now">
        <header><div><span>On demand</span><h3>Back up {project.name} now</h3><p>The job runs in the background and briefly stops database containers while their volumes are captured.</p></div><b class:busy={activeJobs.length}>{activeJobs.length ? `${activeJobs.length} active` : 'Queue ready'}</b></header>
        <div class="destination-row"><label><span>Upload destination</span><select bind:value={selectedStorage}>{#each destinations as item}<option value={item.id}>{item.name} · {item.bucket}</option>{/each}</select></label><button onclick={backupNow} disabled={creating || !selectedStorage}><Icon name="hard-drive" size={15}/>{creating ? 'Adding…' : 'Backup now'}</button></div>
        <footer><Icon name="lock" size={13}/> Credentials remain encrypted inside the archive.</footer>
      </section>
      <aside class="panel recovery-set"><header><span>Recovery set</span><strong>Everything needed to return</strong></header><ul><li><Icon name="layers" size={15}/><span><strong>All service definitions</strong><small>Images, source settings, commands, health checks, and encrypted environments</small></span></li><li><Icon name="network" size={15}/><span><strong>Project routing</strong><small>Domains, paths, ports, and project-level environment</small></span></li><li><Icon name="database" size={15}/><span><strong>{volumeCount} persistent volume{volumeCount === 1 ? '' : 's'}</strong><small>Portable Docker tar streams captured from every database</small></span></li></ul></aside>
    </div>

    <section class="panel schedule-card">
      <header><div class="schedule-title"><i><Icon name="clock" size={17}/></i><div><span>Automation</span><h3>Scheduled project backups</h3><p>Keep a rolling recovery window without manual cleanup.</p></div></div><b class:enabled={schedule.enabled}>{schedule.enabled ? 'Active' : 'Paused'}</b></header>
      <form onsubmit={(event) => { event.preventDefault(); saveSchedule(); }}>
        <label class="toggle"><span><strong>Enable automatic backups</strong><small>Dokyr queues these even when no one is signed in.</small></span><input type="checkbox" bind:checked={schedule.enabled}/><i></i></label>
        <div class="schedule-fields"><label><span>Object storage</span><select bind:value={schedule.objectStorageId} required><option value="" disabled>Select a bucket</option>{#each destinations as item}<option value={item.id}>{item.name} · {item.bucket}</option>{/each}</select></label><label><span>Frequency</span><select bind:value={schedule.frequency}><option value="daily">Every day</option><option value="weekly">Every week</option></select></label>{#if schedule.frequency === 'weekly'}<label><span>Day</span><select bind:value={schedule.weekday}>{#each weekdays as day, i}<option value={i}>{day}</option>{/each}</select></label>{/if}<label><span>Run at</span><input type="time" bind:value={scheduleTime}/></label><label><span>Backups to keep</span><input type="number" min="1" max="100" bind:value={schedule.retentionCount}/></label></div>
        <footer><span><Icon name="globe" size={13}/> {schedule.timezone} · Last run {date(schedule.lastRunAt)}{#if saved} · Saved{/if}</span><button type="submit" disabled={saving || !schedule.objectStorageId}>{saving ? 'Saving…' : 'Save schedule'}</button></footer>
      </form>
    </section>

    <section class="panel history">
      <header><div><span>Archive ledger</span><h3>Backup and restore history</h3></div><button onclick={() => load()} disabled={loading}><Icon name="refresh" size={13}/> Refresh</button></header>
      {#if jobs.length === 0}<div class="history-empty"><Icon name="hard-drive" size={22}/><strong>No recovery points yet</strong><small>Create the first backup now or enable the schedule above.</small></div>{:else}<div class="job-list">{#each jobs as job}<article><i class:restore={job.kind === 'restore'}><Icon name={job.kind === 'restore' ? 'refresh' : 'hard-drive'} size={15}/></i><div><strong>{job.kind === 'restore' ? `Restore from ${job.filename || 'project backup'}` : job.filename || 'Project backup'}</strong><span>{job.objectStorageName} · {job.trigger === 'scheduled' ? 'Scheduled' : job.kind === 'restore' ? 'Recovery' : 'Manual'} · {date(job.createdAt)}</span>{#if job.status === 'failed'}<small>{job.message}</small>{/if}</div><code>{bytes(job.sizeBytes)}</code><b class={job.status}>{job.status}</b><div class="job-actions"><button class="log-button" onclick={() => logJob = job}><Icon name="logs" size={13}/> View logs</button>{#if job.kind === 'backup' && job.status === 'succeeded' && job.objectKey}<button onclick={() => { restoreError = ''; restoreTarget = job; }}>Restore</button>{/if}</div></article>{/each}</div>{/if}
    </section>
  {/if}
{/if}

{#if restoreTarget}<ConfirmDialog title={`Restore ${project.name} from this backup?`} message={`Every current service definition and persistent database volume in ${project.name} will be replaced with the contents of ${restoreTarget.filename}. Changes made after that recovery point will be lost.`} confirmLabel="Queue project restore" requireText={`RESTORE ${project.name}`} busy={restoring} error={restoreError} onConfirm={restore} onClose={() => { if (!restoring) restoreTarget = null; }}/>{/if}

{#if logJob}
  <div class="log-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeLog(); }}>
    <div class="backup-log-dialog" role="dialog" aria-modal="true" aria-labelledby="backup-log-title">
      <header>
        <div><span>Backup worker</span><h2 id="backup-log-title">{logJob.kind === 'restore' ? 'Restore' : 'Backup'} logs</h2><p>{logJob.filename || `${project.name} project ${logJob.kind}`}</p></div>
        <button aria-label="Close backup logs" onclick={closeLog}>×</button>
      </header>
      <div class="log-summary">
        <div><span>Status</span><strong class={logJob.status}>{logJob.status}</strong></div>
        <div><span>Started</span><strong>{date(logJob.startedAt || logJob.createdAt)}</strong></div>
        <div><span>Duration</span><strong>{duration(logJob)}</strong></div>
        <div><span>Destination</span><strong>{logJob.objectStorageName || '—'}</strong></div>
      </div>
      <div class="log-console-head"><i></i><i></i><i></i><strong>project-backup/{logJob.id}</strong><span>{logJob.trigger}</span></div>
      <div class="backup-log-output">
        <div><time>{date(logJob.createdAt)}</time><b>queue</b><code>Project {logJob.kind} accepted ({logJob.trigger} trigger).</code></div>
        {#if logJob.startedAt}<div><time>{date(logJob.startedAt)}</time><b>worker</b><code>Background worker started the {logJob.kind} job.</code></div>{/if}
        {#if logJob.message}<div class:error={logJob.status === 'failed'} class:success={logJob.status === 'succeeded'}><time>{date(logJob.finishedAt || logJob.startedAt)}</time><b>{logJob.status === 'failed' ? 'error' : 'result'}</b><code>{logJob.message}</code></div>{/if}
        {#if !logJob.message}<div><time>—</time><b>status</b><code>{logJob.status === 'queued' ? 'Waiting for an available backup worker.' : 'The job is currently running.'}</code></div>{/if}
      </div>
      <footer><div><span>Archive</span><code>{logJob.filename || 'Not created'}</code></div><div><span>Size</span><code>{bytes(logJob.sizeBytes)}</code></div><button onclick={closeLog}>Close</button></footer>
    </div>
  </div>
{/if}

<style>
  .backup-alert { margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); display: flex; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised)); color: var(--color-danger); font-size: var(--text-sm); }
  .backup-alert button { margin-left: auto; border: 0; background: none; color: inherit; font-size: 20px; cursor: pointer; }
  .backup-loading { min-height: 260px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); }
  .backup-loading div { display: grid; gap: 3px; } .backup-loading small { color: var(--color-muted); }
  .backup-hero { min-height: 150px; margin-bottom: var(--space-4); padding: var(--space-5); display: grid; grid-template-columns: 58px minmax(0,1fr) auto; align-items: center; gap: var(--space-5); overflow: hidden; border: 1px solid color-mix(in srgb, var(--color-accent) 38%, var(--color-rule)); border-radius: var(--radius-lg); background: linear-gradient(105deg, color-mix(in srgb, var(--color-accent) 8%, var(--color-paper-raised)), var(--color-paper-raised) 62%); box-shadow: var(--shadow-panel); }
  .hero-mark { width: 58px; height: 58px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 35%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-paper-raised); color: var(--color-accent); }
  .backup-hero > div:nth-child(2) { display: grid; gap: 5px; } .backup-hero span, .panel header span { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
  .backup-hero h3, .backup-hero p, .panel h3, .panel p { margin: 0; } .backup-hero h3 { font-size: var(--text-xl); } .backup-hero p { max-width: 70ch; color: var(--color-muted); font-size: var(--text-sm); line-height: 1.55; }
  .backup-hero dl { margin: 0; display: flex; } .backup-hero dl div { min-width: 92px; padding: 0 var(--space-4); border-left: 1px solid var(--color-rule); } .backup-hero dt { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; } .backup-hero dd { margin: 4px 0 0; font-size: var(--text-xl); font-weight: 700; }
  .backup-grid { margin-bottom: var(--space-4); display: grid; grid-template-columns: minmax(0,1.6fr) minmax(300px,.8fr); gap: var(--space-4); }
  .panel { overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .backup-now > header, .schedule-card > header, .history > header { min-height: 76px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .backup-now header > div, .history header > div { display: grid; gap: 2px; } .backup-now h3, .schedule-card h3, .history h3 { font-size: var(--text-md); } .backup-now p, .schedule-card p { color: var(--color-muted); font-size: var(--text-xs); }
  .backup-now header > b, .schedule-card header > b { padding: 5px 9px; border-radius: 99px; background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-xs); white-space: nowrap; } .backup-now header > b.busy { color: var(--color-accent); background: var(--color-accent-soft); }
  .destination-row { padding: var(--space-5); display: grid; grid-template-columns: minmax(0,1fr) auto; align-items: end; gap: var(--space-3); } label { display: grid; gap: 7px; color: var(--color-ink-secondary); font-size: var(--text-xs); font-weight: 600; }
  select, input { width: 100%; height: 42px; padding: 0 var(--space-3); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); outline: 0; }
  select:focus, input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 13%, transparent); }
  .destination-row button, .schedule-card form footer button { min-height: 42px; padding: 0 var(--space-4); display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); border: 1px solid var(--color-accent); border-radius: var(--radius-sm); background: var(--color-accent); color: var(--color-accent-ink); font-weight: 700; cursor: pointer; }
  button:disabled { opacity: .55; cursor: not-allowed; } .backup-now > footer { padding: 0 var(--space-5) var(--space-4); display: flex; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); }
  .recovery-set header { padding: var(--space-4) var(--space-5); display: grid; gap: 2px; border-bottom: 1px solid var(--color-rule); } .recovery-set ul { margin: 0; padding: 0; list-style: none; } .recovery-set li { min-height: 65px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 32px minmax(0,1fr); align-items: center; gap: var(--space-2); border-bottom: 1px solid var(--color-rule); } .recovery-set li:last-child { border: 0; } .recovery-set li > :global(svg) { color: var(--color-accent); } .recovery-set li span { display: grid; gap: 2px; } .recovery-set li strong { font-size: var(--text-sm); } .recovery-set li small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.35; }
  .schedule-card { margin-bottom: var(--space-4); } .schedule-title { display: flex; align-items: center; gap: var(--space-3); } .schedule-title > i { width: 40px; height: 40px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); } .schedule-title > div { display: grid; gap: 2px; } .schedule-card header > b { background: var(--color-paper-subtle); color: var(--color-muted); } .schedule-card header > b.enabled { background: var(--color-success-soft); color: var(--color-success); }
  .toggle { min-height: 64px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: minmax(0,1fr) auto; align-items: center; border-bottom: 1px solid var(--color-rule); cursor: pointer; } .toggle > span { display: grid; gap: 2px; } .toggle small { color: var(--color-muted); font-weight: 400; } .toggle input { position: absolute; opacity: 0; } .toggle i { width: 38px; height: 22px; padding: 3px; border-radius: 20px; background: var(--color-rule-strong); } .toggle i::after { content: ''; width: 16px; height: 16px; display: block; border-radius: 50%; background: white; transition: transform .18s ease; } .toggle input:checked + i { background: var(--color-accent); } .toggle input:checked + i::after { transform: translateX(16px); }
  .schedule-fields { padding: var(--space-4) var(--space-5); display: grid; grid-template-columns: 1.5fr repeat(4,minmax(110px,.65fr)); gap: var(--space-3); } .schedule-card form > footer { min-height: 58px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); color: var(--color-muted); font-size: var(--text-xs); } .schedule-card form > footer span { display: flex; align-items: center; gap: var(--space-2); }
  .history > header > button, .history article button { min-height: 32px; padding: 0 var(--space-3); display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .history-empty { min-height: 180px; display: grid; place-content: center; justify-items: center; gap: var(--space-2); color: var(--color-muted); } .history-empty strong { color: var(--color-ink); }
  .job-list article { min-height: 68px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 34px minmax(0,1fr) 76px 86px auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); } .job-list article:last-child { border: 0; } .job-list article > i { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-accent-soft); color: var(--color-accent); } .job-list article > i.restore { background: var(--color-warning-soft); color: var(--color-warning); } .job-list article > div:not(.job-actions) { min-width: 0; display: grid; gap: 2px; } .job-list article strong, .job-list article span, .job-list article small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } .job-list article strong { font-size: var(--text-sm); } .job-list article span, .job-list article small, .job-list article code { color: var(--color-muted); font-size: var(--text-xs); } .job-list article small { color: var(--color-danger); } .job-list article > b { color: var(--color-muted); font-size: var(--text-xs); text-transform: capitalize; } .job-list article > b.succeeded { color: var(--color-success); } .job-list article > b.failed { color: var(--color-danger); } .job-list article > b.running, .job-list article > b.queued { color: var(--color-accent); }
  .job-actions { display: flex; justify-content: flex-end; gap: var(--space-2); } .job-actions .log-button { color: var(--color-accent); }
  .log-backdrop { position: fixed; inset: 0; z-index: 80; padding: var(--space-5); display: grid; place-items: center; background: color-mix(in srgb, var(--color-ink) 58%, transparent); backdrop-filter: blur(4px); }
  .backup-log-dialog { width: min(820px, 100%); max-height: min(760px, calc(100vh - 40px)); display: grid; grid-template-rows: auto auto auto minmax(180px, 1fr) auto; overflow: hidden; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: 0 24px 80px color-mix(in srgb, var(--color-ink) 28%, transparent); }
  .backup-log-dialog > header { min-height: 82px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); } .backup-log-dialog > header > div { min-width: 0; display: grid; gap: 2px; } .backup-log-dialog > header span { color: var(--color-accent); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .08em; text-transform: uppercase; } .backup-log-dialog h2, .backup-log-dialog p { margin: 0; } .backup-log-dialog h2 { font-size: var(--text-lg); } .backup-log-dialog p { overflow: hidden; color: var(--color-muted); font: var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; } .backup-log-dialog > header > button { width: 34px; height: 34px; border: 1px solid var(--color-rule); border-radius: 50%; background: transparent; color: var(--color-muted); font-size: 22px; cursor: pointer; }
  .log-summary { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); border-bottom: 1px solid var(--color-rule); } .log-summary div { min-width: 0; min-height: 64px; padding: var(--space-3) var(--space-4); display: grid; align-content: center; gap: 3px; border-right: 1px solid var(--color-rule); } .log-summary div:last-child { border: 0; } .log-summary span, .backup-log-dialog footer span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .06em; text-transform: uppercase; } .log-summary strong { overflow: hidden; font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; } .log-summary strong.failed { color: var(--color-danger); } .log-summary strong.succeeded { color: var(--color-success); } .log-summary strong.running, .log-summary strong.queued { color: var(--color-accent); }
  .log-console-head { min-height: 38px; padding: 0 var(--space-4); display: flex; align-items: center; gap: 7px; border-bottom: 1px solid var(--color-log-rule); background: var(--color-log-surface); color: var(--color-log-muted); } .log-console-head i { width: 8px; height: 8px; border-radius: 50%; background: #ff5f57; } .log-console-head i:nth-child(2) { background: #febc2e; } .log-console-head i:nth-child(3) { background: #28c840; } .log-console-head strong { margin-left: var(--space-2); color: var(--color-log-text); font: 500 var(--text-xs) var(--font-mono); } .log-console-head span { margin-left: auto; font: var(--text-2xs) var(--font-mono); text-transform: uppercase; }
  .backup-log-output { min-height: 0; padding: var(--space-2) 0; overflow: auto; background: var(--color-log-bg); color: var(--color-log-text); } .backup-log-output > div { padding: var(--space-2) var(--space-4); display: grid; grid-template-columns: 162px 56px minmax(0,1fr); align-items: start; gap: var(--space-3); border-left: 2px solid transparent; } .backup-log-output > div.error { border-left-color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 7%, transparent); } .backup-log-output > div.success { border-left-color: var(--color-success); } .backup-log-output time, .backup-log-output b { color: var(--color-log-muted); font: var(--text-2xs)/1.65 var(--font-mono); } .backup-log-output b { color: var(--color-accent); font-weight: 700; text-transform: uppercase; } .backup-log-output .error b { color: var(--color-danger); } .backup-log-output code { color: var(--color-log-text); font: var(--text-xs)/1.65 var(--font-mono); white-space: pre-wrap; overflow-wrap: anywhere; }
  .backup-log-dialog > footer { min-height: 62px; padding: var(--space-3) var(--space-4); display: flex; align-items: center; gap: var(--space-6); border-top: 1px solid var(--color-rule); } .backup-log-dialog > footer div { min-width: 0; display: grid; gap: 2px; } .backup-log-dialog > footer code { max-width: 360px; overflow: hidden; color: var(--color-muted); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; } .backup-log-dialog > footer button { min-height: 34px; margin-left: auto; padding: 0 var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink); font-weight: 600; cursor: pointer; }
  .no-storage { min-height: 130px; padding: var(--space-5); display: flex; align-items: center; gap: var(--space-4); } .no-storage > div { display: grid; gap: 3px; } .no-storage p { color: var(--color-muted); font-size: var(--text-sm); } .no-storage a { margin-left: auto; color: var(--color-accent); font-size: var(--text-sm); font-weight: 700; text-decoration: none; }
  @media (max-width: 900px) { .backup-hero { grid-template-columns: 48px minmax(0,1fr); } .backup-hero dl { grid-column: 1/-1; } .backup-grid { grid-template-columns: 1fr; } .schedule-fields { grid-template-columns: repeat(2,minmax(0,1fr)); } .job-list article { grid-template-columns: 34px minmax(0,1fr) 76px; } .job-list article > b, .job-actions { grid-column: 2; } .job-actions { justify-content: flex-start; } }
  @media (max-width: 600px) { .backup-hero { padding: var(--space-4); } .backup-hero dl div { min-width: 0; flex: 1; padding: 0 var(--space-2); } .destination-row, .schedule-fields { grid-template-columns: 1fr; } .destination-row button { width: 100%; } .schedule-card form > footer { align-items: stretch; flex-direction: column; } .log-backdrop { padding: 0; } .backup-log-dialog { max-height: 100vh; border-radius: 0; } .log-summary { grid-template-columns: repeat(2,minmax(0,1fr)); } .backup-log-output > div { grid-template-columns: 1fr; gap: 3px; } .backup-log-dialog > footer { align-items: stretch; flex-direction: column; gap: var(--space-2); } .backup-log-dialog > footer button { width: 100%; margin-left: 0; } }
</style>
