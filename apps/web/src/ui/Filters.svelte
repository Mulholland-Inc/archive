<script>
  import Bar from './Bar.svelte';
  import Search from './Search.svelte';
  import Select from './Select.svelte';
  import { native, call, on } from '../native.js';

  let { columns, types = {}, query = $bindable(''), filters = $bindable({}), extra = [], onextra, children } = $props();
  const name = (c) => c.name.replaceAll('_', ' ');
  const options = (c) => {
    const values = c.type === 'bool' ? [['true', 'yes'], ['false', 'no']] : (types[`${c.schema}.${c.type}`]?.enum ?? []).map((v) => [v, v.replaceAll('_', ' ')]);
    return values.length ? [{ value: '', label: 'Any' }, ...values.map(([value, label]) => ({ value, label }))] : null;
  };
  const controls = $derived([
    ...columns.map((c) => {
      const items = options(c);
      return { key: c.name, label: name(c), kind: items ? (c.type === 'bool' ? 'bool' : 'enum') : 'text', options: items ?? [], value: filters[c.name] ?? '' };
    }),
    ...extra,
  ]);

  $effect(() => {
    if (native) call('filters', { show: true, search: query, controls: $state.snapshot(controls) });
  });

  $effect(() => {
    if (!native) return;
    const offFilter = on('filter', ({ key, value }) => {
      if (extra.some((x) => x.key === key)) onextra?.(key, value);
      else filters = { ...filters, [key]: value };
    });
    const offSearch = on('search', ({ text }) => (query = text));
    return () => {
      offFilter();
      offSearch();
      call('filters', { show: false });
    };
  });
</script>

{#if !native}
  <Bar>
    <Search bind:value={query} placeholder="Search" aria-label="Search" />
    {#each columns as c (c.name)}
      {@const items = options(c)}
      {#if items}
        <Select label={name(c)} {items} bind:value={() => filters[c.name] ?? '', (v) => (filters = { ...filters, [c.name]: v })} />
      {:else}
        <Search icon={null} placeholder={name(c)} aria-label={name(c)} bind:value={() => filters[c.name] ?? '', (v) => (filters = { ...filters, [c.name]: v })} />
      {/if}
    {/each}
    {@render children?.()}
  </Bar>
{/if}
