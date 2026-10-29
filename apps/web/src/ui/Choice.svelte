<script>
  import Icon from '#kit/Icon.svelte';

  let { label, items = [], value = $bindable('') } = $props();
  let open = $state(false);
  let root = $state();

  const picked = $derived(value ? value.split(',') : []);
  const names = $derived(Object.fromEntries(items));
  const text = $derived(picked.length === 0 ? label : `${label}: ${picked.length === 1 ? names[picked[0]] ?? picked[0] : picked.length}`);

  function toggle(v) {
    const now = new Set(picked);
    if (!now.delete(v)) now.add(v);
    value = [...now].join(',');
  }

  function pop(el) {
    el.showPopover?.();
  }

  $effect(() => {
    if (!open) return;
    const away = (e) => !root.contains(e.target) && (open = false);
    const esc = (e) => e.key === 'Escape' && (open = false);
    addEventListener('pointerdown', away);
    addEventListener('keydown', esc);
    return () => {
      removeEventListener('pointerdown', away);
      removeEventListener('keydown', esc);
    };
  });
</script>

<div class="choice" bind:this={root}>
  <button type="button" class="pill" class:set={picked.length} class:open aria-haspopup="menu" aria-expanded={open} onclick={() => (open = !open)}>
    <span>{text}</span><Icon name="chevron-down" />
  </button>
  {#if open}
    <div class="menu" role="menu" aria-label={label} popover="manual" {@attach pop}>
      <button type="button" class="item" role="menuitemradio" aria-checked={!picked.length} onclick={() => ((value = ''), (open = false))}>
        <span>Any</span>{#if !picked.length}<Icon name="check" />{/if}
      </button>
      {#each items as [v, name] (v)}
        {@const on = picked.includes(v)}
        <button type="button" class="item" role="menuitemcheckbox" aria-checked={on} onclick={() => toggle(v)}>
          <span>{name}</span>{#if on}<Icon name="check" />{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .choice { display: flex; flex: none; }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    height: var(--filter-height);
    padding: var(--space-2);
    border-radius: var(--radius-full);
    background: var(--fill);
    color: var(--fg-muted);
    line-height: var(--leading-none);
    text-transform: capitalize;
    white-space: nowrap;
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  .pill:hover, .pill.open { background: var(--fill-strong); color: var(--fg); }
  .pill.set { background: var(--fill-strong); color: var(--fg); }
  .pill.open { anchor-name: --filters-choice; }
  .pill :global(.icon) { width: var(--text-table); height: var(--text-table); }
  .menu {
    position: fixed;
    position-anchor: --filters-choice;
    position-area: bottom span-right;
    position-try-fallbacks: flip-inline, flip-block;
    inset: auto;
    display: flex;
    flex-direction: column;
    min-width: var(--menu-width);
    max-height: var(--measure-sm);
    margin: var(--space-1);
    padding: var(--space-1);
    overflow-y: auto;
    border: 0;
    border-radius: var(--radius-panel);
    background: var(--surface-raised);
    color: var(--fg);
  }
  .item {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-1-5);
    border-radius: var(--radius-nav);
    color: var(--fg-soft);
    font-size: var(--text-table);
    line-height: var(--leading-snug);
    text-align: left;
    text-transform: capitalize;
    white-space: nowrap;
    transition: background var(--duration-fast);
  }
  .item:hover, .item:focus-visible { outline: 0; background: var(--fill); color: var(--fg); }
  .item[aria-checked='true'] { color: var(--fg); }
</style>
