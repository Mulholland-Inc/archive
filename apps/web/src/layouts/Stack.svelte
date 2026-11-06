<script>
  import { ws, titleOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { number } from './format.js';

  let { view, rows, shown, text, pick } = $props();

  const SHADES = ['chart', 'chart-2', 'chart-3'];

  const size = $derived(shown.find((c) => c.type === 'number' && c.name !== view.by));
  const amount = (r) => (size ? Math.max(0, Number(r[size.name]) || 0) : 1);

  const all = $derived(ws().viewRows?.[view.id] ?? rows);
  const shade = $derived(
    shown.find((c) => {
      if (c.name === view.by || (c.type !== 'choice' && c.type !== 'text')) return false;
      const v = all.map((r) => text(c, r)).filter(Boolean);
      const n = new Set(v).size;
      return n > 1 && n <= 8 && n < v.length;
    })
  );
  const detail = $derived(shown.find((c) => c.type === 'text' && c.name !== view.by && c !== shade));
  const shades = $derived.by(() => {
    if (!shade) return new Map();
    const counts = new Map();
    for (const r of all) {
      const v = text(shade, r);
      if (v) counts.set(v, (counts.get(v) ?? 0) + 1);
    }
    const order = [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([v]) => v);
    return new Map(order.slice(0, SHADES.length).map((v, i) => [v, SHADES[i]]));
  });
  const shadeOf = (r) => (shade ? (shades.get(text(shade, r)) ?? 'other') : 'chart');

  const floors = $derived.by(() => {
    const out = new Map();
    for (const r of rows) {
      const f = Number(r[view.by]);
      if (r[view.by] == null || r[view.by] === '' || !Number.isFinite(f)) continue;
      if (!out.has(f)) out.set(f, []);
      out.get(f).push(r);
    }
    return [...out].sort((a, b) => b[0] - a[0]).map(([floor, units]) => {
      const total = units.reduce((n, r) => n + amount(r), 0);
      return { floor, units: units.map((r) => ({ r, share: total ? amount(r) / total : 1 / units.length })) };
    });
  });

  const legend = $derived.by(() => {
    if (!shade) return [];
    const drawn = new Set(floors.flatMap((f) => f.units.map((u) => shadeOf(u.r))));
    const named = [...shades].filter(([, s]) => drawn.has(s)).map(([name, s]) => ({ name, shade: s }));
    return drawn.has('other') ? [...named, { name: 'Other', shade: 'other' }] : named;
  });

  function fits(node) {
    const check = () => {
      for (const line of node.querySelectorAll('.line')) {
        line.removeAttribute('data-clipped');
        if (line.scrollWidth > line.clientWidth) line.setAttribute('data-clipped', '');
      }
    };
    const ro = new ResizeObserver(check);
    ro.observe(node);
    document.fonts.ready.then(check);
    return () => ro.disconnect();
  }
</script>

<div class="stack" {@attach fill()}>
  <div class="plan" style:--floors={floors.length}>
    {#each floors as f (f.floor)}
      <span class="floor">{number(f.floor)}</span>
      <div class="band">
        {#each f.units as { r, share }, i (r.id ?? i)}
          <button type="button" class={['unit', shadeOf(r)]} style:--share={share} aria-label={titleOf(r, view)} onclick={() => pick(r)} {@attach fits}>
            <span class="line name">{titleOf(r, view)}</span>
            {#if detail && text(detail, r)}<span class="line detail">{text(detail, r)}</span>{/if}
          </button>
        {/each}
      </div>
    {/each}
    {#if legend.length}
      <ul class="legend">
        {#each legend as l (l.name)}<li><span class={['swatch', l.shade]}></span>{l.name}</li>{/each}
      </ul>
    {/if}
  </div>
</div>

<style>
  .stack {
    height: var(--view-fill);
    overflow: auto;
  }

  .plan {
    display: grid;
    min-width: var(--view-stack-width);
    grid-template-columns: auto minmax(0, 1fr);
    grid-template-rows: repeat(var(--floors), var(--view-stack-floor)) auto;
    gap: var(--view-stack-gap) var(--space-3);
  }

  .floor {
    align-self: center;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    text-align: end;
  }

  .band {
    display: flex;
    min-width: 0;
    gap: var(--view-stack-gap);
  }

  .unit {
    display: flex;
    min-width: 0;
    flex: var(--share) 1 0;
    flex-direction: column;
    justify-content: center;
    padding-inline: var(--space-2-5);
    border-radius: var(--radius-xs);
    color: var(--color-on-chart);
    text-align: start;
    cursor: pointer;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .plan:has(.unit:hover) .unit:not(:hover) {
    opacity: var(--opacity-soft);
  }

  .line {
    overflow: hidden;
    white-space: nowrap;
  }

  .line[data-clipped] {
    visibility: hidden;
  }

  .name {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    line-height: var(--leading-snug);
  }

  .detail {
    font-size: var(--text-xs);
    line-height: var(--leading-snug);
  }

  .chart {
    background: var(--color-chart);
  }

  .chart-2 {
    background: var(--color-chart-2);
  }

  .chart-3 {
    background: var(--color-chart-3);
  }

  .other {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  .other .detail {
    color: var(--color-text-secondary);
  }

  .legend {
    display: flex;
    grid-column: 2;
    flex-wrap: wrap;
    margin-block-start: var(--space-3);
    gap: var(--space-1) var(--space-5);
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
  }

  .legend li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .swatch {
    width: var(--view-swatch);
    height: var(--view-swatch);
    border-radius: var(--view-square-round);
  }
</style>
