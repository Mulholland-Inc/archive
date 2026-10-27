<script>
  import Icon from './Icon.svelte';

  let { icon, label, on = false, ...rest } = $props();
</script>

<button class="tool" class:on title={label} aria-label={label} aria-pressed={on} onmousedown={(e) => e.preventDefault()} {...rest}><Icon name={icon} /></button>

<style>
  .tool {
    display: grid;
    flex: none;
    place-items: center;
    width: var(--size-sm);
    height: var(--size-sm);
    padding: 0;
    border: 0;
    border-radius: var(--radius-pill);
    background: none;
    color: var(--text-2);
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .tool:hover, .tool:focus-visible { outline: 0; background: var(--raised); color: var(--text); }
  .on, .on:hover, .on:focus-visible { background: var(--raised); color: var(--text); }
  .tool :global(svg) { width: var(--text-lg); height: var(--text-lg); }
</style>
