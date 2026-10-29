<script>
  let { error = false, children } = $props();
</script>

<p class:error>{@render children()}</p>

<style>
  p { color: var(--fg-muted); }
  .error { color: var(--danger); }
</style>
