<script>
  import { RefreshCw, Unplug, Plug, Plus, Check, ChevronDown } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import ConnectorIcon from './ConnectorIcon.svelte';
  import Button from './Button.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Menu from './Menu.svelte';
  import MenuItems from './MenuItems.svelte';
  import Badge from './Badge.svelte';
  import StatusBadge from './StatusBadge.svelte';
  import Text from './Text.svelte';
  import Icon from './Icon.svelte';
  import { ws, connect, connectionTypes, syncNow, syncAccount, disconnectAccount, setConnectionType, connections } from '../lib/app.svelte.js';
  import { relative } from '../lib/live.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { connector } = $props();

  connections.use();

  // Each account you connected at this connector, one row of archive.connection each.
  const accounts = $derived(connector.connections ?? []);

  function synced(x) {
    if (x.error) return x.error;
    const when = x.synced_at ? `Synced ${relative(x.synced_at).toLowerCase()}` : x.sync_at ? 'Syncing…' : 'Not synced yet';
    return `${when} · ${(x.items ?? 0).toLocaleString()} items`;
  }

  async function remove(x) {
    if (await confirmDialog({ title: `Disconnect ${x.account}?`, body: `It is disconnected in every archive you are in, and what it synced goes with it. Your other ${connector.name} accounts stay.`, confirm: 'Disconnect' }))
      disconnectAccount(x);
  }

  // Which archives an account is in: only those of its type read what it syncs and act with it. You pick it
  // before you connect one.
  const here = $derived(connectionTypes.find((t) => t.value === ws()?.look?.type)?.label.toLowerCase() ?? 'personal');
  const pick = [{ heading: 'For your archives that are' }, ...connectionTypes.map((t) => ({ label: t.label, onselect: () => connect(connector.id, t.value) }))];
</script>

<Stack gap="6">
  <Stack direction="row" gap="3" align="center" justify="between" wrap>
    <Stack direction="row" gap="3" align="center">
      <ConnectorIcon id={connector.id} size="lg" />
      <Stack gap="1">
        <Text weight="medium">{connector.category}</Text>
        {#if accounts.length}
          <Stack direction="row" gap="2" align="center"><StatusBadge status="connected" />{#if accounts.length > 1}<Badge>{accounts.length} accounts</Badge>{/if}</Stack>
        {:else}<Badge>Not connected</Badge>{/if}
      </Stack>
    </Stack>
    <Stack direction="row" gap="2">
      {#if accounts.length}
        {#if accounts.length > 1}<Button variant="ghost" icon={RefreshCw} onclick={() => syncNow(connector.id)}>Sync all</Button>{/if}
        <Menu placement="bottom-end" width="sm" label="Add account">
          {#snippet trigger(props)}<Button icon={Plus} iconEnd={ChevronDown} disabled={!connector.ready} {...props}>Add account</Button>{/snippet}
          <MenuItems items={pick} />
        </Menu>
      {:else}
        <Menu placement="bottom-end" width="sm" label="Connect">
          {#snippet trigger(props)}<Button variant="primary" icon={Plug} iconEnd={ChevronDown} disabled={!connector.ready} {...props}>Connect</Button>{/snippet}
          <MenuItems items={pick} />
        </Menu>
      {/if}
    </Stack>
  </Stack>
  {#if connector.ready === false}
    <Text as="p" size="md" tone="secondary">This server has no {connector.by} client set up for {connector.name} yet. Ask whoever runs it to set one up.</Text>
  {/if}
  {#if accounts.length}
    <Section title={accounts.length > 1 ? 'Accounts' : 'Account'}>
      <Text as="p" size="md" tone="secondary">Only you see them. Each shows, with what it syncs, in your archives of its type; this one is {here}. To add another, sign in to it with Add account.</Text>
      <ul class="accounts">
        {#each accounts as x (x.id)}
          <li class="account">
            <Stack gap="0-5" grow>
              <Text weight="medium" truncate>{x.account}</Text>
              <Text size="sm" tone={x.error ? 'danger' : 'tertiary'}>{synced(x)}</Text>
            </Stack>
            <div class="controls">
              <SegmentedControl value={x.type} options={connectionTypes} size="sm" label={`Archives ${x.account} is in`} onchange={(t) => setConnectionType(x, t)} />
              <Button size="sm" variant="ghost" icon={RefreshCw} onclick={() => syncAccount(x)}>Sync</Button>
              <Button size="sm" variant="ghost" icon={Unplug} onclick={() => remove(x)}>Disconnect</Button>
            </div>
          </li>
        {/each}
      </ul>
    </Section>
  {/if}
  <Section title="Archive can">
    <Stack gap="2">
      {#each connector.scopes as scope (scope)}
        <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
      {/each}
    </Stack>
  </Section>
</Stack>

<style>
  .accounts {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .account {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--row-height);
    padding-block: var(--space-2);
  }

  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
  }

  @container (max-width: 36rem) {
    .account {
      flex-direction: column;
      align-items: stretch;
    }

    .controls {
      justify-content: flex-start;
    }
  }

  .account + .account {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }
</style>
