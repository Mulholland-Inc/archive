<script lang="ts">
  import { Page, PageHeader, Button, Card, Stack, Row, Text, Notice, Skeleton, EmptyState, Dialog, Spacer } from '../lib';
  import { api } from '../lib/api';
  import { isAdmin, reloadOntology } from '../lib/session.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { formatTime, relative } from '../lib/spec';
  import { toast, fail } from '../lib/toast.svelte';
  import type { Change, ChangePreview } from '../lib/types';
  import ChangeSummary from './parts/ChangeSummary.svelte';

  setCrumbs([{ label: 'Ontology', href: '/types' }, { label: 'Changes' }]);

  let changes = $state<Change[]>([]);
  let loading = $state(true);
  let error = $state('');
  let preview = $state<ChangePreview | null>(null);
  let previewing = $state<string | null>(null);
  let busy = $state(false);

  async function load() {
    try {
      changes = ((await api.changes()) ?? []).filter((c) => !(c.status === 'proposed' && c.proposed_by.startsWith('revert:')));
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }
  load();

  async function open(c: Change) {
    previewing = c.id;
    try {
      preview = await api.preview(c.id);
    } catch (e) {
      fail(e);
    } finally {
      previewing = null;
    }
  }

  async function apply() {
    if (!preview) return;
    busy = true;
    try {
      await api.apply(preview.change.id);
      await reloadOntology();
      toast('Applied the change', 'success');
      preview = null;
      await load();
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
    }
  }

  async function revert(c: Change) {
    try {
      await api.revert(c.id);
      await reloadOntology();
      toast('Reverted the change', 'success');
      await load();
    } catch (e) {
      fail(e);
    }
  }

  const tone = (s: string) => (s === 'applied' ? 'success' : s === 'proposed' ? 'warning' : 'muted') as 'success' | 'warning' | 'muted';
  const label = (s: string) => ({ applied: 'Applied', proposed: 'Waiting for approval', reverted: 'Reverted' })[s] ?? s;
</script>

<Page width="narrow">
  <PageHeader title="Changes" description="Every change to the ontology, newest first. People and agents propose; admins preview and apply." />
  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if loading}
    <Skeleton rows={5} />
  {:else if !changes.length}
    <EmptyState icon="history" title="No changes yet" description="Creating a type or a field makes a change here." />
  {:else}
    <Stack gap="3">
      {#each changes as c (c.id)}
        <Card>
          <Stack gap="2">
            <Row gap="2">
              <Text tone={tone(c.status)}>{label(c.status)}</Text>
              <Text variant="muted">by {c.proposed_by.startsWith('revert:') ? 'a revert' : c.proposed_by}, {relative(c.created_at)}</Text>
              <Spacer />
              {#if c.status === 'proposed'}
                <Button variant={isAdmin() ? 'primary' : 'secondary'} loading={previewing === c.id} onclick={() => open(c)}>{isAdmin() ? 'Review' : 'Preview'}</Button>
              {:else if c.status === 'applied' && isAdmin()}
                <Button variant="ghost" icon="history" onclick={() => revert(c)}>Revert</Button>
              {/if}
            </Row>
            <ChangeSummary ops={c.ops} />
            <Text variant="small">Version {c.from_version} to {c.to_version}, {formatTime(c.created_at)}</Text>
          </Stack>
        </Card>
      {/each}
    </Stack>
  {/if}
</Page>

<Dialog open={!!preview} title="Review change" size="lg" onclose={() => (preview = null)}>
  {#if preview}
    <ChangeSummary ops={preview.change.ops} />
    <Notice tone={preview.failing ? 'warning' : 'success'}>
      {preview.touched} existing {preview.touched === 1 ? 'record changes' : 'records change'}{preview.failing ? `, and ${preview.failing} would no longer be valid until someone fixes them` : ', and all stay valid'}.
    </Notice>
    {#each preview.failures ?? [] as f (f.id)}
      <Text variant="small">{f.type} {f.id.slice(-6)}: {f.errors?.map((e) => `${e.path} ${e.message}`).join('; ')}</Text>
    {/each}
  {/if}
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (preview = null)}>Close</Button>
    {#if isAdmin()}<Button variant="primary" loading={busy} onclick={apply}>Apply change</Button>{/if}
  {/snippet}
</Dialog>
