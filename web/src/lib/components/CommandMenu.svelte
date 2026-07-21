<script>
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { setTheme } from '$lib/theme.js';
  import { logout } from '$lib/auth.js';
  import Icon from './Icon.svelte';

  export let open = false;
  export let onClose = () => {};

  let query = '';
  let selected = 0;
  let inputEl;
  let listEl;

  const commands = [
    { group: 'Navigate', icon: 'grid', label: 'Overview', hint: '/', run: () => goto('/') },
    { group: 'Navigate', icon: 'box', label: 'Projects', hint: '/projects', run: () => goto('/projects') },
    { group: 'Navigate', icon: 'rocket', label: 'Deployments', hint: '/deployments', run: () => goto('/deployments') },
    { group: 'Navigate', icon: 'server', label: 'Servers', hint: '/servers', run: () => goto('/servers') },
    { group: 'Navigate', icon: 'globe', label: 'Proxy', hint: '/proxy', run: () => goto('/proxy') },
    { group: 'Navigate', icon: 'git', label: 'Sources', hint: '/integrations', run: () => goto('/integrations') },
    { group: 'Navigate', icon: 'settings', label: 'Settings', hint: '/settings', run: () => goto('/settings') },
    { group: 'Actions', icon: 'plus', label: 'New project', hint: '', run: () => goto('/projects?new=1') },
    { group: 'Actions', icon: 'sun', label: 'Use light theme', hint: '', run: () => setTheme('light') },
    { group: 'Actions', icon: 'moon', label: 'Use dark theme', hint: '', run: () => setTheme('dark') },
    { group: 'Actions', icon: 'monitor', label: 'Use system theme', hint: '', run: () => setTheme('system') },
    { group: 'Actions', icon: 'logout', label: 'Sign out', hint: '', run: () => logout() }
  ];

  $: filtered = commands.filter((command) => (command.group + ' ' + command.label).toLowerCase().includes(query.trim().toLowerCase()));
  $: if (open) {
    query = '';
    selected = 0;
    tick().then(() => inputEl?.focus());
  }
  $: if (selected >= filtered.length) selected = Math.max(0, filtered.length - 1);

  function handleKeydown(event) {
    if (!open) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        open = true;
      }
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      selected = Math.min(selected + 1, filtered.length - 1);
      scrollSelectedIntoView();
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      selected = Math.max(selected - 1, 0);
      scrollSelectedIntoView();
    } else if (event.key === 'Enter' && filtered[selected]) {
      event.preventDefault();
      run(filtered[selected]);
    }
  }

  function scrollSelectedIntoView() {
    tick().then(() => listEl?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }));
  }

  function run(command) {
    close();
    command.run();
  }

  function close() {
    open = false;
    onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
  <div class="command-backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) close(); }}>
    <div class="command-menu" role="dialog" aria-modal="true" aria-label="Command menu">
      <div class="command-input">
        <Icon name="search" size={15} />
        <input
          bind:this={inputEl}
          bind:value={query}
          placeholder="Search pages and commands…"
          role="combobox"
          aria-expanded="true"
          aria-controls="command-results"
          aria-autocomplete="list"
          spellcheck="false"
        />
        <kbd>esc</kbd>
      </div>
      <div class="command-results" id="command-results" role="listbox" bind:this={listEl}>
        {#if filtered.length === 0}
          <p class="command-empty">No commands match “{query}”.</p>
        {:else}
          {#each filtered as command, index}
            {#if index === 0 || filtered[index - 1].group !== command.group}
              <div class="command-group" aria-hidden="true">{command.group}</div>
            {/if}
            <button
              type="button"
              role="option"
              aria-selected={index === selected}
              onmouseenter={() => (selected = index)}
              onclick={() => run(command)}
            >
              <span class="command-icon"><Icon name={command.icon} size={14} /></span>
              <span class="command-label">{command.label}</span>
              {#if command.hint}<kbd>{command.hint}</kbd>{/if}
            </button>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .command-backdrop {
    position: fixed;
    z-index: 150;
    inset: 0;
    padding: 12vh var(--space-4) var(--space-4);
    display: grid;
    justify-items: center;
    align-content: start;
    background: rgb(6 12 20 / 0.45);
    backdrop-filter: blur(3px);
  }
  .command-menu {
    width: min(520px, 100%);
    overflow: hidden;
    border: 1px solid var(--color-rule-strong);
    border-radius: var(--radius-lg);
    background: var(--color-paper-raised);
    box-shadow: var(--shadow-modal);
  }
  .command-input {
    height: 48px;
    padding: 0 var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    border-bottom: 1px solid var(--color-rule);
    color: var(--color-muted);
  }
  .command-input input {
    min-width: 0;
    flex: 1;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--color-ink);
    font-size: var(--text-md);
  }
  .command-input input::placeholder {
    color: var(--color-faint);
  }
  .command-input kbd {
    padding: 2px 5px;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-xs);
    background: var(--color-paper-subtle);
    color: var(--color-muted);
    font: 500 var(--text-2xs) var(--font-mono);
  }
  .command-results {
    max-height: 320px;
    overflow-y: auto;
    padding: var(--space-2);
  }
  .command-group {
    padding: var(--space-2) var(--space-2) var(--space-1);
    color: var(--color-faint);
    font-size: var(--text-2xs);
    font-weight: 600;
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }
  .command-results button {
    width: 100%;
    min-height: 38px;
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
    cursor: pointer;
  }
  .command-results button[aria-selected='true'] {
    background: var(--color-accent-soft);
    color: var(--color-accent);
  }
  .command-icon {
    width: 26px;
    height: 26px;
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    border: 1px solid var(--color-rule);
    border-radius: var(--radius-sm);
    background: var(--color-paper-raised);
    color: var(--color-muted);
  }
  .command-results button[aria-selected='true'] .command-icon {
    border-color: color-mix(in srgb, var(--color-accent) 30%, var(--color-rule));
    color: var(--color-accent);
  }
  .command-label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .command-results kbd {
    color: var(--color-faint);
    font: 500 var(--text-2xs) var(--font-mono);
  }
  .command-empty {
    padding: var(--space-6);
    margin: 0;
    color: var(--color-muted);
    font-size: var(--text-sm);
    text-align: center;
  }
</style>
