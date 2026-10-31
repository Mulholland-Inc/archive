<script module>
  let seq = 0;
  const key = () => ++seq;

  function outcome(b) {
    const raw = typeof b.content === 'string' ? b.content : (b.content ?? []).map((c) => c.text ?? '').join('');
    try {
      const o = JSON.parse(raw);
      return { result: o.error ? undefined : o.result, logs: o.logs ?? [], error: o.error ?? '', state: 'done' };
    } catch {
      return { result: undefined, logs: [], error: b.is_error ? raw : '', state: 'done' };
    }
  }

  export function build(messages) {
    const out = [];
    const runs = {};
    let turn = null;
    for (const m of messages) {
      const content = typeof m.content === 'string' ? [{ type: 'text', text: m.content }] : m.content;
      if (m.role === 'user') {
        for (const b of content) if (b.type === 'tool_result' && runs[b.tool_use_id]) Object.assign(runs[b.tool_use_id], outcome(b));
        const text = content.filter((b) => b.type === 'text').map((b) => b.text).join('\n');
        if (text) {
          out.push({ key: key(), role: 'user', text });
          out.push((turn = { key: key(), role: 'assistant', parts: [], status: 'done', error: '', pending: false }));
        }
        continue;
      }
      if (!turn) out.push((turn = { key: key(), role: 'assistant', parts: [], status: 'done', error: '', pending: false }));
      for (const b of content) {
        if (b.type === 'text' && b.text.trim()) turn.parts.push({ key: key(), kind: 'text', text: b.text });
        if (b.type === 'tool_use') turn.parts.push((runs[b.id] = { key: key(), kind: 'run', id: b.id, code: b.input?.code ?? '', state: 'done', logs: [], error: '', result: undefined }));
      }
    }
    if (turn && !turn.parts.length) turn.status = 'unanswered';
    return out;
  }

  export function greeting(name, now = new Date()) {
    const h = now.getHours();
    const part = h < 5 ? 'Good night' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
    const first = (name || '').trim().split(/[\s@]+/)[0];
    return first ? `${part}, ${first[0].toUpperCase()}${first.slice(1)}` : part;
  }
</script>

<script>
  import { untrack, tick } from 'svelte';
  import { api, chat, go, Unauthorized } from './api.js';
  import Button from '#kit/Button.svelte';
  import Heading from './ui/Heading.svelte';
  import Composer from './ui/Composer.svelte';
  import Stream from './ui/Stream.svelte';
  import RunCall from './ui/RunCall.svelte';
  import Note from './ui/Note.svelte';
  import Setup from './ui/Setup.svelte';
  import { keepsake } from './ui/keepsake.js';
  import Capture from './ui/Capture.svelte';
  import Dictate from './ui/Dictate.svelte';

  let { id = '', name = '', email = '', family = '', short = '', tint = '', kind = 'family', onfiles, onsaved } = $props();

  let current = $state('');
  let title = $state('');
  let turns = $state([]);
  let busy = $state(false);
  let loading = $state(false);
  let failed = $state('');
  let draft = $state('');
  let field = $state();
  let log = $state();
  let pane = $state();
  let ctrl = null;
  let pinned = true;

  const empty = $derived(!current && !turns.length);
  let picture = $state(null);
  $effect(() => {
    if (kind === 'family') keepsake().then((p) => (picture = p)).catch(() => {});
  });
  const framed = $derived(empty && picture);
  const hello = $derived(family || greeting(name));

  $effect(() => {
    const want = id;
    untrack(() => want !== current && open(want));
  });

  $effect(() => () => ctrl?.abort());

  $effect(() => {
    if (field && untrack(() => empty)) field.focus({ preventScroll: true });
  });

  const scroller = () => (getComputedStyle(pane).overflowY === 'visible' ? document.scrollingElement : pane);

  $effect(() => {
    const scroll = () => {
      const el = scroller();
      pinned = el.scrollHeight - el.scrollTop - el.clientHeight < 96;
    };
    const follow = new ResizeObserver(() => pinned && busy && bottom());
    addEventListener('scroll', scroll, { passive: true });
    pane.addEventListener('scroll', scroll, { passive: true });
    follow.observe(log);
    return () => {
      removeEventListener('scroll', scroll);
      pane.removeEventListener('scroll', scroll);
      follow.disconnect();
    };
  });

  function bottom() {
    const el = scroller();
    el.scrollTo({ top: el.scrollHeight, behavior: 'instant' });
  }

  async function open(want) {
    ctrl?.abort();
    ctrl = null;
    busy = false;
    current = want;
    title = '';
    turns = [];
    failed = '';
    if (!want) {
      await tick();
      field?.focus();
      return;
    }
    loading = true;
    try {
      const c = await api(`/api/chats/${want}`);
      if (current !== want) return;
      title = c.title;
      turns = build(c.messages ?? []);
      await tick();
      bottom();
      field?.focus({ preventScroll: true });
    } catch (e) {
      if (e instanceof Unauthorized) location.assign('/');
      if (current === want) failed = e.message;
    }
    loading = false;
  }

  async function send(text) {
    text = text.trim();
    if (!text || busy) return;
    draft = '';
    if (turns.at(-1)?.status === 'unanswered') turns.pop();
    turns.push({ key: key(), role: 'user', text, fresh: true });
    await stream(current ? { chat: current, text } : { text });
  }

  async function retry() {
    const last = turns.at(-1);
    if (busy || last?.role !== 'assistant') return;
    if (!current) {
      turns.pop();
      const ask = turns.pop();
      return send(ask.text);
    }
    if (last.parts.length) {
      last.status = 'done';
      last.error = '';
    } else turns.pop();
    await stream({ chat: current });
  }

  async function stream(body) {
    turns.push({ key: key(), role: 'assistant', parts: [], status: 'live', error: '', pending: true, fresh: true });
    const turn = turns.at(-1);
    const mine = (ctrl = new AbortController());
    const calls = {};
    let text = null;
    busy = true;
    pinned = true;
    await tick();
    bottom();
    try {
      for await (const ev of chat(body, mine.signal)) {
        if (ev.type === 'chat') {
          if (!current) {
            current = ev.chat.id;
            title = ev.chat.title;
            history.replaceState(null, '', `/chat/${current}`);
            dispatchEvent(new PopStateEvent('popstate'));
          }
          onsaved?.();
        } else if (ev.type === 'thinking') {
          turn.pending = true;
          text = null;
        } else if (ev.type === 'text') {
          turn.pending = false;
          if (!text) {
            turn.parts.push({ key: key(), kind: 'text', text: '' });
            text = turn.parts.at(-1);
          }
          text.text += ev.text;
        } else if (ev.type === 'tool') {
          turn.pending = false;
          text = null;
          turn.parts.push({ key: key(), kind: 'run', id: ev.id, code: '', state: 'writing', logs: [], error: '', result: undefined });
          calls[ev.id] = turn.parts.at(-1);
        } else if (ev.type === 'code' && calls[ev.id]) {
          calls[ev.id].code = ev.code;
        } else if (ev.type === 'run' && calls[ev.id]) {
          Object.assign(calls[ev.id], { code: ev.code, state: 'running' });
        } else if (ev.type === 'result' && calls[ev.id]) {
          Object.assign(calls[ev.id], { state: 'done', result: ev.result, logs: ev.logs ?? [], error: ev.error ?? '' });
          turn.pending = true;
        } else if (ev.type === 'done') {
          turn.status = 'done';
        } else if (ev.type === 'error') {
          turn.status = 'error';
          turn.error = ev.error;
        }
      }
      if (turn.status === 'live') Object.assign(turn, { status: 'error', error: 'The answer stopped early' });
    } catch (e) {
      if (e instanceof Unauthorized) location.assign('/');
      if (e.name === 'AbortError') turn.status = 'stopped';
      else Object.assign(turn, { status: 'error', error: e.message });
    }
    turn.pending = false;
    for (const p of turn.parts) if (p.kind === 'run' && p.state !== 'done') Object.assign(p, { state: 'done', error: p.error || 'Stopped' });
    if (ctrl === mine) {
      ctrl = null;
      busy = false;
    }
    onsaved?.();
  }

  function stop() {
    ctrl?.abort();
  }

  async function rename(next) {
    title = next;
    try {
      title = (await api(`/api/chats/${current}`, { method: 'PATCH', body: { title: next } })).title;
      onsaved?.();
    } catch (e) {
      failed = e.message;
    }
  }

  async function remove() {
    if (!confirm('Delete this chat?')) return;
    await api(`/api/chats/${current}`, { method: 'DELETE' });
    onsaved?.();
    go('/');
  }

  function segments(parts) {
    const out = [];
    for (const p of parts) {
      if (p.kind === 'run' && out.at(-1)?.kind === 'runs') out.at(-1).calls.push(p);
      else if (p.kind === 'run') out.push({ key: p.key, kind: 'runs', calls: [p] });
      else out.push(p);
    }
    return out;
  }
</script>

<Heading plain column="chat" title="" />
{#if current}<span class="remove"><Button icon="trash" title="Delete chat" aria-label="Delete chat" onclick={remove} /></span>{/if}
<div class="chat" class:empty style={tint ? `--primary: ${tint}; --primary-hover: ${tint}; --tint: ${tint}` : undefined} bind:this={pane}>
  {#if empty}
    <h2 class="hello">{hello}</h2>
  {/if}
  <div class="log" class:idle={empty && !failed} bind:this={log} aria-live="polite">
    {#if failed}<Note error>{failed}</Note>{/if}
    {#each turns as turn, i (turn.key)}
      {#if turn.role === 'user'}
        <div class="user" class:fresh={turn.fresh}>{turn.text}</div>
      {:else}
        <div class="turn" class:fresh={turn.fresh}>
          {#each segments(turn.parts) as g (g.key)}
            {#if g.kind === 'text'}
              <Stream text={g.text} live={turn.status === 'live'} />
            {:else}
              <RunCall calls={g.calls} />
            {/if}
          {/each}
          {#if turn.status === 'live' && turn.pending}
            <div class="thinking" role="status" aria-label="Thinking"><span></span><span></span><span></span></div>
          {/if}
          {#if turn.status === 'error'}
            <div class="problem">
              <Note error>{turn.error}</Note>
              {#if i === turns.length - 1}<Button ghost small onclick={retry}>Retry</Button>{/if}
            </div>
          {:else if turn.status === 'unanswered' && i === turns.length - 1}
            <div class="problem"><Button ghost small onclick={retry}>Answer</Button></div>
          {:else if turn.status === 'stopped'}
            <Note>Stopped</Note>
          {/if}
        </div>
      {/if}
    {/each}
    {#if loading}<div class="thinking" role="status" aria-label="Loading"><span></span><span></span><span></span></div>{/if}
  </div>
  <div class="stage" class:framed>
    {#if framed}<img src={picture.url} alt={picture.title} />{/if}
    <div class="dock">
    <Composer bind:value={draft} bind:field {busy} ontalk={() => dispatchEvent(new CustomEvent('capture-memory'))} onsend={send} onstop={stop} placeholder={short ? `Message ${short}` : 'Ask anything'} aria-label="Message">
      {#snippet lead()}<Capture {onfiles} {kind} />{/snippet}
      {#snippet tail()}<Dictate bind:value={draft} />{/snippet}
    </Composer>
    </div>
  </div>
  {#if framed}<p class="caption"><span>From your archive: {picture.title}</span><a href="/library">All photos</a></p>{/if}
  {#if empty}<Setup {email} />{/if}
</div>

<style>
  .chat { --text-table: var(--text-chat); display: flex; flex: 1; flex-direction: column; align-items: center; gap: var(--space-5); min-height: 0; padding: var(--gutter); overflow-y: var(--sheet-overflow); overscroll-behavior-y: none; font-size: var(--text-chat); }
  .remove { position: absolute; top: var(--space-2); right: var(--space-3); z-index: var(--z-raised); }
  .log { display: flex; flex: 1; flex-direction: column; gap: var(--space-3); padding-top: var(--space-7, 40px); width: 100%; max-width: var(--chat-width); min-width: 0; }
  .empty { justify-content: center; gap: var(--space-6); }
  .empty .log { flex: none; }
  .empty:has(.framed) { gap: var(--space-3); }
  .stage { display: flex; flex: none; justify-content: center; width: 100%; }
  .framed { position: relative; max-width: var(--composer-width); aspect-ratio: 16 / 9; max-height: 52svh; border-radius: var(--radius-panel); }
  .framed img { position: absolute; inset: 0; width: 100%; height: 100%; border-radius: inherit; object-fit: cover; object-position: center 30%; }
  .framed .dock { position: absolute; right: var(--space-4); bottom: var(--space-4); left: var(--space-4); width: auto; }
  .caption { display: flex; justify-content: space-between; gap: var(--space-3); width: 100%; max-width: var(--composer-width); margin-top: calc(-1 * var(--space-1)); padding: 0 var(--space-1); color: var(--fg-muted); font-size: var(--text-meta); }
  .caption a { flex: none; }
  .caption a:hover { color: var(--fg); }
  .log :global(.wrap) { margin-inline: 0; }
  .idle { display: none; }
  .hello {
    font: var(--weight-medium) var(--greeting-size) / var(--leading-tight) var(--font-serif);
    letter-spacing: var(--tracking-tight);
    text-align: center;
    text-wrap: balance;
    animation: appear var(--duration-slow) var(--ease-out) both;
  }
  .user {
    align-self: flex-end;
    max-width: var(--bubble-width);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-panel);
    background: var(--tint, var(--fill));
    color: #fff;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .turn { display: flex; flex-direction: column; gap: var(--space-3); align-self: flex-start; max-width: 100%; min-width: 0; padding: var(--space-3); border-radius: var(--radius-panel); background: var(--surface); }
  .fresh { animation: rise var(--duration-normal) var(--ease-out) both; }
  .problem { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); }
  .thinking { display: flex; gap: var(--space-1); padding: var(--space-2); align-self: flex-start; }
  .thinking span { width: var(--size-dot); height: var(--size-dot); border-radius: var(--radius-full); background: var(--fg-muted); animation: dot var(--duration-loop) var(--ease-in-out) infinite both; }
  .thinking span:nth-child(2) { animation-delay: calc(var(--duration-loop) / 6); }
  .thinking span:nth-child(3) { animation-delay: calc(var(--duration-loop) / 3); }
  .dock {
    position: sticky;
    bottom: calc(var(--dock-gap) - var(--dock-inset) + var(--dock-lift) + env(safe-area-inset-bottom));
    z-index: var(--z-raised);
    width: 100%;
    max-width: var(--composer-width);
    margin-bottom: calc(var(--dock-gap) - var(--gutter));
  }
  @keyframes appear { from { opacity: 0; } }
  @keyframes rise { from { opacity: 0; transform: translateY(var(--rise)); } }
  @keyframes dot { 0%, 80%, 100% { opacity: var(--opacity-faint); } 40% { opacity: 1; } }
  @media (prefers-reduced-motion: reduce) {
    .fresh, .hello { animation: none; }
    .thinking span { animation-duration: calc(var(--duration-loop) * 2); }
  }
</style>
