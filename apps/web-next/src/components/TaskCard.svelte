<script>
  import { Bot } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import ProgressBar from './ProgressBar.svelte';

  let { task, href, active = false } = $props();
  const done = $derived(task.steps.filter((s) => s.status === 'done').length);
  const current = $derived(task.steps.find((s) => s.status === 'running' || s.status === 'waiting' || s.status === 'failed'));
  const tone = $derived(task.status === 'failed' ? 'danger' : task.status === 'completed' ? 'success' : task.status === 'scheduled' ? 'neutral' : 'accent');
</script>

<a {href} class={['task', { active }]}>
  <div class="top">
    <span class="agent"><Icon icon={Bot} size="sm" />{task.agent}</span>
    <StatusBadge status={task.status} />
  </div>
  <div class="title">{task.title}</div>
  <div class="now">{current?.label ?? (task.status === 'scheduled' ? `Starts ${task.started}` : `Finished · ${task.elapsed}`)}</div>
  <div class="foot">
    <ProgressBar value={task.progress} {tone} size="sm" />
    <span class="count">{done}/{task.steps.length}</span>
  </div>
</a>

<style>
  .task {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-4);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    transition: box-shadow var(--duration-normal) var(--ease-out), transform var(--duration-normal) var(--ease-out);
  }

  .task:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong), var(--shadow-sm);
  }

  .active {
    box-shadow: inset 0 0 0 var(--focus-width) var(--color-accent);
  }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .agent {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  .title {
    font-size: var(--text-base);
    font-weight: var(--weight-medium);
    line-height: var(--leading-snug);
    text-wrap: pretty;
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
    padding-block-start: var(--space-2);
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }
</style>
