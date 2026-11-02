<script>
  // An outlined area on the specimen page to try a pointer or keyboard interaction on, like a context menu.
  let { label, children, ...rest } = $props();
</script>

<div class="target" tabindex="0" role="button" aria-label={label} {...rest}>{@render children()}</div>

<style>
  .target {
    display: grid;
    place-items: center;
    width: 100%;
    min-height: var(--space-24);
    border-radius: var(--radius-lg);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    user-select: none;
  }

  .target:global([data-contextmenu]) {
    background: var(--color-fill-subtle);
  }
</style>
