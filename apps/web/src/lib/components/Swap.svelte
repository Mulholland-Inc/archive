<script lang="ts">
  import type { Snippet } from 'svelte';
  import { ms, ease } from '../motion';
  let { key, direction = 0, children }: { key: unknown; direction?: number; children: Snippet } = $props();

  function enter(_node: Element) {
    const d = direction;
    return {
      duration: ms('slow'),
      easing: ease('out'),
      css: (t: number) => `opacity:${t};transform:translateX(${(1 - t) * d * 1.5}rem)`
    };
  }
</script>

{#key key}
  <div class="swap" in:enter>{@render children()}</div>
{/key}

<style>
  .swap {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
</style>
