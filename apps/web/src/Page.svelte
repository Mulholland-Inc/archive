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

  let { id, name } = $props();
  let title = $state('');
  let live = $state(false);
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
          el.innerHTML = `<select>${types.map((t) => `<option>${t}</option>`).join('')}</select><span class="mark"></span><input spellcheck="false" placeholder="Type here">`;
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
        live = true;
      });
      es.addEventListener('presence', (e) => {
        try { call('applyPresence', e.data); } catch (err) { console.error(err); }
        cursors();
      });
      es.onmessage = (e) => {
        try { call('apply', e.data); } catch (err) { console.error(err); }
        render();
      };
      es.onerror = () => (live = false);
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
</script>

<header>
  <a class="crumb" href="/pages">Pages <span>/</span> {title}</a>
  <div class="people">
    <span class="dot" class:live></span>
    {#each [{ ...me, site, you: true }, ...online] as p (p.site)}
      <Avatar name={p.name} peer={peer(p)} pulse={p.agent} title={p.you ? `${p.name} (you)` : p.agent ? `${p.name} editing` : p.name} />
    {/each}
  </div>
</header>
<div class="page" bind:this={root}></div>

<style>
  header { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); max-width: var(--measure); margin: calc(-1 * var(--space-7)) auto var(--space-6); }
  .crumb { font-size: var(--text-sm); color: var(--text-3); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .crumb span { margin: 0 var(--space-1); }
  .people { display: flex; align-items: center; gap: var(--space-1); }
  .dot { width: var(--size-xs); height: var(--size-xs); border-radius: 50%; background: var(--line-strong); margin-right: var(--space-2); }
  .dot.live { background: var(--cream); }
  .page { max-width: var(--measure); margin: 0 auto; padding-bottom: var(--space-9); }
  .page :global(.block) { position: relative; display: flex; align-items: baseline; gap: var(--space-2); }
  .page :global(select) {
    width: var(--size-xl);
    margin-left: calc(-1 * var(--size-xl) - var(--space-2));
    padding: 0;
    border: 0;
    background: none;
    font-size: var(--text-xs);
    color: var(--text-3);
    opacity: 0;
    transition: opacity var(--dur) var(--ease);
  }
  .page :global(.block:hover select), .page :global(.block:focus-within select) { opacity: 1; }
  .page :global(input) { flex: 1; min-width: 0; padding: var(--space-1) 0; border: 0; background: none; font-size: var(--text-lg); line-height: 1.6; outline: 0; }
  .page :global(input::placeholder) { color: var(--line-strong); }
  .page :global(.heading1 input) { font: var(--weight-medium) var(--display-lg)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-display); padding-bottom: var(--space-3); }
  .page :global(.heading2 input) { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); padding-top: var(--space-4); }
  .page :global(.heading3 input) { font-size: var(--text-xl); font-weight: var(--weight-bold); padding-top: var(--space-2); }
  .page :global(.quote input) { border-left: var(--stroke-bold) solid var(--line-strong); padding-left: var(--space-3); color: var(--text-2); }
  .page :global(.code input) { font-family: var(--mono); font-size: var(--text-sm); background: var(--raised); padding: var(--space-2) var(--space-3); border-radius: var(--radius-sm); }
  .page :global(.mark) { flex: none; width: var(--space-3); color: var(--text-3); }
  .page :global(.bullet .mark::before) { content: '•'; }
  .page :global(.todo .mark::before) { content: '☐'; }
  .page :global(.caret), .page :global(.sel) { position: absolute; left: var(--x); top: var(--y); height: var(--h); background: var(--peer); pointer-events: none; }
  .page :global(.caret) { width: var(--stroke-bold); transition: left var(--dur-fast); }
  .page :global(.sel) { width: var(--w); opacity: 0.22; }
  .page :global(.caret b) {
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
    header { margin-top: calc(-1 * var(--space-4)); }
    .page :global(select) { display: none; }
  }
</style>
