<script>
  import { getContext } from 'svelte';
  import { Bell, BellOff, Settings, Palette, Sun, Moon, Monitor, LifeBuoy, LogOut, Shapes, Keyboard } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import Avatar from './Avatar.svelte';
  import IconButton from './IconButton.svelte';
  import { theme, setTheme, toggleTheme } from '../lib/theme.svelte.js';
  import { app, openSettings } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let { user } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<div class={['card', { collapsed }]}>
<Menu placement="top-start" label="Account">
  {#snippet trigger(props)}
    <button type="button" class="account" aria-label="Account" {...props}>
      <Avatar name={user.name} seed={user.id} size="ml" />
      <span class="who">
        <span class="name">{user.name}</span>
        <span class="status">{user.status || user.plan}</span>
      </span>
    </button>
  {/snippet}
  <MenuLabel>{user.email}</MenuLabel>
  <MenuItem icon={Settings} shortcut="⌘ ," onselect={() => openSettings()}>Settings</MenuItem>
  <SubMenu icon={Palette} label="Appearance">
    <MenuItem icon={Sun} checked={theme.choice === 'light'} onselect={() => setTheme('light')}>Light</MenuItem>
    <MenuItem icon={Moon} checked={theme.choice === 'dark'} onselect={() => setTheme('dark')}>Dark</MenuItem>
    <MenuItem icon={Monitor} checked={theme.choice === 'system'} onselect={() => setTheme('system')}>System</MenuItem>
  </SubMenu>
  <MenuItem icon={Shapes} href="/specimen">Component specimen</MenuItem>
  <MenuItem icon={Keyboard} shortcut="⌘ K" onselect={() => { app.drawer = false; app.palette = true; }}>Command palette</MenuItem>
  <MenuItem icon={LifeBuoy} onselect={() => { app.drawer = false; app.help = true; }}>Keyboard shortcuts and help</MenuItem>
  <MenuSeparator />
  <MenuItem icon={LogOut} onselect={() => { app.signedOut = true; go('/'); }}>Log out</MenuItem>
</Menu>
  <span class="actions">
    <span class="extra">
      <IconButton icon={app.muted ? BellOff : Bell} label={app.muted ? 'Unmute notifications' : 'Mute notifications'} pressed={app.muted} onclick={() => (app.muted = !app.muted)} />
      <IconButton icon={theme.resolved === 'dark' ? Sun : Moon} label={theme.resolved === 'dark' ? 'Light mode' : 'Dark mode'} onclick={toggleTheme} />
    </span>
    <IconButton icon={Settings} label="Settings" shortcut="⌘ ," onclick={() => openSettings()} />
  </span>
</div>

<style>
  .card {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    height: var(--dock-height);
    padding-inline: var(--space-1-5);
    border-radius: var(--radius-md);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    transition: background-color var(--duration-normal) var(--ease-out);
  }

  .account {
    display: flex;
    align-items: center;
    flex: 1;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-1);
    border-radius: var(--radius-sm);
    text-align: start;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .account:hover,
  .account[aria-expanded='true'] {
    background: var(--color-fill);
  }

  .who {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    line-height: var(--leading-snug);
    white-space: nowrap;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .name,
  .status {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .name {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .status {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .actions,
  .extra {
    display: flex;
    align-items: center;
    flex: none;
    gap: var(--space-0-5);
  }

  .collapsed .who,
  .collapsed .extra {
    display: none;
  }
</style>
