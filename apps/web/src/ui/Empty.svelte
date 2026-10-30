<script>
  import robot from '../../../site/static/art/robot.webp';

  let { title, children } = $props();
</script>

<div class="empty">
  <img src={robot} alt="" />
  <p class="title">{title}</p>
  {#if children}<p class="hint">{@render children()}</p>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    min-height: 60svh;
    padding: var(--space-4);
    text-align: center;
  }
  img { width: 120px; height: auto; margin-bottom: var(--space-2); }
  .title { color: var(--fg); font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  .hint { color: var(--fg-muted); }
</style>
