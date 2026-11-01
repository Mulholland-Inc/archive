<script>
  import { Plus, Search, MessagesSquare, Library, Database, Workflow, Lock } from '@lucide/svelte';
  import Sidebar from './Sidebar.svelte';
  import SidebarItem from './SidebarItem.svelte';
  import SidebarSection from './SidebarSection.svelte';
  import VaultHeader from './VaultHeader.svelte';
  import VaultRail from './VaultRail.svelte';
  import AccountMenu from './AccountMenu.svelte';
  import ChatActions from './ChatActions.svelte';
  import PinButton from './PinButton.svelte';
  import { app, ws, openVaultSettings, closeDrawer, pinnedFirst, switchVault, createVault, renameChat } from '../lib/app.svelte.js';
  import { me } from '../lib/data.js';
  import { route, active } from '../lib/router.svelte.js';
  import NewVaultDialog from './NewVaultDialog.svelte';
  import SetupCard from './SetupCard.svelte';
  import { chatItems } from '../lib/menus.js';

  const running = $derived(ws().workflows.filter((w) => w.runs[0] && (w.runs[0].status === 'running' || w.runs[0].status === 'waiting')).length);
  const visible = $derived(pinnedFirst(ws().chats));


  let creating = $state(false);
  let renaming = $state(null);

  const collapsed = $derived(app.collapsed && !app.drawer);

  $effect(() => {
    route.path;
    app.drawer = false;
  });
</script>

<Sidebar collapsed={app.collapsed} drawer={app.drawer} onclose={closeDrawer}>
  {#snippet rail()}
    <VaultRail vaults={app.vaults} current={app.vault} onselect={switchVault} oncreate={() => { app.drawer = false; creating = true; }} />
  {/snippet}
  {#snippet header()}
    <VaultHeader vault={ws()} onclick={() => { app.drawer = false; openVaultSettings(); }} />
  {/snippet}

  <SetupCard />

  <SidebarSection>
    <SidebarItem icon={Plus} label="New chat" href="#/" accent shortcut="⌘ ⇧ O" />
    <SidebarItem icon={Search} label="Search" shortcut="⌘ K" onclick={() => { app.drawer = false; app.palette = true; }} />
  </SidebarSection>

  <SidebarSection>
    <SidebarItem icon={MessagesSquare} label="Chats" href="#/chats" active={active('/chats')} />
    <SidebarItem icon={Library} label="Library" href="#/library" active={active('/library')} />
    <SidebarItem icon={Database} label="Database" href="#/database" active={active('/database')} />
    <SidebarItem icon={Workflow} label="Workflows" href="#/workflows" active={active('/workflows')} badge={running ? String(running) : undefined} />
    <SidebarItem icon={Lock} label="Access" href="#/access" active={active('/access')} />
  </SidebarSection>

  {#if !collapsed && visible.length}
    <SidebarSection title="Recents">
        {#each visible as chat (chat.id)}
        <SidebarItem label={chat.title} href={`#/chat/${chat.id}`} active={route.path === `/chat/${chat.id}`} items={() => chatItems(chat, { open: false, rename: () => (renaming = chat.id) })} onrename={(title) => renameChat(chat.id, title)} bind:editing={() => renaming === chat.id, (on) => (renaming = on ? chat.id : null)}>
          {#snippet before()}
            {#if chat.pinned}<PinButton {chat} />{/if}
          {/snippet}
          {#snippet actions()}
            <ChatActions {chat} size="sm" inline />
          {/snippet}
        </SidebarItem>
      {/each}
    </SidebarSection>
  {/if}

  {#snippet footer()}
    <AccountMenu user={{ ...me, name: app.profile.name }} />
  {/snippet}
</Sidebar>

<NewVaultDialog bind:open={creating} oncreate={createVault} />
