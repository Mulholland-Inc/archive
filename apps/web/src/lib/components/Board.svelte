<script lang="ts" generics="T extends { id: string }">
  import type { Snippet } from 'svelte';
  import { flip } from 'svelte/animate';
  import { ms, ease } from '../motion';
  import Count from './Count.svelte';
  import IconButton from './IconButton.svelte';
  import Icon from './Icon.svelte';

  let {
    columns,
    onmove,
    onadd,
    card,
    draggable = true
  }: {
    columns: { id: string; label: string; items: T[] }[];
    onmove: (id: string, column: string, index: number) => void;
    onadd?: (column: string) => void;
    card: Snippet<[T, 'rest' | 'lifted' | 'placeholder']>;
    draggable?: boolean;
  } = $props();

  type Drag = { id: string; item: T; from: string; over: string; index: number; dx: number; dy: number; w: number; x: number; y: number; dropping: boolean };
  let drag = $state<Drag | null>(null);
  let pending: { item: T; from: string; sx: number; sy: number; el: HTMLElement } | null = null;
  let collapsed = $state<Record<string, boolean>>({});

  const shown = $derived(
    columns.map((c) => {
      if (!drag) return c;
      const items = c.items.filter((i) => i.id !== drag!.id);
      if (drag.over === c.id) items.splice(Math.min(drag.index, items.length), 0, drag.item);
      return { ...c, items };
    })
  );

  function down(e: PointerEvent, item: T, from: string) {
    if (!draggable || e.pointerType === 'touch' || e.button !== 0 || (e.target as HTMLElement).closest('button,input,textarea')) return;
    pending = { item, from, sx: e.clientX, sy: e.clientY, el: e.currentTarget as HTMLElement };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up, { once: true });
  }

  function move(e: PointerEvent) {
    if (pending && !drag) {
      if (Math.hypot(e.clientX - pending.sx, e.clientY - pending.sy) < 6) return;
      const r = pending.el.getBoundingClientRect();
      const index = columns.find((c) => c.id === pending!.from)?.items.findIndex((i) => i.id === pending!.item.id) ?? 0;
      drag = {
        id: pending.item.id,
        item: pending.item,
        from: pending.from,
        over: pending.from,
        index,
        dx: pending.sx - r.left,
        dy: pending.sy - r.top,
        w: r.width,
        x: r.left,
        y: r.top,
        dropping: false
      };
      document.body.style.cursor = 'grabbing';
      window.getSelection()?.removeAllRanges();
    }
    if (!drag) return;
    e.preventDefault();
    drag.x = e.clientX - drag.dx;
    drag.y = e.clientY - drag.dy;
    const hit = document.elementsFromPoint(e.clientX, e.clientY).find((n) => (n as HTMLElement).dataset?.column) as HTMLElement | undefined;
    if (!hit) return;
    const list = hit.querySelector<HTMLElement>('[data-cards]');
    if (!list) return;
    const top = list.getBoundingClientRect().top;
    const cards = [...list.querySelectorAll<HTMLElement>('[data-card]')].filter((c) => c.dataset.card !== drag!.id);
    let index = cards.findIndex((c) => e.clientY < top + c.offsetTop + c.offsetHeight / 2);
    if (index < 0) index = cards.length;
    drag.over = hit.dataset.column!;
    drag.index = index;
  }

  function up() {
    window.removeEventListener('pointermove', move);
    document.body.style.cursor = '';
    pending = null;
    if (!drag) return;
    const d = drag;
    const swallow = (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
    };
    window.addEventListener('click', swallow, { capture: true, once: true });
    setTimeout(() => window.removeEventListener('click', swallow, { capture: true }), 0);
    const slot = document.querySelector<HTMLElement>(`[data-card="${CSS.escape(d.id)}"]`);
    if (slot) {
      const r = slot.getBoundingClientRect();
      d.dropping = true;
      d.x = r.left;
      d.y = r.top;
    }
    const original = columns.find((c) => c.id === d.from)?.items.findIndex((i) => i.id === d.id) ?? -1;
    setTimeout(() => {
      drag = null;
      if (d.over !== d.from || d.index !== original) onmove(d.id, d.over, d.index);
    }, ms('normal'));
  }
</script>

<div class="board">
  {#each shown as column (column.id)}
    <section class="column" data-column={column.id} data-target={drag?.over === column.id || undefined}>
      <header class="header">
        <button class="title" onclick={() => (collapsed[column.id] = !collapsed[column.id])} aria-expanded={!collapsed[column.id]}>
          <span class="chev" data-open={!collapsed[column.id] || undefined}><Icon name="chevron-down" size="sm" /></span>
          <span class="label">{column.label}</span>
        </button>
        <Count value={column.items.length} />
        <span class="grow"></span>
        {#if onadd}<IconButton icon="plus" label={`Add to ${column.label}`} onclick={() => onadd?.(column.id)} />{/if}
      </header>
      <div class="cards" data-cards data-collapsed={collapsed[column.id] || undefined}>
        {#each column.items as item (item.id)}
          <div
            class="slot"
            data-card={item.id}
            role="listitem"
            onpointerdown={(e) => down(e, item, column.id)}
            ondragstart={(e) => e.preventDefault()}
            animate:flip={{ duration: ms('slow'), easing: ease('out') }}
          >
            {@render card(item, drag?.id === item.id ? 'placeholder' : 'rest')}
          </div>
        {/each}
      </div>
    </section>
  {/each}
</div>

{#if drag}
  <div class="ghost" data-dropping={drag.dropping || undefined} style:--x={`${drag.x}px`} style:--y={`${drag.y}px`} style:--w={`${drag.w}px`}>
    {@render card(drag.item, drag.dropping ? 'rest' : 'lifted')}
  </div>
{/if}

<style>
  .board {
    display: flex;
    gap: var(--space-4);
    min-height: 100%;
    align-items: flex-start;
  }
  .column {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: none;
    width: var(--column-width);
    padding-bottom: var(--space-12);
  }
  .header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .grow {
    flex: 1;
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-1-5);
    min-width: 0;
    height: var(--control-height);
    padding: 0 var(--space-2) 0 var(--space-1);
    border-radius: var(--radius);
    font-weight: var(--font-medium);
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  .title:hover {
    background: var(--hover);
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .chev {
    display: grid;
    color: var(--muted-foreground);
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  .chev:not([data-open]) {
    transform: rotate(-90deg);
  }
  .cards {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-height: var(--space-20);
    padding: var(--space-1);
    margin: calc(var(--space-1) * -1);
    border-radius: var(--card-radius);
    transition:
      opacity var(--duration-normal) var(--ease-out),
      background-color var(--duration-normal) var(--ease-out);
  }
  [data-collapsed] {
    opacity: 0;
    pointer-events: none;
  }
  [data-target] .cards {
    background: var(--tint-1);
  }
  .slot {
    cursor: grab;
  }
  @media (pointer: fine) {
    .slot {
      touch-action: none;
    }
  }
  .ghost {
    position: fixed;
    top: 0;
    left: 0;
    width: var(--w);
    translate: var(--x) var(--y);
    z-index: var(--z-drag);
    pointer-events: none;
  }
  .ghost[data-dropping] {
    transition: translate var(--duration-normal) var(--ease-out);
  }
</style>
