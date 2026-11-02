<script>
  let { children, above } = $props();

  let dock = $state();
  let height = $state(0);

  // The dock floats over the page, so the page keeps this much room at its end for the last message.
  $effect(() => {
    const view = dock?.parentElement;
    view?.style.setProperty('--dock-height', `${height}px`);
    return () => view?.style.removeProperty('--dock-height');
  });
</script>

<div class="dock" bind:this={dock} bind:offsetHeight={height}>
  <div class="inner">
    {#if above}{@render above()}{/if}
    {@render children()}
  </div>
</div>

<style>
  .dock {
    position: absolute;
    inset-inline: 0;
    inset-block-end: 0;
    padding: var(--space-4);
    padding-block-start: 0;
    pointer-events: none;
  }

  .inner {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-width: var(--composer-width);
    margin-inline: auto;
    pointer-events: auto;
  }
</style>
