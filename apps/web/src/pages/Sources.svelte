<script lang="ts">
  import { Page, PageHeader, Button, IconButton, Table, Text, Row, Icon, EmptyState, Notice, Skeleton, Popover, Menu, Dialog, Field, Select, Stack } from '../lib';
  import { api } from '../lib/api';
  import { session, canWrite, spaceName } from '../lib/session.svelte';
  import { query, setQuery } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { humanize, relative, objectTypes } from '../lib/spec';
  import { kindLabel, kindIcon, scheduleLabel, statusLabel, statusTone, schedules } from '../lib/sources';
  import { toast, fail } from '../lib/toast.svelte';
  import type { Source } from '../lib/types';
  import SourceDialog from './parts/SourceDialog.svelte';

  setCrumbs([{ label: 'Sources' }]);

  let sources = $state<Source[]>([]);
  let loading = $state(true);
  let error = $state('');
  let adding = $state(false);
  let running = $state<{ [id: string]: boolean }>({});
  let menus = $state<{ [id: string]: boolean }>({});
  let editing = $state<Source | null>(null);
  let editType = $state('');
  let editSchedule = $state('');
  let saving = $state(false);
  let removing = $state<Source | null>(null);
  let removeBusy = $state(false);
  const callbackError = query().get('error');

  async function load() {
    try {
      sources = (await api.sources()) ?? [];
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  load();

  const connected = query().get('connected');
  if (connected) {
    toast(`Connected ${connected}. The first sync starts now.`, 'success');
    setQuery({ connected: undefined });
  }

  $effect(() => {
    if (!sources.some((s) => s.status === 'running')) return;
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  });

  async function run(s: Source) {
    running[s.id] = true;
    try {
      const r = await api.runSource(s.id);
      if (r.error) toast(`${s.name}: ${r.error}`, 'danger');
      else toast(`${s.name}: fetched ${r.fetched}, ${r.new} new, ${r.mapped} read into records`, 'success');
      await load();
    } catch (e) {
      fail(e);
    } finally {
      running[s.id] = false;
    }
  }

  async function saveEdit() {
    if (!editing) return;
    saving = true;
    try {
      await api.patchSource(editing.id, { type: editType, schedule: editSchedule });
      toast(`Updated ${editing.name}`, 'success');
      editing = null;
      await load();
    } catch (e) {
      fail(e);
    } finally {
      saving = false;
    }
  }

  async function remove() {
    if (!removing) return;
    removeBusy = true;
    try {
      await api.removeSource(removing.id);
      toast(`Removed ${removing.name}. Its records stay.`, 'success');
      removing = null;
      await load();
    } catch (e) {
      fail(e);
    } finally {
      removeBusy = false;
    }
  }

  const visible = $derived(session.space ? sources.filter((s) => spaceName(s.space) === session.space) : sources);
</script>

<Page width="full">
  <PageHeader title="Sources" description="Places Archive syncs from. New items land in raw storage, then intake reads them into records.">
    {#snippet actions()}
      {#if canWrite()}<Button variant="primary" icon="plus" onclick={() => (adding = true)}>Connect a source</Button>{/if}
    {/snippet}
  </PageHeader>

  {#if callbackError}<Notice tone="danger">Google sign-in did not finish: {callbackError}</Notice>{/if}

  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if loading}
    <Skeleton rows={4} />
  {:else if !visible.length}
    <EmptyState icon="plug" title="No sources yet" description="Connect Gmail, Google Drive, an IMAP mailbox, a folder or your own connector to keep Archive up to date on its own.">
      {#if canWrite()}<Button variant="primary" icon="plus" onclick={() => (adding = true)}>Connect a source</Button>{/if}
    </EmptyState>
  {:else}
    <Table
      columns={[
        { id: 'name', label: 'Source' },
        { id: 'type', label: 'Reads into' },
        { id: 'schedule', label: 'Sync' },
        { id: 'status', label: 'Status' },
        { id: 'last', label: 'Last run' },
        { id: 'actions', label: '', width: 'narrow' }
      ]}
      rows={visible}
      key={(s) => s.id}
    >
      {#snippet cell(s, c)}
        {#if c === 'name'}
          <Row gap="2"><Icon name={kindIcon(s.kind)} /><Text variant="strong">{s.name}</Text><Text variant="muted">{kindLabel(s.kind)}</Text></Row>
        {:else if c === 'type'}
          <Text variant={s.type ? 'body' : 'muted'}>{s.type ? humanize(s.type) : 'Unmapped'}</Text>
        {:else if c === 'schedule'}
          <Text>{scheduleLabel(s.schedule)}</Text>
        {:else if c === 'status'}
          <Text tone={statusTone(s.status)}>{statusLabel(s.status)}{s.error ? `: ${s.error}` : ''}</Text>
        {:else if c === 'last'}
          <Text variant="muted">{s.last_run_at ? relative(s.last_run_at) : 'Never'}</Text>
        {:else if c === 'actions' && canWrite(s.space)}
          <Row gap="1">
            <Button variant="ghost" icon="play" loading={running[s.id] || s.status === 'running'} disabled={s.status === 'needs_auth'} onclick={() => run(s)}>Run</Button>
            <Popover bind:open={menus[s.id]} align="end">
              {#snippet trigger({ toggle })}<IconButton icon="more" label={`More for ${s.name}`} onclick={toggle} />{/snippet}
              {#snippet children({ close })}
                <Menu
                  searchable={false}
                  items={[
                    { id: 'edit', label: 'Edit', icon: 'pencil' },
                    { id: 'remove', label: 'Remove', icon: 'trash', danger: true, separator: true }
                  ]}
                  onselect={(i) => {
                    close();
                    if (i.id === 'edit') {
                      editing = s;
                      editType = s.type ?? '';
                      editSchedule = s.schedule ?? '';
                    }
                    if (i.id === 'remove') removing = s;
                  }}
                />
              {/snippet}
            </Popover>
          </Row>
        {/if}
      {/snippet}
    </Table>
  {/if}
</Page>

<SourceDialog bind:open={adding} onadded={(s) => { toast(`Connected ${s.name}`, 'success'); load(); }} />

<Dialog open={!!editing} title={`Edit ${editing?.name ?? ''}`} onclose={() => (editing = null)}>
  <Stack gap="4">
    <Field label="Read into">
      <Select variant="field" bind:value={editType} options={[{ id: '', label: 'Keep them unmapped' }, ...objectTypes(session.ontology).map((t) => ({ id: t, label: humanize(t), icon: 'table' }))]} label="Read into" />
    </Field>
    <Field label="Sync">
      <Select variant="field" bind:value={editSchedule} options={schedules} label="Sync schedule" />
    </Field>
  </Stack>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (editing = null)}>Cancel</Button>
    <Button variant="primary" loading={saving} onclick={saveEdit}>Save</Button>
  {/snippet}
</Dialog>

<Dialog open={!!removing} title={`Remove ${removing?.name ?? ''}?`} description="Archive stops syncing it. Everything it already brought in stays." onclose={() => (removing = null)}>
  {#snippet children()}{/snippet}
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (removing = null)}>Cancel</Button>
    <Button variant="danger" loading={removeBusy} onclick={remove}>Remove</Button>
  {/snippet}
</Dialog>
