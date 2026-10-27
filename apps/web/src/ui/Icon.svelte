<script module>
  const lucide = import.meta.glob('/node_modules/@lucide/svelte/dist/icons/*.svelte', { import: 'default' });
  const load = (name) => (lucide[`/node_modules/@lucide/svelte/dist/icons/${name}.svelte`] ?? lucide['/node_modules/@lucide/svelte/dist/icons/shapes.svelte'])();
  const ICONS = {
    file: { d: 'M11.25 3H6.25A1.75 1.75 0 0 0 4.5 4.75v10.5A1.75 1.75 0 0 0 6.25 17h7.5a1.75 1.75 0 0 0 1.75-1.75V7.25zM11.25 3v4.25h4.25' },
    folder: { d: 'M3.25 6.25A1.75 1.75 0 0 1 5 4.5h3.1l1.6 1.75H15a1.75 1.75 0 0 1 1.75 1.75v6.5A1.75 1.75 0 0 1 15 16.25H5a1.75 1.75 0 0 1-1.75-1.75zM3.25 8.75h13.5' },
    page: { d: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3zM7.75 7.25h4.5M7.75 10h4.5M7.75 12.75h2.75' },
    newpage: { d: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3zM10 7.25v5.5M7.25 10h5.5' },
    signout: { d: 'M8 16.25H5.25a1.5 1.5 0 0 1-1.5-1.5V5.25a1.5 1.5 0 0 1 1.5-1.5H8M12.75 13.5L16.25 10l-3.5-3.5M16.25 10H8.25' },
    plus: { d: 'M10 4.25v11.5M4.25 10h11.5' },
  };
</script>

<script>
  let { name } = $props();
  const icon = $derived(ICONS[name]);
</script>

{#if icon}
  <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
    {#if icon.d}<path d={icon.d} />{/if}
    {#if icon.fill}<path d={icon.fill} fill="currentColor" stroke="none" />{/if}
  </svg>
{:else}
  {#await load(name)}
    <svg class="icon" aria-hidden="true"></svg>
  {:then Lucide}
    <Lucide class="icon" strokeWidth={1.75} aria-hidden="true" />
  {/await}
{/if}

<style>
  :global(svg.icon) { width: var(--size-icon); height: var(--size-icon); }
</style>
