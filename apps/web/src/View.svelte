<script>
  import { arrange, choice, hidden, rows as fetchRows, show } from './api.js';
  import Table from './Table.svelte';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import List from './ui/List.svelte';
  import ListItem from './ui/ListItem.svelte';
  import Note from './ui/Note.svelte';
  import Panel from './ui/Panel.svelte';
  import Select from './ui/Select.svelte';

  let { view, types = {} } = $props();
  let rows = $state(null);
  let error = $state('');
  let query = $state('');
  let filters = $state({});
  let by = $state('');
  let desc = $state(false);

  const columns = $derived(view.columns.filter((c) => !hidden.has(c.name)));
  const label = $derived(columns.find((c) => c.name !== view.by)?.name);
  const details = $derived(columns.filter((c) => c.name !== view.by && c.name !== label && !c.ref).slice(0, 2));
  const server = $derived(Object.fromEntries(Object.entries(filters).filter(([k, v]) => v !== '' && choice(columns.find((c) => c.name === k), types))));
  const key = $derived(JSON.stringify([query, server]));
  const sort = $derived(by ? { name: by, desc } : null);
  const shown = $derived(rows ? arrange(rows, columns, types, { filters, sort }) : null);
  let seq = 0;

  const groups = $derived.by(() => {
    if (!shown) return [];
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
      const out = await fetchRows(view.name, columns, types, { query, filters: server, all: true });
      if (n === seq) (rows = out.rows), (error = '');
    } catch (e) {
      error = e.message;
    }
  }

  $effect(() => {
    if (view.layout === 'table' || !view.by) return;
    const t = setTimeout(load, JSON.parse(key)[0] ? 250 : 0);
    return () => clearTimeout(t);
  });
</script>

{#if view.layout === 'table' || !view.by}
  <Table table={view} {types} />
{:else}
  <Heading title={view.name.replaceAll('_', ' ')} />
  <Filters {columns} {types} bind:query bind:filters>
    {#if view.layout === 'board'}
      <Select label="Sort" icon="arrow-up-down" items={[{ value: '', label: 'None' }, ...columns.map((c) => ({ value: c.name, label: c.name.replaceAll('_', ' ') }))]} bind:value={by} />
      {#if by}<Button quiet icon={desc ? 'arrow-down' : 'arrow-up'} title="Flip order" aria-label="Flip order" onclick={() => (desc = !desc)} />{/if}
    {/if}
  </Filters>
  {#if error}<Panel><Note error>{error}</Note></Panel>{/if}
  {#if view.layout === 'board'}
    <div class="board">
      {#each groups as [key, items] (key)}
        <div class="lane">
          <Panel compact>
            <header><span>{key}</span><span class="count">{items.length}</span></header>
            <div class="cards">
              {#each items as r (r.id)}
                <Card title={show(r[label])} details={details.map((c) => show(r[c.name])).filter(Boolean)} />
              {/each}
            </div>
          </Panel>
        </div>
      {/each}
    </div>
  {:else}
    <Panel>
      {#each groups as [month, items] (month)}
        <div class="month">
          <h2>{month}</h2>
          <List>
            {#each items as r (r.id)}
              <ListItem
                lead={new Date(r[view.by]).toLocaleDateString(undefined, { day: 'numeric', weekday: 'short' })}
                meta={details[0] ? [show(r[details[0].name])] : []}
              >{show(r[label])}</ListItem>
            {/each}
          </List>
        </div>
      {:else}
        <Note>{rows ? (query || Object.values(filters).some(Boolean) ? 'Nothing matches.' : 'Nothing with a date yet.') : ''}</Note>
      {/each}
    </Panel>
  {/if}
{/if}

<style>
  .board { display: flex; gap: var(--space-3); }
  .lane { display: flex; flex: 0 0 var(--lane); flex-direction: column; min-height: 0; }
  .lane > :global(section) { flex: 1; min-height: 0; }
  .cards { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-height: 0; overflow-y: auto; border-radius: var(--radius-lg); }
  header { display: flex; flex: none; justify-content: space-between; padding: var(--space-2); font-weight: var(--weight-medium); text-transform: capitalize; }
  .count { color: var(--text-3); font-weight: var(--weight); }
  .month { display: flex; flex-direction: column; gap: var(--space-3); }
  h2 { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); }
</style>
