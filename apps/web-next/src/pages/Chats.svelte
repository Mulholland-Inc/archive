<script>
  import { Plus, SearchX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import ChatActions from '../components/ChatActions.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, pinnedFirst } from '../lib/app.svelte.js';

  let query = $state('');
  const results = $derived(pinnedFirst(ws().chats.filter((c) => c.title.toLowerCase().includes(query.trim().toLowerCase()))));
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page>
  <StickyHeader title="Chats">
    {#snippet search()}<SearchField bind:value={query} placeholder="Search your chats" />{/snippet}
    {#snippet actions()}<Button variant="primary" icon={Plus} href="#/">New chat</Button>{/snippet}
  </StickyHeader>
  {#if results.length}
    <List label="Chats">
      {#each results as chat (chat.id)}
        <ListItem href={`#/chat/${chat.id}`} title={chat.title}>
          {#snippet trailing()}
            <Stack direction="row" gap="2" align="center">
              <Text size="sm" tone="tertiary">{chat.updated}</Text>
              <ChatActions {chat} placement="bottom-end" size="sm" />
            </Stack>
          {/snippet}
        </ListItem>
      {/each}
    </List>
  {:else}
    <EmptyState icon={SearchX} title="No chats match your search" size="sm" />
  {/if}
</Page>
