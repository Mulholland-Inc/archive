<script>
  import { getContext } from 'svelte';
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import { app, onboardingSteps } from '../lib/app.svelte.js';
  import { href } from '../lib/router.svelte.js';

  const sidebar = getContext('sidebar');
  const done = $derived(onboardingSteps.filter((s) => app.onboarding.includes(s.id)).length);
  const total = onboardingSteps.length;
</script>

{#if !sidebar?.collapsed && done < total}
  <div class="card">
    <div class="head">
      <div class="art"><Art name="onboarding" /></div>
      <div class="progress">
        <a class="title" href={href('/onboarding')}>Finish setting up</a>
        <span class="count">{done} of {total} done</span>
        <ProgressBar value={done / total} size="sm" label="Setup progress" />
      </div>
    </div>
    <Button size="sm" variant="primary" full href={href('/onboarding')} tabindex="-1">Continue</Button>
  </div>
{/if}

<style>
  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-canvas);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .card:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .art {
    display: flex;
    flex: none;
  }

  .art :global(.art) {
    height: var(--space-12);
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

  .progress {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }

  .count {
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .card > :global(.btn) {
    position: relative;
  }
</style>
