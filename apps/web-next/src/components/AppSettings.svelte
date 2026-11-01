<script>
  import { User, Palette, Building2, Bell, Plug } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import Reveal from './Reveal.svelte';
  import ProfileSettings from './ProfileSettings.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import VaultSettings from './VaultSettings.svelte';
  import NotificationSettings from './NotificationSettings.svelte';
  import ConnectorSettings from './ConnectorSettings.svelte';
  import ConnectorDetail from './ConnectorDetail.svelte';
  import { app, ws } from '../lib/app.svelte.js';

  const items = [
    { id: 'profile', label: 'Profile', icon: User },
    { id: 'appearance', label: 'Appearance', icon: Palette },
    { id: 'vault', label: 'Vault', icon: Building2 },
    { id: 'notifications', label: 'Notifications', icon: Bell },
    { id: 'connectors', label: 'Connectors', icon: Plug }
  ];

  const s = app.settings;
  const current = $derived(s.section ?? 'profile');
  const connector = $derived(current === 'connectors' && s.sub ? ws().connectors.find((c) => c.id === s.sub) : null);
  const title = $derived(connector?.name ?? items.find((i) => i.id === current)?.label);
</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} {items} {title} onselect={() => (s.sub = null)} onback={connector ? () => (s.sub = null) : undefined}>
  {#key current + (connector?.id ?? '')}
    <Reveal>
      {#if connector}
        <ConnectorDetail {connector} />
      {:else if current === 'appearance'}
        <AppearanceSettings />
      {:else if current === 'vault'}
        <VaultSettings />
      {:else if current === 'notifications'}
        <NotificationSettings />
      {:else if current === 'connectors'}
        <ConnectorSettings onopen={(id) => (s.sub = id)} />
      {:else}
        <ProfileSettings />
      {/if}
    </Reveal>
  {/key}
</SettingsDialog>
