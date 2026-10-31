<script>
  import Icon from '#kit/Icon.svelte';

  let { onfiles, kind = 'family', row = false } = $props();
  let open = $state(false);
  let below = $state(false);
  let room = $state(0);
  let menu = $state();
  let root = $state();
  let inputs = $state({});

  const items = $derived(kind === 'business'
    ? [
        { key: 'memory', icon: 'mic', title: 'Record a voice memo', hint: 'Capture a thought out loud' },
        { key: 'journal', icon: 'pen-line', title: 'Write it down', hint: 'A memo, an idea or a report' },
        { key: 'docs', icon: 'file-spreadsheet', title: 'Add a spreadsheet or doc', hint: 'Excel, Word, PDF or slides' },
        { key: 'photo', icon: 'camera', title: 'Take a photo' },
        { key: 'media', icon: 'images', title: 'Photos and videos' },
        { key: 'files', icon: 'paperclip', title: 'Files from your computer' },
      ]
    : [
        { key: 'memory', icon: 'mic', title: 'Record a memory', hint: 'Capture a story out loud' },
        { key: 'journal', icon: 'pen-line', title: 'Write it down', hint: 'A journal entry or an old story' },
        { key: 'photo', icon: 'camera', title: 'Take a photo' },
        { key: 'video', icon: 'video', title: 'Take a video' },
        { key: 'media', icon: 'images', title: 'Photos and videos' },
        { key: 'files', icon: 'paperclip', title: 'Files from your computer' },
      ]);

  // The four that matter most, for the home screen.
  const quick = $derived([
    { key: 'memory', icon: 'mic', title: 'Record' },
    { key: 'photo', icon: 'camera', title: 'Photo' },
    { key: 'journal', icon: 'pen-line', title: 'Write' },
    { key: kind === 'business' ? 'docs' : 'files', icon: 'paperclip', title: 'File' },
  ]);

  function choose(key) {
    open = false;
    if (key === 'memory' || key === 'journal') dispatchEvent(new CustomEvent(`capture-${key}`));
    else inputs[key].click();
  }

  // The menu opens upward unless the window is too short for it, as on Home with the box mid-screen.
  $effect(() => {
    if (!open) return;
    const box = root.getBoundingClientRect();
    const fits = menu.scrollHeight <= box.top - 16;
    below = !fits;
    room = fits ? 0 : innerHeight - box.bottom - 24;
  });

  function picked(e) {
    if (e.target.files.length) onfiles([...e.target.files]);
    e.target.value = '';
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

<div class="capture" bind:this={root}>
  <input bind:this={inputs.photo} type="file" accept="image/*" capture="environment" hidden onchange={picked} />
  <input bind:this={inputs.video} type="file" accept="video/*" capture="environment" hidden onchange={picked} />
  <input bind:this={inputs.media} type="file" accept="image/*,video/*" multiple hidden onchange={picked} />
  <input bind:this={inputs.files} type="file" multiple hidden onchange={picked} />
  <input bind:this={inputs.docs} type="file" accept=".xlsx,.xls,.csv,.docx,.doc,.pdf,.pptx,.ppt,.md,.txt" multiple hidden onchange={picked} />
  {#if row}
    <div class="quick">
      {#each quick as it (it.key)}
        <button type="button" onclick={() => choose(it.key)}><span class="glyph"><Icon name={it.icon} /></span>{it.title}</button>
      {/each}
    </div>
  {:else if open}
    <div class="menu" class:below role="menu" bind:this={menu} style={room ? `max-height: ${room}px` : undefined}>
      {#each items as it (it.key)}
        <button type="button" role="menuitem" onclick={() => choose(it.key)}>
          <span class="glyph"><Icon name={it.icon} /></span>
          <span class="words"><span>{it.title}</span>{#if it.hint}<small>{it.hint}</small>{/if}</span>
        </button>
      {/each}
    </div>
  {/if}
  {#if !row}<button class="plus" class:open type="button" aria-label="Add to your archive" aria-expanded={open} aria-haspopup="menu" title="Add to your archive" onclick={() => (open = !open)}><Icon name="plus" /></button>{/if}
</div>

<style>
  .capture { position: relative; flex: none; align-self: flex-end; }
  .capture:has(.quick) { align-self: center; width: 100%; max-width: var(--composer-width); }
  .quick { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-2); }
  .quick button { display: flex; flex-direction: column; align-items: center; gap: var(--space-1); padding: var(--space-2) 0; border-radius: var(--radius-item); color: var(--fg-muted); font-size: var(--text-meta); transition: color var(--duration-fast), transform var(--duration-fast) var(--ease-out); }
  .quick button:hover { color: var(--fg); }
  .quick button:active { transform: scale(var(--scale-press)); }
  .quick .glyph { width: 52px; height: 52px; border-radius: 50%; background: var(--surface); color: var(--primary); }
  .quick .glyph :global(.icon) { width: 22px; height: 22px; }
  .plus { display: grid; place-items: center; width: var(--composer-line); height: var(--composer-line); border-radius: var(--radius-full); background: var(--fill); color: var(--fg); transition: background var(--duration-fast), transform var(--duration-fast) var(--ease-out); }
  .plus:hover { background: var(--fill-strong); }
  .plus.open { transform: rotate(45deg); }
  .plus :global(.icon) { width: 22px; height: 22px; }
  .menu { position: absolute; bottom: calc(100% + var(--space-2)); left: 0; z-index: var(--z-nav); display: flex; flex-direction: column; gap: var(--space-0-5); width: min(300px, calc(100vw - 2 * var(--space-5))); padding: var(--space-1-5); border-radius: var(--radius-panel); background: var(--surface-raised); box-shadow: 0 8px 28px rgb(0 0 0 / 22%); }
  .menu.below { top: calc(100% + var(--space-2)); bottom: auto; overflow-y: auto; }
  .menu button { display: flex; align-items: center; gap: var(--space-3); width: 100%; padding: var(--space-1-5); border-radius: var(--radius-nav); text-align: left; transition: background var(--duration-fast); }
  .menu button:hover, .menu button:focus-visible { outline: 0; background: var(--fill); }
  .glyph { display: grid; place-items: center; flex: none; width: 36px; height: 36px; border-radius: 30%; background: var(--fill-faint); color: var(--fg); }
  .glyph :global(.icon) { width: 18px; height: 18px; }
  .words { display: flex; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  small { color: var(--fg-muted); font-size: var(--text-meta); }
</style>
