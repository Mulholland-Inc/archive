<script>
  // A list of ListItem rows, pulled out so each row's text lines up with the content around it.
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
