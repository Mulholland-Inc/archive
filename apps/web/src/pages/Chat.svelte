<script>
  import { onMount, tick } from 'svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Stack from '../components/Stack.svelte';
  import Message from '../components/Message.svelte';
  import Composer from '../components/Composer.svelte';
  import ComposerDock from '../components/ComposerDock.svelte';
  import ChatActions from '../components/ChatActions.svelte';
  import PinButton from '../components/PinButton.svelte';
  import Button from '../components/Button.svelte';
  import EditableTitle from '../components/EditableTitle.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { MessageSquareOff } from '@lucide/svelte';
  import { app, ws, renameChat } from '../lib/app.svelte.js';
  import { openChat, sendMessage, stopChat } from '../lib/chat.svelte.js';

  let { id } = $props();

  const chat = $derived(ws().chats.find((c) => c.id === id));
  const messages = $derived(ws().conversations[id] ?? []);
  const streaming = $derived(Boolean(chat?.streaming));
  const loading = $derived(Boolean(chat && !chat.draft && !chat.loaded && !messages.length && !chat.error));

  let value = $state('');
  let scroller = $state();
  let element = $state();

  function nearBottom() {
    return scroller ? scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 160 : true;
  }

  async function toBottom(force = false) {
    await tick();
    if (scroller && (force || nearBottom())) scroller.scrollTop = scroller.scrollHeight;
  }

  $effect(() => {
    const last = messages.at(-1);
    last?.text;
    last?.steps?.length;
    toBottom();
  });

  function send(message) {
    sendMessage(id, message);
    toBottom(true);
  }

  $effect(() => {
    if (chat && !chat.draft && !chat.loaded) openChat(id).then(() => toBottom(true));
  });

  onMount(() => {
    if (!document.activeElement?.matches('input')) element?.focus();
    if (app.pending?.id === id) {
      const p = app.pending;
      app.pending = null;
      send(p);
    }
  });
</script>

<TopBar bordered>
  {#snippet leading()}
    <MobileMenu />
    {#if chat}
      {#if chat.pinned}<PinButton {chat} size="sm" />{/if}
      <EditableTitle value={chat.title} label="Rename chat" onsave={(title) => renameChat(chat.id, title)} />
    {/if}
  {/snippet}
  {#snippet trailing()}
    {#if chat}<ChatActions {chat} size="md" inline />{/if}
  {/snippet}
</TopBar>

{#if chat}
  <Page width="chat" bind:element={scroller}>
    <Stack gap="8">
      {#if loading}
        <Skeleton lines={4} />
      {:else if chat.error}
        <EmptyState icon={MessageSquareOff} title="This chat didn’t load">{chat.error}</EmptyState>
      {/if}
      {#each messages as message, i (message.id)}
        <Message {message} streaming={streaming && i === messages.length - 1} />
      {/each}
    </Stack>
  </Page>
  <ComposerDock>
    <Composer bind:value bind:element bind:model={app.model} placeholder="Reply to Archive" {streaming} onsend={send} onstop={() => stopChat(chat.id)} />
  </ComposerDock>
{:else if ws().loading}
  <Page width="chat"><Skeleton lines={4} /></Page>
{:else}
  <Page center>
    <EmptyState icon={MessageSquareOff} title="This chat no longer exists">
      <Button variant="primary" href="/">New chat</Button>
    </EmptyState>
  </Page>
{/if}
