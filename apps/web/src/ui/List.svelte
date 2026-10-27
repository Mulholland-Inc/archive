<script>
  let { children } = $props();
</script>

<div>{@render children()}</div>

<style>
  div { display: flex; flex-direction: column; margin: calc(-1 * var(--space-3)); }
</style>
