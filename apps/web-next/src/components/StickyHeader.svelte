<script>
  let { title, search, actions } = $props();

  let height = $state(0);
  let compact = $state(false);

  function watch(node) {
    let root = node.parentElement;
    while (root && !/(auto|scroll)/.test(getComputedStyle(root).overflowY)) root = root.parentElement;
    const observer = new IntersectionObserver(([entry]) => (compact = !entry.isIntersecting && entry.boundingClientRect.top < (entry.rootBounds?.top ?? 0)), { root, rootMargin: '-1px 0px 0px 0px' });
    observer.observe(node);
    return () => observer.disconnect();
  }
</script>

<header class={['header', { compact }]} style:--top-height={`${height}px`}>
  <div class="top" bind:offsetHeight={height} {@attach watch}>
    <h1>{title}</h1>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </div>
  <div class="bar">
    <span class="mini" aria-hidden={!compact}>{title}</span>
    {#if search}<div class="search">{@render search()}</div>{/if}
    {#if actions && compact}<div class="actions">{@render actions()}</div>{/if}
  </div>
</header>

<style>
  .header {
    position: sticky;
    inset-block-start: calc(var(--top-height) * -1);
    z-index: var(--z-sticky);
    display: flex;
    flex-direction: column;
    margin-block-end: calc(var(--space-8) * -1 + var(--space-5));
    background: var(--color-canvas);
  }

  .top {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--control-xl);
    padding-block-end: var(--space-3);
  }

  h1 {
    flex: 1;
    min-width: 0;
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
  }

  .bar {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--topbar-height);
  }

  .compact .bar {
    border-style: solid;
    border-width: 0;
    border-image: linear-gradient(var(--color-border-subtle), var(--color-border-subtle)) 1 / 0 0 var(--border-width) 0 / 0 100vw;
  }

  .mini {
    flex: none;
    max-width: 0;
    overflow: hidden;
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-serif);
    line-height: var(--leading-tight);
    white-space: nowrap;
    opacity: 0;
    transform: translateY(var(--shift-sm));
    transition:
      max-width var(--duration-normal) var(--ease-out),
      margin var(--duration-normal) var(--ease-out),
      opacity var(--duration-normal) var(--ease-out),
      transform var(--duration-normal) var(--ease-out);
  }

  .compact .mini {
    max-width: var(--col-sm);
    margin-inline-end: var(--space-2);
    opacity: 1;
    transform: none;
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

  .compact .search :global(.box) {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
  }

  .bar .actions {
    margin-inline-start: auto;
    animation: actions-in var(--duration-normal) var(--ease-out);
  }

  @keyframes actions-in {
    from {
      opacity: 0;
      transform: translateY(var(--shift-sm));
    }
  }

  @media (max-width: 40rem) {
    .header {
      margin-block-end: calc(var(--space-5) * -1 + var(--space-3));
    }

    .search > :global(:first-child) {
      max-width: none;
    }

    .compact .mini,
    .bar .actions {
      display: none;
    }
  }
</style>
