<script>
  import Icon from '#kit/Icon.svelte';
  import Avatar from './Avatar.svelte';

  let { workspaces = [], workspace = '', onpick } = $props();
  let open = $state(false);
  let root = $state();

  const here = $derived(workspaces.find((w) => w.id === workspace));

  function choose(id) {
    open = false;
    if (id !== workspace) onpick(id);
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

<div class="workspaces" bind:this={root}>
  <button class="now" type="button" aria-haspopup="menu" aria-expanded={open} aria-label="Switch workspace" onclick={() => (open = !open)}>
    <Avatar name={here?.name} />
    <span class="name">{here?.name}</span>
    <span class="updown"><Icon name="chevrons-up-down" /></span>
  </button>
  {#if open}
    <div class="menu" role="menu">
      {#each workspaces as w (w.id)}
        <button class="item" role="menuitemradio" aria-checked={w.id === workspace} onclick={() => choose(w.id)}>
          <Avatar name={w.name} />
          <span class="name">{w.name}</span>{#if w.id === workspace}<Icon name="check" />{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .workspaces { position: relative; flex: none; margin-bottom: var(--space-2); }
  .now, .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    border-radius: var(--radius-nav);
    color: var(--fg-soft);
    text-align: left;
    transition: background var(--duration-fast);
  }
  .now { padding: var(--item-pad); }
  .now:hover, .now[aria-expanded='true'] { background: var(--fill-faint); }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .updown { display: flex; color: var(--fg-muted); }
  .menu {
    position: absolute;
    top: calc(100% + var(--space-1));
    left: 0;
    z-index: var(--z-nav);
    display: flex;
    flex-direction: column;
    width: 100%;
    padding: var(--space-1);
    border-radius: var(--radius-panel);
    background: var(--surface-raised);
    animation: drop var(--duration-fast) var(--ease-out) both;
  }
  .item { padding: var(--space-1-5); font-size: var(--text-table); line-height: var(--leading-snug); }
  .item:hover, .item:focus-visible { outline: 0; background: var(--fill); color: var(--fg); }
  .item[aria-checked='true'] { color: var(--fg); }
  @keyframes drop { from { opacity: 0; transform: translateY(calc(-1 * var(--rise))); } }
  @media (prefers-reduced-motion: reduce) { .menu { animation: none; } }
</style>
