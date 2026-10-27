<script module>
  const TYPES = [
    { type: 'paragraph', label: 'Text', icon: 'pilcrow' },
    { type: 'heading1', label: 'Heading 1', icon: 'heading-1' },
    { type: 'heading2', label: 'Heading 2', icon: 'heading-2' },
    { type: 'heading3', label: 'Heading 3', icon: 'heading-3' },
    { type: 'bullet', label: 'Bulleted list', icon: 'list' },
    { type: 'numbered', label: 'Numbered list', icon: 'list-ordered' },
    { type: 'todo', label: 'To-do', icon: 'square-check' },
    { type: 'quote', label: 'Quote', icon: 'text-quote' },
    { type: 'code', label: 'Code', icon: 'square-code' },
  ];
  const LISTS = ['bullet', 'numbered', 'todo'];
  const MARKS = [
    { name: 'bold', label: 'Bold', icon: 'bold', key: 'b', expand: 4 },
    { name: 'italic', label: 'Italic', icon: 'italic', key: 'i', expand: 4 },
    { name: 'underline', label: 'Underline', icon: 'underline', key: 'u', expand: 4 },
    { name: 'strike', label: 'Strikethrough', icon: 'strikethrough', key: 'x', expand: 4 },
    { name: 'code', label: 'Inline code', icon: 'code', key: 'e', expand: 0 },
    { name: 'link', label: 'Link', icon: 'link', key: 'k', expand: 0 },
  ];
  const PLACEHOLDERS = { heading1: 'Heading 1', heading2: 'Heading 2', heading3: 'Heading 3', bullet: 'List', numbered: 'List', todo: 'To-do', quote: 'Quote', code: 'Code' };

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

  const escape = (t) => t.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('\u00a0', '&nbsp;');
  const quote = (t) => t.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('\u00a0', '&nbsp;');

  function markup(spans) {
    return spans.map((s) => {
      const names = MARKS.map((m) => m.name).filter((n) => n in s.marks);
      if (!names.length) return escape(s.text);
      return `<span class="${names.join(' ')}"${'link' in s.marks ? ` title="${quote(s.marks.link)}"` : ''}>${escape(s.text)}</span>`;
    }).join('');
  }

  function write(el, text, set) {
    const s = document.activeElement === el ? selection(el) : null;
    const old = el.textContent;
    set();
    if (!s) return;
    let p = 0;
    while (p < old.length && p < text.length && old[p] === text[p]) p++;
    const shift = (n) => (n > p ? Math.max(p, n + text.length - old.length) : n);
    setCaret(el, shift(s.anchor), shift(s.focus));
  }

  const sync = (text) => (el) => {
    if (el.textContent !== text) write(el, text, () => (el.textContent = text));
  };

  const paint = (b) => (el) => {
    const html = markup(b.spans ?? []);
    if (el.innerHTML !== html) write(el, b.text, () => (el.innerHTML = html));
  };

  function marksIn(b, from, to) {
    if (from === to) [from, to] = from > 0 ? [from - 1, from] : [0, 1];
    let at = 0, out = null;
    for (const s of b.spans ?? []) {
      const n = runes(s.text);
      if (at < to && at + n > from) out = out ? out.filter((k) => k in s.marks) : Object.keys(s.marks);
      at += n;
    }
    return out ?? [];
  }

  const place = (m) => (el) => {
    for (const k of ['x', 'y', 'w', 'h']) el.style.setProperty(`--${k}`, `${m[k]}px`);
  };
</script>

<script>
  import { onMount, tick } from 'svelte';
  import { api, editor, run } from './api.js';
  import Bar from './ui/Bar.svelte';
  import People from './ui/People.svelte';
  import Heading from './ui/Heading.svelte';
  import Icon from './ui/Icon.svelte';
  import Menu from './ui/Menu.svelte';
  import Tool from './ui/Tool.svelte';
  import { native, call as bridge, on as listen } from './native.js';
  import { top } from './top.svelte.js';

  let { id, name } = $props();
  let title = $state('');
  let live = $state(false);
  let online = $state([]);
  let blocks = $state([]);
  let marks = $state([]);
  let menu = $state(null);
  let cur = $state(null);
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

  const crowd = $derived([{ ...me, site, you: true }, ...online].map((p) => ({ key: p.site, name: p.you ? `${p.name} (you)` : p.name, pulse: p.agent })));

  $effect(() => {
    top.people = crowd;
    top.live = live;
  });

  $effect(() => () => (top.people = null));

  const active = $derived(cur && blocks.find((b) => b.id === cur.id));
  const on = $derived(active ? marksIn(active, Math.min(cur.anchor, cur.focus), Math.max(cur.anchor, cur.focus)) : []);
  const options = $derived(menu?.turn ? [...TYPES, { type: 'delete', label: 'Delete', icon: 'trash' }] : TYPES);

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
    await edit(() => call('setText', b.id, el.textContent));
    publish();
  }

  async function pick(o) {
    const m = menu;
    menu = null;
    const b = blocks.find((x) => x.id === m.block);
    if (!b) return;
    if (o.type === 'delete') {
      const i = blocks.indexOf(b);
      if (blocks.length === 1) {
        await edit(() => { call('setText', b.id, ''); call('setType', b.id, 'paragraph'); });
        return focus(b.id);
      }
      await edit(() => call('remove', b.id));
      return focus(blocks[Math.max(0, i - 1)].id);
    }
    if (m.turn) {
      await edit(() => call('setType', b.id, o.type));
      return focus(b.id);
    }
    let next;
    await edit(() => (next = call('insert', b.id, o.type)));
    focus(next);
  }

  function toggle(b) {
    edit(() => call('setField', b.id, 'done', b.done ? '' : '1'));
  }

  function track() {
    const el = document.activeElement;
    const b = el?.classList.contains('text') && root?.contains(el) && blocks.find((x) => x.id === el.closest('[data-id]').dataset.id);
    const s = b && selection(el);
    cur = s ? { id: b.id, anchor: runes(el.textContent.slice(0, s.anchor)), focus: runes(el.textContent.slice(0, s.focus)) } : null;
  }

  function turn(type) {
    if (active) edit(() => call('setType', active.id, active.type === type && type !== 'paragraph' ? 'paragraph' : type));
  }

  function indent(by) {
    if (active) edit(() => call('setDepth', active.id, Math.max(0, Math.min(8, (active.depth ?? 0) + by))));
  }

  function format(m) {
    if (!active || cur.anchor === cur.focus) return;
    const from = Math.min(cur.anchor, cur.focus), to = Math.max(cur.anchor, cur.focus);
    const has = marksIn(active, from, to).includes(m.name);
    let value = '';
    if (m.name === 'link' && !has) {
      value = prompt('Link', 'https://')?.trim();
      if (!value) return;
    }
    edit(() => call('mark', active.id, from, to, m.name, value, !has, m.expand));
  }

  $effect(() => {
    if (!native) return;
    bridge('toolbar', { show: true });
    const off = listen('tap', ({ action }) => {
      const [kind, arg] = action.split(':');
      if (kind === 'turn') turn(arg);
      else if (kind === 'indent') indent(Number(arg));
      else if (kind === 'mark') format(MARKS.find((m) => m.name === arg));
    });
    return () => {
      off();
      bridge('toolbar', { show: false });
    };
  });

  $effect(() => {
    if (native) bridge('setState', { type: active ? active.type || 'paragraph' : null, marks: [...on] });
  });

  function onMenuKey(e) {
    if (!menu) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      menu.index = (menu.index + (e.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      pick(options[menu.index]);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      menu = null;
    }
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
    if (menu) return;
    const el = e.currentTarget;
    const s = selection(el);
    const at = s?.focus ?? 0;
    const collapsed = s && s.anchor === s.focus;
    const text = el.textContent;
    const type = b.type || 'paragraph';
    const shortcut = (e.metaKey || e.ctrlKey) && !e.altKey && MARKS.find((m) => m.key === e.key.toLowerCase() && (m.key !== 'x' || e.shiftKey));
    if (shortcut) {
      e.preventDefault();
      track();
      format(shortcut);
    } else if (e.key === 'Tab') {
      e.preventDefault();
      track();
      indent(e.shiftKey ? -1 : 1);
    } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && type === 'todo') {
      e.preventDefault();
      toggle(b);
    } else if (e.key === 'Enter' && !e.shiftKey && (type !== 'code' || e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      if (LISTS.includes(type) && !text) return edit(() => call('setType', b.id, 'paragraph'));
      let next;
      await edit(() => (next = call('split', b.id, runes(text.slice(0, at)), LISTS.includes(type) ? type : 'paragraph')));
      focus(next, 0);
    } else if (e.key === 'Backspace' && collapsed && at === 0) {
      e.preventDefault();
      if (type !== 'paragraph') return edit(() => call('setType', b.id, 'paragraph'));
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

    const onSelection = () => {
      track();
      publish().catch(console.error);
    };
    const onBlur = () => setTimeout(onSelection);
    const onPointer = (e) => { if (!e.target.closest?.('.menu, .add')) menu = null; };

    editor().then((p) => {
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
      document.addEventListener('keydown', onMenuKey, true);
      addEventListener('resize', cursors);
    });

    return () => {
      es?.close();
      clearInterval(timer);
      if (pages) flush().catch(console.error);
      document.removeEventListener('selectionchange', onSelection);
      document.removeEventListener('focusout', onBlur);
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onMenuKey, true);
      removeEventListener('resize', cursors);
    };
  });
</script>

<div class="people">
  <People {live} people={crowd} />
</div>
<Heading editable icon="page" placeholder="Untitled" {@attach (el) => (heading = el)} {@attach sync(title)} oninput={retitle} onkeydown={titleKey} />
{#if !native}
<div class="toolbar">
  <Bar>
    {#each TYPES as t (t.type)}
      <Tool icon={t.icon} label={t.label} on={(active?.type || 'paragraph') === t.type && !!active} onclick={() => turn(t.type)} />
    {/each}
    <span class="sep"></span>
    <Tool icon="list-indent-decrease" label="Outdent" onclick={() => indent(-1)} />
    <Tool icon="list-indent-increase" label="Indent" onclick={() => indent(1)} />
    <span class="sep"></span>
    {#each MARKS as m (m.name)}
      <Tool icon={m.icon} label={m.label} on={on.includes(m.name)} onclick={() => format(m)} />
    {/each}
  </Bar>
</div>
{/if}
<div class="doc">
  <div class="blocks" bind:this={root}>
    {#each blocks as b, i (b.id)}
      <div
        class="block {b.type || 'paragraph'}"
        class:done={b.done}
        class:lonely={blocks.length === 1}
        data-id={b.id}
        role="group"
        style:--depth={b.depth || undefined}
        oncontextmenu={(e) => { e.preventDefault(); menu = { block: b.id, turn: true, index: 0 }; }}
      >
        <button class="add" tabindex="-1" aria-label="Add a block below" onmousedown={(e) => e.preventDefault()} onclick={() => (menu = { block: b.id, index: 0 })}><Icon name="plus" /></button>
        {#if b.depth}<span class="indent"></span>{/if}
        {#if b.type === 'todo'}
          <span class="mark">
            <button class="check" tabindex="-1" aria-label={b.done ? 'Mark as not done' : 'Mark as done'} onmousedown={(e) => e.preventDefault()} onclick={() => toggle(b)}>
              {#if b.done}<Icon name="check" />{/if}
            </button>
          </span>
        {:else if b.type === 'quote'}
          <span class="bar"></span>
        {:else if b.type === 'bullet'}
          <span class="mark">•</span>
        {:else if b.type === 'numbered'}
          <span class="mark">{numbers[b.id]}.</span>
        {/if}
        <div
          class="text"
          contenteditable="plaintext-only"
          role="textbox"
          tabindex="0"
          data-placeholder={PLACEHOLDERS[b.type] ?? 'Write something'}
          {@attach paint(b)}
          oninput={(e) => input(b, e.currentTarget)}
          onkeydown={(e) => key(e, b, i)}
        ></div>
        {#if menu?.block === b.id}
          <div class="popover">
            <Menu title={menu.turn ? 'Turn into' : 'Add below'} items={options} index={menu.index} onpick={pick} onhover={(n) => (menu.index = n)} />
          </div>
        {/if}
      </div>
    {/each}
    {#each marks as m, n (n)}
      <div class="{m.kind} peer-{peer(m.p)}" {@attach place(m)}>{#if m.kind === 'caret'}<b>{m.p.name}</b>{/if}</div>
    {/each}
  </div>
</div>

<style>
  .toolbar { position: relative; z-index: 1; display: flex; flex: none; min-width: 0; padding: var(--space-1); background: var(--panel); border-radius: var(--radius-pill); }
  .toolbar > :global(.bar) { flex: 1; min-width: 0; gap: var(--space-1); }
  .sep { flex: none; align-self: stretch; width: var(--stroke); background: var(--line); }
  .people { position: fixed; top: calc(var(--gutter) + var(--space-3) + (var(--size-logo) - var(--size-sm)) / 2); right: var(--gutter); z-index: 1; display: flex; }
  @media (max-width: 767px) {
    .people { display: none; }
  }

  [contenteditable] { outline: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  [contenteditable]:empty::before { content: attr(data-placeholder); color: var(--text-3); pointer-events: none; }

  .doc { margin: calc(-1 * var(--inset)); padding: var(--inset); }
  .blocks { position: relative; display: flex; flex-direction: column; gap: var(--space-1); }
  .block { position: relative; display: flex; align-items: stretch; gap: var(--space-2); }
  .text { flex: 1; min-width: 0; padding: var(--space-1); font-size: var(--text-lg); line-height: var(--leading-loose); }
  .paragraph:not(:focus-within):not(.lonely) .text:empty::before { content: none; }

  .add {
    position: absolute;
    top: var(--space-2);
    right: calc(100% + var(--space-1));
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
    opacity: 0;
    transition: opacity var(--dur) var(--ease), background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .block:hover .add { opacity: 1; }
  .add:hover { background: var(--hover); color: var(--text); }
  .indent { flex: none; width: calc(var(--depth) * var(--space-6)); }

  .mark {
    display: grid;
    place-items: center;
    flex: none;
    min-width: var(--size-avatar);
    height: calc(var(--text-lg) * var(--leading-loose) + 2 * var(--space-1));
    font-size: var(--text-lg);
    color: var(--text-2);
  }
  .bar { flex: none; width: var(--stroke-bold); border-radius: var(--radius-pill); background: var(--cream); }
  .check {
    display: grid;
    place-items: center;
    width: var(--text-xl);
    height: var(--text-xl);
    padding: 0;
    border: var(--stroke-bold) solid var(--line-strong);
    border-radius: var(--radius-sm);
    background: none;
    color: var(--cream-ink);
    cursor: pointer;
    transition: background var(--dur) var(--ease), border-color var(--dur) var(--ease);
  }
  .check:hover { border-color: var(--line-focus); }
  .check :global(svg.icon) { width: var(--text-md); height: var(--text-md); }
  .done .check { background: var(--cream); border-color: var(--cream); }
  .done .text { color: var(--text-3); text-decoration: line-through; text-decoration-color: var(--line-strong); }

  .heading1 .text { font: var(--weight-medium) var(--display-md)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-display); }
  .heading2 .text { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); }
  .heading3 .text { font-size: var(--text-xl); font-weight: var(--weight-bold); }
  .quote .text { color: var(--text-2); }
  .code .text {
    padding: var(--space-4);
    background: var(--panel);
    border-radius: var(--radius-md);
    font: var(--text-sm)/var(--leading) var(--mono);
    font-variant-ligatures: none;
  }

  .text :global(.bold) { font-weight: var(--weight-bold); }
  .text :global(.italic) { font-style: italic; }
  .text :global(.underline) { text-decoration: underline; }
  .text :global(.strike) { text-decoration: line-through; }
  .text :global(.underline.strike) { text-decoration: underline line-through; }
  .text :global(.code) { background: var(--raised); border-radius: var(--radius-sm); font: var(--text-md) var(--mono); }
  .text :global(.link) { text-decoration: underline; text-decoration-color: var(--line-strong); color: var(--text); }
  .popover { position: absolute; top: calc(100% + var(--space-1)); left: 0; z-index: 1; }

  .caret, .sel { position: absolute; left: var(--x); top: var(--y); height: var(--h); background: var(--peer); pointer-events: none; }
  .caret { width: var(--stroke-bold); transition: left var(--dur-fast), top var(--dur-fast); }
  .sel { width: var(--w); opacity: 0.22; }
  .caret b {
    position: absolute;
    bottom: 100%;
    left: 0;
    padding: var(--space-1);
    border-radius: var(--radius-sm);
    background: var(--peer);
    color: var(--on-peer);
    font: var(--weight-bold) var(--text-xs)/1 var(--font);
    white-space: nowrap;
  }

  @media (max-width: 767px) {
    .add { display: none; }
  }
</style>
