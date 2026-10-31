<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { look, shortName, ui } from './looks.svelte.js';

  let { workspaces = [], workspace = '', latest = '', canAdd = false, onpick, onedit, onadd } = $props();
</script>

<div class="bots">
  {#each workspaces as w (w.id)}
    <div class="wrap">
      <button class="row" class:on={w.id === workspace} type="button" aria-current={w.id === workspace ? 'true' : undefined} onclick={() => onpick(w.id)}>
        {#if ui.sidebarIcons}<Bot seed={w.id} />{/if}
        <span class="text">
          <span class="name">{shortName(w.name, look(w.id).type)}</span>
          {#if w.id === workspace}<span class="sub">{latest || 'Ask me anything'}</span>{/if}
        </span>
      </button>
      {#if w.id === workspace}
        <button class="pencil" type="button" aria-label="Edit {w.name}" title="Edit" onclick={() => onedit(w)}><Icon name="pencil" /></button>
      {/if}
    </div>
  {/each}
  {#if canAdd}
    <button class="row add" type="button" onclick={onadd}>{#if ui.sidebarIcons}<span class="plus"><Icon name="plus" /></span>{/if}<span>Add archive</span></button>
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
  .add { color: var(--fg-muted); }
  .plus { display: grid; place-items: center; flex: none; width: 40px; height: 40px; border: var(--hairline) dashed var(--fg-muted); border-radius: 30%; color: var(--fg-muted); }
</style>
