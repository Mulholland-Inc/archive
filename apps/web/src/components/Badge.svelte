<script>
  import Icon from './Icon.svelte';

  let { icon, spin = false, danger = false, children } = $props();
</script>

<span class="badge">
  {#if icon}<span class={['glyph', { spin, danger }]}><Icon {icon} size="xs" /></span>{/if}
  {@render children()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    flex: none;
    width: fit-content;
    height: var(--avatar-xs);
    padding: var(--space-1) var(--space-1-5);
    border-radius: var(--radius-sm);
    background: var(--color-fill);
    color: var(--color-text);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
    white-space: nowrap;
  }

  /* The icon brings its own room on the left; the text needs it on the right. */
  .badge:has(.glyph) {
    padding-inline-start: var(--space-1);
  }

  .glyph {
    display: inline-flex;
    color: var(--color-text-tertiary);
  }

  .danger {
    color: var(--color-danger);
  }

  .spin {
    animation: spin var(--duration-spin) linear infinite;
  }

  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation: none;
    }
  }
</style>
