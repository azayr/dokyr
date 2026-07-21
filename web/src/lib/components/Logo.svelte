<script>
  export let size = 28;
  export let wordmark = true;
  export let compact = false;
</script>

<span class="logo" class:compact>
  <svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true">
    <rect width="32" height="32" rx="8" fill="var(--color-accent)" />
    <path d="M10.5 9.5h6a6.5 6.5 0 0 1 0 13h-6v-13Z" stroke="var(--color-accent-ink)" stroke-width="2.4" stroke-linejoin="round" />
    <circle cx="21.5" cy="21.5" r="2.1" fill="var(--color-accent-ink)" />
  </svg>
  {#if wordmark}<strong>Dokyr</strong>{/if}
</span>

<style>
  .logo {
    display: inline-flex;
    align-items: center;
    gap: 9px;
    color: var(--color-ink);
  }
  .logo svg {
    flex: 0 0 auto;
    display: block;
  }
  .logo strong {
    font-size: 15px;
    font-weight: 700;
    letter-spacing: -0.02em;
  }
  .compact strong {
    display: none;
  }
</style>
