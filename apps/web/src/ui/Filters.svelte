<script>
  import Choice from './Choice.svelte';
  import Search from './Search.svelte';
  import { filtering, kind, options, read, title, write } from './filter.js';

  let { q = $bindable(), columns = [], types = {}, children } = $props();
  const home = location.pathname;

  const controls = $derived(columns.flatMap((c) => {
    const k = kind(c, types);
    if (!k) return [];
    return [{ c, k, items: k === 'enum' || k === 'bool' ? options(c, types) : [] }];
  }));
  const busy = $derived(!!q.query || filtering(q));

  const get = (name) => q.values[name] ?? '';
  const set = (name, v) => (q.values[name] = v);
  const range = (name) => {
    const [from = '', to = ''] = get(name).split('..');
    return { from, to };
  };

  function span(name, side, v) {
    const r = range(name);
    r[side] = v;
    set(name, r.from || r.to ? `${r.from}..${r.to}` : '');
  }

  function clear() {
    q.query = '';
    q.values = {};
  }

  $effect(() => {
    if (location.pathname === home) write(q);
  });

  $effect(() => {
    const back = () => location.pathname === home && Object.assign(q, read());
    addEventListener('popstate', back);
    return () => removeEventListener('popstate', back);
  });
</script>

<div class="stack">
<div class="filters">
  <Search bind:value={q.query} />
  {#each controls as { c, k, items } (c.name)}
    {#if k === 'enum' || k === 'bool'}
      <Choice label={title(c)} {items} bind:value={() => get(c.name), (v) => set(c.name, v)} />
    {:else if k === 'date'}
      {@const r = range(c.name)}
      <div class="group" class:set={r.from || r.to} role="group" aria-label={title(c)}>
        <span class="name">{title(c)}</span>
        <input class="date" type="date" aria-label="{title(c)} from" value={r.from} max={r.to || undefined} oninput={(e) => span(c.name, 'from', e.currentTarget.value)} />
        <input class="date" type="date" aria-label="{title(c)} to" value={r.to} min={r.from || undefined} oninput={(e) => span(c.name, 'to', e.currentTarget.value)} />
      </div>
    {:else}
      <input
        class="field"
        class:set={get(c.name)}
        type="search"
        placeholder={title(c)}
        aria-label={title(c)}
        value={get(c.name)}
        oninput={(e) => set(c.name, e.currentTarget.value)}
        onkeydown={(e) => e.key === 'Escape' && set(c.name, '')}
      />
    {/if}
  {/each}
  {#if busy}<button type="button" class="clear" onclick={clear}>Clear</button>{/if}
</div>
{@render children?.()}
</div>

<style>
  .stack { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-width: 0; min-height: 0; }
  .filters {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--space-1);
    min-width: 0;
    overflow-x: auto;
    font-size: var(--text-table);
    line-height: var(--leading-none);
  }
  .group {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--space-0-5);
    min-width: 0;
    height: var(--filter-height);
    padding: var(--space-0-5);
    border-radius: var(--radius-full);
    background: var(--fill);
    transition: background var(--duration-fast);
  }
  .group.set { background: var(--fill-strong); }
  .name { flex: none; padding: var(--space-1-5); color: var(--fg-muted); text-transform: capitalize; white-space: nowrap; }
  .date {
    flex: none;
    padding: var(--space-1-5);
    border-radius: var(--radius-full);
    color: var(--fg-muted);
    white-space: nowrap;
    text-transform: capitalize;
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  .date { line-height: var(--leading-none); outline: 0; }
  .date:hover, .date:focus { background: var(--fill); color: var(--fg); }
  .field {
    flex: 0 0 var(--filter-width);
    min-width: 0;
    height: var(--filter-height);
    padding: var(--space-2);
    border-radius: var(--radius-full);
    background: var(--fill);
    color: var(--fg);
    line-height: var(--leading-none);
    outline: 0;
    transition: background var(--duration-fast);
  }
  .field::placeholder { color: var(--fg-muted); text-transform: capitalize; }
  .field::-webkit-search-cancel-button { display: none; }
  .field:hover, .field:focus, .field.set { background: var(--fill-strong); }
  .clear { flex: none; height: var(--filter-height); padding: var(--space-2); color: var(--fg-muted); white-space: nowrap; transition: color var(--duration-fast); }
  .clear:hover { color: var(--fg); }
</style>
