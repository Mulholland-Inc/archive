<script>
  import { Plus, Sun, Moon, Monitor, Settings, Shapes, MessagesSquare, Library, FolderClosed, ListChecks, Users, PanelLeft, FileText, Palette } from '@lucide/svelte';
  import Shell from './components/Shell.svelte';
  import AppSidebar from './components/AppSidebar.svelte';
  import View from './components/View.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Toaster from './components/Toaster.svelte';
  import Home from './pages/Home.svelte';
  import Chat from './pages/Chat.svelte';
  import Chats from './pages/Chats.svelte';
  import LibraryPage from './pages/Library.svelte';
  import Projects from './pages/Projects.svelte';
  import Project from './pages/Project.svelte';
  import Tasks from './pages/Tasks.svelte';
  import Task from './pages/Task.svelte';
  import UsersPage from './pages/Users.svelte';
  import SettingsPage from './pages/Settings.svelte';
  import Specimen from './pages/Specimen.svelte';
  import { route, go } from './lib/router.svelte.js';
  import { app, toggleSidebar } from './lib/app.svelte.js';
  import { setTheme, toggleTheme } from './lib/theme.svelte.js';
  import { library, projects } from './lib/data.js';

  const key = $derived(route.parts[0] === 'settings' ? 'settings' : route.parts.slice(0, 2).join('/') || 'home');

  const commands = $derived([
    { id: 'new', group: 'Actions', label: 'New chat', icon: Plus, shortcut: '⌘ ⇧ O', run: () => go('/') },
    { id: 'theme', group: 'Actions', label: 'Toggle light and dark', icon: Palette, run: toggleTheme },
    { id: 'light', group: 'Actions', label: 'Use light theme', icon: Sun, run: () => setTheme('light'), searchOnly: true },
    { id: 'dark', group: 'Actions', label: 'Use dark theme', icon: Moon, run: () => setTheme('dark'), searchOnly: true },
    { id: 'system', group: 'Actions', label: 'Use system theme', icon: Monitor, run: () => setTheme('system'), searchOnly: true },
    { id: 'sidebar', group: 'Actions', label: 'Toggle sidebar', icon: PanelLeft, shortcut: '⌘ .', run: toggleSidebar },
    { id: 'p-chats', group: 'Go to', label: 'Chats', icon: MessagesSquare, run: () => go('/chats') },
    { id: 'p-library', group: 'Go to', label: 'Library', icon: Library, run: () => go('/library') },
    { id: 'p-projects', group: 'Go to', label: 'Projects', icon: FolderClosed, run: () => go('/projects') },
    { id: 'p-tasks', group: 'Go to', label: 'Tasks', icon: ListChecks, run: () => go('/tasks') },
    { id: 'p-users', group: 'Go to', label: 'Users', icon: Users, run: () => go('/users') },
    { id: 'p-settings', group: 'Go to', label: 'Settings', icon: Settings, shortcut: '⌘ ,', run: () => go('/settings/profile') },
    { id: 'p-specimen', group: 'Go to', label: 'Component specimen', icon: Shapes, run: () => go('/specimen') },
    ...app.chats.slice(0, 8).map((c) => ({ id: `c-${c.id}`, group: 'Recent chats', label: c.title, icon: MessagesSquare, meta: c.updated, run: () => go(`/chat/${c.id}`) })),
    ...app.chats.slice(8).map((c) => ({ id: `c-${c.id}`, group: 'Recent chats', label: c.title, icon: MessagesSquare, meta: c.updated, run: () => go(`/chat/${c.id}`), searchOnly: true })),
    ...projects.map((p) => ({ id: `pr-${p.id}`, group: 'Projects', label: p.name, icon: FolderClosed, run: () => go(`/projects/${p.id}`), searchOnly: true })),
    ...library.map((f) => ({ id: `f-${f.id}`, group: 'Files', label: f.name, icon: FileText, meta: f.folder, run: () => go('/library'), searchOnly: true }))
  ]);

  function onkeydown(event) {
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

<svelte:window {onkeydown} onarchivepalette={() => (app.palette = true)} />

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
      {:else if route.parts[0] === 'projects' && route.parts[1]}
        <Project id={route.parts[1]} />
      {:else if route.parts[0] === 'projects'}
        <Projects />
      {:else if route.parts[0] === 'tasks' && route.parts[1]}
        <Task id={route.parts[1]} />
      {:else if route.parts[0] === 'tasks'}
        <Tasks />
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
<Toaster />
