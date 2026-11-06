<script>
  import Text from '../components/Text.svelte';
  import Stack from '../components/Stack.svelte';
  import { titleOf } from '../lib/app.svelte.js';

  let { view, rows, pick } = $props();

  const month = $derived.by(() => {
    const days = rows.map((r) => r[view.by]).filter(Boolean).map((d) => new Date(d));
    const now = new Date();
    const next = days.filter((d) => d >= new Date(now.getFullYear(), now.getMonth(), 1)).sort((a, b) => a - b)[0] ?? now;
    return new Date(next.getFullYear(), next.getMonth(), 1);
  });
  const weeks = $derived.by(() => {
    const first = new Date(month);
    first.setDate(1 - ((first.getDay() + 6) % 7));
    return Array.from({ length: 6 }, (_, w) => Array.from({ length: 7 }, (_, d) => new Date(first.getFullYear(), first.getMonth(), first.getDate() + w * 7 + d)));
  });
  const key = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  const onDay = $derived.by(() => {
    const out = new Map();
    for (const r of rows) {
      const v = r[view.by];
      if (!v) continue;
      const k = String(v).slice(0, 10);
      if (!out.has(k)) out.set(k, []);
      out.get(k).push(r);
    }
    return out;
  });
</script>

<Stack gap="2">
  <Text weight="medium">{month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</Text>
  <div class="calendar">
    {#each ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] as d (d)}<Text size="sm" tone="tertiary">{d}</Text>{/each}
    {#each weeks.flat() as d (key(d))}
      <div class={['day', { other: d.getMonth() !== month.getMonth() }]}>
        <Text size="sm" tone="tertiary" tabular>{d.getDate()}</Text>
        {#each onDay.get(key(d)) ?? [] as r, i (r.id ?? i)}
          <button type="button" class="event" onclick={() => pick(r)}>{titleOf(r, view)}</button>
        {/each}
      </div>
    {/each}
  </div>
</Stack>

<style>
  .calendar {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: var(--space-px);
  }

  .day {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-height: var(--view-day-height);
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    background: var(--color-fill);
    min-width: 0;
  }

  .other {
    opacity: 0.5;
  }

  .event {
    display: block;
    width: 100%;
    text-align: start;
    cursor: pointer;
    overflow-wrap: anywhere;
    line-height: var(--leading-tight);
    padding: var(--space-0-5) var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--color-fill-strong);
    font-size: var(--text-sm);
  }
</style>
