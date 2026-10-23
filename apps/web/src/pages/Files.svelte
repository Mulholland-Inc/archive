<script lang="ts">
  import { Page, PageHeader, Grid, FileCard, FilePicker, Dropzone, Select, Row, Stack, Text, Progress, Card, EmptyState, Notice, Skeleton, Button, Spacer } from '../lib';
  import { api, upload } from '../lib/api';
  import { session, canWrite } from '../lib/session.svelte';
  import { query } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { objectTypes, humanize, formatBytes, relative } from '../lib/spec';
  import { toast } from '../lib/toast.svelte';
  import type { ArchiveFile } from '../lib/types';

  setCrumbs([{ label: 'Files' }]);

  const pageSize = 60;
  let files = $state<ArchiveFile[]>([]);
  let loading = $state(true);
  let more = $state(false);
  let error = $state('');
  let intake = $state(query().get('intake') ?? '');
  let uploads = $state<{ id: number; name: string; progress: number; error?: string }[]>([]);
  let uploadId = 0;

  const intakeItems = $derived([{ id: '', label: 'Just store them', icon: 'folder' }, ...objectTypes(session.ontology).map((t) => ({ id: t, label: `Read into ${humanize(t).toLowerCase()}`, icon: 'sparkle' }))]);
  const busy = $derived(files.some((f) => f.status === 'pending' || f.status === 'processing'));

  async function load(after?: string) {
    const fs = (await api.files({ space: session.space || undefined, limit: pageSize, after })) ?? [];
    more = fs.length === pageSize;
    return fs;
  }

  $effect(() => {
    session.space;
    loading = true;
    load()
      .then((fs) => (files = fs))
      .catch((e) => (error = e.message))
      .finally(() => (loading = false));
  });

  $effect(() => {
    if (!busy) return;
    const t = setInterval(async () => {
      const pendingIds = files.filter((f) => f.status === 'pending' || f.status === 'processing').map((f) => f.id);
      const fresh = await Promise.all(pendingIds.map((id) => api.file(id).catch(() => null)));
      for (const f of fresh) {
        if (!f) continue;
        const i = files.findIndex((x) => x.id === f.id);
        if (i >= 0) files[i] = f;
        if (f.status === 'done' && f.intake) toast(`Read ${f.name} into ${humanize(f.intake).toLowerCase()}`, 'success');
        if (f.status === 'error') toast(`Could not process ${f.name}: ${f.error ?? 'unknown error'}`, 'danger');
      }
    }, 2500);
    return () => clearInterval(t);
  });

  async function send(list: File[]) {
    const space = session.space || undefined;
    await Promise.all(
      list.map(async (file) => {
        const id = ++uploadId;
        uploads.push({ id, name: file.name, progress: 0 });
        const item = () => uploads.find((u) => u.id === id);
        try {
          const f = await upload(file, { space, intake: intake || undefined }, (p) => {
            const u = item();
            if (u) u.progress = p;
          });
          files = [f, ...files.filter((x) => x.id !== f.id)];
          uploads.splice(uploads.findIndex((u) => u.id === id), 1);
        } catch (e) {
          const u = item();
          if (u) u.error = (e as Error).message;
        }
      })
    );
  }

  function statusTone(f: ArchiveFile) {
    return f.status === 'error' ? 'danger' : f.status === 'done' ? undefined : 'warning';
  }

  function statusLabel(f: ArchiveFile) {
    if (f.status === 'pending') return 'Waiting';
    if (f.status === 'processing') return f.intake ? 'Reading' : 'Processing';
    if (f.status === 'error') return 'Failed';
    return '';
  }

  const writable = $derived(canWrite());
</script>

<Page width="full">
  <PageHeader title="Files" description="Documents, photos, audio and video. Archive extracts their text and, if you ask, reads them into records.">
    {#snippet actions()}
      {#if writable}
        <Select icon="sparkle" bind:value={intake} options={intakeItems} />
        <FilePicker onpick={send} />
      {/if}
    {/snippet}
  </PageHeader>

  {#if uploads.length}
    <Card title="Uploading">
      <Stack gap="3">
        {#each uploads as u (u.id)}
          <Stack gap="1">
            <Row gap="2">
              <Text truncate>{u.name}</Text>
              <Spacer />
              {#if u.error}
                <Text tone="danger">{u.error}</Text>
                <Button variant="ghost" onclick={() => uploads.splice(uploads.findIndex((x) => x.id === u.id), 1)}>Dismiss</Button>
              {:else}
                <Text variant="muted">{Math.round(u.progress * 100)}%</Text>
              {/if}
            </Row>
            {#if !u.error}<Progress value={u.progress} label={`Uploading ${u.name}`} />{/if}
          </Stack>
        {/each}
      </Stack>
    </Card>
  {/if}

  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if loading}
    <Skeleton rows={6} />
  {:else if !files.length && !uploads.length}
    <EmptyState icon="upload" title="No files yet" description="Drop files anywhere on this page, or choose them. Pick a type first to have Archive read them into records.">
      {#if writable}<FilePicker onpick={send} label="Choose files" />{/if}
    </EmptyState>
  {:else}
    <Grid min="tile">
      {#each files as f (f.id)}
        <FileCard
          href={`/library/${f.id}`}
          src={f.metadata?.thumbnail ? api.fileThumbnailURL(f.id) : undefined}
          mime={f.mime}
          name={f.name}
          detail={`${formatBytes(f.size)}, ${relative(f.created_at)}`}
        >
          {#snippet status()}
            {#if statusLabel(f)}<Text tone={statusTone(f)}>{statusLabel(f)}</Text>{/if}
          {/snippet}
        </FileCard>
      {/each}
    </Grid>
    {#if more}
      <Row>
        <Button
          onclick={async () => {
            const fs = await load(files[files.length - 1]?.id);
            files = [...files, ...fs];
          }}>Load more</Button
        >
      </Row>
    {/if}
  {/if}
</Page>

{#if writable}<Dropzone ondrop={send} label={intake ? `Drop to read into ${humanize(intake).toLowerCase()}` : 'Drop files to upload'} />{/if}
