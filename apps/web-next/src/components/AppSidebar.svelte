<script>
  import { PanelLeft, Plus, Search, MessagesSquare, Library, FolderClosed, Workflow, Plug, Users } from '@lucide/svelte';
  import Sidebar from './Sidebar.svelte';
  import SidebarItem from './SidebarItem.svelte';
  import SidebarSection from './SidebarSection.svelte';
  import WorkspaceSwitcher from './WorkspaceSwitcher.svelte';
  import AccountMenu from './AccountMenu.svelte';
  import IconButton from './IconButton.svelte';
  import ChatActions from './ChatActions.svelte';
  import Stack from './Stack.svelte';
  import { app, ws, closeDrawer, toggleSidebar, switchWorkspace, createWorkspace } from '../lib/app.svelte.js';
  import { me, chatGroups } from '../lib/data.js';
  import { route, active, go } from '../lib/router.svelte.js';
  import NewWorkspaceDialog from './NewWorkspaceDialog.svelte';

  const running = $derived(ws().workflows.filter((w) => w.runs[0] && (w.runs[0].status === 'running' || w.runs[0].status === 'waiting')).length);
  const visible = $derived(ws().chats.filter((c) => !c.hidden));

  const groups = $derived(
    chatGroups
      .map((name) => ({
        name,
        chats: name === 'Starred' ? visible.filter((c) => c.starred) : visible.filter((c) => c.group === name && !c.starred)
      }))
      .filter((g) => g.chats.length)
  );

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
      <WorkspaceSwitcher
        workspaces={app.workspaces}
        current={app.workspace}
        onselect={switchWorkspace}
        onsettings={() => go('/settings/workspace')}
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
    <SidebarItem icon={FolderClosed} label="Projects" href="#/projects" active={active('/projects')} />
    <SidebarItem icon={Workflow} label="Workflows" href="#/workflows" active={active('/workflows')} badge={running ? String(running) : undefined} />
    <SidebarItem icon={Plug} label="Connectors" href="#/connectors" active={active('/connectors')} />
    <SidebarItem icon={Users} label="Users" href="#/users" active={active('/users')} />
  </SidebarSection>

  {#if !collapsed}
    {#each groups as group (group.name)}
      <SidebarSection title={group.name}>
        {#each group.chats as chat (chat.id)}
          <SidebarItem label={chat.title} href={`#/chat/${chat.id}`} active={route.path === `/chat/${chat.id}`}>
            {#snippet actions()}<ChatActions {chat} />{/snippet}
          </SidebarItem>
        {/each}
      </SidebarSection>
    {/each}
  {/if}

  {#snippet footer()}
    <AccountMenu user={{ ...me, name: app.profile.name, tone: app.profile.tone }} />
  {/snippet}
</Sidebar>

<NewWorkspaceDialog bind:open={creating} oncreate={createWorkspace} />
