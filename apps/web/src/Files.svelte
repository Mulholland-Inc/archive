<script>
  import { api, go, run, size } from './api.js';
  import Button from './ui/Button.svelte';
  import Heading from './ui/Heading.svelte';
  import ListItem from './ui/ListItem.svelte';
  import Note from './ui/Note.svelte';
  import Panel from './ui/Panel.svelte';

  let { pages = false } = $props();
  let files = $state([]);
  let busy = $state(false);
  let error = $state('');
  let picker = $state();

  async function load() {
    const all = [];
    for (let after = ''; ; ) {
      const next = await api(`/api/files?limit=500${after ? `&after=${after}` : ''}`);
      all.push(...next);
      if (next.length < 500) break;
      after = next.at(-1).id;
    }
    files = all.filter((f) => f.name.endsWith('.page') === pages).reverse();
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

  async function newPage() {
    const name = prompt('Page name', 'Untitled');
    if (!name) return;
    const id = await run(`return (await archive.create(${JSON.stringify(name + '.page')})).id`);
    go(`/pages/${id}`);
  }

  load().catch((e) => (error = e.message));
</script>

<Heading title={pages ? 'Pages' : 'Files'} />
<div class="actions">
  {#if pages}
    <Button icon="newpage" onclick={newPage}>New page</Button>
  {:else}
    <Button icon="plus" onclick={() => picker.click()} disabled={busy}>{busy ? 'Uploading…' : 'Add files'}</Button>
    <input bind:this={picker} type="file" multiple hidden onchange={upload} />
  {/if}
</div>
<Panel>
  {#if error}<Note error>{error}</Note>{/if}
  {#each files as f (f.id)}
    {@const day = new Date(f.created_at).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })}
    <ListItem
      href={pages ? `/pages/${f.id}` : `/api/files/${f.id}/content`}
      target={pages ? undefined : '_blank'}
      icon={pages ? 'page' : 'file'}
      image={f.mime.startsWith('image/') ? `/api/files/${f.id}/thumbnail` : undefined}
      meta={pages ? [day] : [size(f.size), day]}
    >{pages ? f.name.replace(/\.page$/, '') : f.name}</ListItem>
  {:else}
    <Note>{pages ? 'No pages yet. Start one, or ask an agent to.' : 'No files yet. Add the first ones.'}</Note>
  {/each}
</Panel>

<style>
  .actions { margin-top: var(--space-5); }
</style>
