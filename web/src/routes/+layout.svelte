<script>
  import { onMount } from 'svelte';
  import '$lib/tokens.css';
  import '$lib/base.css';
  import { loadSession } from '$lib/auth.js';

  let ready = false;

  onMount(async () => {
    const path = location.pathname;
    const publicAuthPaths = ['/login', '/forgot-password', '/reset-password'];
    const setupResponse = await fetch('/api/setup/status');
    if (!setupResponse.ok) { ready = true; return; }
    const { configured } = await setupResponse.json();
    if (!configured && path !== '/setup') { location.replace('/setup'); return; }
    if (configured && path === '/setup') { location.replace('/login'); return; }
    if (publicAuthPaths.includes(path)) { const user = await loadSession(); if (user && path === '/login') location.replace('/'); else ready = true; return; }
    if (path === '/setup') { ready = true; return; }
    const user = await loadSession();
    if (!user) { location.replace('/login'); return; }
    ready = true;
  });
</script>

{#if ready}
  <slot />
{:else}
  <div class="boot">
    <svg width="40" height="40" viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="var(--color-accent)" />
      <path d="M10.5 9.5h6a6.5 6.5 0 0 1 0 13h-6v-13Z" stroke="var(--color-accent-ink)" stroke-width="2.4" stroke-linejoin="round" />
      <circle cx="21.5" cy="21.5" r="2.1" fill="var(--color-accent-ink)" />
    </svg>
    <p>Starting Dokyr…</p>
  </div>
{/if}

<style>
  .boot {
    min-height: 100vh;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: var(--space-4);
    background: var(--color-paper);
  }
  .boot p {
    margin: 0;
    color: var(--color-muted);
    font: 500 var(--text-sm) var(--font-mono);
    letter-spacing: 0.02em;
  }
</style>
