<script>
  import { User, Palette, Bell, KeyRound, Building2, Server, LifeBuoy, LogOut } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import ProfileSettings from './ProfileSettings.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import NotificationSettings from './NotificationSettings.svelte';
  import ArchiveSettings from './ArchiveSettings.svelte';
  import ServerSettings from './ServerSettings.svelte';
  import ApiKeySettings from './ApiKeySettings.svelte';
  import { app, signOut } from '../lib/app.svelte.js';

  const items = [
    { id: 'profile', label: 'Profile', icon: User, group: 'You' },
    { id: 'appearance', label: 'Appearance', icon: Palette, group: 'You' },
    { id: 'notifications', label: 'Notifications', icon: Bell, group: 'You' },
    { id: 'api', label: 'API key', icon: KeyRound, group: 'You' },
    { id: 'general', label: 'General', icon: Building2, group: 'Archive' },
    { id: 'server', label: 'Server', icon: Server, group: 'Archive' },
    { id: 'archive-api', label: 'API key', icon: KeyRound, group: 'Archive' },
    { id: 'help', label: 'Help', icon: LifeBuoy, run: () => { s.open = false; app.help = true; } },
    { id: 'logout', label: 'Log out', icon: LogOut, run: signOut }
  ];

  const s = app.settings;
  const current = $derived(s.section ?? 'profile');
  const title = $derived(items.find((i) => i.id === current)?.label);
  const back = () => (s.section = null);
</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} {items} trail={[{ label: title }]}>
  {#if current === 'api'}
    <ApiKeySettings onback={back} />
  {:else if current === 'archive-api'}
    <ApiKeySettings scope="archive" onback={back} />
  {:else}
    <SettingsPane onback={back} backNarrow>
      {#if current === 'appearance'}
        <AppearanceSettings />
      {:else if current === 'notifications'}
        <NotificationSettings />
      {:else if current === 'general'}
        <ArchiveSettings />
      {:else if current === 'server'}
        <ServerSettings />
      {:else}
        <ProfileSettings />
      {/if}
    </SettingsPane>
  {/if}
</SettingsDialog>
