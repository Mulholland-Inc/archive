<script>
  let { title, actions, gap = 'md', children } = $props();
</script>

<section class={['section', gap]}>
  {#if title || actions}
    <div class="head">
      {#if title}<h2>{title}</h2>{/if}
      {#if actions}<div class="actions">{@render actions()}</div>{/if}
    </div>
  {/if}
  {@render children()}
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
  }

  .lg {
    gap: var(--space-5);
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: var(--control-md);
  }

  h2 {
    font-size: var(--text-base);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-snug);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
</style>
