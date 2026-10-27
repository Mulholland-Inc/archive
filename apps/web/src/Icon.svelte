<script module>
  const ICONS = {
    file: { d: 'M11.25 3H6.25A1.75 1.75 0 0 0 4.5 4.75v10.5A1.75 1.75 0 0 0 6.25 17h7.5a1.75 1.75 0 0 0 1.75-1.75V7.25zM11.25 3v4.25h4.25' },
    folder: { d: 'M3.25 6.25A1.75 1.75 0 0 1 5 4.5h3.1l1.6 1.75H15a1.75 1.75 0 0 1 1.75 1.75v6.5A1.75 1.75 0 0 1 15 16.25H5a1.75 1.75 0 0 1-1.75-1.75zM3.25 8.75h13.5' },
    page: { d: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3zM7.75 7.25h4.5M7.75 10h4.5M7.75 12.75h2.75', tint: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3z' },
    newpage: { d: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3zM10 7.25v5.5M7.25 10h5.5', tint: 'M6.25 3h7.5a1.75 1.75 0 0 1 1.75 1.75v10.5A1.75 1.75 0 0 1 13.75 17h-7.5a1.75 1.75 0 0 1-1.75-1.75V4.75A1.75 1.75 0 0 1 6.25 3z' },
    graph: { d: 'M7.75 4.5a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0M2.5 15a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0M13 15a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0M8.99 6.51l-3.23 6.48M11.01 6.51l3.23 6.48M7 15h6', tint: 'M10 4.5L15.25 15H4.75z', solid: 'M7.75 4.5a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0M2.5 15a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0M13 15a2.25 2.25 0 1 0 4.5 0 2.25 2.25 0 1 0-4.5 0' },
    hash: { d: 'M8.25 3.5l-1.5 13M13.25 3.5l-1.5 13M4 7.5h12.5M3.5 12.5h12.5' },
    signout: { d: 'M8 16.25H5.25a1.5 1.5 0 0 1-1.5-1.5V5.25a1.5 1.5 0 0 1 1.5-1.5H8M12.75 13.5L16.25 10l-3.5-3.5M16.25 10H8.25' },
    plus: { d: 'M10 4.25v11.5M4.25 10h11.5' },
    search: { d: 'M13.75 8.75a5 5 0 1 1-10 0 5 5 0 0 1 10 0zM12.4 12.4l3.85 3.85' },
  };
</script>

<script>
  let { name, size = 20 } = $props();
  const icon = $derived(ICONS[name]);
</script>

<svg class="icon" width={size} height={size} viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
  {#if icon.tint}<path class="tint" d={icon.tint} stroke="none" />{/if}
  {#if icon.solid}<path class="solid" d={icon.solid} stroke="none" />{/if}
  {#if icon.d}<path d={icon.d} />{/if}
  {#if icon.fill}<path d={icon.fill} fill="currentColor" stroke="none" />{/if}
</svg>

<style>
  .icon {
    display: block;
    flex: none;
  }
  .tint {
    fill: var(--icon-tint, none);
  }
  .solid {
    fill: var(--icon-solid, none);
  }
</style>
