<script>
  import { Plus, Sun, Moon, Monitor, Settings, Shapes, MessagesSquare, Library, Users, PanelLeft, FileText, Palette } from '@lucide/svelte';
  import Shell from './components/Shell.svelte';
  import AppSidebar from './components/AppSidebar.svelte';
  import View from './components/View.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Toaster from './components/Toaster.svelte';
  import Home from './pages/Home.svelte';
  import Chat from './pages/Chat.svelte';
  import Chats from './pages/Chats.svelte';
  import LibraryPage from './pages/Library.svelte';
  import Workflows from './pages/Workflows.svelte';
  import Workflow from './pages/Workflow.svelte';
  import Connectors from './pages/Connectors.svelte';
  import UsersPage from './pages/Users.svelte';
  import SettingsPage from './pages/Settings.svelte';
  import Specimen from './pages/Specimen.svelte';
  import { route, go } from './lib/router.svelte.js';
  import { app, toggleSidebar, closeDrawer } from './lib/app.svelte.js';
  import { setTheme, toggleTheme } from './lib/theme.svelte.js';
  import { ws } from './lib/app.svelte.js';
  import PromptHost from './components/PromptHost.svelte';
  import HelpDialog from './components/HelpDialog.svelte';
  import SignedOut from './components/SignedOut.svelte';
  import { Workflow as WorkflowIcon, Plug, User, Building2 } from '@lucide/svelte';
  import { switchVault } from './lib/app.svelte.js';

  const key = $derived(['settings', 'library', 'users'].includes(route.parts[0]) ? route.parts[0] : route.parts.slice(0, 2).join('/') || 'home');

  const commands = $derived([
    { id: 'new', group: 'Actions', label: 'New chat', icon: Plus, shortcut: '⌘ ⇧ O', run: () => go('/') },
    { id: 'theme', group: 'Actions', label: 'Toggle light and dark', icon: Palette, run: toggleTheme },
    { id: 'light', group: 'Actions', label: 'Use light theme', icon: Sun, run: () => setTheme('light'), searchOnly: true },
    { id: 'dark', group: 'Actions', label: 'Use dark theme', icon: Moon, run: () => setTheme('dark'), searchOnly: true },
    { id: 'system', group: 'Actions', label: 'Use system theme', icon: Monitor, run: () => setTheme('system'), searchOnly: true },
    { id: 'sidebar', group: 'Actions', label: 'Toggle sidebar', icon: PanelLeft, shortcut: '⌘ .', run: toggleSidebar },
    { id: 'p-chats', group: 'Go to', label: 'Chats', icon: MessagesSquare, run: () => go('/chats') },
    { id: 'p-library', group: 'Go to', label: 'Library', icon: Library, run: () => go('/library') },
    { id: 'p-workflows', group: 'Go to', label: 'Workflows', icon: WorkflowIcon, run: () => go('/workflows') },
    { id: 'p-connectors', group: 'Go to', label: 'Connectors', icon: Plug, run: () => go('/connectors') },
    { id: 'p-users', group: 'Go to', label: 'Users', icon: Users, run: () => go('/users') },
    { id: 'p-settings', group: 'Go to', label: 'Settings', icon: Settings, shortcut: '⌘ ,', run: () => go('/settings/profile') },
    { id: 'p-specimen', group: 'Go to', label: 'Component specimen', icon: Shapes, run: () => go('/specimen') },
    ...ws().chats.filter((c) => !c.hidden).map((c, i) => ({ id: `c-${c.id}`, group: 'Chats', label: c.title, icon: MessagesSquare, meta: c.updated, run: () => go(`/chat/${c.id}`), searchOnly: i >= 6 })),
    ...ws().library.map((f) => ({ id: `f-${f.id}`, group: 'Files', label: f.name, icon: FileText, meta: f.folder, run: () => go(`/library/${f.id}`), searchOnly: true })),
    ...ws().members.map((m) => ({ id: `u-${m.id}`, group: 'People', label: m.name, icon: User, meta: m.email, run: () => go(`/users/${m.id}`), searchOnly: true })),
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
      go('/settings/profile');
    } else if (k === 'o' && event.shiftKey) {
      event.preventDefault();
      go('/');
    }
  }
</script>

<svelte:window {onkeydown} />

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
      {:else if route.parts[0] === 'workflows' && route.parts[1]}
        <Workflow id={route.parts[1]} />
      {:else if route.parts[0] === 'workflows'}
        <Workflows />
      {:else if route.parts[0] === 'connectors'}
        <Connectors />
      {:else if route.parts[0] === 'users'}
        <UsersPage />
      {:else if route.parts[0] === 'settings'}
        <SettingsPage section={route.parts[1] ?? 'profile'} />
      {:else if route.parts[0] === 'specimen'}
        <Specimen />
      {:else}
        <Home />
      {/if}
    </View>
  {/key}
</Shell>

<CommandPalette bind:open={app.palette} {commands} />
<PromptHost />
<HelpDialog bind:open={app.help} />
{#if app.signedOut}<SignedOut />{/if}
<Toaster />
