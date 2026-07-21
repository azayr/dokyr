<script>
  import { onMount } from 'svelte';
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  import Icon from '$lib/components/Icon.svelte';

  let token = '';
  let password = '';
  let confirmation = '';
  let busy = false;
  let error = '';
  let complete = false;
  let passwordVisible = false;

  onMount(() => {
    token = new URLSearchParams(location.search).get('token') || '';
  });

  async function submit() {
    if (password !== confirmation) { error = 'Password confirmation does not match.'; return; }
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/password-reset/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, newPassword: password })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not reset the password.');
      password = '';
      confirmation = '';
      complete = true;
    } catch (cause) {
      error = cause.message;
    } finally {
      busy = false;
    }
  }
</script>

<AuthFrame step="Secure recovery" title="Choose a new password" description="This one-time link belongs to this Dokyr installation.">
  {#if complete}
    <div class="auth-success">
      <Icon name="check-circle" size={18} />
      <div>
        <h2>Password updated</h2>
        <p>Your recovery token has been consumed and cannot be used again.</p>
      </div>
    </div>
    <a class="auth-submit" href="/login">Continue to sign in <Icon name="arrow-right" size={15} /></a>
  {:else}
    <p class="auth-eyebrow">New credentials</p>
    <h2 class="auth-title">Reset password</h2>
    <p class="auth-lead">Choose at least 12 characters. Finishing this step invalidates the recovery link.</p>
    {#if !token}<div class="auth-error" role="alert">This password reset link is missing its secure token.</div>{:else if error}<div class="auth-error" role="alert">{error}</div>{/if}
    <form class="auth-form" onsubmit={(event) => { event.preventDefault(); submit(); }}>
      <label class="auth-field">
        New password
        <span class="auth-password-wrap">
          <input class="auth-input" bind:value={password} type={passwordVisible ? 'text' : 'password'} autocomplete="new-password" minlength="12" maxlength="128" required />
          <button class="auth-password-toggle" type="button" aria-label={passwordVisible ? 'Hide password' : 'Show password'} onclick={() => (passwordVisible = !passwordVisible)}>
            <Icon name={passwordVisible ? 'eye-off' : 'eye'} size={15} />
          </button>
        </span>
      </label>
      <label class="auth-field">
        Confirm new password
        <input class="auth-input" bind:value={confirmation} type={passwordVisible ? 'text' : 'password'} autocomplete="new-password" minlength="12" maxlength="128" required />
      </label>
      <button class="auth-submit" disabled={busy || !token}>
        {busy ? 'Updating password…' : 'Update password'}
        {#if !busy}<Icon name="arrow-right" size={15} />{/if}
      </button>
    </form>
    <a class="auth-back" href="/login"><Icon name="arrow-left" size={14} /> Back to sign in</a>
  {/if}
</AuthFrame>
