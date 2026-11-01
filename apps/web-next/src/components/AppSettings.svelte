<script>
  import { User, Palette, Bell, Building2, Lock, Workflow, Plug, KeyRound, Server } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import ProfileSettings from './ProfileSettings.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import VaultSettings from './VaultSettings.svelte';
  import NotificationSettings from './NotificationSettings.svelte';
  import ServerSettings from './ServerSettings.svelte';
  import ConnectorSettings from './ConnectorSettings.svelte';
  import ConnectorDetail from './ConnectorDetail.svelte';
  import { app, ws } from '../lib/app.svelte.js';

  const items = [
    { id: 'profile', label: 'Profile', icon: User },
    { id: 'appearance', label: 'Appearance', icon: Palette },
    { id: 'notifications', label: 'Notifications', icon: Bell },
    { id: 'vault', label: 'Vault', icon: Building2, group: 'Vault' },
    { id: 'access', label: 'Access', icon: Lock, group: 'Vault', href: '#/access' },
    { id: 'workflows', label: 'Workflows', icon: Workflow, group: 'Vault', href: '#/workflows' },
    { id: 'connectors', label: 'Connectors', icon: Plug, group: 'Customize' },
    { id: 'server', label: 'Server', icon: Server, group: 'Platform' },
    { id: 'api', label: 'API keys', icon: KeyRound, group: 'Platform', href: 'https://archive.dev/docs/api', external: true }
  ];

  const s = app.settings;
  const current = $derived(s.section ?? 'profile');
  const connector = $derived(current === 'connectors' && s.sub ? ws().connectors.find((c) => c.id === s.sub) : null);
  const title = $derived(items.find((i) => i.id === current)?.label);
  const trail = $derived(connector ? [{ label: title, onclick: () => (s.sub = null) }, { label: connector.name }] : [{ label: title }]);
  const back = () => (s.section = null);

  function select(id) {
    s.sub = null;
    const item = items.find((i) => i.id === id);
    if (item.href && !item.external) s.open = false;
  }
</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} {items} {trail} onselect={select}>
  {#if connector}
    <SettingsPane onback={() => (s.sub = null)}><ConnectorDetail {connector} /></SettingsPane>
  {:else if current === 'connectors'}
    <ConnectorSettings onopen={(id) => (s.sub = id)} onback={back} backNarrow />
  {:else}
    <SettingsPane onback={back} backNarrow>
      {#if current === 'appearance'}
        <AppearanceSettings />
      {:else if current === 'vault'}
        <VaultSettings />
      {:else if current === 'server'}
        <ServerSettings />
      {:else if current === 'notifications'}
        <NotificationSettings />
      {:else}
        <ProfileSettings />
      {/if}
    </SettingsPane>
  {/if}
</SettingsDialog>
