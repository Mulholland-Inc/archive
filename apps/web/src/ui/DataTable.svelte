<script>
  let { columns, rows, cell } = $props();
</script>

<div class="wrap">
  <table>
    <thead>
      <tr>
        {#each columns as c (c.name)}
          <th title={c.description}>{c.name.replaceAll('_', ' ')}</th>
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
  .wrap { overflow-x: auto; margin: 0 calc(-1 * var(--space-2)); }
  table { border-collapse: collapse; min-width: 100%; }
  th {
    text-align: left;
    font-size: var(--text-sm);
    font-weight: var(--weight);
    color: var(--text-2);
    padding: 0 var(--space-2) var(--space-3);
    border-bottom: var(--stroke) solid var(--line);
    white-space: nowrap;
    text-transform: capitalize;
  }
  td {
    max-width: var(--cell);
    padding: var(--space-3) var(--space-2);
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
