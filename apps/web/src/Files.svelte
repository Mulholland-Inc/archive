<script>
  import { API, api, size } from './api.js';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Filters from './ui/Filters.svelte';
  import Heading from './ui/Heading.svelte';
  import Sheet from './ui/Sheet.svelte';
  import Note from './ui/Note.svelte';
  import Empty from './ui/Empty.svelte';
  import { cycle, order, read } from './ui/filter.js';

  let files = $state([]);
  let busy = $state(false);
  let error = $state('');
  let picker = $state();
  let loaded = $state(false);
  let q = $state({ ...read(), values: {} });
  const columns = $derived([
    { name: 'name', type: 'text' },
    { name: 'type', type: 'text' },
    { name: 'size', type: 'int8' },
    { name: 'created_at', label: 'Added', type: 'timestamptz' },
  ]);
  const rows = $derived(files.map((f) => ({
    ...f,
    type: (f.name.match(/\.([a-z0-9]+)$/i)?.[1] ?? f.mime.split('/')[1] ?? '').toUpperCase(),
    href: `${API}/api/files/${f.id}/content`,
  })));
  const shown = $derived.by(() => {
    const words = q.query.trim().toLowerCase().split(/\s+/).filter(Boolean);
    const hits = words.length ? rows.filter((f) => words.every((w) => f.name.toLowerCase().includes(w))) : rows;
    return order(hits, q.sort, columns);
  });

  const day = (v) => new Date(v).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });

  async function load() {
    const all = [];
    for (let after = ''; ; ) {
      const next = await api(`/api/files?limit=500${after ? `&after=${after}` : ''}`);
      all.push(...next);
      if (next.length < 500) break;
      after = next.at(-1).id;
    }
    files = all.reverse();
    loaded = true;
  }

  async function upload(e) {
    busy = true;
    error = '';
    try {
      for (const file of e.target.files) {
        const form = new FormData();
        form.append('file', file);
        await api('/api/files', { method: 'POST', raw: form });
      }
      await load();
    } catch (err) {
      error = err.message;
    }
    busy = false;
    e.target.value = '';
  }

  load().catch((e) => (error = e.message));
</script>

<Heading title="Files">
  <Button small icon="plus" onclick={() => picker.click()} disabled={busy}>{busy ? 'Uploading…' : 'Add files'}</Button>
  <input bind:this={picker} type="file" multiple hidden onchange={upload} />
</Heading>
<Filters bind:q>
  <Sheet tight>
    {#if error}<Note error>{error}</Note>{/if}
    {#if shown.length}
      <DataTable fill {columns} rows={shown} sort={q.sort} onsort={(col) => (q.sort = cycle(q.sort, col))} href={(r) => r.href} target="_blank">
        {#snippet td(r, c)}
          {#if c.name === 'name'}
            <span class="name">
              {#if r.mime.startsWith('image/')}<img src="{API}/api/files/{r.id}/thumbnail" alt="" loading="lazy" />{:else}<Icon name="file" />{/if}
              <span class="text">{r.name}</span>
            </span>
          {:else if c.name === 'size'}{size(r.size)}{:else if c.name === 'created_at'}{day(r.created_at)}{:else}{r[c.name]}{/if}
        {/snippet}
      </DataTable>
    {:else if loaded}
      {#if files.length}<Note>Nothing matches</Note>{:else}<Empty title="No files yet">Add a PDF, a photo or an email to get started.</Empty>{/if}
    {/if}
  </Sheet>
</Filters>

<style>
  .name { display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
  .name :global(.icon) { color: var(--fg-muted); }
  img { flex: none; width: var(--size-icon); height: var(--size-icon); object-fit: cover; }
  .text { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
</style>
