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
</script>

<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Avatar from './ui/Avatar.svelte';
  import Heading from './ui/Heading.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { id, name } = $props();
  let title = $state('');
  let online = $state([]);
  let root;

  const types = ['paragraph', 'heading1', 'heading2', 'heading3', 'bullet', 'todo', 'quote', 'code'];
  const site = 2 + Math.floor(Math.random() * 2 ** 40);
  const me = { name };
  const peer = (p) => (p.agent ? 'agent' : p.site % 8);
  const runes = (s) => [...s].length;
  const caret = (input) => runes(input.value.slice(0, input.selectionStart));

  onMount(() => {
    let pages, es, timer, lastPresence = '';
    const measure = document.createElement('canvas').getContext('2d');
    api(`/api/files/${id}`).then((f) => (title = f.name.replace(/\.page$/, '')));

    const call = (fn, ...args) => {
      const r = pages[fn](...args);
      if (r instanceof Error) throw r;
      return JSON.parse(r);
    };
    const bytes = (b64) => Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));

    function focusAt(block, offset) {
      const input = root.querySelector(`[data-id="${block}"] input`);
      if (!input) return;
      const at = [...input.value].slice(0, offset).join('').length;
      input.focus();
      input.setSelectionRange(at, at);
    }

    function render() {
      const seen = new Set();
      let prev = null;
      for (const b of call('blocks')) {
        seen.add(b.id);
        let el = root.querySelector(`[data-id="${b.id}"]`);
        if (!el) {
          el = document.createElement('div');
          el.dataset.id = b.id;
          el.innerHTML = `<span class="type"><select aria-label="Block type">${types.map((t) => `<option>${t}</option>`).join('')}</select><svg class="icon" viewBox="0 0 10 10" aria-hidden="true"><path d="M2 3.5 5 6.5 8 3.5"/></svg></span><span class="mark"></span><input spellcheck="false" placeholder="Type here">`;
          const input = el.querySelector('input');
          const select = el.querySelector('select');
          input.oninput = () => { edit(() => call('setText', b.id, input.value)); publish(); };
          select.onchange = () => edit(() => call('setType', b.id, select.value));
          input.onkeydown = (e) => key(e, b.id, input);
        }
        el.className = 'block ' + (b.type || 'paragraph');
        el.querySelector('select').value = types.includes(b.type) ? b.type : 'paragraph';
        const input = el.querySelector('input');
        if (input.value !== b.text) {
          if (document.activeElement === input) {
            const old = input.value, at = input.selectionStart;
            let p = 0;
            while (p < old.length && p < b.text.length && old[p] === b.text[p]) p++;
            const moved = at > p ? at + b.text.length - old.length : at;
            input.value = b.text;
            input.setSelectionRange(moved, moved);
          } else input.value = b.text;
        }
        const after = prev ? prev.nextSibling : root.firstChild;
        if (after !== el) root.insertBefore(el, after);
        prev = el;
      }
      for (const el of [...root.children]) if (!seen.has(el.dataset.id)) el.remove();
      cursors();
    }

    function edit(f) {
      try { f(); } catch (e) { console.error(e); }
      render();
    }

    function key(e, block, input) {
      const blocks = call('blocks'), i = blocks.findIndex((b) => b.id === block);
      if (e.key === 'Enter') {
        e.preventDefault();
        const type = ['bullet', 'todo'].includes(blocks[i].type) ? blocks[i].type : 'paragraph';
        let next;
        edit(() => { next = call('split', block, caret(input), type); });
        focusAt(next, 0);
      } else if (e.key === 'Backspace' && input.selectionStart === 0 && input.selectionEnd === 0 && i > 0) {
        e.preventDefault();
        const above = blocks[i - 1];
        edit(() => call('merge', block));
        focusAt(above.id, runes(above.text));
      } else if ((e.key === 'ArrowUp' && i > 0) || (e.key === 'ArrowDown' && i < blocks.length - 1)) {
        e.preventDefault();
        const to = blocks[e.key === 'ArrowUp' ? i - 1 : i + 1];
        focusAt(to.id, Math.min(caret(input), runes(to.text)));
      }
    }

    async function flush() {
      const ops = call('flush');
      if (ops) await fetch(`/api/pages/${id}/ops`, { method: 'POST', body: bytes(ops) });
    }

    async function publish(force) {
      const input = root.contains(document.activeElement) && document.activeElement.tagName === 'INPUT' ? document.activeElement : null;
      const block = input ? input.closest('.block').dataset.id : '';
      const from = input ? runes(input.value.slice(0, input.selectionStart)) : 0;
      const to = input ? runes(input.value.slice(0, input.selectionEnd)) : 0;
      const k = [block, from, to].join(':');
      if (k === lastPresence && !force) return;
      lastPresence = k;
      const back = input && input.selectionDirection === 'backward';
      const u = call('presence', block, back ? to : from, back ? from : to, JSON.stringify(me));
      await fetch(`/api/pages/${id}/presence`, { method: 'POST', body: bytes(u) });
    }

    function xOf(input, offset) {
      const cs = getComputedStyle(input);
      measure.font = cs.font;
      return input.offsetLeft + parseFloat(cs.paddingLeft) - input.scrollLeft + measure.measureText([...input.value].slice(0, offset).join('')).width;
    }

    function place(el, x, y, w, h) {
      for (const [k, v] of Object.entries({ x, y, w, h })) el.style.setProperty(`--${k}`, `${v}px`);
    }

    function cursors() {
      for (const el of root.querySelectorAll('.caret, .sel')) el.remove();
      const peers = call('peers');
      online = peers;
      for (const p of peers) {
        const block = p.headBlock && root.querySelector(`[data-id="${p.headBlock}"]`);
        if (!block) continue;
        const input = block.querySelector('input');
        const top = input.offsetTop + 4, h = input.offsetHeight - 8;
        const c = document.createElement('div');
        c.className = `caret peer-${peer(p)}`;
        place(c, xOf(input, p.head), top, 0, h);
        c.innerHTML = '<b></b>';
        c.firstChild.textContent = p.name;
        block.append(c);
        if (p.anchorBlock === p.headBlock && p.anchor !== p.head) {
          const a = xOf(input, Math.min(p.anchor, p.head)), z = xOf(input, Math.max(p.anchor, p.head));
          const s = document.createElement('div');
          s.className = `sel peer-${peer(p)}`;
          place(s, a, top, z - a, h);
          block.append(s);
        }
      }
    }

    const onSelection = () => publish().catch(console.error);
    const onBlur = () => setTimeout(onSelection);

    crdt().then((p) => {
      pages = p;
      es = new EventSource(`/api/pages/${id}/sync?site=${site}`);
      es.addEventListener('snapshot', (e) => {
        root.replaceChildren();
        call('load', e.data, site);
        if (call('blocks').length === 0) call('insert', '', 'paragraph');
        render();
        publish(true).catch(console.error);
      });
      es.addEventListener('presence', (e) => {
        try { call('applyPresence', e.data); } catch (err) { console.error(err); }
        cursors();
      });
      es.onmessage = (e) => {
        try { call('apply', e.data); } catch (err) { console.error(err); }
        render();
      };
      timer = setInterval(() => flush().catch(console.error), 150);
      document.addEventListener('selectionchange', onSelection);
      document.addEventListener('focusout', onBlur);
    });

    return () => {
      es?.close();
      clearInterval(timer);
      if (pages) flush().catch(console.error);
      document.removeEventListener('selectionchange', onSelection);
      document.removeEventListener('focusout', onBlur);
    };
  });

  async function rename(next) {
    const before = title;
    title = next;
    try {
      title = (await api(`/api/files/${id}`, { method: 'PATCH', body: { name: `${next}.page` } })).name.replace(/\.page$/, '');
    } catch (e) {
      title = before;
      alert(e.message);
    }
  }
</script>

<Heading plain column="editor" {title} onrename={title ? rename : undefined}>
  <div class="people">
    {#each [{ ...me, site, you: true }, ...online] as p (p.site)}
      <Avatar name={p.name} pulse={p.agent} title={p.you ? `${p.name} (you)` : p.agent ? `${p.name} editing` : p.name} />
    {/each}
  </div>
</Heading>
<Sheet plain>
  <div class="page" bind:this={root}></div>
</Sheet>

<style>
  .people { display: flex; align-items: center; gap: var(--space-1); }
  .page { display: flex; flex-direction: column; align-self: center; width: 100%; max-width: var(--editor-width); }
  .page :global(.block) { position: relative; display: flex; align-items: baseline; gap: var(--space-1); }
  .page :global(.type) {
    display: var(--wide-only);
    flex: none;
    order: 1;
    align-items: center;
    align-self: center;
    gap: var(--space-1);
    padding: var(--space-1-5);
    border-radius: var(--radius-full);
    background: var(--fill);
    color: var(--fg-muted);
    opacity: 0;
    transition: opacity var(--duration-fast), background var(--duration-fast);
  }
  .page :global(.type select) { padding: 0; background: none; color: inherit; font-size: var(--text-app); line-height: var(--leading-none); text-transform: capitalize; appearance: none; outline: 0; cursor: pointer; }
  .page :global(.type .icon) { width: var(--size-icon-sm); height: var(--size-icon-sm); pointer-events: none; }
  .page :global(.type:hover), .page :global(.type:focus-within) { background: var(--fill-strong); color: var(--fg); }
  .page :global(.block:hover .type), .page :global(.block:focus-within .type) { opacity: 1; }
  .page :global(input) { flex: 1; min-width: 0; padding: var(--space-1); font-size: var(--text-editor); line-height: var(--leading-normal); outline: 0; }
  .page :global(input::placeholder) { color: transparent; }
  .page :global(.block:only-child input::placeholder), .page :global(input:focus::placeholder) { color: var(--fg-muted); }
  .page :global(.heading1 input) { font: var(--weight-medium) var(--text-4xl) / var(--leading-normal) var(--font-serif); letter-spacing: var(--tracking-tight); }
  .page :global(.heading2 input) { font: var(--weight-medium) var(--text-2xl) / var(--leading-normal) var(--font-serif); letter-spacing: var(--tracking-snug); }
  .page :global(.heading3 input) { font-size: var(--text-app); font-weight: var(--weight-semibold); line-height: var(--leading-loose); }
  .page :global(.quote input) { color: var(--fg-soft); font-family: var(--font-serif); font-style: italic; }
  .page :global(.code input) { padding: var(--space-2); border-radius: var(--radius-xs); background: var(--fill); font-family: var(--font-mono); font-size: var(--text-app); }
  .page :global(.mark) { display: none; flex: none; width: var(--space-3); color: var(--fg-muted); text-align: center; }
  .page :global(.bullet .mark), .page :global(.todo .mark) { display: block; }
  .page :global(.quote .mark) { display: block; align-self: stretch; width: var(--underline-thick); border-radius: var(--radius-full); background: var(--fill-strong); }
  .page :global(.bullet .mark::before) { content: '•'; }
  .page :global(.todo .mark::before) { content: '☐'; }
  .page :global(.caret), .page :global(.sel) { position: absolute; left: var(--x); top: var(--y); height: var(--h); background: var(--peer); pointer-events: none; }
  .page :global(.caret) { width: var(--underline-thick); transition: left var(--duration-instant); }
  .page :global(.sel) { width: var(--w); opacity: var(--opacity-faint); }
  .page :global(.caret b) {
    position: absolute;
    bottom: 100%;
    left: 0;
    padding: var(--space-1);
    border-radius: var(--radius-xs) var(--radius-xs) var(--radius-xs) 0;
    background: var(--peer);
    color: var(--on-peer);
    font: var(--weight-semibold) var(--text-meta) / var(--leading-none) var(--font-sans);
    white-space: nowrap;
  }
</style>
