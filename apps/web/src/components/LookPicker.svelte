<script>
  import ArchiveIcon from './ArchiveIcon.svelte';
  import AgentAvatar from './AgentAvatar.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import { TYPES, TINTS, ICON_LABELS, KIND_TINTS, AVATAR_LABELS } from '../lib/looks.js';

  let { look = $bindable(), avatars } = $props();

  const types = Object.entries(TYPES).map(([value, t]) => ({ value, label: t.label }));

  // A single line keeps what is picked in view, and a mouse wheel scrolls it sideways.
  let iconRow = $state();
  let tintRow = $state();

  function reveal(node) {
    const on = node?.querySelector('.on');
    if (!on) return;
    if (on.offsetLeft < node.scrollLeft || on.offsetLeft + on.offsetWidth > node.scrollLeft + node.clientWidth) node.scrollLeft = on.offsetLeft - (node.clientWidth - on.offsetWidth) / 2;
  }

  $effect(() => {
    look.icon;
    look.avatar;
    reveal(iconRow);
  });

  $effect(() => {
    look.tint;
    reveal(tintRow);
  });

  function row(node) {
    const wheel = (event) => {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      node.scrollLeft += event.deltaY;
      event.preventDefault();
    };
    node.addEventListener('wheel', wheel, { passive: false });
    return { destroy: () => node.removeEventListener('wheel', wheel) };
  }

  function setType(type) {
    look = { type, icon: TYPES[type].icons[1], tint: KIND_TINTS[type] };
  }
</script>

<div class="picker">
  {#if !avatars}<SegmentedControl value={look.type} options={types} label="Type" onchange={setType} />{/if}
  <div class="icons" role="radiogroup" aria-label="Icon" bind:this={iconRow} use:row>
    {#if avatars}
      {#each avatars as avatar (avatar)}
        <button type="button" role="radio" aria-checked={look.avatar === avatar} aria-label={AVATAR_LABELS[avatar]} title={AVATAR_LABELS[avatar]} class={['choice', { on: look.avatar === avatar }]} onclick={() => (look = { ...look, avatar })}>
          <AgentAvatar avatar={{ figure: avatar }} tint={look.tint} size="lg" />
        </button>
      {/each}
    {:else}
      {#each TYPES[look.type].icons as icon (icon)}
        <button type="button" role="radio" aria-checked={look.icon === icon} aria-label={ICON_LABELS[icon]} title={ICON_LABELS[icon]} class={['choice', { on: look.icon === icon }]} onclick={() => (look = { ...look, icon })}>
          <ArchiveIcon {icon} tint={look.tint} size="lg" />
        </button>
      {/each}
    {/if}
  </div>
  <div class="tints" role="radiogroup" aria-label="Colour" bind:this={tintRow} use:row>
    {#each { length: TINTS } as _, i (i)}
      <button type="button" role="radio" aria-checked={look.tint === i} aria-label={`Colour ${i + 1}`} class={['swatch', { on: look.tint === i }]} style={`--tint: var(--tint-${i})`} onclick={() => (look = { ...look, tint: i })}></button>
    {/each}
  </div>
</div>

<style>
  .picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .choice {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-2xl);
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

  .icons,
  .tints {
    position: relative;
    display: flex;
    flex: none;
    gap: var(--space-2);
    margin: calc(var(--space-1) * -1);
    padding: var(--space-1);
    overflow-x: auto;
    scrollbar-width: none;
  }

  .swatch {
    flex: none;
    width: var(--control-sm);
    height: var(--control-sm);
    border-radius: var(--radius-full);
    background: var(--tint);
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .swatch.on {
    box-shadow: 0 0 0 var(--focus-width) var(--color-surface-raised), 0 0 0 calc(var(--focus-width) * 2) var(--color-text);
  }
</style>
