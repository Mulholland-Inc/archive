<script>
  import Icon from '#kit/Icon.svelte';

  let { href, icon, active = false, plain = false, children } = $props();
</script>

<a {href} class:active class:plain aria-current={active ? 'page' : undefined}>{#if icon}<Icon name={icon} />{/if}<span>{@render children()}</span></a>

<style>
  a {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--space-2);
    padding: var(--item-pad);
    border-radius: var(--radius-nav);
    color: var(--fg-soft);
    line-height: var(--leading-snug);
    white-space: nowrap;
    text-transform: capitalize;
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  span { overflow: hidden; text-overflow: ellipsis; }
  .plain { text-transform: none; }
  a:hover { background: var(--fill-faint); color: var(--fg); }
  .active, .active:hover { background: var(--fill); color: var(--fg); }
</style>
