<script lang="ts">
  import { Sidebar, SidebarSection, SidebarItem, WorkspaceSwitcher, IconButton, Popover, Menu, Spacer, type MenuItem } from '../../lib';
  import { session, setSpace, signOut, isAdmin, spaceLabel, reloadPending } from '../../lib/session.svelte';
  import { route, navigate } from '../../lib/router.svelte';
  import { objectTypes, humanize, typeIcon } from '../../lib/spec';
  import NewTypeDialog from './NewTypeDialog.svelte';
  import { api } from '../../lib/api';
  import { hasIcon } from '../../lib/components/Icon.svelte';
  import type { ModuleApp } from '../../lib/types';

  let { theme, ontheme, oncollapse }: { theme: 'light' | 'dark'; ontheme: () => void; oncollapse: () => void } = $props();

  let spaceOpen = $state(false);
  let accountOpen = $state(false);
  let newType = $state(false);
  let typesOpen = $state(true);
  let viewsOpen = $state(true);

  const types = $derived(objectTypes(session.ontology));

  // pages a module built on the engine adds, such as a family archive's tree
  let apps = $state<ModuleApp[]>([]);
  $effect(() => {
    if (session.status === 'ready') api.apps().then((a) => (apps = a), () => (apps = []));
  });

  $effect(() => {
    route.path;
    session.space;
    reloadPending();
  });
  const views = $derived(session.views.filter((v) => !session.space || session.spaces.find((s) => s.id === v.space)?.name === session.space));

  const spaceItems = $derived<MenuItem[]>([
    { id: '', label: 'All spaces', icon: 'layers' },
    ...session.spaces.map((s, i) => ({ id: s.name, label: spaceLabel(s.name), icon: 'folder', trailing: humanize(s.level), separator: i === 0 })),
    { id: '#manage', label: 'Manage spaces', icon: 'settings', separator: true }
  ]);

  const accountItems: MenuItem[] = [
    { id: 'settings', label: 'Settings', icon: 'settings' },
    { id: 'theme', label: 'Switch theme', icon: 'moon' },
    { id: 'signout', label: 'Sign out', icon: 'logout', separator: true }
  ];

  const at = (prefix: string) => route.path === prefix || route.path.startsWith(prefix + '/');
</script>

<Sidebar>
  {#snippet header()}
    <Popover bind:open={spaceOpen}>
      {#snippet trigger({ toggle, open })}
        <WorkspaceSwitcher name={session.space ? spaceLabel(session.space) : 'All spaces'} pressed={open} onclick={toggle} />
      {/snippet}
      {#snippet children({ close })}
        <Menu
          items={spaceItems}
          searchable={session.spaces.length > 6}
          selected={[session.space]}
          onselect={(item) => {
            close();
            if (item.id === '#manage') navigate('/settings');
            else setSpace(item.id);
          }}
        />
      {/snippet}
    </Popover>
    <IconButton icon="sidebar" label="Hide sidebar" onclick={oncollapse} />
  {/snippet}

  <SidebarSection>
    <SidebarItem href="/" icon="home" label="Home" active={route.path === '/'} />
    <SidebarItem href="/review" icon="inbox" label="Review" count={session.pending} active={at('/review')} />
    <SidebarItem href="/library" icon="folder" label="Files" active={at('/library')} />
    <SidebarItem href="/sources" icon="plug" label="Sources" active={at('/sources')} />
  </SidebarSection>

  {#if apps.length}
    <SidebarSection>
      {#each apps as app (app.path)}
        <SidebarItem href={app.path} icon={hasIcon(app.icon) ? app.icon : 'shapes'} label={app.title} external />
      {/each}
    </SidebarSection>
  {/if}

  {#if views.length}
    <SidebarSection title="Views" bind:open={viewsOpen}>
      {#each views as v (v.id)}
        <SidebarItem href={`/views/${v.id}`} icon={v.layout === 'board' ? 'columns' : v.layout === 'calendar' ? 'calendar' : 'rows'} label={v.name} active={at(`/views/${v.id}`)} />
      {/each}
    </SidebarSection>
  {/if}

  <SidebarSection title="Types" bind:open={typesOpen}>
    {#snippet action()}
      {#if isAdmin()}<IconButton icon="plus" label="New type" size="sm" onclick={() => (newType = true)} />{/if}
    {/snippet}
    {#each types as t (t)}
      <SidebarItem href={`/records/${encodeURIComponent(t)}`} icon={typeIcon(session.ontology, t)} label={humanize(t)} active={route.path.startsWith(`/records/${encodeURIComponent(t)}`)} />
    {:else}
      <SidebarItem href="/types" icon="plus" label="Define a type" />
    {/each}
  </SidebarSection>

  <SidebarSection>
    <SidebarItem href="/types" icon="shapes" label="Ontology" active={at('/types') || at('/changes')} />
  </SidebarSection>

  {#snippet footer()}
    <Popover bind:open={accountOpen} block>
      {#snippet trigger({ toggle })}
        <SidebarItem label={session.actor?.name || session.actor?.email || 'Account'} icon="user" onclick={toggle} />
      {/snippet}
      {#snippet children({ close })}
        <Menu
          items={accountItems.map((i) => (i.id === 'theme' ? { ...i, icon: theme === 'dark' ? 'sun' : 'moon', label: theme === 'dark' ? 'Light theme' : 'Dark theme' } : i))}
          searchable={false}
          onselect={(item) => {
            close();
            if (item.id === 'settings') navigate('/settings');
            if (item.id === 'theme') ontheme();
            if (item.id === 'signout') signOut();
          }}
        />
      {/snippet}
    </Popover>
    <Spacer />
    <IconButton icon={theme === 'dark' ? 'sun' : 'moon'} label="Switch theme" onclick={ontheme} />
  {/snippet}
</Sidebar>

<NewTypeDialog bind:open={newType} />
