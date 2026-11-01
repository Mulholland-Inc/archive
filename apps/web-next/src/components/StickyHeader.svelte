<script>
  let { title, search, actions } = $props();

  let stuck = $state(false);

  function watch(node) {
    let root = node.parentElement;
    while (root && !/(auto|scroll)/.test(getComputedStyle(root).overflowY)) root = root.parentElement;
    const observer = new IntersectionObserver(([entry]) => (stuck = !entry.isIntersecting), { root });
    observer.observe(node);
    return () => observer.disconnect();
  }
</script>

<div class="sentinel" {@attach watch}></div>
<header class={['header', { stuck }]}>
  <h1>{title}</h1>
  {#if search}<div class="search">{@render search()}</div>{/if}
  {#if actions}<div class="actions">{@render actions()}</div>{/if}
</header>

<style>
  .sentinel {
    height: 0;
    margin-block-end: calc(var(--space-8) * -1);
  }

  .header {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2) var(--space-4);
    padding-block: var(--space-2);
    border-block-end: var(--border-width) solid transparent;
    background: var(--color-canvas);
    transition: border-color var(--duration-normal) var(--ease-out);
  }

  .stuck {
    border-block-end-color: var(--color-border-subtle);
  }

  h1 {
    flex: none;
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-normal);
  }

  .search {
    display: flex;
    flex: 1;
    gap: var(--space-2);
    align-items: center;
    min-width: 0;
  }

  .search > :global(:first-child) {
    flex: 1;
    max-width: var(--col-lg);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    margin-inline-start: auto;
  }

  @media (max-width: 40rem) {
    .search {
      flex-basis: 100%;
      order: 1;
    }

    .search > :global(:first-child) {
      max-width: none;
    }
  }
</style>
