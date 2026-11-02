<script>
  import { RefreshCw, Unplug, Plug, Check } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Button from './Button.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import { connect, disconnect, syncConnector, connections } from '../lib/app.svelte.js';

  let { connector } = $props();

  connections.use();
</script>

<Stack gap="6">
  <Stack direction="row" gap="3" align="center" justify="between" wrap>
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={connector.id} size="lg" />
      <Stack gap="1">
        <Text weight="medium">{connector.category}</Text>
        {#if connector.connected}<StatusBadge status="connected" />{:else}<Badge>Not connected</Badge>{/if}
      </Stack>
    </Stack>
    <Stack direction="row" gap="2">
      {#if connector.connected}
        <Button variant="ghost" icon={Unplug} onclick={() => disconnect(connector.id)}>Disconnect</Button>
        {#if !connector.real}<Button icon={RefreshCw} onclick={() => syncConnector(connector.id)}>Sync now</Button>{/if}
      {:else}
        <Button variant="primary" icon={Plug} disabled={connector.real && !connector.ready} onclick={() => connect(connector.id)}>Connect</Button>
      {/if}
    </Stack>
  </Stack>
  {#if connector.real && !connector.ready}
    <Text as="p" size="md" tone="secondary">This server has no {connector.by} client set up for {connector.name} yet. Ask whoever runs it to set one up.</Text>
  {/if}
  {#if connector.connected}
    <Stack gap="0">
      <SettingsRow label="Account"><Text size="sm" tone="secondary">{connector.account}</Text></SettingsRow>
      {#if connector.real}
        {#if connector.error}<SettingsRow label="Problem"><Text size="sm" tone="danger">{connector.error}</Text></SettingsRow>{/if}
      {:else}
        <SettingsRow label="Last sync"><Text size="sm" tone="secondary">{connector.lastSync}</Text></SettingsRow>
        <SettingsRow label="Items imported"><Text size="sm" tone="secondary" tabular>{connector.items}</Text></SettingsRow>
      {/if}
    </Stack>
  {/if}
  <Section title="Archive can">
    <Stack gap="2">
      {#each connector.scopes as scope (scope)}
        <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
      {/each}
    </Stack>
  </Section>
</Stack>
