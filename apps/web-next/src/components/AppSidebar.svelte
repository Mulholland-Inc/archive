<script>
  import { PanelLeft, PinOff, Plus, Search, MessagesSquare, Library, Workflow, Plug, Users } from '@lucide/svelte';
  import Sidebar from './Sidebar.svelte';
  import SidebarItem from './SidebarItem.svelte';
  import SidebarSection from './SidebarSection.svelte';
  import VaultSwitcher from './VaultSwitcher.svelte';
  import AccountMenu from './AccountMenu.svelte';
  import IconButton from './IconButton.svelte';
  import ChatActions from './ChatActions.svelte';
  import Stack from './Stack.svelte';
  import { app, ws, closeDrawer, toggleSidebar, togglePin, pinnedFirst, switchVault, createVault } from '../lib/app.svelte.js';
  import { me } from '../lib/data.js';
  import { route, active, go } from '../lib/router.svelte.js';
  import NewVaultDialog from './NewVaultDialog.svelte';

  const running = $derived(ws().workflows.filter((w) => w.runs[0] && (w.runs[0].status === 'running' || w.runs[0].status === 'waiting')).length);
  const visible = $derived(pinnedFirst(ws().chats.filter((c) => !c.hidden)));


  let creating = $state(false);

  const collapsed = $derived(app.collapsed && !app.drawer);

  $effect(() => {
    route.path;
    app.drawer = false;
  });
</script>

<Sidebar collapsed={app.collapsed} drawer={app.drawer} onclose={closeDrawer}>
  {#snippet header()}
    <Stack direction={collapsed ? 'column' : 'row'} gap="1" align="center">
      <VaultSwitcher
        vaults={app.vaults}
        current={app.vault}
        onselect={switchVault}
        onsettings={() => go('/settings/vault')}
        oncreate={() => { app.drawer = false; creating = true; }}
      />
      <IconButton icon={PanelLeft} label={collapsed ? 'Open sidebar' : 'Close sidebar'} placement={collapsed ? 'right' : 'top'} shortcut="⌘ ." onclick={toggleSidebar} />
    </Stack>
  {/snippet}

  <SidebarSection>
    <SidebarItem icon={Plus} label="New chat" href="#/" accent shortcut="⌘ ⇧ O" />
    <SidebarItem icon={Search} label="Search" shortcut="⌘ K" onclick={() => { app.drawer = false; app.palette = true; }} />
  </SidebarSection>

  <SidebarSection>
    <SidebarItem icon={MessagesSquare} label="Chats" href="#/chats" active={active('/chats')} />
    <SidebarItem icon={Library} label="Library" href="#/library" active={active('/library')} />
    <SidebarItem icon={Workflow} label="Workflows" href="#/workflows" active={active('/workflows')} badge={running ? String(running) : undefined} />
    <SidebarItem icon={Plug} label="Connectors" href="#/connectors" active={active('/connectors')} />
    <SidebarItem icon={Users} label="Users" href="#/users" active={active('/users')} />
  </SidebarSection>

  {#if !collapsed && visible.length}
    <SidebarSection title="Recents">
        {#each visible as chat (chat.id)}
        <SidebarItem label={chat.title} href={`#/chat/${chat.id}`} active={route.path === `/chat/${chat.id}`}>
          {#snippet actions()}
            {#if chat.pinned}<IconButton icon={PinOff} label="Unpin" size="xs" tooltip={false} onclick={() => togglePin(chat.id)} />{/if}
            <ChatActions {chat} />
          {/snippet}
        </SidebarItem>
      {/each}
    </SidebarSection>
  {/if}

  {#snippet footer()}
    <AccountMenu user={{ ...me, name: app.profile.name, tone: app.profile.tone }} />
  {/snippet}
</Sidebar>

<NewVaultDialog bind:open={creating} oncreate={createVault} />
