<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { api, currentUser } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let error = '';
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

  const roleSummaries = {
    owner: 'Full control, including domains, proxy configuration, platform updates, and users.',
    admin: 'Everything except ingress, platform settings, and user management.',
    developer: 'Create and deploy projects, read and change environment variables, run container commands.',
    viewer: 'Read-only access to projects, services, logs, and metrics. No secrets.'
  };

  onMount(loadUsers);

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

  function formatJoined(value) {
    if (!value) return '—';
    return new Date(value).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  }
</script>

<Shell eyebrow="Administration" title="Users" subtitle="Invite people, assign roles, and manage access to this control plane.">
  <div class="users-content">
    {#if error}
      <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><span>{error}</span></div><button class="alert-close" aria-label="Dismiss error" onclick={() => (error = '')}>×</button></div>
    {/if}

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
  .users-content {
    min-width: 0;
    display: grid;
    align-content: start;
    gap: var(--space-4);
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

  @media (max-width: 40rem) {
    .invite-grid { grid-template-columns: minmax(0, 1fr); }
    .user-list-header { display: none; }
    .user-list article { padding: var(--space-4) var(--space-5); grid-template-columns: minmax(0, 1fr) auto; row-gap: var(--space-3); }
    .user-actions { grid-column: 1 / -1; justify-content: flex-start; }
  }
</style>
