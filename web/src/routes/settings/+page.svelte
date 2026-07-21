<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import { api, currentUser, logout } from '$lib/auth.js';
  import { themeMode, setTheme } from '$lib/theme.js';
  import { toast } from '$lib/toast.js';

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

  let smtp = { enabled: false, host: '', port: 587, encryption: 'starttls', username: '', password: '', hasPassword: false, fromName: 'Dokyr', fromEmail: '', notifyDeploymentFailures: true, notifyDeploymentSuccesses: false };
  let smtpLoaded = false;
  let smtpLoading = false;
  let smtpSaving = false;
  let smtpTesting = false;
  let smtpTestRecipient = '';

  const sections = [
    { id: 'profile', label: 'Profile', icon: 'user' },
    { id: 'security', label: 'Security', icon: 'shield' },
    { id: 'appearance', label: 'Appearance', icon: 'sun' },
    { id: 'platform', label: 'Platform', icon: 'server' },
    { id: 'smtp', label: 'SMTP', icon: 'mail' }
  ];

  onMount(async () => {
    const query = new URLSearchParams(location.search);
    if (sections.some((item) => item.id === query.get('section'))) section = query.get('section');
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
      const data = await request('/api/settings/smtp', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: smtp.enabled,
          host: smtp.host,
          port: Number(smtp.port),
          encryption: smtp.encryption,
          username: smtp.username,
          password: smtp.password,
          fromName: smtp.fromName,
          fromEmail: smtp.fromEmail,
          notifyDeploymentFailures: smtp.notifyDeploymentFailures,
          notifyDeploymentSuccesses: smtp.notifyDeploymentSuccesses
        })
      });
      smtp = { ...smtp, ...data.settings, password: '' };
      notice = data.message;
      toast.success('SMTP settings saved');
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
      toast.success('Test email sent');
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
      toast.success('Password updated');
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
        method: 'POST',
        body: JSON.stringify({ code: confirmCode })
      });
      confirmCode = '';
      setupSecret = '';
      setupURI = '';
      notice = data.message;
      toast.success('Two-factor authentication enabled');
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
        method: 'DELETE',
        body: JSON.stringify({ password: disablePassword, code: disableCode })
      });
      disablePassword = '';
      disableCode = '';
      showDisableTwoFactor = false;
      notice = data.message;
      toast.success('Two-factor authentication disabled');
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
        method: 'DELETE',
        body: JSON.stringify({ password: unlinkPassword, code: unlinkCode })
      });
      unlinkPassword = '';
      unlinkCode = '';
      showUnlinkGitHub = false;
      notice = data.message;
      toast.success('GitHub account unlinked');
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

<Shell eyebrow="Administration" title="Settings" subtitle="Account, security, appearance, and control-plane configuration.">
  <div class="settings-layout">
    <nav class="settings-nav" aria-label="Settings sections">
      {#each sections as item}
        <button class:active={section === item.id} aria-current={section === item.id ? 'page' : undefined} onclick={() => selectSection(item.id)}>
          <Icon name={item.icon} size={15} /><span>{item.label}</span>
        </button>
      {/each}
    </nav>

    <div class="settings-content">
      {#if notice}
        <div class="alert alert-success"><Icon name="check-circle" size={15} /><div><span>{notice}</span></div><button class="alert-close" aria-label="Dismiss message" onclick={() => (notice = '')}>×</button></div>
      {/if}
      {#if error}
        <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><span>{error}</span></div><button class="alert-close" aria-label="Dismiss error" onclick={() => (error = '')}>×</button></div>
      {/if}

      {#if section === 'profile'}
        <section class="panel">
          <header class="panel-header">
            <div>
              <span class="eyebrow">Account</span>
              <h2>Profile</h2>
            </div>
          </header>
          <dl class="identity-list">
            <div><dt>Name</dt><dd>{$currentUser?.name}</dd></div>
            <div><dt>Email</dt><dd>{$currentUser?.email}</dd></div>
            <div><dt>Role</dt><dd><span class="badge badge-accent">{$currentUser?.role}</span></dd></div>
            <div><dt>Authentication</dt><dd>Password{security.twoFactorEnabled ? ' + authenticator' : ''}{security.github?.linked ? ' · GitHub linked' : ''}</dd></div>
          </dl>
          <footer class="panel-footer">
            <span>Signing out ends the session on this device.</span>
            <button class="btn btn-danger" onclick={logout}><Icon name="logout" size={14} /> Sign out of Dokyr</button>
          </footer>
        </section>
      {:else if section === 'security'}
        {#if loading}
          <div class="panel loading-block"><span class="spinner"></span><span>Loading account security…</span></div>
        {:else}
          <section class="panel">
            <header class="panel-header">
              <div>
                <span class="eyebrow">Account protection</span>
                <h2>Password</h2>
              </div>
              <span class="badge badge-success"><i></i>Configured</span>
            </header>
            <form class="panel-body form-stack" onsubmit={(event) => { event.preventDefault(); updatePassword(); }}>
              <p class="panel-note">Use a unique password with at least 12 characters.</p>
              <label class="field"><span>Current password</span><input class="input" bind:value={currentPassword} type="password" autocomplete="current-password" required /></label>
              <div class="two-columns">
                <label class="field"><span>New password</span><input class="input" bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" required /></label>
                <label class="field"><span>Confirm new password</span><input class="input" bind:value={confirmPassword} type="password" autocomplete="new-password" minlength="12" required /></label>
              </div>
              {#if security.twoFactorEnabled}
                <label class="field code-field"><span>Authentication code</span><input class="input input-mono" bind:value={passwordCode} inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="000000" required /></label>
              {/if}
              <div class="form-actions"><button class="btn btn-primary" disabled={passwordBusy}>{passwordBusy ? 'Updating…' : 'Update password'}</button></div>
            </form>
          </section>

          <section class="panel">
            <header class="panel-header">
              <div>
                <span class="eyebrow">Second factor</span>
                <h2>Two-factor authentication</h2>
              </div>
              <span class="badge" class:badge-success={security.twoFactorEnabled}><i></i>{security.twoFactorEnabled ? 'Enabled' : 'Not enabled'}</span>
            </header>
            {#if security.twoFactorEnabled}
              <div class="panel-body split-row">
                <div class="explanation"><b>Your account has a second factor.</b><p>Password and GitHub sign-ins both require a current authenticator code.</p></div>
                <button class="btn btn-danger" onclick={() => (showDisableTwoFactor = !showDisableTwoFactor)}>Disable 2FA</button>
              </div>
              {#if showDisableTwoFactor}
                <form class="confirm-box" onsubmit={(event) => { event.preventDefault(); disableTwoFactor(); }}>
                  <div><b>Confirm two-factor removal</b><p>Enter your password and a current authenticator code.</p></div>
                  <div class="two-columns">
                    <label class="field"><span>Password</span><input class="input" bind:value={disablePassword} type="password" autocomplete="current-password" required /></label>
                    <label class="field"><span>Authentication code</span><input class="input input-mono" bind:value={disableCode} inputmode="numeric" maxlength="6" autocomplete="one-time-code" required /></label>
                  </div>
                  <div class="form-actions">
                    <button type="button" class="btn" onclick={() => (showDisableTwoFactor = false)}>Cancel</button>
                    <button class="btn btn-danger-solid" disabled={twoFactorBusy}>{twoFactorBusy ? 'Disabling…' : 'Disable 2FA'}</button>
                  </div>
                </form>
              {/if}
            {:else if setupSecret}
              <div class="panel-body setup-flow">
                <div class="step-copy"><span>1</span><div><b>Add Dokyr to your authenticator</b><p>Choose “enter a setup key,” then use the account email and secret below.</p></div></div>
                <div class="secret-row"><code>{setupSecret}</code><button class="icon-copy" aria-label="Copy authenticator secret" onclick={() => copy(setupSecret, 'Authenticator secret')}><Icon name="copy" size={15} /></button></div>
                <details><summary>Advanced: copy provisioning URI</summary><div class="secret-row uri"><code>{setupURI}</code><button class="icon-copy" aria-label="Copy provisioning URI" onclick={() => copy(setupURI, 'Provisioning URI')}><Icon name="copy" size={15} /></button></div></details>
                <div class="step-copy"><span>2</span><div><b>Verify the connection</b><p>Enter the six-digit code currently shown by your authenticator.</p></div></div>
                <form class="verify-row" onsubmit={(event) => { event.preventDefault(); confirmTwoFactor(); }}>
                  <label class="field"><span>Authentication code</span><input class="input input-mono" bind:value={confirmCode} inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="000000" required /></label>
                  <button class="btn btn-primary" disabled={twoFactorBusy}>{twoFactorBusy ? 'Verifying…' : 'Verify and enable'}</button>
                </form>
              </div>
            {:else}
              <div class="panel-body split-row">
                <div class="explanation"><b>Add protection beyond your password.</b><p>Works with 1Password, Bitwarden, Google Authenticator, Authy, and any standard TOTP app.</p></div>
                <button class="btn btn-primary" onclick={beginTwoFactor} disabled={twoFactorBusy}>{twoFactorBusy ? 'Preparing…' : 'Set up authenticator'}</button>
              </div>
            {/if}
          </section>

          <section class="panel">
            <header class="panel-header">
              <div>
                <span class="eyebrow">Identity provider</span>
                <h2>GitHub login</h2>
              </div>
              <span class="badge" class:badge-success={security.github.linked}><i></i>{security.github.linked ? 'Linked' : 'Not linked'}</span>
            </header>
            {#if !security.providers.github.configured}
              <div class="panel-body split-row">
                <div class="explanation"><b>Authorize Dokyr on GitHub.</b><p>You will be redirected to GitHub to create and authorize a private GitHub App for this server. No client ID or secret needs to be copied manually.</p></div>
                <a class="btn btn-primary" href="/api/account/github/start"><Icon name="github" size={15} /> Connect GitHub</a>
              </div>
            {:else if security.github.linked}
              <div class="panel-body split-row">
                <div class="github-account"><span class="github-avatar"><Icon name="github" size={16} /></span><div><b>@{security.github.login}</b><p>Linked to this Dokyr account</p></div></div>
                <button class="btn btn-danger" onclick={() => (showUnlinkGitHub = !showUnlinkGitHub)}>Unlink GitHub account</button>
              </div>
              {#if showUnlinkGitHub}
                <form class="confirm-box" onsubmit={(event) => { event.preventDefault(); unlinkGitHub(); }}>
                  <div><b>Unlink @{security.github.login}?</b><p>You can still sign in with your email and password.</p></div>
                  <div class="two-columns">
                    <label class="field"><span>Current password</span><input class="input" bind:value={unlinkPassword} type="password" autocomplete="current-password" required /></label>
                    {#if security.twoFactorEnabled}
                      <label class="field"><span>Authentication code</span><input class="input input-mono" bind:value={unlinkCode} inputmode="numeric" maxlength="6" autocomplete="one-time-code" required /></label>
                    {/if}
                  </div>
                  <div class="form-actions">
                    <button type="button" class="btn" onclick={() => (showUnlinkGitHub = false)}>Cancel</button>
                    <button class="btn btn-danger-solid" disabled={githubBusy}>{githubBusy ? 'Unlinking…' : 'Unlink account'}</button>
                  </div>
                </form>
              {/if}
            {:else}
              <div class="panel-body split-row">
                <div class="explanation"><b>Use your existing GitHub identity.</b><p>{security.providers.github.managed && security.providers.github.appSlug ? `Authorize with ${security.providers.github.appSlug}.` : 'You will be redirected to GitHub to authorize this account.'} Repository access remains a separate permission.</p></div>
                <a class="btn btn-primary" href="/api/account/github/start"><Icon name="link" size={14} /> Link GitHub account</a>
              </div>
            {/if}
          </section>
        {/if}
      {:else if section === 'appearance'}
        <section class="panel">
          <header class="panel-header">
            <div>
              <span class="eyebrow">Interface</span>
              <h2>Appearance</h2>
            </div>
          </header>
          <div class="panel-body">
            <p class="panel-note">Choose how Dokyr looks on this device. The preference is stored locally.</p>
            <div class="theme-options" role="radiogroup" aria-label="Color theme">
              {#each [
                { id: 'light', icon: 'sun', label: 'Light', text: 'Bright, neutral surfaces' },
                { id: 'dark', icon: 'moon', label: 'Dark', text: 'Low-light control room' },
                { id: 'system', icon: 'monitor', label: 'System', text: 'Follows the OS setting' }
              ] as option}
                <button
                  type="button"
                  role="radio"
                  aria-checked={$themeMode === option.id}
                  class:active={$themeMode === option.id}
                  onclick={() => setTheme(option.id)}
                >
                  <span class="theme-icon"><Icon name={option.icon} size={16} /></span>
                  <span class="theme-text"><b>{option.label}</b><small>{option.text}</small></span>
                  {#if $themeMode === option.id}<Icon name="check" size={15} />{/if}
                </button>
              {/each}
            </div>
          </div>
        </section>
      {:else if section === 'platform'}
        <section class="panel">
          <header class="panel-header">
            <div>
              <span class="eyebrow">Control plane</span>
              <h2>Platform</h2>
            </div>
          </header>
          <dl class="identity-list">
            <div><dt>Public URL</dt><dd><code>{location.origin}</code></dd></div>
            <div><dt>Session</dt><dd>HTTP-only cookie · 12 hours</dd></div>
            <div><dt>GitHub callback</dt><dd><code>{security.providers.github.callbackUrl || `${location.origin}/api/integrations/oauth/github/callback`}</code></dd></div>
          </dl>
        </section>
      {:else if smtpLoading}
        <div class="panel loading-block"><span class="spinner"></span><span>Loading SMTP configuration…</span></div>
      {:else}
        <form class="panel" onsubmit={(event) => { event.preventDefault(); saveSMTP(); }}>
          <header class="panel-header">
            <div>
              <span class="eyebrow">Outbound email</span>
              <h2>Mail server</h2>
            </div>
            <span class="badge" class:badge-success={smtp.configured && smtp.enabled} class:badge-warning={smtp.configured && !smtp.enabled}>
              <i></i>{smtp.configured && smtp.enabled ? 'Active' : smtp.configured ? 'Disabled' : 'Not configured'}
            </span>
          </header>
          <div class="panel-body form-stack">
            <label class="toggle-row">
              <input class="checkbox" type="checkbox" bind:checked={smtp.enabled} />
              <span><b>Enable outbound email</b><small>Password recovery and selected notifications can use this SMTP connection.</small></span>
            </label>
            <div class="smtp-grid">
              <label class="field"><span>SMTP hostname</span><input class="input input-mono" bind:value={smtp.host} placeholder="smtp.example.com" spellcheck="false" required /></label>
              <label class="field"><span>Port</span><input class="input input-mono" bind:value={smtp.port} type="number" min="1" max="65535" required /></label>
              <label class="field"><span>Encryption</span><select class="select" bind:value={smtp.encryption}><option value="starttls">STARTTLS · usually 587</option><option value="tls">Implicit TLS · usually 465</option><option value="none">None · private networks only</option></select></label>
              <label class="field"><span>Username <em>optional</em></span><input class="input input-mono" bind:value={smtp.username} autocomplete="username" spellcheck="false" placeholder="apikey or user@example.com" /></label>
              <label class="field wide"><span>Password <em>optional</em></span><input class="input" bind:value={smtp.password} type="password" autocomplete="new-password" placeholder={smtp.hasPassword ? 'Stored securely · leave blank to keep it' : 'SMTP password or API key'} /><small>{smtp.hasPassword ? 'A password is already encrypted and stored. Enter a new value only to replace it.' : 'Leave blank when the SMTP server does not require authentication.'}</small></label>
              <label class="field"><span>Sender name</span><input class="input" bind:value={smtp.fromName} maxlength="100" placeholder="Dokyr" required /></label>
              <label class="field"><span>Sender email</span><input class="input input-mono" bind:value={smtp.fromEmail} type="email" autocomplete="email" placeholder="deploy@yourdomain.com" required /></label>
            </div>
            <div class="smtp-section">
              <div><b>Email notifications</b><p>Choose which deployment events are delivered to the owner email.</p></div>
              <div class="notification-toggles">
                <label class="toggle-row"><input class="checkbox" type="checkbox" bind:checked={smtp.notifyDeploymentFailures} /><span><b>Failed deployments</b><small>Recommended</small></span></label>
                <label class="toggle-row"><input class="checkbox" type="checkbox" bind:checked={smtp.notifyDeploymentSuccesses} /><span><b>Successful deployments</b><small>Optional</small></span></label>
              </div>
            </div>
          </div>
          <footer class="panel-footer">
            <span>Reset links expire after 30 minutes and can only be used once.</span>
            <button class="btn btn-primary" disabled={smtpSaving}>{smtpSaving ? 'Saving…' : 'Save SMTP settings'}</button>
          </footer>
        </form>

        <section class="panel">
          <header class="panel-header">
            <div>
              <span class="eyebrow">Verification</span>
              <h2>Test delivery</h2>
            </div>
          </header>
          <form class="panel-body smtp-test" onsubmit={(event) => { event.preventDefault(); testSMTP(); }}>
            <label class="field"><span>Recipient</span><input class="input input-mono" bind:value={smtpTestRecipient} type="email" autocomplete="email" required /></label>
            <button class="btn" disabled={smtpTesting || !smtp.configured}>{smtpTesting ? 'Sending…' : 'Send test email'}</button>
          </form>
          <p class="panel-note smtp-test-note">Save the configuration first, then verify it using a real inbox.</p>
        </section>
      {/if}
    </div>
  </div>
</Shell>

<style>
  .settings-layout {
    display: grid;
    grid-template-columns: 200px minmax(0, 1fr);
    gap: var(--space-6);
    align-items: start;
  }
  .settings-nav {
    display: grid;
    gap: 2px;
    position: sticky;
    top: 72px;
  }
  .settings-nav button {
    min-height: 34px;
    padding: 0 var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-muted);
    font-size: var(--text-sm);
    font-weight: 500;
    text-align: left;
    white-space: nowrap;
    cursor: pointer;
  }
  .settings-nav button:hover {
    background: var(--color-paper-subtle);
    color: var(--color-ink);
  }
  .settings-nav button.active {
    background: var(--color-accent-soft);
    color: var(--color-accent);
    font-weight: 600;
  }
  .settings-content {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: var(--space-4);
  }
  .settings-content .alert {
    margin-bottom: 0;
  }
  .loading-block {
    min-height: 200px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .panel-note {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.55;
  }
  .form-stack {
    display: grid;
    gap: var(--space-4);
  }
  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }
  .code-field {
    max-width: 260px;
  }
  .split-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }
  .explanation b {
    font-size: var(--text-md);
  }
  .explanation p {
    max-width: 56ch;
    margin: var(--space-1) 0 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.55;
  }
  .identity-list {
    margin: 0;
    padding: var(--space-2) var(--space-5);
  }
  .identity-list > div {
    min-height: 52px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-5);
    border-bottom: 1px solid var(--color-rule);
  }
  .identity-list > div:last-child {
    border-bottom: 0;
  }
  .identity-list dt {
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .identity-list dd {
    margin: 0;
    min-width: 0;
    overflow: hidden;
    font-size: var(--text-sm);
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .identity-list code {
    font-size: var(--text-sm);
  }
  .panel-footer {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .confirm-box {
    margin: 0 var(--space-5) var(--space-5);
    padding: var(--space-4);
    display: grid;
    gap: var(--space-4);
    border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-danger) 4%, var(--color-paper-raised));
  }
  .confirm-box > div > b {
    font-size: var(--text-sm);
  }
  .confirm-box > div > p {
    margin: var(--space-1) 0 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
  .setup-flow {
    display: grid;
    gap: var(--space-4);
  }
  .step-copy {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr);
    gap: var(--space-3);
    align-items: start;
  }
  .step-copy > span {
    width: 28px;
    height: 28px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule-strong);
    border-radius: 50%;
    color: var(--color-muted);
    font: 600 var(--text-xs) var(--font-mono);
  }
  .step-copy b {
    font-size: var(--text-sm);
  }
  .step-copy p {
    margin: var(--space-1) 0 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.5;
  }
  .secret-row {
    padding: var(--space-3);
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-2);
    border: 1px dashed var(--color-rule-strong);
    border-radius: var(--radius-sm);
    background: var(--color-log-bg);
  }
  .secret-row code {
    overflow: hidden;
    color: var(--color-log-text);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .icon-copy {
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-log-rule);
    border-radius: var(--radius-sm);
    background: var(--color-log-surface);
    color: var(--color-log-muted);
    cursor: pointer;
  }
  .icon-copy:hover {
    color: var(--color-log-text);
  }
  details summary {
    color: var(--color-muted);
    font-size: var(--text-xs);
    cursor: pointer;
  }
  details .secret-row {
    margin-top: var(--space-2);
  }
  .verify-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: var(--space-3);
  }
  .github-account {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .github-avatar {
    width: 36px;
    height: 36px;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--color-log-bg);
    color: var(--color-log-text);
  }
  .github-account b {
    font-size: var(--text-md);
  }
  .github-account p {
    margin: 1px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .theme-options {
    margin-top: var(--space-4);
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-3);
  }
  .theme-options button {
    min-height: 64px;
    padding: var(--space-3);
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    color: var(--color-muted);
    text-align: left;
    cursor: pointer;
  }
  .theme-options button:hover {
    border-color: var(--color-rule-strong);
  }
  .theme-options button.active {
    border-color: var(--color-accent);
    background: var(--color-accent-soft);
    color: var(--color-accent);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 20%, transparent);
  }
  .theme-icon {
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-sm);
    background: var(--color-paper-subtle);
  }
  .theme-options button.active .theme-icon {
    background: var(--color-paper-raised);
  }
  .theme-text {
    display: grid;
    gap: 1px;
  }
  .theme-text b {
    color: var(--color-ink);
    font-size: var(--text-sm);
  }
  .theme-text small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }

  .toggle-row {
    padding: var(--space-3);
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-surface-subtle);
    cursor: pointer;
  }
  .toggle-row input {
    margin-top: 2px;
  }
  .toggle-row span {
    display: grid;
    gap: 2px;
  }
  .toggle-row b {
    font-size: var(--text-sm);
  }
  .toggle-row small {
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.45;
  }
  .smtp-grid {
    display: grid;
    grid-template-columns: 2fr 0.7fr;
    gap: var(--space-4);
  }
  .smtp-grid .wide {
    grid-column: 1 / -1;
  }
  .smtp-section {
    padding-top: var(--space-4);
    display: grid;
    grid-template-columns: minmax(180px, 0.7fr) minmax(0, 1.3fr);
    gap: var(--space-5);
    border-top: 1px solid var(--color-rule);
  }
  .smtp-section > div > b {
    font-size: var(--text-sm);
  }
  .smtp-section p {
    margin: var(--space-1) 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
  }
  .notification-toggles {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-2);
  }
  .smtp-test {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: var(--space-3);
  }
  .smtp-test-note {
    padding: 0 var(--space-5) var(--space-4);
  }

  @media (max-width: 52rem) {
    .settings-layout {
      grid-template-columns: 1fr;
      gap: var(--space-4);
    }
    .settings-nav {
      position: static;
      display: flex;
      gap: 2px;
      overflow-x: auto;
      border-bottom: 1px solid var(--color-rule);
      scrollbar-width: none;
    }
    .settings-nav::-webkit-scrollbar {
      display: none;
    }
    .settings-nav button {
      position: relative;
      border-radius: 0;
    }
    .settings-nav button:hover {
      background: transparent;
    }
    .settings-nav button.active {
      background: transparent;
      box-shadow: inset 0 -2px var(--color-accent);
    }
    .two-columns,
    .smtp-grid,
    .smtp-section,
    .notification-toggles,
    .theme-options {
      grid-template-columns: 1fr;
    }
    .split-row {
      align-items: flex-start;
      flex-direction: column;
    }
    .verify-row,
    .smtp-test {
      grid-template-columns: 1fr;
    }
  }
</style>
