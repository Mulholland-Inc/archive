<script>
  import VaultIcon from './VaultIcon.svelte';
  import { ws } from '../lib/app.svelte.js';
  import Button from './Button.svelte';
  import { app } from '../lib/app.svelte.js';
  import { fade } from '../lib/motion.js';
</script>

<div class="screen" transition:fade={{ speed: 'normal' }}>
  <div class="card">
    <VaultIcon icon={ws().look.icon} tint={ws().look.tint} seed={ws().id} size="xl" greet />
    <h1>You’re signed out</h1>
    <Button variant="primary" size="lg" onclick={() => (app.signedOut = false)}>Sign back in as {app.profile.name}</Button>
  </div>
</div>

<style>
  .screen {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    display: grid;
    place-items: center;
    padding: var(--space-6);
    background: var(--color-canvas);
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-5);
    text-align: center;
  }

  h1 {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
  }
</style>
