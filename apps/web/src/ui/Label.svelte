<script>
  let { children } = $props();
</script>

<div>{@render children()}</div>

<style>
  div { font-size: var(--text-sm); color: var(--text-2); padding: var(--space-4) var(--space-3) var(--space-2); }
</style>
