<script>
  import AuthFrame from '$lib/components/AuthFrame.svelte';
  import Icon from '$lib/components/Icon.svelte';

  let name = '';
  let email = '';
  let password = '';
  let error = '';
  let busy = false;
  let passwordVisible = false;

  async function submit() {
    busy = true;
    error = '';
    const response = await fetch('/api/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, email, password })
    });
    const data = await response.json();
    if (!response.ok) { error = data.error || 'Setup failed'; busy = false; return; }
    location.href = '/';
  }
</script>

<AuthFrame step="First run · Step 1 of 1" title="Claim this server" description="Create the owner account for this Dokyr installation. Registration closes permanently after this step.">
  <span class="auth-tag">New installation</span>
  <h2 class="auth-title">Owner account</h2>
  <p class="auth-lead">This account manages deployments, secrets, and server access.</p>
  {#if error}<div class="auth-error" role="alert">{error}</div>{/if}
  <form class="auth-form" onsubmit={(event) => { event.preventDefault(); submit(); }}>
    <label class="auth-field">
      Full name
      <input class="auth-input" bind:value={name} autocomplete="name" placeholder="Ada Lovelace" required />
    </label>
    <label class="auth-field">
      Email address
      <input class="auth-input" bind:value={email} type="email" autocomplete="email" placeholder="you@company.com" required />
    </label>
    <label class="auth-field">
      Password
      <span class="auth-password-wrap">
        <input class="auth-input" bind:value={password} type={passwordVisible ? 'text' : 'password'} autocomplete="new-password" minlength="10" placeholder="At least 10 characters" required />
        <button class="auth-password-toggle" type="button" aria-label={passwordVisible ? 'Hide password' : 'Show password'} onclick={() => (passwordVisible = !passwordVisible)}>
          <Icon name={passwordVisible ? 'eye-off' : 'eye'} size={15} />
        </button>
      </span>
    </label>
    <button class="auth-submit" disabled={busy}>
      {busy ? 'Creating owner…' : 'Create owner account'}
      {#if !busy}<Icon name="arrow-right" size={15} />{/if}
    </button>
  </form>
  <p class="auth-note">Registration is disabled after the owner is created.</p>
</AuthFrame>
