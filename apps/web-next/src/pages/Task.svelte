<script>
  import { Pause, Play, Square, Bot, Clock, Zap, CalendarClock, RotateCcw, FileText, ListX } from '@lucide/svelte';
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
  import Composer from '../components/Composer.svelte';
  import ComposerDock from '../components/ComposerDock.svelte';
  import StepPill from '../components/StepPill.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import Attachment from '../components/Attachment.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { tasks } from '../lib/data.js';
  import { app } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  let { id } = $props();
  const task = tasks.find((t) => t.id === id);
  let paused = $state(false);
  let value = $state('');
  let open = $state(true);

  const done = $derived(task ? task.steps.filter((s) => s.status === 'done').length : 0);
  const current = $derived(task?.steps.find((s) => s.status === 'running' || s.status === 'waiting' || s.status === 'failed') ?? task?.steps.at(-1));
  const live = $derived(task && (task.status === 'running' || task.status === 'waiting'));
  const tone = $derived(task?.status === 'failed' ? 'danger' : task?.status === 'completed' ? 'success' : 'accent');

  const intro = $derived(task ? { id: 'u', role: 'user', text: task.title } : null);
  const outputs = $derived(
    task?.status === 'completed'
      ? [{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: `Finished in **${task.elapsed}**. Everything is saved in the library, and the people who follow this folder were notified.` }] }]
      : task?.status === 'failed'
        ? [{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: 'I stopped at batch 2: those scans are below **300 dpi**, so restoring them would make them worse. Rescan them at 600 dpi and I’ll pick up where I left off.' }] }]
        : task?.status === 'waiting'
          ? [{ id: 'o', role: 'assistant', blocks: [{ type: 'p', text: 'I matched most faces, but **52 faces** look like people who aren’t in the family tree yet. Can you take a look before I write the tags?' }] }]
          : []
  );
</script>

<TopBar>
  {#snippet leading()}
    <MobileMenu />
    <Breadcrumbs items={[{ label: 'Tasks', href: '#/tasks' }, { label: task?.title ?? 'Not found' }]} />
  {/snippet}
  {#snippet trailing()}
    {#if live}
      <Button size="sm" icon={paused ? Play : Pause} onclick={() => { paused = !paused; toast(paused ? 'Task paused' : 'Task resumed'); }}>{paused ? 'Resume' : 'Pause'}</Button>
      <Button size="sm" variant="ghost" icon={Square} onclick={() => toast('Task stopped')}>Stop</Button>
    {:else if task?.status === 'failed'}
      <Button size="sm" icon={RotateCcw} onclick={() => toast('Retrying from batch 2')}>Retry</Button>
    {/if}
  {/snippet}
</TopBar>

{#if task}
  <Page width="wide">
    <Split size="lg" sticky>
      <PageHeader title={task.title}>
        <Stack direction="row" gap="3" align="center" wrap>
          <StatusBadge status={paused ? 'scheduled' : task.status} />
          <Text size="sm" tone="tertiary">{task.agent} · started {task.started}{task.elapsed ? ` · ${task.elapsed}` : ''}</Text>
        </Stack>
      </PageHeader>
      <Stack gap="8">
        <Message message={intro} />
        <Steps steps={task.steps} running={live && !paused} bind:open />
        {#each outputs as m (m.id)}
          <Message message={m} />
        {/each}
      </Stack>

      {#snippet aside()}
        <Card padding="md" variant="filled">
          <Section title="Progress">
            {#snippet actions()}<Text size="sm" tone="tertiary" tabular>{done} of {task.steps.length}</Text>{/snippet}
            <ProgressBar value={task.progress} {tone} />
            <Stack gap="2">
              {#each task.steps as step (step.id)}
                <Stack direction="row" gap="2" align="center">
                  <Text tone={step.status === 'done' ? 'success' : step.status === 'failed' ? 'danger' : step.status === 'running' ? 'accent' : 'tertiary'}>
                    <Icon icon={step.status === 'done' ? Zap : Clock} size="sm" />
                  </Text>
                  <Text size="sm" tone={step.status === 'pending' ? 'tertiary' : 'default'} truncate>{step.label}</Text>
                </Stack>
              {/each}
            </Stack>
          </Section>
        </Card>
        <Card padding="md" variant="filled">
          <Section title="Details">
            <Stack gap="0">
              <SettingsRow label="Agent"><Stack direction="row" gap="1-5" align="center"><Icon icon={Bot} size="sm" /><Text size="sm" tone="secondary">{task.agent}</Text></Stack></SettingsRow>
              <SettingsRow label="Trigger"><Stack direction="row" gap="1-5" align="center"><Icon icon={CalendarClock} size="sm" /><Text size="sm" tone="secondary">{task.status === 'scheduled' ? 'Monthly' : 'Manual'}</Text></Stack></SettingsRow>
              <SettingsRow label="Started"><Text size="sm" tone="secondary">{task.started}</Text></SettingsRow>
            </Stack>
          </Section>
        </Card>
        {#if done}
          <Card padding="md" variant="filled">
            <Section title="Output">
              <Stack gap="2">
                <Attachment name={`${task.title}.md`} kind="document" size="12 KB" tone={1} />
                <Attachment name="run-log.txt" kind="document" size="3 KB" tone={3} />
              </Stack>
            </Section>
          </Card>
        {/if}
      {/snippet}
    </Split>
  </Page>
  <ComposerDock>
    {#snippet above()}
      {#if current}<StepPill label={current.label} {done} total={task.steps.length} running={live && !paused} onclick={() => (open = !open)} />{/if}
    {/snippet}
    <Composer bind:value bind:model={app.model} placeholder="Steer this task" onsend={() => toast('Archive will pick that up after this step')} />
  </ComposerDock>
{:else}
  <Page center>
    <EmptyState icon={ListX} title="Task not found">
      <Button href="#/tasks">All tasks</Button>
    </EmptyState>
  </Page>
{/if}
