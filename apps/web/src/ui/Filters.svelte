<script>
  import Bar from './Bar.svelte';
  import Search from './Search.svelte';
  import Select from './Select.svelte';

  let { columns, types = {}, query = $bindable(''), filters = $bindable({}), children } = $props();
  const name = (c) => c.name.replaceAll('_', ' ');
  const options = (c) => {
    const values = c.type === 'bool' ? [['true', 'yes'], ['false', 'no']] : (types[`${c.schema}.${c.type}`]?.enum ?? []).map((v) => [v, v.replaceAll('_', ' ')]);
    return values.length ? [{ value: '', label: 'Any' }, ...values.map(([value, label]) => ({ value, label }))] : null;
  };
</script>

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
