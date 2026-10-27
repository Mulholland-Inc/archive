<script>
  import Icon from './Icon.svelte';

  let { title, icon, details = [], children } = $props();
</script>

<article class:rich={icon || children}>
  {#if icon}<span class="icon"><Icon name={icon} /></span>{/if}
  <div class="title">{title}</div>
  {#each details as d, i (i)}<div class="detail">{d}</div>{/each}
  {@render children?.()}
</article>

<style>
  article { display: flex; flex-direction: column; padding: var(--space-3); background: var(--raised); border-radius: var(--radius-lg); }
  .rich { gap: var(--space-2); padding: var(--space-4); background: var(--panel); border-radius: var(--radius-xl); }
  .icon { display: grid; place-items: center; width: var(--size-lg); height: var(--size-lg); border-radius: var(--radius-md); background: var(--raised); color: var(--text-2); }
  .title { font-weight: var(--weight-medium); }
  .detail { color: var(--text-2); font-size: var(--text-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rich .detail { flex: 1; white-space: normal; }
</style>
