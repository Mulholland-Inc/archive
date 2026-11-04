<script>
  import { Plus, Search, Library, Database, Workflow, Plug, Lock, Settings } from '@lucide/svelte';
  import { layoutOf } from '../layouts/index.js';
  import Sidebar from './Sidebar.svelte';
  import SidebarItem from './SidebarItem.svelte';
  import SidebarSection from './SidebarSection.svelte';
  import ArchiveHeader from './ArchiveHeader.svelte';
  import Menu from './Menu.svelte';
  import Badge from './Badge.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import ArchiveRail from './ArchiveRail.svelte';
  import AccountCard from './AccountCard.svelte';
  import ChatActions from './ChatActions.svelte';
  import PinButton from './PinButton.svelte';
  import { app, ws, openSettings, closeDrawer, pinnedFirst, switchArchive, createArchive, renameChat, renameView, isArchiveOwner, chats, workflows, members, schema } from '../lib/app.svelte.js';

  chats.use();
  schema.use();
  workflows.use();
  members.use();
  import { route, active } from '../lib/router.svelte.js';
  import NewArchiveDialog from './NewArchiveDialog.svelte';
  import SetupCard from './SetupCard.svelte';
  import { chatItems } from '../lib/menus.js';

  const running = $derived(ws().workflows.filter((w) => w.runs[0] && (w.runs[0].status === 'running' || w.runs[0].status === 'waiting')).length);
  const visible = $derived(pinnedFirst(ws().chats));
  // Views are the archive's pages: Postgres views, each drawn the way its comment says.
  const views = $derived((ws().schema ?? []).filter((t) => t.kind === 'view'));


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
    <ArchiveRail archives={app.archives} current={app.archive} onselect={switchArchive} oncreate={() => { app.drawer = false; creating = true; }} />
  {/snippet}
  {#snippet header()}
    <Menu label={ws().name} width="md" morph>
      {#snippet trigger(props)}<ArchiveHeader archive={ws()} {...props} />{/snippet}
      <MenuItem icon={Workflow} href="/workflows">
        {#snippet end()}{#if running}<Badge>{running}</Badge>{/if}{/snippet}
        Workflows
      </MenuItem>
      <MenuItem icon={Library} href="/library">Library</MenuItem>
      <MenuItem icon={Database} href="/database">Database</MenuItem>
      <MenuItem icon={Plug} href="/connectors">Connectors</MenuItem>
      <MenuItem icon={Lock} href="/access">Access</MenuItem>
      <MenuSeparator />
      <MenuItem icon={Settings} onselect={() => { app.drawer = false; openSettings('general'); }}>Settings</MenuItem>
    </Menu>
  {/snippet}

  <SetupCard />

  <SidebarSection>
    <SidebarItem icon={Plus} label="New chat" href="/" accent shortcut="⌘ ⇧ O" />
    <SidebarItem icon={Search} label="Search" shortcut="⌘ K" onclick={() => { app.drawer = false; app.palette = true; }} />
  </SidebarSection>

  {#if views.length}
    <SidebarSection title={collapsed ? undefined : 'Views'}>
      {#each views as view (view.id)}
        <SidebarItem icon={layoutOf(view).icon} label={view.name} href={`/view/${view.id}`} active={active(`/view/${view.id}`)} onrename={isArchiveOwner() ? (name) => renameView(view.id, name) : undefined} bind:editing={() => renaming === view.id, (on) => (renaming = on ? view.id : null)} />
      {/each}
    </SidebarSection>
  {/if}

  {#if !collapsed && visible.length}
    <SidebarSection title="Chats">
        {#each visible as chat (chat.id)}
        <SidebarItem label={chat.title} href={`/chat/${chat.id}`} active={route.path === `/chat/${chat.id}`} items={() => chatItems(chat, { open: false, rename: () => (renaming = chat.id) })} onrename={(title) => renameChat(chat.id, title)} bind:editing={() => renaming === chat.id, (on) => (renaming = on ? chat.id : null)}>
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
    <AccountCard user={{ ...app.user, name: app.profile.name || app.user.name }} />
  {/snippet}
</Sidebar>

<NewArchiveDialog bind:open={creating} oncreate={createArchive} />
