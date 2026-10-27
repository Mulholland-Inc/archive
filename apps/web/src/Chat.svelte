<script>
  import { chat, Unauthorized } from './api.js';
  import Composer from './ui/Composer.svelte';
  import Markdown from './ui/Markdown.svelte';
  import Note from './ui/Note.svelte';
  import Activity from './ui/Activity.svelte';

  let { messages = $bindable([]) } = $props();
  let draft = $state('');
  let live = $state('');
  let calling = $state(false);
  let busy = $state(false);
  let error = $state('');
  let ctrl;
  let scroller = $state();

  const items = $derived.by(() => {
    const results = {};
    for (const m of messages) if (m.role === 'user' && Array.isArray(m.content)) for (const b of m.content) if (b.type === 'tool_result') results[b.tool_use_id] = b;
    const out = [];
    let turn = null;
    for (const m of messages) {
      const blocks = typeof m.content === 'string' ? [{ type: 'text', text: m.content }] : m.content;
      if (m.role === 'user') {
        const text = blocks.filter((b) => b.type === 'text').map((b) => b.text).join('\n');
        if (!text) continue;
        out.push({ kind: 'user', text });
        out.push((turn = { kind: 'turn', calls: [], texts: [], phase: 'done' }));
        continue;
      }
      for (const b of blocks) {
        if (b.type === 'text' && b.text.trim()) turn?.texts.push(b.text);
        else if (b.type === 'tool_use') turn?.calls.push({ code: b.input?.code ?? '', result: results[b.id] });
      }
    }
    if (turn && live) turn.texts.push(live);
    if (turn && busy) turn.phase = calling || turn.calls.some((c) => !c.result) ? 'running' : live ? 'done' : 'thinking';
    return out;
  });

  $effect(() => {
    items.length;
    live;
    busy;
    if (scroller && scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < scroller.clientHeight / 2) scroller.scrollTop = scroller.scrollHeight;
  });

  $effect(() => () => ctrl?.abort());

  async function send() {
    const start = messages.length;
    const text = draft.trim();
    draft = '';
    error = '';
    busy = true;
    ctrl = new AbortController();
    messages = [...messages, { role: 'user', content: text }];
    requestAnimationFrame(() => scroller && (scroller.scrollTop = scroller.scrollHeight));
    try {
      for await (const ev of chat(messages, ctrl.signal)) {
        if (ev.text) live += ev.text;
        if (ev.tool) calling = true;
        if (ev.message) {
          calling = false;
          messages = [...messages, ev.message];
          live = '';
        }
        if (ev.error) throw new Error(ev.error);
      }
      const last = messages.at(-1);
      if (last.role !== 'assistant' || last.content.some((b) => b.type === 'tool_use')) throw new Error('The agent stopped before it answered.');
    } catch (e) {
      if (e instanceof Unauthorized) location.assign('/');
      if (e.name !== 'AbortError') error = e.message;
      messages = messages.slice(0, start);
      draft = text;
    }
    live = '';
    calling = false;
    busy = false;
  }
</script>

<div class="chat" bind:this={scroller}>
  <div class="log">
    {#each items as it, i (i)}
      {#if it.kind === 'user'}
        <div class="user">{it.text}</div>
      {:else}
        {#if it.calls.length || it.phase !== 'done'}<Activity calls={it.calls} phase={it.phase} />{/if}
        {#each it.texts as text, j (j)}<Markdown {text} />{/each}
      {/if}
    {/each}
    {#if error}<Note error>{error}</Note>{/if}
  </div>
  <div class="dock">
    <Composer bind:value={draft} {busy} onsend={send} onstop={() => ctrl?.abort()} placeholder="Message" aria-label="Message" />
  </div>
</div>

<style>
  .chat { display: flex; flex-direction: column; align-items: center; gap: var(--space-5); }
  .log { display: flex; flex: 1; flex-direction: column; gap: var(--space-4); width: 100%; max-width: var(--chat); }
  .user {
    align-self: flex-end;
    max-width: var(--bubble);
    padding: var(--space-3);
    background: var(--raised);
    border-radius: var(--radius-lg);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .dock { position: sticky; bottom: 0; width: 100%; max-width: var(--chat); background: var(--shell); }
</style>
