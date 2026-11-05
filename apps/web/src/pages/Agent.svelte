<script>
  import { onDestroy, onMount, tick } from 'svelte';
  import { Settings, Eraser, Bot } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Stack from '../components/Stack.svelte';
  import Message from '../components/Message.svelte';
  import Composer from '../components/Composer.svelte';
  import ComposerDock from '../components/ComposerDock.svelte';
  import AgentAvatar from '../components/AgentAvatar.svelte';
  import IconButton from '../components/IconButton.svelte';
  import EditableTitle from '../components/EditableTitle.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { app, ws, atLeast, agentById, agents, library, closeFile, updateAgent, rememberAgent, markSeen } from '../lib/app.svelte.js';
  import { conversation, loadEarlier, sendMessage, stopAgent } from '../lib/agent.svelte.js';
  import { clearConversation } from '../lib/menus.js';

  let { id } = $props();

  const agent = $derived(agentById(id));
  const messages = $derived(ws().conversations[id] ?? []);
  const loading = $derived(agent && !agent.loaded && !messages.length && !agent.error);
  const empty = $derived(agent?.loaded && !messages.length && !agent.error);
  const hour = new Date().getHours();
  const part = hour < 5 ? 'Up late' : hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';

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
    last?.files?.length;
    toBottom();
  });

  function send(message) {
    sendMessage(id, message);
    toBottom(true);
  }

  async function onscroll() {
    if (scroller.scrollTop > 160) return;
    const reading = loadEarlier(id);
    if (!reading) return;
    const below = scroller.scrollHeight - scroller.scrollTop;
    await reading;
    await tick();
    scroller.scrollTop = scroller.scrollHeight - below;
  }

  agents.use();
  library.use();
  conversation.use(() => agent && id);
  $effect(() => {
    if (agent) rememberAgent(id);
    if (agent && agent.at !== agent.seen) markSeen(agent);
  });
  let landed = false;
  $effect(() => {
    if (agent?.loaded && !landed) {
      landed = true;
      toBottom(true);
    }
  });

  $effect(() => {
    if (element && !document.activeElement?.matches('input, textarea')) element.focus();
  });

  onDestroy(closeFile);

  onMount(() => {
    if (id && app.pending?.id === id) {
      const p = app.pending;
      app.pending = null;
      send(p);
    }
  });
</script>

<TopBar bordered>
  {#snippet leading()}
    <MobileMenu />
    {#if agent && atLeast('manager')}<EditableTitle value={agent.name} label="Rename agent" onsave={(name) => updateAgent(agent.id, { name })} />{:else if agent}<span class="name">{agent.name}</span>{/if}
  {/snippet}
  {#snippet trailing()}
    {#if agent}
      {#if atLeast('manager')}<IconButton icon={Settings} label="Settings" size="md" onclick={() => (app.agentDialog = agent.id)} />{/if}
      {#if messages.length}<IconButton icon={Eraser} label="Clear" size="md" onclick={() => clearConversation(agent)} />{/if}
    {/if}
  {/snippet}
</TopBar>

{#if empty}
  <Page width="home" center>
    <Stack gap="8" align="stretch">
      <div class="greeting">
        <AgentAvatar avatar={agent.avatar} tint={agent.tint} seed={agent.id} size="xxl" active />
        <h1>{part}, {app.user.first}</h1>
      </div>
      <Composer bind:value bind:element placeholder={`Message ${agent.name}`} draft={id} size="lg" onsend={send} />
    </Stack>
  </Page>
{:else if agent}
  <Page width="chat" bind:element={scroller} {onscroll}>
    <Stack gap="8">
      {#if loading}
        <Skeleton lines={4} />
      {:else if agent.error}
        <EmptyState icon={Bot} title="This agent didn’t load">{agent.error}</EmptyState>
      {/if}
      {#each messages as message, i (message.id)}
        <Message {message} streaming={agent.streaming && i === messages.length - 1} />
      {/each}
    </Stack>
  </Page>
  {#if !loading}
    <ComposerDock>
      <Composer bind:value bind:element placeholder={`Message ${agent.name}`} draft={id} streaming={agent.streaming} onsend={send} onstop={() => stopAgent(id)} />
    </ComposerDock>
  {/if}
{:else if ws().loading}
  <Page width="chat"><Skeleton lines={4} /></Page>
{:else}
  <Page center><EmptyState icon={Bot} title="This agent no longer exists" /></Page>
{/if}

<style>
  .name {
    overflow: hidden;
    padding-inline: var(--space-3);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .greeting {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    text-align: center;
  }

  h1 {
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
    color: var(--color-text);
  }

  @media (max-width: 40rem) {
    h1 {
      font-size: var(--text-2xl);
    }
  }
</style>
