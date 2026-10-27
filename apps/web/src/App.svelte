<script>
  import { api, go, known, preload, scope, Unauthorized } from './api.js';
  import Button from './ui/Button.svelte';
  import Heading from './ui/Heading.svelte';
  import Divider from './ui/Divider.svelte';
  import Logo from './ui/Logo.svelte';
  import NavItem from './ui/NavItem.svelte';
  import SignIn from './SignIn.svelte';
  import Table from './Table.svelte';
  import View from './View.svelte';
  import Files from './Files.svelte';
  import Page from './Page.svelte';
  import Connectors from './Connectors.svelte';
  import Chat from './Chat.svelte';
  import Ontology from './Ontology.svelte';
  import Profile from './ui/Profile.svelte';
  import Drawer from './ui/Drawer.svelte';
  import { call, native, on } from './native.js';
  import People from './ui/People.svelte';
  import { top } from './top.svelte.js';

  let me = $state(null);
  let signedOut = $state(false);
  let tables = $state([]);
  let views = $state([]);
  let types = $state({});
  let path = $state(location.pathname);
  let space = $state('');
  let conversation = $state([]);
  let fresh = $state(0);
  let drawer = $state(false);
  let cover = $state(0);

  $effect(() => on('height', ({ px }) => (cover = px)));

  $effect(() => {
    if (!native || !me) return;
    call('topBar', { show: true });
    const off = on('tap', ({ action }) => {
      if (action === 'sidebar') drawer = !drawer;
      else if (action === 'newChat') newChat();
    });
    const offSpace = on('space', ({ value }) => (space = value));
    const offAdd = on('addSpace', ({ name }) => addSpace(name).catch((e) => alert(e.message)));
    return () => {
      off();
      offSpace();
      offAdd();
      call('spaces', { show: false });
      call('topBar', { show: false });
    };
  });
  let theme = $state(localStorage.theme ?? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'));

  $effect(() => {
    document.documentElement.dataset.theme = theme;
    window.Capacitor?.nativePromise?.('StatusBar', 'setStyle', { style: theme === 'light' ? 'LIGHT' : 'DARK' }).catch(() => {});
  });

  function flip() {
    theme = localStorage.theme = theme === 'light' ? 'dark' : 'light';
  }

  addEventListener('popstate', () => (path = location.pathname));
  const hover = (e) => {
    const href = e.target.closest?.('a[href^="/"]')?.getAttribute('href');
    if (href && !/^\/(api|auth)\//.test(href)) preload(href);
  };
  for (const type of ['pointerover', 'touchstart', 'focusin']) addEventListener(type, hover, { passive: true });

  $effect(() => {
    if (!me) return;
    const here = path;
    const id = requestIdleCallback(() => {
      const links = [...document.querySelectorAll('aside nav a')].map((a) => a.getAttribute('href'));
      const i = links.findIndex((h) => here === h || here.startsWith(`${h}/`));
      for (const h of i < 0 ? links.slice(0, 2) : [links[i - 1], links[i + 1]]) if (h) preload(h);
    });
    return () => cancelIdleCallback(id);
  });
  addEventListener('click', (e) => {
    const a = e.target.closest?.('a[href^="/"]');
    if (!a || e.defaultPrevented || a.target || e.metaKey || e.ctrlKey || e.shiftKey || /^\/(api|auth)\//.test(a.getAttribute('href'))) return;
    e.preventDefault();
    drawer = false;
    go(a.getAttribute('href'));
  });

  $effect(() => {
    if (!native) return;
    call('scrolling', { flow: true });
    window.Capacitor.nativePromise('Keyboard', 'setResizeMode', { mode: 'none' }).catch(() => {});
  });

  const route = $derived.by(() => {
    const [, first = '', second = ''] = path.split('/').map(decodeURIComponent);
    return { first, second };
  });
  const table = (name) => tables.find((t) => t.name === name);
  const who = $derived(me ? me.actor.name || me.actor.email : '');
  $effect(() => {
    if (!native || !me) return;
    const r = drawer && document.querySelector('aside .profile')?.getBoundingClientRect();
    call('spaces', { show: !!r, items: $state.snapshot(spaces), value: space, frame: r ? { x: r.left, y: r.top, w: r.width, h: r.height } : {} });
  });

  const spaces = $derived((me?.spaces ?? []).map((s) => ({ value: s.name, label: s.id === me.home ? 'Personal' : s.name.replaceAll('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase()) })));

  $effect(() => {
    if (!space) return;
    scope(space);
    localStorage.space = space;
  });

  async function addSpace(name) {
    const made = await api('/auth/spaces', { method: 'POST', body: { name: name.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_|_$/g, '') } });
    me = await api('/auth/me');
    space = made.name;
  }

  function newChat(e) {
    e?.preventDefault();
    conversation = [];
    drawer = false;
    fresh++;
    if (path !== '/chat') go('/chat');
  }

  async function load() {
    try {
      me = await api('/auth/me');
      const names = me.spaces.map((s) => s.name);
      const pick = names.includes(localStorage.space) ? localStorage.space : me.spaces.find((s) => s.id === me.home)?.name ?? names[0] ?? '';
      scope(pick);
      space = pick;
      const cat = await api('/api/types');
      known(cat);
      const all = Object.values(cat.tables).sort((a, b) => a.name.localeCompare(b.name));
      types = cat.types;
      tables = all.filter((t) => t.kind === 'table');
      views = all.filter((t) => t.kind === 'view');
      if (location.pathname === '/') history.replaceState(null, '', (path = '/chat'));
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
  <div class="app" style:--cover="{cover}px">
    <Drawer bind:open={drawer} onshift={(px, p, animate) => call('shift', { px, p, animate })}>
      {#snippet side()}
        <a class="brand" href="/chat" aria-label="Chat"><Logo /></a>
        <nav>
          <NavItem href="/chat" icon="square-pen" active={route.first === 'chat'} onclick={newChat}>Chat</NavItem>
          <NavItem href="/files" icon="folder" active={route.first === 'files'}>Files</NavItem>
          <NavItem href="/pages" icon="page" active={route.first === 'pages'}>Pages</NavItem>
          <NavItem href="/ontology" icon="waypoints" active={route.first === 'ontology'}>Ontology</NavItem>
          {#if views.length}
            <Divider />
            {#each views as v (v.name)}
              <NavItem href="/views/{v.name}" icon={v.icon} active={route.first === 'views' && route.second === v.name}>{v.name.replaceAll('_', ' ')}</NavItem>
            {/each}
          {/if}
          <Divider />
          {#each tables as t (t.name)}
            <NavItem href="/{t.name}" icon={t.icon} active={route.first === t.name}>{t.name.replaceAll('_', ' ')}</NavItem>
          {/each}
        </nav>
        <footer>
          <Profile name={who} items={spaces} bind:value={space} onadd={addSpace} />
          <Button quiet href="/connectors" icon="plug" title="Connectors" aria-label="Connectors" />
          <Button quiet icon={theme === 'light' ? 'moon' : 'sun'} title="Switch theme" aria-label="Switch theme" onclick={flip} />
          <Button quiet icon="signout" title="Sign out" aria-label="Sign out" onclick={signOut} />
        </footer>
      {/snippet}
      <header class="top" class:native>
        <Button quiet icon="panel-left" title="Open sidebar" aria-label="Open sidebar" aria-expanded={drawer} onclick={() => (drawer = true)} />
        {#if top.people}<People live={top.live} people={top.people} />{/if}
        <Button quiet href="/chat" icon="square-pen" title="New chat" aria-label="New chat" onclick={newChat} />
      </header>
      <main>
        {#key space}
        {#if route.first === 'chat'}
          {#key fresh}
            <Chat bind:messages={conversation} />
          {/key}
        {:else if route.first === 'files'}
          <Files />
        {:else if route.first === 'pages' && route.second}
          {#key route.second}
            <Page id={route.second} name={who} />
          {/key}
        {:else if route.first === 'pages'}
          <Files pages />
        {:else if route.first === 'ontology'}
          <Ontology {tables} {views} {types} />
        {:else if route.first === 'connectors'}
          <Connectors {space} />
        {:else if route.first === 'views'}
          {@const v = views.find((x) => x.name === route.second)}
          {#if v}
            {#key v.name}
              <View view={v} {types} />
            {/key}
          {/if}
        {:else if table(route.first)}
          {#key route.first}
            <Table table={table(route.first)} {types} />
          {/key}
        {:else}
          <Heading title="Not here" />
        {/if}
        {/key}
      </main>
    </Drawer>
  </div>
{/if}

<style>
  .app { display: grid; grid-template-columns: var(--sidebar) minmax(0, 1fr); grid-template-rows: auto 1fr; column-gap: var(--gutter); min-height: 100vh; padding: var(--gutter); }
  .app::before { content: ''; grid-column: 1 / -1; height: var(--safe-top); }
  .top { display: none; }
  .brand { display: flex; align-items: center; padding: var(--space-3); }
  nav {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    mask-image: linear-gradient(transparent, black var(--space-5), black calc(100% - var(--space-5)), transparent);
  }
  nav::before, nav::after { content: ''; flex: 0 0 var(--space-5); }
  footer { display: flex; align-items: center; gap: var(--space-1); }
  main { display: flex; flex-direction: column; gap: var(--space-5); min-width: 0; }
  main > :global(:last-child) { flex: 1; }
  @media (max-width: 767px) {
    .app { display: block; padding: 0; }
    .app::before { content: none; }
    .top { display: grid; flex: none; grid-template-columns: var(--bar-inset) var(--size-bar) minmax(0, 1fr) var(--size-bar) var(--bar-inset); align-items: center; height: var(--bar); padding: 0; }
    .top::before, .top::after { content: ''; }
    .top::after { grid-column: 5; }
    .top > :global(a) { grid-column: 4; }
    .top > :global(.people) { grid-column: 3; justify-self: center; }
    .top :global(.button) { width: var(--size-bar); height: var(--size-bar); color: var(--text); }
    .top :global(.button svg.icon) { width: var(--text-xl); height: var(--text-xl); }
    .native :global(.button) { visibility: hidden; }
    .top { position: sticky; top: var(--safe-top); z-index: 2; background: var(--shell); }
    main { flex: 1; padding: var(--space-4); }
  }
</style>
