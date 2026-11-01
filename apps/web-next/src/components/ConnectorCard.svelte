<script>
  import { Plus, Check } from '@lucide/svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import IconButton from './IconButton.svelte';
  import Icon from './Icon.svelte';

  let { connector, onopen, onconnect } = $props();
</script>

<article class="card">
  <button type="button" class="hit" aria-label={`${connector.name} details`} onclick={onopen}></button>
  <div class="top">
    <ConnectorIcon id={connector.id} />
    <div class="title">
      <span class="name">{connector.name}</span>
      <span class="tag">{connector.category}</span>
    </div>
    {#if connector.connected}
      <span class="done" role="img" aria-label="Connected"><Icon icon={Check} size="md" /></span>
    {:else}
      <span class="action"><IconButton icon={Plus} label={`Connect ${connector.name}`} size="sm" variant="ghost" tooltip={false} onclick={onconnect} /></span>
    {/if}
  </div>
  <p class="description">{connector.connected ? connector.account : connector.description}</p>
</article>

<style>
  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
    padding: var(--space-4);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    transition: box-shadow var(--duration-normal) var(--ease-out);
  }

  .card:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong), var(--ring);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
  }

  .top {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }

  .title {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--space-0-5) var(--space-2);
    flex: 1;
    min-width: 0;
  }

  .name {
    font-weight: var(--weight-medium);
  }

  .tag {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .action {
    position: relative;
    z-index: var(--z-raised);
  }

  .done {
    display: flex;
    align-items: center;
    justify-content: center;
    width: var(--control-sm);
    height: var(--control-sm);
    color: var(--color-success);
  }

  .description {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
  }
</style>
