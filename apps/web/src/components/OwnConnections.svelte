<script>
  import { Plug, RefreshCw, Unplug } from '@lucide/svelte';
  import Section from './Section.svelte';
  import Stack from './Stack.svelte';
  import Button from './Button.svelte';
  import Text from './Text.svelte';
  import ArchiveIcon from './ArchiveIcon.svelte';
  import { app, ownConnections, syncOwnConnection, disconnectOwnConnection } from '../lib/app.svelte.js';
  import { relative } from '../lib/live.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { connector, reconnect } = $props();

  ownConnections.use();

  const accounts = $derived(app.ownConnections.filter((x) => x.provider === connector.id));

  function synced(x) {
    if (x.error) return x.error;
    const r = x.synced_at && relative(x.synced_at);
    if (r) return `${(x.items ?? 0).toLocaleString()} items synced ${/^(Just|Today|Yesterday|\d)/.test(r) ? r.toLowerCase() : `on ${r}`}`;
    return x.sync_at ? 'Syncing…' : 'Not synced yet';
  }

  async function remove(x) {
    if (await confirmDialog({ title: `Disconnect ${x.account}?`, body: `This disconnects your ${connector.name} from ${x.workspace.name} and deletes what Archive synced from it there.`, confirm: 'Disconnect' }))
      disconnectOwnConnection(x);
  }
</script>

{#if accounts.length}
  <Section title={accounts.length > 1 ? 'Accounts' : 'Account'}>
    <ul class="accounts">
      {#each accounts as x (x.id)}
        <li class="account">
          <Stack gap="0-5">
            <Text weight="medium" truncate>{x.account}</Text>
            <Text size="sm" tone={x.error ? 'danger' : 'tertiary'}>{synced(x)}</Text>
          </Stack>
          <Stack direction="row" gap="3" align="center">
            <ArchiveIcon icon={x.look.icon} tint={x.look.tint} seed={x.workspace.id} size="lg" />
            <Text size="sm" tone="secondary" truncate>{x.workspace.name}</Text>
          </Stack>
          <div class="controls">
            {#if x.error && reconnect}
              <Button size="sm" variant="primary" icon={Plug} onclick={() => reconnect(x)}>Reconnect</Button>
            {:else}
              <Button size="sm" variant="ghost" icon={RefreshCw} onclick={() => syncOwnConnection(x)}>Sync</Button>
            {/if}
            <Button size="sm" variant="ghost" icon={Unplug} onclick={() => remove(x)}>Disconnect</Button>
          </div>
        </li>
      {/each}
    </ul>
  </Section>
{/if}

<style>
  .accounts {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .account {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, var(--col-md)) auto;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--row-height);
    padding-block: var(--space-2);
  }

  .account + .account {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
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
      grid-template-columns: minmax(0, 1fr);
      gap: var(--space-2);
    }

    .controls {
      justify-content: flex-start;
    }
  }
</style>
