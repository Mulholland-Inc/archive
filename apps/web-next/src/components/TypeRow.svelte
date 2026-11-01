<script>
  import { ChevronRight } from '@lucide/svelte';
  import TypeArt from './TypeArt.svelte';
  import Chip from './Chip.svelte';
  import Icon from './Icon.svelte';

  let { type, links = [], onclick, max = 3 } = $props();
</script>

<button type="button" class="row" role="listitem" {onclick}>
  <TypeArt icon={type.icon} size="xs" />
  <span class="name">{type.name}</span>
  <span class="num">{type.rows.toLocaleString('en-US')}</span>
  <span class="num wide">{type.columns.length}</span>
  <span class="links wide">
    {#each links.slice(0, max) as name (name)}<Chip size="sm">{name}</Chip>{/each}
    {#if links.length > max}<Chip size="sm">+{links.length - max}</Chip>{/if}
  </span>
  <span class="muted wide">{type.updated}</span>
  <span class="chevron"><Icon icon={ChevronRight} size="sm" /></span>
</button>

<style>
  .row {
    display: grid;
    grid-template-columns: var(--type-columns);
    align-items: center;
    gap: var(--space-4);
    width: 100%;
    min-height: var(--control-2xl);
    padding-inline: var(--space-2);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .row:hover {
    background: var(--color-fill-subtle);
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: var(--weight-medium);
  }

  .num {
    color: var(--color-text-secondary);
    font-variant-numeric: tabular-nums;
    text-align: end;
  }

  .links {
    display: flex;
    gap: var(--space-1);
    min-width: 0;
    overflow: hidden;
  }

  .muted {
    color: var(--color-text-secondary);
    white-space: nowrap;
  }

  .chevron {
    display: flex;
    color: var(--color-text-tertiary);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .row:hover .chevron,
  .row:focus-visible .chevron {
    opacity: 1;
  }

  @media (max-width: 40rem) {
    .row {
      grid-template-columns: var(--type-columns-narrow);
    }

    .wide {
      display: none;
    }

    .chevron {
      opacity: 1;
    }
  }
</style>
