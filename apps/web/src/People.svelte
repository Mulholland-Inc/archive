<script>
  import { api, workspace } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Avatar from './ui/Avatar.svelte';
  import Bot from './ui/Bot.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';

  let { email: self, archive = 'this archive' } = $props();
  let people = $state(null);
  let error = $state('');
  let busy = $state(false);
  let email = $state('');
  let admin = $state(false);
  let added = $state('');

  const can = $derived(!!people?.find((p) => p.email === self)?.admin);
  const alone = $derived(people?.length === 1);

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
    const who = email.trim();
    if (busy || !who) return;
    if (await act('POST', '/auth/users', { email: who, admin })) {
      added = who;
      email = '';
      admin = false;
    }
  }

  act('GET', '/auth/users');
</script>

<Heading title="People" />
<div class="people">
  {#if error}<Note error>{error}</Note>{/if}

  {#if can}
    <form class="invite" onsubmit={invite}>
      <h2>Invite someone to {archive}</h2>
      <div class="line">
        <input type="email" bind:value={email} placeholder="Their email address" aria-label="Email address" autocomplete="off" required oninput={() => (added = '')} />
        <button class="send" type="submit" disabled={busy || !email.trim()}>Invite</button>
      </div>
      <label class="manage"><input type="checkbox" bind:checked={admin} /><span>They can also manage people and settings</span></label>
      {#if added}<p class="added"><Icon name="check" />{added} can now sign in with that email.</p>{/if}
    </form>
  {/if}

  {#if people}
    <section>
      <h3>{people.length === 1 ? '1 person' : `${people.length} people`}</h3>
      <ul>
        {#each people as p (p.email)}
          <li>
            <Avatar name={p.name || p.email} />
            <span class="who">
              <strong>{p.name || p.email}{#if p.email === self}<em>You</em>{/if}</strong>
              {#if p.name}<small>{p.email}</small>{/if}
            </span>
            <span class="role">{p.admin ? 'Admin' : 'Member'}</span>
            {#if can && p.email !== self}
              <button class="remove" type="button" disabled={busy} aria-label="Remove {p.email}" onclick={() => act('DELETE', `/auth/users/${encodeURIComponent(p.email)}`)}>Remove</button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
    {#if alone}
      <div class="alone"><Bot seed={workspace()} size={84} bare /><p>It's just you so far.</p></div>
    {/if}
  {/if}
</div>

<style>
  .people { display: flex; flex: 1; flex-direction: column; gap: var(--space-5); width: 100%; max-width: var(--composer-width); margin-inline: auto; padding: var(--space-3) var(--gutter) calc(var(--gutter) + var(--dock-lift)); overflow-y: var(--sheet-overflow); }
  .invite { display: flex; flex: none; flex-direction: column; gap: var(--space-2); padding: var(--space-4); border-radius: var(--radius-panel); background: var(--surface); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  .line { display: flex; gap: var(--space-2); }
  .line input { flex: 1; min-width: 0; padding: var(--space-2) var(--space-3); border-radius: var(--radius-full); background: var(--fill-faint); color: var(--fg); outline: 0; }
  .line input::placeholder { color: var(--fg-muted); }
  .send { flex: none; padding: var(--space-2) var(--space-4); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  .send:disabled { opacity: 0.4; }
  .manage { display: flex; align-items: center; gap: var(--space-2); color: var(--fg-muted); font-size: var(--text-table); }
  .added { display: flex; align-items: center; gap: var(--space-1); color: var(--fg-soft); font-size: var(--text-table); }
  .added :global(.icon) { width: 15px; height: 15px; }
  section { display: flex; flex: none; flex-direction: column; gap: var(--space-2); }
  h3 { padding: 0 var(--space-1); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  ul { display: flex; flex-direction: column; gap: var(--space-1); margin: 0; padding: 0; list-style: none; }
  li { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) var(--space-3); border-radius: var(--radius-item); background: var(--surface); }
  .who { display: flex; flex: 1; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  strong { display: flex; align-items: center; gap: var(--space-2); overflow-wrap: anywhere; font-weight: var(--weight-medium); }
  em { padding: 0 var(--space-1-5); border-radius: var(--radius-full); background: var(--fill); color: var(--fg-muted); font-size: var(--text-meta); font-style: normal; }
  small { overflow: hidden; color: var(--fg-muted); font-size: var(--text-meta); text-overflow: ellipsis; }
  .role { flex: none; color: var(--fg-muted); font-size: var(--text-table); }
  .remove { flex: none; color: var(--danger); font-size: var(--text-table); }
  .alone { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); margin: var(--space-3) auto 0; color: var(--fg-muted); }
</style>
