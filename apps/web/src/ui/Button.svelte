<script>
  import Icon from './Icon.svelte';

  let { icon, quiet = false, round = false, children, ...rest } = $props();
</script>

<button class:quiet class:round={round || !children} {...rest}>
  {#if icon}<Icon name={icon} />{/if}
  {@render children?.()}
</button>

<style>
  button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    height: var(--size-lg);
    padding: 0 var(--space-5);
    border: 0;
    border-radius: var(--radius-pill);
    background: var(--cream);
    color: var(--cream-ink);
    font-weight: var(--weight-medium);
    letter-spacing: var(--tracking-tight);
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease), transform var(--dur) var(--ease);
  }
  button:hover { background: var(--cream-hover); }
  button:active { transform: scale(0.96); }
  button:disabled { cursor: default; }
  .round { width: var(--size-lg); padding: 0; }
  .quiet { background: none; color: var(--text-2); width: var(--size-sm); height: var(--size-sm); }
  .quiet:hover { background: var(--hover); color: var(--text); }
</style>
