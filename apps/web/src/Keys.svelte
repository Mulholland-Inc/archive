<script>
  import { API, api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Chip from './ui/Chip.svelte';
  import Copy from './ui/Copy.svelte';
  import Empty from './ui/Empty.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Sheet from './ui/Sheet.svelte';

  let keys = $state(null);
  let name = $state('');
  let viewer = $state(false);
  let fresh = $state('');
  let error = $state('');
  let busy = $state(false);

  const mcp = `${API || (import.meta.env.DEV ? 'http://localhost:8080' : location.origin)}/mcp`;
  const day = (v) => new Date(v).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });

  async function load() {
    try {
      keys = await api('/auth/tokens');
    } catch (e) {
      error = e.message;
      keys = [];
    }
  }

  async function create(e) {
    e.preventDefault();
    if (busy || !name.trim()) return;
    busy = true;
    try {
      const t = await api('/auth/tokens', { method: 'POST', body: { name: name.trim(), viewer } });
      fresh = t.token;
      name = '';
      viewer = false;
      error = '';
      await load();
    } catch (err) {
      error = err.message;
    }
    busy = false;
  }

  async function revoke(id) {
    try {
      await api(`/auth/tokens/${id}`, { method: 'DELETE' });
      await load();
    } catch (e) {
      error = e.message;
    }
  }

  load();
</script>

<Heading plain title="API keys" />
<Sheet>
  <div class="keys">
    {#if error}<Note error>{error}</Note>{/if}
    <section>
      <h3>Connection address</h3>
      <div class="line"><code>{mcp}</code><Copy text={mcp} /></div>
      <p>Add this to your AI app as an MCP server, with a key below as its bearer token.</p>
    </section>

    {#if fresh}
      <section class="fresh">
        <h3>Your new key</h3>
        <div class="line"><code>{fresh}</code><Copy text={fresh} /></div>
        <p>Copy it now. It won't be shown again.</p>
      </section>
    {/if}

    <section>
      <h3>Keys</h3>
      <form onsubmit={create}>
        <input bind:value={name} placeholder="What's this key for?" aria-label="Key name" />
        <button class="viewer" type="button" aria-pressed={viewer} onclick={() => (viewer = !viewer)}>viewer</button>
        <button type="submit" disabled={busy || !name.trim()}><Icon name="plus" />Create key</button>
      </form>
      {#each keys ?? [] as k (k.id)}
        <div class="key">
          <span class="name">{k.name || 'Untitled'}</span>
          {#if k.viewer}<Chip>viewer</Chip>{/if}
          <span class="when">{day(k.created_at)}</span>
          <button class="revoke" type="button" aria-label="Revoke {k.name}" onclick={() => revoke(k.id)}>Revoke</button>
        </div>
      {:else}
        {#if keys}<Empty title="No keys yet">Create one to connect ChatGPT, Claude or your own code.</Empty>{/if}
      {/each}
    </section>
  </div>
</Sheet>

<style>
  .keys { display: flex; flex-direction: column; gap: var(--space-6); }
  section { display: flex; flex-direction: column; gap: var(--space-2); }
  h3 { color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  p { color: var(--fg-muted); font-size: var(--text-table); }
  .line { display: flex; align-items: center; gap: var(--space-2); min-width: 0; padding: var(--space-2) var(--space-3); border-radius: var(--radius-nav); background: var(--fill-faint); }
  code { flex: 1; min-width: 0; overflow-wrap: anywhere; font: var(--text-table) var(--font-mono); }
  .fresh { padding: var(--space-3); border-radius: var(--radius-item); background: var(--fill-faint); }
  .fresh .line { background: var(--surface); }
  form { display: flex; gap: var(--space-2); }
  input { flex: 1; min-width: 0; padding: var(--space-2) var(--space-3); border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg); outline: 0; }
  form .viewer { padding: var(--space-2); border-radius: var(--radius-nav); background: var(--fill); color: var(--fg-muted); }
  form .viewer[aria-pressed='true'] { background: var(--fill-strong); color: var(--fg); }
  form button[type='submit'] { display: inline-flex; flex: none; align-items: center; gap: var(--space-1); padding: var(--space-2) var(--space-3); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  form button[type='submit']:disabled { opacity: 0.4; }
  form :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .key { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) 0; }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .when { color: var(--fg-muted); font-size: var(--text-table); }
  .revoke { color: var(--danger); font-size: var(--text-table); }
</style>
