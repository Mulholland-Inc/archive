<script>
  let { title, size = 'md', sticky = false, leading, actions, children } = $props();
</script>

<header class={['header', size, { sticky }]}>
  {#if leading}<div class="leading">{@render leading()}</div>{/if}
  <div class="row">
    <h1>{title}</h1>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </div>
  {#if children}{@render children()}{/if}
</header>

<style>
  .header {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
  }

  .sticky {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    padding-block: var(--space-3);
    background: var(--color-canvas);
  }

  .sticky h1 {
    flex: none;
  }

  .sticky .actions {
    flex: 1;
    justify-content: flex-end;
    min-width: 0;
  }

  .leading {
    display: flex;
    align-items: center;
    min-height: var(--control-md);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    min-height: var(--control-lg);
  }

  h1 {
    min-width: 0;
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-normal);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lg h1 {
    font-size: var(--text-3xl);
    letter-spacing: var(--tracking-tight);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
  }
</style>
