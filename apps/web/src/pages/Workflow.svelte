<script>
  import { Pause, Play, Square, RotateCcw, ListX, Ellipsis, CircleCheck, Eye } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Card from '../components/Card.svelte';
  import Message from '../components/Message.svelte';
  import Steps from '../components/Steps.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import ProgressBar from '../components/ProgressBar.svelte';
  import TriggerLabel from '../components/TriggerLabel.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItems from '../components/MenuItems.svelte';
  import Text from '../components/Text.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import RightSidebar from '../components/RightSidebar.svelte';
  import DetailsButton from '../components/DetailsButton.svelte';
  import SidebarSection from '../components/SidebarSection.svelte';
  import SidebarItem from '../components/SidebarItem.svelte';
  import SidebarBlock from '../components/SidebarBlock.svelte';
  import { app, workflowById, runWorkflow, renameWorkflow, removeWorkflow, workflows, runs } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';
  import { workflowItems, runItems } from '../lib/menus.js';

  let { id } = $props();
  const wf = $derived(workflowById(id));
  workflows.use();
  runs.use(() => wf && id);
  let selected = $state(null);
  let open = $state(true);

  const run = $derived(wf?.runs.find((r) => r.id === selected) ?? wf?.runs[0]);
  const done = $derived(run ? run.steps.filter((s) => s.status === 'done').length : 0);
  const current = $derived(run?.steps.find((s) => ['running', 'waiting', 'failed'].includes(s.status)) ?? run?.steps.at(-1));
  const live = $derived(run && (run.status === 'running' || run.status === 'waiting'));
  const tone = $derived(run?.status === 'failed' ? 'danger' : run?.status === 'succeeded' ? 'success' : run?.status === 'stopped' ? 'neutral' : 'accent');

  const intro = $derived(wf ? { id: 'u', role: 'user', text: `${wf.trigger.label}: ${wf.steps.join(', ').toLowerCase()}.` } : null);
  const outcome = $derived.by(() => {
    if (!run) return null;
    if (run.status === 'succeeded') return `Run #${run.number} finished in **${run.elapsed}**.`;
    if (run.status === 'failed') return `Run #${run.number} failed: ${current?.detail || 'something went wrong'}. Ask the assistant to fix it, then run the workflow again.`;
    if (run.status === 'stopped') return `Run #${run.number} was stopped.`;
    if (run.status === 'waiting') return `I need your input on **${current?.label}** before I can go on.`;
    return null;
  });

  function pick(r) {
    selected = r.id;
    app.details = false;
  }

  function start() {
    selected = runWorkflow(id);
    open = true;
  }

  async function remove() {
    if (await confirmDialog({ title: 'Delete this workflow?', body: `“${wf.name}” and its run history will be removed.` })) removeWorkflow(id);
  }
</script>

{#snippet topbar()}
  <TopBar>
    {#snippet leading()}
      <MobileMenu />
      <Breadcrumbs inset items={[{ label: 'Workflows', href: '/workflows' }, { label: wf?.name ?? 'Not found' }]} />
    {/snippet}
    {#snippet trailing()}
      {#if run && live}
        <Button size="sm" loading disabled>Running</Button>
      {:else if wf}
        <Button size="sm" variant="primary" icon={run?.status === 'failed' || run?.status === 'stopped' ? RotateCcw : Play} onclick={start}>{run?.status === 'failed' || run?.status === 'stopped' ? 'Run again' : 'Run now'}</Button>
      {/if}
      {#if wf}<DetailsButton />{/if}
    {/snippet}
  </TopBar>
{/snippet}

{#if wf}
  <RightSidebar art="bg-workflows" label="Workflow details">
    {@render topbar()}
    <Page>
      <PageHeader title={wf.name} renameLabel="Rename workflow" onrename={(name) => renameWorkflow(id, name)}>
        {#snippet actions()}
          <Menu placement="bottom-end" width="sm" label="Workflow actions">
            {#snippet trigger(props)}<IconButton icon={Ellipsis} label="Workflow actions" {...props} />{/snippet}
            <MenuItems items={workflowItems(wf, { open: false, onrun: start, ondelete: remove })} />
          </Menu>
        {/snippet}
        <Stack direction="row" gap="3" align="center" wrap>
          {#if run}<StatusBadge status={run.paused ? 'paused' : run.status} plain />{/if}
          <TriggerLabel trigger={wf.trigger} />
        </Stack>
      </PageHeader>
      {#if run}
        <Stack gap="8">
          <Message message={intro} />
          <Steps steps={run.steps} running={live && !run.paused} bind:open />
          {#if outcome}<Message message={{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: outcome }] }} />{/if}
          {#each run.log as m (m.id)}<Message message={m} />{/each}
        </Stack>
      {:else}
        <EmptyState icon={Play} title="This workflow hasn’t run yet">
          <Button variant="primary" icon={Play} onclick={start}>Run now</Button>
        </EmptyState>
      {/if}
    </Page>

    {#snippet side()}
      {#if run}
        <SidebarSection title={`Run #${run.number}`}>
          {#snippet action()}<Text size="xs" tone="tertiary" tabular>{done} of {run.steps.length}</Text>{/snippet}
          <SidebarBlock>
            <StatusBadge status={run.paused ? 'paused' : run.status} plain />
            <ProgressBar value={done / run.steps.length} {tone} />
            <Text size="sm" tone="tertiary">Started {run.started}{#if run.elapsed}, {run.elapsed}{/if}</Text>
          </SidebarBlock>
        </SidebarSection>
      {/if}
      <SidebarSection>
        {#each wf.runs as r (r.id)}
          <SidebarItem label={`Run #${r.number}`} badge={r.started} active={r.id === run?.id} onclick={() => pick(r)} items={() => [{ label: 'Open run', icon: Eye, onselect: () => pick(r) }, ...runItems(r, wf, { onrun: start })]}>
            {#snippet leading()}<StatusBadge status={r.paused ? 'paused' : r.status} plain iconOnly />{/snippet}
          </SidebarItem>
        {:else}
          <SidebarBlock><Text size="sm" tone="tertiary">No runs yet</Text></SidebarBlock>
        {/each}
      </SidebarSection>
    {/snippet}
  </RightSidebar>
{:else}
  {@render topbar()}
  <Page center>
    <EmptyState icon={ListX} title="Workflow not found">
      <Button href="/workflows">All workflows</Button>
    </EmptyState>
  </Page>
{/if}
