<script>
  import Icon from './Icon.svelte';

  let { href, ghost = false, small = false, icon, type = 'button', onclick, children, ...rest } = $props();

  const cls = $derived(children ? ['btn', ghost && 'btn-ghost', small && 'btn-sm'] : 'icon-btn');
</script>

{#if href}
  <a class={cls} {href} {...rest}><span>{@render children()}</span><Icon name="arrow" /></a>
{:else}
  <button {type} class={cls} {onclick} {...rest}>{#if icon}<Icon name={icon} />{/if}{#if children}<span>{@render children()}</span>{/if}</button>
{/if}
