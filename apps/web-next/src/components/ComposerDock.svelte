<script>
  let { children, above } = $props();
</script>

<div class="dock">
  <div class="inner">
    {#if above}{@render above()}{/if}
    {@render children()}
  </div>
</div>

<style>
  .dock {
    position: relative;
    flex: none;
    padding: var(--space-4);
    padding-block-start: var(--space-1);
  }

  .dock::before {
    content: '';
    position: absolute;
    inset-inline: 0;
    inset-block-end: 100%;
    height: var(--fade-size);
    background: linear-gradient(to top, var(--color-canvas), transparent);
    pointer-events: none;
  }

  .inner {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-width: var(--composer-width);
    margin-inline: auto;
  }
</style>
