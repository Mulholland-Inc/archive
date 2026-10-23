<script lang="ts">
  import {
    Shell,
    Sidebar,
    SidebarSection,
    SidebarGroup,
    SidebarItem,
    WorkspaceSwitcher,
    IconButton,
    Breadcrumb,
    Row,
    Toolbar,
    Button,
    Divider,
    AvatarGroup,
    Select,
    SegmentedControl,
    FilterBar,
    FilterChip,
    Popover,
    Menu,
    Scroller,
    Stack,
    Swap,
    RecordGroup,
    RecordRow,
    RelationChip,
    RecordCard,
    Board,
    HoverCard,
    type MenuItem
  } from '../lib';
  import { ms, rise, collapse, pop } from '../lib/motion';
  import {
    records as seed,
    people,
    tags,
    statuses,
    confidences,
    statusItems,
    filterProperties,
    glyphOf,
    confidenceOf,
    type Record,
    type Status,
    type Confidence
  } from '../data';

  type Filter = { key: string; values: string[] };

  let records = $state<Record[]>(structuredClone(seed));
  let collapsed = $state(false);
  let view = $state('list');
  let grouping = $state('status');
  let order = $state('manual');
  let query = $state('');
  let filters = $state<Filter[]>([{ key: 'confidence', values: ['high', 'medium'] }]);
  let menuOpen = $state(false);
  let menuStep = $state<string | null>(null);
  let spaces = $state(true);
  let finance = $state(true);
  let hover = $state<{ el: HTMLElement; record: Record } | null>(null);
  let openPages = $state<string[]>(['REC-197']);
  let groupCollapsed = $state<{ [k: string]: boolean }>({});
  let hoverTimer: ReturnType<typeof setTimeout> | undefined;
  let viewDirection = $state(0);

  const labelList = Object.values(tags).filter((t, i, a) => a.findIndex((x) => x.name === t.name) === i);

  const valueItems: { [k: string]: MenuItem[] } = {
    status: statusItems,
    confidence: confidences.map((c) => ({ id: c.id, label: c.label, glyph: c.glyph })),
    person: [{ id: 'none', label: 'No reviewer', icon: 'user-square' }, ...people.map((p) => ({ id: p.name, label: p.name, avatar: p }))],
    label: labelList.map((t) => ({ id: t.name, label: t.name, icon: 'tag' })),
    source: records.filter((r) => !r.parent).slice(0, 8).map((r) => ({ id: r.id, label: r.id, glyph: glyphOf(r.status) })),
    pages: [{ id: 'yes', label: 'Has pages', glyph: 'status-subissue' }, { id: 'no', label: 'No pages', glyph: 'priority-none' }],
    conflicts: [{ id: 'yes', label: 'Has conflicts', glyph: 'status-blocked' }, { id: 'no', label: 'No conflicts', glyph: 'priority-none' }],
    supersedes: [{ id: 'yes', label: 'Supersedes a record', glyph: 'status-blocks' }, { id: 'no', label: 'Supersedes nothing', glyph: 'priority-none' }]
  };

  const nouns: { [k: string]: string } = {
    status: 'statuses',
    confidence: 'levels',
    person: 'reviewers',
    label: 'labels',
    source: 'sources',
    pages: 'states',
    conflicts: 'states',
    supersedes: 'states'
  };

  function propertyOf(key: string) {
    return filterProperties.find((p) => p.id === key)!;
  }

  function valueLabel(f: Filter) {
    if (f.values.length === 1) return valueItems[f.key].find((v) => v.id === f.values[0])?.label ?? f.values[0];
    return `${f.values.length} ${nouns[f.key]}`;
  }

  function matches(r: Record, f: Filter) {
    if (!f.values.length) return true;
    const flag = (v: boolean) => f.values.includes(v ? 'yes' : 'no');
    switch (f.key) {
      case 'status':
        return f.values.includes(r.status);
      case 'confidence':
        return f.values.includes(r.confidence);
      case 'person':
        return f.values.includes(r.person?.name ?? 'none');
      case 'label':
        return r.labels.some((l) => f.values.includes(l.name));
      case 'source':
        return f.values.includes(r.source ?? r.parent ?? '');
      case 'pages':
        return flag(!!r.pages);
      case 'conflicts':
        return flag(!!r.conflicts?.length);
      case 'supersedes':
        return flag(!!r.supersedes?.length);
    }
    return true;
  }

  const rank = { urgent: 0, high: 1, medium: 2, low: 3, none: 4 };

  const visible = $derived.by(() => {
    const q = query.trim().toLowerCase();
    let list = records.filter((r) => filters.every((f) => matches(r, f)) && (!q || r.title.toLowerCase().includes(q) || r.id.toLowerCase().includes(q)));
    if (order === 'confidence') list = [...list].sort((a, b) => rank[a.confidence] - rank[b.confidence]);
    if (order === 'person') list = [...list].sort((a, b) => (a.person?.name ?? '~').localeCompare(b.person?.name ?? '~'));
    return list;
  });

  const groups = $derived.by(() => {
    if (grouping === 'confidence') {
      return ['urgent', 'high', 'medium', 'low', 'none'].map((c) => {
        const conf = confidenceOf(c as Confidence);
        return { id: c, status: 'todo', glyph: conf.glyph, label: conf.label, items: visible.filter((r) => r.confidence === c) };
      });
    }
    const list = view === 'board' ? ['backlog', 'todo', 'progress', 'review', 'done'] : ['review', 'progress', 'todo', 'backlog', 'done'];
    const extra = statuses.map((s) => s.id).filter((s) => !list.includes(s) && visible.some((r) => r.status === s));
    return [...list, ...extra].map((id) => {
      const s = statuses.find((x) => x.id === id)!;
      return { id, status: id, glyph: undefined, label: s.label, items: visible.filter((r) => r.status === id) };
    });
  });

  const personFilter = $derived(filters.find((f) => f.key === 'person')?.values ?? []);

  function setFilter(key: string, values: string[]) {
    const existing = filters.find((f) => f.key === key);
    if (!values.length) filters = filters.filter((f) => f.key !== key);
    else if (existing) existing.values = values;
    else filters = [...filters, { key, values }];
  }

  function toggleValue(key: string, id: string) {
    const current = filters.find((f) => f.key === key)?.values ?? [];
    setFilter(key, current.includes(id) ? current.filter((v) => v !== id) : [...current, id]);
  }

  function setStatus(r: Record, s: string) {
    const target = records.find((x) => x.id === r.id);
    if (target) target.status = s as Status;
  }

  function move(id: string, column: string, index: number) {
    const from = records.findIndex((r) => r.id === id);
    if (from < 0) return;
    const [item] = records.splice(from, 1);
    if (grouping === 'confidence') item.confidence = column as Confidence;
    else item.status = column as Status;
    order = 'manual';
    const peers = groups.find((g) => g.id === column)?.items.filter((r) => r.id !== id) ?? [];
    const before = peers[index];
    const at = before ? records.findIndex((r) => r.id === before.id) : peers.length ? records.findIndex((r) => r.id === peers[peers.length - 1].id) + 1 : records.length;
    records.splice(at, 0, item);
  }

  let counter = 217;
  function add(column: string) {
    const r: Record = {
      id: `REC-${counter++}`,
      title: 'Untitled record from the Gmail inbox',
      status: grouping === 'status' ? (column as Status) : 'backlog',
      confidence: grouping === 'confidence' ? (column as Confidence) : 'none',
      labels: [tags.gmail]
    };
    records.unshift(r);
    groupCollapsed[column] = false;
  }

  function enter(el: HTMLElement, record: Record) {
    clearTimeout(hoverTimer);
    hoverTimer = setTimeout(() => (hover = { el, record }), hover ? ms('instant') : ms('hover-delay'));
  }

  function leave() {
    clearTimeout(hoverTimer);
    hoverTimer = setTimeout(() => (hover = null), ms('instant'));
  }

  function children(r: Record) {
    return visible.filter((c) => c.parent === r.id);
  }

  function setView(v: string) {
    viewDirection = v === 'board' ? 1 : -1;
    view = v;
  }

  const groupingOptions: MenuItem[] = [
    { id: 'status', label: 'Status', icon: 'layers' },
    { id: 'confidence', label: 'Confidence', icon: 'layers' }
  ];
  const orderOptions: MenuItem[] = [
    { id: 'manual', label: 'Manual', icon: 'list' },
    { id: 'person', label: 'Reviewer', icon: 'list' },
    { id: 'confidence', label: 'Confidence', icon: 'list' }
  ];
  const starred = $derived(['REC-196', 'REC-207', 'REC-208'].map((id) => records.find((r) => r.id === id)!));
</script>

{#snippet relations(r: Record, compact: boolean)}
  {#if r.pages}
    {#if compact}
      <RelationChip glyph="status-subissue" label="Pages" count={r.pages} menu onclick={() => (openPages = openPages.includes(r.id) ? openPages.filter((x) => x !== r.id) : [...openPages, r.id])} />
    {:else}
      {@const pages = records.filter((c) => c.parent === r.id)}
      {#if pages.length}
        <RelationChip glyph="status-subissue" label="Pages" target={pages.map((c) => ({ glyph: glyphOf(c.status), id: c.id }))} />
      {:else}
        <RelationChip glyph="status-subissue" label="Pages" count={r.pages} />
      {/if}
    {/if}
  {/if}
  {#if r.conflicts}
    {#if compact}
      <RelationChip glyph="status-blocked" label="Conflicts with" menu />
    {:else}
      <RelationChip glyph="status-blocked" label="Conflicts with" target={r.conflicts.map((id) => ({ glyph: glyphOf(records.find((x) => x.id === id)?.status ?? 'todo'), id }))} />
    {/if}
  {/if}
  {#if r.supersedes}
    <RelationChip glyph="status-blocks" label="Supersedes" target={r.supersedes.map((id) => ({ glyph: glyphOf(records.find((x) => x.id === id)?.status ?? 'todo'), id }))} />
  {/if}
  {#if r.source && !compact}
    <RelationChip glyph="status-parent" label="Source" target={[{ glyph: glyphOf(records.find((x) => x.id === r.source)?.status ?? 'todo'), id: r.source }]} />
  {/if}
  {#if r.parent && !compact}
    <RelationChip glyph="status-parent" label="Source" target={[{ glyph: glyphOf(records.find((x) => x.id === r.parent)?.status ?? 'todo'), id: r.parent }]} />
  {/if}
{/snippet}

{#snippet card(r: Record, state: 'rest' | 'lifted' | 'placeholder' | 'floating')}
  {@const c = confidenceOf(r.confidence)}
  <RecordCard id={r.id} glyph={glyphOf(r.status)} title={r.title} labels={r.labels} priority={{ glyph: c.glyph, label: c.label }} person={r.person} {state}>
    {#if r.pages || r.conflicts || r.supersedes || r.source || r.parent}{@render relations(r, false)}{/if}
  </RecordCard>
{/snippet}

{#snippet row(r: Record, indent: boolean)}
  <div in:rise out:collapse>
    <RecordRow
      glyph={glyphOf(r.status)}
      title={r.title}
      labels={indent ? [] : r.labels.slice(0, 1)}
      person={r.person}
      {indent}
      statuses={statusItems}
      status={r.status}
      onstatus={(s) => setStatus(r, s)}
      onenter={(el) => enter(el, r)}
      onleave={leave}
    >
      {#if !indent && (r.pages || r.conflicts || r.supersedes)}{@render relations(r, true)}{/if}
    </RecordRow>
  </div>
{/snippet}

<Shell {collapsed}>
  {#snippet sidebar()}
    <Sidebar>
      {#snippet header()}
        <WorkspaceSwitcher name="Mulholland" />
        <IconButton icon="sidebar" label="Hide sidebar" onclick={() => (collapsed = true)} />
      {/snippet}
      <SidebarSection>
        <SidebarItem icon="grid" label="My records" />
        <SidebarItem icon="inbox" label="Inbox" />
        <SidebarItem icon="note" label="My notes" />
      </SidebarSection>
      <SidebarSection title="Starred">
        {#each starred as r (r.id)}
          <SidebarItem glyph={glyphOf(r.status)} label={r.title} />
        {/each}
      </SidebarSection>
      <SidebarSection title="Spaces" bind:open={spaces}>
        <SidebarItem tile={{ icon: 'code', color: 5 }} label="Finance" expanded={finance} onclick={() => (finance = !finance)} />
        <SidebarGroup open={finance}>
          <SidebarItem icon="layers" label="Intake" />
          <SidebarItem icon="check" label="Records" active />
          <SidebarItem icon="columns" label="Views" />
        </SidebarGroup>
        <SidebarItem tile={{ icon: 'home', color: 3 }} label="Household" trailing />
        <SidebarItem tile={{ icon: 'building', color: 4 }} label="Company" trailing />
      </SidebarSection>
      {#snippet footer()}
        <IconButton icon="help" label="Help" size="lg" />
        <IconButton icon="compose" label="New record" variant="solid" size="lg" onclick={() => add('backlog')} />
        <IconButton icon="search" label="Search" size="lg" />
      {/snippet}
    </Sidebar>
  {/snippet}

  {#snippet topbar()}
    <Row gap="1">
      {#if collapsed}<IconButton icon="sidebar" label="Show sidebar" onclick={() => (collapsed = false)} />{/if}
      <IconButton icon="arrow-left" label="Back" />
      <IconButton icon="arrow-right" label="Forward" disabled />
      <Breadcrumb items={[{ label: 'Finance', tile: { icon: 'code', color: 5 } }, { label: 'All records' }]} />
      <IconButton icon="more" label="More" />
    </Row>
  {/snippet}

  <Toolbar>
    {#snippet start()}
      <Button icon="filter" pressed={menuOpen} onclick={() => { menuStep = null; menuOpen = !menuOpen; }}>Filter</Button>
      <Divider vertical />
      <AvatarGroup people={people} selected={personFilter} ontoggle={(name) => toggleValue('person', name)} />
    {/snippet}
    {#snippet end()}
      <Select icon="layers" bind:value={grouping} options={groupingOptions} />
      <Select icon="list" bind:value={order} options={orderOptions} />
      <SegmentedControl
        label="View"
        options={[{ value: 'list', label: 'List', icon: 'rows' }, { value: 'board', label: 'Board', icon: 'columns' }]}
        bind:value={() => view, setView}
      />
      <IconButton icon="settings" label="View settings" />
      <IconButton icon="bell" label="Notifications" />
    {/snippet}
  </Toolbar>

  <Scroller axis={view === 'board' ? 'both' : 'y'}>
    <Stack gap="5">
      <FilterBar bind:query focused={menuOpen} onfocus={() => { if (!menuOpen) { menuStep = null; menuOpen = true; } }}>
        {#snippet chips()}
          {#each filters as f (f.key)}
            <span in:pop out:pop>
              <FilterChip
                icon={(propertyOf(f.key).glyph ?? propertyOf(f.key).icon)!}
                field={propertyOf(f.key).label}
                operator={f.values.length > 1 ? 'is any of' : 'is'}
                value={valueLabel(f)}
                onvalue={() => { menuStep = f.key; menuOpen = true; }}
                onremove={() => (filters = filters.filter((x) => x.key !== f.key))}
              />
            </span>
          {/each}
          <Popover bind:open={menuOpen} onclose={() => (menuStep = null)}>
            {#snippet trigger()}<span></span>{/snippet}
            {#snippet children()}
              {#key menuStep}
                {#if menuStep}
                  <Menu
                    items={valueItems[menuStep]}
                    multiple
                    placeholder={`${propertyOf(menuStep).label}…`}
                    selected={filters.find((f) => f.key === menuStep)?.values ?? []}
                    onselect={(item) => toggleValue(menuStep!, item.id)}
                  />
                {:else}
                  <Menu items={filterProperties} onselect={(item) => (menuStep = item.id)} />
                {/if}
              {/key}
            {/snippet}
          </Popover>
        {/snippet}
      </FilterBar>

      <Swap key={view} direction={viewDirection}>
        {#if view === 'list'}
          <Stack gap="0">
            {#each groups.filter((g) => g.items.length) as g (g.id)}
              <RecordGroup status={g.status} glyph={g.glyph} label={g.label} count={g.items.length} bind:collapsed={() => !!groupCollapsed[g.id], (v) => (groupCollapsed[g.id] = v)} onadd={() => add(g.id)}>
                {#each g.items.filter((r) => !r.parent || !visible.some((p) => p.id === r.parent)) as r (r.id)}
                  {@render row(r, false)}
                  {#if openPages.includes(r.id)}
                    {#each children(r) as c (c.id)}{@render row(c, true)}{/each}
                  {/if}
                {/each}
              </RecordGroup>
            {/each}
          </Stack>
        {:else}
          <Board columns={groups} onmove={move} onadd={add} {card} />
        {/if}
      </Swap>
    </Stack>
  </Scroller>
</Shell>

<HoverCard anchor={hover?.el ?? null}>
  {#if hover}{@render card(hover.record, 'floating')}{/if}
</HoverCard>
