<script>
  import { API, api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import ListItem from './ListItem.svelte';

  let { name = '', seed = '', views = [] } = $props();
  let tab = $state('details');
  let files = $state(null);

  api('/api/files?limit=500').then((all) => (files = all.reverse().slice(0, 8))).catch(() => (files = []));
</script>

<aside class="panel" aria-label={name}>
  <Bot {seed} size={72} />
  <h2>{name}</h2>
  <div class="tabs" role="tablist">
    <button type="button" role="tab" aria-selected={tab === 'details'} onclick={() => (tab = 'details')}>Details</button>
    <button type="button" role="tab" aria-selected={tab === 'library'} onclick={() => (tab = 'library')}>Library</button>
  </div>
  {#if tab === 'details'}
    <div class="list">
      {#each views as v (v.name)}
        <a class="link" href="/views/{v.name}"><Icon name={v.icon ?? 'list'} /><span>{v.name.replaceAll('_', ' ')}</span></a>
      {:else}
        <p>Things you keep here will show up as you add them.</p>
      {/each}
    </div>
  {:else}
    <div class="list">
      {#each files ?? [] as f (f.id)}
        <ListItem href="{API}/api/files/{f.id}/content" target="_blank" icon="file">{f.name}</ListItem>
      {:else}
        {#if files}<p>No files yet.</p>{/if}
      {/each}
      <a class="link" href="/files"><Icon name="folder" /><span>All files</span></a>
    </div>
  {/if}
</aside>

<style>
  .panel { display: flex; flex: none; flex-direction: column; align-items: center; gap: var(--space-3); width: 280px; padding: var(--space-6) var(--space-4); overflow-y: auto; }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .tabs { display: flex; gap: var(--space-1); }
  .tabs button { padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-table); transition: background var(--duration-fast); }
  .tabs button[aria-selected='true'] { background: var(--fill); color: var(--fg); }
  .list { display: flex; flex-direction: column; gap: var(--space-0-5); width: 100%; font-size: var(--text-table); }
  .link { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1-5); border-radius: var(--radius-nav); text-transform: capitalize; transition: background var(--duration-fast); }
  .link:hover { background: var(--fill-faint); }
  p { color: var(--fg-muted); text-align: center; }
  @media (max-width: 1023px) { .panel { display: none; } }
</style>
