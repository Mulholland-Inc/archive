<script>
  import StatusBadge from './StatusBadge.svelte';
  import ProgressBar from './ProgressBar.svelte';
  import TriggerLabel from './TriggerLabel.svelte';

  let { workflow, href } = $props();
  const run = $derived(workflow.runs[0]);
  const done = $derived(run ? run.steps.filter((s) => s.status === 'done').length : 0);
  const status = $derived(!workflow.enabled ? 'off' : run ? (run.paused ? 'paused' : run.status) : 'idle');
  const live = $derived(run && (run.status === 'running' || run.status === 'waiting'));
  const current = $derived(run?.steps.find((s) => ['running', 'waiting', 'failed'].includes(s.status)));
  const tone = $derived(run?.status === 'failed' ? 'danger' : run?.status === 'succeeded' ? 'success' : run?.status === 'stopped' ? 'neutral' : 'accent');
</script>

<a {href} class={['card', { off: !workflow.enabled }]}>
  <div class="top">
    <span class="name">{workflow.name}</span>
    <StatusBadge {status} />
  </div>
  <TriggerLabel trigger={workflow.trigger} />
  <div class="now">
    {#if live}{current?.label}{:else if run}Run #{run.number} · {run.started}{:else}No runs yet{/if}
    {#if workflow.next && workflow.enabled} · next {workflow.next}{/if}
  </div>
  {#if run}
    <div class="foot">
      <ProgressBar value={done / run.steps.length} {tone} size="sm" />
      <span class="count">{done}/{run.steps.length}</span>
    </div>
  {/if}
</a>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-4);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    transition: box-shadow var(--duration-normal) var(--ease-out), opacity var(--duration-normal) var(--ease-out);
  }

  .card:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong), var(--ring);
  }

  .off {
    opacity: 0.7;
  }

  .top {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .name {
    font-size: var(--text-base);
    font-weight: var(--weight-medium);
    line-height: var(--leading-snug);
  }

  .now {
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .foot {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding-block-start: var(--space-1);
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }
</style>
