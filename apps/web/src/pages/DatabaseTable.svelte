<script>
  import { Box, Share, MessageSquare, Download, Rows3, Columns3, Clock, ArrowRight, ArrowLeft, Workflow, Users, Table2 } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import Breadcrumbs from '../components/Breadcrumbs.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import RightSidebar from '../components/RightSidebar.svelte';
  import DetailsButton from '../components/DetailsButton.svelte';
  import SidebarSection from '../components/SidebarSection.svelte';
  import SidebarItem from '../components/SidebarItem.svelte';
  import Section from '../components/Section.svelte';
  import Property from '../components/Property.svelte';
  import Table from '../components/Table.svelte';
  import Chip from '../components/Chip.svelte';
  import Button from '../components/Button.svelte';
  import Text from '../components/Text.svelte';
  import Icon from '../components/Icon.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import CodeBlock from '../components/CodeBlock.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { ws, openShare, previewTable } from '../lib/app.svelte.js';
  import { typeLabel, ddl } from '../lib/schema.js';
  import { api } from '../lib/api.js';
  import { show } from '../lib/live.js';
  import { askAboutTable } from '../lib/menus.js';

  let { id } = $props();

  const types = $derived(ws().schema);
  const type = $derived(types.find((t) => t.id === id));
  const byId = (tid) => types.find((t) => t.id === tid);
  const outgoing = $derived(type ? type.links.filter((l) => byId(l.to)).map((l) => ({ ...l, type: byId(l.to) })) : []);
  const incoming = $derived(type ? types.flatMap((t) => t.links.filter((l) => l.to === type.id && t.id !== type.id).map((l) => ({ ...l, type: t }))) : []);
  const related = $derived([...new Map([...outgoing, ...incoming].filter((l) => l.type.id !== type?.id).map((l) => [l.type.id, l.type])).values()]);
  const flows = $derived(type ? type.workflows.map((name) => ws().workflows.find((w) => w.file === name || w.name === name) ?? { name }) : []);

  $effect(() => {
    if (type) previewTable(type);
  });
  const share = $derived(type && ws().shares.find((s) => s.kind === 'table' && s.target === type.id));
  const people = (roles) => ws().members.filter((m) => m.status === 'active' && roles.includes(m.role));
  const access = $derived(type ? [
    { label: 'Owners', people: people(['owner']), note: 'Full control' },
    { label: 'Can edit', people: people(type.access.edit), note: 'Add and change rows' },
    { label: 'Can view', people: people(type.access.view), note: 'Read only' },
    { label: 'Shared with', people: share?.people ?? [], note: share ? `Shared by ${share.by}` : '' }
  ] : []);
  const count = (n) => n?.toLocaleString('en-US') ?? '—';
  const joined = (names) => (names.length > 1 ? `${names.slice(0, -1).join(', ')} and ${names.at(-1)}` : names[0]);
  const summary = $derived.by(() => {
    if (!type) return '';
    const names = [...new Set(outgoing.map((l) => l.type.name))];
    return `${type.description ? `${type.description.replace(/\.$/, '')}. ` : ''}${count(type.rows)} ${type.rows === 1 ? 'row' : 'rows'} with ${type.columns.length} columns${names.length ? `, linked to ${joined(names)}` : ''}.`;
  });

  const columnColumns = [
    { key: 'name', label: 'Column' },
    { key: 'type', label: 'Type', width: 'sm', mobile: 'trailing' },
    { key: 'notes', label: 'Notes', muted: true, mobile: 'secondary' }
  ];
  const columnRows = $derived(type ? type.columns.map((c) => ({ ...c, id: c.name })) : []);
  const own = ['id', 'access', 'created_at', 'updated_at'];
  const shownColumns = $derived(type ? type.columns.filter((c) => !own.includes(c.name)).slice(0, 4) : []);
  const sampleColumns = $derived(shownColumns.map((c, i) => ({ key: c.name, label: c.name, muted: i > 0 })));
  const sampleRows = $derived(type ? type.sample.map((r, i) => ({ id: r.id ?? i, ...Object.fromEntries(shownColumns.map((c) => [c.name, show(r[c.name])])) })) : []);

  const ask = () => askAboutTable(type);

  let exporting = false;

  async function exportCsv() {
    if (exporting) return;
    exporting = true;
    const rows = [];
    try {
      for (let after = ''; ; ) {
        const next = await api(`/api/${encodeURIComponent(type.id)}?limit=500${after ? `&after=${after}` : ''}`);
        rows.push(...next);
        if (next.length < 500 || !next.at(-1)?.id) break;
        after = next.at(-1).id;
      }
    } catch {}
    exporting = false;
    const keys = type.columns.map((c) => c.name);
    const cell = (v) => `"${show(v).replaceAll('"', '""')}"`;
    const csv = [keys.join(','), ...rows.map((r) => keys.map((k) => cell(r[k])).join(','))].join('\n');
    const a = Object.assign(document.createElement('a'), { href: URL.createObjectURL(new Blob([csv], { type: 'text/csv' })), download: `${type.id}.csv` });
    a.click();
    URL.revokeObjectURL(a.href);
  }
</script>

{#snippet topbar()}
  <TopBar>
    {#snippet leading()}
      <MobileMenu />
      <Breadcrumbs inset items={[{ label: 'Database', href: '#/database' }, { label: type?.name ?? 'Not found' }]} />
    {/snippet}
    {#snippet trailing()}
      {#if type}
        <span class="wide"><Button size="sm" variant="ghost" icon={Share} onclick={() => openShare('table', type.id)}>Share</Button></span>
        <Button size="sm" variant="primary" icon={MessageSquare} onclick={ask}>Ask about this</Button>
        <DetailsButton />
      {/if}
    {/snippet}
  </TopBar>
{/snippet}

{#if type}
  <RightSidebar art="bg-database" label="Table details">
    {@render topbar()}
    <Page>
      <header class="head">
        <h1>{type.name}</h1>
        <Text as="p" size="base" tone="secondary">{summary}</Text>
      </header>

      <div class="properties">
        <Property label="Rows" icon={Rows3}><Text tabular>{count(type.rows)}</Text></Property>
        <Property label="Columns" icon={Columns3}><Text tabular>{type.columns.length}</Text></Property>
        <Property label="Updated" icon={Clock}>{type.updated}</Property>
        <Property label="Links to" icon={ArrowRight}>
          {#each [...new Set(outgoing.map((l) => l.type))] as t (t.id)}<Chip size="sm" icon={Box} href={`#/database/${t.id}`}>{t.name}</Chip>{:else}<Text tone="tertiary">None</Text>{/each}
        </Property>
        <Property label="Linked from" icon={ArrowLeft}>
          {#each [...new Set(incoming.map((l) => l.type))] as t (t.id)}<Chip size="sm" icon={Box} href={`#/database/${t.id}`}>{t.name}</Chip>{:else}<Text tone="tertiary">None</Text>{/each}
        </Property>
        <Property label="Used by" icon={Workflow}>
          {#if flows.length}<Text>{flows.length} {flows.length === 1 ? 'workflow' : 'workflows'}</Text>{:else}<Text tone="tertiary">No workflows</Text>{/if}
        </Property>
        <Property label="Shared with" icon={Users}>
          {#if share}<AvatarGroup people={share.people} /><Text tone="secondary">{share.people.length} people</Text>{:else}<Text tone="tertiary">Only vault members</Text>{/if}
        </Property>
      </div>

      <Section title="Columns">
        <Table columns={columnColumns} rows={columnRows} label="Columns" cards flush>
          {#snippet cell(row, col)}
            {#if col.key === 'name'}
              <span class="column">
                <Text weight="medium">{row.name}</Text>
                {#if row.required}<Text size="sm" tone="tertiary">Required</Text>{/if}
              </span>
            {:else if col.key === 'type'}
              {#if row.to && byId(row.to)}
                <a class="link" href={`#/database/${row.to}`}>{byId(row.to).name}</a>
              {:else}
                <Text tone="secondary">{typeLabel(row)}</Text>
              {/if}
            {:else}
              {row.notes}
            {/if}
          {/snippet}
        </Table>
      </Section>

      <Section title="Preview">
        {#snippet actions()}<Text size="sm" tone="tertiary">{sampleRows.length} of {count(type.rows)} rows</Text>{/snippet}
        {#if sampleRows.length}
          <Table columns={sampleColumns} rows={sampleRows} label="Sample rows" flush />
        {:else}
          <Text tone="tertiary">No rows yet</Text>
        {/if}
      </Section>

      <Section title="Relationships">
        {#if outgoing.length || incoming.length}
          <ul class="list">
            {#each outgoing as l (`out-${l.column}`)}
              <li><a class="row" href={`#/database/${l.type.id}`}>
                <Icon icon={ArrowRight} size="sm" />
                <span><Text weight="medium">{l.column}</Text> <Text tone="secondary">points to</Text> <Text weight="medium">{l.type.name}</Text></span>
              </a></li>
            {/each}
            {#each incoming as l (`in-${l.type.id}-${l.column}`)}
              <li><a class="row" href={`#/database/${l.type.id}`}>
                <Icon icon={ArrowLeft} size="sm" />
                <span><Text weight="medium">{l.type.name}</Text> <Text tone="secondary">points here through</Text> <Text weight="medium">{l.column}</Text></span>
              </a></li>
            {/each}
          </ul>
        {:else}
          <Text tone="tertiary">This table doesn’t link to any other table.</Text>
        {/if}
      </Section>

      <Section title="Workflows">
        {#if flows.length}
          <ul class="list">
            {#each flows as w (w.name)}
              <li>
                {#if w.id}
                  <a class="row" href={`#/workflows/${w.id}`}><Icon icon={Workflow} size="sm" /><Text>{w.name}</Text></a>
                {:else}
                  <span class="row"><Icon icon={Workflow} size="sm" /><Text>{w.name}</Text></span>
                {/if}
              </li>
            {/each}
          </ul>
        {:else}
          <Text tone="tertiary">No workflow reads or writes this table.</Text>
        {/if}
      </Section>

      <Section title="Access">
        <div class="properties">
          {#each access as a (a.label)}
            <Property label={a.label}>
              {#if a.people.length}
                <span class="who">
                  <AvatarGroup people={a.people} />
                  <Text tone="secondary" truncate>{a.people.map((p) => p.name).join(', ')}</Text>
                </span>
              {:else}
                <Text tone="tertiary">Nobody</Text>
              {/if}
            </Property>
          {/each}
        </div>
      </Section>

      <Section title="SQL">
        <CodeBlock code={ddl(type)} lang="sql" />
      </Section>
    </Page>

    {#snippet side()}
      <dl class="stats">
        <div><dt>Rows</dt><dd>{count(type.rows)}</dd></div>
        <div><dt>Columns</dt><dd>{type.columns.length}</dd></div>
        <div><dt>Links</dt><dd>{related.length}</dd></div>
        <div><dt>Updated</dt><dd>{type.updated}</dd></div>
      </dl>
      <SidebarSection>
        <SidebarItem icon={MessageSquare} label="Ask about this" onclick={ask} />
        <SidebarItem icon={Share} label="Share" onclick={() => openShare('table', type.id)} />
        <SidebarItem icon={Download} label="Download CSV" onclick={exportCsv} />
      </SidebarSection>
      {#if related.length}
        <SidebarSection title="Related tables">
          {#each related as t (t.id)}
            <SidebarItem icon={Box} label={t.name} href={`#/database/${t.id}`} badge={count(t.rows)} />
          {/each}
        </SidebarSection>
      {/if}
    {/snippet}
  </RightSidebar>
{:else}
  {@render topbar()}
  <Page center>
    <EmptyState icon={Table2} title="Table not found">
      <Button href="#/database">All tables</Button>
    </EmptyState>
  </Page>
{/if}

<style>
  .wide {
    display: contents;
  }

  @media (max-width: 40rem) {
    .wide {
      display: none;
    }
  }

  .head {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
  }

  h1 {
    font-family: var(--font-serif);
    font-size: var(--text-3xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
    overflow-wrap: anywhere;
  }

  .properties {
    display: flex;
    flex-direction: column;
  }

  .who {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    min-width: 0;
  }

  .column {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
  }

  .link {
    color: var(--color-text);
    text-decoration: underline;
    text-decoration-color: var(--color-border);
    text-underline-offset: 0.2em;
  }

  .link:hover {
    text-decoration-color: currentColor;
  }

  .list {
    display: flex;
    flex-direction: column;
    margin-inline: calc(var(--space-2) * -1);
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--control-lg);
    padding: var(--space-1-5) var(--space-2);
    border-radius: var(--radius-md);
    color: var(--color-text-tertiary);
    font-size: var(--text-md);
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  a.row:hover {
    background: var(--color-fill-subtle);
  }

  .stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4) var(--space-3);
    padding: var(--space-1) var(--space-1-5);
  }

  dt {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  dd {
    font-size: var(--text-lg);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
  }
</style>
