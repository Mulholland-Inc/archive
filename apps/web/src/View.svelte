<script>
  import { hidden, show } from './api.js';
  import Table from './Table.svelte';
  import Card from './ui/Card.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import Sheet from './ui/Sheet.svelte';
  import ListItem from './ui/ListItem.svelte';
  import Note from './ui/Note.svelte';
  import { fetchRows, keep, read } from './ui/filter.js';

  let { view, types = {} } = $props();
  let rows = $state(null);
  let error = $state('');
  let q = $state({ ...read(), sort: null });
  let seq = 0;

  const columns = $derived(view.columns.filter((c) => !hidden.has(c.name)));
  const label = $derived(columns.find((c) => c.name !== view.by)?.name);
  const details = $derived(columns.filter((c) => c.name !== view.by && c.name !== label && !c.ref).slice(0, 2));
  const shown = $derived(rows && keep(rows, q.values, columns, types));
  const query = $derived(q.query.trim());

  const groups = $derived.by(() => {
    if (!shown || view.layout === 'table') return [];
    const out = new Map();
    const col = view.columns.find((c) => c.name === view.by);
    if (view.layout === 'board' && col) for (const v of types[`${col.schema}.${col.type}`]?.enum ?? []) out.set(v, []);
    const sorted = view.layout === 'calendar' ? shown.filter((r) => r[view.by]).sort((a, b) => String(a[view.by]).localeCompare(String(b[view.by]))) : shown;
    for (const r of sorted) {
      const v = r[view.by];
      const key = view.layout === 'calendar'
        ? new Date(v).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
        : v === null || v === undefined || v === '' ? 'None' : show(v);
      if (!out.has(key)) out.set(key, []);
      out.get(key).push(r);
    }
    return [...out];
  });

  async function load() {
    const n = ++seq;
    try {
      const out = await fetchRows(view.name, { query, all: true });
      if (n === seq) (rows = out.rows), (error = '');
    } catch (e) {
      if (n === seq) error = e.message;
    }
  }

  $effect(() => {
    if (view.layout === 'table' || !view.by) return;
    const t = setTimeout(load, query ? 200 : 0);
    return () => clearTimeout(t);
  });
</script>

{#if view.layout === 'table' || !view.by}
  <Table table={view} {types} />
{:else}
  <Heading title={view.name.replaceAll('_', ' ')} />
  <Filters bind:q {columns} {types}>
    {#if view.layout === 'board'}
      {#if error}<Note error>{error}</Note>{/if}
      <div class="board">
        {#each groups as [key, items] (key)}
          <section class="lane">
            <header><span>{key}</span><span class="count">{items.length}</span></header>
            <div class="cards">
              {#each items as r (r.id)}
                <Card title={show(r[label])} details={details.map((c) => show(r[c.name])).filter(Boolean)} />
              {/each}
            </div>
          </section>
        {/each}
      </div>
    {:else}
      <Sheet>
        {#if error}<Note error>{error}</Note>{/if}
        {#each groups as [month, items] (month)}
          <section class="month">
            <h2>{month}</h2>
            <div class="list">
              {#each items as r (r.id)}
                <ListItem
                  lead={new Date(r[view.by]).toLocaleDateString(undefined, { day: 'numeric', weekday: 'short' })}
                  meta={details[0] ? [show(r[details[0].name])] : []}
                >{show(r[label])}</ListItem>
              {/each}
            </div>
          </section>
        {:else}
          {#if rows}<Note>{rows.length || query ? 'Nothing matches' : 'Nothing with a date yet'}</Note>{/if}
        {/each}
      </Sheet>
    {/if}
  </Filters>
{/if}

<style>
  .board { display: flex; flex: 1 1 auto; gap: var(--space-2); min-height: 0; height: var(--board-height); overflow: auto hidden; }
  .lane {
    display: flex;
    flex: 0 0 var(--lane-width);
    flex-direction: column;
    gap: var(--space-1);
    min-height: 0;
    padding: var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--surface-band);
  }
  .cards { display: flex; flex: 1; flex-direction: column; gap: var(--space-1); min-height: 0; overflow-y: auto; }
  .cards > :global(*) { flex: none; }
  header { display: flex; flex: none; align-items: center; justify-content: space-between; gap: var(--space-2); padding: var(--space-2); font-weight: var(--weight-medium); line-height: var(--leading-snug); text-transform: capitalize; }
  .count { min-width: var(--size-avatar); padding: var(--space-1); border-radius: var(--radius-full); background: var(--fill); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-normal); line-height: var(--leading-none); text-align: center; }
  .month { display: flex; flex-direction: column; gap: var(--space-2); }
  h2 { font: var(--weight-medium) var(--text-2xl) / var(--leading-tight) var(--font-serif); letter-spacing: var(--tracking-tight); }
  .list { display: flex; flex-direction: column; gap: var(--space-0-5); }
</style>
