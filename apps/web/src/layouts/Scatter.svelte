<script>
  import { titleOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { label, money, number, steps } from './format.js';

  let { view, rows, shown, pick } = $props();

  const DOT = 4;
  const ON = 6;
  const REACH = 24;

  const xCol = $derived(view.columns.find((c) => c.name === view.by));
  const yCol = $derived(shown.find((c) => (c.type === 'money' || c.type === 'number') && c.name !== view.by));
  const valueOf = (c, r) => (c?.type === 'money' ? r[c.name]?.amount : r[c?.name]);
  const currencyOf = (c) => (c?.type === 'money' ? rows.find((r) => r[c.name]?.currency)?.[c.name].currency : undefined);
  const axis = (c) => {
    const currency = currencyOf(c);
    return { title: c ? label(c) : '', format: (n, compact = false) => (currency ? money(n, currency, compact) : number(n)) };
  };
  const across = $derived(axis(xCol));
  const up = $derived(axis(yCol));

  const points = $derived(
    rows
      .map((r) => ({ r, x: Number(valueOf(xCol, r)), y: Number(valueOf(yCol, r)) }))
      .filter((p) => valueOf(xCol, p.r) != null && valueOf(yCol, p.r) != null && Number.isFinite(p.x) && Number.isFinite(p.y))
  );
  const range = (vs) => (vs.length ? steps(Math.min(...vs), Math.max(...vs)) : steps(0, 0));
  const xt = $derived(range(points.map((p) => p.x)));
  const yt = $derived(range(points.map((p) => p.y)));
  const share = (t, v) => (v - t[0]) / (t.at(-1) - t[0]);

  let width = $state(0);
  let height = $state(0);
  let hovered = $state(null);
  const px = (p) => share(xt, p.x) * width;
  const py = (p) => height - share(yt, p.y) * height;
  const inside = (v, max) => Math.min(Math.max(v, 0.5), max - 0.5);
  const grid = $derived(
    [...xt.map((t) => `M${inside(share(xt, t) * width, width)},0V${height}`), ...yt.map((t) => `M0,${inside(height - share(yt, t) * height, height)}H${width}`)].join('')
  );

  function near(event) {
    const box = event.currentTarget.getBoundingClientRect();
    const [cx, cy] = [event.clientX - box.left, event.clientY - box.top];
    let best = null;
    let reach = REACH;
    points.forEach((p, i) => {
      const d = Math.hypot(px(p) - cx, py(p) - cy);
      if (d <= reach) [best, reach] = [i, d];
    });
    hovered = best;
  }
</script>

<div class="scatter" {@attach fill()}>
  <span class="title up">{up.title}</span>
  <div class="ticks">
    {#each yt as t (t)}<span style:--y={`${share(yt, t) * 100}%`}>{up.format(t, true)}</span>{/each}
  </div>
  <div class="plot" bind:clientWidth={width} bind:clientHeight={height}>
    <svg {width} {height} class={{ on: hovered !== null }} role="presentation" onpointermove={near} onpointerleave={() => (hovered = null)} onclick={() => hovered !== null && pick(points[hovered].r)}>
      <path class="grid" d={grid} />
      {#each points as p, i (p.r.id ?? i)}<circle class="dot" cx={px(p)} cy={py(p)} r={i === hovered ? ON : DOT} />{/each}
    </svg>
    {#if hovered !== null && points[hovered]}
      {@const p = points[hovered]}
      {@const at = share(xt, p.x)}
      <div class={['tip', { start: at < 0.25, end: at > 0.75 }]} style:--x={`${at * 100}%`} style:--y={`${share(yt, p.y) * 100}%`}>
        <span class="tip-name">{titleOf(p.r, view)}</span>
        <dl>
          <dt>{across.title}</dt><dd>{across.format(p.x)}</dd>
          <dt>{up.title}</dt><dd>{up.format(p.y)}</dd>
        </dl>
      </div>
    {/if}
  </div>
  <div class="labels">
    {#each xt as t (t)}<span style:--x={`${share(xt, t) * 100}%`}>{across.format(t, true)}</span>{/each}
  </div>
  <span class="title across">{across.title}</span>
</div>

<style>
  .scatter {
    display: grid;
    height: var(--view-fill);
    grid-template-columns: auto minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr) auto auto;
    gap: var(--space-2) var(--space-3);
  }

  .title {
    color: var(--color-text-secondary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    line-height: var(--leading-normal);
  }

  .up {
    grid-column: 1 / -1;
  }

  .across {
    grid-column: 2;
    text-align: end;
  }

  .ticks {
    display: grid;
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    text-align: end;
  }

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

  svg.on {
    cursor: pointer;
  }

  .grid {
    fill: none;
    stroke: var(--color-border);
    stroke-width: var(--border-width);
    shape-rendering: crispEdges;
  }

  .dot {
    fill: var(--color-chart);
    stroke: var(--color-canvas);
    stroke-width: var(--space-0-5);
  }

  .labels {
    position: relative;
    grid-column: 2;
    height: calc(var(--text-xs) * var(--leading-normal));
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    line-height: var(--leading-normal);
  }

  .labels span {
    position: absolute;
    left: var(--x);
    translate: -50%;
    white-space: nowrap;
  }

  .labels span:last-child {
    translate: -100%;
  }

  .tip {
    position: absolute;
    left: var(--x);
    bottom: var(--y);
    z-index: var(--z-raised);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    translate: -50% calc(-1 * var(--space-3));
    padding: var(--space-2) var(--space-2-5);
    border-radius: var(--radius-sm);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    pointer-events: none;
    white-space: nowrap;
  }

  .tip.start {
    translate: calc(-1 * var(--space-3)) calc(-1 * var(--space-3));
  }

  .tip.end {
    translate: calc(-100% + var(--space-3)) calc(-1 * var(--space-3));
  }

  .tip-name {
    color: var(--color-text);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  dl {
    display: grid;
    grid-template-columns: auto auto;
    gap: var(--space-0-5) var(--space-4);
    font-size: var(--text-xs);
  }

  dt {
    color: var(--color-text-tertiary);
  }

  dd {
    color: var(--color-text-secondary);
    font-variant-numeric: tabular-nums;
    text-align: end;
  }
</style>
