<script>
  import { api, Unauthorized } from './api.js';
  import Icon from './Icon.svelte';
  import Logo from './Logo.svelte';
  import SignIn from './SignIn.svelte';
  import Table from './Table.svelte';
  import Files from './Files.svelte';
  import Page from './Page.svelte';

  let me = $state(null);
  let signedOut = $state(false);
  let tables = $state([]);
  let hash = $state(location.hash);

  addEventListener('hashchange', () => (hash = location.hash));

  const route = $derived.by(() => {
    const [, kind, id] = hash.split('/');
    return { kind: kind ?? '', id: id ? decodeURIComponent(id) : '' };
  });
  const who = $derived(me ? me.actor.name || me.actor.email : '');
  const initials = $derived(who.split(/[\s@.]+/).filter(Boolean).slice(0, 2).map((w) => w[0].toUpperCase()).join(''));

  async function load() {
    try {
      me = await api('/auth/me');
      const cat = await api('/api/types');
      tables = Object.values(cat.tables).sort((a, b) => a.name.localeCompare(b.name));
      if (!location.hash) location.hash = '#/files';
    } catch (e) {
      if (e instanceof Unauthorized) signedOut = true;
      else throw e;
    }
  }

  async function signOut() {
    await api('/auth/signout', { method: 'POST' });
    location.hash = '';
    location.reload();
  }

  load();
</script>

{#if signedOut}
  <SignIn />
{:else if me}
  <div class="app">
    <aside>
      <a class="brand" href="#/files"><Logo height={19} /></a>
      <div class="rows">
        <a class="row" href="#/files" class:on={route.kind === 'files'}><Icon name="folder" size={22} />Files</a>
        <a class="row" href="#/pages" class:on={route.kind === 'pages' || route.kind === 'p'}><Icon name="page" size={22} />Pages</a>
        <div class="label">Tables</div>
        {#each tables as t (t.name)}
          <a class="row" href="#/t/{t.name}" class:on={route.kind === 't' && route.id === t.name}><Icon name="hash" size={22} />{t.name.replaceAll('_', ' ')}</a>
        {/each}
      </div>
      <div class="foot">
        <span class="avatar">{initials}</span>
        <span class="name">{who}</span>
        <button class="quiet" title="Sign out" onclick={signOut}><Icon name="signout" /></button>
      </div>
    </aside>
    <main>
      {#if route.kind === 't'}
        {#key route.id}
          <Table table={tables.find((t) => t.name === route.id)} />
        {/key}
      {:else if route.kind === 'files'}
        <Files />
      {:else if route.kind === 'pages'}
        <Files pages />
      {:else if route.kind === 'p'}
        {#key route.id}
          <Page id={route.id} name={who} />
        {/key}
      {/if}
    </main>
  </div>
{/if}

<style>
  .app { display: grid; grid-template-columns: 285px minmax(0, 1fr); height: 100vh; }
  aside { display: flex; flex-direction: column; min-height: 0; padding: 0 10px; }
  .brand { display: flex; align-items: center; height: 56px; padding: 0 10px; color: var(--text); }
  .rows { flex: 1; overflow-y: auto; min-height: 0; padding-bottom: 12px; }
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
    .row :global(.icon) { display: none; }
    main { padding: 32px 16px; }
  }
</style>
