<script>
  import { getContext } from 'svelte';
  import Art from './Art.svelte';
  import { onboardingSteps, onboards, stepDone } from '../lib/app.svelte.js';
  import { href } from '../lib/router.svelte.js';

  const sidebar = getContext('sidebar');
  const next = $derived(onboardingSteps.find((s) => !stepDone(s.id)));
</script>

{#if !sidebar?.collapsed && next && onboards()}
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
    margin-block: var(--space-0-5) calc(var(--space-1-5) * -1);
    padding: var(--space-1-5) var(--space-3);
    border-radius: var(--radius-md);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .card:hover,
  .card:focus-within {
    background: var(--color-fill);
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
