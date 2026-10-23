<script lang="ts">
  import {
    Page,
    PageHeader,
    Button,
    IconButton,
    Row,
    Stack,
    Spacer,
    SegmentedControl,
    Select,
    FilterChip,
    Table,
    Board,
    MonthGrid,
    EventChip,
    RecordTile,
    EmptyState,
    Notice,
    Skeleton,
    Link,
    Text,
    Popover,
    Menu,
    Dialog,
    Sheet,
    Input
  } from '../lib';
  import { api } from '../lib/api';
  import { session, reloadViews, canWrite } from '../lib/session.svelte';
  import { route, query, setQuery, navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  import { fieldsOf, humanize, kindOf, recordTitle, titleField, formatValue, relative } from '../lib/spec';
  import { remember, refTitle } from '../lib/refs.svelte';
  import { toast, fail } from '../lib/toast.svelte';
  import type { ArchiveRecord, View } from '../lib/types';
  import FieldValue from './parts/FieldValue.svelte';
  import FilterAdd from './parts/FilterAdd.svelte';
  import RecordForm from './parts/RecordForm.svelte';
  import SaveViewDialog from './parts/SaveViewDialog.svelte';
  import RecordPeek from './parts/RecordPeek.svelte';
  import { preloadRecord } from '../lib/preload';

  let { type: typeProp, viewId }: { type?: string; viewId?: string } = $props();

  const pageSize = 100;

  let view = $state<View | null>(null);
  let viewMissing = $state(false);
  let local = $state<{ layout: string; by: string; where: { [k: string]: string } }>({ layout: 'table', by: '', where: {} });
  let records = $state<ArchiveRecord[]>([]);
  let loading = $state(true);
  let more = $state(false);
  let loadingMore = $state(false);
  let error = $state('');
  let creating = $state(false);
  let preset = $state<{ [k: string]: string }>({});
  let saving = $state(false);
  let viewMenu = $state(false);
  let confirmDelete = $state(false);
  let deleting = $state(false);

  const type = $derived(view?.type ?? typeProp ?? '');
  const o = $derived(session.ontology);
  const exists = $derived(!!o?.spec.types[type]);
  const fields = $derived(fieldsOf(o, type));
  const tf = $derived(titleField(o, type));

  const settings = $derived.by(() => {
    if (viewId) return local;
    const q = new URLSearchParams(route.search);
    const where: { [k: string]: string } = {};
    for (const [k, v] of q) if (k.startsWith('where.')) where[k.slice(6)] = v;
    return { layout: q.get('layout') ?? 'table', by: q.get('by') ?? '', where };
  });

  const space = $derived(session.space);
  const whereKey = $derived(JSON.stringify(settings.where));
  const dirty = $derived(
    !!view &&
      (view.layout !== local.layout || (view.by ?? '') !== local.by || JSON.stringify(view.where ?? {}) !== JSON.stringify(local.where))
  );

  const boardFields = $derived(fields.filter(([, f]) => !f.list && ['enum', 'boolean', 'reference', 'text'].includes(kindOf(o, f.type))));
  const dateFields = $derived(fields.filter(([, f]) => !f.list && ['date', 'timestamp'].includes(kindOf(o, f.type))));
  const by = $derived(
    settings.layout === 'board'
      ? (boardFields.find(([n]) => n === settings.by) ?? boardFields.find(([, f]) => kindOf(o, f.type) === 'enum') ?? boardFields[0])?.[0]
      : settings.layout === 'calendar'
        ? (dateFields.find(([n]) => n === settings.by) ?? dateFields[0])?.[0]
        : undefined
  );

  $effect(() => {
    if (!viewId) return;
    view = null;
    viewMissing = false;
    api
      .view(viewId)
      .then((v) => {
        view = v;
        local = { layout: v.layout, by: v.by ?? '', where: { ...(v.where ?? {}) } };
      })
      .catch(() => {
        viewMissing = true;
        loading = false;
      });
  });

  $effect(() => {
    if (view) setCrumbs([{ label: humanize(view.type), href: `/records/${view.type}` }, { label: view.name }]);
    else if (type) setCrumbs([{ label: humanize(type) }]);
  });

  async function fetchPage(after?: string) {
    const rs = await api.list(type, { space: space || undefined, where: JSON.parse(whereKey), limit: pageSize, after });
    rs.forEach(remember);
    return rs;
  }

  $effect(() => {
    if (!type || !exists) return;
    whereKey;
    space;
    let cancelled = false;
    loading = true;
    error = '';
    fetchPage()
      .then((rs) => {
        if (cancelled) return;
        records = rs;
        more = rs.length === pageSize;
      })
      .catch((e) => {
        if (!cancelled) error = e.message;
      })
      .finally(() => {
        if (!cancelled) loading = false;
      });
    return () => {
      cancelled = true;
    };
  });

  async function loadMore() {
    loadingMore = true;
    try {
      const rs = await fetchPage(records[records.length - 1]?.id);
      records = [...records, ...rs];
      more = rs.length === pageSize;
    } catch (e) {
      fail(e);
    } finally {
      loadingMore = false;
    }
  }

  function update(patch: Partial<{ layout: string; by: string; where: { [k: string]: string } }>) {
    if (viewId) {
      local = { ...local, ...patch };
      return;
    }
    const next = { ...settings, ...patch };
    const params: { [k: string]: string | undefined } = { layout: next.layout === 'table' ? undefined : next.layout, by: next.by || undefined };
    for (const k of Object.keys(settings.where)) params[`where.${k}`] = undefined;
    for (const [k, v] of Object.entries(next.where)) params[`where.${k}`] = v;
    setQuery(params);
  }

  function removeFilter(k: string) {
    const w = { ...settings.where };
    delete w[k];
    update({ where: w });
  }

  async function saveView() {
    if (!view) return;
    try {
      view = await api.setView(view.id, { name: view.name, type: view.type, layout: local.layout, by: local.by || undefined, where: local.where });
      await reloadViews();
      toast('Saved the view', 'success');
    } catch (e) {
      fail(e);
    }
  }

  async function deleteView() {
    if (!view) return;
    deleting = true;
    try {
      await api.removeView(view.id);
      await reloadViews();
      toast(`Deleted ${view.name}`, 'success');
      navigate(`/records/${view.type}`);
    } catch (e) {
      fail(e);
    } finally {
      deleting = false;
    }
  }

  function groupKey(r: ArchiveRecord) {
    const v = by ? r.data[by] : undefined;
    return v === undefined || v === null || v === '' ? '__none' : String(v);
  }

  const columns = $derived.by(() => {
    if (settings.layout !== 'board' || !by) return [];
    const f = fields.find(([n]) => n === by)?.[1];
    if (!f) return [];
    const k = kindOf(o, f.type);
    let keys: string[];
    if (k === 'enum') keys = [...(f.enum ?? [])];
    else if (k === 'boolean') keys = ['true', 'false'];
    else keys = [...new Set(shown.map(groupKey).filter((x) => x !== '__none'))].sort();
    keys.push('__none');
    return keys.map((key) => ({
      id: key,
      label: key === '__none' ? 'None' : k === 'boolean' ? (key === 'true' ? 'Yes' : 'No') : formatValue(o, f, key),
      items: shown.filter((r) => groupKey(r) === key)
    }));
  });

  async function move(id: string, column: string) {
    if (!by) return;
    const r = records.find((x) => x.id === id);
    if (!r || groupKey(r) === column) return;
    const f = fields.find(([n]) => n === by)?.[1];
    const value = column === '__none' ? null : f && kindOf(o, f.type) === 'boolean' ? column === 'true' : column;
    const before = r.data[by];
    r.data = { ...r.data, [by]: value };
    try {
      const saved = await api.patch(r.type, r.id, { [by]: value });
      const i = records.findIndex((x) => x.id === id);
      if (i >= 0) records[i] = saved;
    } catch (e) {
      r.data = { ...r.data, [by]: before };
      fail(e);
    }
  }

  function tileRows(r: ArchiveRecord) {
    return fields
      .filter(([n]) => n !== tf && n !== by && r.data[n] !== undefined && r.data[n] !== null)
      .slice(0, 3)
      .map(([n, f]) => ({ label: humanize(n), value: formatValue(o, f, r.data[n]) }));
  }

  const tableColumns = $derived([
    ...(tf ? [{ id: tf, label: humanize(tf) }] : [{ id: '__title', label: 'Record' }]),
    ...fields.filter(([n]) => n !== tf).map(([n]) => ({ id: n, label: humanize(n) })),
    { id: '__updated', label: 'Updated', width: 'narrow' as const }
  ]);

  const peek = $derived.by(() => {
    const [t, i] = (new URLSearchParams(route.search).get('peek') ?? '').split('/');
    return t && i ? { type: t, id: i } : null;
  });

  function openPeek(t: string, i: string) {
    setQuery({ peek: `${t}/${i}` });
  }

  function replaceRecord(r: ArchiveRecord) {
    const i = records.findIndex((x) => x.id === r.id);
    if (i >= 0) records[i] = r;
  }

  let q = $state('');
  let found = $state<ArchiveRecord[]>([]);
  const term = $derived(q.trim().toLowerCase());

  $effect(() => {
    found = [];
    if (!term || !more) return;
    let cancelled = false;
    const t = setTimeout(() => {
      api
        .search(term, { type, space: space || undefined, limit: 50 })
        .then((hits) => Promise.all((hits ?? []).filter((h) => h.kind === 'record' && h.type === type).map((h) => api.get(type, h.id).catch(() => null))))
        .then((rs) => {
          if (!cancelled) found = rs.filter((r): r is ArchiveRecord => !!r);
        })
        .catch(() => {});
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  });

  function haystack(r: ArchiveRecord) {
    return fields
      .map(([n, f]) => {
        const v = r.data[n];
        if (v === undefined || v === null) return '';
        if (kindOf(o, f.type) !== 'reference' || !f.to) return formatValue(o, f, v);
        return (Array.isArray(v) ? v : [v]).map((x) => refTitle(f.to!, String(x))).join(' ');
      })
      .concat(recordTitle(o, r))
      .join(' ')
      .toLowerCase();
  }

  const shown = $derived.by(() => {
    if (!term) return records;
    const hits = records.filter((r) => haystack(r).includes(term));
    const ids = new Set(hits.map((r) => r.id));
    return [...hits, ...found.filter((r) => !ids.has(r.id))];
  });

  const writable = $derived(canWrite());
</script>

{#if viewMissing}
  <Page>
    <EmptyState icon="rows" title="This view does not exist" description="It may have been deleted, or it lives in a space you cannot see.">
      <Button onclick={() => navigate('/')}>Go home</Button>
    </EmptyState>
  </Page>
{:else if type && o && !exists}
  <Page>
    <EmptyState icon="shapes" title={`There is no ${type} type`} description="It may have been removed or renamed in the ontology.">
      <Button onclick={() => navigate('/types')}>Open the ontology</Button>
    </EmptyState>
  </Page>
{:else if type}
  <Page width="full" fit>
    <PageHeader title={view?.name ?? `${humanize(type)}`} description={view ? `${humanize(type)} records` : undefined}>
      {#snippet actions()}
        {#if view}
          {#if dirty}<Button variant="tint" icon="check" onclick={saveView}>Save changes</Button>{/if}
          <Popover bind:open={viewMenu} align="end">
            {#snippet trigger({ toggle })}<IconButton icon="more" label="View options" onclick={toggle} />{/snippet}
            {#snippet children({ close })}
              <Menu
                items={[
                  { id: 'type', label: `All ${humanize(type).toLowerCase()} records`, icon: 'table' },
                  { id: 'delete', label: 'Delete view', icon: 'trash', danger: true, separator: true }
                ]}
                searchable={false}
                onselect={(i) => {
                  close();
                  if (i.id === 'type') navigate(`/records/${type}`);
                  if (i.id === 'delete') confirmDelete = true;
                }}
              />
            {/snippet}
          </Popover>
        {:else}
          <Button variant="ghost" icon="layers" onclick={() => (saving = true)}>Save view</Button>
        {/if}
        {#if writable}
          <Button
            variant="primary"
            icon="plus"
            onclick={() => {
              preset = { ...settings.where };
              creating = true;
            }}>New</Button
          >
        {/if}
      {/snippet}
    </PageHeader>

    <Row gap="2" wrap>
      <div class="search"><Input bind:value={q} type="search" icon="search" placeholder={`Search ${humanize(type).toLowerCase()} records`} label="Search" /></div>
      <FilterAdd {type} where={settings.where} onadd={(k, v) => update({ where: { ...settings.where, [k]: v } })} />
      {#each Object.entries(settings.where) as [k, v] (k)}
        <FilterChip
          icon="filter"
          field={humanize(k)}
          operator="is"
          value={(() => {
            const f = fields.find(([n]) => n === k)?.[1];
            return f && kindOf(o, f.type) === 'reference' ? v.slice(-6) : f ? formatValue(o, f, f && kindOf(o, f.type) === 'boolean' ? v === 'true' : v) : v;
          })()}
          onremove={() => removeFilter(k)}
        />
      {/each}
      <Spacer />
      {#if settings.layout === 'board' && boardFields.length}
        <Select icon="columns" value={by ?? ''} options={boardFields.map(([n]) => ({ id: n, label: `By ${humanize(n).toLowerCase()}` }))} onchange={(v) => update({ by: v })} />
      {/if}
      {#if settings.layout === 'calendar' && dateFields.length}
        <Select icon="calendar" value={by ?? ''} options={dateFields.map(([n]) => ({ id: n, label: `By ${humanize(n).toLowerCase()}` }))} onchange={(v) => update({ by: v })} />
      {/if}
      <SegmentedControl
        label="Layout"
        options={[
          { value: 'table', label: 'Table', icon: 'rows' },
          { value: 'board', label: 'Board', icon: 'columns' },
          { value: 'calendar', label: 'Calendar', icon: 'calendar' }
        ]}
        bind:value={() => settings.layout, (v) => update({ layout: v, by: '' })}
      />
    </Row>

    <div class="body">
    {#if error}
      <Notice tone="danger">{error}</Notice>
    {:else if loading && !records.length}
      <Skeleton rows={8} />
    {:else if term && !shown.length}
      <EmptyState icon="search" title={`Nothing matches “${q.trim()}”`} description="Try other words, or clear the search.">
        <Button onclick={() => (q = '')}>Clear search</Button>
      </EmptyState>
    {:else if !records.length}
      <EmptyState
        icon="table"
        title={Object.keys(settings.where).length ? 'Nothing matches these filters' : `No ${humanize(type).toLowerCase()} records yet`}
        description={Object.keys(settings.where).length
          ? 'Remove a filter to see more.'
          : 'Add one by hand, upload files that Archive reads into this type, or connect a source.'}
      >
        {#if Object.keys(settings.where).length}
          <Button onclick={() => update({ where: {} })}>Clear filters</Button>
        {:else}
          {#if writable}<Button variant="primary" icon="plus" onclick={() => (creating = true)}>New {humanize(type).toLowerCase()}</Button>{/if}
          <Button icon="upload" onclick={() => navigate(`/library?intake=${type}`)}>Upload files</Button>
        {/if}
      </EmptyState>
    {:else if settings.layout === 'board'}
      {#if !by}
        <Notice>This type has no field to group by. Add an enum, boolean or reference field to use the board.</Notice>
      {:else}
        <Board {columns} onmove={(id, col) => move(id, col)} draggable={writable}>
          {#snippet card(r, state)}
            <RecordTile href={`/records/${r.type}/${r.id}`} title={recordTitle(o, r)} rows={tileRows(r)} {state} onclick={() => openPeek(r.type, r.id)} />
          {/snippet}
        </Board>
      {/if}
    {:else if settings.layout === 'calendar'}
      {#if !by}
        <Notice>This type has no date field. Add a date or timestamp field to use the calendar.</Notice>
      {:else}
        <MonthGrid
          items={shown}
          key={(r) => r.id}
          date={(r) => {
            const v = r.data[by];
            return typeof v === 'string' ? v : (v as { utc?: string } | undefined)?.utc;
          }}
        >
          {#snippet item(r)}<EventChip href={`/records/${r.type}/${r.id}`} label={recordTitle(o, r)} onclick={() => openPeek(r.type, r.id)} />{/snippet}
        </MonthGrid>
      {/if}
    {:else}
      <Table columns={tableColumns} rows={shown} key={(r) => r.id} onrowclick={(r) => openPeek(r.type, r.id)} onrowhover={(r) => preloadRecord(r.type, r.id)}>
        {#snippet cell(r, c)}
          {#if c === '__title' || c === tf}
            <Text variant="strong">{recordTitle(o, r)}</Text>
          {:else if c === '__updated'}
            <Text variant="muted">{relative(r.updated_at)}</Text>
          {:else}
            {@const f = fields.find(([n]) => n === c)?.[1]}
            {#if f && r.data[c] !== undefined && r.data[c] !== null}<FieldValue field={f} value={r.data[c]} />{/if}
          {/if}
        {/snippet}
      </Table>
    {/if}

    {#if more && !loading && !term}
      <Row><Button loading={loadingMore} onclick={loadMore}>Load more</Button></Row>
    {/if}
    </div>
  </Page>

  <RecordForm
    bind:open={creating}
    {type}
    {preset}
    onsaved={(r) => {
      records = [r, ...records];
      toast(`Created ${recordTitle(o, r)}`, 'success', { label: 'Open', run: () => navigate(`/records/${r.type}/${r.id}`) });
    }}
  />
  <Sheet open={!!peek} label="Record" onclose={() => setQuery({ peek: undefined })}>
    {#if peek}
      {#key `${peek.type}/${peek.id}`}
        <RecordPeek
          type={peek.type}
          id={peek.id}
          onopen={openPeek}
          onclose={() => setQuery({ peek: undefined })}
          onsaved={replaceRecord}
          ondeleted={(r) => (records = records.filter((x) => x.id !== r.id))}
        />
      {/key}
    {/if}
  </Sheet>
  <SaveViewDialog bind:open={saving} {type} layout={settings.layout} by={by} where={settings.where} />
  <Dialog bind:open={confirmDelete} title="Delete this view?" description="The records stay; only the saved filters and layout go.">
    {#snippet children()}{/snippet}
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (confirmDelete = false)}>Cancel</Button>
      <Button variant="danger" loading={deleting} onclick={deleteView}>Delete view</Button>
    {/snippet}
  </Dialog>
{:else}
  <Page><Skeleton /></Page>
{/if}

<style>
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    overflow: auto;
  }
  .search {
    width: min(20rem, 100%);
  }
</style>
