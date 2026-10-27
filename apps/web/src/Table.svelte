<script>
  import { api, hidden, show } from './api.js';
  import Button from './ui/Button.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Panel from './ui/Panel.svelte';

  let { table } = $props();
  let rows = $state([]);
  let more = $state(false);
  let error = $state('');
  const columns = $derived(table.columns.filter((c) => !hidden.has(c.name)));

  async function page() {
    try {
      const q = new URLSearchParams({ limit: '100' });
      if (rows.length) q.set('after', rows.at(-1).id);
      const next = await api(`/api/${table.name}?${q}`);
      rows = [...rows, ...next];
      more = next.length === 100;
    } catch (e) {
      error = e.message;
    }
  }

  page();
</script>

<Heading title={table.name.replaceAll('_', ' ')} description={table.description} />
<Panel>
  {#if error}<Note error>{error}</Note>{/if}
  <DataTable {columns} {rows} cell={show} />
  {#if !rows.length && !error}<Note>Nothing here yet.</Note>{/if}
  {#if more}<div class="more"><Button onclick={page}>Show more</Button></div>{/if}
</Panel>

<style>
  .more { margin-top: var(--space-5); }
</style>
