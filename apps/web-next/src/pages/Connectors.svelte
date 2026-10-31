<script>
  import { RefreshCw, Unplug, Plug, SearchX, Check } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import Section from '../components/Section.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Tabs from '../components/Tabs.svelte';
  import ConnectorCard from '../components/ConnectorCard.svelte';
  import ConnectorGlyph from '../components/ConnectorGlyph.svelte';
  import Dialog from '../components/Dialog.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import Button from '../components/Button.svelte';
  import Badge from '../components/Badge.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, connect, disconnect, syncConnector } from '../lib/app.svelte.js';

  let query = $state('');
  let tab = $state('all');
  let openId = $state(null);
  let detail = $state(false);

  const open = $derived(ws().connectors.find((c) => c.id === openId));
  const tabs = $derived([
    { value: 'all', label: 'All', count: ws().connectors.length },
    { value: 'connected', label: 'Connected', count: ws().connectors.filter((c) => c.connected).length },
    { value: 'available', label: 'Available', count: ws().connectors.filter((c) => !c.connected).length }
  ]);
  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return ws().connectors.filter(
      (c) => (tab === 'all' || (tab === 'connected') === c.connected) && (!q || c.name.toLowerCase().includes(q) || c.category.toLowerCase().includes(q))
    );
  });

  function show(c) {
    openId = c.id;
    detail = true;
  }
</script>

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Connectors">
    <Tabs bind:value={tab} items={tabs} label="Filter connectors" />
  </PageHeader>
  <SearchField bind:value={query} placeholder="Search connectors" />
  {#if shown.length}
    <Grid min="card" gap="3">
      {#each shown as c (c.id)}
        <ConnectorCard connector={c} onopen={() => show(c)} onconnect={() => connect(c.id)} />
      {/each}
    </Grid>
  {:else}
    <EmptyState icon={SearchX} title="No connectors match" size="sm" />
  {/if}
</Page>

<Dialog bind:open={detail} title={open?.name} size="sm">
  {#if open}
    <Stack direction="row" gap="3" align="center">
      <ConnectorGlyph icon={open.icon} size="lg" />
      <Stack gap="1">
        <Text weight="medium">{open.category}</Text>
        {#if open.connected}<Badge tone="success" dot>Connected</Badge>{:else}<Badge>Not connected</Badge>{/if}
      </Stack>
    </Stack>
    {#if open.connected}
      <Stack gap="0">
        <SettingsRow label="Account"><Text size="sm" tone="secondary">{open.account}</Text></SettingsRow>
        <SettingsRow label="Last sync"><Text size="sm" tone="secondary">{open.lastSync}</Text></SettingsRow>
        <SettingsRow label="Items imported"><Text size="sm" tone="secondary" tabular>{open.items}</Text></SettingsRow>
      </Stack>
    {/if}
    <Section title="Archive can">
      <Stack gap="2">
        {#each open.scopes as scope (scope)}
          <Stack direction="row" gap="2" align="center"><Text tone="success"><Icon icon={Check} size="sm" /></Text><Text size="sm">{scope}</Text></Stack>
        {/each}
      </Stack>
    </Section>
  {/if}
  {#snippet footer({ close })}
    {#if open?.connected}
      <Button variant="ghost" icon={Unplug} onclick={() => { disconnect(open.id); close(); }}>Disconnect</Button>
      <Button icon={RefreshCw} onclick={() => syncConnector(open.id)}>Sync now</Button>
    {:else if open}
      <Button variant="ghost" onclick={close}>Cancel</Button>
      <Button variant="primary" icon={Plug} onclick={() => connect(open.id)}>Connect</Button>
    {/if}
  {/snippet}
</Dialog>
