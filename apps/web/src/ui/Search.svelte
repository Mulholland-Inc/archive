<script>
  import Icon from './Icon.svelte';

  let { value = $bindable(''), icon = 'search', ...rest } = $props();
</script>

<label class="search" class:set={value !== ''}>
  {#if icon}<Icon name={icon} />{/if}
  <input type="search" bind:value {...rest} />
</label>

<style>
  .search {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--space-2);
    width: var(--field);
    height: var(--size-sm);
    padding: var(--space-3);
    border-radius: var(--radius-pill);
    background: var(--panel);
    color: var(--text-2);
    font-size: var(--text-sm);
    transition: background var(--dur) var(--ease);
  }
  .search:hover, .search:focus-within, .set { background: var(--raised); color: var(--text); }
  .search :global(svg) { width: var(--text-md); height: var(--text-md); }
  input:focus-visible { outline: 0; }
  input { flex: 1; min-width: 0; padding: 0; border: 0; background: none; color: var(--text); line-height: 1; outline: 0; }
  input::placeholder { color: var(--text-2); text-transform: capitalize; }
  input::-webkit-search-cancel-button { display: none; }
</style>
