<script>
  import { Play, RotateCcw, ListX, Ellipsis, Eye } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import CodeBlock from '../components/CodeBlock.svelte';
  import Message from '../components/Message.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
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
  import Chip from '../components/Chip.svelte';
  import { app, ws, atLeast, groups, members, groupById, setAccess, workflowById, runWorkflow, renameWorkflow, removeWorkflow, workflows, runs } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';
  import { workflowItems, runItems } from '../lib/menus.js';

  let { id } = $props();
  const wf = $derived(workflowById(id));
  workflows.use();
  runs.use(() => wf && id);
  groups.use();
  members.use();
  let selected = $state(null);

  const me = $derived(ws()?.members.find((m) => m.email === app.user.email));
  const mine = $derived(Boolean(wf) && (!wf.member || wf.member === me?.id));
  const group = $derived(wf?.access ? groupById(wf.access) : null);
  const accessOptions = $derived([
    { value: '', label: 'Only you' },
    ...(ws()?.groups ?? []).filter((g) => atLeast('manager') || me?.groups.includes(g.id) || g.id === wf?.access).map((g) => ({ value: g.id, label: g.name }))
  ]);

  const run = $derived(wf?.runs.find((r) => r.id === selected) ?? wf?.runs[0]);
  const current = $derived(run?.steps.find((s) => ['running', 'waiting', 'failed'].includes(s.status)) ?? run?.steps.at(-1));
  const live = $derived(run && (run.status === 'running' || run.status === 'waiting'));

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
      {:else if wf && mine}
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
      <PageHeader title={wf.name} renameLabel="Rename workflow" onrename={mine ? (name) => renameWorkflow(id, name) : undefined}>
        {#snippet actions()}
          {#if mine}<Menu end width="sm" label="Workflow actions">
            {#snippet trigger(props)}<IconButton icon={Ellipsis} label="Workflow actions" {...props} />{/snippet}
            <MenuItems items={workflowItems(wf, { open: false, onrun: start, ondelete: remove })} />
          </Menu>{/if}
        {/snippet}
        <Stack direction="row" gap="3" align="center" wrap>
          {#if run}<StatusBadge status={run.paused ? 'paused' : run.status} plain />{/if}
          <TriggerLabel trigger={wf.trigger} />
        </Stack>
      </PageHeader>
      <Stack gap="8">
        {#if wf.source}<CodeBlock code={wf.source} lang="typescript" size="body" full />{/if}
        {#if run}
          {#if outcome}<Message message={{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: outcome }] }} />{/if}
          {#each run.log as m (m.id)}<Message message={m} />{/each}
        {:else}
          <EmptyState icon={Play} title="This workflow hasn’t run yet">
            {#if mine}<Button variant="primary" icon={Play} onclick={start}>Run now</Button>{/if}
          </EmptyState>
        {/if}
      </Stack>
    </Page>

    {#snippet side()}
      <SidebarSection title="Who sees it">
        <div class="block">
          {#if mine}
            <Menu value={wf.access ?? ''} options={accessOptions} size="sm" label="Who sees it" full onchange={(v) => setAccess('workflow', wf, v || null)} />
          {:else if group}
            <Chip size="sm" href={`/access/groups/${group.id}`}>{group.name}</Chip>
          {/if}
        </div>
      </SidebarSection>
      {#if run}
        <SidebarSection title={`Run #${run.number}`}>
          <div class="block">
            <StatusBadge status={run.paused ? 'paused' : run.status} plain />
            <Text size="sm" tone="tertiary">Started {run.started}{#if run.elapsed}, {run.elapsed}{/if}</Text>
          </div>
        </SidebarSection>
      {/if}
      <SidebarSection>
        {#each wf.runs as r (r.id)}
          <SidebarItem label={`Run #${r.number}`} badge={r.started} active={r.id === run?.id} onclick={() => pick(r)} items={() => [{ label: 'Open run', icon: Eye, onselect: () => pick(r) }, ...runItems(r, wf, { onrun: start })]}>
            {#snippet leading()}<StatusBadge status={r.paused ? 'paused' : r.status} plain iconOnly />{/snippet}
          </SidebarItem>
        {:else}
          <div class="block"><Text size="sm" tone="tertiary">No runs yet</Text></div>
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

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-1-5);
  }
</style>
