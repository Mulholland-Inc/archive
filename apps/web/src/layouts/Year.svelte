<script>
  import { day, short } from './format.js';

  let { view, rows, pick } = $props();

  const LEVELS = 4;
  const WEEKDAYS = [
    [0, 'Mon'],
    [2, 'Wed'],
    [4, 'Fri']
  ];
  const keyOf = (d) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;

  const dated = $derived(rows.map((r) => [day(r[view.by]), r]).filter(([d]) => d));
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const span = $derived.by(() => {
    const latest = dated.reduce((m, [d]) => (d > m ? d : m), null);
    const start = new Date(today.getFullYear() - 1, today.getMonth(), today.getDate() + 1);
    if (!latest || latest >= start) return { start, end: today, recent: true };
    return { start: new Date(latest.getFullYear(), 0, 1), end: new Date(latest.getFullYear(), 11, 31), recent: false };
  });

  const days = $derived.by(() => {
    const on = new Map();
    for (const [d, r] of dated) {
      if (d < span.start || d > span.end) continue;
      const k = keyOf(d);
      if (!on.has(k)) on.set(k, []);
      on.get(k).push(r);
    }
    const lead = (span.start.getDay() + 6) % 7;
    const out = [];
    for (let d = new Date(span.start), i = lead; d <= span.end; d.setDate(d.getDate() + 1), i++) {
      out.push({ key: keyOf(d), date: new Date(d), week: Math.floor(i / 7), weekday: i % 7, rows: on.get(keyOf(d)) ?? [] });
    }
    return out;
  });
  const weeks = $derived((days.at(-1)?.week ?? 0) + 1);
  const most = $derived(Math.max(LEVELS, ...days.map((d) => d.rows.length)));
  const level = (n) => Math.ceil((n / most) * LEVELS);
  const total = $derived(days.reduce((n, d) => n + d.rows.length, 0));

  const months = $derived.by(() => {
    const out = days.filter((d, i) => i === 0 || d.date.getDate() === 1).map((d) => ({ key: d.key, week: d.week, name: d.date.toLocaleDateString(undefined, { month: 'short' }) }));
    return out.length > 1 && out[1].week - out[0].week < 3 ? out.slice(1) : out;
  });

  let hovered = $state(null);
</script>

<div class="year">
  <div class="scroll">
    <div class="grid" style:--weeks={weeks}>
      {#each months as m (m.key)}<span class="month" style:--col={m.week + 2}>{m.name}</span>{/each}
      {#each WEEKDAYS as [w, name] (w)}<span class="weekday" style:--row={w + 2}>{name}</span>{/each}
      {#each days as d (d.key)}
        {@const tip = hovered === d.key}
        {#if d.rows.length}
          <button type="button" class={['day', `level-${level(d.rows.length)}`]} style:--col={d.week + 2} style:--row={d.weekday + 2} aria-label={`${d.rows.length} on ${short(d.date)}`} onmouseenter={() => (hovered = d.key)} onmouseleave={() => (hovered = null)} onclick={() => pick(d.rows[0])}>
            {#if tip}<span class={['tip', { below: d.weekday < 3, start: d.week < 4, end: d.week > weeks - 5 }]}>{d.rows.length} on {short(d.date)}</span>{/if}
          </button>
        {:else}
          <span class="day level-0" style:--col={d.week + 2} style:--row={d.weekday + 2} role="presentation" onmouseenter={() => (hovered = d.key)} onmouseleave={() => (hovered = null)}>
            {#if tip}<span class={['tip', { below: d.weekday < 3, start: d.week < 4, end: d.week > weeks - 5 }]}>0 on {short(d.date)}</span>{/if}
          </span>
        {/if}
      {/each}
    </div>
  </div>
  <div class="foot">
    <p>{total} {view.name.toLowerCase()} {span.recent ? 'in the last year' : `in ${span.start.getFullYear()}`}</p>
    <div class="legend" aria-hidden="true">
      Less
      {#each Array.from({ length: LEVELS + 1 }, (_, i) => i) as i (i)}<span class={['swatch', `level-${i}`]}></span>{/each}
      More
    </div>
  </div>
</div>

<style>
  .year {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .scroll {
    overflow-x: auto;
  }

  .grid {
    display: grid;
    min-width: var(--view-year-width);
    grid-template-columns: auto repeat(var(--weeks), minmax(0, 1fr));
    gap: var(--view-year-gap);
  }

  .month,
  .weekday {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    line-height: var(--leading-normal);
    white-space: nowrap;
  }

  .month {
    grid-row: 1;
    grid-column: var(--col);
  }

  .weekday {
    display: flex;
    height: 0;
    grid-row: var(--row);
    grid-column: 1;
    align-self: center;
    align-items: center;
    padding-inline-end: var(--space-2);
  }

  .day {
    position: relative;
    grid-row: var(--row);
    grid-column: var(--col);
    aspect-ratio: 1;
    border-radius: var(--view-square-round);
  }

  button.day {
    cursor: pointer;
  }

  .level-0 {
    background: var(--color-fill);
  }

  .level-1 {
    background: var(--color-chart-step-1);
  }

  .level-2 {
    background: var(--color-chart-step-2);
  }

  .level-3 {
    background: var(--color-chart-step-3);
  }

  .level-4 {
    background: var(--color-chart-step-4);
  }

  .tip {
    position: absolute;
    bottom: calc(100% + var(--space-1-5));
    left: 50%;
    z-index: var(--z-raised);
    translate: -50%;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    color: var(--color-text);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
    line-height: var(--leading-normal);
    pointer-events: none;
    white-space: nowrap;
  }

  .tip.below {
    top: calc(100% + var(--space-1-5));
    bottom: auto;
  }

  .tip.start {
    left: 0;
    translate: none;
  }

  .tip.end {
    right: 0;
    left: auto;
    translate: none;
  }

  .foot {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-2) var(--space-6);
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
  }

  .legend {
    display: flex;
    align-items: center;
    gap: var(--view-year-gap);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .swatch {
    width: var(--view-swatch);
    height: var(--view-swatch);
    border-radius: var(--view-square-round);
  }

  .swatch:first-of-type {
    margin-inline-start: var(--space-1);
  }

  .swatch:last-of-type {
    margin-inline-end: var(--space-1);
  }
</style>
