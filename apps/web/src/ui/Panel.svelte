<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';

  let { name = '', seed = '', views = [], spaces = [], home = '', current = '', email = '', onpick, oncreate } = $props();
  let admin = $state(false);
  let adding = $state(false);
  let draft = $state('');
  let failed = $state(false);

  const label = (s) => (s.id === home ? 'Personal' : s.name.replaceAll('_', ' '));

  async function create(e) {
    e.preventDefault();
    const n = draft.trim().toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_]/g, '');
    if (!n) return (adding = false);
    try {
      await oncreate(n);
    } catch {
      failed = true;
    }
  }

  api('/auth/users').then((u) => (admin = !!u.find((p) => p.email === email)?.admin)).catch(() => {});

</script>

<aside class="panel" aria-label={name}>
  <Bot {seed} size={72} />
  <h2>{name}</h2>
    {#if spaces.length > 1 || admin}
      <div class="list">
        <h3>Spaces</h3>
        <button class="link" class:on={!current} type="button" onclick={() => onpick('')}><Icon name="layers" /><span>Everything</span></button>
        {#each spaces as s (s.id)}
          <button class="link" class:on={s.name === current} type="button" onclick={() => onpick(s.name)}><Icon name={s.id === home ? 'user' : 'folder'} /><span>{label(s)}</span></button>
        {/each}
        {#if adding}
          <form class="link" class:failed onsubmit={create}>
            <Icon name="plus" />
            <input bind:value={draft} placeholder="Space name" aria-label="Space name" oninput={() => (failed = false)} {@attach (el) => el.focus()} />
          </form>
        {:else if admin}
          <button class="link quiet" type="button" onclick={() => (adding = true)}><Icon name="plus" /><span>Add space</span></button>
        {/if}
      </div>
    {/if}
    <div class="list">
      {#if views.length}<h3>Lists</h3>{/if}
      {#each views as v (v.name)}
        <a class="link" href="/views/{v.name}"><Icon name={v.icon ?? 'list'} /><span>{v.name.replaceAll('_', ' ')}</span></a>
      {:else}
        <p>Things you keep here will show up as you add them.</p>
      {/each}
    </div>
</aside>

<style>
  .panel { display: flex; flex: none; flex-direction: column; align-items: center; gap: var(--space-3); width: 280px; padding: var(--space-6) var(--space-4); overflow-y: auto; }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .list { display: flex; flex-direction: column; gap: var(--space-0-5); width: 100%; font-size: var(--text-table); }
  .link { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1-5); border-radius: var(--radius-nav); text-transform: capitalize; transition: background var(--duration-fast); }
  .link:hover { background: var(--fill-faint); }
  .link.on { background: var(--fill); }
  .quiet { color: var(--fg-muted); }
  .failed { color: var(--danger); }
  button.link { width: 100%; text-align: left; }
  input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  h3 { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  p { color: var(--fg-muted); text-align: center; }
  @media (max-width: 1023px) { .panel { display: none; } }
</style>
