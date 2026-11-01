<script>
  import { Building2, Server, Blocks, KeyRound, Lock, Workflow } from '@lucide/svelte';
  import SettingsDialog from './SettingsDialog.svelte';
  import SettingsPane from './SettingsPane.svelte';
  import VaultSettings from './VaultSettings.svelte';
  import ServerSettings from './ServerSettings.svelte';
  import IntegrationSettings from './IntegrationSettings.svelte';
  import IntegrationDetail from './IntegrationDetail.svelte';
  import ApiKeySettings from './ApiKeySettings.svelte';
  import { app, ws, vaultIntegrations } from '../lib/app.svelte.js';

  const items = [
    { id: 'general', label: 'General', icon: Building2 },
    { id: 'server', label: 'Server', icon: Server },
    { id: 'integrations', label: 'Integrations', icon: Blocks },
    { id: 'api', label: 'API key', icon: KeyRound },
    { id: 'access', label: 'Access', icon: Lock, group: 'Manage', href: '#/access' },
    { id: 'workflows', label: 'Workflows', icon: Workflow, group: 'Manage', href: '#/workflows' }
  ];

  const s = app.vaultSettings;
  const current = $derived(s.section ?? 'general');
  const integration = $derived(current === 'integrations' && s.sub ? vaultIntegrations().find((c) => c.id === s.sub) : null);
  const title = $derived(items.find((i) => i.id === current)?.label);
  const trail = $derived(integration ? [{ label: title, onclick: () => (s.sub = null) }, { label: integration.name }] : [{ label: title }]);
  const back = () => (s.section = null);

  function select(id) {
    s.sub = null;
    if (items.find((i) => i.id === id)?.href) s.open = false;
  }
</script>

<SettingsDialog bind:open={s.open} bind:section={s.section} root={ws().name} {items} {trail} onselect={select}>
  {#if integration}
    <SettingsPane onback={() => (s.sub = null)}><IntegrationDetail {integration} /></SettingsPane>
  {:else if current === 'integrations'}
    <IntegrationSettings onopen={(id) => (s.sub = id)} onback={back} />
  {:else if current === 'api'}
    <ApiKeySettings scope="vault" onback={back} />
  {:else}
    <SettingsPane onback={back} backNarrow>
      {#if current === 'server'}
        <ServerSettings />
      {:else}
        <VaultSettings />
      {/if}
    </SettingsPane>
  {/if}
</SettingsDialog>
