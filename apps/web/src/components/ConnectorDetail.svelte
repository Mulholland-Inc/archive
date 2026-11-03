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
  import { connect, disconnect, syncNow, connections } from '../lib/app.svelte.js';

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
        <Button icon={RefreshCw} onclick={() => syncNow(connector.id)}>Sync now</Button>
      {:else}
        <Button variant="primary" icon={Plug} disabled={!connector.ready} onclick={() => connect(connector.id)}>Connect</Button>
      {/if}
    </Stack>
  </Stack>
  {#if connector.ready === false}
    <Text as="p" size="md" tone="secondary">This server has no {connector.by} client set up for {connector.name} yet. Ask whoever runs it to set one up.</Text>
  {/if}
  {#if connector.connected}
    <Stack gap="0">
      <SettingsRow label="Account"><Text size="sm" tone="secondary">{connector.account}</Text></SettingsRow>
      <SettingsRow label="Last synced"><Text size="sm" tone="secondary">{connector.lastSync}</Text></SettingsRow>
      <SettingsRow label="Items synced"><Text size="sm" tone="secondary" tabular>{(connector.items ?? 0).toLocaleString()}</Text></SettingsRow>
      {#if connector.error}<SettingsRow label="Problem"><Text size="sm" tone="danger">{connector.error}</Text></SettingsRow>{/if}
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
