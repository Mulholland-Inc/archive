<script>
  import { onMount } from 'svelte';
  import { Play, Trash2, SearchX, Workflow as WorkflowIcon } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import Select from '../components/Select.svelte';
  import Spacer from '../components/Spacer.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Text from '../components/Text.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, runWorkflow, removeWorkflow } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';
  import { go } from '../lib/router.svelte.js';

  const statuses = [
    { value: 'all', label: 'All' },
    { value: 'running', label: 'Running' },
    { value: 'waiting', label: 'Needs input' },
    { value: 'succeeded', label: 'Succeeded' },
    { value: 'failed', label: 'Failed' },
    { value: 'idle', label: 'Never run' }
  ];
  const sorts = [
    { value: 'recent', label: 'Recently run' },
    { value: 'name', label: 'Name' },
    { value: 'runs', label: 'Most runs' }
  ];
  const columns = [
    { key: 'name', label: 'Workflow' },
    { key: 'status', label: 'Last run', width: 'sm' },
    { key: 'last', label: 'When', muted: true, width: 'sm' },
    { key: 'actions', label: '', width: 'sm', align: 'end' }
  ];

  let query = $state('');
  let status = $state('all');
  let sort = $state('recent');
  let search = $state();

  const statusOf = (w) => (!w.enabled ? 'off' : w.runs[0] ? (w.runs[0].paused ? 'paused' : w.runs[0].status) : 'idle');
  const lastRun = (w) => (w.runs[0] ? (w.runs[0].status === 'stopped' ? 'failed' : w.runs[0].status) : 'idle');
  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = ws().workflows.filter(
      (w) => (status === 'all' || lastRun(w) === status) && (!q || w.name.toLowerCase().includes(q))
    );
    if (sort === 'name') return [...out].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === 'runs') return [...out].sort((a, b) => b.runs.length - a.runs.length);
    return out;
  });

  async function remove(workflow) {
    if (await confirmDialog({ title: 'Delete this workflow?', body: `“${workflow.name}” and its runs will be removed.`, confirm: 'Delete' })) removeWorkflow(workflow.id);
  }

  function clear() {
    query = '';
    status = 'all';
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
  <PageHeader title="Workflows" />

  <Stack gap="4">
    <Stack direction="row" gap="2" align="center" wrap>
      <SearchField bind:value={query} bind:element={search} placeholder="Search workflows" shortcut="/" />
    </Stack>
    <Stack direction="row" gap="2" align="center" wrap>
      {#each statuses as s (s.value)}
        <Chip selected={status === s.value} onclick={() => (status = s.value)}>{s.label}</Chip>
      {/each}
      <Spacer />
      <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
    </Stack>
  </Stack>

  {#if !ws().workflows.length}
    <EmptyState icon={WorkflowIcon} title="No workflows yet" />
  {:else if !rows.length}
    <EmptyState icon={SearchX} title="Nothing matches that search">
      <Button onclick={clear}>Show all workflows</Button>
    </EmptyState>
  {:else}
    <Table {columns} {rows} label="Workflows" onrowclick={(w) => go(`/workflows/${w.id}`)}>
      {#snippet cell(row, col)}
        {#if col.key === 'name'}
          <Text weight="medium" truncate>{row.name}</Text>
        {:else if col.key === 'status'}
          <StatusBadge status={statusOf(row)} />
        {:else if col.key === 'last'}
          {row.runs[0]?.started ?? 'Never'}
        {:else if col.key === 'actions'}
          <Stack direction="row" gap="1" align="center" justify="end">
            <IconButton icon={Play} label={`Run ${row.name}`} size="sm" onclick={(e) => { e.stopPropagation(); runWorkflow(row.id); }} />
            <IconButton icon={Trash2} label={`Delete ${row.name}`} size="sm" onclick={(e) => { e.stopPropagation(); remove(row); }} />
          </Stack>
        {/if}
      {/snippet}
    </Table>
  {/if}
</Page>
