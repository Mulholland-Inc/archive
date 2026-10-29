<script>
  let { them, rows } = $props();
</script>

<div class="compare">
  <div class="h"></div><div class="h">Archive</div><div class="h">{them}</div>
  {#each rows as [k, us, other]}<div class="k">{k}</div><div class="us">{us}</div><div>{other}</div>{/each}
</div>
