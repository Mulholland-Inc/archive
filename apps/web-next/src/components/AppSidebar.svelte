<script>
  import { PanelLeft, Plus, Search, MessagesSquare, Library, FolderClosed, ListChecks, Users } from '@lucide/svelte';
  import Sidebar from './Sidebar.svelte';
  import SidebarItem from './SidebarItem.svelte';
  import SidebarSection from './SidebarSection.svelte';
  import WorkspaceSwitcher from './WorkspaceSwitcher.svelte';
  import AccountMenu from './AccountMenu.svelte';
  import IconButton from './IconButton.svelte';
  import ChatActions from './ChatActions.svelte';
  import Stack from './Stack.svelte';
  import { app, toggleSidebar } from '../lib/app.svelte.js';
  import { me, workspaces, chatGroups, tasks } from '../lib/data.js';
  import { route, active, go } from '../lib/router.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  const running = tasks.filter((t) => t.status === 'running' || t.status === 'waiting').length;

  const groups = $derived(
    chatGroups
      .map((name) => ({
        name,
        chats: name === 'Starred' ? app.chats.filter((c) => c.starred) : app.chats.filter((c) => c.group === name && !c.starred)
      }))
      .filter((g) => g.chats.length)
  );

  const collapsed = $derived(app.collapsed && !app.drawer);

  $effect(() => {
    route.path;
    app.drawer = false;
  });
</script>

<Sidebar collapsed={app.collapsed} drawer={app.drawer} onclose={() => (app.drawer = false)}>
  {#snippet header()}
    <Stack direction="row" gap="1" align="center">
      {#if collapsed}
        <IconButton icon={PanelLeft} label="Open sidebar" placement="right" shortcut="⌘ ." onclick={toggleSidebar} />
      {:else}
        <WorkspaceSwitcher
          {workspaces}
          current={app.workspace}
          email={me.email}
          onselect={(id) => { app.workspace = id; toast(`Switched to ${workspaces.find((w) => w.id === id).name}`); }}
          onsettings={() => go('/settings/workspace')}
          oncreate={() => toast('New workspace created')}
        />
        <IconButton icon={PanelLeft} label="Close sidebar" shortcut="⌘ ." onclick={toggleSidebar} />
      {/if}
    </Stack>
  {/snippet}

  <SidebarSection>
    <SidebarItem icon={Plus} label="New chat" href="#/" accent shortcut="⌘ ⇧ O" />
    <SidebarItem icon={Search} label="Search" shortcut="⌘ K" onclick={() => (app.palette = true)} />
  </SidebarSection>

  <SidebarSection>
    <SidebarItem icon={MessagesSquare} label="Chats" href="#/chats" active={active('/chats')} />
    <SidebarItem icon={Library} label="Library" href="#/library" active={active('/library')} />
    <SidebarItem icon={FolderClosed} label="Projects" href="#/projects" active={active('/projects')} />
    <SidebarItem icon={ListChecks} label="Tasks" href="#/tasks" active={active('/tasks')} badge={running ? String(running) : undefined} />
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
    <AccountMenu user={me} />
  {/snippet}
</Sidebar>
