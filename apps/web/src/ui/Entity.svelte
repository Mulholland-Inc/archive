<script>
  import Icon from './Icon.svelte';

  let { title, icon, href, lit = false, aside, children, ...rest } = $props();
</script>

<article class:lit {...rest}>
  <header>
    <span class="icon"><Icon name={icon} /></span>
    <svelte:element this={href ? 'a' : 'span'} class="title" class:link={href} {href}>{title}</svelte:element>
    {@render aside?.()}
  </header>
  <div class="body">{@render children?.()}</div>
</article>

<style>
  article { display: flex; flex-direction: column; gap: var(--space-4); min-width: 0; padding: var(--space-4); background: var(--panel); border-radius: var(--radius-xl); transition: background var(--dur) var(--ease); }
  .lit { --chip: var(--panel); background: var(--raised); }
  header { display: flex; align-items: center; gap: var(--space-3); min-width: 0; }
  .icon { display: grid; flex: none; place-items: center; width: var(--size-lg); height: var(--size-lg); border-radius: var(--radius-md); background: var(--chip, var(--raised)); color: var(--text-2); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: var(--weight-medium); }
  .link:hover { color: var(--text-2); }
  .body { display: flex; flex-direction: column; gap: var(--space-2); }
</style>
