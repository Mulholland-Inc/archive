<script lang="ts" module>
  export type MenuItem = {
    id: string;
    label: string;
    icon?: string;
    glyph?: string;
    avatar?: { name: string; color: number };
    separator?: boolean;
    trailing?: string;
  };
</script>

<script lang="ts">
  import Icon from './Icon.svelte';
  import Avatar from './Avatar.svelte';
  import Checkbox from './Checkbox.svelte';

  let {
    items,
    searchable = true,
    placeholder = 'Filter…',
    multiple = false,
    selected = [],
    onselect
  }: {
    items: MenuItem[];
    searchable?: boolean;
    placeholder?: string;
    multiple?: boolean;
    selected?: string[];
    onselect?: (item: MenuItem) => void;
  } = $props();

  let query = $state('');
  let active = $state(0);
  let list: HTMLElement | undefined = $state();
  let input: HTMLInputElement | undefined = $state();
  let hl = $state({ top: 0, height: 0, visible: false });
  let keyed = $state(false);

  const shown = $derived(
    query ? items.filter((i) => i.label.toLowerCase().includes(query.trim().toLowerCase())) : items
  );

  $effect(() => {
    if (searchable) input?.focus();
    else list?.focus();
  });

  $effect(() => {
    query;
    active = 0;
  });

  $effect(() => {
    const el = list?.querySelectorAll<HTMLElement>('[data-item]')[active];
    if (!el) {
      hl = { ...hl, visible: false };
      return;
    }
    hl = { top: el.offsetTop, height: el.offsetHeight, visible: true };
    if (keyed) el.scrollIntoView({ block: 'nearest' });
  });

  function onkey(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      keyed = true;
      active = (active + 1) % Math.max(shown.length, 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      keyed = true;
      active = (active - 1 + shown.length) % Math.max(shown.length, 1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const item = shown[active];
      if (item) onselect?.(item);
    }
  }
</script>

<div class="menu" role="presentation" onkeydown={onkey}>
  {#if searchable}
    <label class="search">
      <Icon name="search" />
      <input bind:this={input} bind:value={query} {placeholder} aria-label={placeholder} />
    </label>
  {/if}
  <div class="list" role="listbox" tabindex="-1" aria-multiselectable={multiple} bind:this={list}>
    <div class="highlight" data-visible={hl.visible || undefined} style:--top={`${hl.top}px`} style:--h={`${hl.height}px`}></div>
    {#each shown as item, i (item.id)}
      {#if item.separator && !query}<div class="sep" role="separator"></div>{/if}
      <div
        class="item"
        data-item
        role="option"
        tabindex="-1"
        aria-selected={selected.includes(item.id)}
        onpointermove={() => {
          keyed = false;
          active = i;
        }}
        onclick={() => onselect?.(item)}
        onkeydown={() => {}}
      >
        {#if multiple}<Checkbox checked={selected.includes(item.id)} />{/if}
        {#if item.avatar}
          <Avatar name={item.avatar.name} color={item.avatar.color} size="sm" />
        {:else if item.glyph}
          <Icon name={item.glyph} />
        {:else if item.icon}
          <Icon name={item.icon} />
        {/if}
        <span class="text">{item.label}</span>
        {#if !multiple && selected.includes(item.id)}<span class="tick"><Icon name="check" size="sm" /></span>{/if}
        {#if item.trailing}<span class="trailing">{item.trailing}</span>{/if}
      </div>
    {:else}
      <div class="empty">No results</div>
    {/each}
  </div>
</div>

<style>
  .menu {
    width: var(--menu-width);
    padding: var(--space-1);
    border-radius: var(--menu-radius);
    background: var(--popover);
    color: var(--popover-foreground);
    box-shadow: var(--shadow-popover);
    font-size: var(--text-base);
  }
  .search {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: calc(var(--menu-item-height) + var(--space-1-5));
    margin: 0 var(--space-1-5) var(--space-1);
    padding: 0 var(--space-0-5);
    border-bottom: var(--border-width) solid var(--border);
    color: var(--muted-foreground);
  }
  .search input {
    flex: 1;
    min-width: 0;
    color: var(--foreground);
  }
  .search input::placeholder {
    color: var(--muted-foreground);
  }
  .list {
    position: relative;
    max-height: calc(var(--menu-item-height) * 12);
    overflow-y: auto;
    outline: none;
  }
  .highlight {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: var(--h);
    translate: 0 var(--top);
    border-radius: var(--radius);
    background: var(--tint-2);
    opacity: 0;
    transition:
      translate var(--duration-normal) var(--ease-out),
      height var(--duration-normal) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out);
    pointer-events: none;
  }
  .highlight[data-visible] {
    opacity: 1;
  }
  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-2-5);
    height: var(--menu-item-height);
    padding: 0 var(--space-2);
    border-radius: var(--radius);
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
  }
  .text {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tick {
    display: grid;
    color: var(--accent);
    animation: tick-in var(--duration-normal) var(--ease-spring);
  }
  .trailing {
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  .sep {
    height: var(--border-width);
    margin: var(--space-1) var(--space-1-5);
    background: var(--border);
  }
  .empty {
    padding: var(--space-2);
    color: var(--muted-foreground);
  }
  @keyframes tick-in {
    from {
      transform: scale(var(--enter-scale-small));
      opacity: 0;
    }
  }
</style>
