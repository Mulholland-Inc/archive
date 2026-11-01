<script>
  import { FolderOpen, Search, CalendarRange, AudioLines, Workflow } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Stack from '../components/Stack.svelte';
  import Greeting from '../components/Greeting.svelte';
  import Composer from '../components/Composer.svelte';
  import Chip from '../components/Chip.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { suggestions } from '../lib/mock/data.js';
  import { app, ws, newChat } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  const icons = { files: FolderOpen, search: Search, timeline: CalendarRange, audio: AudioLines, workflow: Workflow };

  let value = $state('');
  let element = $state();

  $effect(() => {
    element?.focus();
  });

  function send({ text, attachments }) {
    const title = text ? (text.length > 48 ? text.slice(0, 46).trimEnd() + '…' : text) : attachments[0]?.name ?? 'New chat';
    const id = newChat(title, { attachments, text });
    go(`/chat/${id}`);
  }
</script>

<TopBar>
  {#snippet leading()}<MobileMenu />{/snippet}
</TopBar>
<Page width="home" center>
  <Stack gap="8" align="stretch">
    <Stack gap="3" align="center">
      <Greeting name={app.profile.first} look={ws().look} seed={ws().id} />
    </Stack>
    <Stack gap="4" align="center">
      <Composer bind:value bind:element bind:model={app.model} size="lg" onsend={send} />
      <Stack direction="row" gap="2" wrap justify="center">
        {#each suggestions as s (s.id)}
          <Chip icon={icons[s.icon]} onclick={() => { value = s.prompt; element?.focus(); }}>{s.label}</Chip>
        {/each}
      </Stack>
    </Stack>
  </Stack>
</Page>
