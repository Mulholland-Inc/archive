<script>
  import { Plus, MessagesSquare, Star, SearchX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import ChatActions from '../components/ChatActions.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { app } from '../lib/app.svelte.js';
  import { projects } from '../lib/data.js';

  let query = $state('');
  const results = $derived(app.chats.filter((c) => c.title.toLowerCase().includes(query.trim().toLowerCase())));
  const projectName = (pid) => projects.find((p) => p.id === pid)?.name;
</script>

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page>
  <PageHeader title="Chats">
    {#snippet actions()}<Button variant="primary" icon={Plus} href="#/">New chat</Button>{/snippet}
  </PageHeader>
  <Stack gap="4">
    <SearchField bind:value={query} placeholder="Search your chats" size="lg" />
    <Text size="sm" tone="tertiary">{results.length} chats with Archive</Text>
    {#if results.length}
      <List label="Chats">
        {#each results as chat (chat.id)}
          <ListItem href={`#/chat/${chat.id}`} title={chat.title}>
            {#snippet leading()}<Icon icon={chat.starred ? Star : MessagesSquare} size="md" />{/snippet}
            {#snippet meta()}
              <span>{chat.updated}</span>
              {#if chat.project}<span>· {projectName(chat.project)}</span>{/if}
            {/snippet}
            {#snippet trailing()}<ChatActions {chat} placement="bottom-end" size="sm" />{/snippet}
          </ListItem>
        {/each}
      </List>
    {:else}
      <EmptyState icon={SearchX} title="No chats match your search" size="sm" />
    {/if}
  </Stack>
</Page>
