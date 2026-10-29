<script>
  import Icon from '#kit/Icon.svelte';

  let { value = $bindable(''), placeholder = 'Search' } = $props();
  let field = $state();

  function key(e) {
    if (e.key === 'Escape' && value) {
      e.stopPropagation();
      value = '';
    }
  }

  function clear() {
    value = '';
    field.focus();
  }
</script>

<label class="search" class:set={value !== ''}>
  <Icon name="search" />
  <input bind:this={field} type="search" bind:value {placeholder} aria-label={placeholder} onkeydown={key} />
  {#if value}<button class="clear" type="button" aria-label="Clear search" onclick={clear}><Icon name="x" /></button>{/if}
</label>

<style>
  .search {
    display: flex;
    flex: 0 0 var(--search-width);
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    height: var(--filter-height);
    padding: var(--space-2);
    border-radius: var(--radius-full);
    background: var(--fill);
    color: var(--fg-muted);
    font-size: var(--text-table);
    line-height: var(--leading-none);
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  .search:hover, .search:focus-within, .set { background: var(--fill-strong); color: var(--fg); }
  input { flex: 1; min-width: 0; color: var(--fg); line-height: var(--leading-none); outline: 0; }
  input::placeholder { color: var(--fg-muted); }
  input::-webkit-search-cancel-button { display: none; }
  .clear { display: grid; place-items: center; color: var(--fg-muted); transition: color var(--duration-fast); }
  .clear:hover { color: var(--fg); }
</style>
