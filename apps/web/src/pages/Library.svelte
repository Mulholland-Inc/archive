<script>
  import { onMount, untrack } from 'svelte';
  import { Upload, LayoutGrid, List as ListIcon, Ellipsis, Download, Pencil, MessageSquarePlus, SearchX, FolderPlus, ExternalLink, FileText, Image, AudioLines, FileSpreadsheet, Inbox } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Stack from '../components/Stack.svelte';
  import Grid from '../components/Grid.svelte';
  import SearchField from '../components/SearchField.svelte';
  import Chip from '../components/Chip.svelte';
  import SegmentedControl from '../components/SegmentedControl.svelte';
  import FileTile from '../components/FileTile.svelte';
  import Attachment from '../components/Attachment.svelte';
  import Table from '../components/Table.svelte';
  import Button from '../components/Button.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Menu from '../components/Menu.svelte';
  import MenuItems from '../components/MenuItems.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Avatar from '../components/Avatar.svelte';
  import Badge from '../components/Badge.svelte';
  import Text from '../components/Text.svelte';
  import Spacer from '../components/Spacer.svelte';
  import SettingsRow from '../components/SettingsRow.svelte';
  import Skeleton from '../components/Skeleton.svelte';
  import FileInput from '../components/FileInput.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { ws, seedOf, uploadFiles, addFolder, moveFile, downloadFile, library, members, groups, mayLimit, accessOptions, setFileAccess, folderFiles, fileById, findFiles } from '../lib/app.svelte.js';

  library.use();
  members.use();
  groups.use();
  import { prompt, confirmDialog } from '../lib/dialogs.svelte.js';
  import { route, go, replace } from '../lib/router.svelte.js';
  import { fileArt } from '../lib/live.js';
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
    { key: 'folder', label: 'Who sees it', muted: true, width: 'sm' },
    { key: 'menu', label: '', width: 'xs', align: 'end' }
  ];

  let query = $state('');
  let kind = $state('all');
  // Where in the tree, as the URL says: the folders opened, top first, each a folder's id or a connection's. A connection's has no parent.
  let trail = $state([]);
  let rows = $state([]);
  let found = $state(null);
  let sort = $state('recent');
  let view = $state('grid');
  let openId = $state(null);
  let preview = $state(false);
  let moving = $state(null);
  let moveOpen = $state(false);
  let moveTrail = $state([]);
  let moveRows = $state([]);
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

  // /library/<id> is a connection or a folder to go to, or a file to open where it is.
  async function locate(id) {
    try {
      const f = await fileById(id);
      const chain = f.kind === 'folder' ? [f] : [];
      for (let up = f.parent; up; ) {
        const d = await fileById(up);
        chain.unshift(d);
        up = d.parent;
      }
      if (route.parts[1] !== id) return;
      const from = f.connection && sources.find((s) => s.id === f.connection);
      trail = from ? [from, ...chain] : chain;
      if (f.kind !== 'folder') {
        openId = id;
        preview = true;
      }
    } catch {
      if (route.parts[1] === id) replace('/library');
    }
  }

  $effect(() => {
    const id = route.parts[1];
    if (ws().loading) return;
    const known = sources.find((s) => s.id === id);
    untrack(() => {
      if (!id) {
        if (trail.length) trail = [];
        return;
      }
      if (trail.at(-1)?.id === id) return;
      const at = trail.findIndex((t) => t.id === id);
      if (at >= 0) trail = trail.slice(0, at + 1);
      else if (known) trail = [known];
      else locate(id);
    });
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
      // From a search it may be anywhere, so the URL finds its place.
      if (!found) trail = [...trail, file];
      query = '';
      go(`/library/${file.id}`);
      return;
    }
    openId = file.id;
    preview = true;
  }

  function ask(file, close) {
    close?.();
    askAboutFile(file);
  }

  function details(file) {
    openId = file.id;
    preview = true;
  }

  function startMove(file) {
    moving = file;
    moveTrail = [];
    moveOpen = true;
  }

  // Where it can go: the folders here, never itself, and nothing a connection syncs.
  $effect(() => {
    const at = moveTrail.at(-1);
    if (!moveOpen) return;
    const keep = (list) => list.filter((f) => f.kind === 'folder' && !f.connection && f.id !== moving?.id);
    if (!at) {
      moveRows = keep(ws().library);
      return;
    }
    folderFiles({ parent: at.id }).then((list) => {
      if (moveTrail.at(-1) === at) moveRows = keep(list);
    }).catch(() => {});
  });

  async function moveHere() {
    const to = moveTrail.at(-1);
    const file = moving;
    moveOpen = false;
    // At the top it keeps who sees it; in a folder it takes the folder's.
    if (to && to.access !== file.access && !(await confirmDialog({ title: `Move ${file.name}?`, body: `${to.folder} will see it instead of ${file.folder}.`, confirm: 'Move' }))) return;
    moveFile(file, to?.id ?? null);
  }

  const menuItems = (file) => fileItems(file, { onmove: startMove, onaccess: details });
  const fileActions = (file) => fileItems(file, { onopen: show, onmove: startMove, onaccess: details });

  async function newFolder() {
    const name = await prompt({ title: 'New folder', label: 'Name', placeholder: 'Wedding, 1981', confirm: 'Create' });
    const made = name && (await addFolder(name, here?.id));
    if (made) {
      trail = [...trail, made];
      go(`/library/${made.id}`);
    }
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
  <Menu end width="sm" label="File actions">
    {#snippet trigger(props)}<IconButton icon={Ellipsis} label={`Actions for ${file.name}`} size="sm" tooltip={false} {...props} />{/snippet}
    <MenuItems items={menuItems(file)} />
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
        <Button size="sm" variant="ghost" onclick={() => go('/library')}>Library</Button>
        {#each trail as t, i (t.id)}
          <Text tone="secondary">/</Text>
          <Button size="sm" variant="ghost" onclick={() => go(`/library/${t.id}`)}>{t.name}</Button>
        {/each}
      </Stack>
    {/if}
    <Stack direction="row" gap="2" align="center" wrap>
      {#each kinds as k (k.value)}
        <Chip selected={kind === k.value} onclick={() => (kind = k.value)}>{k.label}</Chip>
      {/each}
      <Spacer />
      <Menu bind:value={sort} options={sorts} variant="ghost" label="Sort by" />
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
            <img class="art" src={fileArt(row.name, row.kind)} alt="" />
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
        {#if open.ext}<SettingsRow label="Kind"><Badge>{open.ext}</Badge></SettingsRow>{/if}
        <SettingsRow label="Who sees it">
          {#if !open.connection && !open.parent && mayLimit(open.access)}
            <Menu value={open.access ?? ''} options={accessOptions(open.access)} size="sm" variant="ghost" label={`Who sees ${open.name}`} onchange={(v) => setFileAccess(open, v || null)} />
          {:else}
            <Text tone="secondary">{open.folder}</Text>
          {/if}
        </SettingsRow>
        <SettingsRow label="Changed"><Text tone="secondary">{open.created ? new Date(open.created).toLocaleString() : open.modified}</Text></SettingsRow>
        {#if open.status && open.status !== 'done'}<SettingsRow label="Status"><Text tone="secondary">{open.error || (open.connection ? 'Still copying it' : 'Still reading it')}</Text></SettingsRow>{/if}
        {#if open.held}<SettingsRow label="Size"><Text tone="secondary">{open.size}</Text></SettingsRow>{/if}
      </Stack>
    </Grid>
  {/if}
  {#snippet footer({ close })}
    {#if open?.url}<Button icon={ExternalLink} onclick={() => window.open(open.url, '_blank', 'noopener')}>Open there</Button>{/if}
    {#if !open?.connection}<Button icon={Pencil} onclick={() => rename(open)}>Rename</Button>{/if}
    {#if open?.held}<Button icon={Download} onclick={() => downloadFile(open)}>Download</Button>{/if}
    {#if open?.kind !== 'folder'}<Button variant="primary" icon={MessageSquarePlus} onclick={() => ask(open, close)}>Ask about this</Button>{/if}
  {/snippet}
</Dialog>

<Dialog bind:open={moveOpen} title={moving ? `Move ${moving.name}` : 'Move'}>
  <div class="moves">
    <div class="crumbs">
      <Stack direction="row" gap="1" align="center" wrap>
        <Button size="sm" variant="ghost" onclick={() => (moveTrail = [])}>Library</Button>
        {#each moveTrail as t, i (t.id)}
          <Text tone="secondary">/</Text>
          <Button size="sm" variant="ghost" onclick={() => (moveTrail = moveTrail.slice(0, i + 1))}>{t.name}</Button>
        {/each}
      </Stack>
    </div>
    {#each moveRows as f (f.id)}
      <Attachment row name={f.name} kind="folder" onopen={() => (moveTrail = [...moveTrail, f])} />
    {/each}
  </div>
  {#snippet footer()}
    <Button variant="primary" disabled={(moveTrail.at(-1)?.id ?? null) === (moving?.parent ?? null)} onclick={moveHere}>Move here</Button>
  {/snippet}
</Dialog>

<style>
  /* A row's and a ghost button's padding reach out of the dialog's edge, so what is in them lines up with its title. */
  .moves {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-inline: calc(var(--space-3) * -1);
  }

  .crumbs {
    margin-inline-start: var(--space-1-5);
  }

  .art {
    flex: none;
    width: var(--control-sm);
    height: var(--control-sm);
  }
</style>
