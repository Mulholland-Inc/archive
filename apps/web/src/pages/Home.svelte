<script lang="ts">
  import { Page, PageHeader, Grid, RecordTile, FileCard, Card, ResultRow, Stack, Notice, Button, Text, Row, Spacer, Link } from '../lib';
  import { api } from '../lib/api';
  import { session, isAdmin, canWrite, reloadOntology, reloadSpaces, reloadViews, reloadPending } from '../lib/session.svelte';
  import { toast, fail } from '../lib/toast.svelte';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { objectTypes, fieldsOf, humanize, formatBytes, relative, typeIcon } from '../lib/spec';
  import { kindIcon, statusLabel, statusTone } from '../lib/sources';
  import type { ArchiveFile, Source } from '../lib/types';
  import NewTypeDialog from './parts/NewTypeDialog.svelte';

  setCrumbs([{ label: 'Home' }]);

  let files = $state<ArchiveFile[]>([]);
  let sources = $state<Source[]>([]);
  let pending = $state(0);
  let creating = $state(false);
  let loadingDemo = $state(false);

  async function demo() {
    loadingDemo = true;
    try {
      const counts = await api.demo();
      await Promise.all([reloadOntology(), reloadSpaces(), reloadViews()]);
      reloadPending();
      const records = Object.entries(counts).filter(([k]) => k !== 'review').reduce((n, [, v]) => n + v, 0);
      toast(`Loaded ${records} example records and ${counts.review ?? 0} items to review`, 'success');
    } catch (e) {
      fail(e);
    } finally {
      loadingDemo = false;
    }
  }

  $effect(() => {
    const space = session.space || undefined;
    api.files({ space, limit: 8 }).then((f) => (files = f ?? [])).catch(() => {});
  });
  api.sources().then((s) => (sources = s ?? [])).catch(() => {});
  api
    .changes()
    .then((cs) => (pending = (cs ?? []).filter((c) => c.status === 'proposed' && !c.proposed_by.startsWith('revert:')).length))
    .catch(() => {});

  const o = $derived(session.ontology);
  const types = $derived(objectTypes(o));
  const first = $derived(session.actor?.name?.split(' ')[0]);
  const failing = $derived(sources.filter((s) => s.status === 'error' || s.status === 'needs_auth'));
</script>

<Page>
  <PageHeader title={first ? `Hello, ${first}` : 'Home'} description="Everything you keep, in the types you define." />

  {#if pending && isAdmin()}
    <Notice tone="warning">
      {pending === 1 ? 'One ontology change waits' : `${pending} ontology changes wait`} for your approval.
      {#snippet action()}<Button variant="ghost" onclick={() => navigate('/changes')}>Review</Button>{/snippet}
    </Notice>
  {/if}
  {#each failing as s (s.id)}
    <Notice tone={s.status === 'error' ? 'danger' : 'warning'}>
      {s.name}: {statusLabel(s.status).toLowerCase()}{s.error ? `, ${s.error}` : ''}.
      {#snippet action()}<Button variant="ghost" onclick={() => navigate('/sources')}>Open</Button>{/snippet}
    </Notice>
  {/each}

  {#if !types.length}
    <Card title="Get started" description="Three steps to a working archive.">
      <Stack gap="0">
        <ResultRow icon="shapes" title="Define a type" meta="Like Invoice, Contact or Receipt">
          <Text variant="muted">Types are the kinds of records you keep, with the fields they have.</Text>
        </ResultRow>
        <ResultRow href="/library" icon="upload" title="Upload files" meta="PDFs, photos, mail">
          <Text variant="muted">Archive extracts their text and reads them into your types.</Text>
        </ResultRow>
        <ResultRow href="/sources" icon="plug" title="Connect a source" meta="Gmail, Drive, IMAP, folders">
          <Text variant="muted">New items keep flowing in on their own.</Text>
        </ResultRow>
      </Stack>
      {#if isAdmin()}
        <Row>
          <Button variant="primary" icon="plus" onclick={() => (creating = true)}>Define your first type</Button>
          <Button icon="sparkle" loading={loadingDemo} onclick={demo}>Load demo data</Button>
        </Row>
      {/if}
    </Card>
  {:else}
    <Stack gap="3">
      <Row>
        <Text variant="heading">Types</Text>
        <Spacer />
        {#if isAdmin()}<Button variant="ghost" icon="plus" onclick={() => (creating = true)}>New type</Button>{/if}
      </Row>
      <Grid min="sm">
        {#each types as t (t)}
          <RecordTile
            href={`/records/${t}`}
            title={humanize(t)}
            icon={typeIcon(o, t)}
            rows={[{ label: 'Fields', value: fieldsOf(o, t).map(([n]) => humanize(n).toLowerCase()).slice(0, 5).join(', ') || 'None yet' }]}
          />
        {/each}
      </Grid>
    </Stack>
  {/if}

  <Stack gap="3">
    <Row>
      <Text variant="heading">Recent files</Text>
      <Spacer />
      <Link href="/library" variant="subtle">All files</Link>
    </Row>
    {#if files.length}
      <Grid min="tile">
        {#each files as f (f.id)}
          <FileCard href={`/library/${f.id}`} src={f.metadata?.thumbnail ? api.fileThumbnailURL(f.id) : undefined} mime={f.mime} name={f.name} detail={`${formatBytes(f.size)}, ${relative(f.created_at)}`} />
        {/each}
      </Grid>
    {:else}
      <Text variant="muted">No files yet. {canWrite() ? 'Drop some on the Files page.' : ''}</Text>
    {/if}
  </Stack>

  {#if sources.length}
    <Stack gap="3">
      <Row>
        <Text variant="heading">Sources</Text>
        <Spacer />
        <Link href="/sources" variant="subtle">Manage</Link>
      </Row>
      <Stack gap="0">
        {#each sources as s (s.id)}
          <ResultRow href="/sources" icon={kindIcon(s.kind)} title={s.name} meta={s.last_run_at ? `Synced ${relative(s.last_run_at)}` : 'Never synced'}>
            <Text tone={statusTone(s.status)}>{statusLabel(s.status)}</Text>
          </ResultRow>
        {/each}
      </Stack>
    </Stack>
  {/if}
</Page>

<NewTypeDialog bind:open={creating} />
