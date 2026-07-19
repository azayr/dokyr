<script>
  import { onMount } from 'svelte';
  import '$lib/tokens.css';
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

{#if ready}<slot/>{:else}<div class="boot"><span>S/</span><p>Establishing control plane…</p></div>{/if}

<style>.boot{min-height:100vh;background:var(--color-paper);color:var(--color-muted);display:grid;place-content:center;text-align:center;font:13px var(--font-mono);letter-spacing:.02em}.boot span{margin:auto;width:40px;height:40px;border-radius:var(--radius-md);display:grid;place-items:center;background:var(--color-accent);color:var(--color-accent-ink);font-weight:700}.boot p{margin-top:var(--space-4)}</style>
