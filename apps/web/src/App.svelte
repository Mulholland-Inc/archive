<script lang="ts">
  import { Shell, Toaster, Row, IconButton, Input, Select } from './lib';
  import { objectTypeItems } from './lib/ontology';
  import { route, match, navigate, query, setQuery } from './lib/router.svelte';
  import { session, load } from './lib/session.svelte';
  import { preload, preloadList } from './lib/preload';
  import { objectTypes } from './lib/spec';
  import AppSidebar from './pages/parts/AppSidebar.svelte';
  import SignIn from './pages/SignIn.svelte';
  import Setup from './pages/Setup.svelte';
  import Home from './pages/Home.svelte';
  import Records from './pages/Records.svelte';
  import RecordPage from './pages/RecordPage.svelte';
  import Search from './pages/Search.svelte';
  import Files from './pages/Files.svelte';
  import FilePage from './pages/FilePage.svelte';
  import Sources from './pages/Sources.svelte';
  import Types from './pages/Types.svelte';
  import TypePage from './pages/TypePage.svelte';
  import Changes from './pages/Changes.svelte';
  import Settings from './pages/Settings.svelte';
  import Kit from './pages/Kit.svelte';
  import Offline from './pages/Offline.svelte';
  import NotFound from './pages/NotFound.svelte';

  let collapsed = $state(localStorage.getItem('archive.sidebar') === 'collapsed');
  const narrowQuery = matchMedia('(max-width: 64rem)');
  let narrow = $state(narrowQuery.matches);
  let drawer = $state(false);
  narrowQuery.addEventListener('change', () => {
    narrow = narrowQuery.matches;
    drawer = false;
  });
  const hidden = $derived(narrow ? !drawer : collapsed);

  $effect(() => {
    route.path;
    route.search;
    drawer = false;
  });
  let theme = $state<'light' | 'dark'>(
    (localStorage.getItem('archive.theme') as 'light' | 'dark') ?? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  );

  $effect(() => {
    document.documentElement.dataset.theme = theme;
  });

  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark';
    localStorage.setItem('archive.theme', next);
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown };
    if (doc.startViewTransition && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
      doc.startViewTransition(() => {
        theme = next;
        document.documentElement.dataset.theme = next;
      });
    } else theme = next;
  }

  function toggleSidebar() {
    if (narrow) {
      drawer = !drawer;
      return;
    }
    collapsed = !collapsed;
    localStorage.setItem('archive.sidebar', collapsed ? 'collapsed' : 'open');
  }

  const publicPage = $derived(route.path === '/signin' || route.path === '/setup' || route.path === '/kit');

  load();

  $effect(() => {
    if (session.status === 'signedout' && !publicPage) {
      navigate(`/signin?next=${encodeURIComponent(route.path + route.search)}`, { replace: true });
    }
    if (session.status === 'ready' && (route.path === '/signin' || route.path === '/setup')) {
      navigate(new URLSearchParams(route.search).get('next') ?? '/', { replace: true });
    }
  });

  $effect(() => {
    if (session.status !== 'ready') return;
    const types = objectTypes(session.ontology);
    session.space;
    const id = setTimeout(() => types.forEach((t) => preloadList(t)), 400);
    return () => clearTimeout(id);
  });

  let find = $state('');

  $effect(() => {
    find = route.path === '/search' ? (query().get('q') ?? '') : '';
  });

  const typeItems = $derived([{ id: '', label: 'Everything', icon: 'layers' }, { id: 'file', label: 'Files', icon: 'folder' }, ...objectTypeItems()]);
  const findType = $derived(route.path === '/search' ? (query().get('type') ?? '') : '');

  function searchType(t: string) {
    if (route.path === '/search') setQuery({ type: t || undefined });
    else navigate(`/search?${new URLSearchParams({ ...(find ? { q: find } : {}), ...(t ? { type: t } : {}) })}`);
  }

  function search(e: Event) {
    const v = (e.target as HTMLInputElement).value;
    if (route.path === '/search') setQuery({ q: v || undefined });
    else if (v) navigate(`/search?q=${encodeURIComponent(v)}`);
  }

  function hover(e: Event) {
    const a = (e.target as Element).closest?.('a[href]');
    if (a) preload(a.getAttribute('href')!);
  }

  const params = $derived.by(() => {
    const p = route.path;
    return {
      record: match('/records/:type/:id', p),
      records: match('/records/:type', p),
      view: match('/views/:id', p),
      file: match('/library/:id', p),
      type: match('/types/:name', p)
    };
  });
</script>

<svelte:document onpointerover={hover} onfocusin={hover} />

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && drawer) drawer = false;
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k' && session.status === 'ready') {
      e.preventDefault();
      document.getElementById('find')?.focus();
    }
  }}
/>

{#if route.path === '/kit'}
  <Kit />
{:else if route.path === '/signin'}
  <SignIn />
{:else if route.path === '/setup'}
  <Setup />
{:else if session.status === 'offline'}
  <Offline />
{:else if session.status === 'ready'}
  <Shell collapsed={hidden} onclose={() => (drawer = false)}>
    {#snippet sidebar()}
      <AppSidebar {theme} ontheme={toggleTheme} oncollapse={toggleSidebar} />
    {/snippet}
    {#snippet topbar()}
      <div class="top">
        <Row gap="1">
          {#if hidden}<IconButton icon={narrow ? 'rows' : 'sidebar'} label="Show menu" onclick={toggleSidebar} />{/if}
        </Row>
        <div class="find">
          <Input
            id="find"
            bind:value={find}
            type="search"
            icon="search"
            placeholder="Search everything  ⌘K"
            label="Search everything"
            oninput={search}
            onkeydown={(e) => {
              if (e.key === 'Escape') (e.target as HTMLInputElement).blur();
              if (e.key === 'Enter' && find && route.path !== '/search') navigate(`/search?q=${encodeURIComponent(find)}`);
            }}
          />
          <Select icon="filter" variant="secondary" value={findType} options={typeItems} searchable={typeItems.length > 8} onchange={searchType} label="Search in" />
        </div>
      </div>
    {/snippet}
    {#key route.path}
      {#if route.path === '/'}
        <Home />
      {:else if params.record}
        <RecordPage type={params.record.type} id={params.record.id} />
      {:else if params.records}
        <Records type={params.records.type} />
      {:else if params.view}
        <Records viewId={params.view.id} />
      {:else if route.path === '/search'}
        <Search />
      {:else if route.path === '/library'}
        <Files />
      {:else if params.file}
        <FilePage id={params.file.id} />
      {:else if route.path === '/sources'}
        <Sources />
      {:else if route.path === '/types'}
        <Types />
      {:else if params.type}
        <TypePage name={params.type.name} />
      {:else if route.path === '/changes'}
        <Changes />
      {:else if route.path === '/settings'}
        <Settings />
      {:else}
        <NotFound />
      {/if}
    {/key}
  </Shell>
{/if}

{#if route.path !== '/kit'}<Toaster />{/if}

<style>
  .top {
    display: grid;
    grid-template-columns: 1fr minmax(0, 28rem) 1fr;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-width: 0;
  }
  .find {
    display: flex;
    gap: var(--space-2);
    min-width: 0;
  }
  .find :global(.button) {
    height: 2rem;
  }
  .find :global(.input) {
    height: 2rem;
    background: var(--card);
  }
  @media (max-width: 40rem) {
    .top {
      grid-template-columns: auto 1fr;
    }
  }
</style>
