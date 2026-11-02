<script>
  // Free content in a sidebar section, inset to line up with the section's items.
  let { children } = $props();
</script>

<div class="block">{@render children()}</div>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-1-5);
  }
</style>
