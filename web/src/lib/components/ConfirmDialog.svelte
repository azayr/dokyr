<script>
  import Icon from './Icon.svelte';

  export let title = 'Are you sure?';
  export let message = '';
  export let confirmLabel = 'Confirm';
  export let busy = false;
  export let error = '';
  export let requireText = '';
  export let onConfirm = () => {};
  export let onClose = () => {};

  let confirmation = '';
  $: confirmed = !requireText || confirmation === requireText;

  function handleKeydown(event) {
    if (event.key === 'Escape' && !busy) onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="modal-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
  <div class="modal confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-dialog-title">
    <header>
      <div>
        <span class="eyebrow danger-eyebrow">Destructive action</span>
        <h2 id="confirm-dialog-title">{title}</h2>
      </div>
      <button type="button" aria-label="Close dialog" onclick={onClose} disabled={busy}>×</button>
    </header>
    <div class="confirm-body">
      <div class="confirm-warning">
        <Icon name="alert" size={16} />
        <p>{message}</p>
      </div>
      {#if error}<div class="alert alert-error"><Icon name="x-circle" size={15} /><div><strong>Action failed</strong><span>{error}</span></div></div>{/if}
      {#if requireText}
        <label class="field">
          <span>Type <code class="confirm-code">{requireText}</code> to confirm</span>
          <input class="input input-mono" bind:value={confirmation} autocomplete="off" spellcheck="false" placeholder={requireText} />
        </label>
      {/if}
    </div>
    <footer>
      <button type="button" class="btn" onclick={onClose} disabled={busy}>Cancel</button>
      <button type="button" class="btn btn-danger-solid" onclick={() => onConfirm(confirmation)} disabled={busy || !confirmed}>
        {busy ? 'Working…' : confirmLabel}
      </button>
    </footer>
  </div>
</div>

<style>
  .confirm-dialog {
    width: min(460px, 100%);
  }
  .danger-eyebrow {
    color: var(--color-danger);
  }
  .confirm-body {
    padding: var(--space-5);
    display: grid;
    gap: var(--space-4);
  }
  .confirm-body .alert {
    margin-bottom: 0;
  }
  .confirm-warning {
    padding: var(--space-3) var(--space-4);
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-rule));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--color-danger) 6%, var(--color-paper-raised));
    color: var(--color-danger);
  }
  .confirm-warning p {
    margin: 0;
    color: var(--color-ink-secondary);
    font-size: var(--text-sm);
    line-height: 1.55;
  }
  .confirm-code {
    padding: 1px 5px;
    border-radius: var(--radius-xs);
    background: var(--color-paper-subtle);
    color: var(--color-danger);
    font-family: var(--font-mono);
  }
</style>
