<script lang="ts">
  import { Page, PageHeader, Button, IconButton, Card, KeyValue, Split, Stack, Row, Text, Link, Code, Notice, Skeleton, EmptyState, Popover, Menu, Dialog, Icon } from '../lib';
  import { api } from '../lib/api';
  import { session, canWrite, spaceLabel } from '../lib/session.svelte';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { fieldsOf, humanize, recordTitle, isSecret, relative, formatTime } from '../lib/spec';
  import { refTitle, remember } from '../lib/refs.svelte';
  import { toast, fail } from '../lib/toast.svelte';
  import type { ArchiveRecord, Provenance, RecordLink } from '../lib/types';
  import FieldValue from './parts/FieldValue.svelte';
  import RecordForm from './parts/RecordForm.svelte';

  let { type, id }: { type: string; id: string } = $props();

  let record = $state<ArchiveRecord | null>(null);
  let prov = $state<Provenance[]>([]);
  let links = $state<RecordLink[]>([]);
  let missing = $state(false);
  let error = $state('');
  let reveal = $state(false);
  let editing = $state(false);
  let menu = $state(false);
  let confirm = $state(false);
  let deleting = $state(false);

  const o = $derived(session.ontology);
  const fields = $derived(fieldsOf(o, record?.type ?? type));
  const hasSecrets = $derived(!!record && Object.values(record.data).some(isSecret));
  const byField = $derived(new Map(prov.map((p) => [p.field_path.split('.')[0], p])));

  async function load() {
    try {
      const r = await api.get(type, id, reveal);
      record = r;
      remember(r);
      const [p, l] = await Promise.all([api.provenance(type, id), api.links(type, id)]);
      prov = p ?? [];
      links = l ?? [];
    } catch (e) {
      if ((e as { status?: number }).status === 404) missing = true;
      else error = (e as Error).message;
    }
  }

  $effect(() => {
    reveal;
    load();
  });

  $effect(() => {
    setCrumbs([{ label: humanize(type), href: `/records/${type}` }, { label: record ? recordTitle(o, record) : '…' }]);
  });

  const outgoing = $derived(links.filter((l) => l.from_id === id));
  const incoming = $derived(links.filter((l) => l.to_id === id && l.from_id !== id));

  async function remove() {
    if (!record) return;
    deleting = true;
    try {
      await api.remove(record.type, record.id);
      toast(`Deleted ${recordTitle(o, record)}`, 'success');
      navigate(`/records/${record.type}`, { replace: true });
    } catch (e) {
      fail(e);
    } finally {
      deleting = false;
    }
  }

  function pct(n: number) {
    return `${Math.round(n * 100)}%`;
  }
</script>

{#if missing}
  <Page>
    <EmptyState icon="search" title="This record does not exist" description="It may have been deleted, or it lives in a space you cannot see.">
      <Button onclick={() => navigate(`/records/${type}`)}>All {humanize(type).toLowerCase()} records</Button>
    </EmptyState>
  </Page>
{:else if error}
  <Page><Notice tone="danger">{error}</Notice></Page>
{:else if !record}
  <Page><Skeleton rows={10} /></Page>
{:else}
  <Page>
    <PageHeader title={recordTitle(o, record)}>
      {#snippet meta()}
        <Link href={`/records/${record!.type}`} variant="subtle">{humanize(record!.type)}</Link>
        <Text>{spaceLabel(record!.space) || 'Unknown space'}</Text>
        <Text>Updated {relative(record!.updated_at)}</Text>
      {/snippet}
      {#snippet actions()}
        {#if hasSecrets || reveal}
          <Button variant="ghost" icon={reveal ? 'eye-off' : 'eye'} onclick={() => (reveal = !reveal)}>{reveal ? 'Hide secrets' : 'Reveal secrets'}</Button>
        {/if}
        {#if canWrite(record!.space)}<Button icon="pencil" onclick={() => (editing = true)}>Edit</Button>{/if}
        <Popover bind:open={menu} align="end">
          {#snippet trigger({ toggle })}<IconButton icon="more" label="More" onclick={toggle} />{/snippet}
          {#snippet children({ close })}
            <Menu
              searchable={false}
              items={[
                { id: 'copy', label: 'Copy ID', icon: 'copy' },
                ...(canWrite(record!.space) ? [{ id: 'delete', label: 'Delete', icon: 'trash', danger: true, separator: true }] : [])
              ]}
              onselect={async (i) => {
                close();
                if (i.id === 'copy') {
                  await navigator.clipboard.writeText(record!.id).catch(() => {});
                  toast('Copied the ID', 'success');
                }
                if (i.id === 'delete') confirm = true;
              }}
            />
          {/snippet}
        </Popover>
      {/snippet}
    </PageHeader>

    <Split>
      {#snippet main()}
        <Card title="Fields">
          <KeyValue items={fields} label={([n]) => humanize(n)}>
            {#snippet value([n, f])}
              <Stack gap="1">
                <FieldValue field={f} value={record!.data[n]} />
                {#if byField.get(n) && byField.get(n)!.method !== 'manual'}
                  {@const p = byField.get(n)!}
                  <Row gap="2" wrap>
                    <Text variant="small">{pct(p.confidence)} confident, from {p.method.replace(/^intake:/, 'intake with ')}</Text>
                    {#if p.file_id}
                      <Link href={`/library/${p.file_id}${p.page ? `?page=${p.page}` : ''}`} variant="accent">{p.page ? `Page ${p.page}` : 'Source file'}</Link>
                    {/if}
                  </Row>
                {/if}
                {#if byField.get(n)?.original && byField.get(n)!.original !== String(record!.data[n])}<Text variant="small">Entered as “{byField.get(n)!.original}”</Text>{/if}
              </Stack>
            {/snippet}
          </KeyValue>
        </Card>
      {/snippet}
      {#snippet aside()}
        <Card title="Links">
          {#if !outgoing.length && !incoming.length}
            <Text variant="muted">No other records point here, and this record points nowhere.</Text>
          {:else}
            <Stack gap="2">
              {#each outgoing as l (l.id)}
                {@const to = fields.find(([n]) => n === l.kind)?.[1]?.to}
                <Row gap="2">
                  <Icon name="arrow-right" size="sm" />
                  <Text variant="muted">{humanize(l.kind)}</Text>
                  {#if to}<Link href={`/records/${to}/${l.to_id}`} variant="accent">{refTitle(to, l.to_id)}</Link>{:else}<Text>{l.to_id}</Text>{/if}
                </Row>
              {/each}
              {#each incoming as l (l.id)}
                {@const from = l.from_type}
                <Row gap="2">
                  <Icon name="arrow-left" size="sm" />
                  {#if from}
                    <Link href={`/records/${from}/${l.from_id}`} variant="accent">{refTitle(from, l.from_id)}</Link>
                    <Text variant="muted">{humanize(from).toLowerCase()} {humanize(l.kind).toLowerCase()}</Text>
                  {:else}
                    <Text>{l.from_id}</Text>
                  {/if}
                </Row>
              {/each}
            </Stack>
          {/if}
        </Card>
        <Card title="Provenance">
          {#if !prov.some((p) => p.method !== 'manual')}
            <Text variant="muted">Entered by hand or through the API; no source document.</Text>
          {:else}
            <Stack gap="2">
              {#each [...new Set(prov.filter((p) => p.method !== 'manual').map((p) => p.file_id ?? p.raw_ids[0] ?? p.method))] as src (src)}
                {@const ps = prov.filter((p) => p.method !== 'manual' && (p.file_id ?? p.raw_ids[0] ?? p.method) === src)}
                <Stack gap="1">
                  {#if ps[0].file_id}
                    <Link href={`/library/${ps[0].file_id}`} variant="accent">Source file</Link>
                  {:else}
                    <Text>{ps[0].method}</Text>
                  {/if}
                  <Text variant="small">{ps.length} {ps.length === 1 ? 'field' : 'fields'}, {pct(Math.min(...ps.map((p) => p.confidence)))} or more confident</Text>
                </Stack>
              {/each}
            </Stack>
          {/if}
        </Card>
        <Card title="Details">
          <Stack gap="3">
            <Code value={record!.id} />
            <Text variant="small">Created {formatTime(record!.created_at)}</Text>
            <Text variant="small">Ontology version {record!.spec_version}</Text>
          </Stack>
        </Card>
      {/snippet}
    </Split>
  </Page>

  <RecordForm bind:open={editing} type={record.type} {record} onsaved={(r) => { record = r; toast('Saved', 'success'); }} />
  <Dialog bind:open={confirm} title="Delete this record?" description="It goes from every view, search and link. Its source files stay.">
    {#snippet children()}{/snippet}
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (confirm = false)}>Cancel</Button>
      <Button variant="danger" loading={deleting} onclick={remove}>Delete</Button>
    {/snippet}
  </Dialog>
{/if}
