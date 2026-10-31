<script>
  import { getContext } from 'svelte';
  import { ChevronsUpDown, Settings, Palette, Sun, Moon, Monitor, LifeBuoy, LogOut, Shapes, Keyboard } from '@lucide/svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuLabel from './MenuLabel.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import Avatar from './Avatar.svelte';
  import Icon from './Icon.svelte';
  import { theme, setTheme } from '../lib/theme.svelte.js';
  import { app } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let { user } = $props();
  const sidebar = getContext('sidebar');
  const collapsed = $derived(sidebar?.collapsed ?? false);
</script>

<Menu placement="top-start" match label="Account">
  {#snippet trigger(props)}
    <button type="button" class={['account', { collapsed }]} aria-label="Account" {...props}>
      <Avatar name={user.name} tone={user.tone} size="md" />
      <span class="who">
        <span class="name">{user.name}</span>
        <span class="plan">{user.plan}</span>
      </span>
      <span class="chev"><Icon icon={ChevronsUpDown} size="sm" /></span>
    </button>
  {/snippet}
  <MenuLabel>{user.email}</MenuLabel>
  <MenuItem icon={Settings} shortcut="⌘ ," href="/settings/profile">Settings</MenuItem>
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

<style>
  .account {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    padding: var(--space-1);
    border-radius: var(--radius-md);
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

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .plan {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
  }

  .chev {
    display: inline-flex;
    color: var(--color-text-tertiary);
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .collapsed .who,
  .collapsed .chev {
    opacity: 0;
  }
</style>
