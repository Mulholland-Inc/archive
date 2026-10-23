<script lang="ts">
  import { Page, PageHeader, Card, KeyValue, Button, Table, Text, Row, Stack, Dialog, Field, Input, Select, Checkbox, Code, Notice, Avatar, Spacer, Columns } from '../lib';
  import { api, passkeyRegister, passkeysSupported } from '../lib/api';
  import { session, reloadSpaces, signOut, spaceLabel } from '../lib/session.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { humanize, formatDate, relative } from '../lib/spec';
  import { toast, fail } from '../lib/toast.svelte';
  import type { Member, Token } from '../lib/types';

  setCrumbs([{ label: 'Settings' }]);

  let tokens = $state<Token[]>([]);
  let passkeyBusy = $state(false);

  let newSpace = $state(false);
  let spaceName = $state('');
  let spaceError = $state('');
  let spaceBusy = $state(false);

  let membersOf = $state<string | null>(null);
  let members = $state<Member[]>([]);
  let inviteEmail = $state('');
  let inviteRole = $state('editor');
  let inviteBusy = $state(false);
  let inviteError = $state('');

  let newToken = $state(false);
  let tokenName = $state('');
  let tokenFull = $state(false);
  let tokenExpires = $state('2160h');
  let tokenSpaces = $state<{ [space: string]: string }>({});
  let tokenBusy = $state(false);
  let tokenError = $state('');
  let created = $state<Token | null>(null);

  const roles = [
    { id: 'viewer', label: 'Viewer', trailing: 'Reads' },
    { id: 'editor', label: 'Editor', trailing: 'Reads and writes' },
    { id: 'admin', label: 'Admin', trailing: 'Also manages members' },
    { id: 'owner', label: 'Owner', trailing: 'Everything' }
  ];

  async function loadTokens() {
    tokens = (await api.tokens().catch(() => [])) ?? [];
  }
  loadTokens();

  async function addPasskey() {
    passkeyBusy = true;
    try {
      const r = await passkeyRegister();
      toast(`Added a passkey. You have ${r.passkeys}.`, 'success');
    } catch (e) {
      if ((e as DOMException).name !== 'NotAllowedError') fail(e);
    } finally {
      passkeyBusy = false;
    }
  }

  async function createSpace(e?: SubmitEvent) {
    e?.preventDefault();
    if (!/^[a-z0-9][a-z0-9_]{0,31}$/.test(spaceName)) {
      spaceError = 'Use up to 32 lowercase letters, digits and underscores.';
      return;
    }
    spaceBusy = true;
    try {
      await api.addSpace(spaceName);
      await reloadSpaces();
      toast(`Created ${humanize(spaceName)}`, 'success');
      newSpace = false;
    } catch (err) {
      spaceError = (err as Error).message;
    } finally {
      spaceBusy = false;
    }
  }

  async function openMembers(space: string) {
    membersOf = space;
    members = [];
    inviteEmail = '';
    inviteError = '';
    try {
      members = (await api.members(space)) ?? [];
    } catch (e) {
      fail(e);
    }
  }

  async function grant(email: string, role: string) {
    if (!membersOf) return;
    inviteBusy = true;
    inviteError = '';
    try {
      members = (await api.setMember(membersOf, email, role)) ?? [];
      if (email === inviteEmail) inviteEmail = '';
      toast(role === 'none' ? `Removed ${email}` : `${email} is now ${role}`, 'success');
    } catch (e) {
      inviteError = (e as Error).message;
    } finally {
      inviteBusy = false;
    }
  }

  function openToken() {
    tokenName = '';
    tokenFull = false;
    tokenExpires = '2160h';
    tokenSpaces = Object.fromEntries(session.spaces.map((s) => [s.name, s.id === session.home ? 'editor' : '']));
    tokenError = '';
    created = null;
    newToken = true;
  }

  async function createToken(e?: SubmitEvent) {
    e?.preventDefault();
    if (!tokenName.trim()) {
      tokenError = 'Name the token after what will use it, like Claude or Terraform.';
      return;
    }
    const spaces = Object.fromEntries(Object.entries(tokenSpaces).filter(([, r]) => r));
    if (!tokenFull && !Object.keys(spaces).length) {
      tokenError = 'Give the token access to at least one space.';
      return;
    }
    tokenBusy = true;
    tokenError = '';
    try {
      created = await api.createToken({ name: tokenName.trim(), spaces: tokenFull ? undefined : spaces, expires: tokenExpires || undefined, full: tokenFull });
      await loadTokens();
    } catch (err) {
      tokenError = (err as Error).message;
    } finally {
      tokenBusy = false;
    }
  }

  async function revoke(t: Token) {
    try {
      await api.revokeToken(t.id);
      toast(`Revoked ${t.name}`, 'success');
      await loadTokens();
    } catch (e) {
      fail(e);
    }
  }
</script>

<Page>
  <PageHeader title="Settings" />

  <Card title="Account">
    <Row gap="3">
      <Avatar name={session.actor?.name || session.actor?.email || '?'} color={1} shape="circle" size="lg" />
      <Stack gap="0">
        <Text variant="strong">{session.actor?.name || 'No name'}</Text>
        <Text variant="muted">{session.actor?.email}</Text>
      </Stack>
      <Spacer />
      {#if passkeysSupported()}<Button icon="key" loading={passkeyBusy} onclick={addPasskey}>Add a passkey</Button>{/if}
      <Button variant="ghost" icon="logout" onclick={signOut}>Sign out</Button>
    </Row>
  </Card>

  <Card title="Spaces" description="Records, files, sources and views each live in one space. Access is granted per space.">
    {#snippet actions()}<Button icon="plus" onclick={() => { spaceName = ''; spaceError = ''; newSpace = true; }}>New space</Button>{/snippet}
    <Table
      columns={[
        { id: 'name', label: 'Space' },
        { id: 'level', label: 'Your role' },
        { id: 'actions', label: '', width: 'narrow' }
      ]}
      rows={session.spaces}
      key={(s) => s.id}
    >
      {#snippet cell(s, c)}
        {#if c === 'name'}
          <Text variant="strong">{spaceLabel(s.id)}</Text>
        {:else if c === 'level'}
          <Text>{humanize(s.level)}</Text>
        {:else if ['admin', 'owner'].includes(s.level)}
          <Button variant="ghost" icon="users" onclick={() => openMembers(s.name)}>Members</Button>
        {/if}
      {/snippet}
    </Table>
  </Card>

  <Card title="API tokens" description="For scripts, Terraform and AI assistants. A token can only narrow your own access, never widen it.">
    {#snippet actions()}<Button icon="plus" onclick={openToken}>New token</Button>{/snippet}
    <Stack gap="3">
      <Text variant="small">Connect an MCP client to {location.origin}/mcp with a token as the bearer.</Text>
      {#if tokens.length}
        <Table
          columns={[
            { id: 'name', label: 'Name' },
            { id: 'access', label: 'Access' },
            { id: 'expires', label: 'Expires' },
            { id: 'actions', label: '', width: 'narrow' }
          ]}
          rows={tokens}
          key={(t) => t.id}
        >
          {#snippet cell(t, c)}
            {#if c === 'name'}
              <Text variant="strong">{t.name || 'Unnamed'}</Text>
            {:else if c === 'access'}
              <Text variant="muted">{t.full ? 'Everything you can do' : Object.entries(t.spaces ?? {}).map(([s, r]) => `${spaceLabel(s) || humanize(s)} as ${r}`).join(', ')}</Text>
            {:else if c === 'expires'}
              <Text variant="muted">{t.expires_at ? formatDate(t.expires_at) : 'Never'}</Text>
            {:else}
              <Button variant="ghost" icon="trash" onclick={() => revoke(t)}>Revoke</Button>
            {/if}
          {/snippet}
        </Table>
      {:else}
        <Text variant="muted">No tokens yet.</Text>
      {/if}
    </Stack>
  </Card>
</Page>

<Dialog bind:open={newSpace} title="New space" description="Use spaces for a household, a team or a project. You become its owner.">
  <form onsubmit={createSpace} novalidate>
    <Field label="Name" hint="Lowercase letters, digits and underscores." error={spaceError}>
      <Input bind:value={spaceName} placeholder="household" mono invalid={!!spaceError} autofocus label="Space name" />
    </Field>
  </form>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (newSpace = false)}>Cancel</Button>
    <Button variant="primary" loading={spaceBusy} onclick={() => createSpace()}>Create space</Button>
  {/snippet}
</Dialog>

<Dialog open={!!membersOf} title={`Members of ${spaceLabel(membersOf ?? '')}`} size="lg" onclose={() => (membersOf = null)}>
  <Stack gap="2">
    {#each members as m (m.email)}
      <Row gap="3">
        <Avatar name={m.name || m.email} color={2} shape="circle" />
        <Stack gap="0">
          <Text>{m.name || m.email}</Text>
          {#if m.name}<Text variant="small">{m.email}</Text>{/if}
        </Stack>
        <Spacer />
        <Select value={m.level} options={[...roles, { id: 'none', label: 'Remove access', danger: true, separator: true }]} onchange={(r) => grant(m.email, r)} />
      </Row>
    {/each}
  </Stack>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      grant(inviteEmail.trim(), inviteRole);
    }}
    novalidate
  >
    <Stack gap="2">
      <Columns count={2} trailing>
        <Field label="Add someone by email" error={inviteError}>
          <Input bind:value={inviteEmail} type="email" placeholder="name@example.com" invalid={!!inviteError} label="Email" />
        </Field>
        <Field label="Role">
          <Select variant="field" bind:value={inviteRole} options={roles} label="Role" />
        </Field>
        <Button type="submit" size="lg" loading={inviteBusy} disabled={!inviteEmail.trim()}>Add</Button>
      </Columns>
      <Text variant="small">They need an Archive account first; an instance admin creates it with archive user add.</Text>
    </Stack>
  </form>
</Dialog>

<Dialog bind:open={newToken} title={created ? 'Copy your token' : 'New API token'} size="lg">
  {#if created}
    <Notice tone="warning">This is the only time Archive shows this token. Store it somewhere safe.</Notice>
    <Code value={created.token ?? ''} />
  {:else}
    <form onsubmit={createToken} novalidate>
      <Stack gap="4">
        {#if tokenError}<Notice tone="danger">{tokenError}</Notice>{/if}
        <Field label="Name">
          <Input bind:value={tokenName} placeholder="Claude" autofocus label="Token name" />
        </Field>
        <Row gap="2"><Checkbox checked={tokenFull} label="Full access" onchange={(v) => (tokenFull = v)} /><Text>Full access, including spaces, members and tokens, as Terraform needs</Text></Row>
        {#if !tokenFull}
          <Stack gap="2">
            {#each session.spaces as s (s.id)}
              <Row gap="3">
                <Text>{spaceLabel(s.id)}</Text>
                <Spacer />
                <Select
                  bind:value={tokenSpaces[s.name]}
                  options={[{ id: '', label: 'No access' }, ...roles.filter((r) => ['viewer', 'editor', 'admin', 'owner'].indexOf(r.id) <= ['viewer', 'editor', 'admin', 'owner'].indexOf(s.level))]}
                />
              </Row>
            {/each}
          </Stack>
        {/if}
        <Field label="Expires">
          <Select
            variant="field"
            bind:value={tokenExpires}
            options={[
              { id: '720h', label: 'In 30 days' },
              { id: '2160h', label: 'In 90 days' },
              { id: '8760h', label: 'In a year' },
              { id: '', label: 'Never' }
            ]}
            label="Expires"
          />
        </Field>
      </Stack>
    </form>
  {/if}
  {#snippet footer()}
    {#if created}
      <Button variant="primary" onclick={() => (newToken = false)}>Done</Button>
    {:else}
      <Button variant="ghost" onclick={() => (newToken = false)}>Cancel</Button>
      <Button variant="primary" loading={tokenBusy} onclick={() => createToken()}>Create token</Button>
    {/if}
  {/snippet}
</Dialog>
