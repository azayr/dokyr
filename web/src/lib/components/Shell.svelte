<script>
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import { page } from '$app/state';
  import { currentUser, logout } from '$lib/auth.js';

  export let eyebrow = 'Workspace';
  export let title = 'Overview';
  export let subtitle = '';
  export let meta = [];

  let dark = false;
  const nav = [
    ['/', 'grid', 'Overview'],
    ['/projects', 'box', 'Projects'],
    ['/deployments', 'rocket', 'Deployments'],
    ['/integrations', 'git', 'Sources'],
    ['/proxy', 'globe', 'Proxy'],
    ['/servers', 'server', 'Infrastructure']
  ];

  onMount(() => {
    const syncStoredTheme = () => applyTheme(localStorage.getItem('selfhost-theme') === 'dark');
    syncStoredTheme();
    window.addEventListener('storage', syncStoredTheme);
    return () => window.removeEventListener('storage', syncStoredTheme);
  });

  const initials = (name = 'Owner') => name.split(' ').map((value) => value[0]).slice(0, 2).join('').toUpperCase();
  const isActive = (href) => href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);

  function applyTheme(value) {
    dark = value;
    document.documentElement.classList.toggle('theme-dark', value);
    const themeColor = document.querySelector('meta[name="theme-color"]');
    if (themeColor) themeColor.setAttribute('content', value ? '#10151b' : '#f7f3ed');
  }

  function toggleTheme() {
    const next = !dark;
    localStorage.setItem('selfhost-theme', next ? 'dark' : 'light');
    applyTheme(next);
  }
</script>

<svelte:head><meta name="description" content="A lightweight self-hosted deployment control plane" /></svelte:head>

<div class:dark class="app">
  <aside>
    <a class="brand" href="/" aria-label="DeployForge overview">
      <span class="mark"><span></span><span></span></span>
      <strong>DeployForge</strong>
    </a>

    <nav aria-label="Primary navigation">
      {#each nav as item}
        <a href={item[0]} class:active={isActive(item[0])} aria-current={isActive(item[0]) ? 'page' : undefined}>
          <Icon name={item[1]} size={16} /><span>{item[2]}</span>
        </a>
      {/each}
    </nav>

    <div class="bottom">
      <a href="/settings" class:active={page.url.pathname.startsWith('/settings')}>
        <Icon name="settings" size={16} /><span>Settings</span>
      </a>
      <a href="/settings" class="docs"><span aria-hidden="true">▤</span><span>Documentation</span></a>
      <button class="identity" onclick={logout} title="Sign out">
        <span>{initials($currentUser?.name)}</span>
        <div><b>{$currentUser?.name || 'Owner'}</b><small>{$currentUser?.email || 'Sign out'}</small></div>
        <em aria-hidden="true">···</em>
      </button>
    </div>
  </aside>

  <div class="workspace-view">
    <header class="topbar">
      <a class="mobile-brand" href="/" aria-label="DeployForge overview"><span class="mark"><span></span><span></span></span><strong>DeployForge</strong></a>
      <p><span>{eyebrow}</span><b>/</b><strong>{title}</strong></p>
      <div class="top-actions">
        <button class="search" type="button"><Icon name="search" size={14}/><span>Search or run command</span><kbd>⌘K</kbd></button>
        <button class="icon-button" onclick={toggleTheme} aria-label="Toggle color theme"><Icon name={dark ? 'sun' : 'moon'} size={16}/></button>
        <button class="avatar" type="button" title={$currentUser?.name || 'Owner'}>{initials($currentUser?.name)}</button>
      </div>
    </header>

    <nav class="mobile-nav" aria-label="Mobile navigation">
      {#each nav as item}
        <a href={item[0]} class:active={isActive(item[0])} aria-label={item[2]}>
          <Icon name={item[1]} size={17} /><span>{item[2]}</span>
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
      </header>
      <slot />
    </main>
  </div>
</div>

<style>
  :global(*) { box-sizing: border-box; }
  :global(html) { overflow-x: clip; background: var(--color-paper); }
  :global(body) { margin: 0; min-width: 320px; overflow-x: clip; font-family: var(--font-body); color: var(--color-ink); background: var(--color-paper); }
  :global(button), :global(input), :global(select), :global(textarea) { font: inherit; }
  :global(button), :global(a) { -webkit-tap-highlight-color: transparent; }
  :global(:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; }

  .app { min-height: 100vh; background: var(--color-paper); color: var(--color-ink); }
  aside { display: none; }
  .workspace-view { min-width: 0; }
  .brand, .mobile-brand { display: flex; align-items: center; gap: 9px; color: var(--color-ink); text-decoration: none; }
  .brand strong, .mobile-brand strong { font-size: 14px; letter-spacing: -0.025em; }
  .mark { position: relative; width: 28px; height: 28px; flex: 0 0 auto; border-radius: 7px; display: grid; place-items: center; overflow: hidden; background: var(--color-log-bg); }
  .mark span:first-child { position: absolute; width: 11px; height: 11px; border: 2px solid var(--color-accent); border-radius: 3px; transform: translate(-3px, -3px); }
  .mark span:last-child { position: absolute; width: 6px; height: 6px; border-radius: 2px; background: var(--color-accent); transform: translate(4px, 4px); }

  .topbar { height: 64px; padding: 0 16px; display: flex; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .topbar > p { display: none; min-width: 0; margin: 0; align-items: center; gap: 7px; color: var(--color-muted); font-size: 11px; white-space: nowrap; }
  .topbar > p span, .topbar > p strong { overflow: hidden; text-overflow: ellipsis; }
  .topbar > p b { color: var(--color-faint); font-weight: 400; }
  .topbar > p strong { color: var(--color-ink-secondary); font-weight: 500; }
  .mobile-brand { min-width: 0; }
  .top-actions { display: flex; align-items: center; gap: 7px; }
  .search { display: none; width: 260px; height: 34px; padding: 0 9px; align-items: center; gap: 8px; border: 1px solid var(--color-rule); border-radius: 7px; background: var(--color-surface-subtle); color: var(--color-muted); text-align: left; font-size: 10px; cursor: pointer; }
  .search span:nth-child(2) { flex: 1; }
  .search kbd { padding: 2px 5px; border: 1px solid var(--color-rule); border-radius: 4px; background: var(--color-paper-raised); color: var(--color-faint); font: 9px var(--font-mono); }
  .icon-button, .avatar { width: 32px; height: 32px; border: 1px solid var(--color-rule); border-radius: 7px; display: grid; place-items: center; background: var(--color-paper-raised); color: var(--color-ink-secondary); cursor: pointer; }
  .icon-button:hover { background: var(--color-paper-subtle); }
  .avatar { border-color: var(--color-log-bg); border-radius: 50%; background: var(--color-log-bg); color: white; font: 600 9px var(--font-mono); }

  .mobile-nav { padding: 7px 12px; display: flex; gap: 3px; overflow-x: auto; border-bottom: 1px solid var(--color-rule); background: var(--color-paper-raised); }
  .mobile-nav a { min-width: max-content; height: 36px; padding: 0 10px; display: flex; align-items: center; gap: 7px; border-radius: 7px; color: var(--color-muted); text-decoration: none; font-size: 11px; font-weight: 500; }
  .mobile-nav a.active { background: var(--color-accent-soft); color: var(--color-accent); }

  main { width: 100%; max-width: 1440px; margin: 0 auto; padding: 0 16px 40px; }
  .page-header { min-height: 112px; padding: 23px 0 18px; display: flex; align-items: flex-start; gap: 18px; }
  .heading { min-width: 0; }
  h1 { min-width: 0; margin: 0; overflow-wrap: anywhere; font-family: var(--font-display); font-size: clamp(24px, 4vw, 29px); line-height: 1.08; letter-spacing: -0.04em; }
  .heading > p { max-width: 720px; margin: 8px 0 0; color: var(--color-ink-secondary); font-size: 12px; line-height: 1.45; }
  .scope { margin-top: 9px; display: flex; flex-wrap: wrap; gap: 12px; color: var(--color-muted); font: 9px var(--font-mono); }
  .scope span + span::before { content: '·'; margin-right: 12px; color: var(--color-rule-strong); }
  @media (min-width: 46rem) {
    .topbar { padding-inline: 24px; }
    .topbar > p { display: flex; }
    .mobile-brand { display: none; }
    .search { display: flex; }
    main { padding-inline: 24px; }
  }

  @media (min-width: 64rem) {
    .app { display: grid; grid-template-columns: 232px minmax(0, 1fr); }
    aside { position: sticky; top: 0; height: 100vh; padding: 18px 16px 16px; display: flex; flex-direction: column; border-right: 1px solid var(--color-rule); background: var(--color-paper-raised); }
    aside .brand { padding: 0 6px; margin-bottom: 17px; }
    aside nav, .bottom { display: grid; gap: 3px; }
    aside nav a, .bottom > a { height: 34px; padding: 0 9px; display: flex; align-items: center; gap: 9px; border-radius: 7px; color: var(--color-muted); text-decoration: none; font-size: 11px; font-weight: 500; transition: color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out); }
    aside nav a:hover, .bottom > a:hover { background: var(--color-paper-subtle); color: var(--color-ink); }
    aside nav a.active, .bottom > a.active { background: var(--color-accent-soft); color: var(--color-accent); }
    .bottom { margin-top: auto; }
    .docs > span:first-child { width: 16px; text-align: center; font: 12px var(--font-mono); }
    .identity { width: 100%; margin-top: 9px; padding: 10px 7px 0; display: grid; grid-template-columns: 29px minmax(0, 1fr) auto; gap: 8px; align-items: center; border: 0; border-top: 1px solid var(--color-rule); background: transparent; color: var(--color-ink); text-align: left; cursor: pointer; }
    .identity > span { width: 29px; height: 29px; border-radius: 50%; display: grid; place-items: center; background: var(--color-log-bg); color: white; font: 600 8px var(--font-mono); }
    .identity div { min-width: 0; display: grid; gap: 1px; }
    .identity b { overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
    .identity small { overflow: hidden; color: var(--color-muted); font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
    .identity em { color: var(--color-faint); font-style: normal; }
    .mobile-nav { display: none; }
  }

  @media (max-width: 34rem) {
    .page-header { min-height: auto; padding-block: 20px; }
  }

  @media (prefers-reduced-motion: reduce) { aside nav a, .bottom > a { transition: none; } }
</style>
