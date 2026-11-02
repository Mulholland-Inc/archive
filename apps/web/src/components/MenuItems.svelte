<script>
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import { SEPARATOR, tidy } from '../lib/contextmenu.svelte.js';

  let { items = [] } = $props();
</script>

{#each tidy(items) as item, i (i)}
  {#if item === SEPARATOR}
    <MenuSeparator />
  {:else if item.heading}
    <MenuLabel>{item.heading}</MenuLabel>
  {:else}
    <MenuItem icon={item.icon} shortcut={item.shortcut} danger={item.danger} disabled={item.disabled} checked={item.checked} href={item.href} onselect={item.onselect}>{item.label}</MenuItem>
  {/if}
{/each}
