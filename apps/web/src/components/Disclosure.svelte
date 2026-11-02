<script>
  import { ChevronRight } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { slide } from '../lib/motion.js';

  let { open = $bindable(false), summary, children } = $props();
</script>

<div class={['disclosure', { open }]}>
  <button type="button" class="summary" aria-expanded={open} onclick={() => (open = !open)}>
    {@render summary()}
    <span class="chev"><Icon icon={ChevronRight} size="sm" /></span>
  </button>
  {#if open}
    <div class="content" transition:slide>
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .disclosure {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .summary {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    align-self: flex-start;
    max-width: 100%;
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    border-radius: var(--radius-sm);
    transition: color var(--duration-fast) var(--ease-out);
  }

  .summary:hover {
    color: var(--color-text);
  }

  .chev {
    display: inline-flex;
    transition: transform var(--duration-normal) var(--ease-out);
  }

  .open .chev {
    transform: rotate(var(--turn-quarter));
  }

  .content {
    min-width: 0;
  }
</style>
