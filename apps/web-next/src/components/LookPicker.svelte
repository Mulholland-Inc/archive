<script>
  import VaultIcon from './VaultIcon.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import { TYPES, TINTS, ICON_LABELS, KIND_TINTS } from '../lib/looks.js';

  let { look = $bindable() } = $props();

  const types = Object.entries(TYPES).map(([value, t]) => ({ value, label: t.label }));

  function setType(type) {
    look = { type, icon: TYPES[type].icons[1], tint: KIND_TINTS[type] };
  }
</script>

<div class="picker">
  <SegmentedControl value={look.type} options={types} label="Type" onchange={setType} />
  <div class="icons" role="radiogroup" aria-label="Icon">
    {#each TYPES[look.type].icons as icon (icon)}
      <button type="button" role="radio" aria-checked={look.icon === icon} aria-label={ICON_LABELS[icon]} title={ICON_LABELS[icon]} class={['choice', { on: look.icon === icon }]} onclick={() => (look = { ...look, icon })}>
        <VaultIcon {icon} tint={look.tint} size="lg" />
      </button>
    {/each}
  </div>
  <div class="tints" role="radiogroup" aria-label="Colour">
    {#each { length: TINTS } as _, i (i)}
      <button type="button" role="radio" aria-checked={look.tint === i} aria-label={`Colour ${i + 1}`} class={['swatch', `tint-${i}`, { on: look.tint === i }]} onclick={() => (look = { ...look, tint: i })}></button>
    {/each}
  </div>
</div>

<style>
  .picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .icons {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(var(--control-2xl), 1fr));
    gap: var(--space-2);
  }

  .choice {
    display: grid;
    place-items: center;
    aspect-ratio: 1;
    padding: var(--space-1);
    border-radius: var(--radius-lg);
    transition: background-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
  }

  .choice:hover {
    background: var(--color-fill-subtle);
  }

  .choice.on {
    background: var(--color-fill);
    box-shadow: inset 0 0 0 var(--focus-width) var(--color-accent);
  }

  .tints {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .swatch {
    width: var(--control-sm);
    height: var(--control-sm);
    border-radius: var(--radius-full);
    background: var(--tint);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .swatch.on {
    box-shadow: 0 0 0 var(--focus-width) var(--color-surface-raised), 0 0 0 calc(var(--focus-width) * 2) var(--color-text);
  }

  .tint-0 { --tint: var(--tint-0); }
  .tint-1 { --tint: var(--tint-1); }
  .tint-2 { --tint: var(--tint-2); }
  .tint-3 { --tint: var(--tint-3); }
  .tint-4 { --tint: var(--tint-4); }
  .tint-5 { --tint: var(--tint-5); }
  .tint-6 { --tint: var(--tint-6); }
  .tint-7 { --tint: var(--tint-7); }
</style>
