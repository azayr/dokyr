<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
  import Toaster from './Toaster.svelte';
  import CommandMenu from './CommandMenu.svelte';
  import { currentUser, logout } from '$lib/auth.js';
  import { themeMode, resolvedTheme, initTheme, setTheme } from '$lib/theme.js';

  export let eyebrow = 'Workspace';
  export let title = 'Overview';
  export let subtitle = '';
  export let meta = [];

  const navGroups = [
    {
      label: 'Workspace',
      items: [
        { href: '/', icon: 'grid', label: 'Overview' },
        { href: '/projects', icon: 'box', label: 'Projects' },
        { href: '/deployments', icon: 'rocket', label: 'Deployments' }
      ]
    },
    {
      label: 'Infrastructure',
      items: [
        { href: '/servers', icon: 'server', label: 'Servers' },
        { href: '/proxy', icon: 'globe', label: 'Proxy' },
        { href: '/integrations', icon: 'git', label: 'Sources' }
      ]
    },
    {
      label: 'Administration',
      items: [{ href: '/settings', icon: 'settings', label: 'Settings' }]
    }
  ];

  let drawerOpen = false;
  let commandOpen = false;
  let userMenuOpen = false;
  let themeMenuOpen = false;

  onMount(() => {
    initTheme();
  });

  $: if (page.url.pathname) {
    drawerOpen = false;
    userMenuOpen = false;
    themeMenuOpen = false;
  }

  const initials = (name = 'Owner') => name.split(' ').map((value) => value[0]).slice(0, 2).join('').toUpperCase();
  const isActive = (href) => (href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href));

  const themeIcons = { light: 'sun', dark: 'moon', system: 'monitor' };

  function chooseTheme(mode) {
    setTheme(mode);
    themeMenuOpen = false;
  }

  function closeMenus() {
    userMenuOpen = false;
    themeMenuOpen = false;
  }
</script>

<svelte:head>
  <meta name="description" content="Dokyr — a calm, self-hosted deployment control plane for projects, servers, and proxies." />
</svelte:head>

<svelte:window onclick={(event) => {
  if (!event.target.closest?.('[data-menu]')) closeMenus();
}} />

<div class="app">
  <aside class="sidebar" class:open={drawerOpen} aria-label="Primary">
    <div class="sidebar-top">
      <a class="brand" href="/" aria-label="Dokyr overview">
        <Logo size={28} />
      </a>
      <button class="icon-btn drawer-close" type="button" aria-label="Close navigation" onclick={() => (drawerOpen = false)}>
        <Icon name="x" size={16} />
      </button>
    </div>

    <nav aria-label="Primary navigation">
      {#each navGroups as group}
        <div class="nav-group">
          <span class="nav-label">{group.label}</span>
          {#each group.items as item}
            <a href={item.href} class:active={isActive(item.href)} aria-current={isActive(item.href) ? 'page' : undefined}>
              <Icon name={item.icon} size={16} /><span>{item.label}</span>
            </a>
          {/each}
        </div>
      {/each}
    </nav>

    <div class="sidebar-bottom">
      <div class="user-menu-wrap" data-menu>
        {#if userMenuOpen}
          <div class="menu" role="menu">
            <div class="menu-head">
              <b>{$currentUser?.name || 'Owner'}</b>
              <small>{$currentUser?.email || ''}</small>
            </div>
            <a href="/settings" role="menuitem" onclick={() => (userMenuOpen = false)}><Icon name="settings" size={14} /> Settings</a>
            <button type="button" role="menuitem" onclick={logout}><Icon name="logout" size={14} /> Sign out</button>
          </div>
        {/if}
        <button class="identity" type="button" aria-haspopup="menu" aria-expanded={userMenuOpen} onclick={() => { userMenuOpen = !userMenuOpen; themeMenuOpen = false; }}>
          <span class="avatar">{initials($currentUser?.name)}</span>
          <span class="identity-text"><b>{$currentUser?.name || 'Owner'}</b><small>{$currentUser?.email || 'Account'}</small></span>
          <Icon name="chevron-down" size={14} />
        </button>
      </div>
    </div>
  </aside>
  {#if drawerOpen}<button class="drawer-scrim" aria-label="Close navigation" onclick={() => (drawerOpen = false)}></button>{/if}

  <div class="workspace-view">
    <header class="topbar">
      <button class="icon-btn menu-btn" type="button" aria-label="Open navigation" onclick={() => (drawerOpen = true)}>
        <Icon name="menu" size={17} />
      </button>
      <a class="mobile-brand" href="/" aria-label="Dokyr overview"><Logo size={24} compact /></a>
      <nav class="crumbs" aria-label="Breadcrumbs">
        <span>{eyebrow}</span>
        <Icon name="chevron-right" size={12} />
        <strong>{title}</strong>
      </nav>
      <div class="top-actions">
        <button class="command-trigger" type="button" onclick={() => (commandOpen = true)}>
          <Icon name="search" size={14} /><span>Search or command</span><kbd>⌘K</kbd>
        </button>
        <button class="icon-btn command-trigger-mobile" type="button" aria-label="Open command menu" onclick={() => (commandOpen = true)}>
          <Icon name="search" size={16} />
        </button>
        <div class="theme-wrap" data-menu>
          <button class="icon-btn" type="button" aria-label="Change theme" aria-haspopup="menu" aria-expanded={themeMenuOpen} data-tip="Theme" onclick={() => { themeMenuOpen = !themeMenuOpen; userMenuOpen = false; }}>
            <Icon name={themeIcons[$themeMode] || 'sun'} size={16} />
          </button>
          {#if themeMenuOpen}
            <div class="menu theme-menu" role="menu">
              {#each ['light', 'dark', 'system'] as mode}
                <button type="button" role="menuitemradio" aria-checked={$themeMode === mode} class:active={$themeMode === mode} onclick={() => chooseTheme(mode)}>
                  <Icon name={themeIcons[mode]} size={14} />
                  <span>{mode[0].toUpperCase() + mode.slice(1)}</span>
                  {#if $themeMode === mode}<Icon name="check" size={13} />{/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <a class="icon-btn" href="/settings" data-tip="Settings" aria-label="Open settings"><Icon name="settings" size={16} /></a>
      </div>
    </header>

    <nav class="mobile-nav" aria-label="Mobile navigation">
      {#each navGroups.flatMap((group) => group.items) as item}
        <a href={item.href} class:active={isActive(item.href)} aria-label={item.label}>
          <Icon name={item.icon} size={15} /><span>{item.label}</span>
        </a>
      {/each}
    </nav>

    <main>
      <header class="page-header">
        <div class="heading">
          <h1>{title}</h1>
          {#if subtitle}<p>{subtitle}</p>{/if}
          {#if meta.length}
            <div class="scope">{#each meta as item}<span>{item}</span>{/each}</div>
          {/if}
        </div>
        {#if $$slots.actions}
          <div class="page-actions"><slot name="actions" /></div>
        {/if}
      </header>
      <slot />
    </main>
  </div>
</div>

<CommandMenu bind:open={commandOpen} />
<Toaster />

<style>
  .app {
    min-height: 100vh;
    background: var(--color-paper);
    color: var(--color-ink);
  }

  /* ---------- Sidebar ---------- */
  .sidebar {
    position: fixed;
    z-index: 120;
    inset: 0 auto 0 0;
    width: 264px;
    padding: var(--space-4) var(--space-3);
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--color-rule);
    background: var(--color-sidebar);
    transform: translateX(-100%);
    transition: transform var(--duration-base) var(--ease-out);
  }
  .sidebar.open {
    transform: none;
    box-shadow: var(--shadow-modal);
  }
  .drawer-scrim {
    position: fixed;
    z-index: 110;
    inset: 0;
    border: 0;
    background: rgb(6 12 20 / 0.45);
    cursor: default;
  }
  .sidebar-top {
    height: 40px;
    padding: 0 var(--space-2);
    margin-bottom: var(--space-5);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .brand {
    color: var(--color-ink);
    text-decoration: none;
  }
  .drawer-close {
    display: grid;
  }
  .sidebar nav {
    flex: 1;
    display: grid;
    align-content: start;
    gap: var(--space-5);
    overflow-y: auto;
  }
  .nav-group {
    display: grid;
    gap: 2px;
  }
  .nav-label {
    padding: 0 var(--space-2) var(--space-1);
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .nav-group a {
    height: 34px;
    padding: 0 var(--space-2);
    display: flex;
    align-items: center;
    gap: 9px;
    border-radius: var(--radius-sm);
    color: var(--color-muted);
    font-size: var(--text-sm);
    font-weight: 500;
    text-decoration: none;
    transition: color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out);
  }
  .nav-group a:hover {
    background: var(--color-paper-subtle);
    color: var(--color-ink);
  }
  .nav-group a.active {
    background: var(--color-accent-soft);
    color: var(--color-accent);
    font-weight: 600;
  }

  .sidebar-bottom {
    padding-top: var(--space-3);
    border-top: 1px solid var(--color-rule);
  }
  .user-menu-wrap {
    position: relative;
  }
  .identity {
    width: 100%;
    min-height: 44px;
    padding: var(--space-1) var(--space-2);
    display: grid;
    grid-template-columns: 30px minmax(0, 1fr) auto;
    align-items: center;
    gap: 9px;
    border: 0;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--color-ink);
    text-align: left;
    cursor: pointer;
  }
  .identity:hover {
    background: var(--color-paper-subtle);
  }
  .avatar {
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--color-accent);
    color: var(--color-accent-ink);
    font: 600 var(--text-2xs) var(--font-mono);
  }
  .identity-text {
    min-width: 0;
    display: grid;
  }
  .identity-text b,
  .identity-text small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .identity-text b {
    font-size: var(--text-sm);
    font-weight: 600;
  }
  .identity-text small {
    color: var(--color-muted);
    font-size: var(--text-xs);
  }
  .identity > :global(svg) {
    color: var(--color-faint);
  }

  .menu {
    position: absolute;
    z-index: 60;
    min-width: 190px;
    padding: var(--space-1);
    display: grid;
    border: 1px solid var(--color-rule-strong);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-popover);
  }
  .menu a,
  .menu button {
    min-height: 34px;
    padding: 0 var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-ink);
    font-size: var(--text-sm);
    text-align: left;
    text-decoration: none;
    cursor: pointer;
  }
  .menu a:hover,
  .menu button:hover {
    background: var(--color-paper-subtle);
  }
  .menu button :global(svg:last-child) {
    margin-left: auto;
    color: var(--color-accent);
  }
  .menu .active {
    color: var(--color-accent);
    font-weight: 600;
  }
  .menu-head {
    padding: var(--space-2) var(--space-2) var(--space-3);
    display: grid;
    gap: 1px;
    border-bottom: 1px solid var(--color-rule);
    margin-bottom: var(--space-1);
  }
  .menu-head b {
    font-size: var(--text-sm);
  }
  .menu-head small {
    overflow: hidden;
    color: var(--color-muted);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .user-menu-wrap .menu {
    bottom: calc(100% + 8px);
    left: 0;
    right: 0;
  }
  .theme-wrap {
    position: relative;
  }
  .theme-menu {
    top: calc(100% + 8px);
    right: 0;
  }

  /* ---------- Topbar ---------- */
  .workspace-view {
    min-width: 0;
  }
  .topbar {
    position: sticky;
    top: 0;
    z-index: 90;
    height: 56px;
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border-bottom: 1px solid var(--color-rule);
    background: color-mix(in srgb, var(--color-paper) 82%, transparent);
    backdrop-filter: blur(10px);
  }
  .icon-btn {
    width: 32px;
    height: 32px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-ink-secondary);
    text-decoration: none;
    cursor: pointer;
  }
  .icon-btn:hover {
    background: var(--color-paper-subtle);
    color: var(--color-ink);
  }
  .crumbs {
    min-width: 0;
    display: none;
    align-items: center;
    gap: 7px;
    color: var(--color-muted);
    font-size: var(--text-sm);
    white-space: nowrap;
  }
  .crumbs strong {
    overflow: hidden;
    color: var(--color-ink);
    font-weight: 600;
    text-overflow: ellipsis;
  }
  .mobile-brand {
    color: var(--color-ink);
    text-decoration: none;
  }
  .top-actions {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .command-trigger {
    width: 230px;
    height: 32px;
    padding: 0 var(--space-2);
    display: none;
    align-items: center;
    gap: var(--space-2);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-muted);
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .command-trigger:hover {
    border-color: var(--color-rule-strong);
    background: var(--color-surface-subtle);
  }
  .command-trigger span {
    flex: 1;
    overflow: hidden;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .command-trigger kbd {
    padding: 1px 5px;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-xs);
    background: var(--color-paper-subtle);
    color: var(--color-faint);
    font: 500 var(--text-2xs) var(--font-mono);
  }
  .command-trigger-mobile {
    display: grid;
  }

  /* ---------- Mobile nav ---------- */
  .mobile-nav {
    padding: var(--space-2) var(--space-3);
    display: flex;
    gap: 2px;
    overflow-x: auto;
    border-bottom: 1px solid var(--color-rule);
    background: var(--color-paper);
    scrollbar-width: none;
  }
  .mobile-nav::-webkit-scrollbar {
    display: none;
  }
  .mobile-nav a {
    min-width: max-content;
    height: 34px;
    padding: 0 var(--space-3);
    display: flex;
    align-items: center;
    gap: 7px;
    border-radius: var(--radius-sm);
    color: var(--color-muted);
    font-size: var(--text-sm);
    font-weight: 500;
    text-decoration: none;
  }
  .mobile-nav a.active {
    background: var(--color-accent-soft);
    color: var(--color-accent);
    font-weight: 600;
  }

  /* ---------- Main ---------- */
  main {
    width: 100%;
    max-width: 1400px;
    margin: 0 auto;
    padding: 0 var(--space-4) var(--space-10);
  }
  .page-header {
    min-height: 96px;
    padding: var(--space-6) 0 var(--space-5);
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-5);
  }
  .heading {
    min-width: 0;
  }
  h1 {
    margin: 0;
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-2xl);
    font-weight: 700;
    line-height: 1.15;
    letter-spacing: -0.03em;
  }
  .heading > p {
    max-width: 720px;
    margin: var(--space-2) 0 0;
    color: var(--color-muted);
    font-size: var(--text-md);
    line-height: 1.5;
  }
  .scope {
    margin-top: var(--space-2);
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    color: var(--color-muted);
    font: 500 var(--text-xs) var(--font-mono);
  }
  .scope span + span::before {
    content: '·';
    margin-right: var(--space-3);
    color: var(--color-rule-strong);
  }
  .page-actions {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  @media (min-width: 46rem) {
    .topbar {
      padding-inline: var(--space-6);
    }
    .crumbs {
      display: flex;
    }
    .mobile-brand {
      display: none;
    }
    .command-trigger {
      display: flex;
    }
    .command-trigger-mobile {
      display: none;
    }
    main {
      padding-inline: var(--space-6);
    }
  }

  @media (min-width: 64rem) {
    .app {
      display: grid;
      grid-template-columns: 248px minmax(0, 1fr);
    }
    .sidebar {
      position: sticky;
      top: 0;
      height: 100vh;
      width: auto;
      transform: none;
      box-shadow: none;
      transition: none;
    }
    .drawer-close,
    .drawer-scrim,
    .menu-btn {
      display: none;
    }
    .mobile-nav {
      display: none;
    }
  }

  @media (max-width: 34rem) {
    .page-header {
      min-height: auto;
      padding-block: var(--space-5) var(--space-4);
      flex-direction: column;
    }
    .page-actions {
      width: 100%;
    }
    .page-actions > :global(*) {
      flex: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .sidebar {
      transition: none;
    }
    .nav-group a {
      transition: none;
    }
  }
</style>
