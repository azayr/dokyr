<script>
  import { onMount } from 'svelte';
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  let token = '';
  let password = '';
  let confirmation = '';
  let busy = false;
  let error = '';
  let complete = false;

  onMount(() => { token = new URLSearchParams(location.search).get('token') || ''; });

  async function submit() {
    if (password !== confirmation) { error = 'Password confirmation does not match.'; return; }
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/password-reset/confirm', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ token, newPassword: password })
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

<AuthFrame step="Secure recovery" title="Choose a new password" description="This one-time link belongs to this Selfhost installation.">
  {#if complete}
    <div class="success"><span>✓</span><div><h2>Password updated</h2><p>Your recovery token has been consumed and cannot be used again.</p></div></div>
    <a class="primary" href="/login">Continue to sign in <span>→</span></a>
  {:else}
    <p class="eyebrow">New credentials</p>
    <h2>Reset password</h2>
    <p class="lead">Choose at least 12 characters. Finishing this step invalidates the recovery link.</p>
    {#if !token}<div class="error">This password reset link is missing its secure token.</div>{:else if error}<div class="error">{error}</div>{/if}
    <form onsubmit={(event) => { event.preventDefault(); submit(); }}>
      <label>New password<input bind:value={password} type="password" autocomplete="new-password" minlength="12" maxlength="128" required/></label>
      <label>Confirm new password<input bind:value={confirmation} type="password" autocomplete="new-password" minlength="12" maxlength="128" required/></label>
      <button disabled={busy || !token}>{busy ? 'Updating password…' : 'Update password'}<span>→</span></button>
    </form>
    <a class="back" href="/login">← Back to sign in</a>
  {/if}
</AuthFrame>

<style>
  .eyebrow{margin:0;color:#d8ff73;font:9px var(--font-mono);letter-spacing:.12em;text-transform:uppercase}h2{margin:9px 0 7px;font-size:25px;letter-spacing:-.04em}.lead{margin:0 0 24px;color:#7e887f;font-size:12px;line-height:1.6}.error{margin-bottom:16px;padding:10px 12px;border:1px solid #6c3432;border-radius:6px;background:#261615;color:#f09490;font-size:11px;line-height:1.45}form{display:grid;gap:15px}label{display:grid;gap:7px;color:#a9b1aa;font-size:10px;font-weight:700}input{height:43px;padding:0 12px;outline:none;border:1px solid #2a302b;border-radius:7px;background:#111412;color:#eef2ec;font:12px var(--font-sans)}input:focus{border-color:#829c43;box-shadow:0 0 0 3px #d8ff7311}button,.primary{height:44px;padding:0 15px;display:flex;align-items:center;justify-content:space-between;border:0;border-radius:7px;background:#d8ff73;color:#0b0d0c;font:700 11px var(--font-sans);text-decoration:none;cursor:pointer}button:disabled{opacity:.45;cursor:not-allowed}.back{display:block;margin:18px auto 0;color:#7e887f;font-size:10px;text-align:center;text-decoration:none}.success{margin:18px 0 24px;padding:18px;display:flex;gap:13px;border:1px solid #31452c;border-radius:9px;background:#111b10}.success>span{width:28px;height:28px;display:grid;place-items:center;border-radius:50%;background:#d8ff7314;color:#d8ff73}.success h2{margin:0 0 5px;font-size:18px}.success p{margin:0;color:#8d978e;font-size:11px;line-height:1.6}
</style>
