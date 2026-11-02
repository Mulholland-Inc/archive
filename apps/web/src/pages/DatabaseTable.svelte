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
  import Heading from '../components/Heading.svelte';
  import Stack from '../components/Stack.svelte';
  import Adaptive from '../components/Adaptive.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import TextLink from '../components/TextLink.svelte';
  import Stats from '../components/Stats.svelte';
  import AvatarGroup from '../components/AvatarGroup.svelte';
  import CodeBlock from '../components/CodeBlock.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { ws, openShare, schema, counts, preview, members, workflows } from '../lib/app.svelte.js';
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

  schema.use();
  counts.use();
  members.use();
  workflows.use();
  preview.use(() => type?.id);
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
      <Breadcrumbs inset items={[{ label: 'Database', href: '/database' }, { label: type?.name ?? 'Not found' }]} />
    {/snippet}
    {#snippet trailing()}
      {#if type}
        <Adaptive inline>{#snippet wide()}<Button size="sm" variant="ghost" icon={Share} onclick={() => openShare('table', type.id)}>Share</Button>{/snippet}</Adaptive>
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
      <Stack as="header" gap="3" align="start">
        <Heading level={1} size="2xl" wrap="anywhere">{type.name}</Heading>
        <Text as="p" size="base" tone="secondary">{summary}</Text>
      </Stack>

      <Stack gap="0">
        <Property label="Rows" icon={Rows3}><Text tabular>{count(type.rows)}</Text></Property>
        <Property label="Columns" icon={Columns3}><Text tabular>{type.columns.length}</Text></Property>
        <Property label="Updated" icon={Clock}>{type.updated}</Property>
        <Property label="Links to" icon={ArrowRight}>
          {#each [...new Set(outgoing.map((l) => l.type))] as t (t.id)}<Chip size="sm" icon={Box} href={`/database/${t.id}`}>{t.name}</Chip>{:else}<Text tone="tertiary">None</Text>{/each}
        </Property>
        <Property label="Linked from" icon={ArrowLeft}>
          {#each [...new Set(incoming.map((l) => l.type))] as t (t.id)}<Chip size="sm" icon={Box} href={`/database/${t.id}`}>{t.name}</Chip>{:else}<Text tone="tertiary">None</Text>{/each}
        </Property>
        <Property label="Used by" icon={Workflow}>
          {#if flows.length}<Text>{flows.length} {flows.length === 1 ? 'workflow' : 'workflows'}</Text>{:else}<Text tone="tertiary">No workflows</Text>{/if}
        </Property>
        <Property label="Shared with" icon={Users}>
          {#if share}<AvatarGroup people={share.people} /><Text tone="secondary">{share.people.length} people</Text>{:else}<Text tone="tertiary">Only archive members</Text>{/if}
        </Property>
      </Stack>

      <Section title="Columns">
        <Table columns={columnColumns} rows={columnRows} label="Columns" cards flush>
          {#snippet cell(row, col)}
            {#if col.key === 'name'}
              <Stack as="span" direction="row" gap="2" align="baseline">
                <Text weight="medium">{row.name}</Text>
                {#if row.required}<Text size="sm" tone="tertiary">Required</Text>{/if}
              </Stack>
            {:else if col.key === 'type'}
              {#if row.to && byId(row.to)}
                <TextLink href={`/database/${row.to}`}>{byId(row.to).name}</TextLink>
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
          <List>
            {#each outgoing as l (`out-${l.column}`)}
              <ListItem href={`/database/${l.type.id}`} icon={ArrowRight}>
                <span><Text weight="medium">{l.column}</Text> <Text tone="secondary">points to</Text> <Text weight="medium">{l.type.name}</Text></span>
              </ListItem>
            {/each}
            {#each incoming as l (`in-${l.type.id}-${l.column}`)}
              <ListItem href={`/database/${l.type.id}`} icon={ArrowLeft}>
                <span><Text weight="medium">{l.type.name}</Text> <Text tone="secondary">points here through</Text> <Text weight="medium">{l.column}</Text></span>
              </ListItem>
            {/each}
          </List>
        {:else}
          <Text tone="tertiary">This table doesn’t link to any other table.</Text>
        {/if}
      </Section>

      <Section title="Workflows">
        {#if flows.length}
          <List>
            {#each flows as w (w.name)}
              <ListItem href={w.id ? `/workflows/${w.id}` : undefined} icon={Workflow}><Text>{w.name}</Text></ListItem>
            {/each}
          </List>
        {:else}
          <Text tone="tertiary">No workflow reads or writes this table.</Text>
        {/if}
      </Section>

      <Section title="Access">
        <Stack gap="0">
          {#each access as a (a.label)}
            <Property label={a.label}>
              {#if a.people.length}
                <Stack as="span" direction="row" gap="2" align="center" grow>
                  <AvatarGroup people={a.people} />
                  <Text tone="secondary" truncate>{a.people.map((p) => p.name).join(', ')}</Text>
                </Stack>
              {:else}
                <Text tone="tertiary">Nobody</Text>
              {/if}
            </Property>
          {/each}
        </Stack>
      </Section>

      <Section title="SQL">
        <CodeBlock code={ddl(type)} lang="sql" />
      </Section>
    </Page>

    {#snippet side()}
      <Stats items={[{ label: 'Rows', value: count(type.rows) }, { label: 'Columns', value: type.columns.length }, { label: 'Links', value: related.length }, { label: 'Updated', value: type.updated }]} />
      <SidebarSection>
        <SidebarItem icon={MessageSquare} label="Ask about this" onclick={ask} />
        <SidebarItem icon={Share} label="Share" onclick={() => openShare('table', type.id)} />
        <SidebarItem icon={Download} label="Download CSV" onclick={exportCsv} />
      </SidebarSection>
      {#if related.length}
        <SidebarSection title="Related tables">
          {#each related as t (t.id)}
            <SidebarItem icon={Box} label={t.name} href={`/database/${t.id}`} badge={count(t.rows)} />
          {/each}
        </SidebarSection>
      {/if}
    {/snippet}
  </RightSidebar>
{:else}
  {@render topbar()}
  <Page center>
    <EmptyState icon={Table2} title="Table not found">
      <Button href="/database">All tables</Button>
    </EmptyState>
  </Page>
{/if}
