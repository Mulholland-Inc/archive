<script>
  let { children } = $props();
</script>

<div>{@render children()}</div>

<style>
  div { display: grid; align-content: start; grid-template-columns: repeat(auto-fill, minmax(min(var(--tile), 100%), 1fr)); gap: var(--space-3); }
</style>
