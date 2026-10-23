<script lang="ts">
  import { Button, IconButton, Icon, Avatar, Popover, Menu, Dialog, Skeleton, Notice } from '../../lib';
  import { api } from '../../lib/api';
  import { session, canWrite, spaceLabel } from '../../lib/session.svelte';
  import { navigate } from '../../lib/router.svelte';
  import { fieldsOf, humanize, recordTitle, titleField, formatValue, kindOf, isSecret, typeIcon } from '../../lib/spec';
  import { refTitle, remember } from '../../lib/refs.svelte';
  import { toast, fail } from '../../lib/toast.svelte';
  import type { ArchiveRecord, Field, RecordLink } from '../../lib/types';
  import FieldInput from './FieldInput.svelte';
  import { RecordEdit, secretTypes } from '../../lib/edit.svelte';
  import { preloadRecord } from '../../lib/preload';

  let {
    type,
    id,
    onopen,
    onclose,
    onsaved,
    ondeleted
  }: {
    type: string;
    id: string;
    onopen: (type: string, id: string) => void;
    onclose: () => void;
    onsaved?: (r: ArchiveRecord) => void;
    ondeleted?: (r: ArchiveRecord) => void;
  } = $props();

  let record = $state<ArchiveRecord | null>(null);
  let links = $state<RecordLink[]>([]);
  let error = $state('');
  let menu = $state(false);
  let confirm = $state(false);
  let deleting = $state(false);

  const o = $derived(session.ontology);
  const tf = $derived(titleField(o, type));
  const fields = $derived(fieldsOf(o, type).filter(([n]) => n !== tf));
  const writable = $derived(!!record && canWrite(record.space));
  const edit = new RecordEdit();
  const secrets = $derived(secretTypes());
  const shown = $derived(edit.active ? fieldsOf(o, type) : fields);

  async function save() {
    const r = await edit.save();
    if (!r) return;
    record = r;
    onsaved?.(r);
  }
  const people = ['user', 'users', 'contact'];

  $effect(() => {
    const [t, i] = [type, id];
    let cancelled = false;
    record = null;
    error = '';
    links = [];
    api
      .get(t, i)
      .then((r) => {
        if (cancelled) return;
        record = r;
        remember(r);
        return api.links(t, i).then((l) => !cancelled && (links = l ?? []));
      })
      .catch((e) => !cancelled && (error = (e as Error).message));
    return () => {
      cancelled = true;
    };
  });

  const related = $derived.by(() => {
    const groups = new Map<string, { type: string; field: string; ids: string[] }>();
    for (const l of links) {
      if (l.to_id !== id || l.from_id === id) continue;
      const from = l.from_type;
      if (!from || !o?.spec.types[from]) continue;
      const key = `${from}.${l.kind}`;
      if (!groups.has(key)) groups.set(key, { type: from, field: l.kind, ids: [] });
      groups.get(key)!.ids.push(l.from_id);
    }
    return [...groups.values()];
  });

  function fieldIcon(f: Field) {
    if (f.type === 'reference' && f.to) return typeIcon(o, f.to);
    const byType: { [t: string]: string } = {
      address: 'map-pin', country: 'map-pin', region: 'map-pin', geo: 'map-pin', percentage: 'percent', duration: 'clock', time: 'clock',
      iban: 'banknote', card: 'credit-card', file: 'document', name: 'user'
    };
    if (byType[f.type]) return byType[f.type];
    const byKind: { [k: string]: string } = {
      text: 'text', richtext: 'text', integer: 'hash', number: 'hash', decimal: 'hash', boolean: 'square-check', enum: 'circle-dot',
      date: 'calendar', timestamp: 'calendar', money: 'banknote', email: 'mail', phone: 'phone', url: 'link'
    };
    return byKind[kindOf(o, f.type)] ?? 'layers';
  }

  function values(f: Field, v: unknown): unknown[] {
    if (v === undefined || v === null || v === '') return [];
    return f.list && Array.isArray(v) ? v : [v];
  }

  function hue(s: string) {
    let h = 0;
    for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
    return h;
  }

  function shortURL(v: string) {
    try {
      const u = new URL(v);
      const rest = u.pathname + u.search;
      return u.host.replace(/^www\./, '') + (rest.length > 1 ? (rest.length > 18 ? rest.slice(0, 16) + '…' : rest) : '');
    } catch {
      return v;
    }
  }

  async function remove() {
    if (!record) return;
    deleting = true;
    try {
      await api.remove(record.type, record.id);
      toast(`Deleted ${recordTitle(o, record)}`, 'success');
      confirm = false;
      ondeleted?.(record);
      onclose();
    } catch (e) {
      fail(e);
    } finally {
      deleting = false;
    }
  }
</script>

<header class="bar">
  <IconButton icon="chevrons-right" label="Close" onclick={onclose} />
  <IconButton icon="maximize" label="Open as page" onclick={() => navigate(`/records/${type}/${id}`)} />
  <span class="crumb"><Icon name={typeIcon(o, type)} size="sm" />{humanize(type)}{#if record?.space}<span class="sep">/</span>{spaceLabel(record.space)}{/if}</span>
  <span class="grow"></span>
  {#if record && writable && !edit.active}<Button variant="ghost" icon="pencil" onclick={() => edit.begin(record!, fieldsOf(o, type))}>Edit</Button>{/if}
  {#if record}
    <Popover bind:open={menu} align="end">
      {#snippet trigger({ toggle })}<IconButton icon="more" label="More" onclick={toggle} />{/snippet}
      {#snippet children({ close })}
        <Menu
          searchable={false}
          items={[
            { id: 'page', label: 'Open as page', icon: 'maximize' },
            { id: 'copy', label: 'Copy link', icon: 'link' },
            ...(canWrite(record!.space) ? [{ id: 'delete', label: 'Delete', icon: 'trash', danger: true, separator: true }] : [])
          ]}
          onselect={async (i) => {
            close();
            if (i.id === 'page') navigate(`/records/${type}/${id}`);
            if (i.id === 'copy') {
              await navigator.clipboard.writeText(`${location.origin}/records/${type}/${id}`).catch(() => {});
              toast('Copied the link', 'success');
            }
            if (i.id === 'delete') confirm = true;
          }}
        />
      {/snippet}
    </Popover>
  {/if}
</header>

<div class="scroll">
  {#if error}
    <Notice tone="danger">{error}</Notice>
  {:else if !record}
    <Skeleton rows={9} />
  {:else}
    <div class="icon"><Icon name={typeIcon(o, type)} size="xl" /></div>
    <h1 class="title">{recordTitle(o, record)}</h1>

    <dl class="props">
      {#each shown as [n, f] (n)}
        {@const vs = values(f, record.data[n])}
        {@const kind = kindOf(o, f.type)}
        <div class="prop">
          <dt><Icon name={fieldIcon(f)} size="sm" /><span>{humanize(n)}</span></dt>
          <dd>
            {#if edit.active}
              <div class="field">
                <FieldInput name={n} field={f} bind:value={edit.values[n]} bind:checked={edit.checks[n]} invalid={!!edit.errors[n]} secret={secrets.has(f.type)} />
                {#if edit.errors[n]}<span class="error">{edit.errors[n]}</span>{/if}
              </div>
            {:else if !vs.length}
              <span class="empty">Empty</span>
            {:else if isSecret(vs[0])}
              <span class="muted"><Icon name="lock" size="xs" /> Hidden</span>
            {:else if kind === 'reference' && f.to}
              {#each vs as v (String(v))}
                {@const title = refTitle(f.to, String(v))}
                <button class="ref" onclick={() => onopen(f.to!, String(v))} onpointerenter={() => preloadRecord(f.to!, String(v))}>
                  {#if people.includes(typeIcon(o, f.to))}<Avatar name={title} shape="circle" size="sm" color={(hue(String(v)) % 5) + 1} />{:else}<Icon name={typeIcon(o, f.to)} size="sm" />{/if}
                  <span class="u">{title}</span>
                </button>
              {/each}
            {:else if kind === 'enum'}
              {#each vs as v (String(v))}<span class="pill" data-tone={hue(String(v)) % 6}><i></i>{humanize(String(v))}</span>{/each}
            {:else if kind === 'boolean'}
              <span class="check" data-on={vs[0] === true || undefined}>{#if vs[0] === true}<Icon name="check" size="xs" />{/if}</span>
            {:else if kind === 'url'}
              {#each vs as v (String(v))}<a class="u" href={String(v)} target="_blank" rel="noreferrer">{shortURL(String(v))}</a>{/each}
            {:else if kind === 'email'}
              {#each vs as v (String(v))}<a class="u" href={`mailto:${v}`}>{String(v)}</a>{/each}
            {:else if kind === 'phone'}
              {#each vs as v (String(v))}<a class="u" href={`tel:${v}`}>{formatValue(o, { ...f, list: false }, v)}</a>{/each}
            {:else}
              <span class="text">{formatValue(o, f, record.data[n])}</span>
            {/if}
          </dd>
        </div>
      {/each}
    </dl>

    {#if related.length}
      <section class="related">
        {#each related as g (`${g.type}.${g.field}`)}
          <h2><Icon name={typeIcon(o, g.type)} size="sm" />{humanize(g.type)}{g.field !== type ? ` · ${humanize(g.field).toLowerCase()}` : ''}<span class="count">{g.ids.length}</span></h2>
          <ul>
            {#each g.ids.slice(0, 12) as rid (rid)}
              <li><button class="row" onclick={() => onopen(g.type, rid)} onpointerenter={() => preloadRecord(g.type, rid)}><span>{refTitle(g.type, rid)}</span><Icon name="arrow-up-right" size="xs" /></button></li>
            {/each}
          </ul>
          {#if g.ids.length > 12}<p class="more">and {g.ids.length - 12} more</p>{/if}
        {/each}
      </section>
    {/if}

  {/if}
</div>

{#if edit.active}
  <footer class="foot">
    {#if edit.failure}<span class="error">{edit.failure}</span>{/if}
    <span class="grow"></span>
    <Button variant="ghost" disabled={edit.busy} onclick={() => edit.cancel()}>Cancel</Button>
    <Button variant="primary" icon="check" loading={edit.busy} onclick={save}>Save</Button>
  </footer>
{/if}

{#if record}
  <Dialog bind:open={confirm} title="Delete this record?" description="It goes from every view, search and link. Its source files stay.">
    {#snippet children()}{/snippet}
    {#snippet footer()}
      <Button variant="ghost" onclick={() => (confirm = false)}>Cancel</Button>
      <Button variant="danger" loading={deleting} onclick={remove}>Delete</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
    border-bottom: var(--border-width) solid var(--tint-2);
  }
  .crumb {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    margin-left: var(--space-2);
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  .sep {
    color: var(--subtle-foreground);
  }
  .grow {
    flex: 1;
  }
  .scroll {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-6) var(--space-5) var(--space-8);
  }
  .icon {
    color: var(--muted-foreground);
    margin-bottom: var(--space-3);
  }
  .icon :global(svg) {
    width: 2.5rem;
    height: 2.5rem;
  }
  .title {
    font-family: var(--font-display);
    font-size: var(--text-2xl);
    font-weight: var(--font-bold);
    letter-spacing: -0.02em;
    line-height: 1.2;
    overflow-wrap: anywhere;
    margin-bottom: var(--space-6);
  }
  .props {
    display: flex;
    flex-direction: column;
  }
  .prop {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    min-height: 2.125rem;
    border-radius: var(--radius-sm);
  }
  dt {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1-5) var(--space-2);
    color: var(--muted-foreground);
    font-size: var(--text-base);
    min-width: 0;
  }
  dt span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  dd {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-1) var(--space-3);
    padding: var(--space-1-5) var(--space-2);
    border-radius: var(--radius-sm);
    min-width: 0;
    min-height: 2.125rem;
  }
  .prop:hover dd {
    background: var(--tint-1);
  }
  .empty {
    color: var(--subtle-foreground);
  }
  .muted {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    color: var(--muted-foreground);
  }
  .text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .u {
    color: inherit;
    text-decoration: underline;
    text-decoration-color: var(--tint-4);
    text-underline-offset: 0.2em;
    overflow-wrap: anywhere;
  }
  a.u:hover,
  .ref:hover .u {
    text-decoration-color: currentColor;
  }
  .ref {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    font-weight: var(--font-medium);
    cursor: pointer;
  }
  .ref :global(.avatar) {
    width: 1.25rem;
    height: 1.25rem;
    font-size: 0.5625rem;
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    padding: 0.0625rem var(--space-2);
    border-radius: var(--radius-full);
    font-size: var(--text-sm);
    --dot: var(--gray-500);
    background: var(--tint-2);
  }
  .pill i {
    width: 0.4375rem;
    height: 0.4375rem;
    border-radius: 50%;
    background: var(--dot);
  }
  [data-tone='1'] { --dot: var(--blue-500); }
  [data-tone='2'] { --dot: var(--emerald-500); }
  [data-tone='3'] { --dot: var(--tangerine-500); }
  [data-tone='4'] { --dot: var(--red-500); }
  [data-tone='5'] { --dot: var(--flame-500); }
  .check {
    display: inline-grid;
    place-items: center;
    width: 1rem;
    height: 1rem;
    border-radius: var(--radius-xs);
    box-shadow: inset 0 0 0 1.5px var(--tint-4);
  }
  .check[data-on] {
    background: var(--accent);
    color: var(--accent-foreground);
    box-shadow: none;
  }
  .related {
    margin-top: var(--space-8);
    padding-top: var(--space-6);
    border-top: var(--border-width) solid var(--tint-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .related h2 {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-3);
    font-size: var(--text-base);
    font-weight: var(--font-semibold);
  }
  .count {
    color: var(--muted-foreground);
    font-weight: var(--font-medium);
  }
  .related ul {
    list-style: none;
    display: flex;
    flex-direction: column;
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-1-5) var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .row span {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row :global(svg) {
    color: var(--muted-foreground);
  }
  .row:hover {
    background: var(--tint-1);
  }
  .more {
    color: var(--muted-foreground);
    font-size: var(--text-sm);
    padding: 0 var(--space-2);
  }
  .field {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }
  .error {
    color: var(--red-500);
    font-size: var(--text-sm);
  }
  .foot {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-top: var(--border-width) solid var(--tint-2);
  }
  @media (max-width: 40rem) {
    .scroll {
      padding: var(--space-6) var(--space-5);
    }
    .prop {
      grid-template-columns: 8.5rem 1fr;
    }
  }
</style>
