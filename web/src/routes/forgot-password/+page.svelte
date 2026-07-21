<script>
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  import Icon from '$lib/components/Icon.svelte';

  let email = '';
  let busy = false;
  let error = '';
  let sent = false;

  async function submit() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/password-reset/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not request a password reset.');
      sent = true;
    } catch (cause) {
      error = cause.message;
    } finally {
      busy = false;
    }
  }
</script>

<AuthFrame step="Account recovery" title="Recover access" description="Receive a secure, one-time link from your own control plane.">
  {#if sent}
    <div class="auth-success">
      <Icon name="check-circle" size={18} />
      <div>
        <h2>Check your inbox</h2>
        <p>If an account exists for <strong>{email}</strong>, a reset link was sent. It expires in 30 minutes.</p>
      </div>
    </div>
    <a class="auth-submit" href="/login">Return to sign in <Icon name="arrow-right" size={15} /></a>
  {:else}
    <p class="auth-eyebrow">Password reset</p>
    <h2 class="auth-title">Forgot your password?</h2>
    <p class="auth-lead">Enter your owner email address. We will send a private recovery link through the configured SMTP server.</p>
    {#if error}<div class="auth-error" role="alert">{error}</div>{/if}
    <form class="auth-form" onsubmit={(event) => { event.preventDefault(); submit(); }}>
      <label class="auth-field">
        Email address
        <input class="auth-input" bind:value={email} type="email" autocomplete="email" placeholder="you@company.com" required />
      </label>
      <button class="auth-submit" disabled={busy}>
        {busy ? 'Sending recovery link…' : 'Send reset link'}
        {#if !busy}<Icon name="arrow-right" size={15} />{/if}
      </button>
    </form>
    <a class="auth-back" href="/login"><Icon name="arrow-left" size={14} /> Back to sign in</a>
  {/if}
</AuthFrame>
