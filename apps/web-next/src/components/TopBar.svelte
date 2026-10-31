<script>
  let { leading, trailing, children, bordered = false } = $props();
</script>

<header class={['topbar', { bordered }]}>
  <div class="side">{@render leading?.()}</div>
  <div class="center">{@render children?.()}</div>
  <div class="side end">{@render trailing?.()}</div>
</header>

<style>
  .topbar {
    position: sticky;
    inset-block-start: 0;
    z-index: var(--z-sticky);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    height: var(--topbar-height);
    padding: var(--space-2);
    background: var(--color-canvas);
    transition: box-shadow var(--duration-normal) var(--ease-out), background-color var(--duration-normal) var(--ease-out);
  }

  .bordered {
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .side {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    min-width: 0;
  }

  .center {
    display: flex;
    align-items: center;
    flex: 1;
    min-width: 0;
  }

  .end {
    justify-content: flex-end;
  }
</style>
