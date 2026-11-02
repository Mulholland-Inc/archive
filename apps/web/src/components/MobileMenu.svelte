<script>
  import { Menu as MenuIcon } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import { app } from '../lib/app.svelte.js';
</script>

<span class="mobile">
  <IconButton icon={MenuIcon} label="Open sidebar" tooltip={false} onclick={() => (app.drawer = true)} />
</span>

<style>
  .mobile {
    display: none;
  }

  @media (max-width: 56rem) {
    .mobile {
      display: inline-flex;
    }
  }
</style>
