<script lang="ts">
  import {
    AuthFrame,
    Avatar,
    Board,
    Breadcrumb,
    Button,
    Canvas,
    Card,
    Checkbox,
    ChoiceCard,
    Code,
    Columns,
    Count,
    DatePicker,
    Dialog,
    Divider,
    Dropzone,
    EmptyState,
    EventChip,
    Field,
    FileCard,
    FilePicker,
    FilePreview,
    FileThumb,
    FilterChip,
    Grid,
    Highlight,
    Icon,
    IconButton,
    Input,
    KeyValue,
    Link,
    Menu,
    MonthGrid,
    Notice,
    Page,
    PageHeader,
    Popover,
    PopoverCard,
    Progress,
    RecordTile,
    ResultRow,
    Row,
    Section,
    SegmentedControl,
    Select,
    Shell,
    Sidebar,
    SidebarItem,
    SidebarSection,
    Skeleton,
    Spacer,
    Specimen,
    Split,
    Stack,
    Swatch,
    Table,
    Text,
    Textarea,
    Tile,
    Toaster,
    WorkspaceSwitcher,
    iconNames
  } from '../lib';
  import { toast } from '../lib/toast.svelte';
  import { iso } from '../lib/calendar';

  const tokens = {
    Surfaces: ['--background', '--surface', '--card', '--popover', '--border', '--foreground', '--muted-foreground', '--subtle-foreground'],
    Intent: ['--primary', '--accent', '--danger', '--success', '--warning', '--info', '--highlight', '--ring'],
    Labels: ['--label-1', '--label-2', '--label-3', '--label-4', '--label-5', '--avatar-1', '--avatar-2', '--avatar-3', '--avatar-4']
  };

  const people = ['Mila Ahn', 'Hana Mori', 'Marco Rossi', 'Arne Olsen'];
  let layout = $state('table');
  let choice = $state('gmail');
  let checked = $state(true);
  let text = $state('');
  let notes = $state('');
  let date = $state('');
  let status = $state('sent');
  let popover = $state(false);
  let dialog = $state(false);
  let collapsed = $state(false);
  let chip = $state(true);
  let progress = $state(0.35);
  let cards = $state([
    { id: 'a', title: 'Hetzner invoice', status: 'sent' },
    { id: 'b', title: 'KPN bill', status: 'paid' },
    { id: 'c', title: 'Dentist', status: 'sent' }
  ]);
  const today = iso(new Date());

  const columns = $derived(['sent', 'paid'].map((s) => ({ id: s, label: s === 'sent' ? 'Sent' : 'Paid', items: cards.filter((c) => c.status === s) })));
</script>

<Canvas>
  <Stack gap="2">
    <Text as="h1" variant="title">Archive components</Text>
    <Text variant="muted">Every component in the dashboard kit, drawn from the tokens in app.css.</Text>
  </Stack>

  <Section title="Color">
    {#each Object.entries(tokens) as [group, list] (group)}
      <Stack gap="3">
        <Text variant="strong">{group}</Text>
        <Grid min="sm">{#each list as token (token)}<Swatch {token} />{/each}</Grid>
      </Stack>
    {/each}
  </Section>

  <Section title="Type and icons">
    <Stack gap="2">
      <Text variant="title">Your stuff, finally yours</Text>
      <Text variant="heading">Source evidence</Text>
      <Text variant="strong">Hetzner invoice for October</Text>
      <Text>Archive reads raw files into clean records.</Text>
      <Text variant="muted">Muted</Text>
      <Text variant="small">Small</Text>
      <Text variant="mono">REC-48291</Text>
      <Row gap="4"><Text tone="success">Up to date</Text><Text tone="warning">Needs sign-in</Text><Text tone="danger">Failing</Text></Row>
      <Highlight text="the **invoice** from Hetzner" />
    </Stack>
    <Grid min="xs">{#each iconNames as name (name)}<Row gap="2"><Icon {name} size="lg" /><Text variant="muted">{name}</Text></Row>{/each}</Grid>
  </Section>

  <Section title="Actions">
    <Grid>
      <Specimen name="Button">
        <Button variant="primary" icon="plus">New</Button>
        <Button icon="upload">Upload</Button>
        <Button variant="tint" icon="layers" trailing="chevron-down">Status</Button>
        <Button variant="ghost">Ghost</Button>
        <Button variant="danger">Delete</Button>
        <Button loading>Saving</Button>
        <Button size="lg" variant="primary">Large</Button>
      </Specimen>
      <Specimen name="IconButton and FilePicker">
        <IconButton icon="settings" label="Settings" />
        <IconButton icon="plus" label="Add" variant="tint" />
        <IconButton icon="more" label="More" size="lg" />
        <FilePicker onpick={(f) => toast(`Picked ${f.length} files`)} />
      </Specimen>
      <Specimen name="SegmentedControl">
        <SegmentedControl
          bind:value={layout}
          label="Layout"
          options={[
            { value: 'table', label: 'Table', icon: 'rows' },
            { value: 'board', label: 'Board', icon: 'columns' },
            { value: 'calendar', label: 'Calendar', icon: 'calendar' }
          ]}
        />
        <SegmentedControl bind:value={layout} label="Layout" variant="plain" options={[{ value: 'table', label: 'Table' }, { value: 'board', label: 'Board' }]} />
      </Specimen>
      <Specimen name="Popover, Menu and PopoverCard">
        <Popover bind:open={popover}>
          {#snippet trigger({ toggle })}<Button icon="filter" onclick={toggle}>Filter</Button>{/snippet}
          {#snippet children({ close })}
            <Menu items={[{ id: 'a', label: 'Edit', icon: 'pencil' }, { id: 'b', label: 'Delete', icon: 'trash', danger: true, separator: true }]} searchable={false} onselect={close} />
          {/snippet}
        </Popover>
        <PopoverCard><Text>Popover content</Text></PopoverCard>
      </Specimen>
      <Specimen name="Dialog and Toaster">
        <Button onclick={() => (dialog = true)}>Open dialog</Button>
        <Button onclick={() => toast('Saved', 'success')}>Toast</Button>
        <Toaster />
      </Specimen>
      <Specimen name="Link and EventChip">
        <Link href="/kit" variant="accent">Accent link</Link>
        <Link href="/kit" variant="subtle">Subtle link</Link>
        <EventChip href="/kit" label="Invoice due" />
      </Specimen>
    </Grid>
  </Section>

  <Section title="Forms">
    <Grid>
      <Specimen name="Field, Input and Textarea" surface="card">
        <Stack gap="3">
          <Field label="Email" hint="We never share it."><Input bind:value={text} type="email" placeholder="you@example.com" label="Email" /></Field>
          <Field label="Search"><Input bind:value={text} type="search" icon="search" placeholder="Search" label="Search" /></Field>
          <Field label="Notes" error="Too short"><Textarea bind:value={notes} invalid label="Notes" /></Field>
        </Stack>
      </Specimen>
      <Specimen name="Select, DatePicker, Checkbox and Columns" surface="card">
        <Stack gap="3">
          <Field label="Status"><Select variant="field" bind:value={status} options={[{ id: 'sent', label: 'Sent' }, { id: 'paid', label: 'Paid' }]} label="Status" /></Field>
          <Field label="Due"><DatePicker bind:value={date} label="Due" /></Field>
          <Row gap="2"><Checkbox {checked} label="Required" onchange={(v) => (checked = v)} /><Text>Required</Text></Row>
          <Columns count={2}><Input placeholder="Left" label="Left" /><Input placeholder="Right" label="Right" /></Columns>
        </Stack>
      </Specimen>
      <Specimen name="ChoiceCard">
        <Stack gap="2">
          <ChoiceCard icon="mail" title="Gmail" description="Read-only through Google" selected={choice === 'gmail'} onclick={() => (choice = 'gmail')} />
          <ChoiceCard icon="folder" title="Folder" description="On the server" selected={choice === 'folder'} onclick={() => (choice = 'folder')} />
        </Stack>
      </Specimen>
      <Specimen name="FilterChip">
        {#if chip}<FilterChip icon="filter" field="Status" operator="is" value="Paid" onremove={() => (chip = false)} />{/if}
      </Specimen>
    </Grid>
  </Section>

  <Section title="Feedback">
    <Grid>
      <Specimen name="Notice">
        <Stack gap="2">
          <Notice>Intake reads files into records.</Notice>
          <Notice tone="warning">One change waits for approval.</Notice>
          <Notice tone="danger">Archive is not reachable.</Notice>
          <Notice tone="success">Applied.</Notice>
        </Stack>
      </Specimen>
      <Specimen name="Progress, Skeleton and Count">
        <Stack gap="3">
          <Progress value={progress} label="Upload" />
          <Button onclick={() => (progress = progress >= 1 ? 0.1 : Math.min(1, progress + 0.3))}>Advance</Button>
          <Skeleton rows={3} />
          <Row gap="2"><Count value={3} /><Count value={128} /></Row>
        </Stack>
      </Specimen>
      <Specimen name="EmptyState">
        <EmptyState icon="table" title="No invoices yet" description="Add one by hand or upload files.">
          <Button variant="primary" icon="plus">New invoice</Button>
        </EmptyState>
      </Specimen>
    </Grid>
  </Section>

  <Section title="Data">
    <Grid>
      <Specimen name="Avatar and Tile">
        {#each people as p, i (p)}<Avatar name={p} color={i + 1} />{/each}
        <Avatar name="Mila Ahn" color={5} shape="circle" size="lg" />
        <Tile icon="code" color={5} />
        <Tile icon="home" color={3} size="lg" />
      </Specimen>
      <Specimen name="RecordTile">
        <RecordTile href="/kit" title="Hetzner invoice" rows={[{ label: 'Total', value: '€48.90' }, { label: 'Due', value: 'Nov 1' }]} />
      </Specimen>
      <Specimen name="ResultRow">
        <Stack gap="0">
          <ResultRow href="/kit" icon="table" title="Hetzner invoice" meta="Invoice"><Highlight text="the **invoice** total" /></ResultRow>
          <ResultRow icon="inbox" title="Unmapped item" meta="gmail" />
        </Stack>
      </Specimen>
      <Specimen name="Code">
        <Stack gap="2">
          <Code value="arc_7vkemkgKDcdrkjhVRCk2qPXsoKJKTsnEjn8jbiJfqw0" />
          <Code value={'Line one\nLine two'} block copy={false} />
        </Stack>
      </Specimen>
      <Specimen name="FileCard, FileThumb and FilePreview">
        <Stack gap="3">
          <Grid min="xs">
            <FileCard href="/kit" mime="application/pdf" name="invoice.pdf" detail="48 KB" />
            <FileThumb mime="image/jpeg" name="photo.jpg" size="sm" />
          </Grid>
          <FilePreview src="/favicon.svg" mime="image/svg+xml" name="Logo" />
        </Stack>
      </Specimen>
      <Specimen name="KeyValue">
        <KeyValue items={[{ k: 'Number', v: 'INV-2026-0141' }, { k: 'Total', v: '€48.90' }]} label={(i) => i.k}>
          {#snippet value(i)}<Text>{i.v}</Text>{/snippet}
        </KeyValue>
      </Specimen>
    </Grid>
    <Specimen name="Table">
      <Table columns={[{ id: 'name', label: 'Name' }, { id: 'total', label: 'Total' }]} rows={cards} key={(r) => r.id} onrowclick={(r) => toast(r.title)}>
        {#snippet cell(r, c)}<Text>{c === 'name' ? r.title : '€48.90'}</Text>{/snippet}
      </Table>
    </Specimen>
    <Specimen name="Board">
      <Board
        {columns}
        onmove={(id, col) => {
          const c = cards.find((x) => x.id === id);
          if (c) c.status = col;
        }}
      >
        {#snippet card(c, state)}<RecordTile href="/kit" title={c.title} {state} />{/snippet}
      </Board>
    </Specimen>
    <Specimen name="MonthGrid">
      <MonthGrid items={[{ id: '1', d: today, t: 'Invoice due' }]} key={(i) => i.id} date={(i) => i.d}>
        {#snippet item(i)}<EventChip href="/kit" label={i.t} />{/snippet}
      </MonthGrid>
    </Specimen>
  </Section>

  <Section title="Layout">
    <Specimen name="Shell, Sidebar, SidebarSection, SidebarItem, WorkspaceSwitcher, Breadcrumb, Page, PageHeader, Split, Card, Divider, Spacer" surface="background" tall>
      <Shell embedded {collapsed}>
        {#snippet sidebar()}
          <Sidebar>
            {#snippet header()}
              <WorkspaceSwitcher name="Archive" detail="All spaces" />
              <IconButton icon="sidebar" label="Hide sidebar" onclick={() => (collapsed = true)} />
            {/snippet}
            <SidebarSection>
              <SidebarItem href="/kit" icon="home" label="Home" active />
              <SidebarItem href="/kit" icon="search" label="Search" />
            </SidebarSection>
            <SidebarSection title="Types">
              <SidebarItem href="/kit" icon="table" label="Invoice" />
            </SidebarSection>
          </Sidebar>
        {/snippet}
        {#snippet topbar()}
          <Row gap="1">
            {#if collapsed}<IconButton icon="sidebar" label="Show sidebar" onclick={() => (collapsed = false)} />{/if}
            <Breadcrumb items={[{ label: 'Invoice' }, { label: 'Hetzner' }]} />
          </Row>
        {/snippet}
        <Page>
          <PageHeader title="Hetzner invoice" description="Invoice in Finance">
            {#snippet actions()}<Button icon="pencil">Edit</Button>{/snippet}
          </PageHeader>
          <Split>
            {#snippet main()}<Card title="Fields"><Text>Main column</Text><Divider /><Row><Text>Left</Text><Spacer /><Text>Right</Text></Row></Card>{/snippet}
            {#snippet aside()}<Card title="Links"><Text variant="muted">Aside</Text></Card>{/snippet}
          </Split>
        </Page>
      </Shell>
    </Specimen>
    <Specimen name="AuthFrame" surface="background" tall>
      <AuthFrame title="Sign in to Archive" description="Your records, files and sources.">
        <Button variant="primary" size="lg" block>Email me a link</Button>
      </AuthFrame>
    </Specimen>
  </Section>
</Canvas>

<Dialog bind:open={dialog} title="Delete this record?" description="It goes from every view and search.">
  <Text>Dialog body</Text>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (dialog = false)}>Cancel</Button>
    <Button variant="danger" onclick={() => (dialog = false)}>Delete</Button>
  {/snippet}
</Dialog>
<Dropzone ondrop={(f) => toast(`Dropped ${f.length} files`)} />
