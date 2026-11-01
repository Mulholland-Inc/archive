<script>
  import { Plus, Check, BadgeCheck } from '@lucide/svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import IconButton from './IconButton.svelte';
  import Icon from './Icon.svelte';

  let { connector, onopen, onconnect } = $props();
</script>

<article class="card">
  <button type="button" class="hit" aria-label={`${connector.name} details`} onclick={onopen}></button>
  <span class="logo"><ConnectorIcon id={connector.brand ?? connector.id} /></span>
  <div class="text">
    <span class="name">{connector.name}<span class="verified" role="img" aria-label="Verified"><Icon icon={BadgeCheck} size="sm" /></span></span>
    <p class="description">{connector.connected ? connector.account : connector.description}</p>
  </div>
  {#if connector.connected}
    <span class="done" role="img" aria-label="Connected"><Icon icon={Check} size="md" /></span>
  {:else if onconnect}
    <span class="action"><IconButton icon={Plus} label={`Connect ${connector.name}`} variant="subtle" tooltip={false} onclick={onconnect} /></span>
  {/if}
</article>

<style>
  .card {
    position: relative;
    display: flex;
    align-items: flex-start;
    gap: var(--space-4);
    min-width: 0;
    padding: var(--space-4);
    border-radius: var(--radius-lg);
    background: var(--color-fill-subtle);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .card:hover {
    background: var(--color-fill);
  }

  .hit {
    position: absolute;
    inset: 0;
    border-radius: inherit;
  }

  .logo {
    display: grid;
    place-items: center;
    flex: none;
  }

  .logo :global(.tile) {
    width: var(--icon-2xl);
    height: var(--icon-2xl);
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1;
    min-width: 0;
  }

  .name {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    min-width: 0;
    font-size: var(--text-base);
    font-weight: var(--weight-medium);
    line-height: var(--leading-tight);
  }

  .verified {
    display: flex;
    color: var(--color-text-secondary);
  }

  .description {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
    overflow-wrap: anywhere;
    color: var(--color-text-secondary);
    font-size: var(--text-md);
  }

  .action {
    position: relative;
    z-index: var(--z-raised);
  }

  .done {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-md);
    height: var(--control-md);
    border-radius: var(--radius-md);
    color: var(--color-success);
  }
</style>
