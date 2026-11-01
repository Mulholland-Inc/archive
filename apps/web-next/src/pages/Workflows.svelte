<script>
  import { Plus, Workflow as WorkflowIcon } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Tabs from '../components/Tabs.svelte';
  import Grid from '../components/Grid.svelte';
  import WorkflowCard from '../components/WorkflowCard.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import NewWorkflowDialog from '../components/NewWorkflowDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, createWorkflow } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let tab = $state('all');
  let creating = $state(false);

  const statusOf = (w) => {
    const r = w.runs[0];
    if (!w.enabled) return 'off';
    if (!r) return 'idle';
    return r.status;
  };
  const count = (s) => ws().workflows.filter((w) => statusOf(w) === s).length;
  const items = $derived([
    { value: 'all', label: 'All', count: ws().workflows.length },
    { value: 'running', label: 'Running', count: count('running') },
    { value: 'waiting', label: 'Needs input', count: count('waiting') },
    { value: 'failed', label: 'Failed', count: count('failed') + count('stopped') },
    { value: 'off', label: 'Off', count: count('off') }
  ]);
  const shown = $derived(
    tab === 'all' ? ws().workflows : ws().workflows.filter((w) => statusOf(w) === tab || (tab === 'failed' && statusOf(w) === 'stopped'))
  );

  function create(data) {
    go(`/workflows/${createWorkflow(data)}`);
  }
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Workflows">
    {#snippet actions()}<Button variant="primary" icon={Plus} onclick={() => (creating = true)}>New workflow</Button>{/snippet}
    <Tabs bind:value={tab} {items} label="Filter workflows" />
  </PageHeader>
  {#if shown.length}
    <Grid min="card" gap="4">
      {#each shown as workflow (workflow.id)}
        <WorkflowCard {workflow} href={`#/workflows/${workflow.id}`} />
      {/each}
    </Grid>
  {:else if ws().workflows.length}
    <EmptyState icon={WorkflowIcon} title="Nothing here right now" />
  {:else}
    <EmptyState icon={WorkflowIcon} title="No workflows yet">
      <Button variant="primary" icon={Plus} onclick={() => (creating = true)}>New workflow</Button>
    </EmptyState>
  {/if}
</Page>

<NewWorkflowDialog bind:open={creating} oncreate={create} />
