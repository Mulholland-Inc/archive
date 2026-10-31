<script>
  import { UserPlus, Ellipsis, Mail, UserMinus, Shield, Plus, SearchX, Users as UsersIcon } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Tabs from '../components/Tabs.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import Card from '../components/Card.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Select from '../components/Select.svelte';
  import Table from '../components/Table.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Icon from '../components/Icon.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import Chip from '../components/Chip.svelte';
  import Text from '../components/Text.svelte';
  import Heading from '../components/Heading.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Spacer from '../components/Spacer.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import InviteDialog from '../components/InviteDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { members as seed, groups } from '../lib/data.js';
  import { toast } from '../lib/toast.svelte.js';

  let members = $state(seed.map((m) => ({ ...m })));
  let tab = $state('members');
  let query = $state('');
  let roleFilter = $state('all');
  let inviting = $state(false);

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' },
    { value: 'owner', label: 'Owner' }
  ];
  const filters = [{ value: 'all', label: 'All roles' }, ...roles];

  const active = $derived(members.filter((m) => m.status === 'active'));
  const invited = $derived(members.filter((m) => m.status === 'invited'));

  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const base = tab === 'invitations' ? invited : active;
    return base.filter((m) => (roleFilter === 'all' || m.role === roleFilter) && (!q || m.name.toLowerCase().includes(q) || m.email.toLowerCase().includes(q)));
  });

  const tabs = $derived([
    { value: 'members', label: 'Members', count: active.length },
    { value: 'groups', label: 'Groups', count: groups.length },
    { value: 'invitations', label: 'Invitations', count: invited.length }
  ]);

  const columns = [
    { key: 'name', label: 'Name' },
    { key: 'role', label: 'Role', width: 'sm' },
    { key: 'groups', label: 'Groups' },
    { key: 'active', label: 'Last active', muted: true, width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ];

  function remove(m) {
    members = members.filter((x) => x.id !== m.id);
    toast(`${m.name} removed`, { action: { label: 'Undo', run: () => members.push(m) } });
  }
</script>

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Users">
    {#snippet actions()}<Button variant="primary" icon={UserPlus} onclick={() => (inviting = true)}>Invite</Button>{/snippet}
    <Tabs bind:value={tab} items={tabs} label="Users" />
  </PageHeader>

  {#if tab === 'groups'}
    <Grid min="card" gap="4">
      {#each groups as g (g.id)}
        {@const names = members.filter((m) => m.groups.includes(g.name)).map((m) => m.name)}
        <Card padding="lg">
          <Stack direction="row" align="center" gap="3">
            <Heading level={3} size="md" serif={false}>{g.name}</Heading>
            <Spacer />
            <Text size="sm" tone="tertiary">{names.length} {names.length === 1 ? 'member' : 'members'}</Text>
          </Stack>
          <Stack direction="row" align="center" gap="2">
            <AvatarGroup {names} size="sm" max={5} />
            <Spacer />
            <Button size="sm" variant="ghost" icon={Plus}>Add</Button>
          </Stack>
        </Card>
      {/each}
      <Card padding="lg" variant="filled" onclick={() => toast('Group created')}>
        <Stack direction="row" align="center" gap="2">
          <Icon icon={Plus} size="md" />
          <Text tone="secondary" weight="medium">New group</Text>
        </Stack>
      </Card>
    </Grid>
  {:else}
    <Stack direction="row" gap="2" align="center" wrap>
      <SearchField bind:value={query} placeholder="Search by name or email" />
      <Spacer />
      <Select bind:value={roleFilter} options={filters} variant="ghost" label="Role" placement="bottom-end" />
    </Stack>
    {#if rows.length}
      <Table {columns} {rows} label="Members">
        {#snippet cell(row, col)}
          {#if col.key === 'name'}
            <Stack direction="row" gap="3" align="center">
              <Avatar name={row.name} tone={row.tone} size="md" />
              <Stack gap="0">
                <Text weight="medium">{row.name}</Text>
                <Text size="sm" tone="tertiary">{row.email}</Text>
              </Stack>
            </Stack>
          {:else if col.key === 'role'}
            <Select bind:value={row.role} options={roles} size="sm" variant="ghost" label={`Role for ${row.name}`} onchange={(v) => toast(`${row.name} is now ${v === 'owner' ? 'an' : 'a'} ${v}`)} />
          {:else if col.key === 'groups'}
            <Stack direction="row" gap="1" wrap>
              {#each row.groups as g (g)}<Chip size="sm">{g}</Chip>{/each}
              {#if !row.groups.length}<Text size="sm" tone="tertiary">None</Text>{/if}
            </Stack>
          {:else if col.key === 'active'}
            {#if row.status === 'invited'}<StatusBadge status="invited" />{:else}{row.active}{/if}
          {:else if col.key === 'menu'}
            <Menu placement="bottom-end" width="sm" label="Member actions">
              {#snippet trigger(props)}<IconButton icon={Ellipsis} label="More" size="sm" tooltip={false} {...props} />{/snippet}
              {#if row.status === 'invited'}
                <MenuItem icon={Mail} onselect={() => toast(`Invitation resent to ${row.email}`)}>Resend invite</MenuItem>
              {/if}
              <MenuItem icon={Shield} onselect={() => { row.role = 'owner'; toast(`${row.name} is now an owner`); }}>Make owner</MenuItem>
              <MenuSeparator />
              <MenuItem icon={UserMinus} danger onselect={() => remove(row)}>Remove from workspace</MenuItem>
            </Menu>
          {/if}
        {/snippet}
      </Table>
    {:else}
      <EmptyState icon={SearchX} title="No one matches that search" size="sm" />
    {/if}
  {/if}
</Page>

<InviteDialog bind:open={inviting} />
