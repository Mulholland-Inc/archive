<script>
  import { UserPlus, SearchX, UsersRound, Plus, Pencil, Trash2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Tabs from '../components/Tabs.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Table from '../components/Table.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Text from '../components/Text.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import Spacer from '../components/Spacer.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import InviteDialog from '../components/InviteDialog.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import GroupDialog from '../components/GroupDialog.svelte';
  import { ws, seedOf, setRole, members, groups, groupById, setGroup, atLeast, ROLES } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  members.use();
  groups.use();
  import { route, go } from '../lib/router.svelte.js';
  import { removeMemberConfirm, renameGroupPrompt, deleteGroupConfirm } from '../lib/menus.js';

  const focus = ws().members.find((m) => m.id === route.parts[1]);
  let tab = $state(route.parts[1] === 'groups' ? 'groups' : focus?.status === 'invited' ? 'invitations' : 'members');
  let query = $state(focus?.name ?? '');
  let roleFilter = $state('all');
  let inviting = $state(false);
  let creating = $state(false);
  const manages = $derived(atLeast('manager'));
  const mayChange = (row) => manages && row.role !== 'owner' && atLeast(row.role);
  const assignable = (row) => ROLES.filter((r) => atLeast(r.value) && (r.value !== 'owner' || row.status === 'active'));
  const roleFilters = [{ value: 'all', label: 'All roles' }, ...ROLES];
  const q = $derived(query.trim().toLowerCase());
  const matches = (...fields) => !q || fields.some((f) => f?.toLowerCase().includes(q));

  const active = $derived(ws().members.filter((m) => m.status === 'active'));
  const invited = $derived(ws().members.filter((m) => m.status === 'invited'));
  const memberRows = $derived((tab === 'invitations' ? invited : active).filter((m) => (roleFilter === 'all' || m.role === roleFilter) && matches(m.name, m.email)));

  const groupRows = $derived(
    ws()
      .groups.map((g) => ({ group: g, id: g.id, people: ws().members.filter((m) => m.group === g.id) }))
      .filter((r) => matches(r.group.name, r.group.description, ...r.people.flatMap((p) => [p.name, p.email])))
  );

  const tabs = $derived([
    { value: 'members', label: 'Members', count: active.length },
    { value: 'groups', label: 'Groups', count: ws().groups.length },
    { value: 'invitations', label: 'Invitations', count: invited.length }
  ]);

  const memberColumns = $derived([
    { key: 'name', label: 'Name' },
    { key: 'email', label: 'Email', muted: true, width: 'lg' },
    { key: 'role', label: 'Role', width: 'sm', mobile: 'trailing' },
    { key: 'group', label: 'Group', width: 'md', mobile: 'hidden' },
    { key: 'active', label: tab === 'invitations' ? 'Status' : 'Last active', muted: true, width: 'sm', mobile: 'hidden' },
    ...(manages ? [{ key: 'actions', label: '', width: 'sm', align: 'end' }] : [])
  ]);
  const groupColumns = $derived([
    { key: 'name', label: 'Name' },
    { key: 'description', label: 'Description', muted: true, width: 'lg', mobile: 'hidden' },
    { key: 'people', label: 'Members', width: 'lg' },
    ...(manages ? [{ key: 'actions', label: '', width: 'sm', align: 'end' }] : [])
  ]);
  const groupOptions = $derived([{ value: '', label: 'None' }, ...ws().groups.map((g) => ({ value: g.id, label: g.name }))]);
  function changeGroup(row, group) {
    const m = ws().members.find((x) => x.id === row.id);
    if (m) setGroup(m, group || null);
  }
  async function changeRole(row, role) {
    const m = ws().members.find((x) => x.id === row.id);
    if (!m || (role === 'owner' && !(await confirmDialog({ title: `Transfer ${ws().name} to ${row.name}?`, body: 'You become an engineer.', confirm: 'Transfer' })))) return;
    setRole(m, role);
  }
</script>

{#snippet avatar(row)}<Avatar name={row.name} seed={seedOf(row)} size="md" />{/snippet}

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <StickyHeader title="Access" art="bg-access">
    {#snippet search()}
      <SearchField bind:value={query} placeholder={tab === 'groups' ? 'Search groups and members' : 'Search by name or email'} />
    {/snippet}
    {#snippet actions()}
      {#if tab === 'groups' && manages}
        <Button variant="primary" icon={Plus} onclick={() => (creating = true)}>New group</Button>
      {:else if manages}
        <Button variant="primary" icon={UserPlus} onclick={() => (inviting = true)}>Invite</Button>
      {/if}
    {/snippet}
  </StickyHeader>

  <Stack direction="row" gap="2" align="center" wrap>
    <Tabs bind:value={tab} items={tabs} label="Access" variant="pill" />
    <Spacer />
    {#if tab !== 'groups'}
      <Menu bind:value={roleFilter} options={roleFilters} variant="ghost" label="Role" />
    {/if}
  </Stack>

  {#if tab === 'groups'}
    {#if groupRows.length}
      <Table columns={groupColumns} rows={groupRows} label="Groups" cards onrowclick={(row) => go(`/access/groups/${row.id}`)}>
        {#snippet cell(row, col)}
          {#if col.key === 'name'}
            <Text weight="medium" truncate>{row.group.name}</Text>
          {:else if col.key === 'description'}
            {row.group.description}
          {:else if col.key === 'people'}
            {#if row.people.length}
              <Stack direction="row" gap="2" align="center">
                <AvatarGroup people={row.people} size="md" max={3} />
                <Text tone="secondary" truncate>{row.people.length === 1 ? '1 member' : `${row.people.length} members`}</Text>
              </Stack>
            {:else}
              <Text tone="tertiary">No members</Text>
            {/if}
          {:else if col.key === 'actions'}
            <span class="actions">
              <IconButton icon={Pencil} label={`Rename ${row.group.name}`} size="sm" onclick={() => renameGroupPrompt(row.group)} />
              <IconButton icon={Trash2} label={`Delete ${row.group.name}`} size="sm" onclick={() => deleteGroupConfirm(row.group)} />
            </span>
          {/if}
        {/snippet}
      </Table>
    {:else if q}
      <EmptyState icon={SearchX} title="No group matches that search" size="sm" />
    {:else}
      <EmptyState icon={UsersRound} title={manages ? 'No groups yet. A row limited to a group is seen only by its members and by managers and up' : 'No groups yet'} size="sm">
        {#if manages}<Button icon={Plus} onclick={() => (creating = true)}>New group</Button>{/if}
      </EmptyState>
    {/if}
  {:else}
    {#if memberRows.length}
      <Table columns={memberColumns} rows={memberRows} label="Members" leading={avatar} cards>
        {#snippet cell(row, col)}
          {#if col.key === 'name'}
            <Text weight="medium" truncate>{row.name}</Text>
          {:else if col.key === 'email'}
            {row.email}
          {:else if col.key === 'role'}
            <span class="pick">
              {#if mayChange(row)}
                <Menu value={row.role} options={assignable(row)} size="sm" variant="ghost" label={`Role for ${row.name}`} onchange={(v) => changeRole(row, v)} />
              {:else}
                {ROLES.find((r) => r.value === row.role)?.label}
              {/if}
            </span>
          {:else if col.key === 'group'}
            <span class="pick">
              {#if manages}
                <Menu value={row.group ?? ''} options={groupOptions} size="sm" variant="ghost" label={`Group for ${row.name}`} onchange={(v) => changeGroup(row, v)} />
              {:else}
                {groupById(row.group)?.name ?? 'None'}
              {/if}
            </span>
          {:else if col.key === 'active'}
            {row.status === 'invited' ? 'Invited' : row.active}
          {:else if col.key === 'actions'}
            <span class="actions">
              {#if mayChange(row)}<IconButton icon={Trash2} label={row.status === 'invited' ? `Revoke ${row.name}’s invitation` : `Remove ${row.name}`} size="sm" onclick={() => removeMemberConfirm(row)} />{/if}
            </span>
          {/if}
        {/snippet}
      </Table>
    {:else}
      <EmptyState icon={SearchX} title={tab === 'invitations' && !query ? 'No pending invitations' : 'No one matches that search'} size="sm" />
    {/if}
  {/if}
</Page>

<InviteDialog bind:open={inviting} />
<GroupDialog bind:open={creating} />

<style>
  .actions {
    display: inline-flex;
    justify-content: flex-end;
    gap: var(--space-1);
  }

  .pick :global(.trigger) {
    margin-inline-start: calc(-1 * var(--space-2));
  }

</style>
