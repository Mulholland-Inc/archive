<script>
  import { Plus, ListChecks } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Tabs from '../components/Tabs.svelte';
  import Grid from '../components/Grid.svelte';
  import TaskCard from '../components/TaskCard.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { tasks } from '../lib/data.js';
  import { go } from '../lib/router.svelte.js';

  let tab = $state('all');
  const count = (s) => tasks.filter((t) => t.status === s).length;
  const items = [
    { value: 'all', label: 'All', count: tasks.length },
    { value: 'running', label: 'Running', count: count('running') },
    { value: 'waiting', label: 'Needs input', count: count('waiting') },
    { value: 'completed', label: 'Completed', count: count('completed') },
    { value: 'failed', label: 'Failed', count: count('failed') },
    { value: 'scheduled', label: 'Scheduled', count: count('scheduled') }
  ];
  const shown = $derived(tab === 'all' ? tasks : tasks.filter((t) => t.status === tab));
</script>

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Tasks">
    {#snippet actions()}<Button variant="primary" icon={Plus} onclick={() => go('/')}>New task</Button>{/snippet}
    <Tabs bind:value={tab} {items} label="Filter tasks" />
  </PageHeader>
  {#if shown.length}
    <Grid min="card" gap="4">
      {#each shown as task (task.id)}
        <TaskCard {task} href={`#/tasks/${task.id}`} />
      {/each}
    </Grid>
  {:else}
    <EmptyState icon={ListChecks} title="Nothing here right now" />
  {/if}
</Page>
