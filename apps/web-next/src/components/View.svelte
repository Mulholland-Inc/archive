<script>
  import { rise } from '../lib/motion.js';

  let { children } = $props();
</script>

<div class="view" in:rise={{ shift: '--shift-sm', speed: 'slow' }}>
  {@render children()}
</div>

<style>
  .view {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }
</style>
