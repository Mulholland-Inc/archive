<script>
  import Icon from './Icon.svelte';

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
    gap: var(--space-4);
    padding: var(--space-3);
    margin: 0 calc(-1 * var(--space-3));
    border-radius: var(--radius-lg);
    transition: background var(--dur) var(--ease);
  }
  .link:hover { background: var(--hover); }
  .lead { flex: none; width: var(--size-xl); font-size: var(--text-sm); color: var(--text-2); }
  .thumb {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--size-lg);
    height: var(--size-lg);
    border-radius: var(--radius-md);
    background: var(--raised);
    color: var(--text-2);
    overflow: hidden;
  }
  img { width: 100%; height: 100%; object-fit: cover; }
  .main { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { flex: none; min-width: var(--size-xl); font-size: var(--text-sm); color: var(--text-2); text-align: right; }
  @media (max-width: 767px) {
    .meta:not(:last-child) { display: none; }
  }
</style>
