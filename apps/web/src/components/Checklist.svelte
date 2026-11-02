<script>
  // A card of ChecklistItem rows.
  let { children } = $props();
</script>

<ul class="checklist">{@render children()}</ul>

<style>
  .checklist {
    display: flex;
    flex-direction: column;
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: var(--ring);
  }

  .checklist > :global(li + li) {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }
</style>
