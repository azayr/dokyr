<script>
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  let email = '';
  let busy = false;
  let error = '';
  let sent = false;

  async function submit() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/auth/password-reset/request', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email })
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
  <p class="eyebrow">Password reset</p>
  {#if sent}
    <div class="success"><span>✓</span><div><h2>Check your inbox</h2><p>If an account exists for <strong>{email}</strong>, a reset link was sent. It expires in 30 minutes.</p></div></div>
    <a class="primary" href="/login">Return to sign in <span>→</span></a>
  {:else}
    <h2>Forgot your password?</h2>
    <p class="lead">Enter your owner email address. We will send a private recovery link through the configured SMTP server.</p>
    {#if error}<div class="error">{error}</div>{/if}
    <form onsubmit={(event) => { event.preventDefault(); submit(); }}>
      <label>Email address<input bind:value={email} type="email" autocomplete="email" placeholder="you@company.com" required/></label>
      <button disabled={busy}>{busy ? 'Sending recovery link…' : 'Send reset link'}<span>→</span></button>
    </form>
    <a class="back" href="/login">← Back to sign in</a>
  {/if}
</AuthFrame>

<style>
  .eyebrow{margin:0;color:#d8ff73;font:9px var(--font-mono);letter-spacing:.12em;text-transform:uppercase}h2{margin:9px 0 7px;font-size:25px;letter-spacing:-.04em}.lead{margin:0 0 24px;color:#7e887f;font-size:12px;line-height:1.6}.error{margin-bottom:16px;padding:10px 12px;border:1px solid #6c3432;border-radius:6px;background:#261615;color:#f09490;font-size:11px;line-height:1.45}form{display:grid;gap:15px}label{display:grid;gap:7px;color:#a9b1aa;font-size:10px;font-weight:700}input{height:43px;padding:0 12px;outline:none;border:1px solid #2a302b;border-radius:7px;background:#111412;color:#eef2ec;font:12px var(--font-sans)}input:focus{border-color:#829c43;box-shadow:0 0 0 3px #d8ff7311}button,.primary{height:44px;padding:0 15px;display:flex;align-items:center;justify-content:space-between;border:0;border-radius:7px;background:#d8ff73;color:#0b0d0c;font:700 11px var(--font-sans);text-decoration:none;cursor:pointer}button:disabled{opacity:.6}.back{display:block;margin:18px auto 0;color:#7e887f;font-size:10px;text-align:center;text-decoration:none}.success{margin:18px 0 24px;padding:18px;display:flex;gap:13px;border:1px solid #31452c;border-radius:9px;background:#111b10}.success>span{width:28px;height:28px;display:grid;place-items:center;border-radius:50%;background:#d8ff7314;color:#d8ff73}.success h2{margin:0 0 5px;font-size:18px}.success p{margin:0;color:#8d978e;font-size:11px;line-height:1.6}.success strong{color:#c7cec8}
</style>
