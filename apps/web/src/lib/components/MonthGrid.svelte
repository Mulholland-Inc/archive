<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';
  import IconButton from './IconButton.svelte';
  import Button from './Button.svelte';
  import { iso, parseISO, monthDays, weekdays, monthLabel } from '../calendar';
  let {
    items,
    date,
    key,
    item
  }: { items: T[]; date: (item: T) => string | undefined; key: (item: T) => string; item: Snippet<[T]> } = $props();

  const today = new Date();
  let year = $state(today.getFullYear());
  let month = $state(today.getMonth());

  const byDay = $derived.by(() => {
    const m = new Map<string, T[]>();
    for (const it of items) {
      const d = parseISO(date(it));
      if (!d) continue;
      const k = iso(d);
      if (!m.has(k)) m.set(k, []);
      m.get(k)!.push(it);
    }
    return m;
  });

  function shift(n: number) {
    const d = new Date(year, month + n, 1);
    year = d.getFullYear();
    month = d.getMonth();
  }
</script>

<div class="calendar">
  <div class="head">
    <span class="title">{monthLabel(year, month)}</span>
    <div class="nav">
      <Button variant="ghost" onclick={() => { year = today.getFullYear(); month = today.getMonth(); }}>Today</Button>
      <IconButton icon="chevron-left" label="Previous month" onclick={() => shift(-1)} />
      <IconButton icon="chevron-right" label="Next month" onclick={() => shift(1)} />
    </div>
  </div>
  <div class="grid">
    {#each weekdays as w (w)}<span class="weekday">{w}</span>{/each}
    {#each monthDays(year, month) as d (iso(d))}
      <div class="day" data-outside={d.getMonth() !== month || undefined} data-today={iso(d) === iso(today) || undefined}>
        <span class="num">{d.getDate()}</span>
        {#each byDay.get(iso(d)) ?? [] as it (key(it))}{@render item(it)}{/each}
      </div>
    {/each}
  </div>
</div>

<style>
  .calendar {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .title {
    font-size: var(--text-md);
    font-weight: var(--font-semibold);
  }
  .nav {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    border-radius: var(--radius-lg);
    background: var(--card);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    overflow: hidden;
  }
  .weekday {
    padding: var(--space-2) var(--space-2-5);
    font-size: var(--text-sm);
    color: var(--muted-foreground);
    border-bottom: var(--border-width) solid var(--border);
  }
  .day {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
    min-height: var(--calendar-cell-height);
    padding: var(--space-1-5);
    border-right: var(--border-width) solid var(--border);
    border-bottom: var(--border-width) solid var(--border);
  }
  .day:nth-child(7n) {
    border-right: none;
  }
  .day:nth-last-child(-n + 7) {
    border-bottom: none;
  }
  [data-outside] {
    background: var(--tint-1);
  }
  [data-outside] .num {
    color: var(--subtle-foreground);
  }
  .num {
    padding: 0 var(--space-1);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }
  [data-today] .num {
    color: var(--accent);
    font-weight: var(--font-semibold);
  }
</style>
