<script>
  import VaultIcon from './VaultIcon.svelte';
  import { rise } from '../lib/motion.js';

  let { name, look, seed } = $props();
  const hour = new Date().getHours();
  const part = hour < 5 ? 'Up late' : hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
</script>

<div class="greeting" in:rise={{ speed: 'slower' }}>
  {#if look}
    {#key seed}
      <span class="icon" in:rise={{ speed: 'slow' }}><VaultIcon icon={look.icon} tint={look.tint} {seed} size="xxl" greet /></span>
    {/key}
  {/if}
  <h1>{part}, {name}</h1>
</div>

<style>
  .greeting {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    text-align: center;
  }

  .icon {
    display: inline-flex;
  }

  h1 {
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
    color: var(--color-text);
  }

  @media (max-width: 40rem) {
    h1 {
      font-size: var(--text-2xl);
    }
  }
</style>
