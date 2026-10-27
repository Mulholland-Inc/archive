<script module>
  let ready;
  function crdt() {
    ready ??= new Promise((resolve) => {
      window.pagesReady = () => resolve(window.pages);
      const go = new window.Go();
      WebAssembly.instantiateStreaming(fetch('/editor.wasm'), go.importObject).then((r) => go.run(r.instance));
    });
    return ready;
  }

  const TYPES = [
    { type: 'paragraph', label: 'Text', icon: 'pilcrow' },
    { type: 'heading1', label: 'Heading 1', icon: 'heading-1', hint: '#' },
    { type: 'heading2', label: 'Heading 2', icon: 'heading-2', hint: '##' },
    { type: 'heading3', label: 'Heading 3', icon: 'heading-3', hint: '###' },
    { type: 'bullet', label: 'Bulleted list', icon: 'list', hint: '-' },
    { type: 'numbered', label: 'Numbered list', icon: 'list-ordered', hint: '1.' },
    { type: 'todo', label: 'To-do', icon: 'square-check', hint: '[]' },
    { type: 'quote', label: 'Quote', icon: 'text-quote', hint: '>' },
    { type: 'callout', label: 'Callout', icon: 'lightbulb', hint: '!' },
    { type: 'code', label: 'Code', icon: 'code', hint: '```' },
  ];
  const SHORTCUTS = [
    ['### ', 'heading3'], ['## ', 'heading2'], ['# ', 'heading1'], ['- ', 'bullet'], ['* ', 'bullet'], ['1. ', 'numbered'],
    ['[] ', 'todo'], ['[ ] ', 'todo'], ['[x] ', 'done'], ['> ', 'quote'], ['! ', 'callout'], ['```', 'code'],
  ];
  const LISTS = ['bullet', 'numbered', 'todo', 'done'];
  const PLACEHOLDERS = { heading1: 'Heading 1', heading2: 'Heading 2', heading3: 'Heading 3', bullet: 'List', numbered: 'List', todo: 'To-do', done: 'To-do', quote: 'Quote', callout: 'Callout', code: 'Code' };

  const runes = (s) => [...s].length;
  const units = (s, n) => [...s].slice(0, n).join('').length;

  function offset(el, node, off) {
    const r = document.createRange();
    r.selectNodeContents(el);
    r.setEnd(node, off);
    return r.toString().length;
  }

  function selection(el) {
    const s = getSelection();
    if (!el || !s.rangeCount || !el.contains(s.anchorNode) || !el.contains(s.focusNode)) return null;
    return { anchor: offset(el, s.anchorNode, s.anchorOffset), focus: offset(el, s.focusNode, s.focusOffset) };
  }

  function point(el, n) {
    const w = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    for (let t; (t = w.nextNode()); n -= t.length) if (n <= t.length) return [t, n];
    return [el, el.childNodes.length];
  }

  function setCaret(el, anchor, focus = anchor) {
    el.focus();
    const [an, ao] = point(el, anchor), [fn, fo] = point(el, focus);
    getSelection().setBaseAndExtent(an, ao, fn, fo);
  }

  function rects(el, from, to) {
    const r = document.createRange();
    r.setStart(...point(el, from));
    r.setEnd(...point(el, to));
    return [...r.getClientRects()];
  }

  function metrics(el) {
    const cs = getComputedStyle(el);
    return { line: parseFloat(cs.lineHeight), top: parseFloat(cs.paddingTop), bottom: parseFloat(cs.paddingBottom), left: parseFloat(cs.paddingLeft) };
  }

  const sync = (text) => (el) => {
    if (el.textContent === text) return;
    const s = document.activeElement === el ? selection(el) : null;
    if (!s) return void (el.textContent = text);
    const old = el.textContent;
    let p = 0;
    while (p < old.length && p < text.length && old[p] === text[p]) p++;
    const shift = (n) => (n > p ? Math.max(p, n + text.length - old.length) : n);
    el.textContent = text;
    setCaret(el, shift(s.anchor), shift(s.focus));
  };

  const place = (m) => (el) => {
    for (const k of ['x', 'y', 'w', 'h']) el.style.setProperty(`--${k}`, `${m[k]}px`);
  };
</script>

<script>
  import { onMount, tick } from 'svelte';
  import { api, run } from './api.js';
  import Avatar from './ui/Avatar.svelte';
  import Icon from './ui/Icon.svelte';
  import Menu from './ui/Menu.svelte';

  let { id, name } = $props();
  let title = $state('');
  let live = $state(false);
  let online = $state([]);
  let blocks = $state([]);
  let marks = $state([]);
  let menu = $state(null);
  let root, heading, pages, renaming;

  const site = 2 + Math.floor(Math.random() * 2 ** 40);
  const me = { name };
  const peer = (p) => (p.agent ? 'agent' : p.site % 8);
  const textOf = (block) => root?.querySelector(`[data-id="${block}"] .text`);

  const numbers = $derived.by(() => {
    const out = {};
    let n = 0;
    for (const b of blocks) out[b.id] = n = b.type === 'numbered' ? n + 1 : 0;
    return out;
  });

  const options = $derived.by(() => {
    if (!menu) return [];
    const q = menu.query.toLowerCase();
    const found = TYPES.filter((t) => t.label.toLowerCase().includes(q) || t.type.startsWith(q));
    return menu.turn ? [...found, { type: 'delete', label: 'Delete', icon: 'trash' }] : found;
  });

  function call(fn, ...args) {
    const r = pages[fn](...args);
    if (r instanceof Error) throw r;
    return JSON.parse(r);
  }

  async function edit(f) {
    try { f(); } catch (e) { console.error(e); }
    blocks = call('blocks');
    await tick();
    cursors();
  }

  async function focus(block, at = Infinity) {
    await tick();
    const el = textOf(block);
    if (el) setCaret(el, Math.min(at, el.textContent.length));
  }

  async function input(b, el) {
    if (!el.textContent && el.firstChild) el.replaceChildren();
    const text = el.textContent;
    const at = selection(el)?.focus ?? text.length;
    const shortcut = (b.type || 'paragraph') === 'paragraph' && SHORTCUTS.find(([p]) => text.startsWith(p) && at === p.length);
    if (shortcut) {
      menu = null;
      await edit(() => {
        call('setText', b.id, text.slice(shortcut[0].length));
        call('setType', b.id, shortcut[1]);
      });
      return focus(b.id, 0);
    }
    const slash = text.slice(0, at).match(/(?:^|\s)\/([^\s/]*)$/);
    menu = slash ? { block: b.id, query: slash[1], from: at - slash[1].length - 1, to: at, index: 0 } : null;
    if (menu && !options.length) menu = null;
    await edit(() => call('setText', b.id, text));
    publish();
  }

  async function pick(o) {
    const m = menu;
    menu = null;
    const b = blocks.find((x) => x.id === m.block);
    if (!b) return;
    if (o.type === 'delete') return remove(b);
    if (m.turn) {
      await edit(() => call('setType', b.id, o.type));
      return focus(b.id);
    }
    const text = textOf(b.id).textContent;
    await edit(() => {
      call('setText', b.id, text.slice(0, m.from) + text.slice(m.to));
      call('setType', b.id, o.type);
    });
    focus(b.id, m.from);
  }

  async function remove(b) {
    const i = blocks.indexOf(b);
    if (i > 0) {
      const above = blocks[i - 1];
      await edit(() => {
        call('setText', b.id, '');
        call('merge', b.id);
      });
      return focus(above.id);
    }
    await edit(() => {
      call('setText', b.id, '');
      call('setType', b.id, 'paragraph');
    });
    focus(b.id);
  }

  async function add(b) {
    let next;
    await edit(() => {
      next = call('insert', b.id, 'paragraph');
      call('setText', next, '/');
    });
    menu = { block: next, query: '', from: 0, to: 1, index: 0 };
    focus(next);
  }

  function toggle(b) {
    edit(() => call('setType', b.id, b.type === 'done' ? 'todo' : 'done'));
  }

  function atEdge(el, up) {
    const s = getSelection();
    if (!s.rangeCount) return true;
    const r = s.getRangeAt(0).cloneRange();
    r.collapse(up);
    const rect = r.getClientRects()[0];
    if (!rect) return true;
    const box = el.getBoundingClientRect(), m = metrics(el);
    return up ? rect.top - box.top - m.top < m.line * 0.8 : box.bottom - m.bottom - rect.bottom < m.line * 0.8;
  }

  function moveTo(el, up) {
    const s = getSelection();
    const x = s.rangeCount ? s.getRangeAt(0).getClientRects()[0]?.left : undefined;
    const box = el.getBoundingClientRect(), m = metrics(el);
    const y = up ? box.bottom - m.bottom - m.line / 2 : box.top + m.top + m.line / 2;
    el.focus();
    const pos = x === undefined ? null : document.caretPositionFromPoint?.(x, y) ?? document.caretRangeFromPoint?.(x, y);
    const node = pos?.offsetNode ?? pos?.startContainer, off = pos?.offset ?? pos?.startOffset;
    if (node && el.contains(node)) s.collapse(node, off);
    else setCaret(el, up ? el.textContent.length : 0);
  }

  async function key(e, b, i) {
    const el = e.currentTarget;
    if (menu?.block === b.id && options.length) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        menu.index = (menu.index + (e.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length;
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        return pick(options[menu.index]);
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        menu = null;
        return;
      }
    }
    const s = selection(el);
    const at = s?.focus ?? 0;
    const collapsed = s && s.anchor === s.focus;
    const text = el.textContent;
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && LISTS.includes(b.type)) {
      e.preventDefault();
      if (b.type !== 'bullet' && b.type !== 'numbered') toggle(b);
    } else if (e.key === 'Enter' && !e.shiftKey && (b.type !== 'code' || e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      menu = null;
      if (LISTS.includes(b.type) && !text) return edit(() => call('setType', b.id, 'paragraph'));
      const type = b.type === 'done' ? 'todo' : LISTS.includes(b.type) ? b.type : 'paragraph';
      let next;
      await edit(() => {
        call('setText', b.id, text);
        next = call('split', b.id, runes(text.slice(0, at)), type);
      });
      focus(next, 0);
    } else if (e.key === 'Backspace' && collapsed && at === 0) {
      e.preventDefault();
      if (b.type && b.type !== 'paragraph') return edit(() => call('setType', b.id, 'paragraph'));
      if (i === 0) return;
      const above = blocks[i - 1];
      await edit(() => call('merge', b.id));
      focus(above.id, above.text.length);
    } else if (e.key === 'ArrowUp' && !e.shiftKey && atEdge(el, true)) {
      e.preventDefault();
      if (i > 0) moveTo(textOf(blocks[i - 1].id), true);
      else setCaret(heading, heading.textContent.length);
    } else if (e.key === 'ArrowDown' && !e.shiftKey && atEdge(el, false) && i < blocks.length - 1) {
      e.preventDefault();
      moveTo(textOf(blocks[i + 1].id), false);
    } else if (e.key === 'Escape') {
      menu = null;
    }
  }

  function retitle() {
    title = heading.textContent;
    clearTimeout(renaming);
    const file = `${title.trim() || 'Untitled'}.page`;
    renaming = setTimeout(() => run(`await archive.sql("UPDATE file SET name = $1 WHERE id = $2", ${JSON.stringify(file)}, ${JSON.stringify(id)})`).catch(console.error), 400);
  }

  function titleKey(e) {
    if (e.key !== 'Enter' && e.key !== 'ArrowDown') return;
    e.preventDefault();
    if (blocks[0]) focus(blocks[0].id, 0);
  }

  let lastPresence = '';
  async function publish(force) {
    const el = document.activeElement?.classList.contains('text') && root.contains(document.activeElement) ? document.activeElement : null;
    const b = el && blocks.find((x) => x.id === el.closest('[data-id]').dataset.id);
    const s = b && selection(el);
    const anchor = s ? runes(el.textContent.slice(0, s.anchor)) : 0;
    const head = s ? runes(el.textContent.slice(0, s.focus)) : 0;
    const k = [b?.id ?? '', anchor, head].join(':');
    if (k === lastPresence && !force) return;
    lastPresence = k;
    const u = call('presence', b?.id ?? '', anchor, head, JSON.stringify(me));
    await fetch(`/api/pages/${id}/presence`, { method: 'POST', body: bytes(u) });
  }

  const bytes = (b64) => Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));

  function cursors() {
    if (!pages || !root) return;
    const peers = call('peers');
    online = peers;
    const box = root.getBoundingClientRect();
    const out = [];
    for (const p of peers) {
      const b = blocks.find((x) => x.id === p.headBlock);
      const el = b && textOf(b.id);
      if (!el) continue;
      const m = metrics(el);
      const at = units(b.text, p.head);
      const e = el.getBoundingClientRect();
      const r = rects(el, at, at)[0] ?? { left: e.left + m.left, top: e.top + m.top, height: m.line };
      out.push({ kind: 'caret', p, x: r.left - box.left, y: r.top - box.top, w: 0, h: r.height || m.line });
      if (p.anchorBlock === p.headBlock && p.anchor !== p.head) {
        const a = units(b.text, Math.min(p.anchor, p.head)), z = units(b.text, Math.max(p.anchor, p.head));
        for (const q of rects(el, a, z)) out.push({ kind: 'sel', p, x: q.left - box.left, y: q.top - box.top, w: q.width, h: q.height });
      }
    }
    marks = out;
  }

  onMount(() => {
    let es, timer;
    api(`/api/files/${id}`).then(async (f) => {
      title = f.name.replace(/\.page$/, '');
      if (title !== 'Untitled') return;
      title = '';
      await tick();
      heading.focus();
    });

    async function flush() {
      const ops = call('flush');
      if (ops) await fetch(`/api/pages/${id}/ops`, { method: 'POST', body: bytes(ops) });
    }

    const onSelection = () => publish().catch(console.error);
    const onBlur = () => setTimeout(onSelection);
    const onPointer = (e) => { if (!e.target.closest?.('.menu, .grip')) menu = null; };

    crdt().then((p) => {
      pages = p;
      es = new EventSource(`/api/pages/${id}/sync?site=${site}`);
      es.addEventListener('snapshot', async (e) => {
        call('load', e.data, site);
        if (call('blocks').length === 0) call('insert', '', 'paragraph');
        await edit(() => {});
        publish(true).catch(console.error);
        live = true;
      });
      es.addEventListener('presence', (e) => {
        try { call('applyPresence', e.data); } catch (err) { console.error(err); }
        cursors();
      });
      es.onmessage = (e) => edit(() => call('apply', e.data));
      es.onerror = () => (live = false);
      timer = setInterval(() => flush().catch(console.error), 150);
      document.addEventListener('selectionchange', onSelection);
      document.addEventListener('focusout', onBlur);
      document.addEventListener('pointerdown', onPointer);
      addEventListener('resize', cursors);
    });

    return () => {
      es?.close();
      clearInterval(timer);
      if (pages) flush().catch(console.error);
      document.removeEventListener('selectionchange', onSelection);
      document.removeEventListener('focusout', onBlur);
      document.removeEventListener('pointerdown', onPointer);
      removeEventListener('resize', cursors);
    };
  });
</script>

<header>
  <a class="crumb" href="/pages"><Icon name="chevron-left" />Pages</a>
  <div class="people">
    <span class="dot" class:live title={live ? 'Live' : 'Connecting'}></span>
    {#each [{ ...me, site, you: true }, ...online] as p (p.site)}
      <Avatar name={p.name} peer={peer(p)} pulse={p.agent} title={p.you ? `${p.name} (you)` : p.agent ? `${p.name} editing` : p.name} />
    {/each}
  </div>
</header>

<article>
  <h1
    class="title"
    contenteditable="plaintext-only"
    role="textbox"
    tabindex="0"
    data-placeholder="Untitled"
    bind:this={heading}
    {@attach sync(title)}
    oninput={retitle}
    onkeydown={titleKey}
  ></h1>

  <div class="blocks" bind:this={root}>
    {#each blocks as b, i (b.id)}
      <div class="block {b.type || 'paragraph'}" class:todo={b.type === 'done'} class:lonely={blocks.length === 1} data-id={b.id}>
        <div class="gutter">
          <button tabindex="-1" aria-label="Add a block below" onmousedown={(e) => e.preventDefault()} onclick={() => add(b)}><Icon name="plus" /></button>
          <button class="grip" tabindex="-1" aria-label="Turn into" onclick={() => (menu = { block: b.id, turn: true, query: '', index: 0 })}><Icon name="grip-vertical" /></button>
        </div>
        {#if b.type === 'todo' || b.type === 'done'}
          <button class="check" tabindex="-1" aria-label={b.type === 'done' ? 'Mark as not done' : 'Mark as done'} onmousedown={(e) => e.preventDefault()} onclick={() => toggle(b)}>
            {#if b.type === 'done'}<Icon name="check" />{/if}
          </button>
        {:else if b.type === 'bullet'}
          <span class="mark">•</span>
        {:else if b.type === 'numbered'}
          <span class="mark">{numbers[b.id]}.</span>
        {:else if b.type === 'callout'}
          <span class="mark"><Icon name="lightbulb" /></span>
        {/if}
        <div
          class="text"
          contenteditable="plaintext-only"
          role="textbox"
          tabindex="0"
          data-placeholder={PLACEHOLDERS[b.type] ?? (blocks.length === 1 ? "Write something, or press '/' for commands" : "Press '/' for commands")}
          {@attach sync(b.text)}
          oninput={(e) => input(b, e.currentTarget)}
          onkeydown={(e) => key(e, b, i)}
        ></div>
        {#if menu?.block === b.id && options.length}
          <div class="popover">
            <Menu title={menu.turn ? 'Turn into' : 'Blocks'} items={options} index={menu.index} onpick={pick} onhover={(n) => (menu.index = n)} />
          </div>
        {/if}
      </div>
    {/each}
    {#each marks as m, n (n)}
      <div class="{m.kind} peer-{peer(m.p)}" {@attach place(m)}>{#if m.kind === 'caret'}<b>{m.p.name}</b>{/if}</div>
    {/each}
  </div>
</article>

<style>
  header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); max-width: var(--measure); margin: calc(-1 * var(--space-7)) auto var(--space-7); }
  .crumb { display: flex; align-items: center; gap: var(--space-1); font-size: var(--text-sm); color: var(--text-3); transition: color var(--dur) var(--ease); }
  .crumb:hover { color: var(--text); }
  .people { display: flex; align-items: center; gap: var(--space-1); }
  .dot { width: var(--size-xs); height: var(--size-xs); border-radius: 50%; background: var(--line-strong); margin-right: var(--space-2); transition: background var(--dur) var(--ease); }
  .dot.live { background: var(--cream); }

  article { max-width: var(--measure); margin: 0 auto; padding-bottom: var(--space-9); }
  [contenteditable] { outline: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  [contenteditable]:empty::before { content: attr(data-placeholder); color: var(--text-3); pointer-events: none; }
  .title {
    margin-bottom: var(--space-5);
    font: var(--weight-medium) var(--display-lg)/var(--leading-tight) var(--display);
    letter-spacing: var(--tracking-display);
  }

  .blocks { position: relative; }
  .block { position: relative; display: flex; align-items: flex-start; gap: var(--space-2); padding: var(--stroke) 0; }
  .text { flex: 1; min-width: 0; padding: var(--space-1) 0; font-size: var(--text-lg); line-height: var(--leading-loose); }
  .paragraph:not(:focus-within):not(.lonely) .text:empty::before { content: none; }

  .gutter {
    position: absolute;
    right: 100%;
    top: var(--space-1);
    display: flex;
    padding-right: var(--space-2);
    opacity: 0;
    transition: opacity var(--dur) var(--ease);
  }
  .block:hover .gutter, .gutter:focus-within { opacity: 1; }
  .gutter button, .check {
    display: grid;
    place-items: center;
    width: var(--size-avatar);
    height: var(--size-avatar);
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-3);
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .gutter button:hover { background: var(--hover); color: var(--text); }
  .gutter :global(svg.icon), .check :global(svg.icon), .mark :global(svg.icon) { width: var(--text-lg); height: var(--text-lg); }

  .mark { flex: none; min-width: var(--size-avatar); padding-top: var(--space-1); font-size: var(--text-lg); line-height: var(--leading-loose); color: var(--text-2); text-align: center; }
  .check { flex: none; margin-top: var(--space-2); width: var(--text-xl); height: var(--text-xl); border: var(--stroke-bold) solid var(--line-strong); border-radius: var(--radius-sm); color: var(--cream-ink); }
  .check:hover { border-color: var(--line-focus); }
  .done .check { background: var(--cream); border-color: var(--cream); }
  .done .text { color: var(--text-3); text-decoration: line-through; text-decoration-color: var(--line-strong); }

  .heading1 { margin-top: var(--space-6); }
  .heading2 { margin-top: var(--space-5); }
  .heading3 { margin-top: var(--space-4); }
  .heading1 .text { font: var(--weight-medium) var(--display-md)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-display); }
  .heading2 .text { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); }
  .heading3 .text { font-size: var(--text-xl); font-weight: var(--weight-bold); }
  .heading1 .gutter, .heading2 .gutter { top: var(--space-3); }

  .quote .text { padding-left: var(--space-4); border-left: var(--stroke-bold) solid var(--cream); color: var(--text-2); }
  .callout { padding: var(--space-3) var(--space-4); margin: var(--space-2) 0; background: var(--panel); border-radius: var(--radius-lg); }
  .callout .gutter { top: var(--space-4); }
  .code .text {
    padding: var(--space-4) var(--space-5);
    margin: var(--space-2) 0;
    background: var(--panel);
    border-radius: var(--radius-md);
    font: var(--text-sm)/var(--leading) var(--mono);
    font-variant-ligatures: none;
  }
  .code .gutter { top: var(--space-5); }

  .popover { position: absolute; top: 100%; left: 0; z-index: 1; margin-top: var(--space-1); }

  .caret, .sel { position: absolute; left: var(--x); top: var(--y); height: var(--h); background: var(--peer); pointer-events: none; }
  .caret { width: var(--stroke-bold); transition: left var(--dur-fast), top var(--dur-fast); }
  .sel { width: var(--w); opacity: 0.22; }
  .caret b {
    position: absolute;
    bottom: 100%;
    left: 0;
    padding: var(--stroke) var(--space-2);
    border-radius: var(--radius-sm) var(--radius-sm) var(--radius-sm) 0;
    background: var(--peer);
    color: var(--on-peer);
    font: var(--weight-bold) var(--text-xs) var(--font);
    white-space: nowrap;
  }

  @media (max-width: 767px) {
    header { margin: 0 0 var(--space-5); }
    .gutter { display: none; }
  }
</style>
