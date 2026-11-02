<script>
  import { LoaderCircle, Check, Hand, X, Circle, Pause, ChevronUp } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { steps = [], running = true, open = $bindable(false), label = 'Steps' } = $props();

  const done = $derived(steps.filter((s) => s.status === 'done').length);
  const current = $derived(
    steps.find((s) => ['running', 'waiting', 'failed'].includes(s.status)) ?? steps.find((s) => s.status === 'pending') ?? steps.at(-1)
  );
  const words = { running: 'Running', waiting: 'Needs input', failed: 'Failed', pending: '' };

  function onkeydown(event) {
    if (open && event.key === 'Escape' && !event.defaultPrevented) open = false;
  }
</script>

<svelte:window {onkeydown} />

{#snippet glyph(status)}
  <span class={['glyph', status, { spin: status === 'running' && running }]}>
    <Icon icon={status === 'done' ? Check : status === 'waiting' ? Hand : status === 'failed' ? X : status === 'running' ? (running ? LoaderCircle : Pause) : Circle} size="sm" />
  </span>
{/snippet}

{#if current}
  <section class={['progress', { open }]} aria-label={label}>
    <div class="panel" inert={!open}>
      <div class="clip">
        <ol class="list">
          {#each steps as step (step.id)}
            <li class={['row', { current: step === current && done < steps.length }]}>
              {@render glyph(step.status)}
              <span class={['name', step.status]}>{step.label}</span>
              <span class="time">{step.time || words[step.status] || ''}</span>
            </li>
          {/each}
        </ol>
      </div>
    </div>
    <button type="button" class="bar" aria-expanded={open} onclick={() => (open = !open)}>
      {@render glyph(current.status)}
      <span class="label">{current.label}</span>
      <span class="count">{done} / {steps.length}</span>
      <span class="chevron"><Icon icon={ChevronUp} size="sm" /></span>
    </button>
  </section>
{/if}

<style>
  .progress {
    display: flex;
    flex-direction: column;
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: var(--ring);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .progress:hover {
    box-shadow: var(--ring-strong);
  }

  .panel {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows var(--duration-slow) var(--ease-out);
  }

  .open .panel {
    grid-template-rows: 1fr;
  }

  .clip {
    min-height: 0;
    overflow: hidden;
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    max-height: var(--composer-max);
    overflow-y: auto;
    padding: var(--space-2);
    padding-block-end: 0;
    opacity: 0;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .open .list {
    opacity: 1;
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--control-md);
    padding-inline: var(--space-2);
    border-radius: var(--radius-md);
  }

  .row.current {
    background: var(--color-fill);
  }

  .name {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    font-size: var(--text-md);
  }

  .name.pending {
    color: var(--color-text-tertiary);
  }

  .time {
    flex: none;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    height: var(--control-xl);
    padding-inline: var(--space-4);
    border-radius: var(--radius-xl);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    text-align: start;
  }

  .glyph {
    display: inline-flex;
    flex: none;
    color: var(--color-text-secondary);
  }

  .glyph.pending {
    color: var(--color-text-disabled);
  }

  .glyph.failed {
    color: var(--color-danger);
  }

  .glyph.waiting {
    color: var(--color-text);
  }

  .spin {
    animation: spin var(--duration-spin) linear infinite;
  }

  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    color: var(--color-text);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  .chevron {
    display: inline-flex;
    color: var(--color-text-tertiary);
    transition: transform var(--duration-slow) var(--ease-out);
  }

  .open .chevron {
    transform: rotate(0.5turn);
  }

  @keyframes spin {
    to {
      transform: rotate(1turn);
    }
  }
</style>
