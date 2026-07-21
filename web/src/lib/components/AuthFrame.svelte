<script>
  import Logo from './Logo.svelte';
  import Icon from './Icon.svelte';
  import Toaster from './Toaster.svelte';

  export let step = 'First run';
  export let title = 'Create your control plane';
  export let description = 'One owner account. Everything else can wait.';

  const highlights = [
    { icon: 'box', title: 'Projects & services', text: 'Compose applications from containers, repositories, and databases.' },
    { icon: 'rocket', title: 'Zero-downtime deploys', text: 'Every release is built, verified, and promoted behind Caddy.' },
    { icon: 'shield', title: 'Private by default', text: 'Your workloads, credentials, and logs stay on this server.' }
  ];
</script>

<svelte:head><title>{title} — Dokyr</title></svelte:head>

<main class="auth">
  <aside class="auth-side">
    <a class="auth-brand" href="/" aria-label="Dokyr home"><Logo size={30} /></a>
    <div class="auth-side-copy">
      <span class="auth-step">{step}</span>
      <h1>{title}</h1>
      <p>{description}</p>
    </div>
    <ul class="auth-highlights">
      {#each highlights as item}
        <li>
          <span class="auth-highlight-icon"><Icon name={item.icon} size={15} /></span>
          <span><b>{item.title}</b><small>{item.text}</small></span>
        </li>
      {/each}
    </ul>
    <footer class="auth-side-footer">Self-hosted · Your server · Your data</footer>
  </aside>

  <section class="auth-main">
    <div class="auth-mobile-brand"><Logo size={28} /></div>
    <div class="auth-card">
      <slot />
    </div>
    <p class="auth-footnote">Dokyr control plane</p>
  </section>
</main>

<Toaster />

<style>
  .auth {
    min-height: 100vh;
    display: grid;
    background: var(--color-paper);
  }
  .auth-side {
    display: none;
  }
  .auth-main {
    padding: var(--space-6) var(--space-4) var(--space-10);
    display: grid;
    align-content: center;
    justify-items: center;
    gap: var(--space-4);
  }
  .auth-mobile-brand {
    margin-bottom: var(--space-2);
  }
  .auth-card {
    width: min(400px, 100%);
    padding: var(--space-6);
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-panel);
  }
  .auth-footnote {
    margin: 0;
    color: var(--color-faint);
    font: 500 var(--text-xs) var(--font-mono);
  }

  @media (min-width: 60rem) {
    .auth {
      grid-template-columns: minmax(380px, 44%) 1fr;
    }
    .auth-side {
      padding: var(--space-10) var(--space-12);
      display: flex;
      flex-direction: column;
      border-right: 1px solid var(--color-rule);
      background: var(--color-paper-raised);
    }
    .auth-brand {
      color: var(--color-ink);
      text-decoration: none;
    }
    .auth-side-copy {
      margin: auto 0 var(--space-8);
      max-width: 420px;
    }
    .auth-step {
      color: var(--color-accent);
      font-size: var(--text-2xs);
      font-weight: 700;
      letter-spacing: 0.1em;
      text-transform: uppercase;
    }
    .auth-side-copy h1 {
      margin: var(--space-3) 0 var(--space-3);
      font-size: 30px;
      font-weight: 700;
      line-height: 1.12;
      letter-spacing: -0.035em;
    }
    .auth-side-copy p {
      margin: 0;
      color: var(--color-muted);
      font-size: var(--text-md);
      line-height: 1.6;
    }
    .auth-highlights {
      margin: 0;
      padding: var(--space-5) 0 0;
      display: grid;
      gap: var(--space-4);
      border-top: 1px solid var(--color-rule);
      list-style: none;
    }
    .auth-highlights li {
      display: grid;
      grid-template-columns: 32px minmax(0, 1fr);
      align-items: start;
      gap: var(--space-3);
    }
    .auth-highlight-icon {
      width: 32px;
      height: 32px;
      display: grid;
      place-items: center;
      border: 1px solid var(--color-rule);
      border-radius: var(--radius-sm);
      background: var(--color-accent-soft);
      color: var(--color-accent);
    }
    .auth-highlights b {
      display: block;
      font-size: var(--text-sm);
    }
    .auth-highlights small {
      display: block;
      margin-top: 2px;
      color: var(--color-muted);
      font-size: var(--text-xs);
      line-height: 1.5;
    }
    .auth-side-footer {
      margin-top: var(--space-8);
      color: var(--color-faint);
      font: 500 var(--text-2xs) var(--font-mono);
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    .auth-mobile-brand {
      display: none;
    }
  }

  /* ---------- Shared auth form styles (used by login/setup/reset pages) ---------- */
  :global(.auth-eyebrow) {
    margin: 0;
    color: var(--color-accent);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }
  :global(.auth-title) {
    margin: var(--space-1) 0 var(--space-1);
    font-size: var(--text-xl);
    font-weight: 700;
    letter-spacing: -0.025em;
  }
  :global(.auth-lead) {
    margin: 0 0 var(--space-5);
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.55;
  }
  :global(.auth-badge) {
    margin-bottom: var(--space-4);
    display: inline-flex;
    align-items: center;
    gap: 7px;
    color: var(--color-success);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }
  :global(.auth-badge i) {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color-success);
    box-shadow: 0 0 0 4px var(--color-success-soft);
  }
  :global(.auth-error) {
    margin-bottom: var(--space-4);
    padding: var(--space-3);
    border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-rule));
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-danger) 7%, var(--color-paper-raised));
    color: var(--color-danger);
    font-size: var(--text-sm);
    line-height: 1.45;
  }
  :global(.auth-form) {
    display: grid;
    gap: var(--space-4);
  }
  :global(.auth-field) {
    display: grid;
    gap: var(--space-2);
    color: var(--color-ink-secondary);
    font-size: var(--text-xs);
    font-weight: 600;
  }
  :global(.auth-field-row) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  :global(.auth-field-row a) {
    color: var(--color-accent);
    font-size: var(--text-xs);
    font-weight: 600;
    text-decoration: none;
  }
  :global(.auth-field-row a:hover) {
    text-decoration: underline;
  }
  :global(.auth-input) {
    height: 38px;
    padding: 0 var(--space-3);
    border: 1px solid var(--color-rule-strong);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-ink);
    font-size: var(--text-sm);
    outline: none;
    transition: border-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
  }
  :global(.auth-input::placeholder) {
    color: var(--color-faint);
  }
  :global(.auth-input:focus) {
    border-color: var(--color-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 16%, transparent);
  }
  :global(.auth-password-wrap) {
    position: relative;
    display: grid;
  }
  :global(.auth-password-wrap .auth-input) {
    width: 100%;
    padding-right: 40px;
  }
  :global(.auth-password-toggle) {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 30px;
    height: 30px;
    display: grid;
    place-items: center;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--color-muted);
    cursor: pointer;
  }
  :global(.auth-password-toggle:hover) {
    background: var(--color-paper-subtle);
    color: var(--color-ink);
  }
  :global(.auth-code-input) {
    height: 52px;
    padding-left: calc(var(--space-3) + 0.3em);
    text-align: center;
    font: 500 20px var(--font-mono);
    letter-spacing: 0.3em;
  }
  :global(.auth-submit) {
    min-height: 40px;
    margin-top: var(--space-1);
    padding: 0 var(--space-4);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    background: var(--color-accent);
    color: var(--color-accent-ink);
    font-size: var(--text-sm);
    font-weight: 600;
    text-decoration: none;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out);
  }
  :global(.auth-submit:hover:not(:disabled)) {
    border-color: var(--color-accent-hover);
    background: var(--color-accent-hover);
  }
  :global(.auth-submit:disabled) {
    opacity: 0.55;
    cursor: not-allowed;
  }
  :global(.auth-divider) {
    margin: var(--space-5) 0;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  :global(.auth-divider::before),
  :global(.auth-divider::after) {
    content: '';
    height: 1px;
    flex: 1;
    background: var(--color-rule);
  }
  :global(.auth-alt-button) {
    width: 100%;
    min-height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    border: 1px solid var(--color-rule-strong);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-ink);
    font-size: var(--text-sm);
    font-weight: 600;
    text-decoration: none;
    cursor: pointer;
  }
  :global(.auth-alt-button:hover:not(:disabled)) {
    background: var(--color-paper-subtle);
  }
  :global(.auth-alt-button:disabled) {
    color: var(--color-faint);
    cursor: not-allowed;
  }
  :global(.auth-provider-help) {
    margin: var(--space-2) 0 0;
    color: var(--color-muted);
    font-size: var(--text-xs);
    line-height: 1.5;
    text-align: center;
  }
  :global(.auth-back) {
    margin: var(--space-5) auto 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    width: max-content;
    border: 0;
    background: transparent;
    color: var(--color-muted);
    font-size: var(--text-sm);
    text-decoration: none;
    cursor: pointer;
  }
  :global(.auth-back:hover) {
    color: var(--color-accent);
  }
  :global(.auth-note) {
    margin: var(--space-4) 0 0;
    color: var(--color-faint);
    font-size: var(--text-xs);
    line-height: 1.5;
    text-align: center;
  }
  :global(.auth-success) {
    margin-bottom: var(--space-5);
    padding: var(--space-4);
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    border: 1px solid color-mix(in srgb, var(--color-success) 35%, var(--color-rule));
    border-radius: var(--radius-md);
    background: var(--color-success-soft);
  }
  :global(.auth-success > svg) {
    flex: 0 0 auto;
    margin-top: 2px;
    color: var(--color-success);
  }
  :global(.auth-success h2) {
    margin: 0 0 var(--space-1);
    font-size: var(--text-md);
  }
  :global(.auth-success p) {
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    line-height: 1.55;
  }
  :global(.auth-success strong) {
    color: var(--color-ink);
  }
  :global(.auth-tag) {
    margin-bottom: var(--space-4);
    padding: 5px 9px;
    display: inline-flex;
    border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--color-rule));
    border-radius: var(--radius-xs);
    background: var(--color-accent-soft);
    color: var(--color-accent);
    font-size: var(--text-2xs);
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  :global(.auth-challenge-icon) {
    width: 44px;
    height: 44px;
    margin-bottom: var(--space-4);
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-md);
    background: var(--color-accent-soft);
    color: var(--color-accent);
  }
</style>
