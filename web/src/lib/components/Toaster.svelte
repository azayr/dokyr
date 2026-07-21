<script>
  import { flip } from 'svelte/animate';
  import { fade, fly } from 'svelte/transition';
  import { toasts, dismissToast } from '$lib/toast.js';
  import Icon from './Icon.svelte';

  const icons = { success: 'check-circle', error: 'x-circle', info: 'info' };
</script>

<div class="toaster" aria-live="polite" aria-label="Notifications">
  {#each $toasts as item (item.id)}
    <div class="toast toast-{item.tone}" animate:flip={{ duration: 160 }} in:fly={{ y: 10, duration: 180 }} out:fade={{ duration: 120 }} role="status">
      <Icon name={icons[item.tone] || 'info'} size={16} />
      <span>{item.message}</span>
      <button type="button" aria-label="Dismiss notification" onclick={() => dismissToast(item.id)}>×</button>
    </div>
  {/each}
</div>

<style>
  .toaster {
    position: fixed;
    z-index: 200;
    right: 16px;
    bottom: 16px;
    display: grid;
    gap: 8px;
    width: min(360px, calc(100vw - 32px));
    pointer-events: none;
  }
  .toast {
    padding: 10px 12px;
    display: flex;
    align-items: flex-start;
    gap: 9px;
    border: 1px solid var(--color-rule-strong);
    border-radius: var(--radius-md);
    background: var(--color-paper-raised);
    color: var(--color-ink);
    box-shadow: var(--shadow-popover);
    font-size: var(--text-sm);
    line-height: 1.45;
    pointer-events: auto;
  }
  .toast span {
    flex: 1;
    min-width: 0;
  }
  .toast button {
    border: 0;
    background: transparent;
    color: var(--color-muted);
    font-size: 15px;
    line-height: 1.2;
    cursor: pointer;
  }
  .toast button:hover {
    color: var(--color-ink);
  }
  .toast-success > :global(svg) {
    color: var(--color-success);
  }
  .toast-error > :global(svg) {
    color: var(--color-danger);
  }
  .toast-info > :global(svg) {
    color: var(--color-info);
  }
</style>
