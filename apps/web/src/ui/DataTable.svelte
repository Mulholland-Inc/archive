<script>
  import Icon from './Icon.svelte';

  let { columns, rows, cell, sort, onsort } = $props();
</script>

<div class="wrap">
  <table>
    <thead>
      <tr>
        {#each columns as c (c.name)}
          {@const on = sort?.name === c.name}
          <th aria-sort={on ? (sort.desc ? 'descending' : 'ascending') : undefined}>
            <button class:on onclick={() => onsort?.(c.name)}>
              {c.name.replaceAll('_', ' ')}
              {#if on}<Icon name={sort.desc ? 'chevron-down' : 'chevron-up'} />{/if}
            </button>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as r (r.id)}
        <tr>
          {#each columns as c (c.name)}
            <td class:ref={c.ref}>{cell(r[c.name])}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap { overflow-x: auto; margin: calc(-1 * var(--space-3)); }
  table { border-collapse: collapse; min-width: 100%; }
  th {
    position: sticky;
    top: 0;
    padding: 0;
    background: var(--panel);
    text-align: left;
    border-bottom: var(--stroke) solid var(--line);
  }
  th button {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
    padding: var(--space-3);
    border: 0;
    background: none;
    font-size: var(--text-sm);
    color: var(--text-2);
    white-space: nowrap;
    text-transform: capitalize;
    cursor: pointer;
    transition: color var(--dur) var(--ease);
  }
  th button:hover, .on { color: var(--text); }
  th :global(svg) { width: var(--text-md); height: var(--text-md); }
  td {
    max-width: var(--cell);
    padding: var(--space-3);
    border-bottom: var(--stroke) solid var(--line);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-2);
  }
  td:first-child { color: var(--text); font-weight: var(--weight-medium); }
  .ref { font-family: var(--mono); font-size: var(--text-xs); color: var(--text-3); }
  tbody tr:last-child td { border-bottom: 0; }
</style>
