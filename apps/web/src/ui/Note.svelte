<script>
  let { error = false, children } = $props();
</script>

<p class:error>{@render children()}</p>

<style>
  p { color: var(--text-2); }
  .error { color: var(--alarm); margin-bottom: var(--space-4); }
</style>
