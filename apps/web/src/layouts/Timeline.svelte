<script>
  import { titleOf } from '../lib/app.svelte.js';
  import { fill } from './fill.js';
  import { label, day } from './format.js';

  // A timeline: each row its name, its two dates and a bar between them over the months they span, for projects
  // and the like. A row starts on the view's by date and ends on the first date column after it; one with no end
  // is a dot on its start, and one with no start has no bar. A line marks today. The months stay at the top and
  // the rows scroll under them.
  let { view, rows, shown, pick } = $props();

  const at = $derived(shown.findIndex((c) => c.name === view.by));
  const endCol = $derived(at < 0 ? undefined : shown.slice(at + 1).find((c) => c.type === 'date'));
  const startOf = (r) => day(r[view.by]);
  const endOf = (r) => (endCol && day(r[endCol.name])) || startOf(r);

  // The timeline runs from the first of the month of the earliest start to the end of the month of the latest end.
  const range = $derived.by(() => {
    const times = rows.flatMap((r) => [startOf(r), endOf(r)]).filter(Boolean).map((d) => d.getTime());
    if (!times.length) return null;
    const first = new Date(Math.min(...times));
    const last = new Date(Math.max(...times));
    return [new Date(first.getFullYear(), first.getMonth(), 1), new Date(last.getFullYear(), last.getMonth() + 1, 1)];
  });
  const place = (d) => `${((d - range[0]) / (range[1] - range[0])) * 100}%`;
  const months = $derived.by(() => {
    if (!range) return [];
    const out = [];
    for (let d = new Date(range[0]); d < range[1]; d.setMonth(d.getMonth() + 1)) {
      const year = out.length === 0 || d.getMonth() === 0;
      out.push({ key: d.getTime(), x: place(d), name: d.toLocaleDateString(undefined, year ? { month: 'short', year: 'numeric' } : { month: 'short' }) });
    }
    return out;
  });
  const now = new Date();
  const today = $derived(range && now >= range[0] && now < range[1] ? place(now) : null);
  const thisYear = now.getFullYear();
  const short = (d) => (d ? d.toLocaleDateString(undefined, d.getFullYear() === thisYear ? { month: 'short', day: 'numeric' } : { month: 'short', day: 'numeric', year: 'numeric' }) : '');
  // A bar reaches to the end of its last day.
  const span = (r) => `${((endOf(r) - startOf(r) + 864e5) / (range[1] - range[0])) * 100}%`;
</script>

<div class="timeline" {@attach fill()}>
  <div class="row head">
    <span>Name</span>
    <span>{label(shown[at] ?? { name: view.by })}</span>
    <span>{endCol ? label(endCol) : ''}</span>
    <span class="track">
      {#each months as m (m.key)}<span class="month" style:--x={m.x}>{m.name}</span>{/each}
    </span>
  </div>
  <div class="scroll">
    <div class="body">
      <!-- The month lines and today's line run down behind every row. -->
      <div class="row lines" aria-hidden="true">
        <span class="track">
          {#each months as m (m.key)}<span class="line" style:--x={m.x}></span>{/each}
          {#if today}<span class="line today" style:--x={today}></span>{/if}
        </span>
      </div>
      {#each rows as r, i (r.id ?? i)}
        <button type="button" class="row item" onclick={() => pick(r)}>
          <span class="name">{titleOf(r, view)}</span>
          <span class="date">{short(startOf(r))}</span>
          <span class="date">{endCol ? short(day(r[endCol.name])) : ''}</span>
          <span class="track">
            {#if range && startOf(r)}<span class="bar" style:--x={place(startOf(r))} style:--span={span(r)}></span>{/if}
          </span>
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  .timeline {
    display: flex;
    height: var(--view-fill);
    flex-direction: column;
  }

  /* Every row, the head's too, has the same columns: the name, the two dates, and the track the bars run on. */
  .row {
    display: grid;
    grid-template-columns: minmax(0, var(--view-timeline-name)) var(--view-timeline-date) var(--view-timeline-date) minmax(0, 1fr);
    gap: var(--space-4);
    align-items: center;
  }

  .head {
    height: var(--control-lg);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .track {
    position: relative;
    align-self: stretch;
  }

  .month {
    position: absolute;
    top: 50%;
    left: var(--x);
    translate: 0 -50%;
    padding-inline-start: var(--space-1-5);
    white-space: nowrap;
  }

  /* The rows scroll in a box as much wider than the timeline as a row's hover reaches out past it. */
  .scroll {
    min-height: 0;
    flex: 1;
    margin-inline: calc(-1 * var(--space-3));
    padding-inline: var(--space-3);
    overflow-y: auto;
  }

  .body {
    position: relative;
  }

  .lines {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .lines .track {
    grid-column: 4;
    height: 100%;
  }

  .line {
    position: absolute;
    top: 0;
    bottom: 0;
    left: var(--x);
    width: var(--border-width);
    background: var(--color-border-subtle);
  }

  .line.today {
    background: var(--color-text-tertiary);
  }

  /* A row's text starts where the head's does, and its divider with it; its hover reaches out past them. */
  .item {
    position: relative;
    width: calc(100% + 2 * var(--space-3));
    height: var(--row-height);
    margin-inline: calc(-1 * var(--space-3));
    padding-inline: var(--space-3);
    border-radius: var(--radius-md);
    font-size: var(--text-md);
    text-align: start;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .item::after {
    content: '';
    position: absolute;
    right: var(--space-3);
    bottom: 0;
    left: var(--space-3);
    height: var(--border-width);
    background: var(--color-border-subtle);
  }

  .item:last-child::after {
    display: none;
  }

  .item:hover {
    background: var(--color-fill-subtle);
  }

  .name {
    overflow: hidden;
    color: var(--color-text);
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .date {
    color: var(--color-text-secondary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .bar {
    position: absolute;
    top: 50%;
    left: var(--x);
    width: var(--span);
    min-width: var(--view-timeline-bar);
    height: var(--view-timeline-bar);
    translate: 0 -50%;
    border-radius: var(--radius-full);
    background: var(--color-chart);
  }
</style>
