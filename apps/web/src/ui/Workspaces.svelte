<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { look, TYPES } from './looks.svelte.js';

  let { workspaces = [], workspace = '', latest = '', canAdd = false, onpick, onrename, onadd } = $props();
  let editing = $state('');
  let adding = $state(false);
  let draft = $state('');
  let error = $state('');
  let kind = $state('family');

  function reset() {
    editing = '';
    adding = false;
    draft = '';
    error = '';
  }

  function rename(w) {
    reset();
    editing = w.id;
    draft = w.name;
  }

  function add() {
    reset();
    adding = true;
  }

  async function save(e) {
    e.preventDefault();
    const name = draft.trim();
    if (!name) return reset();
    try {
      if (adding) await onadd(name, kind);
      else if (name !== workspaces.find((w) => w.id === editing)?.name) await onrename(name);
      reset();
    } catch (err) {
      error = err.status === 403 ? 'Ask your admin to do this.' : err.message;
    }
  }

  const key = (e) => e.key === 'Escape' && reset();
</script>

{#snippet field(label)}
  <form class="edit" onsubmit={save}>
    <input bind:value={draft} placeholder="Name your archive" aria-label={label} onkeydown={key} onblur={() => !error && !draft.trim() && reset()} {@attach (el) => el.select()} />
    {#if adding}
      <span class="kinds">
        {#each Object.entries(TYPES) as [key, t] (key)}
          <button type="button" class:on={kind === key} onmousedown={(e) => e.preventDefault()} onclick={() => (kind = key)}>{t.label}</button>
        {/each}
      </span>
    {/if}
    {#if error}<span class="error">{error}</span>{/if}
  </form>
{/snippet}

<div class="bots">
  {#each workspaces as w (w.id)}
    <div class="wrap">
      {#if editing === w.id}
        <div class="row on"><Bot seed={w.id} />{@render field('Archive name')}</div>
      {:else}
        <button class="row" class:on={w.id === workspace} type="button" aria-current={w.id === workspace ? 'true' : undefined} onclick={() => onpick(w.id)}>
          <Bot seed={w.id} />
          <span class="text">
            <span class="name">{w.name}</span>
            {#if w.id === workspace}<span class="sub">{latest || 'Ask me anything'}</span>{:else if look(w.id).shared}<span class="sub">Shared with you</span>{/if}
          </span>
        </button>
        {#if w.id === workspace}
          <button class="pencil" type="button" aria-label="Rename {w.name}" title="Rename" onclick={() => rename(w)}><Icon name="pencil" /></button>
        {/if}
      {/if}
    </div>
  {/each}
  {#if !canAdd}
  {:else if adding}
    <div class="row"><span class="plus"><Icon name="plus" /></span>{@render field('New archive name')}</div>
  {:else}
    <button class="row add" type="button" onclick={add}><span class="plus"><Icon name="plus" /></span><span>Add archive</span></button>
  {/if}
</div>

<style>
  .bots { display: flex; flex: none; flex-direction: column; gap: var(--space-0-5); margin-bottom: var(--space-3); }
  .wrap { position: relative; }
  .row { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-1-5); border-radius: var(--radius-panel); text-align: left; transition: background var(--duration-fast); }
  button.row:hover { background: var(--fill-faint); }
  .on, button.on:hover { background: var(--fill); }
  .text { display: flex; flex: 1; flex-direction: column; min-width: 0; padding-right: var(--space-5); line-height: var(--leading-snug); }
  .name, .sub { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { color: var(--fg); font-weight: var(--weight-medium); }
  .sub { color: var(--fg-muted); font-size: var(--text-meta); }
  .pencil { position: absolute; top: 50%; right: var(--space-1-5); display: grid; place-items: center; width: 28px; height: 28px; transform: translateY(-50%); border-radius: var(--radius-full); color: var(--fg-muted); opacity: 0; transition: opacity var(--duration-fast); }
  .wrap:hover .pencil, .pencil:focus-visible { opacity: 1; }
  .pencil:hover { color: var(--fg); }
  @media (hover: none) { .pencil { opacity: 1; } }
  .pencil :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .edit { display: flex; flex: 1; flex-direction: column; gap: var(--space-0-5); min-width: 0; }
  input { width: 100%; padding: 0; color: var(--fg); font-weight: var(--weight-medium); outline: 0; }
  input::placeholder { color: var(--fg-muted); font-weight: var(--weight-regular, 400); }
  .error { color: var(--danger); font-size: var(--text-meta); }
  .add { color: var(--fg-muted); }
  .kinds { display: flex; gap: var(--space-0-5); }
  .kinds button { padding: 1px var(--space-1-5); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); }
  .kinds .on { background: var(--fill-strong); color: var(--fg); }
  .plus { display: grid; place-items: center; flex: none; width: 40px; height: 40px; border: var(--hairline) dashed var(--fg-muted); border-radius: 30%; color: var(--fg-muted); }
</style>
