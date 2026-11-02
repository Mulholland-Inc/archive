<script>
  import { Search, CornerDownLeft } from '@lucide/svelte';
  import Dialog from './Dialog.svelte';
  import Icon from './Icon.svelte';
  import Kbd from './Kbd.svelte';
  import { uid } from '../lib/ids.js';

  let { open = $bindable(false), commands = [] } = $props();

  const listId = uid('cmd');
  let query = $state('');
  let index = $state(0);
  let list = $state();

  const results = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const hits = q ? commands.filter((c) => c.label.toLowerCase().includes(q) || c.group.toLowerCase().includes(q)) : commands.filter((c) => !c.searchOnly);
    return hits.slice(0, 40);
  });

  const grouped = $derived.by(() => {
    const out = [];
    for (const [i, c] of results.entries()) {
      let g = out.find((x) => x.name === c.group);
      if (!g) out.push((g = { name: c.group, items: [] }));
      g.items.push({ ...c, i });
    }
    return out;
  });

  $effect(() => {
    query;
    index = 0;
  });

  $effect(() => {
    if (!open) query = '';
  });

  function run(command) {
    open = false;
    command?.run();
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      index = Math.min(results.length - 1, index + 1);
      scroll();
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      index = Math.max(0, index - 1);
      scroll();
    } else if (event.key === 'Enter') {
      event.preventDefault();
      run(results[index]);
    }
  }

  function scroll() {
    requestAnimationFrame(() => list?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }));
  }
</script>

<Dialog bind:open size="palette" bare>
  <div class="palette">
    <div class="search">
      <Icon icon={Search} size="md" />
      <input
        bind:value={query}
        data-autofocus
        placeholder="Search chats, files and commands"
        aria-label="Search"
        role="combobox"
        aria-expanded="true"
        aria-controls={listId}
        aria-activedescendant={results[index] ? `${listId}-${index}` : undefined}
        {onkeydown}
      />
      <Kbd keys="Esc" />
    </div>
    <div class="results" bind:this={list} id={listId} role="listbox" aria-label="Results">
      {#if !results.length}
        <div class="none">No results for “{query}”</div>
      {/if}
      {#each grouped as group (group.name)}
        <div class="group" role="group" aria-label={group.name}>
          <div class="label">{group.name}</div>
          {#each group.items as item (item.id)}
            <button
              type="button"
              id={`${listId}-${item.i}`}
              role="option"
              aria-selected={item.i === index}
              tabindex="-1"
              class="option"
              onclick={() => run(item)}
              onpointermove={() => (index = item.i)}
            >
              {#if item.icon}<Icon icon={item.icon} size="md" />{/if}
              <span class="text">{item.label}</span>
              {#if item.meta}<span class="meta">{item.meta}</span>{/if}
              {#if item.shortcut}<Kbd keys={item.shortcut} subtle />{/if}
              {#if item.i === index}<span class="enter"><Icon icon={CornerDownLeft} size="sm" /></span>{/if}
            </button>
          {/each}
        </div>
      {/each}
    </div>
    <div class="foot">
      <span><Kbd keys="↑ ↓" /> Navigate</span>
      <span><Kbd keys="↵" /> Open</span>
      <span><Kbd keys="⌘ K" /> Toggle</span>
    </div>
  </div>
</Dialog>

<style>
  .palette {
    display: flex;
    flex-direction: column;
    max-height: var(--palette-max-height);
  }

  .search {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    color: var(--color-text-tertiary);
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  input {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    font-size: var(--text-base);
  }

  input::placeholder {
    color: var(--color-text-tertiary);
  }

  input:focus-visible {
    outline: none;
  }

  .results {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-2);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
  }

  .label {
    padding: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  .option {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--control-lg);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    color: var(--color-text);
    text-align: start;
  }

  .option > :global(.icon) {
    color: var(--color-text-secondary);
  }

  .option[aria-selected='true'] {
    background: var(--color-fill);
  }

  .text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    white-space: nowrap;
  }

  .enter {
    display: inline-flex;
    color: var(--color-text-tertiary);
  }

  .none {
    padding: var(--space-8);
    color: var(--color-text-tertiary);
    text-align: center;
  }

  .foot {
    display: flex;
    gap: var(--space-4);
    padding: var(--space-3);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .foot span {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
  }
</style>
