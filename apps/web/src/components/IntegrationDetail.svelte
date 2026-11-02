<script>
  import { Unplug, Plug, Check, RefreshCw } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import OwnerNotice from './OwnerNotice.svelte';
  import Button from './Button.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Select from './Select.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import { connectIntegration, disconnectIntegration, syncIntegration, narrowIntegration, archiveGroups, isArchiveOwner, connections } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { integration: c } = $props();
  const owner = $derived(isArchiveOwner());
  connections.use();

  // Who sees its records: everyone, or one group; an owner picks, everyone else reads it.
  const everyone = 'everyone';
  const options = $derived([{ value: everyone, label: 'Everyone in the archive' }, ...[...new Set([...archiveGroups(), ...(c.access ? [c.access] : [])])].map((g) => ({ value: g, label: `The ${g} group` }))]);
  let access = $state(everyone);
  $effect(() => {
    access = c.access || everyone;
  });

  async function disconnect() {
    if (await confirmDialog({ title: `Disconnect ${c.name}?`, body: `This turns it off for everyone in the archive and deletes what Archive synced from it. Workflows that rely on it will fail.`, confirm: 'Disconnect' })) disconnectIntegration(c.id);
  }
</script>

<Stack gap="6">
  <Stack direction="row" gap="3" align="center" justify="between" wrap>
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={c.id} size="lg" />
      <Stack gap="1">
        <Text weight="medium">{c.category}</Text>
        {#if c.connected}<StatusBadge status="connected" />{:else}<Badge>Not connected</Badge>{/if}
      </Stack>
    </Stack>
    {#if c.connected}
      <Stack direction="row" gap="2">
        <Button variant="ghost" icon={Unplug} disabled={!owner} onclick={disconnect}>Disconnect</Button>
        <Button icon={RefreshCw} disabled={!owner} onclick={() => syncIntegration(c.id)}>Sync now</Button>
      </Stack>
    {:else}
      <Button variant="primary" icon={Plug} disabled={!owner || !c.ready} onclick={() => connectIntegration(c.id)}>Connect</Button>
    {/if}
  </Stack>
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
      <SettingsRow label="Records"><Text size="sm" tone="secondary" tabular>{c.items.toLocaleString()}</Text></SettingsRow>
      {#if c.error}<SettingsRow label="Problem"><Text size="sm" tone="danger">{c.error}</Text></SettingsRow>{/if}
      <SettingsRow label="Who sees it">
        {#if owner}
          <Select bind:value={access} {options} size="sm" label="Who sees it" placement="bottom-end" onchange={(v) => narrowIntegration(c.id, v === everyone ? '' : v)} />
        {:else}
          <Text size="sm" tone="secondary">{c.access ? `The ${c.access} group` : 'Everyone in the archive'}</Text>
        {/if}
      </SettingsRow>
    </Stack>
  {/if}
  <Section title="Archive can">
    <Stack gap="2">
      {#each c.scopes as scope (scope)}
        <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
      {/each}
    </Stack>
  </Section>
  <Section title="Who it covers">
    <Text as="p" size="md" tone="secondary">It is the archive’s, not one person’s: {c.access ? `members of the ${c.access} group and the owners` : 'everyone in the archive'} read what it syncs, and editors among them act with it. Only owners connect, narrow and disconnect it. Each person’s own Gmail, Outlook, Drive and Notion stay theirs, in their own settings.</Text>
  </Section>
</Stack>
