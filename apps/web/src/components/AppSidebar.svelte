<script>
  import { Plus, Library, Database, Workflow, Blocks, Lock, Settings } from '@lucide/svelte';
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
  import AgentAvatar from './AgentAvatar.svelte';
  import AgentDialog from './AgentDialog.svelte';
  import { app, ws, openSettings, closeDrawer, switchArchive, createArchive, lastAgent, unread, renameView, atLeast, agents, workflows, members, schema } from '../lib/app.svelte.js';
  import { route, active } from '../lib/router.svelte.js';
  import NewArchiveDialog from './NewArchiveDialog.svelte';
  import SetupCard from './SetupCard.svelte';
  import { agentItems } from '../lib/menus.js';

  agents.use();
  schema.use();
  workflows.use();
  members.use();

  const running = $derived(ws().workflows.filter((w) => w.runs[0] && (w.runs[0].status === 'running' || w.runs[0].status === 'waiting')).length);
  const here = $derived(route.parts[0] === 'agent' ? route.parts[1] : route.path === '/' ? lastAgent()?.id : null);
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
    <Menu label={ws().name} width="md">
      {#snippet trigger(props)}<ArchiveHeader archive={ws()} {...props} />{/snippet}
      <MenuItem icon={Workflow} href="/workflows">
        {#snippet end()}{#if running}<Badge>{running}</Badge>{/if}{/snippet}
        Workflows
      </MenuItem>
      <MenuItem icon={Library} href="/library">Library</MenuItem>
      <MenuItem icon={Database} href="/database">Database</MenuItem>
      <MenuItem icon={Blocks} onselect={() => { app.drawer = false; openSettings('integrations'); }}>Integrations</MenuItem>
      <MenuItem icon={Lock} href="/access">Access</MenuItem>
      <MenuSeparator />
      <MenuItem icon={Settings} onselect={() => { app.drawer = false; openSettings('general'); }}>Settings</MenuItem>
    </Menu>
  {/snippet}

  <SetupCard />

  <SidebarSection>
    {#if atLeast('manager')}<SidebarItem icon={Plus} label="New agent" accent onclick={() => { app.drawer = false; app.agentDialog = true; }} />{/if}
  </SidebarSection>

  {#if ws().agents.length}
    <SidebarSection title={collapsed ? undefined : 'Agents'}>
      {#each ws().agents as agent (agent.id)}
        <SidebarItem label={agent.name} tall={!collapsed} detail={agent.description} dot={unread(agent) && here !== agent.id} href={`/agent/${agent.id}`} active={here === agent.id} items={() => agentItems(agent)} ondouble={() => (app.agentDialog = agent.id)}>
          {#snippet leading()}
            <AgentAvatar avatar={agent.avatar} tint={agent.tint} seed={agent.id} size={collapsed ? 'xs' : 'lg'} active={agent.streaming} />
          {/snippet}
        </SidebarItem>
      {/each}
    </SidebarSection>
  {/if}

  {#if views.length}
    <SidebarSection title={collapsed ? undefined : 'Views'}>
      {#each views as view (view.id)}
        <SidebarItem icon={layoutOf(view).icon} label={view.name} href={`/view/${view.id}`} active={active(`/view/${view.id}`)} onrename={atLeast('manager') ? (name) => renameView(view.id, name) : undefined} bind:editing={() => renaming === view.id, (on) => (renaming = on ? view.id : null)} />
      {/each}
    </SidebarSection>
  {/if}

  {#snippet footer()}
    <AccountCard user={app.user} />
  {/snippet}
</Sidebar>

<NewArchiveDialog bind:open={creating} oncreate={createArchive} />
<AgentDialog />
