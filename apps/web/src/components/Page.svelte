<script>
  let { width = 'page', center = false, children, element = $bindable(), onscroll } = $props();
</script>

<div class="scroll" bind:this={element} {onscroll}>
  <div class={['inner', width, { center }]}>
    {@render children()}
  </div>
</div>

<style>
  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    container-type: inline-size;
  }

  .inner {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    width: 100%;
    margin-inline: auto;
    padding: var(--space-8);
  }

  .chat { max-width: calc(var(--chat-width) + var(--space-16)); padding-block-end: calc(var(--space-8) + var(--dock-height)); }
  .home { max-width: calc(var(--home-width) + var(--space-16)); }
  .page { max-width: calc(var(--page-width) + var(--space-16)); }
  .wide { max-width: calc(var(--page-wide) + var(--space-16)); }
  .full { max-width: none; }

  .center {
    min-height: 100%;
    justify-content: center;
  }

  @media (max-width: 40rem) {
    .inner {
      gap: var(--space-5);
      padding: var(--space-4);
    }

    .chat {
      padding-block-end: calc(var(--space-4) + var(--dock-height));
    }
  }
</style>
