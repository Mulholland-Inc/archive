<script>
  import { ChevronDown, Search } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import Input from './Input.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { filterKind, filterOptions, setFilter, viewState } from '../lib/app.svelte.js';

  let { id, column, label, text } = $props();

  const kind = $derived(filterKind(column));
  const filter = $derived(viewState(id).filters[column.name] ?? {});
  const placeholder = 'Empty';
  const money = $derived(column.type === 'money' ? '$' : '');
  const shown = (v) => (v === null ? placeholder : text(column, { [column.name]: column.list ? [v] : v }) || placeholder);
  const day = (s) => new Date(`${s}T00:00:00`).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });

  const summary = $derived.by(() => {
    if (filter.contains) return `contains ${filter.contains}`;
    const values = filter.values ?? [];
    if (values.length === 1) return shown(values[0]);
    if (values.length) return `${values.length} selected`;
    const fmt = (v) => (kind === 'dates' ? day(v) : `${money}${v}`);
    if (filter.min && filter.max) return `${fmt(filter.min)} to ${fmt(filter.max)}`;
    if (filter.min) return kind === 'dates' ? `from ${fmt(filter.min)}` : `${fmt(filter.min)} or more`;
    if (filter.max) return kind === 'dates' ? `until ${fmt(filter.max)}` : `${fmt(filter.max)} or less`;
    return '';
  });

  let term = $state('');
  let options = $state([]);
  let loading = $state(false);
  let input = $state();
  let timer;
  let asked = 0;

  async function load() {
    if (kind !== 'values') return;
    const mine = ++asked;
    loading = true;
    const got = await filterOptions(id, column, term.trim());
    if (mine !== asked) return;
    options = got;
    loading = false;
  }

  function opened(props) {
    props.onclick();
    term = '';
    load();
    requestAnimationFrame(() => requestAnimationFrame(() => input?.focus()));
  }

  function searching() {
    clearTimeout(timer);
    timer = setTimeout(load, 200);
  }

  function keep(event) {
    if (event.key !== 'ArrowDown' && event.key !== 'Escape') event.stopPropagation();
  }

  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  const picked = (v) => (filter.values ?? []).some((x) => same(x, v));

  function toggle(v) {
    const values = filter.values ?? [];
    setFilter(id, column.name, { ...filter, values: picked(v) ? values.filter((x) => !same(x, v)) : [...values, v] });
  }

  const unpicked = $derived((filter.values ?? []).filter((v) => !options.some((o) => same(o.v, v))));
  const iso = (d) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 10);
  const back = (n) => {
    const d = new Date();
    d.setDate(d.getDate() - n);
    return iso(d);
  };
  const ranges = [
    { label: 'Today', min: () => back(0), max: () => back(0) },
    { label: 'Last 7 days', min: () => back(6), max: () => back(0) },
    { label: 'Last 30 days', min: () => back(29), max: () => back(0) },
    { label: 'This month', min: () => `${back(0).slice(0, 7)}-01`, max: () => back(0) }
  ];
</script>

<Menu {label} width="lg" head={false}>
  {#snippet trigger(props)}
    <button type="button" class={['pill', { active: summary }]} {...props} onclick={() => opened(props)}>
      <span class="name">{label}</span>
      {#if summary}<span class="value">{summary}</span>{/if}
      <Icon icon={ChevronDown} size="sm" />
    </button>
  {/snippet}

  {#if kind === 'values'}
    <div class="field">
      <Input bind:element={input} bind:value={term} size="sm" icon={Search} placeholder={`Find ${label.toLowerCase()}`} aria-label={`Find ${label.toLowerCase()}`} oninput={searching} onkeydown={keep} />
    </div>
    {#if term.trim() && !column.list}
      <MenuItem onselect={() => setFilter(id, column.name, { contains: term.trim() })}>Contains “{term.trim()}”</MenuItem>
    {/if}
    {#each unpicked as v (JSON.stringify(v))}
      <MenuItem checked keepOpen onselect={() => toggle(v)}>{shown(v)}</MenuItem>
    {/each}
    {#each options as o (JSON.stringify(o.v))}
      <MenuItem checked={picked(o.v)} keepOpen onselect={() => toggle(o.v)}>
        {shown(o.v)}
        {#snippet end()}<span class="count">{o.n.toLocaleString()}</span>{/snippet}
      </MenuItem>
    {/each}
    {#if !options.length && !unpicked.length && !loading}<p class="none">Nothing found</p>{/if}
  {:else if kind === 'contains'}
    <div class="field">
      <Input bind:element={input} size="sm" icon={Search} placeholder="Contains" aria-label={`${label} contains`} value={filter.contains ?? ''} oninput={(e) => setFilter(id, column.name, { contains: e.target.value.trim() }, 250)} onkeydown={keep} />
    </div>
  {:else}
    {#if kind === 'dates'}
      {#each ranges as r (r.label)}
        <MenuItem checked={filter.min === r.min() && filter.max === r.max()} keepOpen onselect={() => setFilter(id, column.name, { min: r.min(), max: r.max() })}>{r.label}</MenuItem>
      {/each}
      <MenuSeparator />
    {/if}
    <div class="range">
      <Input bind:element={input} size="sm" type={kind === 'dates' ? 'date' : 'number'} placeholder="Min" aria-label={`${label} from`} value={filter.min ?? ''} oninput={(e) => setFilter(id, column.name, { ...filter, min: e.target.value }, 250)} onkeydown={keep} />
      <Input size="sm" type={kind === 'dates' ? 'date' : 'number'} placeholder="Max" aria-label={`${label} to`} value={filter.max ?? ''} oninput={(e) => setFilter(id, column.name, { ...filter, max: e.target.value }, 250)} onkeydown={keep} />
    </div>
  {/if}

  {#if summary}
    <MenuSeparator />
    <MenuItem onselect={() => setFilter(id, column.name, null)}>Clear</MenuItem>
  {/if}
</Menu>

<style>
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    max-width: var(--menu-max);
    height: var(--control-md);
    padding-inline: var(--space-2-5) var(--space-2);
    border-radius: var(--radius-md);
    background: var(--color-fill);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    white-space: nowrap;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .pill:hover {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .active {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
    color: var(--color-text);
  }

  .name {
    flex: none;
  }

  .active .name {
    color: var(--color-text-tertiary);
  }

  .value {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .field {
    position: sticky;
    top: 0;
    z-index: 1;
    padding-block-end: var(--space-1);
    background: var(--color-surface-raised);
  }

  .field :global(input),
  .range :global(input) {
    width: 100%;
    min-width: 0;
  }

  .range {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: var(--space-1-5);
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .none {
    padding: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
