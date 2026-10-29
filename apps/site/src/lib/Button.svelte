<script>
  import Icon from './Icon.svelte';

  let { href, ghost = false, small = false, onclick, children } = $props();

  const cls = $derived(['btn', ghost && 'btn-ghost', small && 'btn-sm']);
</script>

{#if href}
  <a class={cls} {href}><span>{@render children()}</span><Icon name="arrow" /></a>
{:else}
  <button type="button" class={cls} {onclick}><span>{@render children()}</span></button>
{/if}
