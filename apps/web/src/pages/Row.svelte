<script>
  import { Table2, Type, AlignLeft, CalendarDays, Hash, CircleCheck, ArrowUpRight, Paperclip, AtSign, DollarSign, MapPin, CircleDot, Fingerprint, ArrowLeft, Box } from '@lucide/svelte';
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
  import Chip from '../components/Chip.svelte';
  import Text from '../components/Text.svelte';
  import Heading from '../components/Heading.svelte';
  import Stack from '../components/Stack.svelte';
  import List from '../components/List.svelte';
  import ListItem from '../components/ListItem.svelte';
  import Stats from '../components/Stats.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { ws, schema, rowOf, rowRelations, linkOf, titlesOf } from '../lib/app.svelte.js';
  import { show, relative } from '../lib/live.js';

  // One row of a view, laid out like a table's page: its fields as properties, links by name, and the rows of
  // other tables that point at it.
  let { view: vid, id } = $props();

  schema.use();

  const view = $derived(ws().schema.find((t) => t.id === vid));
  const own = ['id', 'access', 'created_at', 'updated_at'];
  const fields = $derived((view?.columns ?? []).filter((c) => !own.includes(c.name)));
  const title = $derived(fields.find((c) => c.type === 'text') ?? fields[0]);
  const label = (name) => name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());
  const icons = { text: Type, long: AlignLeft, date: CalendarDays, number: Hash, money: DollarSign, yes: CircleCheck, link: ArrowUpRight, file: Paperclip, email: AtSign, place: MapPin, choice: CircleDot, id: Fingerprint };
  const nameOf = (tid) => ws().schema.find((t) => t.id === tid)?.name ?? tid;

  let row = $state(null);
  let missing = $state(false);
  let from = $state([]);
  let titles = $state({});

  $effect(() => {
    const [v, r] = [vid, id];
    if (!view) return;
    row = null;
    missing = false;
    from = [];
    rowOf(v, r).then((x) => {
      if (vid !== v || id !== r) return;
      row = x;
      rowRelations(v, x).then((rel) => vid === v && id === r && (from = rel.from));
    }).catch(() => (missing = true));
  });

  const linked = $derived(view ? [...new Set(fields.map((c) => linkOf(view, c)).filter(Boolean))] : []);
  $effect(() => {
    for (const t of linked) if (!titles[t]) titlesOf(t).then((m) => (titles = { ...titles, [t]: m }));
  });
  const refs = (c) => {
    const v = row?.[c.name];
    if (v == null || v === '') return [];
    return (Array.isArray(v) ? v : [v]).map((rid) => ({ id: rid, title: titles[linkOf(view, c)]?.[rid] ?? '…' }));
  };
  const empty = (v) => v == null || v === '' || (Array.isArray(v) && !v.length);
  const name = $derived(row ? show(row[title?.name]) || view?.name : '');
</script>

{#snippet topbar()}
  <TopBar>
    {#snippet leading()}
      <MobileMenu />
      <Breadcrumbs inset items={[{ label: view?.name ?? 'View', href: `/view/${vid}` }, { label: name || 'Not found' }]} />
    {/snippet}
    {#snippet trailing()}{#if row}<DetailsButton />{/if}{/snippet}
  </TopBar>
{/snippet}

{#if view && row}
  <RightSidebar art="bg-database" label="Row details">
    {@render topbar()}
    <Page>
      <Stack as="header" gap="3" align="start">
        <Heading level={1} size="2xl" wrap="anywhere">{name}</Heading>
        <Text as="p" size="base" tone="secondary">In {view.name}{row.updated_at ? `, changed ${relative(row.updated_at).toLowerCase()}` : ''}.</Text>
      </Stack>

      <Stack gap="0">
        {#each fields.filter((c) => c !== title) as c (c.name)}
          <Property label={label(c.name)} icon={icons[c.type] ?? Type}>
            {#if empty(row[c.name])}<Text tone="tertiary">Empty</Text>
            {:else if linkOf(view, c)}{#each refs(c) as l (l.id)}<Chip size="sm" icon={Box} href={`/view/${linkOf(view, c)}/${l.id}`}>{l.title}</Chip>{/each}
            {:else}<Text wrap="anywhere">{show(row[c.name])}</Text>{/if}
          </Property>
        {/each}
      </Stack>

      {#each from as l (`${l.table}-${l.column}`)}
        <Section title={nameOf(l.table)}>
          <List>
            {#each l.rows as r (r.id)}<ListItem href={`/view/${l.table}/${r.id}`} icon={ArrowLeft}><Text>{r.title}</Text></ListItem>{/each}
          </List>
        </Section>
      {/each}
    </Page>

    {#snippet side()}
      <Stats items={[{ label: 'Created', value: row.created_at ? relative(row.created_at) : '—' }, { label: 'Updated', value: row.updated_at ? relative(row.updated_at) : '—' }, { label: 'Links', value: linked.length }, { label: 'Linked from', value: from.reduce((n, l) => n + l.rows.length, 0) }]} />
      <SidebarSection>
        <SidebarItem icon={Table2} label={`Back to ${view.name}`} href={`/view/${vid}`} />
      </SidebarSection>
    {/snippet}
  </RightSidebar>
{:else}
  {@render topbar()}
  <Page center>
    {#if missing || (!view && ws().schema.length)}<EmptyState icon={Table2} title="Row not found" />{/if}
  </Page>
{/if}
