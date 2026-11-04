<script>
  import { getContext } from 'svelte';
  import { Bell, BellOff, Moon, Settings, User, Plug, LifeBuoy, LogOut } from '@lucide/svelte';
  import Avatar from './Avatar.svelte';
  import IconButton from './IconButton.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import { theme, toggleTheme } from '../lib/theme.svelte.js';
  import { app, openSettings, signOut } from '../lib/app.svelte.js';

  let { user } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);

  function settings(section) {
    app.drawer = false;
    openSettings(section);
  }

  function help() {
    app.drawer = false;
    app.help = true;
  }
</script>

<Menu label={user.name} placement="top-start" morph>
  {#snippet trigger(props)}
    <div class={['card', { collapsed }]}>
      <button type="button" class="account" aria-label={collapsed ? user.name : undefined} {...props}>
        <Avatar name={user.name} seed={user.id} size="ml" />
        <span class="who">
          <span class="name">{user.name}</span>
          <span class="status">{user.status || user.plan}</span>
        </span>
      </button>
      <span class="actions">
        <IconButton icon={app.muted ? BellOff : Bell} label={app.muted ? 'Unmute notifications' : 'Mute notifications'} pressed={app.muted} onclick={() => (app.muted = !app.muted)} />
        <IconButton icon={Settings} label="Settings" shortcut="⌘ ," onclick={() => settings()} />
      </span>
    </div>
  {/snippet}
  <MenuItem icon={User} onselect={() => settings('profile')}>Profile</MenuItem>
  <MenuItem icon={Plug} onselect={() => settings('connectors')}>Connectors</MenuItem>
  <MenuItem icon={LifeBuoy} onselect={help}>Help</MenuItem>
  <MenuItem icon={Moon} toggle={theme.resolved === 'dark'} keepOpen onselect={toggleTheme}>Dark mode</MenuItem>
  <MenuSeparator />
  <MenuItem icon={LogOut} onselect={signOut}>Log out</MenuItem>
</Menu>

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

  .account:hover {
    background: var(--color-fill);
  }

  .who {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    line-height: var(--leading-snug);
    white-space: nowrap;
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

  .actions {
    display: flex;
    align-items: center;
    flex: none;
    gap: var(--space-0-5);
  }

  .collapsed .who,
  .collapsed .actions {
    display: none;
  }
</style>
