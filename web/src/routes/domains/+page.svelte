<script>
  import { onMount } from 'svelte';
  import Shell from '$lib/components/Shell.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { api } from '$lib/auth.js';
  import { toast } from '$lib/toast.js';

  let loading = true;
  let saving = false;
  let deleting = false;
  let configSaving = false;
  let resetting = false;
  let copied = false;
  let error = '';
  let notice = '';
  let connected = false;
  let connectionError = '';
  let projects = [];
  let managedDomains = [];
  let dnsTarget = { type: 'A', name: '@', value: '127.0.0.1' };
  let registryDomain = { domain: '', httpsEnabled: true, attached: false, registryHosts: [] };
  let controlHosts = [];
  let publicURL = '';
  let platform = { publicURL: '', domain: '', customDomainConfigured: false };
  let platformDomain = '';
  let platformSaving = false;
  let platformInput;
  let routes = [];
  let configuration = '';
  let managedConfiguration = '';
  let configDirty = false;
  let query = '';
  let modalOpen = false;
  let editingProjectId = '';
  let editingDomainIndex = -1;
  let draft = null;
  let verifyingId = '';
  let copiedDNS = '';

  $: projectDomains = projects.flatMap((project) =>
    project.domains.map((domain, index) => ({ ...domain, kind: 'project', index, projectId: project.id, projectName: project.name, services: project.services }))
  );
  $: catalogDomains = managedDomains.map((managed) => {
    const binding = projectDomains.find((item) => item.domain.toLowerCase() === managed.domain.toLowerCase());
    return {
      ...managed,
      ...binding,
      id: managed.id,
      kind: 'managed',
      domain: managed.domain,
      projectId: binding?.projectId || '',
      projectName: binding?.projectName || 'Available for a project',
      services: binding?.services || [],
      rules: binding?.rules || [],
      httpsEnabled: Boolean(binding?.httpsEnabled)
    };
  });
  $: registryDomains = (registryDomain.registryHosts || []).map((host) => ({
    kind: 'registry',
    domain: host,
    editableDomain: registryDomain.attached ? registryDomain.domain : hostnameOnly(host),
    projectId: 'registry',
    projectName: registryDomain.attached ? 'Container registry' : 'Container registry · environment fallback',
    httpsEnabled: registryDomain.attached ? registryDomain.httpsEnabled : false,
    attached: registryDomain.attached,
    services: [
      { id: 'registry-auth', name: 'Dokyr token service', containerPort: 8080 },
      { id: 'registry', name: 'Docker Registry', containerPort: 5000 }
    ],
    rules: [
      { path: '/api/registry/token', serviceId: 'registry-auth', port: 8080 },
      { path: '/*', serviceId: 'registry', port: 5000 }
    ]
  }));
  $: controlDomains = controlHosts.map((host) => ({
    kind: 'control',
    domain: controlDisplayHost(host),
    projectId: 'control',
    projectName: 'Dokyr control plane · system route',
    httpsEnabled: false,
    services: [{ id: 'control', name: 'Dokyr control plane', containerPort: 8080 }],
    rules: [{ path: '/*', serviceId: 'control', port: 8080 }],
    url: controlURL(host)
  }));
  $: domains = [...catalogDomains, ...registryDomains, ...controlDomains];
  $: filteredDomains = domains.filter((domain) =>
    `${domain.domain} ${domain.projectName} ${domain.status || ''} ${(domain.rules || []).map((rule) => `${rule.path} ${serviceName(domain.services, rule.serviceId)}`).join(' ')}`
      .toLowerCase()
      .includes(query.trim().toLowerCase())
  );
  $: verifiedCount = managedDomains.filter((domain) => domain.status === 'verified').length;
  $: secureCount = domains.filter((domain) => domain.httpsEnabled).length;
  $: edgeTargets = new Set(domains.map((domain) => `${domain.kind}:${domain.projectId}`)).size;
  $: eligibleProjects = projects.filter((project) => project.services.length > 0);
  $: activeProject = projects.find((project) => project.id === draft?.projectId);
  $: activeServices = activeProject?.services || [];

  onMount(async () => {
    await load();
    if (new URLSearchParams(location.search).get('platform') === '1') {
      platformInput?.focus();
      document.getElementById('platform-domain')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  });

  async function load() {
    loading = true;
    error = '';
    try {
      const response = await api('/api/domains');
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not load domains');
      connected = payload.connected;
      connectionError = payload.connectionError || '';
      projects = (payload.projects || []).map((project) => ({
        ...project,
        domains: (project.domains || []).map(normalizeBinding),
        services: project.services || []
      }));
      managedDomains = payload.managedDomains || [];
      dnsTarget = payload.dnsTarget || dnsTarget;
      registryDomain = normalizeRegistry(payload.registry);
      controlHosts = payload.controlHosts || [];
      publicURL = payload.publicURL || '';
      platform = payload.platform || platform;
      platformDomain = platform.domain || '';
      routes = payload.routes || [];
      configuration = payload.configuration || '';
      managedConfiguration = configuration;
      configDirty = false;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not load domains';
    } finally {
      loading = false;
    }
  }

  async function savePlatformDomain(domain = platformDomain) {
    platformSaving = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/settings/platform/domain', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: domain.trim() })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not update the platform domain');
      platform = payload;
      platformDomain = payload.domain || '';
      await load();
      notice = payload.domain
        ? `${payload.domain} is now the permanent Dokyr address. Automatic HTTPS is being provisioned.`
        : 'The custom platform domain was removed. Dokyr is using its temporary server address again.';
      toast.success(payload.domain ? 'Platform domain connected' : 'Platform domain removed');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not update the platform domain';
    } finally {
      platformSaving = false;
    }
  }

  function normalizeBinding(binding) {
    return {
      domain: binding.domain || '',
      httpsEnabled: Boolean(binding.httpsEnabled),
      rules: (binding.rules || []).map((rule) => ({
        path: rule.path || '/*',
        port: Number(rule.port) || 80,
        serviceId: rule.serviceId || ''
      }))
    };
  }

  function normalizeRegistry(value = {}) {
    return {
      domain: value.domain || '',
      httpsEnabled: Boolean(value.httpsEnabled),
      attached: Boolean(value.attached),
      registryHosts: value.registryHosts || []
    };
  }

  function hostnameOnly(value) {
    if (value.startsWith('[')) return value.slice(1, value.indexOf(']'));
    return value.replace(/:\d+$/, '');
  }

  function controlDisplayHost(host) {
    try {
      const parsed = new URL(publicURL);
      return parsed.hostname === host ? parsed.host : host;
    } catch {
      return host;
    }
  }

  function controlURL(host) {
    try {
      const parsed = new URL(publicURL);
      if (parsed.hostname === host) return parsed.toString();
    } catch {
      // Fall through to the Caddy HTTP listener.
    }
    return `http://${host}`;
  }

  function serviceName(services, serviceId) {
    return services.find((service) => service.id === (serviceId || ''))?.name || 'Unknown service';
  }

  function endpoint(domain) {
    return domain.url || `${domain.httpsEnabled ? 'https' : 'http'}://${domain.domain}`;
  }

  function openCreate() {
    const project = eligibleProjects[0];
    const service = project?.services[0];
    editingProjectId = '';
    editingDomainIndex = -1;
    draft = {
      kind: 'project',
      projectId: '',
      domain: '',
      httpsEnabled: true,
      managedId: '',
      status: 'pending',
      dns: null,
      rules: service ? [{ path: '/*', serviceId: service.id, port: service.containerPort }] : []
    };
    error = '';
    modalOpen = true;
  }

  function openEdit(domain) {
    if (domain.kind === 'registry') {
      editingProjectId = 'registry';
      editingDomainIndex = -1;
      draft = {
        kind: 'registry',
        domain: domain.editableDomain,
        httpsEnabled: domain.httpsEnabled,
        attached: domain.attached
      };
      error = '';
      modalOpen = true;
      return;
    }
    editingProjectId = domain.projectId || '';
    editingDomainIndex = domain.projectId ? domain.index : -1;
    draft = {
      kind: domain.kind === 'managed' ? 'project' : domain.kind,
      projectId: domain.projectId || '',
      domain: domain.domain,
      httpsEnabled: domain.httpsEnabled,
      managedId: domain.id || '',
      status: domain.status || 'pending',
      lastError: domain.lastError || '',
      observedRecords: domain.observedRecords || '',
      dns: domain.dns || null,
      rules: (domain.rules || []).length ? domain.rules.map((rule) => ({ ...rule })) : []
    };
    error = '';
    modalOpen = true;
  }

  function closeModal() {
    if (saving || deleting) return;
    modalOpen = false;
    draft = null;
    editingProjectId = '';
    editingDomainIndex = -1;
  }

  function chooseProject(projectId, clearWhenEmpty = true) {
    const project = projects.find((item) => item.id === projectId);
    const service = project?.services[0];
    draft = {
      ...draft,
      projectId,
      rules: service ? [{ path: '/*', serviceId: service.id, port: service.containerPort }] : clearWhenEmpty ? [] : draft.rules
    };
  }

  function chooseService(ruleIndex, serviceId) {
    const service = activeServices.find((item) => item.id === serviceId);
    draft = {
      ...draft,
      rules: draft.rules.map((rule, index) => index === ruleIndex
        ? { ...rule, serviceId, port: service?.containerPort || rule.port }
        : rule)
    };
  }

  function addRule() {
    const service = activeServices[0];
    if (!service) return;
    draft = {
      ...draft,
      rules: [...draft.rules, { path: '/api/*', serviceId: service.id, port: service.containerPort }]
    };
  }

  function removeRule(index) {
    if (draft.rules.length === 1) return;
    draft = { ...draft, rules: draft.rules.filter((_, ruleIndex) => ruleIndex !== index) };
  }

  function serializedBindings(bindings) {
    return bindings.map((binding) => ({
      domain: binding.domain.trim(),
      httpsEnabled: binding.httpsEnabled,
      rules: binding.rules.map((rule) => ({
        path: rule.path.trim(),
        port: Number(rule.port),
        serviceId: rule.serviceId || ''
      }))
    }));
  }

  async function persistProjectDomains(projectId, bindings) {
    const response = await api(`/api/projects/${projectId}/domain`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domains: serializedBindings(bindings) })
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || 'Could not update domain routing');
    projects = projects.map((project) => project.id === projectId
      ? { ...project, domains: (payload.domainBindings || []).map(normalizeBinding) }
      : project);
    await refreshRuntime();
  }

  async function persistRegistryDomain(domain, httpsEnabled) {
    const response = await api('/api/registry/domain', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain, httpsEnabled })
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || 'Could not update the registry domain');
    registryDomain = normalizeRegistry(payload);
    await refreshRuntime();
  }

  async function saveDomain() {
    if (!draft) return;
    saving = true;
    error = '';
    notice = '';
    try {
      if (draft.kind === 'registry') {
        const hostname = draft.domain.trim();
        await persistRegistryDomain(hostname, draft.httpsEnabled);
        notice = `${hostname} is now the container registry domain.`;
        toast.success('Registry domain updated');
        modalOpen = false;
        draft = null;
        editingProjectId = '';
        editingDomainIndex = -1;
        return;
      }
      let managed = managedDomains.find((item) => item.id === draft.managedId);
      if (!managed) {
        const response = await api('/api/domains', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ domain: draft.domain.trim() })
        });
        const payload = await response.json();
        if (!response.ok) throw new Error(payload.error || 'Could not add domain');
        managed = payload.domain;
        managedDomains = [managed, ...managedDomains];
        draft = { ...draft, managedId: managed.id, dns: managed.dns, status: managed.status };
      }
      if (!draft.projectId) {
        notice = `${managed.domain} is saved. Add the DNS record, verify it, then attach it to a project whenever you are ready.`;
        toast.success('Domain saved');
        modalOpen = false;
        draft = null;
        return;
      }
      const project = projects.find((item) => item.id === draft.projectId);
      if (!project) throw new Error('Choose a project or keep the domain unassigned');
      const nextBinding = normalizeBinding(draft);
      const bindings = editingDomainIndex === -1
        ? [...project.domains, nextBinding]
        : project.domains.map((binding, index) => index === editingDomainIndex ? nextBinding : binding);
      await persistProjectDomains(project.id, bindings);
      notice = `${nextBinding.domain.trim()} now routes to ${nextBinding.rules.length} service path${nextBinding.rules.length === 1 ? '' : 's'}.`;
      toast.success(editingDomainIndex === -1 ? 'Domain added' : 'Domain route updated');
      modalOpen = false;
      draft = null;
      editingProjectId = '';
      editingDomainIndex = -1;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not update domain routing';
    } finally {
      saving = false;
    }
  }

  async function deleteDomain() {
    if (!draft || (draft.kind !== 'registry' && !draft.managedId && editingDomainIndex === -1)) return;
    deleting = true;
    error = '';
    notice = '';
    try {
      if (draft.kind === 'registry') {
        const removedDomain = draft.domain;
        await persistRegistryDomain('', true);
        const fallback = registryDomain.registryHosts.join(', ');
        notice = `${removedDomain} was detached. ${fallback || 'The environment fallback'} is active again.`;
        toast.success('Registry domain detached');
        modalOpen = false;
        draft = null;
        editingProjectId = '';
        editingDomainIndex = -1;
        return;
      }
      if (draft.managedId && !draft.projectId) {
        const response = await api(`/api/domains/${draft.managedId}`, { method: 'DELETE' });
        if (!response.ok) {
          const payload = await response.json();
          throw new Error(payload.error || 'Could not delete domain');
        }
        managedDomains = managedDomains.filter((item) => item.id !== draft.managedId);
        notice = `${draft.domain} was removed from Dokyr.`;
        toast.success('Domain deleted');
        modalOpen = false;
        draft = null;
        editingProjectId = '';
        editingDomainIndex = -1;
        return;
      }
      const project = projects.find((item) => item.id === editingProjectId);
      if (!project) throw new Error('Project not found');
      const removedDomain = project.domains[editingDomainIndex]?.domain || draft.domain;
      await persistProjectDomains(project.id, project.domains.filter((_, index) => index !== editingDomainIndex));
      notice = `${removedDomain} was detached from ${project.name} and remains available in Domains.`;
      toast.success('Domain detached');
      modalOpen = false;
      draft = null;
      editingProjectId = '';
      editingDomainIndex = -1;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not remove domain';
    } finally {
      deleting = false;
    }
  }

  async function refreshRuntime() {
    const response = await api('/api/caddy/config');
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || 'Could not refresh Caddy state');
    connected = payload.connected;
    connectionError = payload.connectionError || '';
    routes = payload.routes || [];
    configuration = payload.configuration || '';
    managedConfiguration = configuration;
    configDirty = false;
  }

  function changeConfiguration(event) {
    configuration = event.currentTarget.value;
    configDirty = configuration !== managedConfiguration;
    notice = '';
  }

  async function applyConfiguration() {
    configSaving = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/caddy/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ configuration })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Caddy rejected the configuration');
      connected = true;
      notice = 'Caddy accepted the runtime configuration without downtime.';
      toast.success('Runtime configuration applied');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Caddy rejected the configuration';
    } finally {
      configSaving = false;
    }
  }

  async function resetManaged() {
    resetting = true;
    error = '';
    notice = '';
    try {
      const response = await api('/api/caddy/reset', { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not restore managed routes');
      routes = payload.routes || [];
      configuration = payload.configuration || '';
      managedConfiguration = configuration;
      configDirty = false;
      connected = true;
      notice = 'Saved domain routes have been restored.';
      toast.success('Managed routes restored');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not restore managed routes';
    } finally {
      resetting = false;
    }
  }

  async function copyConfiguration() {
    await navigator.clipboard.writeText(configuration);
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }

  async function verifyDNS(domain = draft) {
    if (!domain?.managedId && !domain?.id) return;
    const id = domain.managedId || domain.id;
    verifyingId = id;
    error = '';
    try {
      const response = await api(`/api/domains/${id}/verify`, { method: 'POST' });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || 'Could not check DNS');
      managedDomains = managedDomains.map((item) => item.id === id ? payload.domain : item);
      if (draft?.managedId === id) draft = { ...draft, ...payload.domain, managedId: id };
      toast[payload.verified ? 'success' : 'warning'](payload.verified ? 'DNS verified' : 'DNS is still propagating');
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Could not check DNS';
    } finally {
      verifyingId = '';
    }
  }

  async function copyDNS(domain) {
    if (!domain?.dns) return;
    await navigator.clipboard.writeText(`${domain.dns.type}\t${domain.dns.name}\t${domain.dns.value}`);
    copiedDNS = domain.id || domain.managedId || domain.domain || 'draft';
    setTimeout(() => (copiedDNS = ''), 1500);
  }
</script>

<Shell eyebrow="Infrastructure" title="Domains" subtitle="Add once, verify DNS, and connect a hostname to any project when it is ready.">
  <div slot="actions" class="page-actions">
    <div class="connection" class:offline={!connected}>
      <i></i>
      <span><strong>{connected ? 'Edge online' : 'Edge unavailable'}</strong><small>Caddy router</small></span>
    </div>
    <button class="btn btn-primary" onclick={openCreate} disabled={loading}>
      <Icon name="plus" size={14} /> Add domain
    </button>
  </div>

  {#if error && !modalOpen}
    <div class="alert alert-error page-alert"><Icon name="x-circle" size={15} /><div><strong>Domain not changed</strong><span>{error}</span></div></div>
  {/if}
  {#if notice}
    <div class="alert alert-success page-alert"><Icon name="check-circle" size={15} /><div><strong>Domains updated</strong><span>{notice}</span></div></div>
  {/if}
  {#if connectionError}
    <div class="alert alert-warning page-alert"><Icon name="alert" size={15} /><div><strong>The edge router is unavailable</strong><span>{connectionError}</span></div></div>
  {/if}

  <section id="platform-domain" class="panel platform-domain" aria-labelledby="platform-domain-title">
    <div class="platform-domain-copy">
      <span class="platform-domain-icon"><Icon name="shield" size={18} /></span>
      <div>
        <span class="eyebrow">Control panel address</span>
        <h2 id="platform-domain-title">{platform.customDomainConfigured ? 'Permanent domain connected' : 'Replace the temporary server URL'}</h2>
        <p>{platform.customDomainConfigured ? `Dokyr is available at ${platform.publicURL}.` : `The current address, ${platform.publicURL || publicURL}, is based on this server's IP. Point a domain to ${dnsTarget.value}, then connect it here.`}</p>
      </div>
    </div>
    <form onsubmit={(event) => { event.preventDefault(); savePlatformDomain(); }}>
      <label>
        <span>Platform domain</span>
        <div class="platform-domain-input">
          <Icon name="globe" size={14} />
          <input bind:this={platformInput} bind:value={platformDomain} autocomplete="off" spellcheck="false" placeholder="panel.example.com" required />
        </div>
      </label>
      <button class="btn btn-primary" type="submit" disabled={platformSaving || !platformDomain.trim()}>{platformSaving ? 'Connecting…' : platform.domain ? 'Update domain' : 'Connect domain'}</button>
      {#if platform.domain}<button class="btn" type="button" onclick={() => savePlatformDomain('')} disabled={platformSaving}>Remove</button>{/if}
    </form>
    <footer>
      <span><Icon name="network" size={13} /> DNS record</span>
      <code>{dnsTarget.type} · {platformDomain || 'panel.example.com'} → {dnsTarget.value}</code>
      <span><Icon name="lock" size={13} /> Automatic HTTPS</span>
    </footer>
  </section>

  <section class="domain-overview" aria-label="Domain summary">
    <article>
      <span class="metric-icon"><Icon name="globe" size={16} /></span>
      <div><strong>{loading ? '—' : managedDomains.length}</strong><span>Saved domains</span></div>
    </article>
    <article>
      <span class="metric-icon secure"><Icon name="check-circle" size={16} /></span>
      <div><strong>{loading ? '—' : verifiedCount}</strong><span>DNS verified</span></div>
    </article>
    <article>
      <span class="metric-icon projects"><Icon name="box" size={16} /></span>
      <div><strong>{loading ? '—' : edgeTargets}</strong><span>Edge targets</span></div>
    </article>
    <div class="flow-note">
      <span>Request flow</span>
      <code>DNS → domain → project → service</code>
    </div>
  </section>

  <section class="panel domains-panel" aria-busy={loading}>
    <header class="panel-header domain-toolbar">
      <div>
        <span class="eyebrow">Reusable domain catalog</span>
        <h2>Domain names {#if !loading}<span class="count">{filteredDomains.length}</span>{/if}</h2>
      </div>
      {#if domains.length > 0}
        <label class="search-field">
          <Icon name="search" size={14} />
          <input bind:value={query} type="search" placeholder="Search domains or services" aria-label="Search domains or services" />
        </label>
      {/if}
    </header>

    {#if loading}
      <div class="domain-loading">
        {#each Array(3) as _}
          <div><span class="skeleton" style="width:38px;height:38px"></span><span class="skeleton grow" style="height:14px"></span><span class="skeleton" style="width:100px;height:24px"></span></div>
        {/each}
      </div>
    {:else if domains.length === 0}
      <EmptyState icon="globe" title="No Caddy hostnames configured" description="Add a project service or configure a registry hostname to create the first edge route.">
        {#if eligibleProjects.length > 0}
          <button class="btn btn-primary btn-sm" onclick={openCreate}><Icon name="plus" size={13} /> Add first domain</button>
        {:else}
          <a class="btn btn-primary btn-sm" href="/projects"><Icon name="box" size={13} /> Open projects</a>
        {/if}
      </EmptyState>
    {:else if filteredDomains.length === 0}
      <EmptyState icon="search" title="No matching domains" description="Try a hostname, project, service, or path.">
        <button class="btn btn-sm" onclick={() => (query = '')}>Clear search</button>
      </EmptyState>
    {:else}
      <div class="domain-list">
        {#each filteredDomains as domain}
          <article class="domain-row" class:unassigned={domain.kind === 'managed' && !domain.projectId}>
            <span class="domain-status" class:secure={domain.status === 'verified' || domain.httpsEnabled} class:pending={domain.kind === 'managed' && domain.status !== 'verified'}>
              <Icon name={domain.kind === 'registry' ? 'layers' : domain.status === 'verified' ? 'check' : domain.httpsEnabled ? 'lock' : 'globe'} size={15} />
            </span>
            <div class="domain-identity">
              {#if domain.projectId || domain.kind !== 'managed'}
                <a href={endpoint(domain)} target="_blank" rel="noreferrer">{domain.domain}<Icon name="external" size={12} /></a>
              {:else}<strong>{domain.domain}</strong>{/if}
              <span><Icon name={domain.kind === 'registry' ? 'layers' : domain.kind === 'control' ? 'shield' : 'box'} size={12} /> {domain.projectName}</span>
            </div>
            <div class="route-stack">
              {#if domain.kind === 'managed'}
                <div class="dns-summary"><code>{domain.dns.type}</code><strong>{domain.dns.name}</strong><Icon name="arrow-right" size={12} /><span>{domain.dns.value}</span></div>
              {:else}
                {#each domain.rules as rule}
                  <div><code>{rule.path}</code><Icon name="arrow-right" size={12} /><strong>{serviceName(domain.services, rule.serviceId)}</strong><small>:{rule.port}</small></div>
                {/each}
              {/if}
            </div>
            {#if domain.kind === 'managed'}
              <button class="verify-badge" class:verified={domain.status === 'verified'} type="button" onclick={() => verifyDNS(domain)} disabled={verifyingId === domain.id}>
                <Icon name={domain.status === 'verified' ? 'check-circle' : 'refresh'} size={12} />{verifyingId === domain.id ? 'Checking…' : domain.status === 'verified' ? 'DNS verified' : 'Verify DNS'}
              </button>
            {:else}<span class="tls-badge" class:http={!domain.httpsEnabled}>{domain.httpsEnabled ? 'Automatic SSL' : 'HTTP only'}</span>{/if}
            {#if domain.kind === 'control'}
              <span class="system-badge"><Icon name="lock" size={11} /> System</span>
            {:else}
              <button class="edit-button" type="button" onclick={() => openEdit(domain)}><Icon name="settings" size={13} /> {domain.kind === 'managed' && !domain.projectId ? 'Use domain' : domain.kind === 'registry' && !domain.attached ? 'Configure' : 'Edit'}</button>
            {/if}
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="edge-guide" aria-label="How domain routing works">
    <article><span><Icon name="globe" size={15} /></span><div><strong>Add once</strong><p>Save a domain to Dokyr without choosing a project yet.</p></div></article>
    <article><span><Icon name="network" size={15} /></span><div><strong>Verify DNS</strong><p>Dokyr shows the exact record and checks public DNS propagation.</p></div></article>
    <article><span><Icon name="shield" size={15} /></span><div><strong>Use anywhere</strong><p>Attach it now or select it later from a project's Domains tab.</p></div></article>
  </section>

  <details class="panel advanced-panel">
    <summary>
      <span class="advanced-icon"><Icon name="terminal" size={15} /></span>
      <span><strong>Advanced Caddy configuration</strong><small>Inspect or temporarily override the generated runtime configuration.</small></span>
      <span class="summary-chevron"><Icon name="chevron-down" size={15} /></span>
    </summary>
    <div class="advanced-body">
      <header>
        <div>
          <span class="eyebrow">Runtime editor</span>
          <h2>Generated Caddyfile</h2>
          <p>Domain changes regenerate this file. Manual edits are temporary and can be replaced by the next managed route update.</p>
        </div>
        <div class="editor-actions">
          <button class="btn btn-sm" onclick={copyConfiguration}><Icon name={copied ? 'check' : 'copy'} size={13} />{copied ? 'Copied' : 'Copy'}</button>
          <button class="btn btn-sm" onclick={resetManaged} disabled={resetting}><Icon name="refresh" size={13} />{resetting ? 'Restoring…' : 'Restore managed'}</button>
          <button class="btn btn-sm btn-primary" onclick={applyConfiguration} disabled={configSaving || !configDirty || !connected}>{configSaving ? 'Applying…' : 'Validate & apply'}</button>
        </div>
      </header>
      <div class="editor-state">
        <span><i class:changed={configDirty}></i>{configDirty ? 'Unsaved runtime override' : 'Matches saved domains'}</span>
        <code>{domains.length} domain{domains.length === 1 ? '' : 's'} · text/caddyfile</code>
      </div>
      <textarea value={configuration} oninput={changeConfiguration} spellcheck="false" aria-label="Caddy configuration"></textarea>
    </div>
  </details>
</Shell>

{#if modalOpen && draft}
  <div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) closeModal(); }}>
    <div class="domain-modal" role="dialog" aria-modal="true" aria-labelledby="domain-modal-title">
      <header>
        <div>
          <span>{draft.kind === 'registry' ? 'Container registry endpoint' : draft.managedId ? 'Managed domain' : 'Add to Dokyr'}</span>
          <h2 id="domain-modal-title">{draft.kind === 'registry' ? 'Configure registry domain' : draft.managedId ? draft.domain : 'Add a domain'}</h2>
        </div>
        <button type="button" aria-label="Close" onclick={closeModal} disabled={saving || deleting}><Icon name="x" size={16} /></button>
      </header>

      <form onsubmit={(event) => { event.preventDefault(); saveDomain(); }}>
        <div class="modal-scroll">
          {#if error}
            <div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Check this route</strong><span>{error}</span></div></div>
          {/if}

          <section class="form-section">
            <div class="section-number">1</div>
            <div class="section-content">
              <div class="section-heading">
                <strong>{draft.kind === 'registry' ? 'Hostname and destination' : 'Hostname and project'}</strong>
                <span>{draft.kind === 'registry' ? 'The built-in registry keeps its authentication and image API routes together.' : 'The project sets the list of services available as destinations.'}</span>
              </div>
              <div class="identity-grid">
                <label class="field">
                  <span>Domain name</span>
                  <div class="domain-input"><Icon name="globe" size={14} /><input bind:value={draft.domain} autocomplete="off" spellcheck="false" placeholder="app.example.com" required readonly={Boolean(draft.managedId)} /></div>
                  <small>Hostname only — do not include <code>http://</code> or a path.</small>
                </label>
                {#if draft.kind === 'registry'}
                  <div class="registry-destination">
                    <span class="registry-mark"><Icon name="layers" size={17} /></span>
                    <span><strong>Dokyr Container Registry</strong><small>Fixed private upstream <code>registry:5000</code></small></span>
                    {#if !draft.attached}<em>Environment fallback</em>{/if}
                  </div>
                {:else}
                  <label class="field">
                    <span>Project</span>
                    <select class="select" value={draft.projectId} onchange={(event) => chooseProject(event.currentTarget.value)} disabled={editingDomainIndex !== -1}>
                      <option value="">Keep unassigned</option>
                      {#each eligibleProjects as project}<option value={project.id}>{project.name}</option>{/each}
                    </select>
                    <small>{editingDomainIndex === -1 ? 'Attach it now, or save it to use from a project later.' : 'Detach it before moving it to another project.'}</small>
                  </label>
                {/if}
              </div>
              {#if draft.kind !== 'registry'}
                <div class="dns-card" class:verified={draft.status === 'verified'}>
                  <span class="dns-card-icon"><Icon name={draft.status === 'verified' ? 'check-circle' : 'network'} size={18} /></span>
                  <div class="dns-card-copy">
                    <span>{draft.status === 'verified' ? 'DNS verified' : 'Point this hostname to Dokyr'}</span>
                    <div class="dns-record">
                      <code>{draft.dns?.type || dnsTarget.type}</code>
                      <strong>{draft.domain || 'your hostname'}</strong>
                      <Icon name="arrow-right" size={13} />
                      <code>{draft.dns?.value || dnsTarget.value}</code>
                    </div>
                    {#if draft.lastError}<small>{draft.lastError}</small>{:else}<small>Add this record at your DNS provider. Verification may take a few minutes after propagation.</small>{/if}
                  </div>
                  <div class="dns-card-actions">
                    <button type="button" onclick={() => copyDNS({ ...draft, dns: draft.dns || { ...dnsTarget, name: draft.domain } })}><Icon name={copiedDNS === (draft.managedId || draft.domain || 'draft') ? 'check' : 'copy'} size={13} />{copiedDNS === (draft.managedId || draft.domain || 'draft') ? 'Copied' : 'Copy'}</button>
                    {#if draft.managedId}<button type="button" onclick={() => verifyDNS(draft)} disabled={verifyingId === draft.managedId}><Icon name="refresh" size={13} />{verifyingId === draft.managedId ? 'Checking…' : 'Verify now'}</button>{/if}
                  </div>
                </div>
              {/if}
            </div>
          </section>

          <section class="form-section">
            <div class="section-number">2</div>
            <div class="section-content">
              {#if draft.kind === 'registry'}
                <div class="section-heading"><strong>Managed registry routes</strong><span>These protected paths stay fixed so Docker authentication and image traffic continue to work.</span></div>
                <div class="registry-routes">
                  <div><code>/api/registry/token</code><Icon name="arrow-right" size={13} /><span><strong>Dokyr token service</strong><small>:8080</small></span></div>
                  <div><code>/*</code><Icon name="arrow-right" size={13} /><span><strong>Docker Registry</strong><small>:5000</small></span></div>
                </div>
              {:else if draft.projectId}
                <div class="section-heading route-heading">
                  <div><strong>Reverse proxy destinations</strong><span>Rules are evaluated from top to bottom. Use <code>/*</code> as the catch-all.</span></div>
                  <button type="button" onclick={addRule}><Icon name="plus" size={13} /> Add path</button>
                </div>
                <div class="rule-list">
                  {#each draft.rules as rule, index}
                    <div class="rule-row">
                      <label><span>Request path</span><input bind:value={rule.path} required placeholder="/*" spellcheck="false" /></label>
                      <span class="rule-arrow"><Icon name="arrow-right" size={15} /></span>
                      <label><span>Service</span><select value={rule.serviceId || ''} onchange={(event) => chooseService(index, event.currentTarget.value)}>{#each activeServices as service}<option value={service.id}>{service.name}{service.legacy ? ' · default' : ''}</option>{/each}</select></label>
                      <label class="port-field"><span>Port</span><input bind:value={rule.port} type="number" min="1" max="65535" required /></label>
                      <button class="remove-rule" type="button" onclick={() => removeRule(index)} disabled={draft.rules.length === 1} aria-label="Remove path rule"><Icon name="trash" size={13} /></button>
                    </div>
                  {/each}
                </div>
              {:else}
                <div class="unassigned-note"><Icon name="folder" size={18} /><div><strong>Ready for later</strong><span>This domain will stay in Infrastructure → Domains until you select it from a project.</span></div></div>
              {/if}
            </div>
          </section>

          {#if draft.kind === 'registry' || draft.projectId}
          <section class="form-section">
            <div class="section-number">3</div>
            <div class="section-content">
              <div class="section-heading"><strong>Connection security</strong><span>Choose how visitors connect to this hostname.</span></div>
              <div class="security-picker">
                <label class:active={draft.httpsEnabled}>
                  <input type="radio" bind:group={draft.httpsEnabled} value={true} />
                  <span class="security-icon"><Icon name="shield" size={17} /></span>
                  <span><strong>Automatic SSL</strong><small>HTTPS with automatic certificate issue, renewal, and HTTP redirect.</small></span>
                  <i><Icon name="check" size={12} /></i>
                </label>
                <label class:active={!draft.httpsEnabled}>
                  <input type="radio" bind:group={draft.httpsEnabled} value={false} />
                  <span class="security-icon"><Icon name="globe" size={17} /></span>
                  <span><strong>HTTP only</strong><small>Plain HTTP for local networks, development hostnames, or upstream TLS termination.</small></span>
                  <i><Icon name="check" size={12} /></i>
                </label>
              </div>
            </div>
          </section>
          {/if}
        </div>

        <footer>
          {#if draft.kind === 'registry' && draft.attached}
            <button class="btn btn-danger" type="button" onclick={deleteDomain} disabled={saving || deleting}><Icon name="trash" size={13} />{deleting ? 'Detaching…' : 'Detach domain'}</button>
          {:else if draft.kind === 'project' && draft.managedId}
            <button class="btn btn-danger" type="button" onclick={deleteDomain} disabled={saving || deleting}><Icon name="trash" size={13} />{deleting ? 'Updating…' : draft.projectId ? 'Detach from project' : 'Delete domain'}</button>
          {:else}
            <span>Changes are applied to Caddy without downtime.</span>
          {/if}
          <div>
            <button class="btn" type="button" onclick={closeModal} disabled={saving || deleting}>Cancel</button>
            <button class="btn btn-primary" type="submit" disabled={saving || deleting || !draft.domain.trim() || (draft.kind === 'project' && draft.projectId && draft.rules.length === 0)}>
              <Icon name="check" size={13} />{saving ? 'Saving…' : draft.kind === 'registry' ? 'Save registry domain' : draft.projectId ? 'Save & use domain' : 'Save for later'}
            </button>
          </div>
        </footer>
      </form>
    </div>
  </div>
{/if}

<style>
  .page-actions { display: flex; align-items: center; gap: var(--space-2); }
  .connection { min-width: 134px; min-height: 34px; padding: 6px var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid color-mix(in srgb, var(--color-success) 30%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-paper-raised); }
  .connection > i { width: 7px; height: 7px; flex: none; border-radius: 50%; background: var(--color-success); box-shadow: 0 0 0 4px var(--color-success-soft); }
  .connection.offline { border-color: color-mix(in srgb, var(--color-danger) 30%, var(--color-rule)); }
  .connection.offline > i { background: var(--color-danger); box-shadow: none; }
  .connection > span { display: grid; gap: 1px; }
  .connection strong { font-size: var(--text-xs); }
  .connection small { color: var(--color-muted); font-size: var(--text-2xs); }
  .page-alert { margin-bottom: var(--space-4); }

  .platform-domain { margin-bottom: var(--space-4); overflow: hidden; }
  .platform-domain-copy { padding: var(--space-5); display: flex; align-items: center; gap: var(--space-4); }
  .platform-domain-icon { width: 42px; height: 42px; flex: none; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .platform-domain-copy > div { min-width: 0; display: grid; gap: 3px; }
  .platform-domain-copy h2 { margin: 0; font-size: var(--text-lg); }
  .platform-domain-copy p { margin: 0; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .platform-domain form { padding: 0 var(--space-5) var(--space-5); display: grid; grid-template-columns: minmax(240px, 1fr) auto auto; align-items: end; gap: var(--space-2); }
  .platform-domain form label { display: grid; gap: 6px; }
  .platform-domain form label > span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; }
  .platform-domain-input { height: 38px; padding: 0 var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); color: var(--color-muted); }
  .platform-domain-input:focus-within { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 14%, transparent); }
  .platform-domain-input input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: var(--color-ink); font: var(--text-sm) var(--font-mono); }
  .platform-domain footer { min-height: 42px; padding: 0 var(--space-5); display: flex; align-items: center; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); }
  .platform-domain footer span { display: inline-flex; align-items: center; gap: 5px; }
  .platform-domain footer code { color: var(--color-ink-secondary); font-size: var(--text-2xs); }
  .platform-domain footer span:last-child { margin-left: auto; color: var(--color-success); }

  .domain-overview { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(3, minmax(140px, .45fr)) minmax(260px, 1fr); overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .domain-overview article { min-height: 74px; padding: var(--space-4); display: flex; align-items: center; gap: var(--space-3); border-right: 1px solid var(--color-rule); }
  .metric-icon { width: 34px; height: 34px; display: grid; place-items: center; flex: none; border: 1px solid color-mix(in srgb, var(--color-accent) 25%, var(--color-rule)); border-radius: var(--radius-md); background: var(--color-accent-softer); color: var(--color-accent); }
  .metric-icon.secure { border-color: color-mix(in srgb, var(--color-success) 25%, var(--color-rule)); background: var(--color-success-soft); color: var(--color-success); }
  .metric-icon.projects { border-color: color-mix(in srgb, var(--color-info) 25%, var(--color-rule)); background: var(--color-info-soft); color: var(--color-info); }
  .domain-overview article div { display: grid; }
  .domain-overview article strong { font-size: var(--text-xl); line-height: 1.1; letter-spacing: -0.03em; }
  .domain-overview article span:last-child { color: var(--color-muted); font-size: var(--text-xs); }
  .flow-note { padding: var(--space-4) var(--space-5); display: grid; align-content: center; justify-items: end; gap: var(--space-1); background: linear-gradient(120deg, var(--color-surface-subtle), var(--color-accent-softer)); }
  .flow-note span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
  .flow-note code { color: var(--color-ink); font-size: var(--text-sm); }

  .domains-panel { margin-bottom: var(--space-4); }
  .domain-toolbar h2 { display: flex; align-items: center; gap: var(--space-2); }
  .count { min-width: 22px; height: 20px; padding: 0 6px; display: inline-grid; place-items: center; border-radius: 999px; background: var(--color-paper-subtle); color: var(--color-muted); font-size: var(--text-2xs); }
  .search-field { width: min(290px, 45vw); height: 34px; padding: 0 var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-muted); }
  .search-field:focus-within { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 14%, transparent); }
  .search-field input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: var(--color-ink); font-size: var(--text-xs); }
  .domain-loading { padding: var(--space-2) var(--space-5); }
  .domain-loading > div { min-height: 66px; display: flex; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .domain-loading > div:last-child { border-bottom: 0; }
  .domain-loading .grow { flex: 1; }
  .domain-list { padding: 0 var(--space-5); }
  .domain-row { min-height: 76px; display: grid; grid-template-columns: 38px minmax(180px, .8fr) minmax(260px, 1.4fr) auto auto; align-items: center; gap: var(--space-3); border-bottom: 1px solid var(--color-rule); }
  .domain-row:last-child { border-bottom: 0; }
  .domain-status { width: 34px; height: 34px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; background: var(--color-surface-subtle); color: var(--color-muted); }
  .domain-status.secure { border-color: color-mix(in srgb, var(--color-success) 28%, var(--color-rule)); background: var(--color-success-soft); color: var(--color-success); }
  .domain-status.pending { border-style: dashed; border-color: color-mix(in srgb, var(--color-warning) 45%, var(--color-rule)); background: var(--color-warning-soft); color: var(--color-warning); }
  .domain-identity { min-width: 0; display: grid; gap: 4px; }
  .domain-identity > a { min-width: 0; display: flex; align-items: center; gap: 5px; overflow: hidden; color: var(--color-ink); font: 600 var(--text-sm) var(--font-mono); text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
  .domain-identity > strong { overflow: hidden; color: var(--color-ink); font: 600 var(--text-sm) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .domain-identity > a:hover { color: var(--color-accent); }
  .domain-identity > span { display: flex; align-items: center; gap: 5px; color: var(--color-muted); font-size: var(--text-xs); }
  .route-stack { min-width: 0; padding: var(--space-2) 0; display: grid; gap: 4px; }
  .route-stack > div { min-width: 0; display: grid; grid-template-columns: minmax(58px, auto) auto minmax(80px, auto) 1fr; align-items: center; justify-content: start; gap: var(--space-2); color: var(--color-muted); font-size: var(--text-xs); }
  .route-stack code { max-width: 120px; overflow: hidden; color: var(--color-ink-secondary); text-overflow: ellipsis; white-space: nowrap; }
  .route-stack strong { overflow: hidden; color: var(--color-ink); font-size: var(--text-xs); text-overflow: ellipsis; white-space: nowrap; }
  .route-stack small { color: var(--color-muted); font: var(--text-2xs) var(--font-mono); }
  .route-stack .dns-summary { grid-template-columns: auto minmax(100px, auto) auto minmax(100px, 1fr); }
  .dns-summary code:first-child { padding: 3px 6px; border-radius: 4px; background: var(--color-accent-softer); color: var(--color-accent); font-weight: 700; }
  .dns-summary span { overflow: hidden; color: var(--color-muted); font-family: var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .verify-badge { min-height: 28px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 5px; border: 1px solid color-mix(in srgb, var(--color-warning) 32%, var(--color-rule)); border-radius: 999px; background: var(--color-warning-soft); color: var(--color-warning); font-size: var(--text-2xs); font-weight: 700; white-space: nowrap; cursor: pointer; }
  .verify-badge.verified { border-color: color-mix(in srgb, var(--color-success) 28%, var(--color-rule)); background: var(--color-success-soft); color: var(--color-success); }
  .verify-badge:disabled { opacity: .65; cursor: wait; }
  .tls-badge { min-height: 24px; padding: 0 var(--space-2); display: inline-flex; align-items: center; border: 1px solid color-mix(in srgb, var(--color-success) 28%, var(--color-rule)); border-radius: 999px; background: var(--color-success-soft); color: var(--color-success); font-size: var(--text-2xs); font-weight: 700; white-space: nowrap; }
  .tls-badge.http { border-color: var(--color-rule); background: var(--color-surface-subtle); color: var(--color-muted); }
  .edit-button { min-height: 30px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .edit-button:hover { border-color: var(--color-rule-strong); background: var(--color-paper-subtle); color: var(--color-ink); }
  .system-badge { min-height: 26px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 5px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; white-space: nowrap; }

  .edge-guide { margin-bottom: var(--space-4); display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--space-3); }
  .edge-guide article { padding: var(--space-4); display: grid; grid-template-columns: 30px minmax(0, 1fr); gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-paper-raised); box-shadow: var(--shadow-panel); }
  .edge-guide article > span { width: 30px; height: 30px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 25%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); }
  .edge-guide strong { font-size: var(--text-sm); }
  .edge-guide p { margin: 3px 0 0; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }

  .advanced-panel { margin-bottom: var(--space-4); }
  .advanced-panel > summary { min-height: 62px; padding: var(--space-3) var(--space-5); display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); list-style: none; cursor: pointer; }
  .advanced-panel > summary::-webkit-details-marker { display: none; }
  .summary-chevron { display: grid; transition: transform var(--duration-base) var(--ease-out); }
  .advanced-panel[open] .summary-chevron { transform: rotate(180deg); }
  .advanced-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: var(--radius-sm); background: var(--color-log-surface); color: var(--color-log-text); }
  .advanced-panel > summary > span:nth-child(2) { display: grid; gap: 2px; }
  .advanced-panel > summary strong { font-size: var(--text-sm); }
  .advanced-panel > summary small { color: var(--color-muted); font-size: var(--text-xs); }
  .advanced-body { border-top: 1px solid var(--color-rule); }
  .advanced-body > header { padding: var(--space-4) var(--space-5); display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); }
  .advanced-body h2 { margin: 0; font-size: var(--text-md); }
  .advanced-body p { max-width: 72ch; margin: var(--space-1) 0 0; color: var(--color-muted); font-size: var(--text-xs); line-height: 1.5; }
  .editor-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--space-2); }
  .editor-state { min-height: 38px; padding: 0 var(--space-4); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-bottom: 1px solid var(--color-log-rule); background: var(--color-log-surface); color: var(--color-log-muted); }
  .editor-state span { display: flex; align-items: center; gap: var(--space-2); font: 500 var(--text-xs) var(--font-mono); }
  .editor-state i { width: 7px; height: 7px; border-radius: 50%; background: var(--color-success); }
  .editor-state i.changed { background: var(--color-warning); }
  .editor-state code { font-size: var(--text-2xs); }
  .advanced-body textarea { width: 100%; min-height: 380px; padding: var(--space-4); display: block; resize: vertical; border: 0; outline: 0; background: var(--color-log-bg); color: var(--color-log-text); caret-color: var(--color-accent); font: var(--text-sm)/1.65 var(--font-mono); tab-size: 2; }

  .modal-backdrop { position: fixed; z-index: 180; inset: 0; padding: var(--space-5); display: grid; place-items: center; background: rgb(6 12 20 / .58); backdrop-filter: blur(3px); }
  .domain-modal { width: min(880px, 100%); max-height: min(820px, calc(100vh - 40px)); overflow: hidden; display: grid; grid-template-rows: auto minmax(0, 1fr); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-lg); background: var(--color-paper-raised); box-shadow: var(--shadow-modal); }
  .domain-modal > header { min-height: 68px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-rule); }
  .domain-modal > header div { display: grid; gap: 1px; }
  .domain-modal > header span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
  .domain-modal > header h2 { margin: 0; overflow-wrap: anywhere; font-size: var(--text-lg); }
  .domain-modal > header button { width: 32px; height: 32px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }
  .domain-modal > form { min-height: 0; display: grid; grid-template-rows: minmax(0, 1fr) auto; }
  .modal-scroll { min-height: 0; padding: var(--space-5); overflow-y: auto; }
  .modal-scroll > .alert { margin-bottom: var(--space-4); }
  .form-section { padding: var(--space-5) 0; display: grid; grid-template-columns: 28px minmax(0, 1fr); gap: var(--space-4); border-bottom: 1px solid var(--color-rule); }
  .form-section:first-of-type { padding-top: 0; }
  .form-section:last-child { padding-bottom: 0; border-bottom: 0; }
  .section-number { width: 26px; height: 26px; display: grid; place-items: center; border-radius: 50%; background: var(--color-accent); color: var(--color-accent-ink); font: 700 var(--text-xs) var(--font-mono); }
  .section-content { min-width: 0; display: grid; gap: var(--space-4); }
  .section-heading { display: grid; gap: 2px; }
  .section-heading strong { font-size: var(--text-sm); }
  .section-heading > span, .section-heading > div > span { color: var(--color-muted); font-size: var(--text-xs); }
  .route-heading { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); }
  .route-heading > div { display: grid; }
  .route-heading button { min-height: 28px; padding: 0 var(--space-2); display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-ink); font-size: var(--text-xs); font-weight: 600; cursor: pointer; }
  .identity-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
  .field small { color: var(--color-muted); font-size: var(--text-2xs); }
  .domain-input { height: 38px; padding: 0 var(--space-3); display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); color: var(--color-muted); }
  .domain-input:focus-within { border-color: var(--color-focus); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-focus) 14%, transparent); }
  .domain-input input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: var(--color-ink); font: var(--text-sm) var(--font-mono); }
  .domain-input input:read-only { color: var(--color-muted); cursor: default; }
  .dns-card { padding: var(--space-4); display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-rule)); border-radius: var(--radius-md); background: color-mix(in srgb, var(--color-warning-soft) 55%, var(--color-paper-raised)); }
  .dns-card.verified { border-color: color-mix(in srgb, var(--color-success) 32%, var(--color-rule)); background: color-mix(in srgb, var(--color-success-soft) 55%, var(--color-paper-raised)); }
  .dns-card-icon { width: 38px; height: 38px; display: grid; place-items: center; border-radius: 50%; background: var(--color-paper-raised); color: var(--color-warning); box-shadow: inset 0 0 0 1px var(--color-rule); }
  .dns-card.verified .dns-card-icon { color: var(--color-success); }
  .dns-card-copy { min-width: 0; display: grid; gap: 5px; }
  .dns-card-copy > span { font-size: var(--text-xs); font-weight: 700; }
  .dns-card-copy > small { color: var(--color-muted); font-size: var(--text-2xs); line-height: 1.45; }
  .dns-record { min-width: 0; display: flex; align-items: center; gap: var(--space-2); }
  .dns-record code:first-child { padding: 3px 6px; border-radius: 4px; background: var(--color-paper-raised); color: var(--color-accent); font-size: var(--text-2xs); font-weight: 800; }
  .dns-record strong, .dns-record code:last-child { min-width: 0; overflow: hidden; color: var(--color-ink); font: 600 var(--text-xs) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .dns-card-actions { display: grid; gap: 6px; }
  .dns-card-actions button { min-height: 28px; padding: 0 var(--space-2); display: inline-flex; align-items: center; justify-content: center; gap: 6px; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-paper-raised); color: var(--color-ink-secondary); font-size: var(--text-2xs); font-weight: 700; cursor: pointer; }
  .unassigned-note { min-height: 74px; padding: var(--space-4); display: flex; align-items: center; gap: var(--space-3); border: 1px dashed var(--color-rule-strong); border-radius: var(--radius-md); background: var(--color-surface-subtle); color: var(--color-accent); }
  .unassigned-note div { display: grid; gap: 3px; }
  .unassigned-note strong { color: var(--color-ink); font-size: var(--text-sm); }
  .unassigned-note span { color: var(--color-muted); font-size: var(--text-xs); }
  .registry-destination { min-height: 68px; padding: var(--space-3); display: grid; grid-template-columns: 34px minmax(0, 1fr) auto; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .registry-mark { width: 34px; height: 34px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--color-accent) 28%, var(--color-rule)); border-radius: var(--radius-sm); background: var(--color-accent-softer); color: var(--color-accent); }
  .registry-destination > span:nth-child(2) { min-width: 0; display: grid; gap: 3px; }
  .registry-destination strong { font-size: var(--text-xs); }
  .registry-destination small { color: var(--color-muted); font-size: var(--text-2xs); }
  .registry-destination em { padding: 3px 6px; border-radius: 999px; background: var(--color-warning-soft); color: var(--color-warning); font-size: var(--text-2xs); font-style: normal; font-weight: 700; white-space: nowrap; }
  .registry-routes { display: grid; gap: var(--space-2); }
  .registry-routes > div { min-height: 48px; padding: 0 var(--space-3); display: grid; grid-template-columns: minmax(145px, .8fr) auto minmax(160px, 1.2fr); align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: var(--color-surface-subtle); color: var(--color-faint); }
  .registry-routes code { color: var(--color-ink-secondary); font-size: var(--text-xs); }
  .registry-routes > div > span { min-width: 0; display: flex; align-items: baseline; gap: 5px; }
  .registry-routes strong { color: var(--color-ink); font-size: var(--text-xs); }
  .registry-routes small { color: var(--color-muted); font: var(--text-2xs) var(--font-mono); }
  .rule-list { display: grid; gap: var(--space-2); }
  .rule-row { padding: var(--space-3); display: grid; grid-template-columns: minmax(120px, .7fr) auto minmax(180px, 1.2fr) 92px 30px; align-items: end; gap: var(--space-2); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); }
  .rule-row label { min-width: 0; display: grid; gap: 5px; }
  .rule-row label > span { color: var(--color-muted); font-size: var(--text-2xs); font-weight: 600; }
  .rule-row input, .rule-row select { width: 100%; min-width: 0; height: 34px; padding: 0 var(--space-2); border: 1px solid var(--color-rule-strong); border-radius: var(--radius-sm); outline: 0; background: var(--color-paper-raised); color: var(--color-ink); font-size: var(--text-xs); }
  .rule-row input:focus, .rule-row select:focus { border-color: var(--color-focus); }
  .rule-row input { font-family: var(--font-mono); }
  .rule-arrow { height: 34px; display: grid; place-items: center; color: var(--color-faint); }
  .remove-rule { width: 30px; height: 34px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: var(--radius-sm); background: transparent; color: var(--color-muted); cursor: pointer; }
  .remove-rule:hover:not(:disabled) { border-color: var(--color-danger); color: var(--color-danger); }
  .remove-rule:disabled { opacity: .35; cursor: not-allowed; }
  .security-picker { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
  .security-picker label { position: relative; min-height: 92px; padding: var(--space-4); display: grid; grid-template-columns: 36px minmax(0, 1fr) 20px; align-items: center; gap: var(--space-3); border: 1px solid var(--color-rule); border-radius: var(--radius-md); background: var(--color-surface-subtle); cursor: pointer; transition: border-color var(--duration-fast), background var(--duration-fast); }
  .security-picker label.active { border-color: var(--color-accent); background: var(--color-accent-softer); box-shadow: inset 0 0 0 1px var(--color-accent); }
  .security-picker input { position: absolute; opacity: 0; pointer-events: none; }
  .security-icon { width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid var(--color-rule); border-radius: 50%; background: var(--color-paper-raised); color: var(--color-muted); }
  .security-picker label.active .security-icon { border-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-rule)); color: var(--color-accent); }
  .security-picker label > span:nth-of-type(2) { display: grid; gap: 3px; }
  .security-picker strong { font-size: var(--text-sm); }
  .security-picker small { color: var(--color-muted); font-size: var(--text-xs); line-height: 1.45; }
  .security-picker i { width: 18px; height: 18px; display: none; place-items: center; border-radius: 50%; background: var(--color-accent); color: var(--color-accent-ink); }
  .security-picker label.active i { display: grid; }
  .domain-modal footer { min-height: 66px; padding: var(--space-3) var(--space-5); display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); border-top: 1px solid var(--color-rule); background: var(--color-surface-subtle); }
  .domain-modal footer > span { color: var(--color-muted); font-size: var(--text-xs); }
  .domain-modal footer > div { margin-left: auto; display: flex; gap: var(--space-2); }

  @media (max-width: 70rem) {
    .domain-overview { grid-template-columns: repeat(3, 1fr); }
    .flow-note { grid-column: 1 / -1; justify-items: start; border-top: 1px solid var(--color-rule); }
    .domain-row { grid-template-columns: 38px minmax(160px, .8fr) minmax(220px, 1.2fr) auto; }
    .domain-row .edit-button { grid-column: 4; }
    .tls-badge { display: none; }
  }
  @media (max-width: 54rem) {
    .domain-row { padding: var(--space-3) 0; grid-template-columns: 38px minmax(0, 1fr) auto; }
    .route-stack { grid-column: 2 / -1; }
    .domain-row .edit-button { grid-column: 3; grid-row: 1; }
    .edge-guide { grid-template-columns: 1fr; }
    .advanced-body > header { flex-direction: column; }
    .editor-actions { justify-content: flex-start; }
  }
  @media (max-width: 42rem) {
    .page-actions .connection { display: none; }
    .platform-domain form { grid-template-columns: 1fr 1fr; }
    .platform-domain form label { grid-column: 1 / -1; }
    .platform-domain footer { align-items: flex-start; flex-direction: column; gap: 4px; padding-block: var(--space-3); }
    .platform-domain footer span:last-child { margin-left: 0; }
    .domain-overview { grid-template-columns: 1fr; }
    .domain-overview article { border-right: 0; border-bottom: 1px solid var(--color-rule); }
    .flow-note { grid-column: 1; }
    .domain-toolbar { align-items: stretch; flex-direction: column; }
    .search-field { width: 100%; }
    .identity-grid, .security-picker { grid-template-columns: 1fr; }
    .dns-card { grid-template-columns: 38px minmax(0, 1fr); }
    .dns-card-actions { grid-column: 2; display: flex; }
    .registry-destination { grid-template-columns: 34px minmax(0, 1fr); }
    .registry-destination em { grid-column: 2; width: max-content; }
    .registry-routes > div { grid-template-columns: 1fr auto; padding: var(--space-3); }
    .registry-routes > div > span { grid-column: 1 / -1; }
    .rule-row { grid-template-columns: 1fr 90px 30px; }
    .rule-row > label:first-child { grid-column: 1 / -1; }
    .rule-arrow { display: none; }
    .rule-row > label:nth-of-type(2) { grid-column: 1; }
    .route-heading { align-items: flex-start; }
    .domain-modal footer { align-items: stretch; flex-direction: column; }
    .domain-modal footer > div, .domain-modal footer > .btn-danger { width: 100%; }
    .domain-modal footer > div .btn { flex: 1; }
    .editor-state code { display: none; }
  }
  @media (max-width: 34rem) {
    .modal-backdrop { padding: 0; }
    .domain-modal { width: 100%; max-height: 100vh; height: 100vh; border: 0; border-radius: 0; }
    .modal-scroll { padding: var(--space-4); }
    .form-section { grid-template-columns: 1fr; }
    .section-number { margin-bottom: calc(-1 * var(--space-2)); }
    .domain-list { padding: 0 var(--space-4); }
    .route-stack > div { grid-template-columns: minmax(50px, auto) auto minmax(70px, auto) 1fr; }
  }
</style>
