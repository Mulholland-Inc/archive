<script>
  import { onMount } from 'svelte';
  import { Upload, LayoutGrid, List as ListIcon, Ellipsis, Download, Pencil, MessageSquarePlus, SearchX, FolderPlus, Share, FileText, Image, AudioLines, FileSpreadsheet, Inbox } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
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
  import MenuItems from '../components/MenuItems.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Badge from '../components/Badge.svelte';
  import Icon from '../components/Icon.svelte';
  import Text from '../components/Text.svelte';
  import Spacer from '../components/Spacer.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, seedOf, uploadFile, addFolder, downloadFile, openShare } from '../lib/app.svelte.js';
  import { prompt } from '../lib/dialogs.svelte.js';
  import { route } from '../lib/router.svelte.js';
  import { fileItems, askAboutFile, renameFilePrompt as rename } from '../lib/menus.js';

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
  let folder = $state('all');
  let sort = $state('recent');
  let view = $state('grid');
  let openId = $state(route.parts[1] ?? null);
  let preview = $state(Boolean(route.parts[1]));
  let searchEl = $state();

  const open = $derived(ws().library.find((f) => f.id === openId));
  const folders = $derived([{ value: 'all', label: 'All folders' }, ...ws().folders.map((f) => ({ value: f, label: f }))]);
  const bytes = (s) => parseFloat(s) * (s.includes('MB') ? 1024 : 1);
  const files = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const out = ws().library.filter(
      (f) => (kind === 'all' || f.kind === kind) && (folder === 'all' || f.folder === folder) && (!q || f.name.toLowerCase().includes(q) || f.folder.toLowerCase().includes(q))
    );
    if (sort === 'name') return [...out].sort((a, b) => a.name.localeCompare(b.name));
    if (sort === 'size') return [...out].sort((a, b) => bytes(b.size) - bytes(a.size));
    return out;
  });

  function show(file) {
    openId = file.id;
    preview = true;
  }

  function ask(file, close) {
    close?.();
    askAboutFile(file);
  }

  const fileActions = (file) => fileItems(file, { onopen: show });

  async function newFolder() {
    const name = await prompt({ title: 'New folder', label: 'Name', placeholder: 'Wedding, 1981', confirm: 'Create' });
    if (name) {
      addFolder(name);
      folder = name;
    }
  }

  function upload() {
    const file = uploadFile(folder === 'all' ? undefined : folder);
    if (kind !== 'all' && file.kind !== kind) kind = 'all';
    query = '';
  }

  function clear() {
    query = '';
    kind = 'all';
    folder = 'all';
  }

  onMount(() => {
    const key = (event) => {
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !/INPUT|TEXTAREA/.test(document.activeElement?.tagName ?? '')) {
        event.preventDefault();
        searchEl?.focus();
      }
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  });
</script>

{#snippet fileMenu(file)}
  <Menu placement="bottom-end" width="sm" label="File actions">
    {#snippet trigger(props)}<IconButton icon={Ellipsis} label={`Actions for ${file.name}`} size="sm" tooltip={false} {...props} />{/snippet}
    <MenuItems items={fileItems(file)} />
  </Menu>
{/snippet}

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <StickyHeader title="Library" art="bg-library">
    {#snippet search()}
      <SearchField bind:value={query} bind:element={searchEl} placeholder="Search files and folders" shortcut="/" />
      <Select bind:value={folder} options={folders} variant="ghost" label="Folder" />
    {/snippet}
    {#snippet actions()}
      {#if folder !== 'all'}<IconButton icon={Share} label={`Share ${folder}`} variant="secondary" onclick={() => openShare('folder', folder)} />{/if}
      <IconButton icon={FolderPlus} label="New folder" variant="secondary" onclick={newFolder} />
      <Button variant="primary" icon={Upload} onclick={upload}>Upload</Button>
    {/snippet}
  </StickyHeader>

  <Stack gap="4">
    <Stack direction="row" gap="2" align="center" wrap>
      {#each kinds as k (k.value)}
        <Chip selected={kind === k.value} onclick={() => (kind = k.value)}>{k.label}</Chip>
      {/each}
      <Spacer />
      <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
      <SegmentedControl bind:value={view} options={views} iconOnly label="View" />
    </Stack>
  </Stack>

  {#if !ws().library.length}
    <EmptyState icon={Inbox} title="No files yet">
      <Button variant="primary" icon={Upload} onclick={upload}>Upload</Button>
    </EmptyState>
  {:else if !files.length}
    <EmptyState icon={SearchX} title={folder !== 'all' && !query && kind === 'all' ? `${folder} is empty` : 'Nothing matches that search'}>
      <Button onclick={clear}>Show all files</Button>
      {#if folder !== 'all'}<Button variant="primary" icon={Upload} onclick={upload}>Upload here</Button>{/if}
    </EmptyState>
  {:else if view === 'grid'}
    <Grid min="tile" gap="2">
      {#each files as file (file.id)}
        <FileTile {file} onclick={() => show(file)} items={() => fileActions(file)}>
          {#snippet menu()}{@render fileMenu(file)}{/snippet}
        </FileTile>
      {/each}
    </Grid>
  {:else}
    <Table {columns} rows={files} label="Files" onrowclick={show} menu={fileActions}>
      {#snippet cell(row, col)}
        {#if col.key === 'name'}
          <Stack direction="row" gap="3" align="center">
            <Icon icon={kinds.find((k) => k.value === row.kind)?.icon ?? FileText} size="md" />
            <Text weight="medium" truncate>{row.name}</Text>
          </Stack>
        {:else if col.key === 'owner'}
          <Stack direction="row" gap="2" align="center">
            <Avatar name={row.owner} seed={seedOf(row.owner)} size="xs" />
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
          <Stack direction="row" gap="2" align="center"><Avatar name={open.owner} seed={seedOf(open.owner)} size="xs" /><Text tone="secondary">{open.owner}</Text></Stack>
        </SettingsRow>
        <SettingsRow label="Modified"><Text tone="secondary">{open.modified}</Text></SettingsRow>
        <SettingsRow label="Size"><Text tone="secondary">{open.size}</Text></SettingsRow>
      </Stack>
    </Grid>
  {/if}
  {#snippet footer({ close })}
    <Button icon={Pencil} onclick={() => rename(open)}>Rename</Button>
    <Button icon={Download} onclick={() => downloadFile(open)}>Download</Button>
    <Button variant="primary" icon={MessageSquarePlus} onclick={() => ask(open, close)}>Ask about this</Button>
  {/snippet}
</Dialog>
