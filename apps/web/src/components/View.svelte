<script>
  // A page shows at once: no entrance, which made every navigation feel slow and flash.
  let { children } = $props();
</script>

<div class="view">
  {@render children()}
</div>

<style>
  .view {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }
</style>
