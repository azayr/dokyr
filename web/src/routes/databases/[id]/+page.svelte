<script>
  import { onDestroy, onMount } from 'svelte';
  import { page } from '$app/state';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import Status from '$lib/components/Status.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  const engines = {
    postgres: { label: 'PostgreSQL', version: '17', mark: 'PG' },
    mysql: { label: 'MySQL', version: '8.4', mark: 'MY' },
    mariadb: { label: 'MariaDB', version: '11.8', mark: 'MA' }
  };

  let cluster = null;
  let loading = true;
  let loadError = '';
  let activeTab = 'databases';
  let databaseModal = false;
  let databaseSaving = false;
  let databaseError = '';
  let databaseForm = { name: '', ownerUserId: '' };
  let userModal = false;
  let userSaving = false;
  let userError = '';
  let userForm = { username: '', password: '' };
  let grantModal = false;
  let grantSaving = false;
  let grantError = '';
  let grantForm = { databaseId: '', userId: '' };
  let deleteTarget = null;
  let deleteBusy = false;
  let deleteError = '';
  let credentialsTarget = null;
  let credentials = null;
  let credentialsLoading = false;
  let copied = '';
  let statusPollTimer;

  $: engine = engines[cluster?.engine] || { label: cluster?.engine || 'Database', version: '', mark: 'DB' };
  $: availableGrants = cluster
    ? cluster.databases.flatMap((database) => cluster.users
        .filter((user) => !cluster.grants.some((grant) => grant.databaseId === database.id && grant.userId === user.id))
        .map((user) => ({ database, user })))
    : [];

  onMount(load);
  onDestroy(() => clearTimeout(statusPollTimer));

  async function load(silent = false) {
    if (!silent) {
      loading = true;
      loadError = '';
    }
    try {
      const response = await api('/api/databases/' + page.params.id);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load database cluster');
      cluster = {
        ...payload.cluster,
        databases: payload.cluster?.databases || [],
        users: payload.cluster?.users || [],
        grants: payload.cluster?.grants || [],
        projects: payload.cluster?.projects || []
      };
    } catch (cause) {
      if (!silent) loadError = cause instanceof Error ? cause.message : 'Could not load database cluster';
    } finally {
      if (!silent) loading = false;
      clearTimeout(statusPollTimer);
      if (cluster?.status === 'deploying' || cluster?.status === 'created') {
        statusPollTimer = setTimeout(() => load(true), 2500);
      }
    }
  }

  const userById = (id) => cluster?.users.find((user) => user.id === id);
  const databaseById = (id) => cluster?.databases.find((database) => database.id === id);
  const grantsForDatabase = (id) => cluster?.grants.filter((grant) => grant.databaseId === id) || [];
  const grantsForUser = (id) => cluster?.grants.filter((grant) => grant.userId === id) || [];
  const projectsForDatabase = (id) => cluster?.projects.filter((project) => project.databaseId === id) || [];
  const projectsForGrant = (databaseId, userId) => cluster?.projects.filter((project) => project.databaseId === databaseId && project.userId === userId) || [];

  function openDatabaseModal() {
    databaseError = '';
    databaseForm = { name: '', ownerUserId: cluster?.users[0]?.id || '' };
    databaseModal = true;
  }

  async function createDatabase() {
    databaseSaving = true;
    databaseError = '';
    try {
      const response = await api(`/api/databases/${cluster.id}/logical-databases`, { method: 'POST', body: JSON.stringify(databaseForm) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not create database');
      cluster = payload.cluster;
      databaseModal = false;
      toast.success(payload.message || `${databaseForm.name} is ready`);
    } catch (cause) {
      databaseError = cause instanceof Error ? cause.message : 'Could not create database';
    } finally {
      databaseSaving = false;
    }
  }

  function openUserModal() {
    userError = '';
    userForm = { username: '', password: '' };
    userModal = true;
  }

  async function createUser() {
    userSaving = true;
    userError = '';
    try {
      const response = await api(`/api/databases/${cluster.id}/users`, { method: 'POST', body: JSON.stringify(userForm) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not create database user');
      userModal = false;
      await load();
      credentialsTarget = payload.user;
      credentials = payload.credentials;
      toast.success(payload.message || `${userForm.username} was created`);
    } catch (cause) {
      userError = cause instanceof Error ? cause.message : 'Could not create database user';
    } finally {
      userSaving = false;
    }
  }

  function openGrantModal(databaseId = '') {
    const preferred = availableGrants.find((pair) => !databaseId || pair.database.id === databaseId) || availableGrants[0];
    grantForm = { databaseId: preferred?.database.id || '', userId: preferred?.user.id || '' };
    grantError = '';
    grantModal = true;
  }

  function chooseGrantDatabase(databaseId) {
    const candidate = availableGrants.find((pair) => pair.database.id === databaseId);
    grantForm = { databaseId, userId: candidate?.user.id || '' };
  }

  function grantableUsers(databaseId) {
    return cluster?.users.filter((user) => !cluster.grants.some((grant) => grant.databaseId === databaseId && grant.userId === user.id)) || [];
  }

  async function createGrant() {
    grantSaving = true;
    grantError = '';
    try {
      const response = await api(`/api/databases/${cluster.id}/grants`, { method: 'POST', body: JSON.stringify(grantForm) });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not grant database access');
      grantModal = false;
      toast.success(payload.message || 'Database access granted');
      await load();
    } catch (cause) {
      grantError = cause instanceof Error ? cause.message : 'Could not grant database access';
    } finally {
      grantSaving = false;
    }
  }

  async function revokeGrant(databaseId, userId) {
    try {
      const response = await api(`/api/databases/${cluster.id}/grants/${databaseId}/${userId}`, { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not revoke database access');
      toast.success(payload.message || 'Database access revoked');
      await load();
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not revoke database access');
    }
  }

  function confirmDelete(kind, item) {
    deleteTarget = { kind, item };
    deleteError = '';
  }

  async function deleteResource() {
    deleteBusy = true;
    deleteError = '';
    try {
      const endpoint = deleteTarget.kind === 'database'
        ? `/api/databases/${cluster.id}/logical-databases/${deleteTarget.item.id}`
        : `/api/databases/${cluster.id}/users/${deleteTarget.item.id}`;
      const response = await api(endpoint, { method: 'DELETE' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not delete resource');
      deleteTarget = null;
      toast.success(payload.message || 'Resource deleted');
      await load();
    } catch (cause) {
      deleteError = cause instanceof Error ? cause.message : 'Could not delete resource';
    } finally {
      deleteBusy = false;
    }
  }

  async function revealCredentials(user) {
    credentialsTarget = user;
    credentials = null;
    credentialsLoading = true;
    copied = '';
    try {
      const response = await api(`/api/databases/${cluster.id}/users/${user.id}/credentials`);
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not reveal credentials');
      credentials = payload;
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not reveal credentials');
      credentialsTarget = null;
    } finally {
      credentialsLoading = false;
    }
  }

  async function copy(field, value) {
    await navigator.clipboard.writeText(value);
    copied = field;
    setTimeout(() => { if (copied === field) copied = ''; }, 1400);
  }
</script>

<svelte:window onkeydown={(event) => {
  if (event.key !== 'Escape') return;
  if (databaseModal && !databaseSaving) databaseModal = false;
  if (userModal && !userSaving) userModal = false;
  if (grantModal && !grantSaving) grantModal = false;
  if (deleteTarget && !deleteBusy) deleteTarget = null;
  if (credentialsTarget) credentialsTarget = null;
}} />

<Shell eyebrow="Database cluster" title={cluster?.name || 'Cluster'} subtitle="Logical databases, users, grants, and private project connections.">
  <a slot="actions" class="btn" href="/databases"><Icon name="arrow-left" size={14} /> All clusters</a>

  {#if loading}
    <div class="detail-loading"><span></span><strong>Reading cluster configuration…</strong></div>
  {:else if loadError}
    <div class="detail-error"><Icon name="x-circle" size={19} /><div><strong>Cluster unavailable</strong><p>{loadError}</p></div><button class="btn" onclick={load}>Retry</button></div>
  {:else if cluster}
    <section class="cluster-hero">
      <div class="engine-mark" class:postgres={cluster.engine === 'postgres'} class:mysql={cluster.engine === 'mysql'}>{engine.mark}</div>
      <div class="cluster-heading"><span>{engine.label} {engine.version}</span><h2>{cluster.name}</h2><p>{cluster.image}</p></div>
      <Status value={cluster.status} />
      <dl>
        <div><dt>Databases</dt><dd>{cluster.databases.length}</dd></div>
        <div><dt>Users</dt><dd>{cluster.users.length}</dd></div>
        <div><dt>Grants</dt><dd>{cluster.grants.length}</dd></div>
        <div><dt>Projects</dt><dd>{cluster.projectCount}</dd></div>
      </dl>
      <div class="cluster-address"><Icon name={cluster.publicEnabled ? 'globe' : 'lock'} size={16} /><div><span>{cluster.publicEnabled ? 'Public access enabled' : 'Private network only'}</span><code>{cluster.container || `selfhost-db-${cluster.id}`}:{cluster.internalPort}</code></div></div>
    </section>

    {#if cluster.status === 'deploying'}
      <div class="provision-note"><span class="provision-spinner"></span><div><strong>Provisioning in the background</strong><p>You can leave this page. The cluster status updates automatically while its image downloads and the container starts.</p></div></div>
    {:else if cluster.status === 'failed'}
      <div class="provision-note failed"><Icon name="x-circle" size={18} /><div><strong>Provisioning failed</strong><p>{cluster.lastError || 'Open deployment logs from the cluster list for details, then retry the deployment.'}</p></div></div>
    {/if}

    <nav class="workspace-tabs" aria-label="Cluster workspace">
      <button class:active={activeTab === 'databases'} onclick={() => activeTab = 'databases'}><Icon name="database" size={14} /> Databases <span>{cluster.databases.length}</span></button>
      <button class:active={activeTab === 'access'} onclick={() => activeTab = 'access'}><Icon name="key" size={14} /> Users & access <span>{cluster.users.length}</span></button>
      <button class:active={activeTab === 'projects'} onclick={() => activeTab = 'projects'}><Icon name="network" size={14} /> Projects <span>{cluster.projects.length}</span></button>
    </nav>

    {#if activeTab === 'databases'}
      <section class="workspace-panel panel">
        <header><div><span>Logical data stores</span><h3>Databases</h3><p>Each database has one owner and can grant access to additional cluster users.</p></div>{#if cluster.users.length}<button class="btn btn-primary" onclick={openDatabaseModal}><Icon name="plus" size={14} /> Create database</button>{:else}<button class="btn btn-primary" onclick={openUserModal}><Icon name="user-plus" size={14} /> Create first user</button>{/if}</header>
        {#if cluster.databases.length === 0}
          <div class="empty-resources"><span><Icon name="database" size={24} /></span><div><strong>No databases in the catalog</strong><p>{cluster.users.length ? 'Create a logical database and choose its default user.' : 'Create a cluster user first, then add the initial logical database.'}</p></div><button class="btn btn-primary" onclick={cluster.users.length ? openDatabaseModal : openUserModal}>{cluster.users.length ? 'Create database' : 'Create first user'}</button></div>
        {:else}
          <div class="table-head database-grid" aria-hidden="true"><span>Database</span><span>Owner</span><span>Users with access</span><span>Project usage</span><span></span></div>
          <div class="resource-list">
            {#each cluster.databases as database}
              <article class="resource-row database-grid">
                <div class="resource-name"><span><Icon name="database" size={16} /></span><div><strong>{database.name}</strong><small>{database.primary ? 'Primary cluster database' : `Created ${new Date(database.createdAt).toLocaleDateString()}`}</small></div>{#if database.primary}<b>PRIMARY</b>{/if}</div>
                <div class="owner-cell"><Icon name="user" size={14} /><code>{database.username}</code></div>
                <div class="access-pills">{#each grantsForDatabase(database.id) as grant}<span title={userById(grant.userId)?.username}>{userById(grant.userId)?.username}</span>{/each}<button onclick={() => openGrantModal(database.id)} disabled={grantableUsers(database.id).length === 0} aria-label={`Grant a user access to ${database.name}`}>＋</button></div>
                <div class="usage-cell">{#if projectsForDatabase(database.id).length}{#each projectsForDatabase(database.id).slice(0, 2) as project}<a href={`/projects/${project.id}#databases`}><i></i>{project.name}</a>{/each}{#if projectsForDatabase(database.id).length > 2}<small>+{projectsForDatabase(database.id).length - 2} more</small>{/if}{:else}<span>Not used by a project</span>{/if}</div>
                <button class="delete-action" onclick={() => confirmDelete('database', database)} disabled={database.primary || database.projectCount > 0} title={database.primary ? 'The primary database is protected' : database.projectCount > 0 ? 'Detach it from every project first' : 'Delete database'}><Icon name="trash" size={14} /> Delete</button>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    {:else if activeTab === 'access'}
      <div class="access-layout">
        <section class="workspace-panel panel users-panel">
          <header><div><span>Cluster identities</span><h3>Database users</h3><p>Credentials belong to the cluster and can be reused across granted databases.</p></div><button class="btn btn-primary" onclick={openUserModal}><Icon name="user-plus" size={14} /> Create user</button></header>
          {#if cluster.users.length === 0}<div class="empty-resources compact"><span><Icon name="user-plus" size={22} /></span><div><strong>No database users</strong><p>Create the first login before provisioning a logical database.</p></div><button class="btn btn-primary" onclick={openUserModal}>Create user</button></div>{:else}<div class="user-list">
            {#each cluster.users as user}
              <article class="user-row">
                <span class="user-avatar"><Icon name="user" size={16} /></span>
                <div><strong>{user.username}</strong><small>{user.admin ? 'Cluster administrator' : `${user.databaseCount} database grant${user.databaseCount === 1 ? '' : 's'}`}</small></div>
                {#if user.admin}<b>ADMIN</b>{/if}
                <div class="user-databases">{#each grantsForUser(user.id) as grant}<span>{databaseById(grant.databaseId)?.name}</span>{/each}</div>
                <div class="user-actions"><button onclick={() => revealCredentials(user)}><Icon name="eye" size={13} /> Credentials</button><button class="danger" onclick={() => confirmDelete('user', user)} disabled={user.admin || user.projectCount > 0 || user.databaseCount > 0} title={user.admin ? 'The cluster administrator is protected' : user.projectCount > 0 || user.databaseCount > 0 ? 'Remove project usage and database grants first' : 'Delete user'}><Icon name="trash" size={13} /></button></div>
              </article>
            {/each}
          </div>{/if}
        </section>

        <section class="workspace-panel panel grants-panel">
          <header><div><span>Access map</span><h3>Database grants</h3><p>Grant or revoke full application access per database.</p></div><button class="btn" onclick={() => openGrantModal()} disabled={availableGrants.length === 0}><Icon name="link" size={14} /> Grant access</button></header>
          {#if cluster.grants.length === 0}<div class="empty-resources compact"><span><Icon name="link" size={22} /></span><div><strong>No access grants</strong><p>Grants appear after a database and user are available.</p></div></div>{:else}<div class="grant-list">
            {#each cluster.grants as grant}
              {@const database = databaseById(grant.databaseId)}
              {@const user = userById(grant.userId)}
              {@const consumers = projectsForGrant(grant.databaseId, grant.userId)}
              <article><span class="grant-user"><Icon name="user" size={13} /><code>{user?.username}</code></span><span class="grant-arrow">→</span><span class="grant-database"><Icon name="database" size={13} /><code>{database?.name}</code></span><span class="grant-use">{consumers.length ? `${consumers.length} project${consumers.length === 1 ? '' : 's'}` : 'Unused'}</span><button onclick={() => revokeGrant(grant.databaseId, grant.userId)} disabled={database?.ownerUserId === grant.userId || consumers.length > 0} title={database?.ownerUserId === grant.userId ? 'Owner access cannot be revoked' : consumers.length ? 'A project uses this grant' : 'Revoke access'}>Revoke</button></article>
            {/each}
          </div>{/if}
        </section>
      </div>
    {:else}
      <section class="workspace-panel panel projects-panel">
        <header><div><span>Private network consumers</span><h3>Attached projects</h3><p>Every attachment selects one database, one granted user, and a project-local DNS alias.</p></div><a class="btn" href="/projects"><Icon name="grid" size={14} /> Browse projects</a></header>
        {#if cluster.projects.length === 0}
          <div class="empty-projects"><Icon name="network" size={24} /><div><strong>No project connections</strong><p>Open a project and choose Add database to connect this cluster to its private network.</p></div></div>
        {:else}
          <div class="project-list">
            {#each cluster.projects as project}
              <a href={`/projects/${project.id}#databases`}><span><Icon name="grid" size={15} /></span><div><strong>{project.name}</strong><small>Private network attachment</small></div><dl><div><dt>Database</dt><dd>{project.databaseName}</dd></div><div><dt>User</dt><dd>{project.username}</dd></div><div><dt>Internal DNS</dt><dd><code>{project.alias}:{cluster.internalPort}</code></dd></div></dl><Icon name="arrow-right" size={15} /></a>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  {/if}
</Shell>

{#if databaseModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !databaseSaving) databaseModal = false; }}>
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="create-database-title">
      <header><div><span>Logical data store</span><h2 id="create-database-title">Create database</h2></div><button aria-label="Close" onclick={() => databaseModal = false} disabled={databaseSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createDatabase(); }}>
        <p class="modal-intro">The database is created inside <strong>{cluster.name}</strong>. Its owner receives access automatically.</p>
        {#if databaseError}<div class="modal-error"><Icon name="x-circle" size={14} /><span>{databaseError}</span></div>{/if}
        <label class="field"><span>Database name</span><input class="input input-mono" bind:value={databaseForm.name} required maxlength="63" pattern="[A-Za-z0-9_]+" placeholder="customer_portal" autocomplete="off" /></label>
        <label class="field"><span>Owner</span><select class="input" bind:value={databaseForm.ownerUserId} required>{#each cluster.users as user}<option value={user.id}>{user.username}{user.admin ? ' · administrator' : ''}</option>{/each}</select><small>The owner grant cannot be revoked while this database exists.</small></label>
        <footer><button class="btn" type="button" onclick={() => databaseModal = false} disabled={databaseSaving}>Cancel</button><button class="btn btn-primary" type="submit" disabled={databaseSaving}>{databaseSaving ? 'Creating…' : 'Create database'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if userModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !userSaving) userModal = false; }}>
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="create-user-title">
      <header><div><span>Cluster identity</span><h2 id="create-user-title">Create database user</h2></div><button aria-label="Close" onclick={() => userModal = false} disabled={userSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createUser(); }}>
        <p class="modal-intro">New users start without database access. Add grants after the user is created.</p>
        {#if userError}<div class="modal-error"><Icon name="x-circle" size={14} /><span>{userError}</span></div>{/if}
        <label class="field"><span>Username</span><input class="input input-mono" bind:value={userForm.username} required maxlength="63" pattern="[A-Za-z0-9_]+" placeholder="reporting_app" autocomplete="off" /></label>
        <label class="field"><span>Password <em>optional</em></span><input class="input input-mono" bind:value={userForm.password} type="password" minlength="12" maxlength="256" placeholder="Securely generated if empty" autocomplete="new-password" /><small>The generated or supplied password is encrypted before it is stored.</small></label>
        <footer><button class="btn" type="button" onclick={() => userModal = false} disabled={userSaving}>Cancel</button><button class="btn btn-primary" type="submit" disabled={userSaving}>{userSaving ? 'Creating…' : 'Create user'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if grantModal}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !grantSaving) grantModal = false; }}>
    <div class="modal grant-modal" role="dialog" aria-modal="true" aria-labelledby="grant-title">
      <header><div><span>Access policy</span><h2 id="grant-title">Grant database access</h2></div><button aria-label="Close" onclick={() => grantModal = false} disabled={grantSaving}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); createGrant(); }}>
        <div class="grant-diagram"><span><Icon name="user" size={18} /></span><i>→</i><span><Icon name="database" size={18} /></span></div>
        {#if grantError}<div class="modal-error"><Icon name="x-circle" size={14} /><span>{grantError}</span></div>{/if}
        {#if availableGrants.length === 0}<p class="modal-intro">Every cluster user already has access to every database.</p>{:else}
          <label class="field"><span>Database</span><select class="input" value={grantForm.databaseId} onchange={(event) => chooseGrantDatabase(event.currentTarget.value)} required>{#each cluster.databases.filter((database) => grantableUsers(database.id).length > 0) as database}<option value={database.id}>{database.name}</option>{/each}</select></label>
          <label class="field"><span>User</span><select class="input" bind:value={grantForm.userId} required>{#each grantableUsers(grantForm.databaseId) as user}<option value={user.id}>{user.username}</option>{/each}</select><small>The user receives full application privileges for this database.</small></label>
        {/if}
        <footer><button class="btn" type="button" onclick={() => grantModal = false} disabled={grantSaving}>Cancel</button><button class="btn btn-primary" type="submit" disabled={grantSaving || !grantForm.databaseId || !grantForm.userId}>{grantSaving ? 'Granting…' : 'Grant access'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if deleteTarget}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !deleteBusy) deleteTarget = null; }}>
    <div class="modal delete-modal" role="dialog" aria-modal="true" aria-labelledby="delete-resource-title">
      <header><div><span>Permanent operation</span><h2 id="delete-resource-title">Delete {deleteTarget.kind === 'database' ? 'database' : 'user'}?</h2></div><button aria-label="Close" onclick={() => deleteTarget = null} disabled={deleteBusy}>×</button></header>
      <form onsubmit={(event) => { event.preventDefault(); deleteResource(); }}>
        <div class="danger-note"><Icon name="alert" size={19} /><div><strong>{deleteTarget.item.name || deleteTarget.item.username}</strong><p>{deleteTarget.kind === 'database' ? 'The logical database and all of its data will be permanently removed from the cluster.' : 'This login will be permanently removed from the database engine.'}</p></div></div>
        {#if deleteError}<div class="modal-error"><Icon name="x-circle" size={14} /><span>{deleteError}</span></div>{/if}
        <footer><button class="btn" type="button" onclick={() => deleteTarget = null} disabled={deleteBusy}>Cancel</button><button class="btn btn-danger-solid" type="submit" disabled={deleteBusy}>{deleteBusy ? 'Deleting…' : 'Delete permanently'}</button></footer>
      </form>
    </div>
  </div>
{/if}

{#if credentialsTarget}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) credentialsTarget = null; }}>
    <div class="modal credentials-modal" role="dialog" aria-modal="true" aria-labelledby="user-credentials-title">
      <header><div><span>Database secret</span><h2 id="user-credentials-title">{credentialsTarget.username} credentials</h2></div><button aria-label="Close" onclick={() => credentialsTarget = null}>×</button></header>
      {#if credentialsLoading}<div class="credential-loading"><span></span>Decrypting credentials…</div>{:else if credentials}
        <div class="secret-warning"><Icon name="shield" size={17} /><p>Use these credentials only with databases listed in this user’s access grants.</p></div>
        <div class="credential-rows"><div><span>Username</span><code>{credentials.username}</code><button onclick={() => copy('username', credentials.username)}>{copied === 'username' ? 'Copied' : 'Copy'}</button></div><div><span>Password</span><code>{credentials.password}</code><button onclick={() => copy('password', credentials.password)}>{copied === 'password' ? 'Copied' : 'Copy'}</button></div></div>
        <div class="credential-access"><span>Granted databases</span><div>{#each grantsForUser(credentialsTarget.id) as grant}<code>{databaseById(grant.databaseId)?.name}</code>{/each}{#if grantsForUser(credentialsTarget.id).length === 0}<small>No database access yet</small>{/if}</div></div>
      {/if}
      <footer><button class="btn btn-primary" onclick={() => credentialsTarget = null}>Done</button></footer>
    </div>
  </div>
{/if}

<style>
  .detail-loading { min-height: 420px; display: grid; place-content: center; justify-items: center; gap: var(--space-3); color: var(--color-muted); }
  .detail-loading span, .credential-loading > span { width: 20px; height: 20px; border: 2px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .detail-error { min-height: 240px; padding: var(--space-8); display: flex; align-items: center; justify-content: center; gap: var(--space-4); border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); border-radius: var(--radius-lg); background: var(--color-danger-soft); color: var(--color-danger); }
  .detail-error p { margin: 3px 0 0; color: var(--color-muted); }
  .cluster-hero { margin-bottom: var(--space-4); padding: var(--space-5); display: grid; grid-template-columns: 58px minmax(220px, 1fr) auto; align-items: center; gap: var(--space-4); border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: linear-gradient(120deg, var(--color-paper-raised) 0 64%, var(--color-accent-softer)); box-shadow: var(--shadow-panel); }
  .engine-mark { width: 58px; height: 58px; display: grid; place-items: center; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-paper-subtle); font: 800 var(--text-sm)/1 var(--font-mono); }
  .engine-mark.postgres { border-color: color-mix(in srgb, #336791 42%, var(--color-rule)); background: color-mix(in srgb, #336791 11%, var(--color-paper-raised)); color: #336791; }
  .engine-mark.mysql { border-color: color-mix(in srgb, #e48e00 42%, var(--color-rule)); background: color-mix(in srgb, #e48e00 11%, var(--color-paper-raised)); color: #b56f00; }
  .cluster-heading span, .workspace-panel > header span { color: var(--color-accent); font: 700 var(--text-2xs)/1 var(--font-mono); letter-spacing: .1em; text-transform: uppercase; }
  .cluster-heading h2 { margin: 4px 0 2px; font: 600 var(--text-2xl)/1.1 var(--font-display); letter-spacing: -.04em; }
  .cluster-heading p { margin: 0; color: var(--color-muted); font: var(--text-xs) var(--font-mono); }
  .cluster-hero dl { grid-column: 2 / 3; margin: var(--space-2) 0 0; display: flex; gap: var(--space-5); }
  .cluster-hero dl div { display: grid; } .cluster-hero dt { color: var(--color-muted); font: 600 var(--text-2xs) var(--font-mono); text-transform: uppercase; } .cluster-hero dd { margin: 0; font: 600 var(--text-lg) var(--font-display); }
  .cluster-address { grid-column: 3; grid-row: 2; display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); }
  .cluster-address div { display: grid; } .cluster-address span { font-size: var(--text-xs); } .cluster-address code { color: var(--color-ink-secondary); font-size: var(--text-xs); }
  .provision-note { margin: calc(var(--space-2) * -1) 0 var(--space-4); padding: var(--space-3) var(--space-4); display: flex; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .provision-note div { display: grid; } .provision-note strong { font-size: var(--text-sm); } .provision-note p { margin: 2px 0 0; color: var(--color-muted); font-size: var(--text-xs); }
  .provision-note.failed { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-rule)); background: var(--color-danger-soft); color: var(--color-danger); }
  .provision-spinner { width: 16px; height: 16px; flex: 0 0 auto; border: 2px solid var(--color-rule-strong); border-top-color: var(--color-accent); border-radius: 50%; animation: spin .7s linear infinite; }
  .workspace-tabs { margin-bottom: var(--space-4); padding: 4px; display: flex; gap: 3px; overflow-x: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .workspace-tabs button { min-height: 38px; padding: 0 var(--space-4); display: inline-flex; align-items: center; gap: var(--space-2); border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: var(--text-sm); font-weight: 600; white-space: nowrap; cursor: pointer; }
  .workspace-tabs button span { min-width: 21px; height: 21px; padding: 0 5px; display: grid; place-items: center; border-radius: 999px; background: var(--color-paper-subtle); font: var(--text-2xs) var(--font-mono); }
  .workspace-tabs button.active { background: var(--color-paper-raised); color: var(--color-ink); box-shadow: var(--shadow-xs); }
  .workspace-tabs button.active :global(svg) { color: var(--color-accent); }
  .workspace-panel > header { min-height: 76px; padding: var(--space-4) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .workspace-panel > header h3 { margin: 4px 0 1px; font: 600 var(--text-lg) var(--font-display); } .workspace-panel > header p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); }
  .table-head { min-height: 34px; padding: 0 var(--space-5); align-items: center; border-bottom: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font: 600 var(--text-2xs) var(--font-mono); letter-spacing: .07em; text-transform: uppercase; }
  .empty-resources { min-height: 260px; padding: var(--space-8); display: grid; grid-template-columns: 48px minmax(0, 480px) auto; align-items: center; justify-content: center; gap: var(--space-4); }
  .empty-resources.compact { min-height: 190px; padding: var(--space-6); grid-template-columns: 42px minmax(0, 320px) auto; }
  .empty-resources > span { width: 48px; height: 48px; display: grid; place-items: center; border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); color: var(--color-muted); }
  .empty-resources.compact > span { width: 42px; height: 42px; }
  .empty-resources strong { font-size: var(--text-sm); }
  .empty-resources p { margin: 3px 0 0; color: var(--color-muted); font-size: var(--text-xs); }
  .database-grid { display: grid; grid-template-columns: minmax(210px, 1.1fr) minmax(120px, .55fr) minmax(180px, .9fr) minmax(180px, .8fr) 82px; gap: var(--space-4); }
  .resource-row { min-height: 82px; padding: var(--space-3) var(--space-5); align-items: center; border-bottom: 1px solid var(--color-rule); }
  .resource-row:last-child { border-bottom: 0; } .resource-row:hover { background: var(--color-surface-subtle); }
  .resource-name { min-width: 0; display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); }
  .resource-name > span, .user-avatar { width: 34px; height: 34px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); }
  .resource-name div { min-width: 0; display: grid; } .resource-name strong { overflow: hidden; text-overflow: ellipsis; } .resource-name small { color: var(--color-muted); font-size: var(--text-xs); }
  .resource-name b, .user-row > b { padding: 3px 6px; border-radius: var(--radius-xs); background: var(--color-info-soft); color: var(--color-info); font: 700 var(--text-2xs) var(--font-mono); letter-spacing: .05em; }
  .owner-cell { display: flex; align-items: center; gap: var(--space-2); color: var(--color-muted); } .owner-cell code { color: var(--color-ink-secondary); }
  .access-pills, .usage-cell { display: flex; flex-wrap: wrap; gap: 5px; }
  .access-pills span, .user-databases span { padding: 4px 7px; border: 1px solid var(--color-rule); border-radius: 999px; background: var(--color-paper-raised); color: var(--color-ink-secondary); font-size: var(--text-xs); }
  .access-pills button { width: 25px; height: 25px; border: 1px dashed var(--color-rule-strong); border-radius: 50%; background: transparent; color: var(--color-accent); cursor: pointer; } .access-pills button:disabled { opacity: .35; cursor: not-allowed; }
  .usage-cell a { padding: 4px 7px; display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: 999px; color: var(--color-ink-secondary); font-size: var(--text-xs); text-decoration: none; }
  .usage-cell i { width: 5px; height: 5px; border-radius: 50%; background: var(--color-success); } .usage-cell > span, .usage-cell small { align-self: center; color: var(--color-muted); font-size: var(--text-xs); }
  .delete-action { min-height: 30px; display: inline-flex; align-items: center; justify-content: center; gap: 6px; border: 1px solid color-mix(in srgb, var(--color-danger) 28%, var(--color-rule)); border-radius: var(--radius-sm); background: transparent; color: var(--color-danger); font-size: var(--text-xs); cursor: pointer; }
  .delete-action:disabled { border-color: var(--color-rule); color: var(--color-faint); cursor: not-allowed; }
  .access-layout { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(360px, .85fr); gap: var(--space-4); align-items: start; }
  .user-row { min-height: 76px; padding: var(--space-3) var(--space-4); display: grid; grid-template-columns: 34px minmax(120px, .7fr) auto minmax(150px, 1fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .user-row:last-child { border: 0; } .user-row > div:nth-child(2) { display: grid; } .user-row small { color: var(--color-muted); font-size: var(--text-xs); }
  .user-databases { display: flex; flex-wrap: wrap; gap: 4px; }
  .user-actions { display: flex; gap: 5px; } .user-actions button, .grant-list button { min-height: 29px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink-secondary); font-size: var(--text-xs); cursor: pointer; }
  .user-actions button.danger { color: var(--color-danger); } .user-actions button:disabled, .grant-list button:disabled { opacity: .35; cursor: not-allowed; }
  .grant-list { padding: var(--space-2); }
  .grant-list article { min-height: 52px; padding: var(--space-2); display: grid; grid-template-columns: minmax(90px, 1fr) 16px minmax(90px, 1fr) auto auto; align-items: center; gap: var(--space-2); border-bottom: 1px solid var(--color-rule); }
  .grant-list article:last-child { border: 0; } .grant-user, .grant-database { min-width: 0; display: flex; align-items: center; gap: 6px; } .grant-user code, .grant-database code { overflow: hidden; text-overflow: ellipsis; }
  .grant-arrow, .grant-use { color: var(--color-muted); font-size: var(--text-xs); } .grant-list button { color: var(--color-danger); }
  .empty-projects { min-height: 260px; display: flex; align-items: center; justify-content: center; gap: var(--space-4); color: var(--color-muted); } .empty-projects p { margin: 3px 0 0; font-size: var(--text-sm); }
  .project-list > a { min-height: 76px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 36px minmax(150px, .7fr) minmax(360px, 1.3fr) auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; }
  .project-list > a:hover { background: var(--color-surface-subtle); } .project-list > a > span { width: 36px; height: 36px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-paper-subtle); color: var(--color-accent); }
  .project-list > a > div { display: grid; } .project-list small { color: var(--color-muted); font-size: var(--text-xs); }
  .project-list dl { margin: 0; display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-4); } .project-list dl div { display: grid; } .project-list dt { color: var(--color-muted); font-size: var(--text-2xs); text-transform: uppercase; } .project-list dd { margin: 2px 0 0; font-size: var(--text-xs); }
  .modal-backdrop { position: fixed; z-index: 500; inset: 0; padding: var(--space-4); display: grid; place-items: center; overflow-y: auto; background: rgb(4 10 18 / .58); backdrop-filter: blur(3px); }
  .modal { width: min(580px, 100%); overflow: hidden; border: 1px solid var(--color-rule-strong); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-modal); }
  .modal > header { min-height: 65px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .modal > header span { color: var(--color-accent); font: 700 var(--text-2xs) var(--font-mono); letter-spacing: .1em; text-transform: uppercase; } .modal > header h2 { margin: 3px 0 0; font: 600 var(--text-lg) var(--font-display); }
  .modal > header button { width: 30px; height: 30px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }
  .modal form { padding: var(--space-5); display: grid; gap: var(--space-4); } .modal-intro { margin: 0; color: var(--color-muted); font-size: var(--text-sm); }
  .modal .field { display: grid; gap: 6px; } .modal .field > span { color: var(--color-ink-secondary); font-size: var(--text-xs); font-weight: 600; } .modal .field em { color: var(--color-muted); font-style: normal; font-weight: 400; } .modal .field small { color: var(--color-muted); font-size: var(--text-xs); }
  .modal footer { margin: var(--space-1) calc(var(--space-5) * -1) calc(var(--space-5) * -1); padding: var(--space-3) var(--space-5); display: flex; justify-content: flex-end; gap: var(--space-2); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .modal-error { padding: var(--space-3); display: flex; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-danger-soft); color: var(--color-danger); font-size: var(--text-xs); }
  .grant-diagram { display: flex; align-items: center; justify-content: center; gap: var(--space-4); } .grant-diagram span { width: 48px; height: 48px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-accent); } .grant-diagram i { color: var(--color-muted); font-style: normal; }
  .danger-note { padding: var(--space-4); display: flex; gap: var(--space-3); border-left: 3px solid var(--color-danger); background: var(--color-danger-soft); color: var(--color-danger); } .danger-note p { margin: 3px 0 0; color: var(--color-muted); font-size: var(--text-xs); }
  .credentials-modal > footer { margin: 0; } .credential-loading { min-height: 210px; display: flex; align-items: center; justify-content: center; gap: var(--space-3); color: var(--color-muted); }
  .secret-warning { margin: var(--space-5); padding: var(--space-3); display: flex; gap: var(--space-3); border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); } .secret-warning p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); }
  .credential-rows { padding: 0 var(--space-5); } .credential-rows > div { min-height: 52px; display: grid; grid-template-columns: 90px minmax(0, 1fr) 50px; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); } .credential-rows span { color: var(--color-muted); font-size: var(--text-xs); } .credential-rows code { overflow: hidden; text-overflow: ellipsis; } .credential-rows button { border: 0; background: transparent; color: var(--color-accent); cursor: pointer; }
  .credential-access { padding: var(--space-4) var(--space-5) var(--space-5); display: grid; gap: var(--space-2); } .credential-access > span { color: var(--color-muted); font-size: var(--text-xs); } .credential-access div { display: flex; flex-wrap: wrap; gap: 5px; } .credential-access code { padding: 4px 7px; border: 1px solid var(--color-rule); border-radius: var(--radius-xs); } .credential-access small { color: var(--color-muted); }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (max-width: 72rem) { .access-layout { grid-template-columns: 1fr; } .database-grid { grid-template-columns: minmax(200px, 1fr) minmax(120px, .6fr) minmax(180px, .9fr) 82px; } .database-grid > :nth-child(4) { display: none; } }
  @media (max-width: 52rem) { .cluster-hero { grid-template-columns: 48px minmax(0, 1fr) auto; } .engine-mark { width: 48px; height: 48px; } .cluster-hero dl { grid-column: 1 / -1; } .cluster-address { grid-column: 1 / -1; grid-row: auto; } .workspace-panel > header { align-items: flex-start; flex-direction: column; } .table-head { display: none; } .empty-resources, .empty-resources.compact { grid-template-columns: 44px minmax(0, 1fr); } .empty-resources button { grid-column: 1 / -1; } .resource-row.database-grid { grid-template-columns: 1fr auto; } .resource-row > :not(.resource-name, .delete-action) { grid-column: 1 / -1; } .resource-row > :nth-child(4) { display: flex; } .user-row { grid-template-columns: 34px minmax(0, 1fr) auto; } .user-databases, .user-actions { grid-column: 2 / -1; } .project-list > a { grid-template-columns: 36px minmax(0, 1fr) auto; } .project-list dl { grid-column: 2 / -1; grid-row: 2; } }
  @media (max-width: 34rem) { .cluster-hero dl { display: grid; grid-template-columns: 1fr 1fr; } .workspace-tabs button { padding: 0 var(--space-3); } .grant-list article { grid-template-columns: minmax(0, 1fr) 16px minmax(0, 1fr) auto; } .grant-use { display: none; } .project-list dl { grid-template-columns: 1fr; } .modal footer { flex-direction: column; } }
  @media (prefers-reduced-motion: reduce) { .detail-loading span, .credential-loading > span, .provision-spinner { animation: none; } }
</style>
