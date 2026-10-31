<script>
  import { Check, LoaderCircle, CircleAlert, Hand } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import Disclosure from './Disclosure.svelte';
  import { rise } from '../lib/motion.js';

  let { steps = [], running = false, worked, open = $bindable(false) } = $props();

  const heading = $derived(
    running
      ? (steps.find((s) => s.status === 'running' || s.status === 'waiting')?.label ?? steps.at(-1)?.label ?? 'Thinking')
      : worked
        ? `Worked for ${worked}`
        : steps.some((s) => s.status === 'waiting')
          ? 'Waiting for you'
          : `${steps.filter((s) => s.status === 'done').length} of ${steps.length} steps done`
  );
</script>

<Disclosure bind:open>
  {#snippet summary()}
    {#if running}<span class="spin"><Icon icon={LoaderCircle} size="sm" /></span>{/if}
    <span class={['label', { shimmer: running }]}>{heading}</span>
  {/snippet}
  <ol class="steps">
    {#each steps as step (step.id)}
      <li class={['step', step.status]} in:rise={{ shift: '--shift-sm' }}>
        <span class="glyph">
          {#if step.status === 'running'}
            <span class="spin"><Icon icon={LoaderCircle} size="sm" /></span>
          {:else if step.status === 'failed'}
            <Icon icon={CircleAlert} size="sm" />
          {:else if step.status === 'waiting'}
            <Icon icon={Hand} size="sm" />
          {:else if step.status === 'done'}
            <Icon icon={Check} size="sm" />
          {/if}
        </span>
        <span class="body">
          <span class="title">{step.label}</span>
          {#if step.detail}<span class="detail">{step.detail}</span>{/if}
        </span>
        {#if step.time}<span class="time">{step.time}</span>{/if}
      </li>
    {/each}
  </ol>
</Disclosure>

<style>
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .shimmer {
    background: linear-gradient(90deg, var(--color-text-tertiary) 0%, var(--color-text) 50%, var(--color-text-tertiary) 100%);
    background-size: 200% 100%;
    background-clip: text;
    -webkit-background-clip: text;
    color: transparent;
    animation: shimmer calc(var(--duration-slower) * 5) linear infinite;
  }

  .spin {
    display: inline-flex;
    animation: spin calc(var(--duration-slower) * 2) linear infinite;
  }

  .steps {
    display: flex;
    flex-direction: column;
    gap: var(--space-0);
    padding-block-start: var(--space-3);
  }

  .step {
    position: relative;
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding-block-end: var(--space-3);
  }

  .step:not(:last-child)::before {
    content: '';
    position: absolute;
    inset-block: var(--control-xs) 0;
    inset-inline-start: calc(var(--control-xs) / 2);
    width: var(--border-width);
    background: var(--color-border);
  }

  .glyph {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-xs);
    height: var(--control-xs);
    border-radius: var(--radius-full);
    background: var(--color-fill);
    color: var(--color-text-secondary);
  }

  .running .glyph {
    background: var(--color-accent-subtle);
    color: var(--color-accent-text);
  }

  .failed .glyph {
    background: var(--color-danger-subtle);
    color: var(--color-danger);
  }

  .waiting .glyph {
    background: var(--color-warning-subtle);
    color: var(--color-warning);
  }

  .pending .glyph {
    background: none;
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
    color: var(--color-text-disabled);
  }

  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    flex: 1;
    min-width: 0;
    padding-block-start: var(--space-0-5);
  }

  .title {
    font-size: var(--text-md);
    color: var(--color-text);
  }

  .pending .title {
    color: var(--color-text-tertiary);
  }

  .detail {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  .time {
    padding-block-start: var(--space-0-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  @keyframes spin {
    to {
      transform: rotate(1turn);
    }
  }

  @keyframes shimmer {
    from {
      background-position: 100% 0;
    }
    to {
      background-position: -100% 0;
    }
  }
</style>
