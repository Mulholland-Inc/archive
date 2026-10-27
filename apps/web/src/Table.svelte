<script>
  import { arrange, choice, hidden, rows as fetchRows, show } from './api.js';
  import Button from './ui/Button.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Panel from './ui/Panel.svelte';

  let { table, types = {} } = $props();
  let rows = $state([]);
  let more = $state(false);
  let error = $state('');
  let query = $state('');
  let filters = $state({});
  let sort = $state(null);
  const columns = $derived(table.columns.filter((c) => !hidden.has(c.name)));
  const server = $derived(Object.fromEntries(Object.entries(filters).filter(([k, v]) => v !== '' && choice(columns.find((c) => c.name === k), types))));
  const all = $derived(!!sort || Object.entries(filters).some(([k, v]) => v !== '' && !(k in server)));
  const key = $derived(JSON.stringify([query, server, all]));
  let seq = 0;
  const shown = $derived(arrange(rows, columns, types, { filters, sort }));

  async function load(after = '') {
    const n = ++seq;
    try {
      const out = await fetchRows(table.name, columns, types, { query, filters: server, all, after });
      if (n !== seq) return;
      rows = after ? [...rows, ...out.rows] : out.rows;
      more = out.more;
      error = '';
    } catch (e) {
      error = e.message;
    }
  }

  $effect(() => {
    const t = setTimeout(load, JSON.parse(key)[0] ? 250 : 0, '');
    return () => clearTimeout(t);
  });

  function order(name) {
    sort = sort?.name !== name ? { name, desc: false } : sort.desc ? null : { name, desc: true };
  }
</script>

<Heading title={table.name.replaceAll('_', ' ')} />
<Filters {columns} {types} bind:query bind:filters />
<Panel>
  {#if error}<Note error>{error}</Note>{/if}
  <DataTable {columns} rows={shown} cell={show} {sort} onsort={order} />
  {#if !shown.length && !error}<Note>{query || Object.values(filters).some(Boolean) ? 'Nothing matches.' : 'Nothing here yet.'}</Note>{/if}
  {#if more}<div><Button onclick={() => load(rows.at(-1).id)}>Show more</Button></div>{/if}
</Panel>
