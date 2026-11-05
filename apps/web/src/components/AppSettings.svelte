<script>
  import { Palette, Plug, Blocks, KeyRound, Building2, Server } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import ArchiveSettings from './ArchiveSettings.svelte';
  import ServerSettings from './ServerSettings.svelte';
  import ApiKeySettings from './ApiKeySettings.svelte';
  import ConnectorSettings from './ConnectorSettings.svelte';
  import ConnectorDetail from './ConnectorDetail.svelte';
  import IntegrationSettings from './IntegrationSettings.svelte';
  import IntegrationDetail from './IntegrationDetail.svelte';
  import { app, ws, atLeast, archiveIntegrations } from '../lib/app.svelte.js';

  const items = $derived([
    { id: 'appearance', label: 'Appearance', icon: Palette, group: 'You' },
    { id: 'connectors', label: 'Connectors', icon: Plug, group: 'You' },
    { id: 'api', label: 'API key', icon: KeyRound, group: 'You' },
    { id: 'general', label: 'General', icon: Building2, group: 'Archive' },
    { id: 'server', label: 'Server', icon: Server, group: 'Archive' },
    { id: 'integrations', label: 'Integrations', icon: Blocks, group: 'Archive' },
    ...(atLeast('engineer') ? [{ id: 'archive-api', label: 'API key', icon: KeyRound, group: 'Archive' }] : [])
  ]);

  const s = app.settings;
  const current = $derived(items.some((i) => i.id === s.section) ? s.section : 'appearance');
  const title = $derived(items.find((i) => i.id === current)?.label);
  const connector = $derived(current === 'connectors' && s.sub ? ws().connectors.find((c) => c.id === s.sub) : null);
  const integration = $derived(current === 'integrations' && s.sub ? archiveIntegrations().find((c) => c.id === s.sub) : null);
  const name = $derived((connector ?? integration)?.name);
  const trail = $derived(name ? [{ label: title, onclick: () => (s.sub = null) }, { label: name }] : [{ label: title }]);
  const back = () => (s.section = null);
</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} {items} {trail} onselect={() => (s.sub = null)}>
  {#if connector}
    <ConnectorDetail {connector} onback={() => (s.sub = null)} />
  {:else if integration}
    <IntegrationDetail {integration} onback={() => (s.sub = null)} />
  {:else if current === 'connectors'}
    <ConnectorSettings onopen={(id) => (s.sub = id)} onback={back} backNarrow />
  {:else if current === 'integrations'}
    <IntegrationSettings onopen={(id) => (s.sub = id)} onback={back} />
  {:else if current === 'api'}
    <ApiKeySettings personal onback={back} />
  {:else if current === 'archive-api'}
    <ApiKeySettings onback={back} />
  {:else}
    <SettingsPane onback={back} backNarrow>
      {#if current === 'general'}
        <ArchiveSettings />
      {:else if current === 'server'}
        <ServerSettings />
      {:else}
        <AppearanceSettings />
      {/if}
    </SettingsPane>
  {/if}
</SettingsDialog>
