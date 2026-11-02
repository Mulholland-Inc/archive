<script>
  import { Plus, Sun, Moon, Monitor, Settings, Shapes, MessagesSquare, Library, Database, Lock, PanelLeft, FileText, Palette } from '@lucide/svelte';
  import Shell from './components/Shell.svelte';
  import AppSidebar from './components/AppSidebar.svelte';
  import View from './components/View.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import ContextMenu from './components/ContextMenu.svelte';
  import Home from './pages/Home.svelte';
  import Chat from './pages/Chat.svelte';
  import Chats from './pages/Chats.svelte';
  import LibraryPage from './pages/Library.svelte';
  import DatabasePage from './pages/Database.svelte';
  import Workflows from './pages/Workflows.svelte';
  import Workflow from './pages/Workflow.svelte';
  import DatabaseTable from './pages/DatabaseTable.svelte';
  import AccessPage from './pages/Access.svelte';
  import AppSettings from './components/AppSettings.svelte';
  import VaultSettingsDialog from './components/VaultSettingsDialog.svelte';
  import Specimen from './pages/Specimen.svelte';
  import Onboarding from './pages/Onboarding.svelte';
  import { route, go, replace } from './lib/router.svelte.js';
  import { app, toggleSidebar, closeDrawer } from './lib/app.svelte.js';
  import { setTheme, toggleTheme } from './lib/theme.svelte.js';
  import { ws } from './lib/app.svelte.js';
  import PromptHost from './components/PromptHost.svelte';
  import HelpDialog from './components/HelpDialog.svelte';
  import ShareDialog from './components/ShareDialog.svelte';
  import SignIn from './components/SignIn.svelte';
  import PasskeyPrompt from './components/PasskeyPrompt.svelte';
  import Spinner from './components/Spinner.svelte';
  import { Workflow as WorkflowIcon, Plug, User, Building2, ListChecks, Server, KeyRound, Blocks } from '@lucide/svelte';
  import { switchVault, openSettings, openVaultSettings, boot } from './lib/app.svelte.js';

  boot();

  let last = '/';

  $effect(() => {
    const [head, a, b] = route.parts;
    if (head === 'settings') {
      openSettings(a ?? 'profile', a === 'connectors' ? (b ?? null) : null);
      replace(last);
    } else if (head === 'vault') {
      openVaultSettings(a ?? 'general', a === 'integrations' ? (b ?? null) : null);
      replace(last);
    } else if (head === 'connectors') {
      openSettings('connectors', a ?? null);
      replace(last);
    } else {
      last = route.path;
    }
  });

  const key = $derived(['library', 'access'].includes(route.parts[0]) ? route.parts[0] : route.parts.slice(0, 2).join('/') || 'home');

  const commands = $derived(!ws() ? [] : [
    { id: 'new', group: 'Actions', label: 'New chat', icon: Plus, shortcut: '⌘ ⇧ O', run: () => go('/') },
    { id: 'theme', group: 'Actions', label: 'Toggle light and dark', icon: Palette, run: toggleTheme },
    { id: 'light', group: 'Actions', label: 'Use light theme', icon: Sun, run: () => setTheme('light'), searchOnly: true },
    { id: 'dark', group: 'Actions', label: 'Use dark theme', icon: Moon, run: () => setTheme('dark'), searchOnly: true },
    { id: 'system', group: 'Actions', label: 'Use system theme', icon: Monitor, run: () => setTheme('system'), searchOnly: true },
    { id: 'sidebar', group: 'Actions', label: 'Toggle sidebar', icon: PanelLeft, shortcut: '⌘ .', run: toggleSidebar },
    { id: 'p-chats', group: 'Go to', label: 'Chats', icon: MessagesSquare, run: () => go('/chats') },
    { id: 'p-library', group: 'Go to', label: 'Library', icon: Library, run: () => go('/library') },
    { id: 'p-database', group: 'Go to', label: 'Database', icon: Database, run: () => go('/database') },
    { id: 'p-workflows', group: 'Go to', label: 'Workflows', icon: WorkflowIcon, run: () => go('/workflows') },
    { id: 'p-connectors', group: 'Go to', label: 'Connectors', icon: Plug, run: () => openSettings('connectors') },
    { id: 'p-access', group: 'Go to', label: 'Access', icon: Lock, run: () => go('/access') },
    { id: 'p-onboarding', group: 'Go to', label: 'Onboarding', icon: ListChecks, run: () => go('/onboarding') },
    { id: 'p-vault', group: 'Go to', label: 'Vault settings', icon: Building2, run: () => openVaultSettings() },
    { id: 'p-server', group: 'Go to', label: 'Server', icon: Server, run: () => openVaultSettings('server'), searchOnly: true },
    { id: 'p-integrations', group: 'Go to', label: 'Integrations', icon: Blocks, run: () => openVaultSettings('integrations'), searchOnly: true },
    { id: 'p-keys', group: 'Go to', label: 'Vault API key', icon: KeyRound, run: () => openVaultSettings('api'), searchOnly: true },
    { id: 'p-my-key', group: 'Go to', label: 'Your API key', icon: KeyRound, run: () => openSettings('api'), searchOnly: true },
    { id: 'p-settings', group: 'Go to', label: 'Settings', icon: Settings, shortcut: '⌘ ,', run: () => openSettings() },
    { id: 'p-specimen', group: 'Go to', label: 'Component specimen', icon: Shapes, run: () => go('/specimen') },
    ...ws().chats.map((c, i) => ({ id: `c-${c.id}`, group: 'Chats', label: c.title, icon: MessagesSquare, meta: c.updated, run: () => go(`/chat/${c.id}`), searchOnly: i >= 6 })),
    ...ws().library.map((f) => ({ id: `f-${f.id}`, group: 'Files', label: f.name, icon: FileText, meta: f.folder, run: () => go(`/library/${f.id}`), searchOnly: true })),
    ...ws().schema.map((t) => ({ id: `t-${t.id}`, group: 'Tables', label: t.name, icon: Database, meta: t.rows === null ? '' : `${t.rows.toLocaleString('en-US')} rows`, run: () => go(`/database/${t.id}`), searchOnly: true })),
    ...ws().members.map((m) => ({ id: `u-${m.id}`, group: 'People', label: m.name, icon: User, meta: m.email, run: () => go(`/access/${m.id}`), searchOnly: true })),
    ...ws().workflows.map((w) => ({ id: `w-${w.id}`, group: 'Workflows', label: w.name, icon: WorkflowIcon, meta: w.trigger.label, run: () => go(`/workflows/${w.id}`), searchOnly: true })),
    ...app.vaults.filter((w) => w.id !== app.vault).map((w) => ({ id: `ws-${w.id}`, group: 'Vaults', label: `Switch to ${w.name}`, icon: Building2, run: () => switchVault(w.id), searchOnly: true }))
  ]);

  function onkeydown(event) {
    if (event.key === 'Escape' && app.drawer && !document.querySelector('dialog[open], :popover-open')) {
      closeDrawer();
      return;
    }
    const mod = event.metaKey || event.ctrlKey;
    if (!mod) return;
    const k = event.key.toLowerCase();
    if (k === 'k') {
      event.preventDefault();
      app.palette = !app.palette;
    } else if (k === '.') {
      event.preventDefault();
      toggleSidebar();
    } else if (k === ',') {
      event.preventDefault();
      if (!app.settings.open) openSettings();
    } else if (k === 'o' && event.shiftKey) {
      event.preventDefault();
      go('/');
    }
  }
</script>

<svelte:window {onkeydown} />

{#if app.status === 'signedOut'}
  <SignIn />
{:else if app.status !== 'ready' || !ws()}
  <div class="boot" aria-label="Loading"><Spinner /></div>
{:else}
<Shell>
  {#snippet sidebar()}<AppSidebar />{/snippet}
  {#key key}
    <View>
      {#if route.parts[0] === 'chat'}
        <Chat id={route.parts[1]} />
      {:else if route.parts[0] === 'chats'}
        <Chats />
      {:else if route.parts[0] === 'library'}
        <LibraryPage />
      {:else if route.parts[0] === 'database' && route.parts[1]}
        <DatabaseTable id={route.parts[1]} />
      {:else if route.parts[0] === 'database'}
        <DatabasePage />
      {:else if route.parts[0] === 'workflows' && route.parts[1]}
        <Workflow id={route.parts[1]} />
      {:else if route.parts[0] === 'workflows'}
        <Workflows />
      {:else if route.parts[0] === 'access'}
        <AccessPage />
      {:else if route.parts[0] === 'onboarding'}
        <Onboarding />
      {:else if route.parts[0] === 'specimen'}
        <Specimen />
      {:else}
        <Home />
      {/if}
    </View>
  {/key}
</Shell>

<CommandPalette bind:open={app.palette} {commands} />
<AppSettings />
<VaultSettingsDialog />
<PromptHost />
<HelpDialog bind:open={app.help} />
<ShareDialog />
<ContextMenu />
{#if app.askPasskey}<PasskeyPrompt />{/if}
{/if}

<style>
  .boot {
    display: grid;
    place-items: center;
    min-height: 100dvh;
    color: var(--color-text-tertiary);
  }
</style>
