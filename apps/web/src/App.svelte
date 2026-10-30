<script>
  import { setContext } from 'svelte';
  import { api, go, passkeyAsked, passkeys, setSpace, setWorkspace, space, workspace, Unauthorized } from './api.js';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import Theme from '#kit/Theme.svelte';
  import Word from '#kit/Word.svelte';
  import Profile from './ui/Profile.svelte';
  import Workspaces from './ui/Workspaces.svelte';
  import Heading from './ui/Heading.svelte';
  import NavItem from './ui/NavItem.svelte';
  import SignIn from './SignIn.svelte';
  import Table from './Table.svelte';
  import View from './View.svelte';
  import Record from './Record.svelte';
  import { isTree } from './ui/FamilyTree.svelte';
  import Files from './Files.svelte';
  import Workflows from './Workflows.svelte';
  import Users from './Users.svelte';
  import Passkey from './Passkey.svelte';
  import Chat from './Chat.svelte';

  let me = $state(null);
  let signedOut = $state(false);
  let asking = $state(false);
  let tables = $state([]);
  let views = $state([]);
  let types = $state({});
  let path = $state(location.pathname);
  let chats = $state([]);
  let scope = $state(space());
  let generation = $state(0);
  let open = $state(false);
  const toggle = () => (open = !open);

  setContext('side', {
    get open() {
      return open;
    },
    toggle
  });

  addEventListener('popstate', () => (path = location.pathname));
  addEventListener('click', (e) => {
    const a = e.target.closest?.('a[href^="/"]');
    if (!a || a.target || e.metaKey || e.ctrlKey || e.shiftKey || /^\/(api|auth)\//.test(a.getAttribute('href'))) return;
    e.preventDefault();
    open = false;
    if (a.getAttribute('href') === '/') generation++;
    go(a.getAttribute('href'));
  });

  const route = $derived.by(() => {
    const [, first = '', second = ''] = path.split('/').map(decodeURIComponent);
    return { first, second };
  });
  const table = (name) => tables.find((t) => t.name === name);
  const who = $derived(me ? me.actor.name || me.actor.email : '');

  async function loadChats() {
    try {
      chats = await api('/api/chats');
    } catch {}
  }

  async function create(name) {
    const sp = await api('/auth/spaces', { method: 'POST', body: { name } });
    me = await api('/auth/me');
    pick(sp.name);
  }

  function switchTo(id) {
    setWorkspace(id);
    setSpace('');
    location.assign('/');
  }

  function pick(name) {
    setSpace(name);
    scope = name;
    generation++;
  }

  const named = new URLSearchParams(location.search).get('workspace');
  if (named) {
    setWorkspace(named);
    history.replaceState(null, '', location.pathname);
  }

  async function load() {
    try {
      me = await api('/auth/me');
      if (me.workspace !== workspace()) setWorkspace(me.workspace);
      asking = passkeys && !passkeyAsked(me.actor.user_id);
      if (scope && !me.spaces.some((s) => s.name === scope)) pick('');
      loadChats();
      const cat = await api('/api/types');
      const all = Object.values(cat.tables).sort((a, b) => a.name.localeCompare(b.name));
      types = cat.types;
      tables = all.filter((t) => t.kind === 'table');
      views = all.filter((t) => t.kind === 'view');
    } catch (e) {
      if (!(e instanceof Unauthorized) && e.status !== 403) throw e;
      if (workspace() && !(e instanceof Unauthorized)) {
        setWorkspace('');
        return load();
      }
      signedOut = true;
    }
  }

  async function signOut() {
    await api('/auth/signout', { method: 'POST' });
    setWorkspace('');
    setSpace('');
    location.assign('/');
  }

  load();
</script>

{#if signedOut}
  <SignIn />
{:else if me}
  <div class="shell">
    {#if open}<button class="scrim" type="button" aria-label="Close sidebar" onclick={toggle}></button>{/if}
    <aside class:open>
      <div class="top">
        <a class="brand" href="/" aria-label="Archive"><Word /></a>
        <button class="side" type="button" aria-label="Sidebar" aria-expanded={open} onclick={toggle}><Icon name="panel-left" /></button>
      </div>
      {#if me.workspaces?.length > 1}
        <Workspaces workspaces={me.workspaces} workspace={me.workspace} onpick={switchTo} />
      {/if}
      <nav>
        <NavItem href="/" icon="house" active={!route.first}>Home</NavItem>
        <NavItem href="/files" icon="folder" active={route.first === 'files'}>Files</NavItem>
        <NavItem href="/workflows" icon="workflow" active={route.first === 'workflows'}>Workflows</NavItem>
        <NavItem href="/users" icon="users" active={route.first === 'users'}>Users</NavItem>
        {#if chats.length}
          <div class="chats">
            <hr />
            {#each chats as c (c.id)}
              <NavItem plain href="/chat/{c.id}" active={route.first === 'chat' && route.second === c.id}>{c.title || 'Untitled'}</NavItem>
            {/each}
          </div>
        {/if}
        {#if views.length}
          <hr />
          {#each views as v (v.name)}
            <NavItem href="/views/{v.name}" icon={v.icon} active={route.first === 'views' && route.second === v.name}>{v.name.replaceAll('_', ' ')}</NavItem>
          {/each}
        {/if}
        {#if tables.length}
          <hr />
          {#each tables as t (t.name)}
            <NavItem href="/{t.name}" icon={t.icon} active={route.first === t.name}>{t.name.replaceAll('_', ' ')}</NavItem>
          {/each}
        {/if}
      </nav>
      <footer>
        <Profile name={who} spaces={me.spaces} home={me.home} current={scope} onpick={pick} oncreate={create} />
        <Theme />
        <Button icon="signout" title="Sign out" aria-label="Sign out" onclick={signOut} />
      </footer>
    </aside>
    <main class:asking>
      {#key generation}
      {#if !route.first || route.first === 'chat'}
        <Chat id={route.second} name={me.actor.name} email={me.actor.email} onsaved={loadChats} />
      {:else if route.first === 'files'}
        <Files />
      {:else if route.first === 'workflows'}
        <Workflows />
      {:else if route.first === 'users'}
        <Users email={me.actor.email} />
      {:else if route.first === 'views'}
        {@const v = views.find((x) => x.name === route.second)}
        {#if v}
          {#key v.name}
            <View view={v} {types} />
          {/key}
        {/if}
      {:else if table(route.first) && route.second}
        {#key path}
          <Record table={table(route.first)} id={route.second} tree={route.first === 'person' ? views.find(isTree) : null} />
        {/key}
      {:else if table(route.first)}
        {#key route.first}
          <Table table={table(route.first)} />
        {/key}
      {:else}
        <Heading plain title="Not here" />
      {/if}
      {/key}
      {#if asking}<Passkey user={me.actor.user_id} ondone={() => (asking = false)} />{/if}
    </main>
  </div>
{/if}

<style>
  .shell {
    display: grid;
    grid-template-columns: var(--app-columns);
    grid-template-rows: minmax(0, 1fr);
    gap: var(--frame);
    min-height: calc(100svh - 2 * var(--frame) - env(safe-area-inset-top));
  }
  aside {
    position: var(--side-position);
    top: var(--side-top);
    bottom: var(--side-bottom);
    left: 0;
    z-index: var(--z-nav);
    display: flex;
    flex-direction: column;
    align-items: stretch;
    align-self: var(--side-align);
    width: var(--sidebar-width);
    min-width: 0;
    height: var(--side-height);
    padding: var(--side-pad-top) var(--side-pad) var(--side-pad-bottom);
    background: var(--bg);
    visibility: var(--side-visibility);
    transform: var(--side-closed);
    transition: transform var(--duration-normal) var(--ease-out), visibility var(--duration-normal);
  }
  aside.open { visibility: visible; transform: none; }
  .scrim { display: var(--narrow-only); position: fixed; inset: 0; z-index: var(--z-nav); background: var(--scrim); animation: fade var(--duration-normal) var(--ease-out) both; }
  .top { display: flex; flex: none; align-items: center; justify-content: space-between; }
  .side { display: var(--narrow-only); flex: none; align-items: center; justify-content: center; width: var(--size-control-lg); height: var(--size-control-lg); color: var(--fg-muted); transition: color var(--duration-fast); }
  .side :global(.icon) { width: var(--size-icon-lg); height: var(--size-icon-lg); }
  .side:hover { color: var(--fg); }
  .chats { display: flex; flex: none; flex-direction: column; gap: var(--space-0-5); }
  .brand { display: flex; flex: none; align-items: center; min-height: var(--size-control-lg); padding: var(--item-pad); color: var(--fg); }
  .brand :global(.word) { height: var(--text-lg); width: auto; fill: currentColor; }
  nav {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
    min-height: 0;
    overflow: auto;
    overscroll-behavior-y: none;
  }
  hr { flex: none; align-self: stretch; min-width: var(--hairline); min-height: var(--hairline); margin: var(--space-2); border: 0; }
  footer { display: flex; flex: none; align-items: center; gap: var(--space-1); }
  main { display: flex; flex-direction: column; min-width: 0; height: var(--main-height); }
  main.asking { --dock-lift: var(--prompt-lift); }
  main > :global(header) { position: sticky; top: 0; z-index: var(--z-raised); padding: var(--header-pad) var(--header-inset); background: var(--bg); }
  @keyframes fade { from { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .scrim { animation: none; } }
</style>
