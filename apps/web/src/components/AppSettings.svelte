<script>
  import { User, Palette, Bell, Plug, KeyRound } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import ProfileSettings from './ProfileSettings.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import NotificationSettings from './NotificationSettings.svelte';
  import ConnectorSettings from './ConnectorSettings.svelte';
  import ConnectorDetail from './ConnectorDetail.svelte';
  import ApiKeySettings from './ApiKeySettings.svelte';
  import { app, ws } from '../lib/app.svelte.js';

  const items = [
    { id: 'profile', label: 'Profile', icon: User },
    { id: 'appearance', label: 'Appearance', icon: Palette },
    { id: 'notifications', label: 'Notifications', icon: Bell },
    { id: 'connectors', label: 'Connectors', icon: Plug },
    { id: 'api', label: 'API key', icon: KeyRound }
  ];

  const s = app.settings;
  const current = $derived(s.section ?? 'profile');
  const connector = $derived(current === 'connectors' && s.sub ? ws().connectors.find((c) => c.id === s.sub) : null);
  const title = $derived(items.find((i) => i.id === current)?.label);
  const trail = $derived(connector ? [{ label: title, onclick: () => (s.sub = null) }, { label: connector.name }] : [{ label: title }]);
  const back = () => (s.section = null);

</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} {items} {trail} onselect={() => (s.sub = null)}>
  {#if connector}
    <SettingsPane onback={() => (s.sub = null)}><ConnectorDetail {connector} /></SettingsPane>
  {:else if current === 'connectors'}
    <ConnectorSettings onopen={(id) => (s.sub = id)} onback={back} backNarrow />
  {:else if current === 'api'}
    <ApiKeySettings onback={back} />
  {:else}
    <SettingsPane onback={back} backNarrow>
      {#if current === 'appearance'}
        <AppearanceSettings />
      {:else if current === 'notifications'}
        <NotificationSettings />
      {:else}
        <ProfileSettings />
      {/if}
    </SettingsPane>
  {/if}
</SettingsDialog>
