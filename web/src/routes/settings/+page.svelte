<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api, currentUser, logout } from '$lib/auth.js';

  let section = 'profile';
  let loading = true;
  let security = {
    twoFactorEnabled: false,
    github: { linked: false, login: '' },
    providers: { github: { configured: false, callbackUrl: '' } }
  };
  let notice = '';
  let error = '';

  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let passwordCode = '';
  let passwordBusy = false;

  let setupSecret = '';
  let setupURI = '';
  let confirmCode = '';
  let twoFactorBusy = false;
  let showDisableTwoFactor = false;
  let disablePassword = '';
  let disableCode = '';

  let showUnlinkGitHub = false;
  let unlinkPassword = '';
  let unlinkCode = '';
  let githubBusy = false;
  let smtp = { enabled: false, host: '', port: 587, encryption: 'starttls', username: '', password: '', hasPassword: false, fromName: 'DeployForge', fromEmail: '', notifyDeploymentFailures: true, notifyDeploymentSuccesses: false };
  let smtpLoaded = false;
  let smtpLoading = false;
  let smtpSaving = false;
  let smtpTesting = false;
  let smtpTestRecipient = '';

  onMount(async () => {
    const query = new URLSearchParams(location.search);
    if (['profile', 'security', 'platform', 'smtp'].includes(query.get('section'))) section = query.get('section');
    if (query.get('github') === 'linked') notice = 'GitHub account linked. You can now use it to sign in.';
    if (query.get('error')) error = query.get('error');
    await loadSecurity();
    if (section === 'smtp') await loadSMTP();
  });

  async function loadSecurity() {
    loading = true;
    try {
      const response = await api('/api/account/security');
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not load security settings.');
      security = data;
    } catch (cause) {
      error = cause.message;
    } finally {
      loading = false;
    }
  }

  function selectSection(next) {
    section = next;
    notice = '';
    error = '';
    history.replaceState(null, '', `/settings?section=${next}`);
    if (next === 'smtp' && !smtpLoaded) loadSMTP();
  }

  async function loadSMTP() {
    smtpLoading = true;
    try {
      const response = await api('/api/settings/smtp');
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not load SMTP settings.');
      smtp = { ...smtp, ...data, password: '' };
      smtpTestRecipient ||= $currentUser?.email || '';
      smtpLoaded = true;
    } catch (cause) {
      error = cause.message;
    } finally {
      smtpLoading = false;
    }
  }

  async function saveSMTP() {
    smtpSaving = true;
    try {
      const data = await request('/api/settings/smtp', { method: 'PUT', body: JSON.stringify({
        enabled: smtp.enabled, host: smtp.host, port: Number(smtp.port), encryption: smtp.encryption,
        username: smtp.username, password: smtp.password, fromName: smtp.fromName, fromEmail: smtp.fromEmail,
        notifyDeploymentFailures: smtp.notifyDeploymentFailures, notifyDeploymentSuccesses: smtp.notifyDeploymentSuccesses
      }) });
      smtp = { ...smtp, ...data.settings, password: '' };
      notice = data.message;
    } catch (cause) {
      error = cause.message;
    } finally {
      smtpSaving = false;
    }
  }

  async function testSMTP() {
    smtpTesting = true;
    try {
      const data = await request('/api/settings/smtp/test', { method: 'POST', body: JSON.stringify({ recipient: smtpTestRecipient }) });
      notice = data.message;
    } catch (cause) {
      error = cause.message;
    } finally {
      smtpTesting = false;
    }
  }

  async function request(path, options) {
    notice = '';
    error = '';
    const response = await api(path, {
      ...options,
      headers: { 'Content-Type': 'application/json', ...(options?.headers || {}) }
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'The request could not be completed.');
    return data;
  }

  async function updatePassword() {
    if (newPassword !== confirmPassword) { error = 'New password confirmation does not match.'; return; }
    passwordBusy = true;
    try {
      const data = await request('/api/account/password', {
        method: 'PUT',
        body: JSON.stringify({ currentPassword, newPassword, code: passwordCode })
      });
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
      passwordCode = '';
      notice = data.message;
    } catch (cause) {
      error = cause.message;
    } finally {
      passwordBusy = false;
    }
  }

  async function beginTwoFactor() {
    twoFactorBusy = true;
    try {
      const data = await request('/api/account/2fa/setup', { method: 'POST', body: '{}' });
      setupSecret = data.secret;
      setupURI = data.uri;
      notice = 'Authenticator secret created. Verify one code to finish setup.';
    } catch (cause) {
      error = cause.message;
    } finally {
      twoFactorBusy = false;
    }
  }

  async function confirmTwoFactor() {
    twoFactorBusy = true;
    try {
      const data = await request('/api/account/2fa/confirm', {
        method: 'POST', body: JSON.stringify({ code: confirmCode })
      });
      confirmCode = '';
      setupSecret = '';
      setupURI = '';
      notice = data.message;
      await loadSecurity();
    } catch (cause) {
      error = cause.message;
    } finally {
      twoFactorBusy = false;
    }
  }

  async function disableTwoFactor() {
    twoFactorBusy = true;
    try {
      const data = await request('/api/account/2fa', {
        method: 'DELETE', body: JSON.stringify({ password: disablePassword, code: disableCode })
      });
      disablePassword = '';
      disableCode = '';
      showDisableTwoFactor = false;
      notice = data.message;
      await loadSecurity();
    } catch (cause) {
      error = cause.message;
    } finally {
      twoFactorBusy = false;
    }
  }

  async function unlinkGitHub() {
    githubBusy = true;
    try {
      const data = await request('/api/account/github', {
        method: 'DELETE', body: JSON.stringify({ password: unlinkPassword, code: unlinkCode })
      });
      unlinkPassword = '';
      unlinkCode = '';
      showUnlinkGitHub = false;
      notice = data.message;
      await loadSecurity();
    } catch (cause) {
      error = cause.message;
    } finally {
      githubBusy = false;
    }
  }

  async function copy(value, label) {
    await navigator.clipboard.writeText(value);
    notice = `${label} copied to clipboard.`;
  }
</script>

<Shell eyebrow="Control plane" title="Settings">
  <div class="settings-layout">
    <div class="settings-nav" aria-label="Settings sections" role="tablist">
      <button role="tab" aria-selected={section === 'profile'} aria-controls="settings-panel" class:active={section === 'profile'} onclick={() => selectSection('profile')}><Icon name="settings" size={15}/>Profile</button>
      <button role="tab" aria-selected={section === 'security'} aria-controls="settings-panel" class:active={section === 'security'} onclick={() => selectSection('security')}><Icon name="shield" size={15}/>Security</button>
      <button role="tab" aria-selected={section === 'platform'} aria-controls="settings-panel" class:active={section === 'platform'} onclick={() => selectSection('platform')}><Icon name="server" size={15}/>Platform</button>
      <button role="tab" aria-selected={section === 'smtp'} aria-controls="settings-panel" class:active={section === 'smtp'} onclick={() => selectSection('smtp')}><Icon name="mail" size={15}/>SMTP</button>
    </div>

    <div class="settings-content" id="settings-panel" role="tabpanel">
      {#if notice}<div class="feedback success"><Icon name="check" size={16}/><span>{notice}</span><button aria-label="Dismiss message" onclick={() => notice = ''}>×</button></div>{/if}
      {#if error}<div class="feedback danger"><span>!</span><span>{error}</span><button aria-label="Dismiss error" onclick={() => error = ''}>×</button></div>{/if}

      {#if section === 'profile'}
        <section class="panel">
          <header class="panel-header">
            <div class="header-icon"><Icon name="settings" size={19}/></div>
            <div><small>Account</small><h2>Profile</h2><p>The owner identity used for administrative actions.</p></div>
          </header>
          <dl class="identity-list">
            <div><dt>Name</dt><dd>{$currentUser?.name}</dd></div>
            <div><dt>Email</dt><dd>{$currentUser?.email}</dd></div>
            <div><dt>Role</dt><dd><span class="role">{$currentUser?.role}</span></dd></div>
          </dl>
          <footer class="panel-footer"><button class="danger-button" onclick={logout}>Sign out of Selfhost</button></footer>
        </section>
      {:else if section === 'security'}
        {#if loading}
          <div class="loading">Loading account security…</div>
        {:else}
          <div class="security-heading"><small>Account protection</small><h2>Security</h2><p>Manage the credentials and identity providers that can access this control plane.</p></div>

          <section class="panel security-panel">
            <header class="security-card-header">
              <div class="header-icon"><Icon name="lock" size={19}/></div>
              <div><h3>Password</h3><p>Use a unique password with at least 12 characters.</p></div>
              <span class="state neutral">Configured</span>
            </header>
            <form class="form-grid" onsubmit={(event) => { event.preventDefault(); updatePassword(); }}>
              <label>Current password<input bind:value={currentPassword} type="password" autocomplete="current-password" required/></label>
              <div class="two-columns">
                <label>New password<input bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" required/></label>
                <label>Confirm new password<input bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" required/></label>
              </div>
              {#if security.twoFactorEnabled}<label class="code-field">Authentication code<input bind:value={passwordCode} inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="000000" required/></label>{/if}
              <div class="form-actions"><button class="primary" disabled={passwordBusy}>{passwordBusy ? 'Updating…' : 'Update password'}</button></div>
            </form>
          </section>

          <section class="panel security-panel">
            <header class="security-card-header">
              <div class="header-icon"><Icon name="shield" size={19}/></div>
              <div><h3>Two-factor authentication</h3><p>Require a time-based code from your authenticator after sign-in.</p></div>
              <span class:enabled={security.twoFactorEnabled} class="state">{security.twoFactorEnabled ? 'Enabled' : 'Not enabled'}</span>
            </header>
            {#if security.twoFactorEnabled}
              <div class="card-body split-row">
                <div class="explanation"><b>Your account has a second factor.</b><p>Password and GitHub sign-ins both require a current authenticator code.</p></div>
                <button class="danger-button" onclick={() => showDisableTwoFactor = !showDisableTwoFactor}>Disable 2FA</button>
              </div>
              {#if showDisableTwoFactor}
                <form class="confirm-box" onsubmit={(event) => { event.preventDefault(); disableTwoFactor(); }}>
                  <div><b>Confirm two-factor removal</b><p>Enter your password and a current authenticator code.</p></div>
                  <div class="two-columns"><label>Password<input bind:value={disablePassword} type="password" autocomplete="current-password" required/></label><label>Authentication code<input bind:value={disableCode} inputmode="numeric" maxlength="6" autocomplete="one-time-code" required/></label></div>
                  <div class="form-actions"><button type="button" class="secondary" onclick={() => showDisableTwoFactor = false}>Cancel</button><button class="danger-solid" disabled={twoFactorBusy}>{twoFactorBusy ? 'Disabling…' : 'Disable 2FA'}</button></div>
                </form>
              {/if}
            {:else if setupSecret}
              <div class="setup-flow">
                <div class="step-copy"><span>1</span><div><b>Add Selfhost to your authenticator</b><p>Choose “enter a setup key,” then use the account email and secret below.</p></div></div>
                <div class="secret-row"><code>{setupSecret}</code><button class="icon-button" aria-label="Copy authenticator secret" onclick={() => copy(setupSecret, 'Authenticator secret')}><Icon name="copy" size={16}/></button></div>
                <details><summary>Advanced: copy provisioning URI</summary><div class="secret-row uri"><code>{setupURI}</code><button class="icon-button" aria-label="Copy provisioning URI" onclick={() => copy(setupURI, 'Provisioning URI')}><Icon name="copy" size={16}/></button></div></details>
                <div class="step-copy"><span>2</span><div><b>Verify the connection</b><p>Enter the six-digit code currently shown by your authenticator.</p></div></div>
                <form class="verify-row" onsubmit={(event) => { event.preventDefault(); confirmTwoFactor(); }}><label>Authentication code<input bind:value={confirmCode} inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="000000" required/></label><button class="primary" disabled={twoFactorBusy}>{twoFactorBusy ? 'Verifying…' : 'Verify and enable'}</button></form>
              </div>
            {:else}
              <div class="card-body split-row"><div class="explanation"><b>Add protection beyond your password.</b><p>Works with 1Password, Bitwarden, Google Authenticator, Authy, and any standard TOTP app.</p></div><button class="primary" onclick={beginTwoFactor} disabled={twoFactorBusy}>{twoFactorBusy ? 'Preparing…' : 'Set up authenticator'}</button></div>
            {/if}
          </section>

          <section class="panel security-panel">
            <header class="security-card-header">
              <div class="header-icon github"><Icon name="github" size={20}/></div>
              <div><h3>GitHub login</h3><p>Link your GitHub identity as an additional way to access Selfhost.</p></div>
              <span class:enabled={security.github.linked} class="state">{security.github.linked ? 'Linked' : 'Not linked'}</span>
            </header>
            {#if !security.providers.github.configured}
              <div class="card-body split-row"><div class="explanation configuration-note" style="margin:0;padding:0;border:0;background:transparent;display:block"><b>Authorize DeployForge on GitHub.</b><p>You will be redirected to GitHub to create and authorize a private GitHub App for this server. No client ID or secret needs to be copied manually.</p></div><a class="primary button-link" href="/api/account/github/start"><Icon name="github" size={16}/>Connect GitHub</a></div>
            {:else if security.github.linked}
              <div class="card-body split-row"><div class="github-account"><span class="avatar"><Icon name="github" size={18}/></span><div><b>@{security.github.login}</b><p>Linked to this Selfhost account</p></div></div><button class="danger-button" onclick={() => showUnlinkGitHub = !showUnlinkGitHub}>Unlink GitHub account</button></div>
              {#if showUnlinkGitHub}
                <form class="confirm-box" onsubmit={(event) => { event.preventDefault(); unlinkGitHub(); }}><div><b>Unlink @{security.github.login}?</b><p>You can still sign in with your email and password.</p></div><div class="two-columns"><label>Current password<input bind:value={unlinkPassword} type="password" autocomplete="current-password" required/></label>{#if security.twoFactorEnabled}<label>Authentication code<input bind:value={unlinkCode} inputmode="numeric" maxlength="6" autocomplete="one-time-code" required/></label>{/if}</div><div class="form-actions"><button type="button" class="secondary" onclick={() => showUnlinkGitHub = false}>Cancel</button><button class="danger-solid" disabled={githubBusy}>{githubBusy ? 'Unlinking…' : 'Unlink account'}</button></div></form>
              {/if}
            {:else}
              <div class="card-body split-row"><div class="explanation"><b>Use your existing GitHub identity.</b><p>{security.providers.github.managed && security.providers.github.appSlug ? `Authorize with ${security.providers.github.appSlug}.` : 'You will be redirected to GitHub to authorize this account.'} Repository access remains a separate permission.</p></div><a class="primary button-link" href="/api/account/github/start"><Icon name="link" size={15}/>Link GitHub account</a></div>
            {/if}
          </section>
        {/if}
      {:else if section === 'platform'}
        <section class="panel">
          <header class="panel-header"><div class="header-icon"><Icon name="server" size={19}/></div><div><small>Control plane</small><h2>Platform security</h2><p>Authentication endpoints derived from this Selfhost installation.</p></div></header>
          <dl class="identity-list"><div><dt>Public URL</dt><dd><code>{location.origin}</code></dd></div><div><dt>Session</dt><dd>HTTP-only cookie · 12 hours</dd></div><div><dt>GitHub callback</dt><dd><code>{security.providers.github.callbackUrl || `${location.origin}/api/integrations/oauth/github/callback`}</code></dd></div></dl>
        </section>
      {:else if smtpLoading}
        <div class="loading">Loading SMTP configuration…</div>
      {:else}
        <div class="security-heading"><small>Outbound email</small><h2>SMTP</h2><p>Send account recovery links and deployment notifications from this server.</p></div>
        <form class="panel smtp-panel" onsubmit={(event) => { event.preventDefault(); saveSMTP(); }}>
          <header class="security-card-header"><div class="header-icon"><Icon name="mail" size={19}/></div><div><h3>Mail server</h3><p>PostgreSQL is the source of truth. Docker Compose can create this record once, but restarts never overwrite it.</p></div><span class:enabled={smtp.configured && smtp.enabled} class="state">{smtp.configured && smtp.enabled ? 'Active' : smtp.configured ? 'Disabled' : 'Not configured'}</span></header>
          <div class="smtp-body">
            <label class="smtp-enable"><input type="checkbox" bind:checked={smtp.enabled}/><span><b>Enable outbound email</b><small>Password recovery and selected notifications can use this SMTP connection.</small></span></label>
            <div class="smtp-grid">
              <label>SMTP hostname<input bind:value={smtp.host} placeholder="smtp.example.com" spellcheck="false" required/></label>
              <label>Port<input bind:value={smtp.port} type="number" min="1" max="65535" required/></label>
              <label>Encryption<select bind:value={smtp.encryption}><option value="starttls">STARTTLS · usually 587</option><option value="tls">Implicit TLS · usually 465</option><option value="none">None · private networks only</option></select></label>
              <label>Username <em>optional</em><input bind:value={smtp.username} autocomplete="username" spellcheck="false" placeholder="apikey or user@example.com"/></label>
              <label class="wide">Password <em>optional</em><input bind:value={smtp.password} type="password" autocomplete="new-password" placeholder={smtp.hasPassword ? 'Stored securely · leave blank to keep it' : 'SMTP password or API key'}/><small>{smtp.hasPassword ? 'A password is already encrypted and stored. Enter a new value only to replace it.' : 'Leave blank when the SMTP server does not require authentication.'}</small></label>
              <label>Sender name<input bind:value={smtp.fromName} maxlength="100" placeholder="DeployForge" required/></label>
              <label>Sender email<input bind:value={smtp.fromEmail} type="email" autocomplete="email" placeholder="deploy@yourdomain.com" required/></label>
            </div>
            <div class="smtp-section"><div><b>Email notifications</b><p>Choose which deployment events should be delivered to the owner email.</p></div><div class="notification-toggles"><label><input type="checkbox" bind:checked={smtp.notifyDeploymentFailures}/><span><b>Failed deployments</b><small>Recommended</small></span></label><label><input type="checkbox" bind:checked={smtp.notifyDeploymentSuccesses}/><span><b>Successful deployments</b><small>Optional</small></span></label></div></div>
          </div>
          <footer class="smtp-footer"><span>Reset links expire after 30 minutes and can only be used once.</span><button class="primary" disabled={smtpSaving}>{smtpSaving ? 'Saving…' : 'Save SMTP settings'}</button></footer>
        </form>

        <section class="panel smtp-test-panel">
          <header class="security-card-header"><div class="header-icon"><Icon name="check" size={19}/></div><div><h3>Test delivery</h3><p>Save the configuration first, then verify it using a real inbox.</p></div></header>
          <form class="smtp-test" onsubmit={(event) => { event.preventDefault(); testSMTP(); }}><label>Recipient<input bind:value={smtpTestRecipient} type="email" autocomplete="email" required/></label><button class="secondary" disabled={smtpTesting || !smtp.configured}>{smtpTesting ? 'Sending…' : 'Send test email'}</button></form>
        </section>
      {/if}
    </div>
  </div>
</Shell>

<style>
  .settings-layout{display:grid;gap:20px;align-items:start}.settings-nav{display:flex;align-items:flex-end;gap:24px;border-bottom:1px solid var(--line);overflow-x:auto;scrollbar-width:none}.settings-nav::-webkit-scrollbar{display:none}.settings-nav button{height:42px;border:0;background:transparent;color:var(--muted);padding:0 1px;display:flex;align-items:center;gap:8px;font-size:11px;font-weight:650;white-space:nowrap;cursor:pointer;position:relative}.settings-nav button::after{content:'';position:absolute;left:0;right:0;bottom:-1px;height:2px;border-radius:2px 2px 0 0;background:transparent}.settings-nav button:hover{color:var(--ink)}.settings-nav button.active{color:var(--ink)}.settings-nav button.active::after{background:var(--accent)}.settings-nav button.active :global(svg){color:var(--accent)}.settings-nav button:focus-visible{outline:2px solid var(--color-focus);outline-offset:-2px;border-radius:4px}.settings-content{min-width:0;display:grid;gap:14px}.panel{border:1px solid var(--line);border-radius:12px;background:var(--surface);box-shadow:var(--shadow-panel);overflow:hidden}.panel-header{padding:22px;display:flex;align-items:flex-start;gap:13px;border-bottom:1px solid var(--line)}.header-icon{width:38px;height:38px;border-radius:9px;background:var(--surface2);color:var(--muted);display:grid;place-items:center;flex:0 0 auto}.header-icon.github{background:var(--ink);color:var(--surface)}.panel-header small,.security-heading small{font:9px var(--font-mono);color:var(--accent);text-transform:uppercase;letter-spacing:.1em}.panel h2,.security-heading h2{font-size:18px;letter-spacing:-.025em;margin:4px 0}.panel-header p,.security-heading p{font-size:11px;color:var(--muted);margin:0;line-height:1.55}.identity-list{margin:0;padding:8px 22px}.identity-list>div{min-height:53px;display:flex;align-items:center;justify-content:space-between;gap:20px;border-bottom:1px solid var(--line)}.identity-list>div:last-child{border:0}.identity-list dt{font-size:11px;color:var(--muted)}.identity-list dd{margin:0;font-size:11px;text-align:right;overflow-wrap:anywhere}.identity-list code{font:10px var(--font-mono)}.role{font:9px var(--font-mono);text-transform:uppercase;color:var(--accent);padding:4px 7px;background:var(--accent-soft);border-radius:99px}.panel-footer{padding:16px 22px;border-top:1px solid var(--line);display:flex;justify-content:flex-end}.security-heading{padding:4px 2px 5px}.security-heading h2{font-size:24px}.security-panel{overflow:visible}.security-card-header{padding:18px 20px;display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:13px;border-bottom:1px solid var(--line)}.security-card-header h3{font-size:14px;margin:0 0 3px}.security-card-header p{font-size:10px;color:var(--muted);margin:0;line-height:1.45}.state{font:8px var(--font-mono);letter-spacing:.06em;text-transform:uppercase;border:1px solid var(--line);color:var(--muted);padding:5px 8px;border-radius:99px;white-space:nowrap}.state.enabled{color:var(--accent);border-color:color-mix(in srgb,var(--accent) 35%,var(--line));background:var(--accent-soft)}.state.neutral{color:var(--ink)}.form-grid,.setup-flow{padding:18px 20px;display:grid;gap:15px}.two-columns{display:grid;grid-template-columns:1fr 1fr;gap:12px}label{display:grid;gap:7px;font-size:10px;font-weight:650;color:var(--ink)}input{height:40px;border:1px solid var(--line2);border-radius:7px;background:var(--surface);color:var(--ink);padding:0 11px;font-size:11px;outline:none}input:focus{border-color:var(--color-focus);box-shadow:0 0 0 3px color-mix(in srgb,var(--color-focus) 14%,transparent)}.code-field{max-width:240px}.form-actions{display:flex;justify-content:flex-end;gap:8px}.primary,.secondary,.danger-button,.danger-solid,.icon-button{height:37px;border-radius:7px;padding:0 13px;font-size:10px;font-weight:700;display:inline-flex;align-items:center;justify-content:center;gap:7px;cursor:pointer;text-decoration:none}.primary{border:1px solid var(--accent);background:var(--accent);color:var(--color-accent-ink)}.primary:hover{background:var(--color-accent-hover)}.primary:disabled,.danger-solid:disabled{opacity:.55;cursor:wait}.secondary{border:1px solid var(--line);background:var(--surface2);color:var(--ink)}.danger-button{border:1px solid color-mix(in srgb,var(--red) 40%,var(--line));background:transparent;color:var(--red)}.danger-solid{border:1px solid var(--red);background:var(--red);color:white}.button-link{white-space:nowrap}.card-body{padding:18px 20px}.split-row{display:flex;align-items:center;justify-content:space-between;gap:22px}.explanation b,.github-account b{font-size:11px}.explanation p,.github-account p,.step-copy p,.confirm-box p,.configuration-note p{font-size:10px;color:var(--muted);line-height:1.5;margin:4px 0 0}.step-copy{display:flex;gap:11px;align-items:flex-start}.step-copy>span{width:24px;height:24px;border-radius:50%;background:var(--accent-soft);color:var(--accent);display:grid;place-items:center;font:700 9px var(--font-mono);flex:0 0 auto}.step-copy b,.confirm-box b,.configuration-note b{font-size:11px}.secret-row{display:flex;align-items:center;border:1px solid var(--line);border-radius:8px;background:var(--surface2);min-width:0}.secret-row code{padding:12px;flex:1;font:11px var(--font-mono);letter-spacing:.08em;overflow-wrap:anywhere;min-width:0}.secret-row.uri code{font-size:9px;letter-spacing:0}.icon-button{width:38px;padding:0;border:0;border-left:1px solid var(--line);border-radius:0;background:transparent;color:var(--muted)}details{font-size:10px;color:var(--muted)}details summary{cursor:pointer;margin-bottom:8px}.verify-row{display:flex;align-items:end;gap:10px}.verify-row label{max-width:220px;flex:1}.confirm-box,.configuration-note{margin:0 20px 18px;padding:15px;border:1px solid color-mix(in srgb,var(--red) 30%,var(--line));border-radius:9px;background:color-mix(in srgb,var(--red) 4%,var(--surface));display:grid;gap:13px}.configuration-note{border-color:color-mix(in srgb,var(--amber) 35%,var(--line));background:color-mix(in srgb,var(--amber) 5%,var(--surface));margin-top:18px}.github-account{display:flex;align-items:center;gap:10px}.avatar{width:34px;height:34px;border-radius:8px;background:var(--ink);color:var(--surface);display:grid;place-items:center}.feedback{min-height:42px;padding:9px 12px;border-radius:8px;display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:9px;font-size:10px}.feedback.success{border:1px solid color-mix(in srgb,var(--accent) 35%,var(--line));background:var(--accent-soft);color:var(--accent)}.feedback.danger{border:1px solid color-mix(in srgb,var(--red) 35%,var(--line));background:var(--color-danger-soft);color:var(--red)}.feedback>button{border:0;background:transparent;color:inherit;font-size:17px;cursor:pointer}.loading{padding:50px;text-align:center;color:var(--muted);font:10px var(--font-mono)}@media(max-width:780px){.settings-nav{gap:20px}.settings-nav button{min-width:max-content}.two-columns{grid-template-columns:1fr}.split-row{align-items:flex-start;flex-direction:column}.security-card-header{grid-template-columns:auto 1fr}.security-card-header .state{grid-column:2;justify-self:start}.verify-row{align-items:stretch;flex-direction:column}.verify-row label{max-width:none}.button-link{width:100%}}
  .smtp-panel{overflow:visible}.smtp-body{padding:20px;display:grid;gap:20px}.smtp-enable{padding:14px;display:flex;align-items:flex-start;gap:11px;border:1px solid color-mix(in srgb,var(--accent) 28%,var(--line));border-radius:9px;background:var(--accent-soft);cursor:pointer}.smtp-enable input,.notification-toggles input{width:16px;height:16px;margin:1px 0 0;accent-color:var(--accent);flex:0 0 auto}.smtp-enable span,.notification-toggles span{display:grid;gap:3px}.smtp-enable b,.smtp-section b,.notification-toggles b{font-size:11px}.smtp-enable small,.notification-toggles small{color:var(--muted);font-size:9px;font-weight:500;line-height:1.45}.smtp-grid{display:grid;grid-template-columns:2fr .7fr;gap:14px}.smtp-grid label{align-content:start}.smtp-grid .wide{grid-column:1/-1}.smtp-grid em{color:var(--muted);font-size:9px;font-style:normal;font-weight:500}.smtp-grid select{height:40px;border:1px solid var(--line2);border-radius:7px;background:var(--surface);color:var(--ink);padding:0 10px;font-size:11px;outline:none}.smtp-grid select:focus{border-color:var(--color-focus);box-shadow:0 0 0 3px color-mix(in srgb,var(--color-focus) 14%,transparent)}.smtp-grid label>small{color:var(--muted);font-size:9px;font-weight:500;line-height:1.45}.smtp-section{padding-top:18px;display:grid;grid-template-columns:minmax(180px,.7fr) minmax(0,1.3fr);gap:20px;border-top:1px solid var(--line)}.smtp-section p{margin:4px 0 0;color:var(--muted);font-size:10px;line-height:1.5}.notification-toggles{display:grid;grid-template-columns:1fr 1fr;gap:9px}.notification-toggles label{min-height:58px;padding:12px;display:flex;align-items:flex-start;gap:9px;border:1px solid var(--line);border-radius:8px;background:var(--surface2);cursor:pointer}.smtp-footer{padding:15px 20px;display:flex;align-items:center;justify-content:space-between;gap:15px;border-top:1px solid var(--line);background:var(--surface2)}.smtp-footer>span{color:var(--muted);font:9px var(--font-mono)}.smtp-test-panel{overflow:visible}.smtp-test{padding:18px 20px;display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:10px}.smtp-test .secondary{min-width:140px}.smtp-test .secondary:disabled{opacity:.5;cursor:not-allowed}@media(max-width:780px){.smtp-grid,.smtp-section,.notification-toggles,.smtp-test{grid-template-columns:1fr}.smtp-grid .wide{grid-column:auto}.smtp-footer{align-items:stretch;flex-direction:column}.smtp-footer .primary,.smtp-test .secondary{width:100%}}
</style>
