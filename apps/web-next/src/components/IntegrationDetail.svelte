<script>
  import { Unplug, Plug, Check } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import OwnerNotice from './OwnerNotice.svelte';
  import AvatarGroup from './AvatarGroup.svelte';
  import Button from './Button.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import { connectIntegration, disconnectIntegration, isVaultOwner } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { integration: c } = $props();
  const owner = $derived(isVaultOwner());
  const reach = $derived(c.covers === 'domain' ? 'Everyone in the vault with an address on the connected domain.' : 'Everyone in the vault, including people who join later.');

  async function disconnect() {
    if (await confirmDialog({ title: `Disconnect ${c.name}?`, body: `This turns it off for everyone in the vault. Workflows that rely on it will pause.`, confirm: 'Disconnect' })) disconnectIntegration(c.id);
  }
</script>

<Stack gap="6">
  <Stack direction="row" gap="3" align="center" justify="between" wrap>
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={c.brand} size="lg" />
      <Stack gap="1">
        <Text weight="medium">{c.category}</Text>
        {#if c.connected}<StatusBadge status="connected" />{:else}<Badge>Not connected</Badge>{/if}
      </Stack>
    </Stack>
    {#if c.connected}
      <Button variant="ghost" icon={Unplug} disabled={!owner} onclick={disconnect}>Disconnect</Button>
    {:else}
      <Button variant="primary" icon={Plug} disabled={!owner} onclick={() => connectIntegration(c.id)}>Connect for everyone</Button>
    {/if}
  </Stack>
  {#if !owner}<OwnerNotice what="integrations" />{/if}
  <Text as="p" size="md" tone="secondary">{c.description}.</Text>
  {#if c.connected}
    <Stack gap="0">
      <SettingsRow label={c.covers === 'domain' ? 'Domain' : 'Account'}><Text size="sm" tone="secondary" truncate>{c.account}</Text></SettingsRow>
      <SettingsRow label="Connected by"><Text size="sm" tone="secondary">{c.connectedBy}, {c.date}</Text></SettingsRow>
      <SettingsRow label="Covers">
        <Stack direction="row" gap="2" align="center">
          <AvatarGroup people={c.members} />
          <Text size="sm" tone="secondary" tabular>{c.members.length} {c.members.length === 1 ? 'member' : 'members'}</Text>
        </Stack>
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
    <Text as="p" size="md" tone="secondary">{reach} Their personal connectors stay separate.</Text>
  </Section>
</Stack>
