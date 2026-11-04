<script>
  import { onMount } from 'svelte';
  import { Upload, LayoutGrid, List as ListIcon, Ellipsis, Download, Pencil, MessageSquarePlus, SearchX, FolderPlus, ExternalLink, FileText, Folder, Image, AudioLines, FileSpreadsheet, Inbox } from '@lucide/svelte';
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
  import Skeleton from '../components/Skeleton.svelte';
  import FileInput from '../components/FileInput.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, seedOf, uploadFiles, addFolder, downloadFile, library, members, groups, mayLimit, accessOptions, setFileAccess, folderFiles, fileById, findFiles } from '../lib/app.svelte.js';

  library.use();
  members.use();
  groups.use();
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
    { value: 'recent', label: 'Recently added' },
    { value: 'name', label: 'Name' },
    { value: 'size', label: 'Size' }
  ];
  const views = [
    { value: 'grid', label: 'Grid', icon: LayoutGrid },
    { value: 'list', label: 'List', icon: ListIcon }
  ];
  const columns = [
    { key: 'name', label: 'Name' },
    { key: 'modified', label: 'Changed', muted: true, width: 'sm' },
    { key: 'size', label: 'Size', muted: true, align: 'end', width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ];

  let query = $state('');
  let kind = $state('all');
  // Where in the tree: the folders opened, top first, each a folder's id or a connection's. A connection's has no parent.
  let trail = $state([]);
  let rows = $state([]);
  let found = $state(null);
  let sort = $state('recent');
  let view = $state('grid');
  let openId = $state(route.parts[1] ?? null);
  let preview = $state(Boolean(route.parts[1]));
  let searchEl = $state();

  const here = $derived(trail.at(-1));
  const synced = $derived(trail.some((t) => t.connection));
  let opened = $state(null);
  const open = $derived(opened?.id === openId ? opened : null);
  const sources = $derived(ws().sources.map((s) => ({ id: s.id, name: s.name, kind: 'folder', ext: '', sub: s.account, connection: s.id, source: true, tone: 1 })));
  const bytes = (s) => parseFloat(s) * (s.includes('MB') ? 1024 : 1);
  const listed = $derived(found ?? (here ? rows : [...sources, ...ws().library]));
  const files = $derived.by(() => {
    const out = listed.filter((f) => kind === 'all' || f.kind === kind || f.kind === 'folder');
    const by = sort === 'name' ? (a, b) => a.name.localeCompare(b.name) : sort === 'size' ? (a, b) => (b.bytes ?? bytes(b.size ?? '0')) - (a.bytes ?? bytes(a.size ?? '0')) : () => 0;
    return [...out].sort((a, b) => (b.kind === 'folder') - (a.kind === 'folder') || by(a, b));
  });

  // A folder is read again when it opens, and when what it is read from moves.
  $effect(() => {
    const at = here;
    ws().library;
    if (!at) return;
    folderFiles(at.source ? { connection: at.id } : { parent: at.id }).then((list) => {
      if (here === at) rows = list;
    }).catch(() => {});
  });
  $effect(() => {
    const q = query.trim();
    if (!q) {
      found = null;
      return;
    }
    const wait = setTimeout(() => findFiles(q).then((list) => {
      if (query.trim() === q) found = list;
    }).catch(() => {}), 200);
    return () => clearTimeout(wait);
  });
  $effect(() => {
    const id = openId;
    if (!id) return;
    const known = listed.find((f) => f.id === id);
    if (known) opened = known;
    else fileById(id).then((f) => (opened = f)).catch(() => {});
  });

  function show(file) {
    if (file.kind === 'folder') {
      trail = [...trail, file];
      query = '';
      return;
    }
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
    const made = name && (await addFolder(name, here?.id));
    if (made) trail = [...trail, made];
  }

  let picker = $state();
  let uploading = $state(false);

  function upload() {
    picker?.click();
  }

  async function picked(event) {
    const list = [...event.currentTarget.files];
    event.currentTarget.value = '';
    if (!list.length) return;
    uploading = true;
    const added = await uploadFiles(list, here?.id);
    uploading = false;
    if (kind !== 'all' && added.some((f) => f.kind !== kind)) kind = 'all';
    query = '';
  }

  function dropped(event) {
    if (!event.dataTransfer?.files?.length) return;
    event.preventDefault();
    picked({ currentTarget: { files: event.dataTransfer.files, value: '' } });
  }

  function clear() {
    query = '';
    kind = 'all';
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

<svelte:window ondragover={(e) => e.dataTransfer?.types.includes('Files') && e.preventDefault()} ondrop={dropped} />
<FileInput multiple bind:element={picker} onchange={picked} />

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page width="wide">
  <StickyHeader title="Library" art="bg-library">
    {#snippet search()}
      <SearchField bind:value={query} bind:element={searchEl} placeholder="Search files and folders" shortcut="/" />
    {/snippet}
    {#snippet actions()}
      {#if !synced}
        <IconButton icon={FolderPlus} label="New folder" variant="secondary" onclick={newFolder} />
        <Button variant="primary" icon={Upload} loading={uploading} onclick={upload}>Upload</Button>
      {/if}
    {/snippet}
  </StickyHeader>

  <Stack gap="4">
    {#if trail.length && !found}
      <Stack direction="row" gap="1" align="center" wrap>
        <Button size="sm" variant="ghost" onclick={() => (trail = [])}>Library</Button>
        {#each trail as t, i (t.id)}
          <Text tone="secondary">/</Text>
          <Button size="sm" variant="ghost" onclick={() => (trail = trail.slice(0, i + 1))}>{t.name}</Button>
        {/each}
      </Stack>
    {/if}
    <Stack direction="row" gap="2" align="center" wrap>
      {#each kinds as k (k.value)}
        <Chip selected={kind === k.value} onclick={() => (kind = k.value)}>{k.label}</Chip>
      {/each}
      <Spacer />
      <Select bind:value={sort} options={sorts} variant="ghost" label="Sort by" placement="bottom-end" />
      <SegmentedControl bind:value={view} options={views} iconOnly label="View" />
    </Stack>
  </Stack>

  {#if ws().loading && !listed.length}
    <Grid min="tile" gap="2">{#each { length: 6 } as _, i (i)}<Skeleton variant="block" size="lg" />{/each}</Grid>
  {:else if !listed.length && !found}
    <EmptyState icon={Inbox} title={here ? `${here.name} is empty` : 'No files yet'}>
      {#if !synced}<Button variant="primary" icon={Upload} onclick={upload}>Upload</Button>{/if}
    </EmptyState>
  {:else if !files.length}
    <EmptyState icon={SearchX} title="Nothing matches that search">
      <Button onclick={clear}>Show all files</Button>
    </EmptyState>
  {:else if view === 'grid'}
    <Grid min="tile" gap="2">
      {#each files as file (file.id)}
        <FileTile {file} onclick={() => show(file)} items={file.source ? undefined : () => fileActions(file)}>
          {#snippet menu()}{#if !file.source}{@render fileMenu(file)}{/if}{/snippet}
        </FileTile>
      {/each}
    </Grid>
  {:else}
    <Table {columns} rows={files} label="Files" onrowclick={show} menu={(f) => (f.source ? [] : fileActions(f))}>
      {#snippet cell(row, col)}
        {#if col.key === 'name'}
          <Stack direction="row" gap="3" align="center">
            <Icon icon={row.kind === 'folder' ? Folder : (kinds.find((k) => k.value === row.kind)?.icon ?? FileText)} size="md" />
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
        <SettingsRow label="Who sees it">
          {#if !open.connection && mayLimit(open.access)}
            <Select value={open.access ?? ''} options={accessOptions(open.access)} size="sm" variant="ghost" label={`Who sees ${open.name}`} onchange={(v) => setFileAccess(open, v || null)} />
          {:else}
            <Text tone="secondary">{open.folder}</Text>
          {/if}
        </SettingsRow>
        <SettingsRow label="Changed"><Text tone="secondary">{open.created ? new Date(open.created).toLocaleString() : open.modified}</Text></SettingsRow>
        {#if open.status && open.status !== 'done'}<SettingsRow label="Status"><Text tone="secondary">{open.error || (open.connection ? 'Still copying it' : 'Still reading it')}</Text></SettingsRow>{/if}
        <SettingsRow label="Size"><Text tone="secondary">{open.size}</Text></SettingsRow>
      </Stack>
    </Grid>
  {/if}
  {#snippet footer({ close })}
    {#if open?.url}<Button icon={ExternalLink} onclick={() => window.open(open.url, '_blank', 'noopener')}>Open there</Button>{/if}
    {#if !open?.connection}<Button icon={Pencil} onclick={() => rename(open)}>Rename</Button>{/if}
    <Button icon={Download} onclick={() => downloadFile(open)}>Download</Button>
    <Button variant="primary" icon={MessageSquarePlus} onclick={() => ask(open, close)}>Ask about this</Button>
  {/snippet}
</Dialog>
