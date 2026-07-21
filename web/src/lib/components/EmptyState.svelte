<script>
  import Icon from './Icon.svelte';
  export let icon = 'box';
  export let title = 'Nothing here yet';
  export let description = '';
</script>

<div class="empty-state">
  <span class="empty-icon"><Icon name={icon} size={20} /></span>
  <h3>{title}</h3>
  {#if description}<p>{description}</p>{/if}
  <slot />
</div>
