<script>
  import { Upload, LayoutGrid, List as ListIcon, Ellipsis, Download, Pencil, MessageSquarePlus, Trash2, SearchX, FolderOpen, FileText, Image, AudioLines, FileSpreadsheet } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import PageHeader from '../components/PageHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import Select from '../components/Select.svelte';
  import SegmentedControl from '../components/SegmentedControl.svelte';
  import FileTile from '../components/FileTile.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItem from '../components/MenuItem.svelte';
  import MenuSeparator from '../components/MenuSeparator.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Badge from '../components/Badge.svelte';
  import Icon from '../components/Icon.svelte';
  import Text from '../components/Text.svelte';
  import Spacer from '../components/Spacer.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { library } from '../lib/data.js';
  import { app, newChat } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';
  import { toast } from '../lib/toast.svelte.js';

  const kinds = [
    { value: 'all', label: 'All' },
    { value: 'document', label: 'Documents', icon: FileText },
    { value: 'image', label: 'Images', icon: Image },
    { value: 'recording', label: 'Recordings', icon: AudioLines },
    { value: 'pdf', label: 'PDFs', icon: FileText },
    { value: 'spreadsheet', label: 'Spreadsheets', icon: FileSpreadsheet }
  ];
  const sorts = [
    { value: 'recent', label: 'Recently modified' },
    { value: 'name', label: 'Name' },
    { value: 'size', label: 'Size' }
  ];
  const views = [
    { value: 'grid', label: 'Grid', icon: LayoutGrid },
    { value: 'list', label: 'List', icon: ListIcon }
  ];
  const columns = [
    { key: 'name', label: 'Name' },
    { key: 'folder', label: 'Folder', muted: true, width: 'md' },
    { key: 'owner', label: 'Owner', width: 'md' },
    { key: 'modified', label: 'Modified', muted: true, width: 'sm' },
    { key: 'size', label: 'Size', muted: true, align: 'end', width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ];

  let query = $state('');
  let kind = $state('all');
  let sort = $state('recent');
  let view = $state('grid');
  let open = $state(null);
  let preview = $state(false);

  const bytes = (s) => parseFloat(s) * (s.includes('MB') ? 1024 : 1);
  const files = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = library.filter((f) => (kind === 'all' || f.kind === kind) && (!q || f.name.toLowerCase().includes(q) || f.folder.toLowerCase().includes(q)));
    if (sort === 'name') return [...out].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === 'size') return [...out].sort((a, b) => bytes(b.size) - bytes(a.size));
    return out;
  });

  function show(file) {
    open = file;
    preview = true;
  }

  function ask(file, close) {
    close?.();
    const id = newChat(`About ${file.name}`);
    app.pending = { id, text: `What is in ${file.name}?`, attachments: [{ id: file.id, name: file.name, kind: file.kind, size: file.size }] };
    go(`/chat/${id}`);
  }
</script>

{#snippet fileMenu(file)}
  <Menu placement="bottom-end" width="sm" label="File actions">
    {#snippet trigger(props)}<IconButton icon={Ellipsis} label="More" size="sm" tooltip={false} {...props} />{/snippet}
    <MenuItem icon={MessageSquarePlus} onselect={() => ask(file)}>Ask about this</MenuItem>
    <MenuItem icon={Pencil}>Rename</MenuItem>
    <MenuItem icon={Download} onselect={() => toast(`Downloading ${file.name}`)}>Download</MenuItem>
    <MenuSeparator />
    <MenuItem icon={Trash2} danger onselect={() => toast(`${file.name} moved to trash`)}>Move to trash</MenuItem>
  </Menu>
{/snippet}

<TopBar>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <PageHeader title="Library">
    {#snippet actions()}
      <Button icon={FolderOpen}>New folder</Button>
      <Button variant="primary" icon={Upload} onclick={() => toast('3 files uploaded', { tone: 'success' })}>Upload</Button>
    {/snippet}
  </PageHeader>

  <Stack gap="4">
    <Stack direction="row" gap="2" align="center" wrap>
      <SearchField bind:value={query} placeholder="Search files and folders" shortcut="/" />
    </Stack>
    <Stack direction="row" gap="2" align="center" wrap>
      {#each kinds as k (k.value)}
        <Chip selected={kind === k.value} onclick={() => (kind = k.value)}>{k.label}</Chip>
      {/each}
      <Spacer />
      <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
      <SegmentedControl bind:value={view} options={views} iconOnly label="View" />
    </Stack>
  </Stack>

  {#if !files.length}
    <EmptyState icon={SearchX} title="Nothing matches that search">
      <Button onclick={() => { query = ''; kind = 'all'; }}>Clear filters</Button>
    </EmptyState>
  {:else if view === 'grid'}
    <Grid min="tile" gap="2">
      {#each files as file (file.id)}
        <FileTile {file} onclick={() => show(file)}>
          {#snippet menu()}{@render fileMenu(file)}{/snippet}
        </FileTile>
      {/each}
    </Grid>
  {:else}
    <Table {columns} rows={files} label="Files" onrowclick={show}>
      {#snippet cell(row, col)}
        {#if col.key === 'name'}
          <Stack direction="row" gap="3" align="center">
            <Icon icon={kinds.find((k) => k.value === row.kind)?.icon ?? FileText} size="md" />
            <Text weight="medium" truncate>{row.name}</Text>
          </Stack>
        {:else if col.key === 'owner'}
          <Stack direction="row" gap="2" align="center">
            <Avatar name={row.owner} size="xs" />
            <Text truncate>{row.owner}</Text>
          </Stack>
        {:else if col.key === 'menu'}
          {@render fileMenu(row)}
        {:else}
          {row[col.key]}
        {/if}
      {/snippet}
    </Table>
  {/if}
</Page>

<Dialog bind:open={preview} title={open?.name} size="lg">
  {#if open}
    <Grid columns="2" gap="6">
      <FileTile file={open} />
      <Stack gap="0">
        <SettingsRow label="Kind"><Badge>{open.ext}</Badge></SettingsRow>
        <SettingsRow label="Folder"><Text tone="secondary">{open.folder}</Text></SettingsRow>
        <SettingsRow label="Owner">
          <Stack direction="row" gap="2" align="center"><Avatar name={open.owner} size="xs" /><Text tone="secondary">{open.owner}</Text></Stack>
        </SettingsRow>
        <SettingsRow label="Modified"><Text tone="secondary">{open.modified}</Text></SettingsRow>
        <SettingsRow label="Size"><Text tone="secondary">{open.size}</Text></SettingsRow>
      </Stack>
    </Grid>
  {/if}
  {#snippet footer({ close })}
    <Button icon={Download} onclick={() => toast(`Downloading ${open.name}`)}>Download</Button>
    <Button variant="primary" icon={MessageSquarePlus} onclick={() => ask(open, close)}>Ask about this</Button>
  {/snippet}
</Dialog>
