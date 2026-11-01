<script>
  import { onMount, tick } from 'svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Stack from '../components/Stack.svelte';
  import Message from '../components/Message.svelte';
  import Composer from '../components/Composer.svelte';
  import ComposerDock from '../components/ComposerDock.svelte';
  import RunProgress from '../components/RunProgress.svelte';
  import ChatActions from '../components/ChatActions.svelte';
  import Button from '../components/Button.svelte';
  import EditableTitle from '../components/EditableTitle.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { MessageSquareOff } from '@lucide/svelte';
  import { app, ws, renameChat } from '../lib/app.svelte.js';
  import { streamReply } from '../lib/data.js';
  import { duration } from '../lib/motion.js';
  import { toast } from '../lib/toast.svelte.js';
  import { uid } from '../lib/ids.js';

  let { id } = $props();

  const chat = $derived(ws().chats.find((c) => c.id === id));

  function fallback(c) {
    return c
      ? [
          { id: 'f1', role: 'user', text: c.title },
          { id: 'f2', role: 'assistant', blocks: structuredClone(streamReply) }
        ]
      : [];
  }

  const messages = $derived(ws().conversations[id] ?? fallback(chat));

  const task = $derived(messages.findLast((m) => m.role === 'assistant' && m.task));

  let value = $state('');
  let expanded = $state(false);
  let streaming = $state(false);
  let scroller = $state();
  let element = $state();
  let cancelled = false;

  const wait = (ms) => new Promise((r) => setTimeout(r, ms));

  function nearBottom() {
    return scroller ? scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 160 : true;
  }

  async function toBottom(force = false) {
    await tick();
    if (scroller && (force || nearBottom())) scroller.scrollTop = scroller.scrollHeight;
  }

  async function type(target, key, text) {
    const words = text.split(/(\s+)/);
    for (const w of words) {
      if (cancelled) return;
      target[key] += w;
      if (w.trim()) {
        toBottom();
        await wait(duration('stream'));
      }
    }
  }

  async function send({ text, attachments }) {
    const conv = ws().conversations;
    if (!conv[id]) conv[id] = fallback(chat);
    const list = conv[id];
    chat.updated = 'Just now';
    list.push({ id: uid('m'), role: 'user', text, attachments });
    toBottom(true);
    const plan = [
      'Searching the archive for what you asked about',
      'Reading the 14 letters and photos that match',
      'Putting the results in order'
    ];
    list.push({ id: uid('m'), role: 'assistant', task: true, steps: plan.map((label, i) => ({ id: `s${i}`, label, status: 'pending', time: '' })), blocks: [] });
    const reply = list.at(-1);
    streaming = true;
    cancelled = false;
    const started = Date.now();
    for (const step of reply.steps) {
      if (cancelled) break;
      const begun = Date.now();
      step.status = 'running';
      toBottom();
      await wait(duration('slower') * 4);
      if (cancelled) break;
      step.status = 'done';
      step.time = `${Math.max(1, Math.round((Date.now() - begun) / 1000))}s`;
    }
    for (const step of reply.steps) if (step.status === 'running') step.status = 'failed';
    if (!cancelled) {
      reply.worked = `${Math.max(1, Math.round((Date.now() - started) / 1000))}s`;
      for (const block of streamReply) {
        if (cancelled) break;
        if (block.type === 'ul' || block.type === 'ol') {
          reply.blocks.push({ type: block.type, items: [] });
          const live = reply.blocks.at(-1);
          for (const item of block.items) {
            if (cancelled) break;
            live.items.push('');
            await type(live.items, live.items.length - 1, item);
          }
        } else {
          reply.blocks.push({ type: block.type, text: '' });
          await type(reply.blocks.at(-1), 'text', block.text);
        }
      }
    }
    if (!reply.blocks.length) reply.blocks.push({ type: 'p', text: '*Stopped.*' });
    streaming = false;
  }

  function stop() {
    cancelled = true;
  }

  onMount(() => {
    toBottom(true);
    if (!document.activeElement?.matches('input')) element?.focus();
    if (app.pending?.id === id) {
      const p = app.pending;
      app.pending = null;
      send(p);
    }
    return () => (cancelled = true);
  });
</script>

<TopBar bordered>
  {#snippet leading()}
    <MobileMenu />
    {#if chat}
      <EditableTitle value={chat.title} label="Rename chat" onsave={(title) => renameChat(chat.id, title)} />
    {/if}
  {/snippet}
  {#snippet trailing()}
    {#if chat}<ChatActions {chat} size="sm" inline />{/if}
  {/snippet}
</TopBar>

{#if chat}
  <Page width="chat" bind:element={scroller}>
    <Stack gap="8">
      {#each messages as message, i (message.id)}
        <Message {message} streaming={streaming && i === messages.length - 1} onedit={(text) => { value = text; element?.focus(); }} />
      {/each}
    </Stack>
  </Page>
  <ComposerDock>
    {#if task}<RunProgress steps={task.steps} running={streaming || task.steps.some((s) => s.status === 'running')} bind:open={expanded} label="Task steps" />{/if}
    <Composer bind:value bind:element bind:model={app.model} placeholder="Reply to Archive" {streaming} onsend={send} onstop={stop} />
  </ComposerDock>
{:else}
  <Page center>
    <EmptyState icon={MessageSquareOff} title="This chat no longer exists">
      <Button variant="primary" href="#/">New chat</Button>
    </EmptyState>
  </Page>
{/if}
