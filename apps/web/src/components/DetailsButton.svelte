<script>
  import { PanelRight } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import { app } from '../lib/app.svelte.js';
</script>

<span class="mobile">
  <IconButton icon={PanelRight} label="Details" tooltip={false} pressed={app.details} onclick={() => (app.details = true)} />
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
