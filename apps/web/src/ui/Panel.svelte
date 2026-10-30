<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { look, setLook, TINTS, TYPES } from './looks.svelte.js';

  let { name = '', seed = '', views = [], spaces = [], home = '', current = '', email = '', onpick, oncreate } = $props();
  let admin = $state(false);
  let adding = $state(false);
  let draft = $state('');
  let failed = $state(false);
  let picking = $state(false);
  let naming = $state(false);
  let who = $state('');
  let reach = $state('');

  const l = $derived(look(seed));
  const label = (s) => (s.id === home ? 'Personal' : s.name.replaceAll('_', ' '));
  const isLocked = (s) => l.locked.includes(s.name);

  function lock(s) {
    setLook(seed, { locked: isLocked(s) ? l.locked.filter((n) => n !== s.name) : [...l.locked, s.name] });
  }

  function choose(s) {
    if (l.shared && isLocked(s)) return;
    onpick(s.name);
  }

  function setType(type) {
    setLook(seed, { type, icon: 'robot' });
  }

  function saveCustodian(e) {
    e.preventDefault();
    if (who.trim()) setLook(seed, { custodian: { name: who.trim(), reach: reach.trim() } });
    naming = false;
  }

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
  <button class="avatar" type="button" aria-label="Change icon and colour" aria-expanded={picking} title="Change icon and colour" onclick={() => (picking = !picking)}>
    <Bot {seed} size={72} /><span class="edit"><Icon name="pencil" /></span>
  </button>
  <h2>{name}</h2>
  <p class="kind">{TYPES[l.type].label} archive{l.shared ? ' · Shared with you' : ''}</p>

  {#if picking}
    <div class="picker">
      <div class="types">
        {#each Object.entries(TYPES) as [key, t] (key)}
          <button type="button" class:on={l.type === key} onclick={() => setType(key)}>{t.label}</button>
        {/each}
      </div>
      <div class="icons">
        {#each TYPES[l.type].icons as icon (icon)}
          <button type="button" class:on={l.icon === icon} aria-label={icon} onclick={() => setLook(seed, { icon })}>
            {#if icon === 'robot'}<Icon name="bot" />{:else}<Icon name={icon} />{/if}
          </button>
        {/each}
      </div>
      <div class="tints">
        {#each TINTS as tint (tint)}
          <button type="button" class:on={l.tint === tint} style="background: {tint}" aria-label="Colour {tint}" onclick={() => setLook(seed, { tint })}></button>
        {/each}
      </div>
    </div>
  {/if}

  {#if spaces.length > 1 || admin}
    <div class="list">
      <h3>Spaces</h3>
      <button class="link" class:on={!current} type="button" onclick={() => onpick('')}><Icon name="layers" /><span>Everything</span></button>
      {#each spaces as s (s.id)}
        {@const shut = isLocked(s)}
        <div class="line">
          <button class="link" class:on={s.name === current} class:shut={shut && l.shared} type="button" disabled={shut && l.shared} onclick={() => choose(s)}>
            <Icon name={shut ? 'lock' : s.id === home ? 'user' : 'folder'} /><span>{label(s)}</span>
            {#if shut && l.shared}<small>Locked</small>{/if}
          </button>
          {#if !l.shared && s.id !== home}
            <button class="key" type="button" aria-label="{shut ? 'Unlock' : 'Lock'} {label(s)}" title={shut ? 'Unlock' : 'Lock'} onclick={() => lock(s)}><Icon name={shut ? 'lock-open' : 'lock'} /></button>
          {/if}
        </div>
      {/each}
      {#if adding}
        <form class="link" class:failed onsubmit={create}>
          <Icon name="plus" />
          <input bind:value={draft} placeholder="Space name" aria-label="Space name" oninput={() => (failed = false)} {@attach (el) => el.focus()} />
        </form>
      {:else if admin && !l.shared}
        <button class="link quiet" type="button" onclick={() => (adding = true)}><Icon name="plus" /><span>Add space</span></button>
      {/if}
      {#if l.shared && l.locked.length}<p class="note">Locked by the archive's owners. Ask them if you need access.</p>{/if}
    </div>
  {/if}

  <div class="list">
    {#if views.length}<h3>Lists</h3>{/if}
    {#each views as v (v.name)}
      <a class="link" href="/views/{v.name}"><Icon name={v.icon ?? 'list'} /><span>{v.name.replaceAll('_', ' ')}</span></a>
    {/each}
  </div>

  {#if !l.shared}
    <div class="list">
      <h3>Custodian</h3>
      {#if l.custodian && !naming}
        <button class="link" type="button" onclick={() => { who = l.custodian.name; reach = l.custodian.reach; naming = true; }}>
          <Icon name="shield-check" /><span>{l.custodian.name}</span>
        </button>
      {:else if naming}
        <form class="form" onsubmit={saveCustodian}>
          <input bind:value={who} placeholder="Their name" aria-label="Custodian name" {@attach (el) => el.focus()} />
          <input bind:value={reach} placeholder="Email or phone" aria-label="Custodian contact" />
          <button type="submit">Save</button>
        </form>
      {:else}
        <button class="link quiet" type="button" onclick={() => (naming = true)}><Icon name="shield-plus" /><span>Add a custodian</span></button>
      {/if}
      <p class="note">Someone you trust, who can step in if something happens to you.</p>
    </div>
  {/if}
</aside>

<style>
  .panel { display: flex; flex: none; flex-direction: column; align-items: center; gap: var(--space-3); width: 280px; padding: var(--space-6) var(--space-4) calc(var(--space-6) + 40px); overflow-y: auto; }
  .avatar { position: relative; flex: none; }
  .edit { position: absolute; right: -6px; bottom: -6px; display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--surface); color: var(--fg-muted); box-shadow: 0 1px 3px rgb(0 0 0 / 20%); }
  .edit :global(.icon) { width: 13px; height: 13px; }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .kind { margin-top: calc(-1 * var(--space-2)); color: var(--fg-muted); font-size: var(--text-meta); text-align: center; }
  .picker { display: flex; flex: none; flex-direction: column; gap: var(--space-2); width: 100%; padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface); }
  .types { display: flex; gap: var(--space-1); }
  .types button { flex: 1; padding: var(--space-1); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); }
  .types .on { background: var(--fill); color: var(--fg); }
  .icons { display: grid; grid-template-columns: repeat(6, 1fr); gap: var(--space-1); }
  .icons button { display: grid; place-items: center; aspect-ratio: 1; border-radius: var(--radius-nav); color: var(--fg-soft); }
  .icons .on { background: var(--fill-strong); color: var(--fg); }
  .icons :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .tints { display: flex; justify-content: space-between; }
  .tints button { width: 24px; height: 24px; border-radius: 50%; }
  .tints .on { box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--fg-muted); }
  .list { display: flex; flex: none; flex-direction: column; gap: var(--space-0-5); width: 100%; font-size: var(--text-table); }
  .line { position: relative; display: flex; }
  .link { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-1-5); border-radius: var(--radius-nav); text-align: left; text-transform: capitalize; transition: background var(--duration-fast); }
  a.link:hover, button.link:hover:enabled { background: var(--fill-faint); }
  .link.on { background: var(--fill); }
  .shut { color: var(--fg-muted); cursor: not-allowed; }
  small { margin-left: auto; font-size: var(--text-meta); }
  .key { position: absolute; top: 50%; right: var(--space-1); display: grid; place-items: center; width: 26px; height: 26px; transform: translateY(-50%); color: var(--fg-muted); opacity: 0; }
  .line:hover .key, .key:focus-visible { opacity: 1; }
  .key:hover { color: var(--fg); }
  @media (hover: none) { .key { opacity: 1; } }
  .key :global(.icon) { width: var(--size-icon); height: var(--size-icon); }
  .quiet { color: var(--fg-muted); }
  .failed { color: var(--danger); }
  input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  .form { display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-2); border-radius: var(--radius-nav); background: var(--surface); }
  .form button { align-self: flex-end; padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); background: var(--fill); }
  h3 { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  .note { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); line-height: var(--leading-snug); }
  @media (max-width: 1023px) { .panel { display: none; } }
</style>
