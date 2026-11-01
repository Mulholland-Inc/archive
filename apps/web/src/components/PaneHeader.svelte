<script>
  import Art from './Art.svelte';

  let { title, description, art, search } = $props();

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

<header class={['head', { compact, sticky: !!search }]} style:--lead-height={`${height}px`}>
  <div class="lead" bind:offsetHeight={height} {@attach watch}>
    {#if art}<div class="art"><Art name={art} /></div>{/if}
    <div class="text">
      <h2>{title}</h2>
      {#if description}<p>{description}</p>{/if}
    </div>
  </div>
  {#if search}
    <div class="bar">
      <span class="mini" aria-hidden={!compact}>{title}</span>
      <div class="search">{@render search()}</div>
    </div>
  {/if}
</header>

<style>
  .head {
    display: flex;
    flex-direction: column;
    margin-inline: calc(var(--pane-pad, 0px) * -1);
    margin-block-end: var(--space-4);
    padding-inline: var(--pane-pad, 0px);
    background: var(--color-surface-raised);
  }

  .sticky {
    position: sticky;
    inset-block-start: calc((var(--lead-height) + var(--pane-pad-top, 0px)) * -1);
    z-index: var(--z-sticky);
    margin-block-end: var(--space-2);
  }

  .compact {
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .lead {
    display: flex;
    align-items: center;
    gap: var(--space-5);
    padding-block: var(--space-4) var(--space-5);
  }

  .art {
    display: flex;
    flex: none;
    width: calc(var(--pane-art) * 1.4);
    height: var(--pane-art);
  }

  .art :global(img) {
    width: 100%;
    height: 100%;
    margin: 0;
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: var(--space-1-5);
    min-width: 0;
  }

  h2 {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
  }

  p {
    max-width: 36rem;
    color: var(--color-text-secondary);
    font-size: var(--text-md);
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--topbar-height);
    padding-block: var(--space-2);
  }

  .mini {
    flex: none;
    max-width: 0;
    overflow: hidden;
    font-family: var(--font-serif);
    font-size: var(--text-lg);
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
    max-width: var(--col-md);
    margin-inline-end: var(--space-2);
    opacity: 1;
    transform: none;
  }

  .search {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  @container (max-width: 32rem) {
    .lead {
      gap: var(--space-4);
    }

    .art {
      width: var(--pane-art);
      height: calc(var(--pane-art) * 0.75);
    }

    h2 {
      font-size: var(--text-xl);
    }

    .compact .mini {
      display: none;
    }
  }
</style>
