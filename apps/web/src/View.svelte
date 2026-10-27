<script>
  import { api, hidden, show } from './api.js';
  import Table from './Table.svelte';

  let { view, table, types = {} } = $props();
  let rows = $state(null);
  let error = $state('');

  const label = $derived(table ? table.columns.find((c) => !hidden.has(c.name) && c.name !== view.by)?.name : '');
  const details = $derived(table ? table.columns.filter((c) => !hidden.has(c.name) && c.name !== view.by && c.name !== label && !c.ref).slice(0, 2) : []);
  const filter = $derived(Object.entries(view.where ?? {}).map(([k, v]) => `${k.replaceAll('_', ' ')} is ${v}`).join(', '));
  const lead = $derived(`${table?.name.replaceAll('_', ' ') ?? view.type}${filter ? `, where ${filter}` : ''}${view.by ? `, by ${view.by.replaceAll('_', ' ')}` : ''}`);

  const groups = $derived.by(() => {
    if (!rows || view.layout === 'table') return [];
    const out = new Map();
    const col = table?.columns.find((c) => c.name === view.by);
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
      const q = new URLSearchParams({ ...view.where, limit: '500' });
      if (after) q.set('after', after);
      const next = await api(`/api/${view.type}?${q}`);
      all.push(...next);
      if (next.length < 500) break;
      after = next.at(-1).id;
    }
    rows = all;
  }

  load().catch((e) => (error = e.message));
</script>

{#if view.layout === 'table' || !view.by}
  {#if rows}
    <Table {table} where={view.where ?? {}} title={view.name} {lead} {rows} />
  {/if}
{:else}
  <h1>{view.name}</h1>
  <p class="lead">{lead}</p>
  {#if error}<p class="error">{error}</p>{/if}
  {#if view.layout === 'board'}
    <div class="board">
      {#each groups as [key, items] (key)}
        <section>
          <header><span>{key}</span><span class="count">{items.length}</span></header>
          {#each items as r (r.id)}
            <article>
              <div class="title">{show(r[label])}</div>
              {#each details as c (c.name)}
                {#if show(r[c.name])}<div class="detail">{show(r[c.name])}</div>{/if}
              {/each}
            </article>
          {/each}
        </section>
      {/each}
    </div>
  {:else}
    <div class="panel">
      {#each groups as [month, items] (month)}
        <h2>{month}</h2>
        {#each items as r (r.id)}
          <div class="day">
            <span class="date">{new Date(r[view.by]).toLocaleDateString(undefined, { day: 'numeric', weekday: 'short' })}</span>
            <span class="title">{show(r[label])}</span>
            {#if details[0]}<span class="detail">{show(r[details[0].name])}</span>{/if}
          </div>
        {/each}
      {:else}
        <p class="muted">{rows ? 'Nothing with a date yet.' : ''}</p>
      {/each}
    </div>
  {/if}
{/if}

<style>
  .board { display: flex; gap: 12px; margin-top: 32px; overflow-x: auto; padding-bottom: 8px; align-items: flex-start; }
  section { flex: 0 0 280px; background: var(--panel); border-radius: 28px; padding: 18px; }
  section header { display: flex; justify-content: space-between; padding: 0 6px 12px; text-transform: capitalize; font-weight: 500; }
  .count { color: var(--text-3); font-weight: 400; }
  article { background: var(--raised); border-radius: 16px; padding: 12px 14px; margin-top: 8px; }
  article .title { font-weight: 500; }
  .detail { color: var(--text-2); font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  h2 { font: 500 28px/1.2 var(--display); letter-spacing: -0.02em; margin: 24px 0 8px; }
  h2:first-child { margin-top: 0; }
  .day { display: grid; grid-template-columns: 96px 1fr auto; gap: 16px; align-items: baseline; padding: 10px 0; border-bottom: 1px solid var(--line); }
  .date { color: var(--text-2); font-size: 14px; }
  .day .detail { text-align: right; }
  .error { color: var(--alarm); }
  .muted { margin: 0; }
</style>
