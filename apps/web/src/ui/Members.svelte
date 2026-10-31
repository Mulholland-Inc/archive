<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';

  let { space, title = '', group = '', self = '', others = [], sharedTo = [], nameOf, onshare, ongroup, onchange, onclose } = $props();

  const LEVELS = [['owner', 'Owner'], ['admin', 'Admin'], ['editor', 'Can edit'], ['viewer', 'Can view']];
  const path = $derived(`/auth/spaces/${encodeURIComponent(space.name)}/members`);
  const can = $derived(space.level === 'owner' || space.level === 'admin');

  let members = $state(null);
  let people = $state([]);
  let query = $state('');
  let email = $state('');
  let level = $state('viewer');
  let error = $state('');
  let busy = $state(false);
  let named = $state(group);

  const found = $derived((members ?? []).filter((m) => `${m.name} ${m.email}`.toLowerCase().includes(query.trim().toLowerCase())));
  const addable = $derived(people.filter((p) => !members?.some((m) => m.email === p.email)));
  const initials = (m) => (m.name || m.email).split(/\s+/).map((w) => w[0]).slice(0, 2).join('').toUpperCase();

  function loaded(list) {
    members = list ?? [];
    onchange?.(members.length);
  }

  async function set(who, role) {
    busy = true;
    error = '';
    try {
      loaded(await api(path, { method: 'POST', body: { email: who, role } }));
      return true;
    } catch (e) {
      error = e.status === 404 ? `${who} isn't in this archive yet. Add them under People first.` : e.message;
      return false;
    } finally {
      busy = false;
    }
  }

  async function add(e) {
    e.preventDefault();
    if (email.trim() && (await set(email.trim(), level))) email = '';
  }

  api(path).then(loaded).catch((e) => ((members = []), (error = e.message)));
  api('/auth/users').then((p) => (people = p)).catch(() => {});

  const esc = (e) => e.key === 'Escape' && onclose();
</script>

<svelte:window onkeydown={esc} />

<button class="scrim" type="button" aria-label="Close" onclick={onclose}></button>
<div class="sheet" role="dialog" aria-modal="true" aria-label="People in {title}">
  <header>
    <div><h2>{title}</h2><p>{group || 'Only the people here can see this space'}</p></div>
    <button class="close" type="button" aria-label="Close" onclick={onclose}><Icon name="x" /></button>
  </header>

  <div class="body">
    {#if can}
      <label class="field">
        <span>Who is this space for?</span>
        <input bind:value={named} placeholder="Immediate family" onchange={() => ongroup(named.trim())} />
      </label>
    {/if}

    <section>
      <h3>{members ? `${members.length} ${members.length === 1 ? 'person' : 'people'}` : 'People'}</h3>
      {#if (members?.length ?? 0) > 6}
        <label class="search"><Icon name="search" /><input bind:value={query} placeholder="Search people" aria-label="Search people" /></label>
      {/if}
      <ul>
        {#each found as m (m.email)}
          <li>
            <span class="face">{initials(m)}</span>
            <span class="who"><strong>{m.name || m.email}{m.email === self ? ' (you)' : ''}</strong>{#if m.name}<small>{m.email}</small>{/if}</span>
            {#if can}
              <select value={m.level} disabled={busy} aria-label="Access for {m.name || m.email}" onchange={(e) => set(m.email, e.currentTarget.value)}>
                {#each LEVELS as [key, word] (key)}<option value={key}>{word}</option>{/each}
                <option value="none">Remove</option>
              </select>
            {:else}
              <span class="level">{LEVELS.find(([key]) => key === m.level)?.[1]}</span>
            {/if}
          </li>
        {:else}
          {#if members && query}<p class="quiet">Nobody by that name.</p>{/if}
        {/each}
      </ul>
    </section>

    {#if can}
      <form class="add" onsubmit={add}>
        <input bind:value={email} type="email" list="archive-people" placeholder="Add someone by email" aria-label="Email" />
        <datalist id="archive-people">{#each addable as p (p.email)}<option value={p.email}>{p.name}</option>{/each}</datalist>
        <select bind:value={level} aria-label="Access">
          {#each LEVELS.slice(2) as [key, word] (key)}<option value={key}>{word}</option>{/each}
        </select>
        <button type="submit" disabled={busy || !email.trim()}>Add</button>
      </form>
    {/if}
    {#if error}<p class="error">{error}</p>{/if}

    {#if can && others.length}
      <section>
        <h3>Share with another archive <em>Demo</em></h3>
        <div class="to">
          {#each others as a (a.id)}
            <button type="button" aria-pressed={sharedTo.includes(a.id)} onclick={() => onshare(a.id)}>
              <Bot seed={a.id} size={22} /><span>{nameOf(a.id)}</span>{#if sharedTo.includes(a.id)}<Icon name="check" />{/if}
            </button>
          {/each}
        </div>
      </section>
    {/if}
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 1); width: 100%; background: rgb(0 0 0 / 45%); cursor: default; }
  .sheet { position: fixed; top: 50%; left: 50%; z-index: calc(var(--z-nav) + 2); display: flex; flex-direction: column; width: min(480px, calc(100vw - 2 * var(--space-3))); max-height: calc(100svh - 2 * var(--space-3)); overflow: hidden; transform: translate(-50%, -50%); border-radius: var(--radius-panel); background: var(--surface-raised); box-shadow: 0 16px 48px rgb(0 0 0 / 30%); }
  header { display: flex; flex: none; align-items: flex-start; justify-content: space-between; gap: var(--space-2); padding: var(--space-3) var(--space-4); border-bottom: var(--hairline) solid var(--fill); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-sans); text-transform: capitalize; }
  header p { color: var(--fg-muted); font-size: var(--text-meta); }
  .close { display: grid; place-items: center; flex: none; width: 32px; height: 32px; color: var(--fg-muted); }
  .body { display: flex; flex-direction: column; gap: var(--space-3); padding: var(--space-3) var(--space-4) var(--space-4); overflow-y: auto; }
  section { display: flex; flex-direction: column; gap: var(--space-1-5); }
  h3, .field span { color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  h3 em { font-style: normal; opacity: 0.7; }
  .field { display: flex; flex-direction: column; gap: var(--space-1); }
  input { min-width: 0; color: var(--fg); outline: 0; }
  .field input, .add input { flex: 1; padding: var(--space-1-5) var(--space-2); border-radius: var(--radius-nav); background: var(--fill-faint); }
  .search { display: flex; align-items: center; gap: var(--space-1-5); padding: var(--space-1-5) var(--space-2); border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg-muted); }
  .search :global(.icon) { width: 15px; height: 15px; }
  .search input { flex: 1; }
  ul { display: flex; flex-direction: column; max-height: 300px; overflow-y: auto; }
  li { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1) 0; }
  .face { display: grid; place-items: center; flex: none; width: 32px; height: 32px; border-radius: 50%; background: var(--fill); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  .who { display: flex; flex: 1; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  .who strong, .who small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .who strong { font-weight: var(--weight-medium); }
  small, .level, .quiet { color: var(--fg-muted); font-size: var(--text-meta); }
  select { flex: none; padding: var(--space-1) var(--space-1-5); border: 0; border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg); font: inherit; font-size: var(--text-meta); }
  .add { display: flex; align-items: center; gap: var(--space-1); }
  .add button { flex: none; padding: var(--space-1-5) var(--space-3); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  .add button:disabled { opacity: 0.4; }
  .error { color: var(--danger); font-size: var(--text-table); }
  .to { display: flex; flex-direction: column; gap: var(--space-0-5); }
  .to button { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-1); border-radius: var(--radius-nav); text-align: left; }
  .to button:hover { background: var(--fill); }
  .to span { flex: 1; }
  .to :global(.icon) { width: 15px; height: 15px; }
</style>
