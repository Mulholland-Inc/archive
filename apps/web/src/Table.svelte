<script>
  import { api, hidden, show } from './api.js';
  import DataTable from './ui/DataTable.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import Sheet from './ui/Sheet.svelte';
  import Note from './ui/Note.svelte';
  import Empty from './ui/Empty.svelte';
  import { cycle, fetchRows, keep, order, read } from './ui/filter.js';

  let { table, types: given } = $props();
  let rows = $state([]);
  let error = $state('');
  let loaded = $state(false);
  let fetched = $state({});
  let q = $state(read());
  let seq = 0;
  const types = $derived(given ?? fetched);
  const columns = $derived(table.columns.filter((c) => !hidden.has(c.name)));
  const shown = $derived(order(keep(rows, q.values, columns, types), q.sort, columns, types));
  const query = $derived(q.query.trim());

  if (!given) api('/api/types').then((c) => (fetched = c.types)).catch(() => {});

  async function load() {
    const n = ++seq;
    try {
      for (let after = ''; ; ) {
        const out = await fetchRows(table.name, { query, after });
        if (n !== seq) return;
        rows = after ? [...rows, ...out.rows] : out.rows;
        error = '';
        loaded = true;
        if (!out.more) break;
        after = out.rows.at(-1).id;
      }
    } catch (e) {
      if (n === seq) (error = e.message), (loaded = true);
    }
  }

  $effect(() => {
    const t = setTimeout(load, query ? 200 : 0);
    return () => clearTimeout(t);
  });
</script>

<Heading title={table.name.replaceAll('_', ' ')} />
<Filters bind:q {columns} {types}>
  <Sheet tight>
    {#if error}<Note error>{error}</Note>{/if}
    <DataTable fill {columns} rows={shown} cell={show} sort={q.sort} onsort={(col) => (q.sort = cycle(q.sort, col))} href={table.kind === 'table' ? (r) => `/${table.name}/${r.id}` : undefined} />
    {#if loaded && !shown.length && !error}{#if rows.length || q.query}<Note>Nothing matches</Note>{:else}<Empty title="Nothing here yet" />{/if}{/if}
  </Sheet>
</Filters>
