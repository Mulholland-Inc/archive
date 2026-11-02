<script>
  import StatusBadge from './StatusBadge.svelte';
  import WorkflowActions from './WorkflowActions.svelte';
  import { contextmenu } from '../lib/contextmenu.svelte.js';
  import { workflowItems } from '../lib/menus.js';

  let { workflow, status, onopen, ondelete } = $props();
</script>

<li class="item" {@attach contextmenu(() => workflowItems(workflow, { ondelete }))}>
  <button type="button" class="open" onclick={onopen}>
    <span class="title">{workflow.name}</span>
    <span class="meta">
      <StatusBadge {status} plain />
      <span>{workflow.runs[0]?.started ?? 'Never'}</span>
    </span>
  </button>
  <WorkflowActions {workflow} {ondelete} />
</li>

<style>
  .item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding-block: var(--space-3);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .item:last-child {
    box-shadow: none;
  }

  .open {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1;
    min-width: 0;
    text-align: start;
  }

  .title {
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    overflow-wrap: anywhere;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
