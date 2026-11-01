<script>
  let { columns = [], rows = [], cell, leading, key = 'id', onrowclick, label, selected = [], cards = false } = $props();

  const link = $derived(columns.findIndex((c) => !c.control));
  const role = (col, i) => col.mobile ?? (i === link ? 'primary' : col.align === 'end' ? 'trailing' : 'secondary');
</script>

{#snippet content(row, col, i)}
  {#if leading && i === 0}
    <div class="person">
      <div class="lead">{@render leading(row)}</div>
      <div class="main">{#if cell}{@render cell(row, col)}{:else}{row[col.key]}{/if}</div>
    </div>
  {:else if cell}{@render cell(row, col)}{:else}{row[col.key]}{/if}
{/snippet}

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
        <tr class={{ clickable: onrowclick, selected: selected.includes(row[key]), led: !!leading }}>
          {#each columns as col, i (col.key)}
            <td class={[col.align, col.width && `w-${col.width}`, col.muted && 'muted', `m-${role(col, i)}`, leading && i === 0 && 'haslead']}>
              {#if onrowclick && i === link}
                <button type="button" class="rowlink" onclick={() => onrowclick(row)}>{@render content(row, col, i)}</button>
              {:else}{@render content(row, col, i)}{/if}
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

  .w-icon { width: var(--control-md); padding-inline-end: var(--space-0); }
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

  .person {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }

  .lead {
    display: flex;
    flex: none;
  }

  .main {
    display: flex;
    min-width: 0;
  }

  @media (max-width: 40rem) {
    .cards table,
    .cards tbody {
      display: block;
    }

    .cards thead {
      display: none;
    }

    .cards tr {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr);
      grid-template-rows: auto auto;
      grid-auto-columns: auto;
      align-items: center;
      padding-block: var(--space-2-5);
      box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
    }

    .cards tr:last-child {
      box-shadow: none;
    }

    .cards td {
      width: auto;
      height: auto;
      padding: var(--space-0);
      box-shadow: none;
    }

    .cards .m-hidden {
      display: none;
    }

    .cards .m-primary {
      grid-column: 2;
      grid-row: 1;
      min-width: 0;
    }

    .cards .m-secondary {
      grid-column: 2;
      grid-row: 2;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      color: var(--color-text-secondary);
      font-size: var(--text-sm);
    }

    .cards .m-trailing {
      grid-row: 1 / span 2;
      padding-inline-start: var(--space-2);
      color: var(--color-text-secondary);
    }

    .cards .m-trailing.end {
      padding-inline-start: var(--space-1);
    }

    .cards tr:not(.led) :is(.m-primary, .m-secondary) {
      grid-column: 1 / 3;
    }

    .cards .m-primary.haslead,
    .cards .haslead .person {
      display: contents;
    }

    .cards .haslead .lead {
      grid-column: 1;
      grid-row: 1 / span 2;
      padding-inline-end: var(--space-3);
    }

    .cards .haslead .main {
      grid-column: 2;
      grid-row: 1;
    }
  }
</style>
