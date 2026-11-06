<script>
  let { label, children } = $props();
</script>

<ul class="list" aria-label={label}>
  {@render children()}
</ul>

<style>
  .list {
    display: flex;
    flex-direction: column;
    margin-inline: calc(var(--space-2) * -1);
  }
</style>
