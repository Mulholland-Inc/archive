<script>
  import { onMount } from 'svelte';
  import TopBar from '../components/TopBar.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import SearchField from '../components/SearchField.svelte';
  import SchemaGraph from '../components/SchemaGraph.svelte';
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

  function show(id) {
    openId = id;
    preview = true;
  }

  function ask(type, close) {
    close?.();
    const id = newChat(`About ${type.name}`, { text: `What is in the ${type.name} table?` });
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
<div class="head">
  <PageHeader title="Database">
    {#snippet actions()}
      <div class="search"><SearchField bind:value={query} bind:element={search} icon={null} placeholder="Find a table or column" shortcut="/" /></div>
    {/snippet}
  </PageHeader>
</div>

{#if ws().schema.length}
  {#key ws().id}
    <SchemaGraph types={ws().schema} {query} onopen={show} />
  {/key}
{:else}
  <EmptyState title="No tables yet" />
{/if}

<Dialog bind:open={preview} title={open?.name} size="tall">
  {#if open}
    {#key open.id}
      <TypeDetail type={open} types={ws().schema} workflows={ws().workflows} onopen={show} onworkflow={(id) => go(`/workflows/${id}`)} />
    {/key}
  {/if}
  {#snippet footer({ close })}
    <Button variant="primary" onclick={() => ask(open, close)}>Ask about this</Button>
  {/snippet}
</Dialog>

<style>
  .head {
    flex: none;
    padding: var(--space-4) var(--space-6) var(--space-0);
  }

  .search {
    flex: 0 1 var(--col-lg);
    min-width: 0;
  }

  @media (max-width: 40rem) {
    .head {
      padding: var(--space-2) var(--space-4) var(--space-0);
    }
  }
</style>
