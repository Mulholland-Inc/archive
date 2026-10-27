<script>
  let { children } = $props();
</script>

<div>{@render children()}</div>

<style>
  div { padding: var(--space-3); font-size: var(--text-sm); color: var(--text-2); }
</style>
