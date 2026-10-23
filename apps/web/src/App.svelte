<script lang="ts">
  import { Shell, Toaster, Breadcrumb, Row, IconButton } from './lib';
  import { route, match, navigate } from './lib/router.svelte';
  import { session, load } from './lib/session.svelte';
  import { page } from './lib/page.svelte';
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

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && drawer) drawer = false;
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k' && session.status === 'ready') {
      e.preventDefault();
      navigate('/search');
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
      <Row gap="1">
        {#if hidden}<IconButton icon={narrow ? 'rows' : 'sidebar'} label="Show menu" onclick={toggleSidebar} />{/if}
        <Breadcrumb items={page.crumbs.map((c) => ({ label: c.label, onclick: c.href ? () => navigate(c.href!) : undefined }))} />
      </Row>
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
