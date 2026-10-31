<script>
  import ConnectorIcon from './ConnectorIcon.svelte';
  import Badge from './Badge.svelte';
  import Button from './Button.svelte';

  let { connector, onopen, onconnect } = $props();
</script>

<article class="card">
  <button type="button" class="hit" aria-label={`${connector.name} details`} onclick={onopen}></button>
  <ConnectorIcon id={connector.id} />
  <div class="text">
    <span class="name">{connector.name}</span>
    <span class="meta">{connector.connected ? connector.account : connector.category}</span>
  </div>
  {#if connector.connected}
    <Badge tone="success" dot>Connected</Badge>
  {:else}
    <span class="action"><Button size="sm" onclick={onconnect}>Connect</Button></span>
  {/if}
</article>

<style>
  .card {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    padding: var(--space-3);
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

  .action {
    position: relative;
    z-index: var(--z-raised);
  }

  .text {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }

  .name {
    font-weight: var(--weight-medium);
  }

  .meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
