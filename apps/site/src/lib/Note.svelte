<script>
  let { mark = 'i', children } = $props();
</script>

<div class="note"><b>{mark}</b><p>{@render children()}</p></div>
