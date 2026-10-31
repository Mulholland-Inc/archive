<script>
  import { FileText, Image, AudioLines, FileSpreadsheet, X } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { name, kind = 'document', size, onremove, tone = 1, href } = $props();
  const icons = { image: Image, recording: AudioLines, spreadsheet: FileSpreadsheet, pdf: FileText, document: FileText };
</script>

<svelte:element this={href ? 'a' : 'span'} {href} class={['att', kind, { link: href }]}>
  <span class={['thumb', `t${tone}`]}><Icon icon={icons[kind] ?? FileText} size="md" /></span>
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
    max-width: var(--col-lg);
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .link {
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .link:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .thumb {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-lg);
    height: var(--control-lg);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
  }

  .t1 { background: var(--color-thumb-1); }
  .t2 { background: var(--color-thumb-2); }
  .t3 { background: var(--color-thumb-3); }
  .t4 { background: var(--color-thumb-4); }

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
