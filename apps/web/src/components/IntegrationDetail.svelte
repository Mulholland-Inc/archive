<script>
  import { Unplug, Plug, Check, RefreshCw } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import OwnerNotice from './OwnerNotice.svelte';
  import Button from './Button.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import KeysDialog from './KeysDialog.svelte';
  import { connectIntegration, connectLink, disconnectIntegration, syncIntegration, isArchiveOwner, connections } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { integration: c, onback } = $props();
  const owner = $derived(isArchiveOwner());
  let keying = $state(false);
  connections.use();

  async function disconnect() {
    if (await confirmDialog({ title: `Disconnect ${c.name}?`, body: `This turns it off for everyone in the archive and deletes what Archive synced from it. Workflows that rely on it will fail.`, confirm: 'Disconnect' })) disconnectIntegration(c.id);
  }
</script>

<SettingsPane {onback}>
  {#snippet lead()}
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={c.id} size="lg" />
      <Text weight="medium">{c.name}</Text>
      {#if c.connected}<StatusBadge status="connected" />{:else}<Badge>Not connected</Badge>{/if}
    </Stack>
  {/snippet}
  {#snippet tools()}
    {#if c.connected}
      <Button variant="ghost" icon={Unplug} disabled={!owner} onclick={disconnect}>Disconnect</Button>
      <Button icon={RefreshCw} disabled={!owner} onclick={() => syncIntegration(c.id)}>Sync now</Button>
    {:else}
      <Button variant="primary" icon={Plug} disabled={!owner || !c.ready} onclick={() => (c.link ? connectLink(c.id) : c.keys?.length || c.ask ? (keying = true) : connectIntegration(c.id))}>Connect</Button>
    {/if}
  {/snippet}
  <Stack gap="6">
    {#if !owner}<OwnerNotice what="integrations" />{/if}
    {#if c.ready === false}
      <Text as="p" size="md" tone="secondary">This server has no {c.by} client set up for {c.name} yet. Ask whoever runs it to set one up.</Text>
    {/if}
    <Text as="p" size="md" tone="secondary">{c.description}.</Text>
    {#if c.connected}
      <Stack gap="0">
        <SettingsRow label="Account"><Text size="sm" tone="secondary" truncate>{c.account}</Text></SettingsRow>
        <SettingsRow label="Connected by"><Text size="sm" tone="secondary">{c.connectedBy}, {c.date}</Text></SettingsRow>
        <SettingsRow label="Last synced"><Text size="sm" tone="secondary">{c.lastSync}</Text></SettingsRow>
        <SettingsRow label="Items synced"><Text size="sm" tone="secondary" tabular>{c.items.toLocaleString()}</Text></SettingsRow>
        {#if c.error}<SettingsRow label="Problem"><Text size="sm" tone="danger">{c.error}</Text></SettingsRow>{/if}
      </Stack>
    {/if}
    <Section title="Archive can">
      <Stack gap="2">
        {#each c.scopes as scope (scope)}
          <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
        {/each}
      </Stack>
    </Section>
  </Stack>
</SettingsPane>
<KeysDialog bind:open={keying} connector={c} />
