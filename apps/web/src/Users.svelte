<script>
  import { api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Chip from './ui/Chip.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { email: self } = $props();
  let people = $state(null);
  let error = $state('');
  let busy = $state(false);
  let email = $state('');
  let admin = $state(false);

  const columns = [
    { name: 'name', type: 'text' },
    { name: 'email', type: 'text' },
    { name: 'admin', label: '', type: 'text' },
    { name: 'act', label: '', type: 'text' },
  ];
  const can = $derived(!!people?.find((p) => p.email === self)?.admin);

  async function act(method, url, body) {
    busy = true;
    try {
      people = await api(url, { method, body });
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
    if (busy || !email.trim()) return;
    if (await act('POST', '/auth/users', { email: email.trim(), admin })) {
      email = '';
      admin = false;
    }
  }

  act('GET', '/auth/users');
</script>

<Heading title="Users" />
<Sheet tight>
  {#if error}<Note error>{error}</Note>{/if}
  {#if can}
    <form class="invite" onsubmit={invite}>
      <input type="email" bind:value={email} placeholder="Email" aria-label="Email" autocomplete="off" required />
      <label class="admin"><input type="checkbox" bind:checked={admin} />Admin</label>
      <button type="submit" title="Invite" aria-label="Invite" disabled={busy || !email.trim()}><Icon name="plus" /></button>
    </form>
  {/if}
  {#if people}
    <DataTable fill {columns} rows={people} key={(r) => r.email}>
      {#snippet td(r, c)}
        {#if c.name === 'name'}{r.name}
        {:else if c.name === 'email'}{r.email}
        {:else if c.name === 'admin'}{#if r.admin}<Chip>Admin</Chip>{/if}
        {:else if can && r.email !== self}
          <span class="acts">
            <button type="button" title="Remove" aria-label="Remove {r.email}" disabled={busy} onclick={() => act('DELETE', `/auth/users/${encodeURIComponent(r.email)}`)}><Icon name="trash" /></button>
          </span>
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Sheet>

<style>
  .invite { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2); }
  .invite input[type='email'] { flex: 1; min-width: 0; color: var(--fg); outline: 0; }
  .invite input::placeholder { color: var(--fg-muted); }
  .admin { display: inline-flex; align-items: center; gap: var(--space-1); color: var(--fg-muted); font-size: var(--text-table); }
  .invite button, .acts button { display: grid; place-items: center; color: var(--fg-muted); transition: color var(--duration-fast); }
  .invite button:hover, .acts button:hover { color: var(--fg); }
  .invite button:disabled, .acts button:disabled { opacity: var(--opacity-dim); }
  .acts { display: flex; justify-content: flex-end; gap: var(--space-2); }
</style>
