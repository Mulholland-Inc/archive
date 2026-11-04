<script>
  import { getContext } from 'svelte';
  import Art from './Art.svelte';
  import { app, onboardingSteps } from '../lib/app.svelte.js';
  import { href } from '../lib/router.svelte.js';

  const sidebar = getContext('sidebar');
  const next = $derived(onboardingSteps.find((s) => !app.onboarding.includes(s.id)));
</script>

{#if !sidebar?.collapsed && next}
  <div class="card">
    <div class="art"><Art name="onboarding" /></div>
    <div class="text">
      <a class="title" href={href('/onboarding')}>Finish setting up</a>
      <span class="next">{next.title}</span>
    </div>
  </div>
{/if}

<style>
  .card {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-2-5);
    padding-inline: var(--space-3);
    border-radius: var(--radius-lg);
  }

  .art {
    display: flex;
    flex: none;
  }

  .art :global(.art) {
    height: var(--space-8);
  }

  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: var(--leading-snug);
  }

  .title {
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    border-radius: var(--radius-xs);
  }

  .title::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
  }

  .next {
    overflow: hidden;
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
