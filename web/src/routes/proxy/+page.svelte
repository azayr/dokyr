<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  onMount(() => goto('/domains', { replaceState: true }));
</script>

<svelte:head>
  <title>Domains · Dokyr</title>
</svelte:head>

<p class="redirect">Opening Domains…</p>

<style>
  .redirect {
    padding: var(--space-8);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }
</style>
