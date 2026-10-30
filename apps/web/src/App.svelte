<script>
  import { api, go, setSpace, space, Unauthorized } from './api.js';
  import Button from '#kit/Button.svelte';
  import Theme from '#kit/Theme.svelte';
  import Word from '#kit/Word.svelte';
  import Profile from './ui/Profile.svelte';
  import Heading from './ui/Heading.svelte';
  import NavItem from './ui/NavItem.svelte';
  import SignIn from './SignIn.svelte';
  import Table from './Table.svelte';
  import View from './View.svelte';
  import Files from './Files.svelte';
  import Chat from './Chat.svelte';

  let me = $state(null);
  let signedOut = $state(false);
  let tables = $state([]);
  let views = $state([]);
  let types = $state({});
  let path = $state(location.pathname);
  let chats = $state([]);
  let scope = $state(space());
  let generation = $state(0);

  addEventListener('popstate', () => (path = location.pathname));
  addEventListener('click', (e) => {
    const a = e.target.closest?.('a[href^="/"]');
    if (!a || a.target || e.metaKey || e.ctrlKey || e.shiftKey || /^\/(api|auth)\//.test(a.getAttribute('href'))) return;
    e.preventDefault();
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

  function pick(name) {
    setSpace(name);
    scope = name;
    generation++;
  }

  async function load() {
    try {
      me = await api('/auth/me');
      if (scope && !me.spaces.some((s) => s.name === scope)) pick('');
      loadChats();
      const cat = await api('/api/types');
      const all = Object.values(cat.tables).sort((a, b) => a.name.localeCompare(b.name));
      types = cat.types;
      tables = all.filter((t) => t.kind === 'table');
      views = all.filter((t) => t.kind === 'view');
      if (location.pathname === '/') history.replaceState(null, '', (path = '/files'));
    } catch (e) {
      if (e instanceof Unauthorized) signedOut = true;
      else throw e;
    }
  }

  async function signOut() {
    await api('/auth/signout', { method: 'POST' });
    location.assign('/');
  }

  load();
</script>

{#if signedOut}
  <SignIn />
{:else if me}
  <div class="shell">
    <aside>
      <a class="brand" href="/files" aria-label="Archive"><Word /></a>
      <nav>
        <NavItem href="/chat" icon="plus" active={route.first === 'chat' && !route.second}>New chat</NavItem>
        <NavItem href="/files" icon="folder" active={route.first === 'files'}>Files</NavItem>
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
    <main>
      {#key generation}
      {#if route.first === 'chat'}
        <Chat id={route.second} name={me.actor.name} onsaved={loadChats} />
      {:else if route.first === 'files'}
        <Files />
      {:else if route.first === 'views'}
        {@const v = views.find((x) => x.name === route.second)}
        {#if v}
          {#key v.name}
            <View view={v} {types} />
          {/key}
        {/if}
      {:else if table(route.first)}
        {#key route.first}
          <Table table={table(route.first)} />
        {/key}
      {:else}
        <Heading plain title="Not here" />
      {/if}
      {/key}
    </main>
  </div>
{/if}

<style>
  .shell {
    display: grid;
    grid-template-columns: var(--app-columns);
    grid-template-rows: var(--shell-rows);
    gap: var(--frame);
    min-height: calc(100svh - 2 * var(--frame) - env(safe-area-inset-top));
  }
  aside {
    position: sticky;
    top: var(--bar-top);
    bottom: var(--bar-bottom);
    z-index: var(--z-raised);
    order: var(--bar-order);
    display: flex;
    flex-direction: var(--bar-direction);
    align-items: stretch;
    align-self: start;
    min-width: 0;
    height: var(--bar-height);
    padding: var(--bar-pad);
    padding-bottom: calc(var(--bar-pad) + env(safe-area-inset-bottom));
    background: var(--bg);
  }
  .chats { display: flex; flex: none; flex-direction: var(--bar-direction); gap: var(--space-0-5); }
  .brand { display: var(--wide-only); flex: none; align-items: center; min-height: var(--size-control-lg); padding: var(--item-pad); color: var(--fg); }
  .brand :global(.word) { height: var(--text-lg); width: auto; fill: currentColor; }
  nav {
    display: flex;
    flex: 1;
    flex-direction: var(--bar-direction);
    gap: var(--space-0-5);
    min-width: 0;
    min-height: 0;
    overflow: auto;
    overscroll-behavior-y: none;
  }
  hr { flex: none; align-self: stretch; min-width: var(--hairline); min-height: var(--hairline); margin: var(--space-2); border: 0; }
  footer { display: flex; flex: none; align-items: center; gap: var(--space-1); }
  main { display: flex; flex-direction: column; min-width: 0; height: var(--main-height); }
  main > :global(header) { position: sticky; top: 0; z-index: var(--z-raised); padding: var(--header-pad) var(--header-inset); background: var(--bg); }
</style>
