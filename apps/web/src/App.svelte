<script>
  import AppSidebar from './components/AppSidebar.svelte';
  import ContextMenu from './components/ContextMenu.svelte';
  import { route, replace } from './lib/router.svelte.js';
  import { app, ws, onboards, toggleSidebar, closeDrawer, openSettings, boot, account, lastAgent } from './lib/app.svelte.js';
  import { firstBoot } from './lib/api.js';
  import PromptHost from './components/PromptHost.svelte';
  import Loading from './components/Loading.svelte';
  import { pageOf } from './lib/preload.js';

  boot(firstBoot);

  const page = $derived(pageOf(route.parts, onboards()));
  const load = $derived(page.load);
  load();

  let settings = $state(false);
  let help = $state(false);

  $effect(() => {
    settings ||= app.settings.open;
    help ||= app.help;
  });

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
  {#await import('./components/SignIn.svelte') then m}<m.default />{/await}
{:else if app.status !== 'ready' || !ws()}
  <Loading />
{:else}
<div class="shell">
  <AppSidebar />
  <main class="main">
  {#key key}
    <div class="view">
      {#await load() then m}
        {#key page.home && lastAgent()?.id}<m.default {...page.props} id={page.home ? lastAgent()?.id : page.props.id} />{/key}
      {/await}
    </div>
  {/key}
  </main>
  {#if app.canvas}{#await import('./components/Canvas.svelte') then m}{#key app.canvas.id}<m.default />{/key}{/await}{/if}
</div>

{#if settings}{#await import('./components/AppSettings.svelte') then m}<m.default />{/await}{/if}
<PromptHost />
{#if help}{#await import('./components/HelpDialog.svelte') then m}<m.default bind:open={app.help} />{/await}{/if}
<ContextMenu />
{#if app.askPasskey}{#await import('./components/PasskeyPrompt.svelte') then m}<m.default />{/await}{/if}
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
