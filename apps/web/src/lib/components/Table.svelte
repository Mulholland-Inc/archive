<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';
  let {
    columns,
    rows,
    key,
    cell,
    onrowclick,
    onrowhover,
    empty
  }: {
    columns: { id: string; label: string; width?: 'narrow' | 'wide' | 'auto' }[];
    rows: T[];
    key: (row: T) => string;
    cell: Snippet<[T, string]>;
    onrowclick?: (row: T, e: MouseEvent) => void;
    onrowhover?: (row: T) => void;
    empty?: Snippet;
  } = $props();
</script>

<div class="wrap">
  <table class="table">
    <thead>
      <tr>
        {#each columns as c (c.id)}<th data-width={c.width ?? 'auto'} scope="col">{c.label}</th>{/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as row (key(row))}
        <tr
          data-clickable={onrowclick ? true : undefined}
          tabindex={onrowclick ? 0 : undefined}
          onclick={(e) => {
            if ((e.target as HTMLElement).closest('a,button,input,label')) return;
            onrowclick?.(row, e);
          }}
          onkeydown={(e) => e.key === 'Enter' && onrowclick?.(row, e as unknown as MouseEvent)}
          onpointerenter={() => onrowhover?.(row)}
        >
          {#each columns as c (c.id)}<td data-width={c.width ?? 'auto'}>{@render cell(row, c.id)}</td>{/each}
        </tr>
      {/each}
    </tbody>
  </table>
  {#if !rows.length && empty}<div class="empty">{@render empty()}</div>{/if}
</div>

<style>
  .wrap {
    min-width: 0;
    min-height: 0;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    border-radius: var(--radius-lg);
    background: var(--card);
    border: var(--border-width) solid var(--border);
  }
  .table {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0;
    font-size: var(--text-sm);
  }
  th {
    position: sticky;
    top: 0;
    height: var(--table-row-height);
    padding: 0 var(--space-2-5);
    text-align: start;
    font-weight: var(--font-medium);
    color: var(--muted-foreground);
    white-space: nowrap;
    background: var(--card);
    border-bottom: var(--border-width) solid var(--border);
    user-select: none;
  }
  td {
    height: var(--table-row-height);
    max-width: var(--table-cell-width);
    padding: 0 var(--space-2-5);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    border-bottom: var(--border-width) solid var(--border);
  }
  tbody tr:last-child td {
    border-bottom: none;
  }
  th:not(:last-child),
  td:not(:last-child) {
    border-right: var(--border-width) solid var(--border);
  }
  [data-width='narrow'] {
    width: 1%;
  }
  [data-width='wide'] {
    max-width: calc(var(--table-cell-width) * 2);
  }
  tr[data-clickable] {
    cursor: pointer;
    outline: none;
  }
  tbody tr {
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  tr[data-clickable]:hover,
  tr[data-clickable]:focus-visible {
    background: var(--tint-1);
  }
  tr[data-clickable]:focus-visible td:first-child {
    box-shadow: inset var(--focus-width) 0 0 var(--ring);
  }
  .empty {
    padding: var(--space-8) var(--space-5);
  }
</style>
