<script module>
  const money = /^[-+\u2212]?\s*(?:[€$£¥]|EUR|USD|GBP)?\s*[-+\u2212]?\d[\d,]*(?:\.\d+)?\s*(?:%|[€$£¥]|EUR|USD|GBP)?$/i;

  function number(s) {
    if (!money.test(s)) return null;
    const n = parseFloat(s.replace(/\u2212/g, '-').replace(/[^\d.-]/g, ''));
    return Number.isNaN(n) ? null : n;
  }

  function date(s) {
    if (!/\d/.test(s) || !/[a-z]{3}|\d{4}-\d{2}/i.test(s)) return null;
    const t = Date.parse(s.replace(/^[a-z]{3},?\s+/i, ''));
    return Number.isNaN(t) ? null : t;
  }

  export function sorter(rows, col) {
    const texts = rows.map((r) => r.text[col]).filter(Boolean);
    const as = texts.length && texts.every((t) => number(t) !== null) ? number : texts.length && texts.every((t) => date(t) !== null) ? date : null;
    return (r) => {
      const t = r.text[col];
      if (!t) return null;
      return as ? as(t) : t;
    };
  }
</script>

<script>
  import Button from '#kit/Button.svelte';
  import DataTable from './DataTable.svelte';
  import { cycle } from './filter.js';

  let { columns, rows } = $props();
  let sort = $state(null);
  let all = $state(false);
  const LIMIT = 12;

  const sorted = $derived.by(() => {
    if (!sort) return rows;
    const k = sorter(rows, sort.col);
    const dir = sort.desc ? -1 : 1;
    return [...rows].sort((a, b) => {
      const x = k(a);
      const y = k(b);
      if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
      return dir * (typeof x === 'number' ? x - y : String(x).localeCompare(String(y), undefined, { numeric: true, sensitivity: 'base' }));
    });
  });
  const shown = $derived(all ? sorted : sorted.slice(0, LIMIT));
</script>

<div class="box">
  <DataTable fill {columns} rows={shown} key={(r) => rows.indexOf(r)} {sort} onsort={(col) => (sort = cycle(sort, col))}>
    {#snippet td(r, c)}{@html r.html[c.name]}{/snippet}
  </DataTable>
  {#if rows.length > LIMIT}
    <div class="more"><Button ghost small onclick={() => (all = !all)}>{all ? 'Show less' : `Show all ${rows.length}`}</Button></div>
  {/if}
</div>

<style>
  .box { --table-bleed: 0px; display: flex; flex-direction: column; min-width: 0; overflow: clip; border-radius: var(--radius-panel); background: var(--surface); }
  .more { display: flex; justify-content: center; padding: var(--space-2); }
</style>
