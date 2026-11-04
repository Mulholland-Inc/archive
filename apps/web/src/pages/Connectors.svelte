<script>
  import { SearchX, Plug } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import Stack from '../components/Stack.svelte';
  import Section from '../components/Section.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import ConnectorCard from '../components/ConnectorCard.svelte';
  import ConnectorDetail from '../components/ConnectorDetail.svelte';
  import IntegrationDetail from '../components/IntegrationDetail.svelte';
  import OwnerNotice from '../components/OwnerNotice.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, connect, connections, archiveIntegrations, isArchiveOwner } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let { id } = $props();
  connections.use();

  const statuses = [
    { value: 'all', label: 'All' },
    { value: 'connected', label: 'Connected' },
    { value: 'available', label: 'Not connected' }
  ];

  let query = $state('');
  let status = $state('all');

  const owner = $derived(isArchiveOwner());
  const connector = $derived(ws().connectors.find((c) => c.id === id));
  const integration = $derived(archiveIntegrations().find((c) => c.id === id));
  const open = (c) => go(`/connectors/${c.id}`);

  function matching(list) {
    const q = query.trim().toLowerCase();
    return list.filter(
      (c) =>
        (status === 'all' || (status === 'connected') === Boolean(c.connected)) &&
        (!q || [c.name, c.category, c.description, c.by, c.account].some((v) => (v ?? '').toLowerCase().includes(q)))
    );
  }

  const yours = $derived(matching(ws().connectors));
  const shared = $derived(matching(archiveIntegrations()));
</script>

{#if id}
  <TopBar>
    {#snippet leading()}
      <MobileMenu />
      <Breadcrumbs inset items={[{ label: 'Connectors', href: '/connectors' }, { label: connector?.name ?? integration?.name ?? 'Not found' }]} />
    {/snippet}
  </TopBar>
  <Page>
    {#if connector}
      <ConnectorDetail {connector} />
    {:else if integration}
      <IntegrationDetail {integration} />
    {:else}
      <EmptyState icon={Plug} title={ws().loading ? 'Loading connector' : 'No such connector'} />
    {/if}
  </Page>
{:else}
  <TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
  <Page width="wide">
    <StickyHeader title="Connectors" art="connectors">
      {#snippet search()}
        <SearchField bind:value={query} placeholder="Search connectors and integrations" />
      {/snippet}
    </StickyHeader>

    <Stack gap="6">
      <Stack direction="row" gap="2" align="center" wrap>
        {#each statuses as s (s.value)}
          <Chip selected={status === s.value} onclick={() => (status = s.value)}>{s.label}</Chip>
        {/each}
      </Stack>

      {#if yours.length}
        <Section title="Yours">
          <div class="grid">
            {#each yours as c (c.id)}
              <ConnectorCard connector={c} onopen={() => open(c)} onconnect={() => (c.link || c.keys?.length || c.ask ? open(c) : connect(c.id))} />
            {/each}
          </div>
        </Section>
      {/if}

      {#if shared.length}
        <Section title="Integrations">
          {#if !owner}<OwnerNotice what="integrations" />{/if}
          <div class="grid">
            {#each shared as c (c.id)}
              <ConnectorCard connector={c} onopen={() => open(c)} onconnect={owner ? () => open(c) : undefined} />
            {/each}
          </div>
        </Section>
      {/if}

      {#if !yours.length && !shared.length}
        <EmptyState icon={SearchX} title="No connectors match" size="sm" />
      {/if}
    </Stack>
  </Page>
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(var(--col-lg), 100%), 1fr));
    gap: var(--space-3);
  }
</style>
