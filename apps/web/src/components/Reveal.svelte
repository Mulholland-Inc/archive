<script>
  import { rise } from '../lib/motion.js';

  let { children } = $props();
</script>

<div class="reveal" in:rise={{ shift: '--shift-sm' }}>{@render children()}</div>

<style>
  .reveal {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    min-width: 0;
  }
</style>
