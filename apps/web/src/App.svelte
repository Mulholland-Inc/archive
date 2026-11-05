<script>
  import AppSidebar from './components/AppSidebar.svelte';
  import ContextMenu from './components/ContextMenu.svelte';
  import Agent from './pages/Agent.svelte';
  import LibraryPage from './pages/Library.svelte';
  import DatabasePage from './pages/Database.svelte';
  import Workflows from './pages/Workflows.svelte';
  import Workflow from './pages/Workflow.svelte';
  import DatabaseTable from './pages/DatabaseTable.svelte';
  import ViewPage from './pages/View.svelte';
  import RowPage from './pages/Row.svelte';
  import AccessPage from './pages/Access.svelte';
  import GroupPage from './pages/Group.svelte';
  import Canvas from './components/Canvas.svelte';
  import AppSettings from './components/AppSettings.svelte';
  import Onboarding from './pages/Onboarding.svelte';
  import { route, replace } from './lib/router.svelte.js';
  import { app, ws, onboards, toggleSidebar, closeDrawer, openSettings, boot, account, lastAgent } from './lib/app.svelte.js';
  import PromptHost from './components/PromptHost.svelte';
  import HelpDialog from './components/HelpDialog.svelte';
  import SignIn from './components/SignIn.svelte';
  import PasskeyPrompt from './components/PasskeyPrompt.svelte';
  import Loading from './components/Loading.svelte';

  boot();

  account.use(() => app.status === 'ready');

  let last = '/';

  $effect(() => {
    const [head, a, b] = route.parts;
    if (head === 'settings') {
      openSettings(a ?? 'appearance', b ?? null);
      replace(last);
    } else {
      last = route.path;
    }
  });

  const key = $derived(route.parts[0] === 'access' && route.parts[2] ? route.parts.slice(0, 3).join('/') : ['library', 'access'].includes(route.parts[0]) ? route.parts[0] : route.parts.slice(0, 2).join('/') || 'home');

  function onkeydown(event) {
    if (event.key === 'Escape' && app.drawer && !document.querySelector('dialog[open], :popover-open')) {
      closeDrawer();
      return;
    }
    const mod = event.metaKey || event.ctrlKey;
    if (!mod) return;
    const k = event.key.toLowerCase();
    if (k === '.') {
      event.preventDefault();
      toggleSidebar();
    } else if (k === ',') {
      event.preventDefault();
      if (!app.settings.open) openSettings();
    }
  }
</script>

<svelte:window {onkeydown} />

{#if app.status === 'signedOut'}
  <SignIn />
{:else if app.status !== 'ready' || !ws()}
  <Loading />
{:else}
<div class="shell">
  <AppSidebar />
  <main class="main">
  {#key key}
    <div class="view">
      {#if route.parts[0] === 'agent'}
        <Agent id={route.parts[1]} />
      {:else if route.parts[0] === 'library'}
        <LibraryPage />
      {:else if route.parts[0] === 'view' && route.parts[2]}
        <RowPage view={route.parts[1]} id={route.parts[2]} />
      {:else if route.parts[0] === 'view' && route.parts[1]}
        <ViewPage id={route.parts[1]} />
      {:else if route.parts[0] === 'database' && route.parts[1]}
        <DatabaseTable id={route.parts[1]} />
      {:else if route.parts[0] === 'database'}
        <DatabasePage />
      {:else if route.parts[0] === 'workflows' && route.parts[1]}
        <Workflow id={route.parts[1]} />
      {:else if route.parts[0] === 'workflows'}
        <Workflows />
      {:else if route.parts[0] === 'access' && route.parts[1] === 'groups' && route.parts[2]}
        <GroupPage id={route.parts[2]} />
      {:else if route.parts[0] === 'access'}
        <AccessPage />
      {:else if route.parts[0] === 'onboarding' && onboards()}
        <Onboarding />
      {:else}
        {#key lastAgent()?.id}<Agent id={lastAgent()?.id} />{/key}
      {/if}
    </div>
  {/key}
  </main>
  {#if app.canvas}{#key app.canvas.id}<Canvas />{/key}{/if}
</div>

<AppSettings />
<PromptHost />
<HelpDialog bind:open={app.help} />
<ContextMenu />
{#if app.askPasskey}<PasskeyPrompt />{/if}
{/if}

<style>
  .shell {
    display: flex;
    height: 100%;
    min-height: 0;
  }

  .main {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    min-height: 0;
  }

  .view {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }
</style>
