<script>
  import { api, hidden, show } from './api.js';

  let { table, where = {}, title, lead, rows: given } = $props();
  let rows = $state([]);
  let more = $state(false);
  let error = $state('');
  const columns = $derived(table ? table.columns.filter((c) => !hidden.has(c.name) && !(c.name in where)) : []);

  async function page() {
    try {
      const q = new URLSearchParams({ ...where, limit: '100' });
      if (rows.length) q.set('after', rows.at(-1).id);
      const next = await api(`/api/${table.name}?${q}`);
      rows = [...rows, ...next];
      more = next.length === 100;
    } catch (e) {
      error = e.message;
    }
  }

  if (given) rows = given;
  else if (table) page();
</script>

{#if table}
  <h1>{title ?? table.name.replaceAll('_', ' ')}</h1>
  {#if lead ?? table.description}<p class="lead">{lead ?? table.description}</p>{/if}
  <div class="panel">
    {#if error}<p class="error">{error}</p>{/if}
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
                <td class:ref={c.ref}>{show(r[c.name])}</td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if !rows.length && !error}<p class="muted">Nothing here yet.</p>{/if}
    {#if more && !given}<button class="more" onclick={page}>Show more</button>{/if}
  </div>
{/if}

<style>
  h1 { text-transform: capitalize; }
  .wrap { overflow-x: auto; margin: 0 -8px; }
  table { border-collapse: collapse; min-width: 100%; }
  th {
    text-align: left;
    font-size: 14px;
    font-weight: 400;
    color: var(--text-2);
    padding: 0 8px 12px;
    border-bottom: 1px solid var(--line);
    white-space: nowrap;
    text-transform: capitalize;
  }
  td {
    padding: 14px 8px;
    border-bottom: 1px solid var(--line);
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-2);
    font-size: 15px;
  }
  td:first-child { color: var(--text); font-weight: 500; }
  td.ref { font-family: var(--mono); font-size: 12px; color: var(--text-3); }
  tbody tr:last-child td { border-bottom: 0; }
  .error { color: var(--alarm); margin: 0 0 16px; }
  .muted { margin: 0; }
  .more { margin-top: 24px; }
</style>
