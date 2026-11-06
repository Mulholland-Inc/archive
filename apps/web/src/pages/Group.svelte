<script>
  import { UsersRound, UserPlus, UserMinus, Table2, Plug, Pencil, Trash2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import InlineEdit from '../components/InlineEdit.svelte';
  import Section from '../components/Section.svelte';
  import Table from '../components/Table.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Text from '../components/Text.svelte';
  import Stack from '../components/Stack.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { ws, seedOf, members, groups, usage, connections, groupById, updateGroup, setGroup, atLeast, tableOf } from '../lib/app.svelte.js';
  import { renameGroupPrompt, deleteGroupConfirm, usesText } from '../lib/menus.js';
  import { go } from '../lib/router.svelte.js';

  let { id } = $props();

  members.use();
  groups.use();
  connections.use();
  usage.use(() => id);

  const group = $derived(groupById(id));
  const manages = $derived(atLeast('manager'));
  const people = $derived(ws().members.filter((m) => m.group === id));
  const others = $derived(ws().members.filter((m) => m.group !== id));
  const uses = $derived(ws().usage[id]);
  const integrations = $derived((ws().integrations ?? []).filter((i) => i.connected && i.access === id));
  const tables = $derived((uses ?? []).filter((u) => u.table !== 'connection'));
  const inUse = $derived(Boolean(uses?.length));
  let describing = $state(false);

  const memberColumns = $derived([
    { key: 'name', label: 'Name' },
    { key: 'email', label: 'Email', muted: true, width: 'lg' },
    ...(manages ? [{ key: 'menu', label: '', width: 'xs', align: 'end' }] : [])
  ]);
  const placeOf = (table) => (table === 'file' ? '/library' : tableOf(table) ? `/database/${table}` : null);

  async function remove() {
    if (await deleteGroupConfirm(group)) go('/access/groups');
  }
</script>

{#snippet avatar(row)}<Avatar name={row.name} seed={seedOf(row)} size="md" />{/snippet}

<TopBar>
  {#snippet leading()}
    <MobileMenu />
    <Breadcrumbs inset items={[{ label: 'Groups', href: '/access/groups' }, { label: group?.name ?? (ws().groups.length ? 'Not found' : '') }]} />
  {/snippet}
</TopBar>

{#if group}
  <Page>
    <PageHeader title={group.name} renameLabel="Rename group" onrename={manages ? (name) => updateGroup(group, { name }) : undefined}>
      {#snippet actions()}
        {#if manages}
          <IconButton icon={Pencil} label="Rename" onclick={() => renameGroupPrompt(group)} />
          <IconButton icon={Trash2} label="Delete" onclick={remove} />
        {/if}
      {/snippet}
      {#if describing}
        <InlineEdit value={group.description} label="Description" onsave={(description) => { describing = false; updateGroup(group, { description }); }} oncancel={() => (describing = false)} />
      {:else if manages}
        <button type="button" class="describe" onclick={() => (describing = true)}>
          <Text tone={group.description ? 'secondary' : 'tertiary'}>{group.description || 'Add a description'}</Text>
        </button>
      {:else if group.description}
        <Text as="p" tone="secondary">{group.description}</Text>
      {/if}
    </PageHeader>

    <Stack gap="8">
      <Section title={people.length === 1 ? '1 member' : `${people.length} members`}>
        {#snippet actions()}
          {#if manages && others.length}
            <Menu end width="md" label="Add a member">
              {#snippet trigger(props)}<Button size="sm" icon={UserPlus} {...props}>Add member</Button>{/snippet}
              {#each others as m (m.id)}
                <MenuItem onselect={() => setGroup(m, id)}>
                  {#snippet leading()}<Avatar name={m.name} seed={seedOf(m)} size="xs" />{/snippet}
                  {m.name}
                </MenuItem>
              {/each}
            </Menu>
          {/if}
        {/snippet}
        {#if people.length}
          <Table columns={memberColumns} rows={people} label="Members" leading={avatar} cards>
            {#snippet cell(row, col)}
              {#if col.key === 'name'}
                <Text weight="medium" truncate>{row.name}</Text>
              {:else if col.key === 'email'}
                {row.email}
              {:else if col.key === 'menu'}
                <IconButton icon={UserMinus} label={`Remove ${row.name} from ${group.name}`} size="sm" onclick={() => setGroup(row, null)} />
              {/if}
            {/snippet}
          </Table>
        {:else}
          <EmptyState icon={UsersRound} title="No one is in this group yet" size="sm" />
        {/if}
      </Section>

      <Section title="Where it is used">
        {#if !uses}
          <Text tone="tertiary">Looking…</Text>
        {:else if tables.length || integrations.length}
          <List>
            {#each tables as u (u.table)}
              <ListItem href={placeOf(u.table)} icon={Table2}>
                <Text>{u.table === 'file' ? 'Files' : (tableOf(u.table)?.name ?? u.table)}</Text>
                <Text tone="tertiary">{u.rows === 1 ? '1 row' : `${u.rows} rows`}</Text>
              </ListItem>
            {/each}
            {#each integrations as i (i.id)}
              <ListItem href={`/settings/integrations/${i.id}`} icon={Plug}>
                <Text>{i.name}</Text>
                <Text tone="tertiary">Narrowed to {group.name}</Text>
              </ListItem>
            {/each}
          </List>
        {:else}
          <Text tone="tertiary">No row is limited to {group.name}.</Text>
        {/if}
        {#if manages}
          <Stack direction="row" gap="3" align="center" wrap>
            <Button variant="danger" disabled={!uses || inUse} onclick={remove}>Delete group</Button>
            {#if inUse}
              <Text size="sm" tone="secondary">It can’t be deleted while it is the access of {usesText(uses)}. Give them other access first, so no row is left without a group.</Text>
            {/if}
          </Stack>
        {/if}
      </Section>
    </Stack>
  </Page>
{:else}
  <Page center>
    <EmptyState icon={UsersRound} title="Group not found"><Button href="/access/groups">All groups</Button></EmptyState>
  </Page>
{/if}

<style>
  .describe {
    display: flex;
    min-width: 0;
    padding: var(--space-1-5);
    margin-inline: calc(var(--space-1-5) * -1);
    border-radius: var(--radius-md);
    text-align: start;
  }

  .describe:hover {
    background: var(--color-fill);
  }
</style>
