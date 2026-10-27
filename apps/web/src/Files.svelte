<script>
  import { api, go, run, size } from './api.js';
  import Icon from './Icon.svelte';

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

<h1>{pages ? 'Pages' : 'Files'}</h1>
<p class="lead">{pages ? 'Documents you and your agents write together, live.' : 'Everything in your archive, newest first.'}</p>
<div class="actions">
  {#if pages}
    <button onclick={newPage}><Icon name="newpage" />New page</button>
  {:else}
    <button onclick={() => picker.click()} disabled={busy}><Icon name="plus" />{busy ? 'Uploading…' : 'Add files'}</button>
    <input bind:this={picker} type="file" multiple hidden onchange={upload} />
  {/if}
</div>
<div class="panel">
  {#if error}<p class="error">{error}</p>{/if}
  {#each files as f (f.id)}
    <a class="file" href={pages ? `/pages/${f.id}` : `/api/files/${f.id}/content`} target={pages ? undefined : '_blank'}>
      <span class="thumb">
        {#if f.mime.startsWith('image/')}
          <img src="/api/files/{f.id}/thumbnail" alt="" loading="lazy" />
        {:else}
          <Icon name={pages ? 'page' : 'file'} />
        {/if}
      </span>
      <span class="name">{pages ? f.name.replace(/\.page$/, '') : f.name}</span>
      {#if !pages}<span class="meta">{size(f.size)}</span>{/if}
      <span class="meta">{new Date(f.created_at).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })}</span>
    </a>
  {:else}
    <p class="muted">{pages ? 'No pages yet. Start one, or ask an agent to.' : 'No files yet. Add the first ones.'}</p>
  {/each}
</div>

<style>
  .actions { margin-top: 28px; }
  .file {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 10px 12px;
    margin: 0 -12px;
    border-radius: 16px;
    transition: background var(--dur) var(--ease);
  }
  .file:hover { background: var(--hover); }
  .thumb {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    background: var(--raised);
    color: var(--text-2);
    overflow: hidden;
    flex: none;
  }
  .thumb img { width: 100%; height: 100%; object-fit: cover; }
  .name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--text-2); font-size: 14px; min-width: 56px; text-align: right; }
  .muted { margin: 0; }
  .error { color: var(--alarm); margin: 0 0 16px; }
  @media (max-width: 767px) {
    .meta:last-child { display: none; }
  }
</style>
