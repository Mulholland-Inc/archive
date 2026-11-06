<script>
  let { element = $bindable(), multiple = false, onchange } = $props();
</script>

<input class="picker" type="file" {multiple} bind:this={element} {onchange} tabindex="-1" aria-hidden="true" />

<style>
  .picker {
    display: none;
  }
</style>
