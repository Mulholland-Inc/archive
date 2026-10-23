<script lang="ts">
  import Popover from './Popover.svelte';
  import Icon from './Icon.svelte';
  import IconButton from './IconButton.svelte';
  import { iso, parseISO, monthDays, weekdays, monthLabel } from '../calendar';
  import { formatDate } from '../spec';
  let {
    value = $bindable(''),
    placeholder = 'Pick a date',
    invalid = false,
    label
  }: { value?: string; placeholder?: string; invalid?: boolean; label?: string } = $props();

  let open = $state(false);
  const today = new Date();
  let year = $state(today.getFullYear());
  let month = $state(today.getMonth());

  $effect(() => {
    if (open) {
      const d = parseISO(value) ?? new Date();
      year = d.getFullYear();
      month = d.getMonth();
    }
  });

  function shift(n: number) {
    const d = new Date(year, month + n, 1);
    year = d.getFullYear();
    month = d.getMonth();
  }
</script>

<Popover bind:open block>
  {#snippet trigger({ toggle })}
    <button class="trigger" type="button" data-invalid={invalid || undefined} data-empty={!value || undefined} aria-label={label} onclick={toggle}>
      <Icon name="calendar" />
      <span>{value ? formatDate(value) : placeholder}</span>
    </button>
  {/snippet}
  {#snippet children({ close })}
    <div class="picker">
      <div class="head">
        <IconButton icon="chevron-left" label="Previous month" onclick={() => shift(-1)} />
        <span class="month">{monthLabel(year, month)}</span>
        <IconButton icon="chevron-right" label="Next month" onclick={() => shift(1)} />
      </div>
      <div class="grid">
        {#each weekdays as w (w)}<span class="weekday">{w}</span>{/each}
        {#each monthDays(year, month) as d (iso(d))}
          <button
            type="button"
            class="day"
            data-outside={d.getMonth() !== month || undefined}
            data-today={iso(d) === iso(today) || undefined}
            data-selected={iso(d) === value || undefined}
            onclick={() => {
              value = iso(d);
              close();
            }}>{d.getDate()}</button
          >
        {/each}
      </div>
      <div class="foot">
        <button type="button" class="text" onclick={() => { value = iso(today); close(); }}>Today</button>
        {#if value}<button type="button" class="text" onclick={() => { value = ''; close(); }}>Clear</button>{/if}
      </div>
    </div>
  {/snippet}
</Popover>

<style>
  .trigger {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    height: var(--field-height);
    padding: 0 var(--space-3);
    border-radius: var(--field-radius);
    background: var(--field-background);
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border);
    text-align: start;
    color: var(--muted-foreground);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }
  .trigger:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border-hover);
  }
  .trigger span {
    color: var(--foreground);
  }
  [data-empty] span {
    color: var(--subtle-foreground);
  }
  [data-invalid] {
    box-shadow: inset 0 0 0 var(--border-width) var(--danger);
  }
  .picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2);
    border-radius: var(--menu-radius);
    background: var(--popover);
    box-shadow: var(--shadow-popover);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .month {
    font-weight: var(--font-medium);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(7, var(--control-height));
    gap: var(--space-0-5);
  }
  .weekday {
    display: grid;
    place-items: center;
    height: var(--control-height);
    font-size: var(--text-xs);
    color: var(--muted-foreground);
  }
  .day {
    height: var(--control-height);
    border-radius: var(--radius-sm);
    font-variant-numeric: tabular-nums;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .day:hover {
    background: var(--hover);
  }
  .day:active {
    transform: scale(var(--press-scale-icon));
  }
  [data-outside] {
    color: var(--subtle-foreground);
  }
  [data-today] {
    color: var(--accent);
    font-weight: var(--font-semibold);
  }
  [data-selected],
  [data-selected]:hover {
    background: var(--accent);
    color: var(--accent-foreground);
  }
  .foot {
    display: flex;
    justify-content: space-between;
    padding: var(--space-1) var(--space-1) 0;
    border-top: var(--border-width) solid var(--border);
  }
  .text {
    padding: var(--space-1);
    color: var(--accent);
    font-weight: var(--font-medium);
  }
</style>
