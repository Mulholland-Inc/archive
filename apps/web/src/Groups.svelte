<script>
  import { api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Chip from './ui/Chip.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { owner = false, people = [], onchange } = $props();
  let error = $state('');
  let busy = $state(false);
  let name = $state('');
  let picked = $state([]);

  const groups = $derived([...new Set(people.flatMap((p) => p.groups ?? []))].sort().map((g) => [g, people.filter((p) => p.groups?.includes(g))]));
  const named = $derived(name.trim().toLowerCase().replace(/\s+/g, '_'));
  const valid = $derived(/^[a-z0-9][a-z0-9_]{0,23}$/.test(named) && picked.length > 0 && !groups.some(([g]) => g === named));
  const who = (p) => p.name || p.email;

  async function save(changes) {
    busy = true;
    try {
      for (const [p, next] of changes) await api(`/api/member/${p.id}`, { method: 'PATCH', body: { groups: next } });
      error = '';
      return true;
    } catch (e) {
      error = e.message;
      return false;
    } finally {
      try {
        onchange(await api('/auth/users'));
      } catch {}
      busy = false;
    }
  }

  async function create(e) {
    e.preventDefault();
    if (busy || !valid) return;
    if (await save(people.filter((p) => picked.includes(p.id)).map((p) => [p, [...(p.groups ?? []), named]]))) {
      name = '';
      picked = [];
    }
  }

  const toggle = (id) => (picked = picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);
  const add = (g, id) => id && save(people.filter((p) => p.id === id).map((p) => [p, [...(p.groups ?? []), g]]));
  const remove = (g, list) => save(list.map((p) => [p, p.groups.filter((x) => x !== g)]));

  api('/auth/users').then((p) => onchange(p)).catch((e) => (error = e.message));
</script>

<Heading plain title="Groups" />
<Sheet>
  {#if error}<Note error>{error}</Note>{/if}
  {#if owner}
    <form class="group" onsubmit={create}>
      <div class="line">
        <input bind:value={name} placeholder="Name" aria-label="Group name" maxlength="24" autocomplete="off" />
        <button class="send" type="submit" disabled={busy || !valid}>Create</button>
      </div>
      <div class="chips">
        {#each people as p (p.id)}
          <button class="pick" type="button" aria-pressed={picked.includes(p.id)} onclick={() => toggle(p.id)}>{who(p)}</button>
        {/each}
      </div>
    </form>
  {/if}
  {#each groups as [g, members] (g)}
    {@const others = people.filter((p) => !members.includes(p))}
    <section class="group">
      <div class="line">
        <h3>{g}</h3>
        {#if owner}<button class="delete" type="button" disabled={busy} title="Delete" aria-label="Delete {g}" onclick={() => remove(g, members)}><Icon name="trash" /></button>{/if}
      </div>
      <div class="chips">
        {#each members as p (p.id)}
          {#if owner}<Chip label={who(p)} onremove={() => remove(g, [p])}>{who(p)}</Chip>{:else}<Chip>{who(p)}</Chip>{/if}
        {/each}
        {#if owner && others.length}
          <select value="" disabled={busy} aria-label="Add to {g}" onchange={(e) => { add(g, e.currentTarget.value); e.currentTarget.value = ''; }}>
            <option value="">Add</option>
            {#each others as p (p.id)}<option value={p.id}>{who(p)}</option>{/each}
          </select>
        {/if}
      </div>
    </section>
  {/each}
</Sheet>

<style>
  .group { display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-3); border-radius: var(--radius-item); background: var(--fill-faint); }
  .line { display: flex; align-items: center; gap: var(--space-2); }
  h3 { flex: 1; font-weight: var(--weight-medium); }
  input { flex: 1; min-width: 0; padding: var(--space-2); border-radius: var(--radius-nav); background: var(--surface); color: var(--fg); outline: 0; }
  input::placeholder { color: var(--fg-muted); }
  .send { flex: none; padding: var(--space-2); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  .send:disabled { opacity: var(--opacity-dim); }
  .chips { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-1); }
  .pick, select { flex: none; padding: var(--space-1); border: 0; border-radius: var(--radius-xs); background: var(--fill); color: var(--fg-muted); font: inherit; font-size: var(--text-meta); line-height: var(--leading-none); outline: 0; }
  .pick[aria-pressed='true'] { background: var(--fill-strong); color: var(--fg); }
  .delete { display: grid; place-items: center; flex: none; color: var(--fg-muted); transition: color var(--duration-fast); }
  .delete:hover { color: var(--danger); }
  .delete:disabled { opacity: var(--opacity-dim); }
</style>
