<script>
  import { Plus, SearchX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import ChatActions from '../components/ChatActions.svelte';
  import PinButton from '../components/PinButton.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, pinnedFirst, chats } from '../lib/app.svelte.js';

  chats.use();
  import { go } from '../lib/router.svelte.js';
  import { chatItems } from '../lib/menus.js';

  let query = $state('');
  const columns = [
    { key: 'pin', label: '', width: 'icon', control: true, mobile: 'trailing' },
    { key: 'title', label: 'Chat' },
    { key: 'updated', label: 'Updated', muted: true, width: 'sm' },
    { key: 'actions', label: '', width: 'xs', align: 'end' }
  ];
  const results = $derived(pinnedFirst(ws().chats.filter((c) => c.title.toLowerCase().includes(query.trim().toLowerCase()))));
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page>
  <StickyHeader title="Chats" art="bg-chats">
    {#snippet search()}<SearchField bind:value={query} placeholder="Search your chats" />{/snippet}
    {#snippet actions()}<Button variant="primary" icon={Plus} href="/">New chat</Button>{/snippet}
  </StickyHeader>
  {#if results.length}
    <Table {columns} rows={results} label="Chats" cards onrowclick={(chat) => go(`/chat/${chat.id}`)} menu={(chat) => chatItems(chat)}>
      {#snippet cell(chat, col)}
        {#if col.key === 'pin'}
          {#if chat.pinned}<PinButton {chat} size="sm" />{/if}
        {:else if col.key === 'title'}
          <Text weight="medium" truncate>{chat.title}</Text>
        {:else if col.key === 'updated'}
          {chat.updated}
        {:else if col.key === 'actions'}
          <ChatActions {chat} placement="bottom-end" size="sm" />
        {/if}
      {/snippet}
    </Table>
  {:else}
    <EmptyState icon={SearchX} title="No chats match your search" size="sm" />
  {/if}
</Page>
