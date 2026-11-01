<script>
  let { leading, trailing, children, mobile = false } = $props();
</script>

<header class={['topbar', { mobile }]}>
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
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  @media (min-width: 56rem) {
    .mobile {
      display: none;
    }
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
