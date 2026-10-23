<script lang="ts">
  import { Page, PageHeader, Button, Table, Text, Checkbox, EmptyState, Notice, Skeleton, Sheet } from '../lib';
  import { api } from '../lib/api';
  import { session, reloadPending } from '../lib/session.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { humanize, relative } from '../lib/spec';
  import { proposalTitle, proposalSource } from '../lib/review';
  import Confidence from './parts/Confidence.svelte';
  import { toast, fail } from '../lib/toast.svelte';
  import type { Proposal, ProposalStatus } from '../lib/types';
  import ProposalPeek from './parts/ProposalPeek.svelte';

  setCrumbs([{ label: 'Review' }]);

  let status = $state<ProposalStatus>('review');
  let items = $state<Proposal[]>([]);
  let counts = $state<{ [s: string]: number }>({});
  let loading = $state(true);
  let error = $state('');
  let selected = $state<Set<string>>(new Set());
  let openId = $state<string | null>(null);
  let busy = $state(false);

  const o = $derived(session.ontology);
  const open = $derived(items.find((p) => p.id === openId) ?? null);
  const decidable = $derived(status === 'review' || status === 'accepted');
  const tabs: { value: ProposalStatus; label: string }[] = [
    { value: 'review', label: 'To review' },
    { value: 'accepted', label: 'Auto-approved' },
    { value: 'approved', label: 'Approved' },
    { value: 'rejected', label: 'Rejected' }
  ];

  let tabEls = $state<{ [s: string]: HTMLButtonElement }>({});
  let line = $state({ left: 0, width: 0 });

  $effect(() => {
    const el = tabEls[status];
    counts;
    if (el) line = { left: el.offsetLeft, width: el.offsetWidth };
  });

  async function load() {
    const [s, space] = [status, session.space || undefined];
    loading = true;
    error = '';
    try {
      const [list, c] = await Promise.all([api.review({ status: s, space, limit: 1000 }), api.reviewCounts(space)]);
      if (s !== status) return;
      items = [...(list ?? [])].reverse();
      counts = c;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    status;
    session.space;
    selected = new Set();
    openId = null;
    load();
  });

  function toggle(id: string, on: boolean) {
    const next = new Set(selected);
    if (on) next.add(id);
    else next.delete(id);
    selected = next;
  }

  async function decide(kind: 'approve' | 'reject', ids: string[], edits?: { [id: string]: { [field: string]: unknown } }) {
    if (!ids.length) return;
    busy = true;
    try {
      const res = kind === 'approve' ? await api.approve(ids, edits) : await api.reject(ids);
      const done = new Set(res.done.map((p) => p.id));
      items = items.filter((p) => !done.has(p.id));
      selected = new Set([...selected].filter((id) => !done.has(id)));
      if (openId && done.has(openId)) openId = null;
      const verb = kind === 'approve' ? 'Approved' : 'Rejected';
      if (res.failed.length) toast(`${verb} ${res.done.length}; ${res.failed.length} could not be ${kind}d: ${res.failed[0].error}`, 'danger');
      else toast(`${verb} ${res.done.length} ${res.done.length === 1 ? 'item' : 'items'}`, 'success');
      counts = await api.reviewCounts(session.space || undefined);
      reloadPending();
    } catch (e) {
      fail(e);
    } finally {
      busy = false;
    }
  }
</script>

<Page width="full" fit>
  <PageHeader title="Review" />

  <nav class="tabs" aria-label="Status">
    {#each tabs as t (t.value)}
      <button class="tab" bind:this={tabEls[t.value]} aria-current={status === t.value || undefined} onclick={() => (status = t.value)}>
        {t.label}{#if counts[t.value]}<span class="n">{counts[t.value]}</span>{/if}
      </button>
    {/each}
    <span class="line" style:left="{line.left}px" style:width="{line.width}px"></span>
    <span class="grow"></span>
    {#if decidable && items.length}
      <Button variant="ghost" onclick={() => (selected = selected.size === items.length ? new Set() : new Set(items.map((p) => p.id)))}>
        {selected.size === items.length ? 'Select none' : 'Select all'}
      </Button>
    {/if}
  </nav>

  <div class="body" data-bulk={(selected.size && decidable) || undefined}>
    {#if error}
      <Notice tone="danger">{error}</Notice>
    {:else if loading && !items.length}
      <Skeleton rows={8} />
    {:else if !items.length}
      <EmptyState
        icon={status === 'review' ? 'check' : 'inbox'}
        title={status === 'review' ? 'All caught up' : 'Nothing here yet'}
      />
    {:else}
      <Table
        columns={[
          ...(decidable ? [{ id: 'select', label: '', width: 'narrow' as const }] : []),
          { id: 'item', label: 'Item', width: 'wide' as const },
          { id: 'type', label: 'Type' },
          { id: 'confidence', label: 'Confidence', width: 'narrow' as const },
          { id: 'source', label: 'Source', width: 'wide' as const },
          { id: 'read', label: 'Added', width: 'narrow' as const }
        ]}
        rows={items}
        key={(p) => p.id}
        onrowclick={(p) => (openId = p.id)}
      >
        {#snippet cell(p, c)}
          {#if c === 'select'}
            <Checkbox checked={selected.has(p.id)} label="Select" onchange={(v) => toggle(p.id, v)} />
          {:else if c === 'item'}
            <Text variant="strong">{proposalTitle(o, p)}</Text>
          {:else if c === 'type'}
            <Text variant="muted">{humanize(p.type)}</Text>
          {:else if c === 'confidence'}
            <Confidence value={p.confidence} />
          {:else if c === 'source'}
            <Text variant="muted">{p.label || proposalSource(p).label}</Text>
          {:else}
            <Text variant="muted">{relative(p.decided_at ?? p.created_at)}</Text>
          {/if}
        {/snippet}
      </Table>
    {/if}
  </div>

  {#if selected.size && decidable}
    <div class="bulk">
      <Text variant="strong">{selected.size} selected</Text>
      <Button variant="ghost" onclick={() => (selected = new Set())}>Clear</Button>
      <Button variant="ghost" icon="x" disabled={busy} onclick={() => decide('reject', [...selected])}>Reject</Button>
      <Button variant="primary" icon="check" loading={busy} onclick={() => decide('approve', [...selected])}>Approve {selected.size}</Button>
    </div>
  {/if}
</Page>

<Sheet open={!!open} label="Review item" onclose={() => (openId = null)}>
  {#if open}
    {#key open.id}
      <ProposalPeek
        proposal={open}
        {busy}
        onclose={() => (openId = null)}
        onapprove={(edits) => decide('approve', [open.id], Object.keys(edits).length ? { [open.id]: edits } : undefined)}
        onreject={() => decide('reject', [open.id])}
      />
    {/key}
  {/if}
</Sheet>

<style>
  .tabs {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-5);
    margin-top: calc(var(--space-2) * -1);
    border-bottom: var(--border-width) solid var(--border);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    padding: var(--space-2) 0;
    border: 0;
    background: none;
    color: var(--muted-foreground);
    transition: color var(--duration-fast) var(--ease-out);
    font: inherit;
    font-weight: var(--font-medium);
    cursor: pointer;
  }
  .tab:hover {
    color: var(--foreground);
  }
  .tab[aria-current] {
    color: var(--foreground);
  }
  .line {
    position: absolute;
    bottom: calc(var(--border-width) * -1);
    height: 2px;
    border-radius: var(--radius-full);
    background: var(--foreground);
    transition:
      left var(--duration-normal) var(--ease-out),
      width var(--duration-normal) var(--ease-out);
  }
  .n {
    color: var(--subtle-foreground);
    font-variant-numeric: tabular-nums;
  }
  .grow {
    flex: 1;
  }
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: auto;
  }
  .body[data-bulk] {
    padding-bottom: 4.5rem;
  }
  .bulk {
    position: fixed;
    left: 50%;
    bottom: var(--space-6);
    z-index: var(--z-popover);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
    border-radius: var(--radius-full);
    background: var(--card);
    box-shadow: var(--shadow-lift);
    transform: translateX(-50%);
  }
</style>
