<script>
  import Icon from '#kit/Icon.svelte';
  import Avatar from './Avatar.svelte';
  import Theme from '#kit/Theme.svelte';
  import { setFloating, ui } from './looks.svelte.js';

  let { name, dev = false, ondev, onsignout, spaces = [], home, current = '', onpick, oncreate } = $props();
  let adding = $state(false);
  let draft = $state('');
  let failed = $state(false);

  const label = (s) => (s.id === home ? 'Personal' : s.name.replaceAll('_', ' '));
  const here = $derived(spaces.find((s) => s.name === current));

  function pick(space) {
    open = false;
    if (space !== current) onpick(space);
  }

  async function create(e) {
    e.preventDefault();
    const n = draft.trim().toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_]/g, '');
    if (!n) return;
    try {
      await oncreate(n);
      open = adding = failed = false;
      draft = '';
    } catch {
      failed = true;
    }
  }

  $effect(() => {
    if (!open) adding = failed = false;
  });
  let open = $state(false);
  let root = $state();


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

<div class="profile" bind:this={root}>
  <button class="me" type="button" aria-haspopup="menu" aria-expanded={open} aria-label="Account" onclick={() => (open = !open)}>
    <Avatar {name} />
    <span class="text">
      <span class="name" title={name}>{name}</span>
      {#if dev}<span class="space">{here ? label(here) : 'All spaces'}</span>{/if}
    </span>
  </button>
  {#if open}
    <div class="menu" role="menu">
      {#if dev}
        <button class="item" role="menuitemradio" aria-checked={!current} onclick={() => pick('')}>
          <span>All spaces</span>{#if !current}<Icon name="check" />{/if}
        </button>
        {#each spaces as s (s.id)}
          <button class="item" role="menuitemradio" aria-checked={s.name === current} onclick={() => pick(s.name)}>
            <span>{label(s)}</span>{#if s.name === current}<Icon name="check" />{/if}
          </button>
        {/each}
        {#if adding}
          <form class="item add" class:failed onsubmit={create}>
            <Icon name="plus" />
            <input bind:value={draft} placeholder="Space name" aria-label="Space name" {@attach (el) => el.focus()} oninput={() => (failed = false)} />
          </form>
        {:else}
          <button class="item new" role="menuitem" onclick={() => (adding = true)}><Icon name="plus" /><span>New space</span></button>
        {/if}
        <hr />
      {/if}
      <div class="item plain"><span>Appearance</span><Theme /></div>
      <button class="item" role="menuitemcheckbox" aria-checked={ui.floating} onclick={() => setFloating(!ui.floating)}>
        <span>Floating icons</span>{#if ui.floating}<Icon name="check" />{/if}
      </button>
      <button class="item" role="menuitemcheckbox" aria-checked={dev} onclick={() => { open = false; ondev(); }}>
        <span>Developer mode</span>{#if dev}<Icon name="check" />{/if}
      </button>
      <button class="item" role="menuitem" onclick={onsignout}><span>Log out</span></button>
    </div>
  {/if}
</div>

<style>
  .profile { position: relative; display: flex; flex: 1; min-width: 0; }
  .me {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--item-pad);
    border-radius: var(--radius-nav);
    text-align: left;
    transition: background var(--duration-fast);
  }
  .me:hover, .me[aria-expanded='true'] { background: var(--fill-faint); }
  .text { display: flex; flex: 1; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  .space { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { display: -webkit-box; overflow: hidden; color: var(--fg); overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .space { color: var(--fg-muted); font-size: var(--text-meta); }
  .menu {
    position: absolute;
    bottom: calc(100% + var(--space-1));
    left: 0;
    z-index: var(--z-nav);
    display: flex;
    flex-direction: column;
    min-width: var(--menu-width);
    padding: var(--space-1);
    border-radius: var(--radius-panel);
    background: var(--surface-raised);
    animation: rise var(--duration-fast) var(--ease-out) both;
  }
  .item {
    display: flex;
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
  .plain { padding-block: var(--space-0-5); }
  .plain:hover { background: none; }
  .new, .add { justify-content: flex-start; color: var(--fg-muted); }
  hr { flex: none; min-height: var(--hairline); margin: var(--space-1); border: 0; }
  .add { background: var(--fill-faint); }
  .add.failed { color: var(--danger); }
  .add input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  .add input::placeholder { color: var(--fg-muted); }
  @keyframes rise { from { opacity: 0; transform: translateY(var(--rise)); } }
  @media (prefers-reduced-motion: reduce) { .menu { animation: none; } }
</style>
