<script>
  import Icon from './Icon.svelte';

  let { icon, href, quiet = false, round = false, children, ...rest } = $props();
</script>

<svelte:element this={href ? 'a' : 'button'} class="button" class:quiet class:round={round || !children} {href} {...rest}>
  {#if icon}<Icon name={icon} />{/if}
  {@render children?.()}
</svelte:element>

<style>
  .button {
    display: inline-flex;
    flex: none;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-3);
    border: 0;
    border-radius: var(--radius-pill);
    background: var(--cream);
    color: var(--cream-ink);
    font-weight: var(--weight-medium);
    line-height: 1;
    letter-spacing: var(--tracking-tight);
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease), transform var(--dur) var(--ease);
  }
  .button:hover { background: var(--cream-hover); }
  .button:active { transform: scale(0.96); }
  .button:disabled { cursor: default; }
  .round { width: var(--size-lg); height: var(--size-lg); padding: 0; }
  .quiet { width: var(--size-sm); height: var(--size-sm); padding: 0; background: none; color: var(--text-2); }
  .quiet:hover { background: var(--hover); color: var(--text); }
</style>
