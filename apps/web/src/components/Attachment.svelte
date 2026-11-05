<script>
  import { X } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import { fileArt } from '../lib/live.js';

  let { name, kind = 'document', size, onremove, onopen, row = false } = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<svelte:element this={onopen ? 'button' : 'span'} type={onopen ? 'button' : undefined} class={['att', { row }]} onclick={onopen}>
  <img class="thumb" src={fileArt(name, kind)} alt="" />
  <span class="meta">
    <span class="name">{name}</span>
    {#if size}<span class="size">{size}</span>{/if}
  </span>
  {#if onremove}
    <button type="button" class="remove" aria-label={`Remove ${name}`} onclick={onremove}><Icon icon={X} size="xs" /></button>
  {/if}
</svelte:element>

<style>
  .att {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    max-width: min(var(--col-lg), 100%);
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  button.att {
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  button.att:hover {
    background: var(--color-fill);
  }

  .row {
    display: flex;
    width: 100%;
    max-width: none;
    padding: var(--space-2) var(--space-3) var(--space-2) var(--space-2);
    border-radius: 0;
    background: transparent;
    box-shadow: none;
  }

  .row:not(:last-child) {
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  button.row:hover {
    background: var(--color-fill-subtle);
  }

  .row .thumb {
    width: var(--control-md);
    height: var(--control-md);
  }

  .row .meta {
    flex: 1;
    flex-direction: row;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    padding-inline-end: 0;
  }

  .row .size {
    flex: none;
    font-size: var(--text-sm);
  }

  .thumb {
    flex: none;
    width: var(--control-lg);
    height: var(--control-lg);
  }

  .meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
    padding-inline-end: var(--space-2);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .size {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .remove {
    position: absolute;
    inset-block-start: calc(var(--space-1-5) * -1);
    inset-inline-end: calc(var(--space-1-5) * -1);
    display: grid;
    place-items: center;
    width: var(--avatar-xs);
    height: var(--avatar-xs);
    border-radius: var(--radius-full);
    background: var(--color-inverse);
    color: var(--color-text-inverse);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .att:hover .remove,
  .remove:focus-visible {
    opacity: 1;
  }
</style>
