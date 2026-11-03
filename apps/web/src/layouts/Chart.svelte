<script>
  import Table from '../components/Table.svelte';
  import { fill } from './fill.js';
  import { label, money, number, day, steps } from './format.js';

  // A chart: for each value of the view's by column, the total of its first money column, or failing that its
  // first number, or else how many rows have it, as columns over a baseline, with the same totals in a table
  // under it. A date by gives a column for each month from the first to the last, or for each day when they are
  // all in one month; any other by a column for each of its values, largest first, past twelve folded into Other.
  let { view, rows, shown, text } = $props();

  // A column's widest and the rounding of its top, in pixels, as the marks are drawn.
  const BAR = 24;
  const ROUND = 4;
  const MOST = 12;

  const byCol = $derived(view.columns.find((c) => c.name === view.by));
  const measure = $derived(shown.find((c) => c.type === 'money') ?? shown.find((c) => c.type === 'number' && c.name !== view.by));
  const currency = $derived(measure?.type === 'money' ? rows.find((r) => r[measure.name]?.currency)?.[measure.name].currency : undefined);
  const amount = (r) => (!measure ? 1 : Number(measure.type === 'money' ? r[measure.name]?.amount : r[measure.name]) || 0);
  const format = (n, compact = false) => (currency ? money(n, currency, compact) : number(n, compact));

  const bars = $derived.by(() => {
    if (byCol?.type === 'date') {
      const dated = rows.map((r) => [day(r[view.by]), r]).filter(([d]) => d);
      if (!dated.length) return [];
      const times = dated.map(([d]) => d.getTime());
      const first = new Date(Math.min(...times));
      const last = new Date(Math.max(...times));
      const daily = first.getFullYear() === last.getFullYear() && first.getMonth() === last.getMonth();
      const keyOf = (d) => (daily ? d.getDate() : d.getFullYear() * 12 + d.getMonth());
      const out = [];
      for (let d = new Date(first.getFullYear(), first.getMonth(), daily ? first.getDate() : 1); d <= last; daily ? d.setDate(d.getDate() + 1) : d.setMonth(d.getMonth() + 1)) {
        const year = out.length === 0 || (!daily && d.getMonth() === 0);
        out.push({
          key: String(keyOf(d)),
          short: daily ? String(d.getDate()) : d.toLocaleDateString(undefined, year ? { month: 'short', year: 'numeric' } : { month: 'short' }),
          long: d.toLocaleDateString(undefined, daily ? { day: 'numeric', month: 'long', year: 'numeric' } : { month: 'long', year: 'numeric' }),
          value: 0
        });
      }
      const at = new Map(out.map((b) => [b.key, b]));
      for (const [d, r] of dated) at.get(String(keyOf(d))).value += amount(r);
      return out;
    }
    const sums = new Map();
    for (const r of rows) {
      const k = (byCol && text(byCol, r)) || 'None';
      sums.set(k, (sums.get(k) ?? 0) + amount(r));
    }
    const sorted = [...sums].sort((a, b) => b[1] - a[1]);
    const kept = sorted.length > MOST ? [...sorted.slice(0, MOST - 1), ['Other', sorted.slice(MOST - 1).reduce((n, [, v]) => n + v, 0)]] : sorted;
    return kept.map(([k, value]) => ({ key: k, short: k, long: k, value }));
  });

  // The axis runs from 0 to a round number past the largest total.
  const ticks = $derived(steps(0, Math.max(0, ...bars.map((b) => b.value))));
  const top = $derived(ticks.at(-1));

  let width = $state(0);
  let height = $state(0);
  let hovered = $state(null);
  const band = $derived(bars.length ? width / bars.length : 0);
  const thick = $derived(Math.min(BAR, band * 0.6));
  // A column grows from the baseline, square there and rounded at its top.
  function column(i, value) {
    const h = (value / top) * height;
    const r = Math.min(ROUND, thick / 2, h);
    const x = band * i + (band - thick) / 2;
    const y = height - h;
    return `M${x},${height}V${y + r}A${r},${r} 0 0 1 ${x + r},${y}H${x + thick - r}A${r},${r} 0 0 1 ${x + thick},${y + r}V${height}Z`;
  }

  const columns = $derived([
    { key: 'long', label: byCol ? label(byCol) : 'Value' },
    { key: 'total', label: measure ? label(measure) : 'Rows', align: 'end' }
  ]);
  const cells = $derived(bars.map((b) => ({ id: b.key, long: b.long, total: format(b.value) })));
</script>

<div class="chart-view" {@attach fill()}>
  <div class="chart">
    <div class="ticks">
      {#each ticks as t (t)}<span style:--y={`${(t / top) * 100}%`}>{format(t, true)}</span>{/each}
    </div>
    <div class="plot" bind:clientWidth={width} bind:clientHeight={height}>
      <svg {width} {height} aria-hidden="true">
        {#each ticks as t (t)}<line class="grid" x1="0" x2={width} y1={height - (t / top) * height} y2={height - (t / top) * height} />{/each}
        {#each bars as b, i (b.key)}
          {#if b.value > 0}<path class={['bar', { dim: hovered !== null && hovered !== i }]} d={column(i, b.value)} />{/if}
          <rect class="hit" x={band * i} y="0" width={band} {height} role="presentation" onmouseenter={() => (hovered = i)} onmouseleave={() => (hovered = null)} />
        {/each}
      </svg>
      {#if hovered !== null && bars[hovered]}
        {@const b = bars[hovered]}
        <div class="tip" style:--x={`${((hovered + 0.5) / bars.length) * 100}%`} style:--y={`${(b.value / top) * 100}%`}>
          <span class="tip-label">{b.long}</span>
          <span class="tip-value">{format(b.value)}</span>
        </div>
      {/if}
    </div>
    <div class="labels">
      {#each bars as b, i (b.key)}<span style:--x={`${(i / bars.length) * 100}%`} style:--band={`${100 / bars.length}%`}>{b.short}</span>{/each}
    </div>
  </div>

  <div class="table">
    <Table {columns} rows={cells} label={view.name} flush />
  </div>
</div>

<style>
  .chart-view {
    display: grid;
    height: var(--view-fill);
    grid-template-rows: auto minmax(0, 1fr);
    gap: var(--space-8);
  }

  /* The axis's numbers on the left, as wide as the widest, then the plot, and under it the columns' names. */
  .chart {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    grid-template-rows: var(--view-chart-height) auto;
    gap: var(--space-2) var(--space-3);
  }

  .ticks {
    display: grid;
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    text-align: end;
  }

  /* Every number sits in the one cell, moved up from its foot to its own gridline. */
  .ticks span {
    position: relative;
    grid-area: 1 / 1;
    align-self: end;
    bottom: var(--y);
    translate: 0 50%;
    line-height: var(--leading-none);
  }

  .plot {
    position: relative;
    min-width: 0;
  }

  svg {
    display: block;
    overflow: visible;
  }

  .grid {
    stroke: var(--color-border);
    stroke-width: var(--border-width);
    shape-rendering: crispEdges;
  }

  .bar {
    fill: var(--color-chart);
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .bar.dim {
    opacity: var(--opacity-soft);
  }

  .hit {
    fill: transparent;
  }

  .labels {
    position: relative;
    grid-column: 2;
    height: calc(var(--text-xs) * var(--leading-normal));
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    line-height: var(--leading-normal);
  }

  .labels span {
    position: absolute;
    left: var(--x);
    width: var(--band);
    overflow: hidden;
    text-align: center;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  /* The total of the column under the cursor, over its top. */
  .tip {
    position: absolute;
    left: var(--x);
    bottom: var(--y);
    z-index: var(--z-raised);
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    translate: -50% calc(-1 * var(--space-2));
    padding: var(--space-1-5) var(--space-2-5);
    border-radius: var(--radius-sm);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    pointer-events: none;
    white-space: nowrap;
  }

  .tip-label {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .tip-value {
    color: var(--color-text);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
  }

  /* The table scrolls in its own box, its header row staying at the top. */
  .table {
    min-height: 0;
  }

  .table :global(.wrap) {
    height: 100%;
    overflow: auto;
  }

  .table :global(th) {
    position: sticky;
    top: 0;
    z-index: var(--z-raised);
    background: var(--color-canvas);
  }
</style>
