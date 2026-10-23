<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let { tone = 'info', children, action }: { tone?: 'info' | 'warning' | 'danger' | 'success'; children: Snippet; action?: Snippet } = $props();
  const icon = $derived(tone === 'danger' || tone === 'warning' ? 'alert' : tone === 'success' ? 'check' : 'bell');
</script>

<div class="notice" data-tone={tone} role={tone === 'danger' ? 'alert' : 'status'}>
  <span class="icon"><Icon name={icon} /></span>
  <div class="text">{@render children()}</div>
  {#if action}<div class="action">{@render action()}</div>{/if}
</div>

<style>
  .notice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2-5);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--tint-1);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
  }
  .icon {
    display: grid;
    padding-top: var(--space-0-5);
    color: var(--muted-foreground);
  }
  [data-tone='warning'] .icon {
    color: var(--warning);
  }
  [data-tone='danger'] .icon {
    color: var(--danger);
  }
  [data-tone='success'] .icon {
    color: var(--success);
  }
  .text {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .action {
    display: flex;
    gap: var(--space-2);
    margin: calc(var(--space-1) * -1) 0;
  }
</style>
