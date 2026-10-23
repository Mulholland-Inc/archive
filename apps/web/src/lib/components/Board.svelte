<script lang="ts" generics="T extends { id: string }">
  import type { Snippet } from 'svelte';
  import { flip } from 'svelte/animate';
  import { ms, ease } from '../motion';
  import GroupPill from './GroupPill.svelte';
  import Count from './Count.svelte';
  import IconButton from './IconButton.svelte';

  let {
    columns,
    onmove,
    onadd,
    card
  }: {
    columns: { id: string; status: string; glyph?: string; label: string; items: T[] }[];
    onmove: (id: string, column: string, index: number) => void;
    onadd?: (column: string) => void;
    card: Snippet<[T, 'rest' | 'lifted' | 'placeholder']>;
  } = $props();

  type Drag = { id: string; item: T; dx: number; dy: number; w: number; x: number; y: number; dropping: boolean };
  let drag = $state<Drag | null>(null);
  let pending: { id: string; item: T; sx: number; sy: number; el: HTMLElement } | null = null;
  let collapsed = $state<Record<string, boolean>>({});

  function down(e: PointerEvent, item: T) {
    if (e.button !== 0 || (e.target as HTMLElement).closest('button')) return;
    pending = { id: item.id, item, sx: e.clientX, sy: e.clientY, el: e.currentTarget as HTMLElement };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up, { once: true });
  }

  function move(e: PointerEvent) {
    if (pending && !drag) {
      if (Math.hypot(e.clientX - pending.sx, e.clientY - pending.sy) < 5) return;
      const r = pending.el.getBoundingClientRect();
      drag = { id: pending.id, item: pending.item, dx: pending.sx - r.left, dy: pending.sy - r.top, w: r.width, x: r.left, y: r.top, dropping: false };
      document.body.style.cursor = 'grabbing';
    }
    if (!drag) return;
    drag.x = e.clientX - drag.dx;
    drag.y = e.clientY - drag.dy;
    const hit = document.elementsFromPoint(e.clientX, e.clientY).find((n) => (n as HTMLElement).dataset?.column) as HTMLElement | undefined;
    if (!hit) return;
    const col = hit.dataset.column!;
    const list = hit.querySelector<HTMLElement>('[data-cards]');
    if (!list) return;
    const top = list.getBoundingClientRect().top;
    const cards = [...list.querySelectorAll<HTMLElement>('[data-card]')].filter((c) => c.dataset.card !== drag!.id);
    let index = cards.findIndex((c) => e.clientY < top + c.offsetTop + c.offsetHeight / 2);
    if (index < 0) index = cards.length;
    const here = columns.find((c) => c.items.some((i) => i.id === drag!.id));
    if (here?.id === col && here.items.findIndex((i) => i.id === drag!.id) === index) return;
    onmove(drag.id, col, index);
  }

  function up() {
    window.removeEventListener('pointermove', move);
    document.body.style.cursor = '';
    pending = null;
    if (!drag) return;
    const slot = document.querySelector<HTMLElement>(`[data-card="${CSS.escape(drag.id)}"]`);
    if (slot) {
      const r = slot.getBoundingClientRect();
      drag.dropping = true;
      drag.x = r.left;
      drag.y = r.top;
    }
    setTimeout(() => (drag = null), ms('normal'));
  }
</script>

<div class="board">
  {#each columns as column (column.id)}
    <section class="column" data-column={column.id} data-target={drag && column.items.some((i) => i.id === drag?.id) ? true : undefined}>
      <header class="header">
        <GroupPill
          status={column.status}
          glyph={column.glyph}
          label={column.label}
          collapsed={collapsed[column.id]}
          onclick={() => (collapsed[column.id] = !collapsed[column.id])}
        />
        <Count value={column.items.length} />
        <span class="grow"></span>
        <IconButton icon="plus" label={`Add to ${column.label}`} onclick={() => onadd?.(column.id)} />
      </header>
      <div class="cards" data-cards data-collapsed={collapsed[column.id] || undefined}>
        {#each column.items as item (item.id)}
          <div
            class="slot"
            data-card={item.id}
            role="listitem"
            onpointerdown={(e) => down(e, item)}
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
    min-height: 100%;
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
  .cards {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-height: var(--space-20);
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
    touch-action: none;
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
