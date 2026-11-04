<script>
  import { Plug, Check } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import Button from './Button.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import OwnConnections from './OwnConnections.svelte';
  import KeysDialog from './KeysDialog.svelte';
  import { connect, connectLink, connections } from '../lib/app.svelte.js';

  let { connector, onback } = $props();
  let keying = $state(false);
  // A connector with keys has no sign-in: Connect asks for them, or opens the widget that hands them over. One
  // whose sign-in is the account's own asks which account first.
  const start = () => (connector.link ? connectLink(connector.id) : connector.keys?.length || connector.ask ? (keying = true) : connect(connector.id));

  connections.use();
</script>

<SettingsPane {onback}>
  {#snippet lead()}
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={connector.id} size="lg" />
      <Text weight="medium">{connector.name}</Text>
      {#if connector.connected}<StatusBadge status="connected" />{:else}<Badge>Not connected</Badge>{/if}
    </Stack>
  {/snippet}
  {#snippet tools()}
    {#if !connector.connected}
      <Button variant="primary" icon={Plug} disabled={!connector.ready} onclick={start}>Connect</Button>
    {/if}
  {/snippet}
  <Stack gap="6">
    {#if connector.ready === false}
      <Text as="p" size="md" tone="secondary">This server has no {connector.by} client set up for {connector.name} yet. Ask whoever runs it to set one up.</Text>
    {/if}
    <OwnConnections {connector} />
    <Section title="Archive can">
      <Stack gap="2">
        {#each connector.scopes as scope (scope)}
          <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
        {/each}
      </Stack>
    </Section>
  </Stack>
</SettingsPane>
<KeysDialog bind:open={keying} {connector} />
