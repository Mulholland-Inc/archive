<script>
  import { onMount } from 'svelte';
  import { Plus, SearchX, Database, MessageSquarePlus } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import SearchField from '../components/SearchField.svelte';
  import TypeTile from '../components/TypeTile.svelte';
  import TypeArt from '../components/TypeArt.svelte';
  import TypeDetail from '../components/TypeDetail.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Dialog from '../components/Dialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, newChat } from '../lib/app.svelte.js';
  import { go, route } from '../lib/router.svelte.js';

  let query = $state('');
  let openId = $state(route.parts[1] ?? null);
  let preview = $state(Boolean(route.parts[1]));
  let search = $state();

  const open = $derived(ws().schema.find((t) => t.id === openId));
  const types = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return ws().schema.filter((t) => !q || t.name.toLowerCase().includes(q) || t.columns.some((c) => c.name.includes(q)));
  });

  function show(id) {
    openId = id;
    preview = true;
  }

  function ask(type, close) {
    close?.();
    const id = newChat(`About ${type.name}`, { text: `What is in the ${type.name} table?` });
    go(`/chat/${id}`);
  }

  function create() {
    const id = newChat('New table', { text: 'Add a new table to this vault' });
    go(`/chat/${id}`);
  }

  onMount(() => {
    const key = (event) => {
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !/INPUT|TEXTAREA/.test(document.activeElement?.tagName ?? '')) {
        event.preventDefault();
        search?.focus();
      }
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  });
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Database">
    {#snippet actions()}
      <Button variant="primary" icon={Plus} onclick={create}>New table</Button>
    {/snippet}
  </PageHeader>

  <Stack gap="4">
    <Stack direction="row" gap="2" align="center" wrap>
      <SearchField bind:value={query} bind:element={search} placeholder="Search tables and columns" shortcut="/" />
    </Stack>
  </Stack>

  {#if !ws().schema.length}
    <EmptyState icon={Database} title="No tables yet">
      <Button variant="primary" icon={Plus} onclick={create}>New table</Button>
    </EmptyState>
  {:else if !types.length}
    <EmptyState icon={SearchX} title="Nothing matches that search">
      <Button onclick={() => (query = '')}>Show all tables</Button>
    </EmptyState>
  {:else}
    <Grid min="tile" gap="2">
      {#each types as type (type.id)}
        <TypeTile {type} onclick={() => show(type.id)} />
      {/each}
    </Grid>
  {/if}
</Page>

<Dialog bind:open={preview} title={open?.name} size="tall">
  {#snippet leading()}{#if open}<TypeArt icon={open.icon} size="sm" />{/if}{/snippet}
  {#if open}
    {#key open.id}
      <TypeDetail type={open} types={ws().schema} workflows={ws().workflows} onopen={show} onworkflow={(id) => go(`/workflows/${id}`)} />
    {/key}
  {/if}
  {#snippet footer({ close })}
    <Button variant="primary" icon={MessageSquarePlus} onclick={() => ask(open, close)}>Ask about this</Button>
  {/snippet}
</Dialog>
