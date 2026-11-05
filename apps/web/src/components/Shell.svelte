<script>
  let { sidebar, pane, children } = $props();
</script>

<div class="shell">
  {@render sidebar()}
  <main class="main">{@render children()}</main>
  {@render pane?.()}
</div>

<style>
  .shell {
    display: flex;
    height: 100%;
    min-height: 0;
  }

  .main {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    min-height: 0;
  }
</style>
