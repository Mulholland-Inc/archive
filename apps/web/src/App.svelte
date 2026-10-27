<script>
  import { api, go, icon, Unauthorized } from './api.js';
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
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
  let icons = $state({});
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
  const initials = $derived(who.split(/[\s@.]+/).filter(Boolean).slice(0, 2).map((w) => w[0].toUpperCase()).join(''));

  async function load() {
    try {
      me = await api('/auth/me');
      const cat = await api('/api/types');
      const all = Object.values(cat.tables).sort((a, b) => a.name.localeCompare(b.name));
      icons = Object.fromEntries(await Promise.all(all.map(async (t) => [t.name, await icon(t.icon)])));
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
      <a class="brand" href="/files"><Logo height={19} /></a>
      <div class="rows">
        <a class="row" href="/files" class:on={route.first === 'files'}><Icon name="folder" size={22} />Files</a>
        <a class="row" href="/pages" class:on={route.first === 'pages'}><Icon name="page" size={22} />Pages</a>
        {#if views.length}
          <div class="label">Views</div>
          {#each views as v (v.name)}
            {@const ViewIcon = icons[v.name]}
            <a class="row" href="/views/{v.name}" class:on={route.first === 'views' && route.second === v.name}><ViewIcon size={21} strokeWidth={1.75} />{v.name.replaceAll('_', ' ')}</a>
          {/each}
        {/if}
        <div class="label">Tables</div>
        {#each tables as t (t.name)}
          {@const TableIcon = icons[t.name]}
          <a class="row" href="/{t.name}" class:on={route.first === t.name}><TableIcon size={21} strokeWidth={1.75} />{t.name.replaceAll('_', ' ')}</a>
        {/each}
      </div>
      <div class="foot">
        <span class="avatar">{initials}</span>
        <span class="name">{who}</span>
        <button class="quiet" title="Sign out" onclick={signOut}><Icon name="signout" /></button>
      </div>
    </aside>
    <main>
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
        <h1>Not here</h1>
        <p class="lead">There is no table, view or page at this address.</p>
      {/if}
    </main>
  </div>
{/if}

<style>
  .app { display: grid; grid-template-columns: 285px minmax(0, 1fr); height: 100vh; }
  aside { display: flex; flex-direction: column; min-height: 0; padding: 0 10px; }
  .brand { display: flex; align-items: center; height: 56px; padding: 0 10px; color: var(--text); }
  .rows { flex: 1; overflow-y: auto; min-height: 0; padding-bottom: 12px; scrollbar-width: none; }
  .rows::-webkit-scrollbar { display: none; }
  .row {
    display: flex;
    align-items: center;
    gap: 9px;
    height: 40px;
    padding: 0 10px;
    border-radius: 999px;
    font-weight: 450;
    letter-spacing: -0.02em;
    text-transform: capitalize;
    color: var(--text);
    transition: background var(--dur) var(--ease);
  }
  .row :global(svg) { flex: none; }
  .row:hover { background: var(--hover); }
  .row.on { background: var(--cream); color: var(--cream-ink); font-weight: 600; }
  .label { font-size: 14px; line-height: 37px; padding: 8px 10px 0; color: var(--text-2); }
  .foot { display: flex; align-items: center; gap: 10px; height: 60px; padding: 0 6px 0 10px; border-top: 1px solid var(--line); }
  .avatar {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--cream);
    color: var(--cream-ink);
    font-size: 10px;
    font-weight: 700;
    flex: none;
  }
  .name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 15px; }
  main { overflow: auto; min-width: 0; padding: 72px 48px 48px; }
  @media (max-width: 767px) {
    .app { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
    aside { flex-direction: row; align-items: center; padding: 6px 8px; border-bottom: 1px solid var(--line); }
    .brand { padding: 0 8px; height: 44px; }
    .rows { display: flex; gap: 2px; overflow-x: auto; padding: 0; }
    .label, .foot { display: none; }
    .row { white-space: nowrap; flex: none; }
    .row :global(svg) { display: none; }
    main { padding: 32px 16px; }
  }
</style>
