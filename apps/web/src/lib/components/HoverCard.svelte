<script lang="ts">
  import type { Snippet } from 'svelte';
  import { pop } from '../motion';
  let { anchor, children }: { anchor: HTMLElement | null; children: Snippet } = $props();
  let panel: HTMLElement | undefined = $state();
  let x = $state(0);
  let y = $state(0);
  let placed = $state(false);

  $effect(() => {
    if (!anchor || !panel) {
      placed = false;
      return;
    }
    const r = anchor.getBoundingClientRect();
    const h = panel.offsetHeight;
    const gap = parseFloat(getComputedStyle(panel).fontSize) * 0.5;
    const below = r.bottom + gap + h < window.innerHeight;
    y = below ? r.bottom + gap : r.top - gap - h;
    x = Math.max(gap, Math.min(r.left + gap * 4, window.innerWidth - panel.offsetWidth - gap));
    if (!placed) requestAnimationFrame(() => (placed = true));
  });
</script>

{#if anchor}
  <div class="hovercard" data-placed={placed || undefined} bind:this={panel} style:--x={`${x}px`} style:--y={`${y}px`} transition:pop={{ y: 0.375 }}>
    {@render children()}
  </div>
{/if}

<style>
  .hovercard {
    position: fixed;
    top: 0;
    left: 0;
    translate: var(--x) var(--y);
    width: var(--hovercard-width);
    z-index: var(--z-popover);
    pointer-events: none;
  }
  [data-placed] {
    transition: translate var(--duration-normal) var(--ease-out);
  }
</style>
