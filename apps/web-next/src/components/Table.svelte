<script>
  let { columns = [], rows = [], cell, key = 'id', onrowclick, label, selected = [], cards = false } = $props();
</script>

<div class={['wrap', { cards }]}>
  <table aria-label={label}>
    <thead>
      <tr>
        {#each columns as col (col.key)}
          <th class={[col.align, col.width && `w-${col.width}`]} scope="col">{col.label}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as row (row[key])}
        <tr class={{ clickable: onrowclick, selected: selected.includes(row[key]) }}>
          {#each columns as col, i (col.key)}
            <td class={[col.align, col.width && `w-${col.width}`, col.muted && 'muted']}>
              {#if onrowclick && i === 0}
                <button type="button" class="rowlink" onclick={() => onrowclick(row)}>
                  {#if cell}{@render cell(row, col)}{:else}{row[col.key]}{/if}
                </button>
              {:else if cell}{@render cell(row, col)}{:else}{row[col.key]}{/if}
            </td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap {
    min-width: 0;
    overflow-x: auto;
  }

  table {
    width: 100%;
    font-size: var(--text-md);
  }

  th {
    height: var(--control-lg);
    padding: var(--space-2) var(--space-3);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    text-align: start;
    white-space: nowrap;
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  td {
    height: var(--row-height);
    padding: var(--space-1-5) var(--space-3);
    white-space: nowrap;
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  tr:last-child td {
    box-shadow: none;
  }

  .muted {
    color: var(--color-text-secondary);
  }

  .end {
    text-align: end;
  }

  .center {
    text-align: center;
  }

  .w-xs { width: var(--control-xl); }
  .w-sm { width: var(--col-sm); }
  .w-md { width: var(--col-md); }
  .w-lg { width: var(--col-lg); }

  .clickable {
    position: relative;
    cursor: pointer;
  }

  .rowlink {
    display: flex;
    align-items: center;
    text-align: start;
    border-radius: var(--radius-xs);
  }

  .rowlink::after {
    content: '';
    position: absolute;
    inset: 0;
  }

  .clickable td :global(:is(button, a):not(.rowlink)) {
    position: relative;
    z-index: var(--z-raised);
  }

  .clickable:hover td,
  .selected td {
    background: var(--color-fill-subtle);
  }

  .clickable:hover td:first-child,
  .selected td:first-child {
    border-start-start-radius: var(--radius-md);
    border-end-start-radius: var(--radius-md);
  }

  .clickable:hover td:last-child,
  .selected td:last-child {
    border-start-end-radius: var(--radius-md);
    border-end-end-radius: var(--radius-md);
  }

  @media (max-width: 40rem) {
    .cards table,
    .cards tbody,
    .cards tr,
    .cards td {
      display: block;
    }

    .cards thead {
      display: none;
    }

    .cards tr {
      position: relative;
      padding-block: var(--space-3);
      box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
    }

    .cards tr:last-child {
      box-shadow: none;
    }

    .cards td {
      width: auto;
      height: auto;
      padding: var(--space-1) var(--space-0);
      white-space: normal;
      box-shadow: none;
    }

    .cards td:empty {
      display: none;
    }

    .cards td.muted {
      font-size: var(--text-sm);
    }

    .cards td:first-child {
      padding-inline-end: var(--control-xl);
    }

    .cards td.end {
      position: absolute;
      inset-block-start: var(--space-3);
      inset-inline-end: var(--space-0);
      padding: var(--space-0);
    }
  }
</style>
