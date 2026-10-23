<script lang="ts">
  import {
    Avatar,
    AvatarGroup,
    Board,
    Breadcrumb,
    Button,
    Canvas,
    Checkbox,
    Count,
    Divider,
    Dock,
    FilterBar,
    FilterChip,
    Grid,
    GroupPill,
    HoverCard,
    Icon,
    IconButton,
    Label,
    Menu,
    Popover,
    RecordCard,
    RecordGroup,
    RecordRow,
    RelationChip,
    Row,
    Scroller,
    Section,
    SegmentedControl,
    Select,
    Shell,
    Sidebar,
    SidebarGroup,
    SidebarItem,
    SidebarSection,
    Spacer,
    Specimen,
    Stack,
    Swap,
    Swatch,
    Text,
    Tile,
    Toolbar,
    WorkspaceSwitcher,
    iconNames,
    glyphNames,
    type MenuItem
  } from '../lib';
  import { people, tags, statusItems, filterProperties, records, glyphOf, confidenceOf, statuses } from '../data';

  const tokens = {
    Surfaces: ['--background', '--surface', '--card', '--popover', '--muted', '--border', '--foreground', '--muted-foreground', '--subtle-foreground', '--ring'],
    Intent: ['--primary', '--secondary', '--accent', '--destructive', '--success', '--warning', '--info'],
    Status: ['--status-done', '--status-review', '--status-progress', '--status-todo', '--status-unscoped', '--status-backlog', '--status-canceled', '--status-triage', '--status-blocked', '--status-blocks'],
    Groups: ['--group-done', '--group-review', '--group-progress', '--group-todo', '--group-backlog', '--filter-segment'],
    Labels: ['--label-1', '--label-2', '--label-3', '--label-4', '--label-5', '--avatar-1', '--avatar-2', '--avatar-3', '--avatar-4', '--avatar-5']
  };

  const glyphLabels: { [k: string]: string } = {
    'status-done': 'Done',
    'status-review': 'In review',
    'status-progress': 'In progress',
    'status-todo': 'To do',
    'status-unscoped': 'Unscoped',
    'status-backlog': 'Backlog',
    'status-canceled': 'Canceled',
    'status-duplicate': 'Duplicate',
    'status-triage': 'Triage',
    'status-blocked': 'Blocked by',
    'status-blocks': 'Blocks',
    'status-parent': 'Parent',
    'status-subissue': 'Sub-record',
    'priority-urgent': 'Urgent',
    'priority-high': 'High',
    'priority-medium': 'Medium',
    'priority-low': 'Low',
    'priority-none': 'No priority'
  };

  let segment = $state('list');
  let grouping = $state('status');
  let checked = $state(true);
  let swapKey = $state(0);
  let unchecked = $state(false);
  let menuSelected = $state<string[]>(['Mila Ahn']);
  let popoverOpen = $state(false);
  let query = $state('');
  let groupCollapsed = $state(false);
  let hoverAnchor = $state<HTMLElement | null>(null);
  let filterOn = $state(true);
  let routeDemo = $state('specimen');
  let shellCollapsed = $state(false);
  let sectionOpen = $state(true);
  let board = $state(structuredClone(records.slice(0, 6)));

  const personItems: MenuItem[] = [{ id: 'none', label: 'No reviewer', icon: 'user-square' }, ...people.map((p) => ({ id: p.name, label: p.name, avatar: p }))];

  const columns = $derived(
    (['todo', 'progress', 'review'] as const).map((s) => ({
      id: s,
      status: s,
      label: statuses.find((x) => x.id === s)!.label,
      items: board.filter((r) => r.status === s)
    }))
  );

  function move(id: string, column: string, index: number) {
    const from = board.findIndex((r) => r.id === id);
    const [item] = board.splice(from, 1);
    item.status = column as typeof item.status;
    const peers = board.filter((r) => r.status === column);
    const before = peers[index];
    board.splice(before ? board.indexOf(before) : board.length, 0, item);
  }

</script>

{#snippet sample(r: (typeof records)[number], state: 'rest' | 'lifted' | 'placeholder' | 'floating')}
  {@const c = confidenceOf(r.confidence)}
  <RecordCard id={r.id} glyph={glyphOf(r.status)} title={r.title} labels={r.labels} priority={{ glyph: c.glyph, label: c.label }} person={r.person} {state} />
{/snippet}

<Canvas>
  <Stack gap="2">
    <Text as="h1" variant="title">Archive components</Text>
    <Text variant="muted">Every component in the library, drawn from the same tokens in light and dark.</Text>
  </Stack>

  <Section title="Color" description="Three tiers: primitives, semantic roles and component values. Dark mode redefines only primitives.">
    {#each Object.entries(tokens) as [group, list] (group)}
      <Stack gap="3">
        <Text variant="strong">{group}</Text>
        <Grid min="sm">
          {#each list as token (token)}<Swatch {token} />{/each}
        </Grid>
      </Stack>
    {/each}
  </Section>

  <Section title="Type">
    <Stack gap="3">
      <Text variant="title">Your stuff, finally yours</Text>
      <Text variant="heading">Source evidence</Text>
      <Text variant="strong">Hetzner invoice for October</Text>
      <Text>Archive maps raw files into clean records with calibrated confidence.</Text>
      <Text variant="muted">Documents behind this record</Text>
      <Text variant="small">Proposed on Oct 21 at 10:42 AM</Text>
      <Text variant="mono">REC-48291</Text>
      <Text variant="subtle">Draft 2</Text>
    </Stack>
  </Section>

  <Section title="Status glyphs" description="Drawn at 16 units on a squircle, colored by status tokens, or mono inside a pill.">
    <Grid min="xs">
      {#each glyphNames as name (name)}
        <Row gap="3"><Icon {name} size="xl" /><Text>{glyphLabels[name]}</Text></Row>
      {/each}
    </Grid>
  </Section>

  <Section title="Icons">
    <Grid min="xs">
      {#each iconNames as name (name)}
        <Row gap="3"><Icon {name} size="lg" /><Text variant="muted">{name}</Text></Row>
      {/each}
    </Grid>
  </Section>

  <Section title="Buttons">
    <Grid>
      <Specimen name="Button">
        <Button icon="filter">Filter</Button>
        <Button variant="tint" icon="layers" trailing="chevron-down">Status</Button>
        <Button variant="ghost" icon="plus">New</Button>
        <Button disabled>Disabled</Button>
      </Specimen>
      <Specimen name="IconButton">
        <IconButton icon="settings" label="Settings" />
        <IconButton icon="bell" label="Notifications" />
        <IconButton icon="plus" label="Add" variant="tint" />
        <IconButton icon="compose" label="New" variant="solid" size="lg" />
        <IconButton icon="arrow-right" label="Forward" disabled />
      </Specimen>
      <Specimen name="SegmentedControl, toolbar">
        <SegmentedControl bind:value={segment} label="View" options={[{ value: 'list', label: 'List', icon: 'rows' }, { value: 'board', label: 'Board', icon: 'columns' }]} />
        <SegmentedControl bind:value={routeDemo} label="Page" variant="plain" options={[{ value: 'dashboard', label: 'Dashboard' }, { value: 'specimen', label: 'Specimen' }]} />
      </Specimen>
      <Specimen name="Select">
        <Select icon="layers" bind:value={grouping} options={[{ id: 'status', label: 'Status' }, { id: 'confidence', label: 'Confidence' }]} />
        <Select variant="secondary" icon="list" value="person" options={[{ id: 'person', label: 'Reviewer' }]} />
      </Specimen>
      <Specimen name="Swap">
        <Button icon="refresh" onclick={() => swapKey++}>Swap</Button>
        <Swap key={swapKey} direction={1}><Text>Version {swapKey + 1}</Text></Swap>
      </Specimen>
      <Specimen name="Checkbox">
        <Checkbox checked={checked} label="Checked" onchange={(v) => (checked = v)} />
        <Checkbox checked={unchecked} label="Unchecked" onchange={(v) => (unchecked = v)} />
      </Specimen>
      <Specimen name="Dock">
        <Dock floating={false} bind:route={routeDemo} routes={[{ value: 'dashboard', label: 'Dashboard' }, { value: 'specimen', label: 'Specimen' }]} theme="light" ontheme={() => {}} />
      </Specimen>
    </Grid>
  </Section>

  <Section title="People and labels">
    <Grid>
      <Specimen name="Avatar">
        {#each people as p (p.name)}<Avatar name={p.name} color={p.color} />{/each}
        <Avatar name="Mila Ahn" color={5} shape="circle" size="lg" />
        <Avatar name="Arne Olsen" color={4} ring />
      </Specimen>
      <Specimen name="AvatarGroup">
        <AvatarGroup {people} selected={menuSelected} ontoggle={(n) => (menuSelected = menuSelected.includes(n) ? menuSelected.filter((x) => x !== n) : [...menuSelected, n])} />
      </Specimen>
      <Specimen name="Label" surface="card">
        {#each Object.values(tags).slice(0, 6) as t (t.name)}<Label name={t.name} color={t.color} />{/each}
      </Specimen>
      <Specimen name="Count and Tile">
        <Count value={6} />
        <Count value={102} />
        <Tile icon="code" color={5} />
        <Tile icon="home" color={3} />
        <Tile icon="building" color={4} size="lg" />
      </Specimen>
    </Grid>
  </Section>

  <Section title="Records">
    <Grid>
      <Specimen name="GroupPill">
        <GroupPill status="backlog" label="Inbox" />
        <GroupPill status="todo" label="Queued" />
        <GroupPill status="progress" label="Extracting" />
        <GroupPill status="review" label="In review" />
        <GroupPill status="done" label="Accepted" />
      </Specimen>
      <Specimen name="RelationChip" surface="card">
        <RelationChip glyph="status-subissue" label="Pages" count={2} menu />
        <RelationChip glyph="status-blocked" label="Conflicts with" menu />
        <RelationChip glyph="status-blocks" label="Supersedes" target={[{ glyph: 'status-progress', id: 'REC-213' }]} />
        <RelationChip glyph="status-parent" label="Source" target={[{ glyph: 'status-progress', id: 'REC-213' }]} />
        <RelationChip glyph="status-subissue" label="Pages" target={[{ glyph: 'status-progress', id: 'REC-213' }, { glyph: 'status-todo', id: 'REC-214' }]} />
      </Specimen>
      <Specimen name="FilterChip and FilterBar">
        <Stack gap="3">
          {#if filterOn}<FilterChip icon="priority-medium" field="Confidence" operator="is any of" value="2 levels" onremove={() => (filterOn = false)} />{/if}
          <FilterBar bind:query>
            {#snippet chips()}
              <FilterChip icon="tag" field="Label" operator="is" value="Invoice" />
            {/snippet}
          </FilterBar>
        </Stack>
      </Specimen>
      <Specimen name="Popover and Menu">
        <Popover bind:open={popoverOpen}>
          {#snippet trigger({ toggle })}<Button icon="filter" onclick={toggle}>Filter</Button>{/snippet}
          {#snippet children({ close })}<Menu items={filterProperties} onselect={close} />{/snippet}
        </Popover>
        <Menu items={personItems} multiple selected={menuSelected} onselect={(i) => (menuSelected = menuSelected.includes(i.id) ? menuSelected.filter((x) => x !== i.id) : [...menuSelected, i.id])} searchable={false} />
      </Specimen>
      <Specimen name="RecordCard">
        {@render sample(records[1], 'rest')}
      </Specimen>
      <Specimen name="RecordGroup, RecordRow and HoverCard">
        <Stack gap="0">
          <RecordGroup status="review" label="In review" count={2} bind:collapsed={groupCollapsed}>
            <RecordRow glyph="status-review" title={records[0].title} labels={[tags.invoice]} person={people[0]} statuses={statusItems} status="review" onenter={(el) => (hoverAnchor = el)} onleave={() => (hoverAnchor = null)} />
            <RecordRow glyph="status-review" title={records[1].title} statuses={statusItems} status="review">
              <RelationChip glyph="status-subissue" label="Pages" count={2} menu />
            </RecordRow>
            <RecordRow glyph="status-progress" title={records[2].title} indent statuses={statusItems} status="progress" />
          </RecordGroup>
        </Stack>
        <HoverCard anchor={hoverAnchor}>{@render sample(records[0], 'floating')}</HoverCard>
      </Specimen>
    </Grid>
    <Specimen name="Board, drag cards between columns">
      <Board {columns} onmove={move} card={sample} />
    </Specimen>
  </Section>

  <Section title="Navigation">
    <Specimen name="Shell, Sidebar, SidebarSection, SidebarGroup, SidebarItem, WorkspaceSwitcher, Breadcrumb, Toolbar" surface="background" tall>
      <Shell embedded collapsed={shellCollapsed}>
        {#snippet sidebar()}
          <Sidebar>
            {#snippet header()}
              <WorkspaceSwitcher name="Mulholland" />
              <IconButton icon="sidebar" label="Hide sidebar" onclick={() => (shellCollapsed = true)} />
            {/snippet}
            <SidebarSection>
              <SidebarItem icon="grid" label="My records" />
              <SidebarItem icon="inbox" label="Inbox" />
            </SidebarSection>
            <SidebarSection title="Spaces" bind:open={sectionOpen}>
              <SidebarItem tile={{ icon: 'code', color: 5 }} label="Finance" expanded />
              <SidebarGroup>
                <SidebarItem icon="check" label="Records" active />
                <SidebarItem icon="columns" label="Views" />
              </SidebarGroup>
            </SidebarSection>
          </Sidebar>
        {/snippet}
        {#snippet topbar()}
          <Row gap="1">
            {#if shellCollapsed}<IconButton icon="sidebar" label="Show sidebar" onclick={() => (shellCollapsed = false)} />{/if}
            <Breadcrumb items={[{ label: 'Finance', tile: { icon: 'code', color: 5 } }, { label: 'All records' }]} />
          </Row>
        {/snippet}
        <Toolbar>
          {#snippet start()}
            <Button icon="filter">Filter</Button>
            <Divider vertical />
            <AvatarGroup {people} />
          {/snippet}
          {#snippet end()}
            <IconButton icon="settings" label="Settings" />
          {/snippet}
        </Toolbar>
        <Scroller>
          <Row>
            <Text variant="muted">Content</Text>
            <Spacer />
            <Text variant="mono">REC-196</Text>
          </Row>
        </Scroller>
      </Shell>
    </Specimen>
  </Section>

</Canvas>
