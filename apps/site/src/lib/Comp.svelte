<script>
  let { band = false, children } = $props();
</script>

<section class={band ? 'band' : undefined}>
  <div class="wrap">
    <div class="comp">{@render children()}</div>
  </div>
</section>
