<script>
  let { zoom = 1, onin, onout, onfit } = $props();
</script>

<div class="zoom" role="group" aria-label="Zoom">
  <button type="button" class="btn" aria-label="Zoom out" onclick={onout}>−</button>
  <button type="button" class="btn pct" aria-label="Fit to screen" onclick={onfit}>{Math.round(zoom * 100)}%</button>
  <button type="button" class="btn" aria-label="Zoom in" onclick={onin}>+</button>
</div>

<style>
  .zoom {
    display: flex;
    align-items: center;
    gap: var(--space-px);
    padding: var(--space-0-5);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .btn {
    display: grid;
    place-items: center;
    min-width: var(--control-sm);
    height: var(--control-sm);
    padding-inline: var(--space-1);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    line-height: var(--leading-none);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .pct {
    min-width: var(--control-xl);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .btn:hover {
    background: var(--color-fill-subtle);
    color: var(--color-text);
  }
</style>
