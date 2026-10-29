<script>
  import Icon from '#kit/Icon.svelte';

  let { href, target, icon, image, lead, meta = [], children } = $props();
</script>

<svelte:element this={href ? 'a' : 'div'} class="item" class:link={href} {href} {target}>
  {#if lead}<span class="lead">{lead}</span>{/if}
  {#if icon || image}
    <span class="thumb">
      {#if image}<img src={image} alt="" loading="lazy" />{:else}<Icon name={icon} />{/if}
    </span>
  {/if}
  <span class="main">{@render children()}</span>
  {#each meta as m, i (i)}<span class="meta">{m}</span>{/each}
</svelte:element>

<style>
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-inline: calc(-1 * var(--item-pad));
    padding: var(--item-pad);
    border-radius: var(--radius-item);
    transition: background var(--duration-fast);
  }
  .link:hover { background: var(--fill-faint); }
  .lead { flex: none; width: var(--size-control-lg); color: var(--fg-muted); font-size: var(--text-table); }
  .thumb {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--size-control-xs);
    height: var(--size-control-xs);
    overflow: hidden;
    border-radius: var(--radius-xs);
    background: var(--fill);
    color: var(--fg-muted);
  }
  img { width: 100%; height: 100%; object-fit: cover; }
  .main { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { flex: none; justify-content: flex-end; text-align: right; min-width: var(--size-control-lg); color: var(--fg-muted); font-size: var(--text-table); }
  .meta:not(:last-child) { display: var(--wide-only); }
</style>
