<script>
  let { id, title, children } = $props();
</script>

<section {id} class="section">
  <h2>{title}</h2>
  <div class="body">{@render children()}</div>
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    scroll-margin-block-start: var(--space-8);
  }

  h2 {
    font-family: var(--font-serif);
    font-size: var(--text-xl);
    font-weight: var(--weight-serif);
    letter-spacing: var(--tracking-serif);
    padding-block-end: var(--space-3);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
</style>
