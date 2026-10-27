<script>
  let { title, details = [] } = $props();
</script>

<article>
  <div class="title">{title}</div>
  {#each details as d, i (i)}<div class="detail">{d}</div>{/each}
</article>

<style>
  article { padding: var(--space-3); background: var(--raised); border-radius: var(--radius-lg); }
  .title { font-weight: var(--weight-medium); }
  .detail { color: var(--text-2); font-size: var(--text-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
