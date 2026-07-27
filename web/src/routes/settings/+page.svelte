<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api, currentUser, currentPermissions, can, logout } from '$lib/auth.js';
  import { themeMode, setTheme } from '$lib/theme.js';
  import { toast } from '$lib/toast.js';
  import { platformUpdate, loadPlatformUpdate, checkPlatformUpdate, formatPlatformVersion } from '$lib/platform.js';

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
  let updateLoading = false;
  let updateChecking = false;
  let updateSaving = false;
  let updateApplying = false;
  let updateSettings = { autoUpdate: false, checkIntervalMinutes: 60, maintenanceHour: 3, timezone: 'UTC' };

  let users = [];
  let assignableRoles = [];
  let usersLoaded = false;
  let usersLoading = false;
  let inviteName = '';
  let inviteEmail = '';
  let inviteRole = 'developer';
  let inviteBusy = false;
  let inviteError = '';
  let invitation = null;
  let roleBusy = '';
  let removeTarget = null;
  let removeBusy = false;
  let removeError = '';

  // Only an owner can manage users, so the section is hidden for everyone else.
  // The server rejects the requests regardless; this keeps the nav honest.
  $: manageUsers = can($currentPermissions, 'user:manage');
  // Platform and SMTP settings share the same owner-only permission. The forms
  // stay visible but disabled for everyone else, instead of only surfacing the
  // rejection after a submit.
  $: canWritePlatform = can($currentPermissions, 'platform:write');
  $: sections = [
    { id: 'profile', label: 'Profile', icon: 'user' },
    { id: 'security', label: 'Security', icon: 'shield' },
    ...(manageUsers ? [{ id: 'users', label: 'Users', icon: 'users' }] : []),
    { id: 'appearance', label: 'Appearance', icon: 'sun' },
    { id: 'platform', label: 'Platform', icon: 'server' },
    { id: 'smtp', label: 'SMTP', icon: 'mail' }
  ];

  const roleSummaries = {
    owner: 'Full control, including domains, proxy configuration, platform updates, and users.',
    admin: 'Everything except ingress, platform settings, and user management.',
    developer: 'Create and deploy projects, read and change environment variables, run container commands.',
    viewer: 'Read-only access to projects, services, logs, and metrics. No secrets.'
  };

  onMount(async () => {
    const query = new URLSearchParams(location.search);
    if (sections.some((item) => item.id === query.get('section'))) section = query.get('section');
    if (query.get('github') === 'linked') notice = 'GitHub account linked. You can now use it to sign in.';
    if (query.get('error')) error = query.get('error');
    await loadSecurity();
    if (section === 'smtp') await loadSMTP();
    if (section === 'platform') await loadUpdateStatus();
    if (section === 'users') await loadUsers();
  });

  async function loadUsers() {
    usersLoading = true;
    try {
      const response = await api('/api/users');
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not load users.');
      users = data.users || [];
      assignableRoles = data.assignableRoles || [];
      if (!assignableRoles.includes(inviteRole)) inviteRole = assignableRoles[0] || 'viewer';
      usersLoaded = true;
    } catch (cause) {
      error = cause.message;
    } finally {
      usersLoading = false;
    }
  }

  async function inviteUser() {
    inviteBusy = true;
    inviteError = '';
    invitation = null;
    try {
      const response = await api('/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: inviteName, email: inviteEmail, role: inviteRole })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not invite this person.');
      invitation = { url: data.invitationUrl, emailSent: data.emailSent, email: inviteEmail, name: inviteName };
      inviteName = '';
      inviteEmail = '';
      await loadUsers();
      toast.success(data.emailSent ? 'Invitation emailed.' : 'Invitation created — share the link.');
    } catch (cause) {
      inviteError = cause.message;
    } finally {
      inviteBusy = false;
    }
  }

  async function resendInvitation(user) {
    roleBusy = user.id;
    try {
      const response = await api(`/api/users/${user.id}/invitation`, { method: 'POST' });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not create a new invitation.');
      invitation = { url: data.invitationUrl, emailSent: data.emailSent, email: user.email, name: user.name };
      toast.success(data.emailSent ? 'A new invitation was emailed.' : 'New invitation link created.');
    } catch (cause) {
      toast.error(cause.message);
    } finally {
      roleBusy = '';
    }
  }

  async function changeRole(user, role) {
    if (role === user.role) return;
    roleBusy = user.id;
    try {
      const response = await api(`/api/users/${user.id}/role`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not change this role.');
      toast.success(`${user.name} is now ${role}.`);
      await loadUsers();
    } catch (cause) {
      toast.error(cause.message);
      await loadUsers();
    } finally {
      roleBusy = '';
    }
  }

  async function removeUser() {
    removeBusy = true;
    removeError = '';
    try {
      const response = await api(`/api/users/${removeTarget.id}`, { method: 'DELETE' });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Could not remove this account.');
      toast.success(`${removeTarget.name} was removed.`);
      removeTarget = null;
      await loadUsers();
    } catch (cause) {
      removeError = cause.message;
    } finally {
      removeBusy = false;
    }
  }

  async function copyInvitation() {
    try {
      await navigator.clipboard.writeText(invitation.url);
      toast.success('Invitation link copied');
    } catch {
      toast.error('Could not copy the invitation link');
    }
  }

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
    if (next === 'platform') loadUpdateStatus();
    if (next === 'users' && !usersLoaded) loadUsers();
  }

  function formatJoined(value) {
    if (!value) return '—';
    return new Date(value).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  }

  async function loadUpdateStatus(refresh = false) {
    updateLoading = true;
    try {
      const data = await loadPlatformUpdate(refresh);
      updateSettings = { ...updateSettings, ...data.settings };
    } catch (cause) {
      error = cause.message;
    } finally {
      updateLoading = false;
    }
  }

  async function checkForUpdate() {
    updateChecking = true;
    try {
      const data = await checkPlatformUpdate();
      updateSettings = { ...updateSettings, ...data.settings };
      toast.success(data.updateAvailable ? `Dokyr ${data.latest.version} is available` : 'Dokyr is up to date');
    } catch (cause) {
      error = cause.message;
    } finally {
      updateChecking = false;
    }
  }

  async function saveUpdateSettings() {
    updateSaving = true;
    try {
      const data = await request('/api/settings/platform/update', {
        method: 'PUT',
        body: JSON.stringify(updateSettings)
      });
      updateSettings = { ...updateSettings, ...data };
      platformUpdate.update((state) => state ? { ...state, settings: data } : state);
      toast.success('Update policy saved');
    } catch (cause) {
      error = cause.message;
    } finally {
      updateSaving = false;
    }
  }

  async function applyUpdate() {
    const target = $platformUpdate?.latest?.version || 'the latest release';
    if (!confirm(`Update Dokyr to ${target}? The control panel will reconnect briefly while the new container is verified.`)) return;
    updateApplying = true;
    try {
      const data = await request('/api/settings/platform/update/apply', { method: 'POST', body: '{}' });
      notice = data.message;
      toast.success('Platform update started');
      watchPlatformRestart(target);
    } catch (cause) {
      error = cause.message;
      updateApplying = false;
    }
  }

  async function watchPlatformRestart(target) {
    const deadline = Date.now() + 180000;
    while (Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 2500));
      try {
        const data = await loadPlatformUpdate();
        if (data.current?.version === target || data.job?.status === 'failed') {
          updateApplying = false;
          if (data.job?.status === 'failed') error = data.job.message || 'The update failed and the previous version was restored.';
          else {
            toast.success(`Dokyr ${target} is running`);
            location.reload();
          }
          return;
        }
      } catch {
        // A short connection gap is expected while the control-plane container is exchanged.
      }
    }
    updateApplying = false;
    error = 'The update is taking longer than expected. Refresh to check the current platform status.';
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
      {:else if section === 'users'}
        <form class="panel" onsubmit={(event) => { event.preventDefault(); inviteUser(); }}>
          <header class="panel-header">
            <div><span class="eyebrow">Team</span><h2>Invite someone</h2></div>
          </header>
          <div class="panel-body form-stack">
            <div class="invite-grid">
              <label class="field"><span>Name</span><input class="input" bind:value={inviteName} maxlength="120" placeholder="Jordan Lee" required /></label>
              <label class="field"><span>Email</span><input class="input input-mono" bind:value={inviteEmail} type="email" autocomplete="off" spellcheck="false" placeholder="jordan@example.com" required /></label>
              <label class="field"><span>Role</span>
                <select class="select" bind:value={inviteRole}>
                  {#each assignableRoles as role}<option value={role}>{role}</option>{/each}
                </select>
              </label>
            </div>
            <p class="role-summary"><Icon name="info" size={14} /><span>{roleSummaries[inviteRole] || ''}</span></p>
            {#if inviteError}<div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Invitation failed</strong><span>{inviteError}</span></div></div>{/if}
          </div>
          <footer class="panel-footer">
            <span>They choose their own password from a one-time link valid for 7 days.</span>
            <button class="btn btn-primary" disabled={inviteBusy || !inviteName.trim() || !inviteEmail.trim()}>{inviteBusy ? 'Inviting…' : 'Send invitation'}</button>
          </footer>
        </form>

        {#if invitation}
          <div class="panel invitation-reveal">
            <header>
              <div class="reveal-icon"><Icon name="check" size={16} /></div>
              <div>
                <strong>Invitation ready for {invitation.name || invitation.email}</strong>
                <span>{invitation.emailSent ? 'An email was sent. You can also share this link directly.' : 'No mail server is configured, so share this link yourself.'}</span>
              </div>
              <button type="button" aria-label="Dismiss invitation link" onclick={() => { invitation = null; }}>×</button>
            </header>
            <div class="invitation-link">
              <code>{invitation.url}</code>
              <button type="button" onclick={copyInvitation}><Icon name="copy" size={14} />Copy</button>
            </div>
            <p>Anyone with this link can set the password for that account, so send it over a private channel.</p>
          </div>
        {/if}

        <section class="panel">
          <header class="panel-header">
            <div><span class="eyebrow">Accounts</span><h2>People with access</h2></div>
            <button type="button" class="btn btn-ghost" onclick={() => loadUsers()} disabled={usersLoading}><Icon name="refresh" size={14} />Refresh</button>
          </header>
          {#if usersLoading && !usersLoaded}
            <div class="loading-block"><span class="spinner"></span><span>Loading accounts…</span></div>
          {:else}
            <div class="user-list">
              <div class="user-list-header"><span>Person</span><span>Role</span><span>Status</span><span>Added</span><span></span></div>
              {#each users as user}
                <article>
                  <div class="user-identity">
                    <span class="user-avatar"><Icon name="user" size={15} /></span>
                    <div><strong>{user.name}</strong><code>{user.email}</code></div>
                  </div>
                  {#if user.id === $currentUser?.id}
                    <span class="role-static">{user.role}<em>you</em></span>
                  {:else}
                    <select class="select select-compact" value={user.role} disabled={roleBusy === user.id} onchange={(event) => changeRole(user, event.currentTarget.value)}>
                      {#each assignableRoles as role}<option value={role}>{role}</option>{/each}
                    </select>
                  {/if}
                  <span class="status-pill" class:pending={user.mustSetPassword}>{user.mustSetPassword ? 'Invited' : 'Active'}</span>
                  <time>{formatJoined(user.createdAt)}</time>
                  <div class="user-actions">
                    {#if user.mustSetPassword}
                      <button type="button" class="ghost-action" disabled={roleBusy === user.id} onclick={() => resendInvitation(user)}><Icon name="mail" size={14} />Resend</button>
                    {/if}
                    {#if user.id !== $currentUser?.id}
                      <button type="button" class="ghost-action danger" aria-label={`Remove ${user.name}`} onclick={() => { removeTarget = user; removeError = ''; }}><Icon name="trash" size={14} />Remove</button>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          {/if}
          <p class="panel-note">You cannot change or remove your own account here, and the last owner cannot be removed or demoted.</p>
        </section>
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
        {#if updateLoading && !$platformUpdate}
          <div class="panel loading-block"><span class="spinner"></span><span>Reading the running Dokyr release…</span></div>
        {:else}
        <section class="panel platform-release" class:update-available={$platformUpdate?.updateAvailable} class:panel-disabled={!canWritePlatform}>
          <header class="panel-header">
            <div>
              <span class="eyebrow">Control plane</span>
              <h2>Dokyr {formatPlatformVersion($platformUpdate?.current?.version, 'Development')}</h2>
            </div>
            <span class="badge" class:badge-warning={$platformUpdate?.updateAvailable} class:badge-success={!$platformUpdate?.updateAvailable && $platformUpdate?.updateSupported}>
              <i></i>{$platformUpdate?.updateAvailable ? 'Update available' : $platformUpdate?.updateSupported ? 'Up to date' : 'Development build'}
            </span>
          </header>
          <div class="release-body">
            <div class="release-track" aria-label="Platform release comparison">
              <div>
                <span>RUNNING</span>
                <strong>{formatPlatformVersion($platformUpdate?.current?.version, 'Development')}</strong>
                <small>Installed version</small>
              </div>
              <span class="release-line" class:active={$platformUpdate?.updateAvailable}><i></i><Icon name="arrow-right" size={15} /><i></i></span>
              <div>
                <span>LATEST</span>
                <strong>{formatPlatformVersion($platformUpdate?.latest?.version, $platformUpdate?.updateSupported ? 'Unavailable' : 'Not checked')}</strong>
                <small>{$platformUpdate?.updateSupported ? 'Stable channel' : 'Development build'}</small>
              </div>
            </div>
            {#if $platformUpdate?.error}
              <div class="alert alert-warning"><Icon name="alert" size={15} /><div><strong>Update check needs attention</strong><span>{$platformUpdate.error}</span></div></div>
            {/if}
            {#if $platformUpdate?.job}
              <div class="update-job">
                <span class="job-state" class:running={['pending','pulling','restarting'].includes($platformUpdate.job.status)}></span>
                <div><b>Last update · {$platformUpdate.job.status}</b><p>{$platformUpdate.job.message || `${$platformUpdate.job.sourceVersion} → ${$platformUpdate.job.targetVersion}`}</p></div>
              </div>
            {/if}
          </div>
          <footer class="panel-footer">
            <span>{updateApplying ? 'The page will reconnect automatically after verification.' : $platformUpdate?.updateSupported ? 'The current container remains available for automatic rollback.' : 'Self-update is disabled for development builds.'}</span>
            <div class="release-actions">
              {#if $platformUpdate?.updateSupported}
                <button class="btn" onclick={checkForUpdate} disabled={updateChecking || updateApplying || !canWritePlatform}><Icon name="refresh" size={14} /> {updateChecking ? 'Checking…' : 'Check again'}</button>
              {/if}
              {#if $platformUpdate?.updateAvailable}
                <button class="btn btn-primary" onclick={applyUpdate} disabled={updateApplying || !$platformUpdate?.updateSupported || !canWritePlatform}>
                  <Icon name="arrow-right" size={14} /> {updateApplying ? 'Updating…' : `Update to ${$platformUpdate.latest.version}`}
                </button>
              {/if}
            </div>
          </footer>
        </section>

        <form class="panel" class:panel-disabled={!canWritePlatform} onsubmit={(event) => { event.preventDefault(); saveUpdateSettings(); }}>
          <header class="panel-header">
            <div><span class="eyebrow">Release policy</span><h2>Automatic updates</h2></div>
            <span class="badge" class:badge-success={updateSettings.autoUpdate}><i></i>{updateSettings.autoUpdate ? 'Enabled' : 'Manual'}</span>
          </header>
          <fieldset class="panel-fieldset" disabled={!canWritePlatform}>
            <div class="panel-body form-stack">
              <label class="toggle-row">
                <input class="checkbox" type="checkbox" bind:checked={updateSettings.autoUpdate} />
                <span><b>Install stable releases automatically</b><small>Dokyr checks the configured registry channel and uses the same verified rollback flow as a manual update.</small></span>
              </label>
              <div class="three-columns update-policy-fields">
                <label class="field"><span>Check frequency</span><select class="select" bind:value={updateSettings.checkIntervalMinutes}><option value={15}>Every 15 minutes</option><option value={60}>Every hour</option><option value={360}>Every 6 hours</option><option value={1440}>Every day</option></select></label>
                <label class="field"><span>Maintenance hour</span><select class="select" bind:value={updateSettings.maintenanceHour}>{#each Array(24) as _, hour}<option value={hour}>{String(hour).padStart(2, '0')}:00</option>{/each}</select></label>
                <label class="field"><span>Timezone</span><input class="input input-mono" bind:value={updateSettings.timezone} placeholder="UTC" required /></label>
              </div>
              <p class="panel-note">Automatic installation begins only during the selected hour. Managed applications, databases, and Caddy continue running.</p>
            </div>
            <footer class="panel-footer"><span>{canWritePlatform ? 'Automatic updates are disabled by default.' : 'Only the owner can change platform settings.'}</span><button class="btn btn-primary" disabled={updateSaving}>{updateSaving ? 'Saving…' : 'Save update policy'}</button></footer>
          </fieldset>
        </form>

        <section class="panel">
          <header class="panel-header"><div><span class="eyebrow">Runtime identity</span><h2>Platform details</h2></div></header>
          <dl class="identity-list">
            <div><dt>Image</dt><dd><code>{$platformUpdate?.currentImage || 'Local development process'}</code></dd></div>
            <div><dt>Image digest</dt><dd><code>{$platformUpdate?.currentDigest ? `${$platformUpdate.currentDigest.slice(0, 24)}…` : 'Not available'}</code></dd></div>
            <div><dt>Build date</dt><dd>{$platformUpdate?.current?.buildDate || 'Unknown'}</dd></div>
            <div><dt>Public URL</dt><dd><code>{location.origin}</code></dd></div>
          </dl>
        </section>
        {/if}
      {:else if smtpLoading}
        <div class="panel loading-block"><span class="spinner"></span><span>Loading SMTP configuration…</span></div>
      {:else}
        <form class="panel" class:panel-disabled={!canWritePlatform} onsubmit={(event) => { event.preventDefault(); saveSMTP(); }}>
          <header class="panel-header">
            <div>
              <span class="eyebrow">Outbound email</span>
              <h2>Mail server</h2>
            </div>
            <span class="badge" class:badge-success={smtp.configured && smtp.enabled} class:badge-warning={smtp.configured && !smtp.enabled}>
              <i></i>{smtp.configured && smtp.enabled ? 'Active' : smtp.configured ? 'Disabled' : 'Not configured'}
            </span>
          </header>
          <fieldset class="panel-fieldset" disabled={!canWritePlatform}>
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
              <span>{canWritePlatform ? 'Reset links expire after 30 minutes and can only be used once.' : 'Only the owner can change SMTP settings.'}</span>
              <button class="btn btn-primary" disabled={smtpSaving}>{smtpSaving ? 'Saving…' : 'Save SMTP settings'}</button>
            </footer>
          </fieldset>
        </form>

        <section class="panel" class:panel-disabled={!canWritePlatform}>
          <header class="panel-header">
            <div>
              <span class="eyebrow">Verification</span>
              <h2>Test delivery</h2>
            </div>
          </header>
          <fieldset class="panel-fieldset" disabled={!canWritePlatform}>
            <form class="panel-body smtp-test" onsubmit={(event) => { event.preventDefault(); testSMTP(); }}>
              <label class="field"><span>Recipient</span><input class="input input-mono" bind:value={smtpTestRecipient} type="email" autocomplete="email" required /></label>
              <button class="btn" disabled={smtpTesting || !smtp.configured}>{smtpTesting ? 'Sending…' : 'Send test email'}</button>
            </form>
          </fieldset>
          <p class="panel-note smtp-test-note">{canWritePlatform ? 'Save the configuration first, then verify it using a real inbox.' : 'Only the owner can send a test email.'}</p>
        </section>
      {/if}
    </div>
  </div>
</Shell>

{#if removeTarget}
  <ConfirmDialog
    title={`Remove ${removeTarget.name}?`}
    message={`${removeTarget.email} loses access immediately, including any session they already have open. Projects and deployments they created are kept.`}
    confirmLabel="Remove account"
    busy={removeBusy}
    error={removeError}
    onConfirm={removeUser}
    onClose={() => { if (!removeBusy) { removeTarget = null; removeError = ''; } }}
  />
{/if}

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
  .panel-fieldset {
    border: none;
    margin: 0;
    padding: 0;
    min-width: 0;
  }
  .panel-fieldset:disabled .btn,
  .panel-fieldset:disabled .input,
  .panel-fieldset:disabled .select,
  .panel-fieldset:disabled .checkbox {
    cursor: not-allowed;
  }
  .panel-disabled {
    opacity: 0.55;
  }
  .invite-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr) minmax(120px, .6fr); gap: var(--space-4); }
  .role-summary { margin: 0; display: flex; align-items: flex-start; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .role-summary > :global(svg) { flex: 0 0 auto; margin-top: 2px; }
  .invitation-reveal { overflow: hidden; }
  .invitation-reveal header { padding: var(--space-4); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .reveal-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 50%; background: var(--color-success); color: white; }
  .invitation-reveal header > div:nth-child(2) { min-width: 0; display: grid; gap: 2px; }
  .invitation-reveal header strong { font-size: var(--text-sm); }
  .invitation-reveal header span, .invitation-reveal > p { color: var(--color-muted); font-size: var(--text-xs); }
  .invitation-reveal header > button { width: 28px; height: 28px; border: 0; background: transparent; color: var(--color-muted); font-size: 20px; cursor: pointer; }
  .invitation-link { margin: var(--space-4); display: grid; grid-template-columns: minmax(0, 1fr) auto; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-log-bg); }
  .invitation-link code { min-width: 0; padding: 10px var(--space-3); overflow: auto; color: var(--color-log-text); font-size: var(--text-xs); white-space: nowrap; }
  .invitation-link button { padding: 0 var(--space-3); display: flex; align-items: center; gap: 5px; border: 0; border-left: 1px solid var(--color-log-rule); background: transparent; color: var(--color-log-muted); font-size: var(--text-xs); cursor: pointer; }
  .invitation-link button:hover { color: var(--color-log-text); }
  .invitation-reveal > p { margin: 0; padding: 0 var(--space-4) var(--space-4); }
  .user-list { display: grid; }
  .user-list-header, .user-list article { padding: 0 var(--space-5); display: grid; grid-template-columns: minmax(200px, 1.5fr) minmax(120px, .7fr) minmax(84px, .5fr) minmax(110px, .6fr) minmax(180px, auto); align-items: center; gap: var(--space-3); }
  .user-list-header { min-height: 34px; border-top: 1px solid var(--color-rule); border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; text-transform: uppercase; letter-spacing: .05em; }
  .user-list article { min-height: 68px; border-bottom: 1px solid var(--color-rule); }
  .user-list article:last-child { border-bottom: 0; }
  .user-identity { min-width: 0; display: flex; align-items: center; gap: var(--space-3); }
  .user-avatar { width: 32px; height: 32px; flex: 0 0 auto; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; background: var(--color-surface-subtle); color: var(--color-muted); }
  .user-identity > div { min-width: 0; display: grid; gap: 2px; }
  .user-identity strong { overflow: hidden; font-size: var(--text-sm); text-overflow: ellipsis; white-space: nowrap; }
  .user-identity code { overflow: hidden; color: var(--color-muted); font-size: var(--text-2xs); text-overflow: ellipsis; white-space: nowrap; }
  .select-compact { min-height: 32px; padding: 0 var(--space-2); font-size: var(--text-xs); text-transform: capitalize; }
  .role-static { display: flex; align-items: center; gap: 6px; color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; text-transform: capitalize; }
  .role-static em { padding: 2px 6px; border: 1px solid var(--color-rule); border-radius: 999px; color: var(--color-muted); font-size: var(--text-2xs); font-style: normal; font-weight: 700; text-transform: uppercase; }
  .status-pill { width: max-content; padding: 4px 7px; border: 1px solid color-mix(in srgb, var(--color-success) 28%, var(--color-rule)); border-radius: 999px; background: color-mix(in srgb, var(--color-success) 10%, transparent); color: var(--color-success); font-size: var(--text-2xs); font-weight: 700; }
  .status-pill.pending { border-color: color-mix(in srgb, var(--color-warning) 34%, var(--color-rule)); background: color-mix(in srgb, var(--color-warning) 12%, transparent); color: var(--color-warning); }
  .user-list time { color: var(--color-muted); font-size: var(--text-xs); }
  .user-actions { display: flex; justify-content: flex-end; gap: var(--space-2); }
  .ghost-action { min-height: 30px; padding: 0 var(--space-2); display: flex; align-items: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); font-size: var(--text-xs); cursor: pointer; }
  .ghost-action:hover:not(:disabled) { border-color: var(--color-accent); color: var(--color-accent); }
  .ghost-action.danger:hover:not(:disabled) { border-color: var(--color-danger); color: var(--color-danger); }
  .ghost-action:disabled { opacity: .55; cursor: not-allowed; }
  @media (max-width: 900px) {
    .invite-grid { grid-template-columns: minmax(0, 1fr); }
    .user-list-header { display: none; }
    .user-list article { padding: var(--space-4) var(--space-5); grid-template-columns: minmax(0, 1fr) auto; row-gap: var(--space-3); }
    .user-actions { grid-column: 1 / -1; justify-content: flex-start; }
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

  .platform-release.update-available {
    border-color: color-mix(in srgb, var(--color-warning) 42%, var(--color-rule));
    box-shadow: inset 3px 0 var(--color-warning), var(--shadow-panel);
  }
  .release-body {
    padding: var(--space-5);
    display: grid;
    gap: var(--space-4);
  }
  .release-track {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(76px, 0.55fr) minmax(0, 1fr);
    align-items: center;
    gap: var(--space-3);
  }
  .release-track > div {
    min-height: 88px;
    padding: var(--space-3) var(--space-4);
    display: grid;
    align-content: center;
    gap: 3px;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-surface-subtle);
  }
  .release-track > div:last-child {
    background: var(--color-accent-softer);
  }
  .release-track span {
    color: var(--color-faint);
    font: 600 var(--text-2xs) var(--font-mono);
    letter-spacing: 0.08em;
  }
  .release-track strong {
    color: var(--color-ink);
    font: 600 var(--text-xl) var(--font-mono);
  }
  .release-track small {
    color: var(--color-muted);
    font: 500 var(--text-xs) var(--font-mono);
  }
  .release-line {
    display: flex;
    align-items: center;
    color: var(--color-faint);
  }
  .release-line i {
    height: 1px;
    flex: 1;
    background: var(--color-rule-strong);
  }
  .release-line.active {
    color: var(--color-warning);
  }
  .release-line.active i {
    background: var(--color-warning);
  }
  .update-job {
    padding: var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-surface-subtle);
  }
  .job-state {
    width: 9px;
    height: 9px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: var(--color-success);
  }
  .job-state.running {
    background: var(--color-warning);
    box-shadow: 0 0 0 4px var(--color-warning-soft);
    animation: update-pulse 1.6s ease-in-out infinite;
  }
  .update-job b {
    font-size: var(--text-sm);
    text-transform: capitalize;
  }
  .update-job p {
    margin: 2px 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .release-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .three-columns {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-3);
  }
  .update-policy-fields {
    padding-top: var(--space-1);
  }
  @keyframes update-pulse {
    50% { box-shadow: 0 0 0 7px color-mix(in srgb, var(--color-warning) 8%, transparent); }
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
    .three-columns,
    .smtp-grid,
    .smtp-section,
    .notification-toggles,
    .theme-options {
      grid-template-columns: 1fr;
    }
    .release-track {
      grid-template-columns: 1fr;
    }
    .release-line {
      min-height: 24px;
      flex-direction: column;
      transform: rotate(90deg);
    }
    .release-line i {
      width: 24px;
      flex: 0 0 1px;
    }
    .panel-footer,
    .release-actions {
      align-items: stretch;
      flex-direction: column;
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
