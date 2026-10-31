<script>
  import { api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Avatar from './ui/Avatar.svelte';
  import Chip from './ui/Chip.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { email: self, owner = false, people = [], onchange } = $props();
  let error = $state('');
  let busy = $state(false);
  let email = $state('');
  let role = $state('viewer');

  const ROLES = ['viewer', 'editor', 'owner'];

  async function act(method, url, body) {
    busy = true;
    try {
      const out = await api(url, { method, body });
      onchange(Array.isArray(out) ? out : await api('/auth/users'));
      error = '';
      return true;
    } catch (e) {
      error = e.message;
      return false;
    } finally {
      busy = false;
    }
  }

  async function invite(e) {
    e.preventDefault();
    const who = email.trim();
    if (busy || !who) return;
    if (await act('POST', '/auth/users', { email: who, role })) {
      email = '';
      role = 'viewer';
    }
  }

  async function setRole(p, el) {
    if (!(await act('PATCH', `/api/member/${p.id}`, { role: el.value }))) el.value = p.role;
  }

  act('GET', '/auth/users');
</script>

<Heading plain title="People" />
<Sheet>
  {#if error}<Note error>{error}</Note>{/if}
  {#if owner}
    <form class="invite" onsubmit={invite}>
      <input type="email" bind:value={email} placeholder="Email" aria-label="Email" autocomplete="off" required />
      <select bind:value={role} aria-label="Role">{#each ROLES as r (r)}<option value={r}>{r}</option>{/each}</select>
      <button class="send" type="submit" disabled={busy || !email.trim()}>Invite</button>
    </form>
  {/if}
  <ul>
    {#each people as p (p.id)}
      <li>
        <Avatar name={p.name || p.email} />
        <span class="who">
          <strong>{p.name || p.email}</strong>
          {#if p.name}<small>{p.email}</small>{/if}
        </span>
        {#each p.groups ?? [] as g (g)}<Chip>{g}</Chip>{/each}
        {#if owner}
          <select value={p.role} disabled={busy} aria-label="Role of {p.email}" onchange={(e) => setRole(p, e.currentTarget)}>{#each ROLES as r (r)}<option value={r}>{r}</option>{/each}</select>
          {#if p.email !== self}
            <button class="remove" type="button" disabled={busy} title="Remove" aria-label="Remove {p.email}" onclick={() => act('DELETE', `/auth/users/${encodeURIComponent(p.email)}`)}><Icon name="trash" /></button>
          {/if}
        {:else}
          <Chip>{p.role}</Chip>
        {/if}
      </li>
    {/each}
  </ul>
</Sheet>

<style>
  .invite { display: flex; align-items: center; gap: var(--space-2); }
  input { flex: 1; min-width: 0; padding: var(--space-2); border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg); outline: 0; }
  input::placeholder { color: var(--fg-muted); }
  select { flex: none; padding: var(--space-1); border: 0; border-radius: var(--radius-xs); background: var(--fill); color: var(--fg-muted); font: inherit; font-size: var(--text-meta); outline: 0; }
  .invite select { padding: var(--space-2); border-radius: var(--radius-nav); font-size: var(--text-table); }
  .send { flex: none; padding: var(--space-2); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  .send:disabled { opacity: var(--opacity-dim); }
  ul { display: flex; flex-direction: column; gap: var(--space-1); margin: 0; padding: 0; list-style: none; }
  li { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2); border-radius: var(--radius-item); background: var(--fill-faint); }
  .who { display: flex; flex: 1; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  strong { overflow-wrap: anywhere; font-weight: var(--weight-medium); }
  small { overflow: hidden; color: var(--fg-muted); font-size: var(--text-meta); text-overflow: ellipsis; }
  .remove { display: grid; place-items: center; flex: none; color: var(--fg-muted); transition: color var(--duration-fast); }
  .remove:hover { color: var(--danger); }
  .remove:disabled { opacity: var(--opacity-dim); }
</style>
