<script>
  import { Plus, Star, FileText, MessagesSquare, SearchX } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import Card from '../components/Card.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Select from '../components/Select.svelte';
  import Button from '../components/Button.svelte';
  import Heading from '../components/Heading.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import Spacer from '../components/Spacer.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import NewProjectDialog from '../components/NewProjectDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { projects as seed, me } from '../lib/data.js';
  import { toast } from '../lib/toast.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let list = $state(seed.map((p) => ({ ...p })));
  let query = $state('');
  let sort = $state('activity');
  let creating = $state(false);

  const sorts = [
    { value: 'activity', label: 'Recent activity' },
    { value: 'name', label: 'Name' },
    { value: 'files', label: 'Most files' }
  ];

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = list.filter((p) => p.name.toLowerCase().includes(q));
    if (sort === 'name') return [...out].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === 'files') return [...out].sort((a, b) => b.files - a.files);
    return out;
  });

  function create({ name, instructions }) {
    const id = `p-${Date.now().toString(36)}`;
    seed.unshift({ id, name, instructions, updated: 'Just now', files: 0, chats: 0, members: [me.name] });
    list.unshift({ ...seed[0] });
    toast(`Created ${name}`, { tone: 'success' });
    go(`/projects/${id}`);
  }
</script>

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Projects">
    {#snippet actions()}<Button variant="primary" icon={Plus} onclick={() => (creating = true)}>New project</Button>{/snippet}
  </PageHeader>
  <Stack direction="row" gap="2" align="center" wrap>
    <SearchField bind:value={query} placeholder="Search projects" />
    <Spacer />
    <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
  </Stack>
  {#if shown.length}
    <Grid min="card" gap="4">
      {#each shown as p (p.id)}
        <Card href={`#/projects/${p.id}`} padding="lg">
          <Stack direction="row" align="start" justify="between" gap="3">
            <Heading level={3} size="md" serif={false}>{p.name}</Heading>
            {#if p.starred}<Text tone="warning"><Icon icon={Star} size="sm" /></Text>{/if}
          </Stack>
          <Text as="p" size="sm" tone="secondary">{p.instructions}</Text>
          <Spacer />
          <Stack direction="row" align="center" gap="4">
            <AvatarGroup names={p.members} size="xs" max={3} />
            <Spacer />
            <Text size="xs" tone="tertiary"><Stack as="span" direction="row" gap="1" align="center"><Icon icon={FileText} size="xs" />{p.files}</Stack></Text>
            <Text size="xs" tone="tertiary"><Stack as="span" direction="row" gap="1" align="center"><Icon icon={MessagesSquare} size="xs" />{p.chats}</Stack></Text>
            <Text size="xs" tone="tertiary">{p.updated}</Text>
          </Stack>
        </Card>
      {/each}
    </Grid>
  {:else}
    <EmptyState icon={SearchX} title="No projects match your search" />
  {/if}
</Page>

<NewProjectDialog bind:open={creating} oncreate={create} />
