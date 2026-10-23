<script lang="ts">
  import type { Snippet } from 'svelte';
  import { ms, ease } from '../motion';
  let { open = $bindable(false), label, children, onclose }: { open?: boolean; label: string; children: Snippet; onclose?: () => void } = $props();

  let panel: HTMLElement | undefined = $state();

  function close() {
    open = false;
    onclose?.();
  }

  function slide(_node: Element) {
    return { duration: ms('normal'), easing: ease('out'), css: (t: number) => `opacity:${t};transform:translateX(${(1 - t) * 1.5}rem)` };
  }

  $effect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !e.defaultPrevented && !document.querySelector('[aria-modal="true"]')) {
        e.preventDefault();
        close();
      }
    };
    const onDown = (e: PointerEvent) => {
      const t = e.target as Element;
      if (!panel || panel.contains(t) || !t.isConnected) return;
      if (t.closest('[role="dialog"], [role="presentation"], [data-clickable], a[href^="/records/"]')) return;
      close();
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('pointerdown', onDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('pointerdown', onDown);
    };
  });

</script>

{#if open}
  <aside class="sheet" aria-label={label} bind:this={panel} transition:slide>
    {@render children()}
  </aside>
{/if}

<style>
  .sheet {
    position: fixed;
    top: var(--space-2);
    right: var(--space-2);
    bottom: var(--space-2);
    z-index: calc(var(--z-dialog) - 5);
    display: flex;
    flex-direction: column;
    width: min(26rem, calc(100vw - var(--space-4)));
    border-radius: var(--radius-xl);
    background: var(--card);
    color: var(--card-foreground);
    box-shadow: var(--shadow-lift);
    overflow: hidden;
  }
</style>
