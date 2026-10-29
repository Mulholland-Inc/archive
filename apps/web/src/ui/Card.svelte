<script>
  let { title, details = [] } = $props();
</script>

<article>
  <div class="title">{title}</div>
  {#each details as d, i (i)}<div class="detail">{d}</div>{/each}
</article>

<style>
  article { display: flex; flex-direction: column; gap: var(--space-0-5); padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface-raised); }
  .title { font-weight: var(--weight-medium); line-height: var(--leading-snug); }
  .detail { overflow: hidden; color: var(--fg-muted); font-size: var(--text-table); text-overflow: ellipsis; white-space: nowrap; }
</style>
