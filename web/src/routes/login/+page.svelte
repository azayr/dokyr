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
  let passwordVisible = false;

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

<AuthFrame step="Secure access" title="Welcome back" description="Sign in to manage projects, deployments, and infrastructure on this server.">
  {#if stage === 'credentials'}
    <span class="auth-badge"><i></i> Control plane online</span>
    <h2 class="auth-title">Sign in</h2>
    <p class="auth-lead">Use your Dokyr account or a linked GitHub identity.</p>
    {#if error}<div class="auth-error" role="alert">{error}</div>{/if}
    <form class="auth-form" onsubmit={(event) => { event.preventDefault(); submitCredentials(); }}>
      <label class="auth-field">
        Email address
        <input class="auth-input" bind:value={email} type="email" autocomplete="email" placeholder="you@company.com" required />
      </label>
      <label class="auth-field">
        <span class="auth-field-row">Password {#if passwordResetEnabled}<a href="/forgot-password">Forgot password?</a>{/if}</span>
        <span class="auth-password-wrap">
          <input class="auth-input" bind:value={password} type={passwordVisible ? 'text' : 'password'} autocomplete="current-password" placeholder="Your password" required />
          <button class="auth-password-toggle" type="button" aria-label={passwordVisible ? 'Hide password' : 'Show password'} onclick={() => (passwordVisible = !passwordVisible)}>
            <Icon name={passwordVisible ? 'eye-off' : 'eye'} size={15} />
          </button>
        </span>
      </label>
      <button class="auth-submit" disabled={busy}>
        {busy ? 'Signing in…' : 'Sign in'}
        {#if !busy}<Icon name="arrow-right" size={15} />{/if}
      </button>
    </form>
    <div class="auth-divider"><span>or</span></div>
    {#if githubConfigured}
      <a class="auth-alt-button" href="/api/auth/github/start"><Icon name="github" size={16} /> Continue with GitHub</a>
    {:else}
      <button class="auth-alt-button" disabled title="GitHub login is not configured on this server"><Icon name="github" size={16} /> GitHub login is not configured</button>
      <p class="auth-provider-help">An owner can configure and link GitHub from Settings → Security.</p>
    {/if}
  {:else}
    <span class="auth-challenge-icon"><Icon name="shield" size={20} /></span>
    <p class="auth-eyebrow">Second step</p>
    <h2 class="auth-title">Authentication code</h2>
    <p class="auth-lead">Enter the six-digit code from the authenticator linked to this account.</p>
    {#if error}<div class="auth-error" role="alert">{error}</div>{/if}
    <form class="auth-form" onsubmit={(event) => { event.preventDefault(); submitCode(); }}>
      <label class="auth-field">
        Six-digit code
        <input class="auth-input auth-code-input" bind:value={code} inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern={'[0-9]{6}'} placeholder="000000" required />
      </label>
      <button class="auth-submit" disabled={busy}>
        {busy ? 'Verifying…' : 'Verify and continue'}
        {#if !busy}<Icon name="arrow-right" size={15} />{/if}
      </button>
    </form>
    <button class="auth-back" onclick={cancelChallenge}><Icon name="arrow-left" size={14} /> Back to sign in</button>
  {/if}
</AuthFrame>
