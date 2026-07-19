<script>
  import { onMount } from 'svelte';
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { loadSession } from '$lib/auth.js';

  let email = '';
  let password = '';
  let code = '';
  let error = '';
  let busy = false;
  let stage = 'credentials';
  let githubConfigured = false;
  let passwordResetEnabled = false;

  onMount(async () => {
    const query = new URLSearchParams(location.search);
    if (query.get('error')) error = query.get('error');
    if (query.get('twoFactor')) stage = 'two-factor';
    try {
      const response = await fetch('/api/auth/providers');
      if (response.ok) {
        const providers = await response.json();
        githubConfigured = providers.github?.configured === true;
      }
    } catch {}
    try {
      const response = await fetch('/api/auth/password-reset/status');
      if (response.ok) passwordResetEnabled = (await response.json()).enabled === true;
    } catch {}
  });

  async function submitCredentials() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Sign in failed.');
      password = '';
      if (data.requiresTwoFactor) {
        stage = 'two-factor';
        return;
      }
      await finishLogin();
    } catch (cause) {
      error = cause.message;
    } finally {
      busy = false;
    }
  }

  async function submitCode() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/2fa', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Authentication code was rejected.');
      await finishLogin();
    } catch (cause) {
      error = cause.message;
    } finally {
      busy = false;
    }
  }

  async function finishLogin() {
    await loadSession();
    location.href = '/';
  }

  async function cancelChallenge() {
    await fetch('/api/auth/logout', { method: 'POST' });
    code = '';
    stage = 'credentials';
    history.replaceState(null, '', '/login');
  }
</script>

<AuthFrame step="Secure access" title="Return to the console" description="Your infrastructure is waiting exactly where you left it.">
  <div class="status"><i></i> Control plane online</div>
  {#if stage === 'credentials'}
    <h2>Sign in</h2>
    <p class="lead">Use your Selfhost account or a linked GitHub identity.</p>
    {#if error}<div class="error">{error}</div>{/if}
    <form onsubmit={(event) => { event.preventDefault(); submitCredentials(); }}>
      <label>Email address<input bind:value={email} type="email" autocomplete="email" placeholder="you@company.com" required/></label>
      <label><span class="password-label">Password{#if passwordResetEnabled}<a href="/forgot-password">Forgot password?</a>{/if}</span><input bind:value={password} type="password" autocomplete="current-password" placeholder="Your password" required/></label>
      <button class="submit" disabled={busy}>{busy ? 'Authenticating…' : 'Enter console'}<span>→</span></button>
    </form>
    <div class="divider"><span>or</span></div>
    {#if githubConfigured}
      <a class="github-button" href="/api/auth/github/start"><Icon name="github" size={18}/>Continue with GitHub</a>
    {:else}
      <button class="github-button unavailable" disabled title="Configure GitHub OAuth in Selfhost first"><Icon name="github" size={18}/>GitHub login is not configured</button>
      <p class="provider-help">An owner can configure and link GitHub from Settings → Security.</p>
    {/if}
  {:else}
    <div class="challenge-icon"><Icon name="shield" size={24}/></div>
    <p class="eyebrow">Second step</p>
    <h2>Authentication code</h2>
    <p class="lead">Enter the six-digit code from the authenticator linked to this account.</p>
    {#if error}<div class="error">{error}</div>{/if}
    <form onsubmit={(event) => { event.preventDefault(); submitCode(); }}>
      <label>Six-digit code<input class="code" bind:value={code} inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" placeholder="000000" required/></label>
      <button class="submit" disabled={busy}>{busy ? 'Verifying…' : 'Verify and continue'}<span>→</span></button>
    </form>
    <button class="back" onclick={cancelChallenge}>← Back to sign in</button>
  {/if}
</AuthFrame>

<style>
  .status{display:inline-flex;align-items:center;gap:8px;color:#83cc92;font:9px var(--font-mono);text-transform:uppercase;letter-spacing:.08em}.status i{width:6px;height:6px;border-radius:50%;background:#78e08f;box-shadow:0 0 0 5px #78e08f13}h2{font-size:25px;letter-spacing:-.04em;margin:19px 0 6px}.lead{font-size:12px;color:#7e887f;margin:0 0 25px;line-height:1.55}.error{padding:10px 12px;border:1px solid #6c3432;background:#261615;color:#f09490;border-radius:6px;font-size:11px;line-height:1.45;margin-bottom:16px}form{display:grid;gap:15px}label{display:grid;gap:7px;font-size:10px;font-weight:700;color:#a9b1aa}.password-label{display:flex;align-items:center;justify-content:space-between;gap:12px}.password-label a{color:#a9c95f;text-decoration:none;font-weight:600}.password-label a:hover{color:#d8ff73}input{height:43px;background:#111412;border:1px solid #2a302b;border-radius:7px;padding:0 12px;color:#eef2ec;font:12px var(--font-sans);outline:none}input:focus{border-color:#829c43;box-shadow:0 0 0 3px #d8ff7311}.code{height:58px;text-align:center;font:22px var(--font-mono);letter-spacing:.28em;padding-left:calc(12px + .28em)}.submit{height:44px;border:0;border-radius:7px;background:#d8ff73;color:#0b0d0c;font:700 11px var(--font-sans);margin-top:5px;display:flex;justify-content:space-between;align-items:center;padding:0 15px;cursor:pointer}.submit:disabled{opacity:.6}.divider{height:35px;display:flex;align-items:center;color:#566057;font:9px var(--font-mono);text-transform:uppercase}.divider:before,.divider:after{content:'';height:1px;background:#252a26;flex:1}.divider span{padding:0 11px}.github-button{width:100%;height:43px;border:1px solid #313832;border-radius:7px;background:#151917;color:#eef2ec;text-decoration:none;display:flex;align-items:center;justify-content:center;gap:9px;font:700 11px var(--font-sans);cursor:pointer}.github-button:hover{border-color:#4c574f;background:#1a201c}.github-button.unavailable{color:#707970;cursor:not-allowed}.provider-help{text-align:center;font-size:9px;color:#586159;line-height:1.5;margin:9px 0 0}.challenge-icon{width:48px;height:48px;border-radius:12px;background:#d8ff7314;color:#d8ff73;display:grid;place-items:center;margin-bottom:18px}.eyebrow{font:9px var(--font-mono);color:#d8ff73;text-transform:uppercase;letter-spacing:.11em;margin:0}.challenge-icon~h2{margin-top:8px}.back{display:block;margin:17px auto 0;border:0;background:transparent;color:#7e887f;font:10px var(--font-sans);cursor:pointer}.back:hover{color:#eef2ec}
</style>
