<script>
  import Icon from './Icon.svelte';

  let { icon, href, onclick, active = false, children, ...rest } = $props();
</script>

<svelte:element this={href ? 'a' : onclick ? 'button' : 'span'} class="chip" class:link={href || onclick} class:active {href} {onclick} type={!href && onclick ? 'button' : undefined} {...rest}>
  {#if icon}<Icon name={icon} />{/if}
  {@render children?.()}
</svelte:element>

<style>
  .chip {
    display: inline-flex;
    flex: none;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-pill);
    background: var(--chip, var(--raised));
    color: var(--text-2);
    font: var(--text-xs)/1 var(--mono);
    white-space: nowrap;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .chip :global(svg.icon) { width: var(--text-sm); height: var(--text-sm); }
  .link { cursor: pointer; }
  .link:hover { background: var(--raised-hover); color: var(--text); }
  .active, .active:hover { background: var(--cream); color: var(--cream-ink); }
</style>
