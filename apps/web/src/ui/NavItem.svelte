<script>
  import Icon from './Icon.svelte';

  let { href, icon, active = false, children } = $props();
</script>

<a {href} class:active aria-current={active ? 'page' : undefined}><Icon name={icon} />{@render children()}</a>

<style>
  a {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3);
    line-height: 1;
    border-radius: var(--radius-pill);
    font-weight: var(--weight-medium);
    text-transform: capitalize;
    transition: background var(--dur) var(--ease);
  }
  a:hover { background: var(--hover); }
  .active, .active:hover { background: var(--cream); color: var(--cream-ink); font-weight: var(--weight-bold); }
  @media (max-width: 767px) {
    a { flex: none; white-space: nowrap; }
    a :global(svg) { display: none; }
  }
</style>
