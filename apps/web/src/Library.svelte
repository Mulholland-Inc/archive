<script>
  import { API, api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Heading from './ui/Heading.svelte';
  import ListItem from './ui/ListItem.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { views = [], tables = [] } = $props();
  let files = $state(null);

  const links = new Set(['parent_link', 'partnership']);
  const label = (n) => n.replaceAll('_', ' ');
  const kept = $derived(tables.filter((t) => !links.has(t.name)));

  api('/api/files?limit=500').then((all) => (files = all.reverse().slice(0, 6))).catch(() => (files = []));
</script>

<Heading plain title="Library" />
<Sheet>
  <div class="library">
    {#if views.length}
      <section>
        <h3>Lists</h3>
        <div class="cards">
          {#each views as v (v.name)}<a class="card" href="/views/{v.name}"><Icon name={v.icon ?? 'list'} /><span>{label(v.name)}</span></a>{/each}
        </div>
      </section>
    {/if}
    {#if kept.length}
      <section>
        <h3>Everything kept here</h3>
        <div class="cards">
          {#each kept as t (t.name)}<a class="card" href="/{t.name}"><Icon name={t.icon ?? 'table'} /><span>{label(t.name)}</span></a>{/each}
        </div>
      </section>
    {/if}
    <section>
      <div class="top"><h3>Recent files</h3><a href="/files">All files</a></div>
      {#each files ?? [] as f (f.id)}
        <ListItem href="{API}/api/files/{f.id}/content" target="_blank" icon="file" meta={f.access ? [f.access] : []}>{f.name}</ListItem>
      {:else}
        {#if files}<p>No files yet.</p>{/if}
      {/each}
    </section>
  </div>
</Sheet>

<style>
  .library { display: flex; flex-direction: column; gap: var(--space-6); }
  section { display: flex; flex-direction: column; gap: var(--space-2); }
  h3 { color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  .top { display: flex; align-items: center; justify-content: space-between; }
  .top a { color: var(--fg-muted); font-size: var(--text-meta); }
  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: var(--space-2); }
  .card { display: flex; align-items: center; gap: var(--space-2); min-width: 0; padding: var(--space-3); border-radius: var(--radius-item); background: var(--bg); text-transform: capitalize; transition: background var(--duration-fast); }
  .card:hover { background: var(--fill); }
  .card span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  p { color: var(--fg-muted); }
</style>
