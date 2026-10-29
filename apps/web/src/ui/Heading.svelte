<script>
  import { tick } from 'svelte';

  let { title, plain = false, column = '', onrename, children } = $props();
  let editing = $state(false);
  let draft = $state('');
  let field = $state();

  async function edit() {
    draft = title;
    editing = true;
    await tick();
    field.select();
  }

  function save() {
    if (!editing) return;
    editing = false;
    const next = draft.trim();
    if (next && next !== title) onrename(next);
  }

  function key(e) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') editing = false;
  }
</script>

<header class={column}>
  {#if editing}
    <input class="title editable" bind:this={field} bind:value={draft} onblur={save} onkeydown={key} aria-label="Title" />
  {:else if onrename}
    <h1 class="plain"><button class="editable" onclick={edit} title="Rename">{title}</button></h1>
  {:else}
    <h1 class:plain>{title}</h1>
  {/if}
  {@render children?.()}
</header>

<style>
  header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-3); }
  .chat, .editor { align-self: center; width: 100%; padding-inline: var(--gutter); }
  .chat { max-width: calc(var(--chat-width) + 2 * var(--gutter)); }
  .editor { max-width: calc(var(--editor-width) + 2 * var(--gutter)); }
  h1, .title {
    font: var(--weight-medium) var(--title-size) / var(--leading-tight) var(--font-serif);
    letter-spacing: var(--tracking-tight);
    text-transform: capitalize;
    text-wrap: balance;
  }
  h1 { flex: 1; min-width: 0; }
  .plain { text-transform: none; }
  .editable {
    min-width: 0;
    max-width: 100%;
    margin: calc(-1 * var(--space-1));
    padding: var(--space-1);
    border-radius: var(--radius-item);
    text-align: left;
    transition: background var(--duration-fast);
  }
  button.editable { font: inherit; letter-spacing: inherit; }
  button.editable:hover { background: var(--fill-faint); }
  .title { flex: 1; color: var(--fg); background: var(--fill); outline: 0; text-transform: none; }
</style>
