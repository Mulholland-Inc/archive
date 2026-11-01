<script>
  import { Pause, Play, Square, Bot, RotateCcw, ListX, Ellipsis, Trash2, CircleCheck } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Split from '../components/Split.svelte';
  import Card from '../components/Card.svelte';
  import Section from '../components/Section.svelte';
  import Message from '../components/Message.svelte';
  import Steps from '../components/Steps.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import ProgressBar from '../components/ProgressBar.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import TriggerLabel from '../components/TriggerLabel.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Switch from '../components/Switch.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { workflowById, runWorkflow, toggleWorkflow, renameWorkflow, removeWorkflow, pauseRun, stopRun, resolveRun } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  let { id } = $props();
  const wf = $derived(workflowById(id));
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
    if (run.status === 'succeeded') return `Run #${run.number} finished in **${run.elapsed}**. The results are saved in the library.`;
    if (run.status === 'failed') return `Run #${run.number} stopped at **${current?.label}**: ${current?.detail || 'something went wrong'}. Fix it and run the workflow again.`;
    if (run.status === 'stopped') return `Run #${run.number} was stopped.`;
    if (run.status === 'waiting') return `I need your input on **${current?.label}** before I can go on.`;
    return null;
  });

  function start() {
    selected = runWorkflow(id);
    open = true;
  }

  async function remove() {
    if (await confirmDialog({ title: 'Delete this workflow?', body: `“${wf.name}” and its run history will be removed.` })) removeWorkflow(id);
  }
</script>

<TopBar>
  {#snippet leading()}
    <MobileMenu />
    <Breadcrumbs items={[{ label: 'Workflows', href: '#/workflows' }, { label: wf?.name ?? 'Not found' }]} />
  {/snippet}
  {#snippet trailing()}
    {#if run && live}
      <Button size="sm" icon={run.paused ? Play : Pause} onclick={() => pauseRun(run)}>{run.paused ? 'Resume' : 'Pause'}</Button>
      <Button size="sm" variant="ghost" icon={Square} onclick={() => stopRun(run)}>Stop</Button>
    {:else if wf}
      <Button size="sm" variant="primary" icon={run?.status === 'failed' || run?.status === 'stopped' ? RotateCcw : Play} onclick={start}>{run?.status === 'failed' || run?.status === 'stopped' ? 'Run again' : 'Run now'}</Button>
    {/if}
  {/snippet}
</TopBar>

{#if wf}
  <Page width="wide">
    <Split size="lg" sticky>
      <PageHeader title={wf.name} renameLabel="Rename workflow" onrename={(name) => renameWorkflow(id, name)}>
        {#snippet actions()}
          <Menu placement="bottom-end" width="sm" label="Workflow actions">
            {#snippet trigger(props)}<IconButton icon={Ellipsis} label="Workflow actions" {...props} />{/snippet}
            <MenuItem icon={Play} onselect={start}>Run now</MenuItem>
            <MenuSeparator />
            <MenuItem icon={Trash2} danger onselect={remove}>Delete</MenuItem>
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
          {#if run.status === 'waiting'}
            <Card variant="filled" padding="md">
              <Text weight="medium">{current?.label}</Text>
              <Stack direction="row" gap="2">
                <Button variant="primary" icon={CircleCheck} onclick={() => resolveRun(run)}>Review and continue</Button>
                <Button variant="ghost" onclick={() => stopRun(run)}>Skip this run</Button>
              </Stack>
            </Card>
          {/if}
          {#if outcome}<Message message={{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: outcome }] }} />{/if}
          {#each run.log as m (m.id)}<Message message={m} />{/each}
        </Stack>
      {:else}
        <EmptyState icon={Play} title="This workflow hasn’t run yet">
          <Button variant="primary" icon={Play} onclick={start}>Run now</Button>
        </EmptyState>
      {/if}

      {#snippet aside()}
        {#if run}
          <Card padding="md" variant="filled">
            <Section title={`Run #${run.number}`}>
              {#snippet actions()}<Text size="sm" tone="tertiary" tabular>{done} of {run.steps.length}</Text>{/snippet}
              <ProgressBar value={done / run.steps.length} {tone} />
              <Text size="sm" tone="tertiary">Started {run.started}, {run.elapsed}</Text>
            </Section>
          </Card>
        {/if}
        <Card padding="md" variant="filled">
          <Section title="Settings">
            <Stack gap="0">
              <SettingsRow label="On"><Switch checked={wf.enabled} label="Workflow on" onchange={() => toggleWorkflow(id)} /></SettingsRow>
              <SettingsRow label="Agent"><Stack direction="row" gap="1-5" align="center"><Icon icon={Bot} size="sm" /><Text size="sm" tone="secondary">{wf.agent}</Text></Stack></SettingsRow>
              {#if wf.next}<SettingsRow label="Next run"><Text size="sm" tone="secondary">{wf.enabled ? wf.next : 'Off'}</Text></SettingsRow>{/if}
            </Stack>
          </Section>
        </Card>
        <Card padding="md" variant="filled">
          <Section title="Runs">
            {#if wf.runs.length}
              <List label="Runs" divided={false} inset>
                {#each wf.runs as r (r.id)}
                  <ListItem title={`Run #${r.number}`} size="sm" active={r.id === run?.id} onclick={() => (selected = r.id)}>
                    {#snippet meta()}{r.started}, {r.elapsed}{/snippet}
                    {#snippet trailing()}<StatusBadge status={r.paused ? 'paused' : r.status} plain />{/snippet}
                  </ListItem>
                {/each}
              </List>
            {:else}
              <Text size="sm" tone="tertiary">No runs yet</Text>
            {/if}
          </Section>
        </Card>
      {/snippet}
    </Split>
  </Page>
{:else}
  <Page center>
    <EmptyState icon={ListX} title="Workflow not found">
      <Button href="#/workflows">All workflows</Button>
    </EmptyState>
  </Page>
{/if}
