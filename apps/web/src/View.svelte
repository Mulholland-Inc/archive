<script>
  import { api, hidden, show } from './api.js';
  import Table from './Table.svelte';
  import Card from './ui/Card.svelte';
  import Heading from './ui/Heading.svelte';
  import List from './ui/List.svelte';
  import ListItem from './ui/ListItem.svelte';
  import Note from './ui/Note.svelte';
  import Panel from './ui/Panel.svelte';

  let { view, types = {} } = $props();
  let rows = $state(null);
  let error = $state('');

  const label = $derived(view.columns.find((c) => !hidden.has(c.name) && c.name !== view.by)?.name);
  const details = $derived(view.columns.filter((c) => !hidden.has(c.name) && c.name !== view.by && c.name !== label && !c.ref).slice(0, 2));

  const groups = $derived.by(() => {
    if (!rows || view.layout === 'table') return [];
    const out = new Map();
    const col = view.columns.find((c) => c.name === view.by);
    if (view.layout === 'board' && col) for (const v of types[`${col.schema}.${col.type}`]?.enum ?? []) out.set(v, []);
    const sorted = view.layout === 'calendar' ? [...rows].filter((r) => r[view.by]).sort((a, b) => String(a[view.by]).localeCompare(String(b[view.by]))) : rows;
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
    const all = [];
    for (let after = ''; ; ) {
      const q = new URLSearchParams({ limit: '500' });
      if (after) q.set('after', after);
      const next = await api(`/api/${view.name}?${q}`);
      all.push(...next);
      if (next.length < 500) break;
      after = next.at(-1).id;
    }
    rows = all;
  }

  if (view.layout !== 'table' && view.by) load().catch((e) => (error = e.message));
</script>

{#if view.layout === 'table' || !view.by}
  <Table table={view} />
{:else}
  <Heading title={view.name.replaceAll('_', ' ')} />
  {#if error}<Panel><Note error>{error}</Note></Panel>{/if}
  {#if view.layout === 'board'}
    <div class="board">
      {#each groups as [key, items] (key)}
        <div class="lane">
          <Panel compact>
            <header><span>{key}</span><span class="count">{items.length}</span></header>
            {#each items as r (r.id)}
              <Card title={show(r[label])} details={details.map((c) => show(r[c.name])).filter(Boolean)} />
            {/each}
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
        <Note>{rows ? 'Nothing with a date yet.' : ''}</Note>
      {/each}
    </Panel>
  {/if}
{/if}

<style>
  .board { display: flex; align-items: flex-start; gap: var(--space-3); }
  .lane { flex: 0 0 var(--lane); }
  header { display: flex; justify-content: space-between; padding: var(--space-2); font-weight: var(--weight-medium); text-transform: capitalize; }
  .count { color: var(--text-3); font-weight: var(--weight); }
  .month { display: flex; flex-direction: column; gap: var(--space-3); }
  h2 { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); }
</style>
