<script>
  let { wide, narrow } = $props();
</script>

<div class="wide">{@render wide()}</div>
<div class="narrow">{@render narrow()}</div>

<style>
  .narrow {
    display: none;
  }

  @media (max-width: 40rem) {
    .wide {
      display: none;
    }

    .narrow {
      display: block;
    }
  }
</style>
