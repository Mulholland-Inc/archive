<script>
  import Icon from '#kit/Icon.svelte';
  import Avatar from './Avatar.svelte';

  let { name, email = '', dev = false, ondev, onsignout } = $props();
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
      <span class="name">{name}</span>
      <span class="space">{email}</span>
    </span>
    <span class="updown"><Icon name="chevrons-up-down" /></span>
  </button>
  {#if open}
    <div class="menu" role="menu">
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
  .name, .space { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { color: var(--fg-soft); }
  .space { color: var(--fg-muted); font-size: var(--text-meta); }
  .updown { display: flex; color: var(--fg-muted); }
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
  .new, .add { justify-content: flex-start; color: var(--fg-muted); }
  hr { flex: none; min-height: var(--hairline); margin: var(--space-1); border: 0; }
  .add { background: var(--fill-faint); }
  .add.failed { color: var(--danger); }
  .add input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  .add input::placeholder { color: var(--fg-muted); }
  @keyframes rise { from { opacity: 0; transform: translateY(var(--rise)); } }
  @media (prefers-reduced-motion: reduce) { .menu { animation: none; } }
</style>
