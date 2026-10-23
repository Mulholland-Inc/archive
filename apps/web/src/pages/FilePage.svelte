<script lang="ts">
  import { Page, PageHeader, Button, Split, Card, KeyValue, Text, Code, Notice, Skeleton, EmptyState, FilePreview, Stack } from '../lib';
  import { api } from '../lib/api';
  import { spaceLabel } from '../lib/session.svelte';
  import { navigate, query } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { formatBytes, formatTime, humanize } from '../lib/spec';
  import { toast, fail } from '../lib/toast.svelte';
  import type { ArchiveFile } from '../lib/types';

  let { id }: { id: string } = $props();

  let file = $state<ArchiveFile | null>(null);
  let text = $state<string | null>(null);
  let missing = $state(false);
  let error = $state('');
  let sharing = $state(false);
  const page = Number(query().get('page')) || undefined;

  $effect(() => {
    api
      .file(id)
      .then((f) => {
        file = f;
        api.fileText(id).then((t) => (text = t)).catch(() => (text = ''));
      })
      .catch((e) => {
        if (e.status === 404) missing = true;
        else error = e.message;
      });
  });

  $effect(() => {
    if (!file || (file.status !== 'pending' && file.status !== 'processing')) return;
    const t = setInterval(async () => {
      const f = await api.file(id).catch(() => null);
      if (f) file = f;
    }, 2500);
    return () => clearInterval(t);
  });

  $effect(() => {
    setCrumbs([{ label: 'Files', href: '/library' }, { label: file?.name ?? '…' }]);
  });

  async function share() {
    sharing = true;
    try {
      const l = await api.fileLink(id);
      const url = new URL(l.url, location.origin).toString();
      await navigator.clipboard.writeText(url).catch(() => {});
      toast('Copied a link that works for 24 hours, without signing in', 'success');
    } catch (e) {
      fail(e);
    } finally {
      sharing = false;
    }
  }

  function download() {
    const a = document.createElement('a');
    a.href = api.fileContentURL(id);
    a.download = file?.name ?? 'file';
    document.body.append(a);
    a.click();
    a.remove();
  }

  const meta = $derived(
    file
      ? Object.entries(file.metadata ?? {})
          .filter(([k, v]) => k !== 'thumbnail' && v !== null && typeof v !== 'object')
          .map(([k, v]) => ({ label: humanize(k), value: String(v) }))
      : []
  );

  const statusText: { [k: string]: string } = { pending: 'Waiting to be processed', processing: 'Processing now', done: 'Processed', error: 'Processing failed' };
</script>

{#if missing}
  <Page>
    <EmptyState icon="folder" title="This file does not exist" description="It may have been deleted, or it lives in a space you cannot see.">
      <Button onclick={() => navigate('/library')}>All files</Button>
    </EmptyState>
  </Page>
{:else if error}
  <Page><Notice tone="danger">{error}</Notice></Page>
{:else if !file}
  <Page><Skeleton rows={10} /></Page>
{:else}
  <Page>
    <PageHeader title={file.name}>
      {#snippet meta()}
        <Text>{spaceLabel(file!.space)}</Text>
        <Text>{formatBytes(file!.size)}</Text>
        <Text tone={file!.status === 'error' ? 'danger' : file!.status === 'done' ? undefined : 'warning'}>{statusText[file!.status] ?? file!.status}</Text>
      {/snippet}
      {#snippet actions()}
        <Button icon="link" loading={sharing} onclick={share}>Share link</Button>
        <Button icon="download" onclick={download}>Download</Button>
      {/snippet}
    </PageHeader>

    {#if file.status === 'error' && file.error}<Notice tone="danger">{file.error}</Notice>{/if}

    <Split>
      {#snippet main()}
        <FilePreview src={api.fileContentURL(file!.id)} mime={file!.mime} name={file!.name} {page} />
        {#if text}
          <Card title="Extracted text">
            <Code value={text} block copy={false} />
          </Card>
        {/if}
      {/snippet}
      {#snippet aside()}
        <Card title="Details">
          <KeyValue
            items={[
              { label: 'Type', value: file!.mime },
              { label: 'Added', value: formatTime(file!.created_at) },
              ...(file!.intake ? [{ label: 'Read into', value: humanize(file!.intake) }] : []),
              ...meta
            ]}
            label={(i) => i.label}
          >
            {#snippet value(i)}<Text>{i.value}</Text>{/snippet}
          </KeyValue>
        </Card>
        <Card title="Fingerprint">
          <Stack gap="2">
            <Text variant="small">SHA-256 of the content, which Archive uses to spot duplicates.</Text>
            <Code value={file!.sha256} />
          </Stack>
        </Card>
      {/snippet}
    </Split>
  </Page>
{/if}
