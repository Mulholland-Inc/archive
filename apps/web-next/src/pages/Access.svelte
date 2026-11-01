<script>
  import { UserPlus, Ellipsis, Mail, UserMinus, Shield, SearchX, Share as ShareIcon, Link, Ban, Settings2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Tabs from '../components/Tabs.svelte';
  import Stack from '../components/Stack.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Select from '../components/Select.svelte';
  import Table from '../components/Table.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Text from '../components/Text.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import Spacer from '../components/Spacer.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import InviteDialog from '../components/InviteDialog.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import Section from '../components/Section.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, seedOf, removeMember, invite, shareItem, shareKinds, permissions, isExternal, openShare, stopSharing, removeGuest, copyLink } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';
  import { route, go } from '../lib/router.svelte.js';

  const focus = ws().members.find((m) => m.id === route.parts[1]);
  let tab = $state(focus?.status === 'invited' ? 'invitations' : 'members');
  let query = $state(focus?.name ?? '');
  let roleFilter = $state('all');
  let kindFilter = $state('all');
  let inviting = $state(false);

  const roles = [
    { value: 'viewer', label: 'Viewer' },
    { value: 'editor', label: 'Editor' },
    { value: 'owner', label: 'Owner' }
  ];
  const roleFilters = [{ value: 'all', label: 'All roles' }, ...roles];
  const kindFilters = [{ value: 'all', label: 'All items' }, ...Object.entries(shareKinds).map(([value, label]) => ({ value, label: `${label}s` }))];
  const permissionLabel = (v) => permissions.find((p) => p.value === v)?.label ?? v;
  const others = (names, n) => (names.length > n ? `${names.slice(0, n).join(', ')} and ${names.length - n} ${names.length - n === 1 ? 'other' : 'others'}` : names.join(', '));
  const firstName = (name) => name.split(' ')[0];
  function accessOf(share) {
    const kinds = [...new Set(share.people.map((p) => p.permission))];
    if (!kinds.length) return share.link === 'view' ? 'Can view' : '';
    return kinds.length === 1 ? permissionLabel(kinds[0]) : 'Mixed';
  }

  const q = $derived(query.trim().toLowerCase());
  const matches = (...fields) => !q || fields.some((f) => f?.toLowerCase().includes(q));

  const active = $derived(ws().members.filter((m) => m.status === 'active'));
  const invited = $derived(ws().members.filter((m) => m.status === 'invited'));
  const memberRows = $derived((tab === 'invitations' ? invited : active).filter((m) => (roleFilter === 'all' || m.role === roleFilter) && matches(m.name, m.email)));

  const shares = $derived(ws().shares.map((s) => ({ share: s, id: s.id, item: shareItem(s.kind, s.target) })).filter((r) => r.item));
  const shareRows = $derived(shares.filter((r) => (kindFilter === 'all' || r.share.kind === kindFilter) && matches(r.item.name, ...r.share.people.flatMap((p) => [p.name, p.email]))));

  const guests = $derived.by(() => {
    const out = new Map();
    for (const { share, item } of shares) {
      for (const p of share.people) {
        if (!isExternal(p.email)) continue;
        if (!out.has(p.email)) out.set(p.email, { id: p.email, name: p.name, email: p.email, tone: p.tone, items: [] });
        out.get(p.email).items.push({ id: share.id, name: item.name, permission: p.permission });
      }
    }
    return [...out.values()];
  });
  const guestRows = $derived(guests.filter((g) => matches(g.name, g.email, ...g.items.map((i) => i.name))));

  const tabs = $derived([
    { value: 'members', label: 'Members', count: active.length },
    { value: 'shares', label: 'Shares', count: shares.length },
    { value: 'invitations', label: 'Invitations', count: invited.length }
  ]);

  const memberColumns = $derived([
    { key: 'name', label: 'Name' },
    { key: 'email', label: 'Email', muted: true, width: 'lg' },
    { key: 'role', label: 'Role', width: 'sm' },
    { key: 'active', label: tab === 'invitations' ? 'Status' : 'Last active', muted: true, width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ]);
  const shareColumns = $derived([
    { key: 'item', label: 'Item' },
    { key: 'kind', label: 'Kind', muted: true, width: 'sm' },
    { key: 'people', label: 'Shared with', width: 'md' },
    { key: 'access', label: 'Access', muted: true, width: 'sm' },
    ...(shareRows.some((r) => r.share.link === 'view') ? [{ key: 'link', label: 'Link', muted: true, width: 'md' }] : []),
    { key: 'date', label: 'Shared', muted: true, width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ]);
  const guestColumns = [
    { key: 'name', label: 'Name' },
    { key: 'email', label: 'Email', muted: true, width: 'lg' },
    { key: 'items', label: 'Has access to', muted: true, width: 'lg' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ];

  function changeRole(row, role) {
    row.role = role;
    toast(`${row.name} is now ${role === 'owner' ? 'an' : 'a'} ${role}`);
  }
</script>

{#snippet person(row)}
  <Stack direction="row" gap="3" align="center">
    <Avatar name={row.name} seed={seedOf(row)} size="md" />
    <Text weight="medium" truncate>{row.name}</Text>
  </Stack>
{/snippet}

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <StickyHeader title="Access">
    {#snippet search()}
      <SearchField bind:value={query} placeholder={tab === 'shares' ? 'Search items and people' : 'Search by name or email'} />
    {/snippet}
    {#snippet actions()}<Button variant="primary" icon={UserPlus} onclick={() => (inviting = true)}>Invite</Button>{/snippet}
  </StickyHeader>

  <Stack direction="row" gap="2" align="center" wrap>
    <Tabs bind:value={tab} items={tabs} label="Access" variant="pill" />
    <Spacer />
    {#if tab === 'shares'}
      <Select bind:value={kindFilter} options={kindFilters} variant="ghost" label="Kind" placement="bottom-end" />
    {:else}
      <Select bind:value={roleFilter} options={roleFilters} variant="ghost" label="Role" placement="bottom-end" />
    {/if}
  </Stack>

  {#if tab === 'shares'}
    {#if shareRows.length}
      <Table columns={shareColumns} rows={shareRows} label="Shares" cards>
        {#snippet cell(row, col)}
          {#if col.key === 'item'}
            <Text weight="medium" truncate>{row.item.name}</Text>
          {:else if col.key === 'kind'}
            {shareKinds[row.share.kind]}
          {:else if col.key === 'people'}
            {#if row.share.people.length}
              <Stack direction="row" gap="2" align="center">
                <AvatarGroup people={row.share.people} size="md" max={3} />
                <Text tone="secondary" truncate>{others(row.share.people.map((p) => firstName(p.name)), 1)}</Text>
              </Stack>
            {/if}
          {:else if col.key === 'access'}
            {accessOf(row.share)}
          {:else if col.key === 'link'}
            {row.share.link === 'view' ? 'Anyone with the link' : ''}
          {:else if col.key === 'date'}
            {row.share.date}
          {:else if col.key === 'menu'}
            <Menu placement="bottom-end" width="sm" label="Share actions">
              {#snippet trigger(props)}<IconButton icon={Ellipsis} label={`Actions for ${row.item.name}`} size="sm" tooltip={false} {...props} />{/snippet}
              <MenuItem icon={Settings2} onselect={() => openShare(row.share.kind, row.share.target)}>Manage sharing</MenuItem>
              <MenuItem icon={Link} onselect={() => copyLink(row.item.path)}>Copy link</MenuItem>
              <MenuSeparator />
              <MenuItem icon={Ban} danger onselect={() => stopSharing(row.share.id)}>Stop sharing</MenuItem>
            </Menu>
          {/if}
        {/snippet}
      </Table>
    {:else}
      <EmptyState icon={q || kindFilter !== 'all' ? SearchX : ShareIcon} title={q || kindFilter !== 'all' ? 'Nothing matches that search' : 'Nothing is shared yet'} size="sm" />
    {/if}
  {:else}
    {#if memberRows.length}
      <Table columns={memberColumns} rows={memberRows} label="Members" cards>
        {#snippet cell(row, col)}
          {#if col.key === 'name'}
            {@render person(row)}
          {:else if col.key === 'email'}
            {row.email}
          {:else if col.key === 'role'}
            <Select value={row.role} options={roles} size="sm" variant="ghost" label={`Role for ${row.name}`} onchange={(v) => changeRole(row, v)} />
          {:else if col.key === 'active'}
            {row.status === 'invited' ? 'Invited' : row.active}
          {:else if col.key === 'menu'}
            <Menu placement="bottom-end" width="sm" label="Member actions">
              {#snippet trigger(props)}<IconButton icon={Ellipsis} label={`Actions for ${row.name}`} size="sm" tooltip={false} {...props} />{/snippet}
              {#if row.status === 'invited'}
                <MenuItem icon={Mail} onselect={() => toast(`Invitation resent to ${row.email}`)}>Resend invite</MenuItem>
              {/if}
              {#if row.role !== 'owner'}
                <MenuItem icon={Shield} onselect={() => changeRole(row, 'owner')}>Make owner</MenuItem>
              {/if}
              <MenuSeparator />
              <MenuItem icon={UserMinus} danger onselect={() => removeMember(row.id)}>{row.status === 'invited' ? 'Revoke invitation' : 'Remove from vault'}</MenuItem>
            </Menu>
          {/if}
        {/snippet}
      </Table>
    {:else}
      <EmptyState icon={SearchX} title={tab === 'invitations' && !query ? 'No pending invitations' : 'No one matches that search'} size="sm" />
    {/if}
    {#if tab === 'members' && roleFilter === 'all' && guestRows.length}
      <Section title="Guests">
        <Table columns={guestColumns} rows={guestRows} label="Guests" cards>
          {#snippet cell(row, col)}
            {#if col.key === 'name'}
              {@render person(row)}
            {:else if col.key === 'email'}
              {row.email}
            {:else if col.key === 'items'}
              {others(row.items.map((i) => i.name), 1)}
            {:else if col.key === 'menu'}
              <Menu placement="bottom-end" width="sm" label="Guest actions">
                {#snippet trigger(props)}<IconButton icon={Ellipsis} label={`Actions for ${row.name}`} size="sm" tooltip={false} {...props} />{/snippet}
                <MenuItem icon={UserPlus} onselect={() => invite({ emails: [row.email], role: 'viewer' })}>Invite to vault</MenuItem>
                <MenuSeparator />
                <MenuItem icon={UserMinus} danger onselect={() => removeGuest(row.email)}>Remove all access</MenuItem>
              </Menu>
            {/if}
          {/snippet}
        </Table>
      </Section>
    {/if}
  {/if}
</Page>

<InviteDialog bind:open={inviting} />
