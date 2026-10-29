<script>
  import { api, hidden, show } from './api.js';
  import Button from '#kit/Button.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import Sheet from './ui/Sheet.svelte';
  import Note from './ui/Note.svelte';
  import { active, cycle, fetchRows, keep, order, read } from './ui/filter.js';

  let { table, types: given } = $props();
  let rows = $state([]);
  let more = $state(false);
  let error = $state('');
  let loaded = $state(false);
  let fetched = $state({});
  let q = $state(read());
  let seq = 0;
  const types = $derived(given ?? fetched);
  const columns = $derived(table.columns.filter((c) => !hidden.has(c.name)));
  const shown = $derived(order(keep(rows, q.values, columns, types), q.sort, columns, types));
  const want = $derived(JSON.stringify([q.query.trim(), active(q)]));

  if (!given) api('/api/types').then((c) => (fetched = c.types)).catch(() => {});

  async function load(after = '') {
    const n = ++seq;
    try {
      const out = await fetchRows(table.name, { query: q.query, all: active(q), after });
      if (n !== seq) return;
      rows = after ? [...rows, ...out.rows] : out.rows;
      more = out.more;
      error = '';
    } catch (e) {
      if (n === seq) error = e.message;
    }
    loaded = true;
  }

  $effect(() => {
    const t = setTimeout(load, JSON.parse(want)[0] ? 200 : 0);
    return () => clearTimeout(t);
  });
</script>

<Heading title={table.name.replaceAll('_', ' ')} />
<Filters bind:q {columns} {types}>
  <Sheet tight>
    {#if error}<Note error>{error}</Note>{/if}
    <DataTable fill {columns} rows={shown} cell={show} sort={q.sort} onsort={(col) => (q.sort = cycle(q.sort, col))} />
    {#if loaded && !shown.length && !error}<Note>{rows.length || q.query ? 'Nothing matches' : 'Nothing here yet'}</Note>{/if}
    {#if more}<div class="more"><Button ghost small onclick={() => load(rows.at(-1).id)}>Show more</Button></div>{/if}
  </Sheet>
</Filters>

<style>
  .more { display: flex; justify-content: center; }
</style>
