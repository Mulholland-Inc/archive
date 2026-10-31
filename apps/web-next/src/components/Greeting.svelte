<script>
  import Mark from './Mark.svelte';
  import { rise } from '../lib/motion.js';

  let { name } = $props();
  const hour = new Date().getHours();
  const part = hour < 5 ? 'Up late' : hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
</script>

<h1 class="greeting" in:rise={{ speed: 'slower' }}>
  <span class="mark"><Mark size="lg" /></span>
  <span>{part}, {name}</span>
</h1>

<style>
  .greeting {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-tight);
    color: var(--color-text);
    text-align: center;
  }

  .mark {
    display: inline-flex;
    color: var(--color-accent);
  }

  @media (max-width: 40rem) {
    .greeting {
      font-size: var(--text-2xl);
    }
  }
</style>
