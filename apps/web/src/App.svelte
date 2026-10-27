<script>
  import { api, go, Unauthorized } from './api.js';
  import Avatar from './ui/Avatar.svelte';
  import Button from './ui/Button.svelte';
  import Heading from './ui/Heading.svelte';
  import Label from './ui/Label.svelte';
  import Logo from './ui/Logo.svelte';
  import NavItem from './ui/NavItem.svelte';
  import SignIn from './SignIn.svelte';
  import Table from './Table.svelte';
  import View from './View.svelte';
  import Files from './Files.svelte';
  import Page from './Page.svelte';

  let me = $state(null);
  let signedOut = $state(false);
  let tables = $state([]);
  let views = $state([]);
  let types = $state({});
  let path = $state(location.pathname);

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

  async function load() {
    try {
      me = await api('/auth/me');
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
  <div class="app">
    <aside>
      <a class="brand" href="/files"><Logo /></a>
      <nav>
        <NavItem href="/files" icon="folder" active={route.first === 'files'}>Files</NavItem>
        <NavItem href="/pages" icon="page" active={route.first === 'pages'}>Pages</NavItem>
        {#if views.length}
          <Label>Views</Label>
          {#each views as v (v.name)}
            <NavItem href="/views/{v.name}" icon={v.icon} active={route.first === 'views' && route.second === v.name}>{v.name.replaceAll('_', ' ')}</NavItem>
          {/each}
        {/if}
        <Label>Tables</Label>
        {#each tables as t (t.name)}
          <NavItem href="/{t.name}" icon={t.icon} active={route.first === t.name}>{t.name.replaceAll('_', ' ')}</NavItem>
        {/each}
      </nav>
      <footer>
        <Avatar name={who} />
        <span class="name">{who}</span>
        <Button quiet icon="signout" title="Sign out" aria-label="Sign out" onclick={signOut} />
      </footer>
    </aside>
    <main class:document={route.first === 'pages' && route.second}>
      {#if route.first === 'files'}
        <Files />
      {:else if route.first === 'pages' && route.second}
        {#key route.second}
          <Page id={route.second} name={who} />
        {/key}
      {:else if route.first === 'pages'}
        <Files pages />
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
        <Heading title="Not here" />
      {/if}
    </main>
  </div>
{/if}

<style>
  .app { display: grid; grid-template-columns: var(--sidebar) minmax(0, 1fr); height: 100vh; }
  aside { display: flex; flex-direction: column; min-height: 0; padding: var(--space-2); }
  .brand { display: flex; align-items: center; padding: var(--space-3); }
  nav {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    scrollbar-width: none;
    mask-image: linear-gradient(transparent, black var(--space-5), black calc(100% - var(--space-5)), transparent);
  }
  nav::before, nav::after { content: ''; flex: 0 0 var(--space-5); }
  footer { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-3); }
  .name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  main { display: flex; flex-direction: column; min-width: 0; min-height: 0; padding: var(--space-7); overflow: hidden; }
  main > :global(section:last-child), main > :global(.board) { flex: 1; min-height: 0; overflow: auto; }
  main.document { overflow: auto; }
  @media (max-width: 767px) {
    .app { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(0, 1fr) auto; }
    aside { order: 1; flex-direction: row; align-items: center; padding: var(--space-3); margin-bottom: env(safe-area-inset-bottom); }
    nav { flex-direction: row; gap: var(--space-1); overflow-x: auto; mask-image: linear-gradient(to right, transparent, black var(--space-5), black calc(100% - var(--space-5)), transparent); }
    nav > :global(div), footer { display: none; }
    main { padding: var(--space-4); mask-image: linear-gradient(black calc(100% - var(--space-5)), transparent); }
  }
</style>
