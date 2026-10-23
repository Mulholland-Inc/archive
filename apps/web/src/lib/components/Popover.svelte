<script lang="ts">
  import type { Snippet } from 'svelte';
  import { pop } from '../motion';
  let {
    open = $bindable(),
    align = 'start',
    block = false,
    trigger,
    children,
    onclose
  }: {
    open?: boolean | undefined;
    align?: 'start' | 'end';
    block?: boolean;
    trigger: Snippet<[{ toggle: () => void; open: boolean }]>;
    children: Snippet<[{ close: () => void }]>;
    onclose?: () => void;
  } = $props();

  let anchor: HTMLElement;
  let panel: HTMLElement | undefined = $state();
  let x = $state(0);
  let y = $state(0);
  let up = $state(false);
  let width = $state(0);
  let placed = $state(false);

  function place() {
    if (!anchor || !panel) return;
    const r = anchor.getBoundingClientRect();
    width = r.width;
    const p = { width: panel.offsetWidth, height: panel.offsetHeight };
    const gap = parseFloat(getComputedStyle(anchor).fontSize) * 0.375;
    up = r.bottom + gap + p.height > window.innerHeight - gap && r.top - gap - p.height > 0;
    y = up ? r.top - gap - p.height : r.bottom + gap;
    const left = align === 'end' ? r.right - p.width : r.left;
    x = Math.max(gap, Math.min(left, window.innerWidth - p.width - gap));
  }

  function close() {
    if (!open) return;
    open = false;
    onclose?.();
  }

  function toggle() {
    if (open) close();
    else open = true;
  }

  $effect(() => {
    if (!open || !panel) {
      placed = false;
      return;
    }
    place();
    const raf = requestAnimationFrame(() => (placed = true));
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (!panel?.contains(t) && !anchor.contains(t)) close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        close();
        (anchor.querySelector('button,[tabindex]') as HTMLElement | null)?.focus();
      }
    };
    const ro = new ResizeObserver(place);
    if (anchor.parentElement) ro.observe(anchor.parentElement);
    window.addEventListener('pointerdown', onDown, true);
    window.addEventListener('keydown', onKey, true);
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => {
      ro.disconnect();
      cancelAnimationFrame(raf);
      window.removeEventListener('pointerdown', onDown, true);
      window.removeEventListener('keydown', onKey, true);
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    };
  });
</script>

<span class="anchor" data-block={block || undefined} bind:this={anchor}>{@render trigger({ toggle, open: !!open })}</span>
{#if open}
  <div
    class="panel"
    bind:this={panel}
    data-up={up || undefined}
    data-placed={placed || undefined}
    data-align={align}
    style:--x={`${x}px`}
    style:--y={`${y}px`}
    style:--anchor-width={`${width}px`}
    transition:pop={{ y: up ? 0.25 : -0.25 }}
  >
    {@render children({ close })}
  </div>
{/if}

<style>
  .anchor {
    display: inline-flex;
    min-width: 0;
  }
  .panel[data-placed] {
    transition: translate var(--duration-normal) var(--ease-out);
  }
  .anchor[data-block] {
    display: flex;
    width: 100%;
  }
  .panel {
    position: fixed;
    top: 0;
    left: 0;
    translate: var(--x) var(--y);
    z-index: var(--z-popover);
    transform-origin: top left;
  }
  [data-align='end'] {
    transform-origin: top right;
  }
  [data-up] {
    transform-origin: bottom left;
  }
</style>
