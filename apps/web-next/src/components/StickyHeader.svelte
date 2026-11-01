<script>
  let { title, search, actions } = $props();
</script>

<div class="sentinel"></div>
<header class="header">
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
    background: var(--color-canvas);
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
    .sentinel {
      margin-block-end: calc(var(--space-5) * -1);
    }

    .search {
      flex-basis: 100%;
      order: 1;
    }

    .search > :global(:first-child) {
      max-width: none;
    }
  }
</style>
