<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import Members from './Members.svelte';
  import { incoming, look, setLook, shortName, TINTS, TYPES } from './looks.svelte.js';

  let { name = '', seed = '', views = [], archives = [], spaces = [], home = '', current = '', email = '', onpick, oncreate } = $props();
  let admin = $state(false);
  let adding = $state(false);
  let draft = $state('');
  let failed = $state(false);
  let picking = $state(false);
  let naming = $state(false);
  let asked = $state('');
  // An explanation leaves as soon as you do anything else.
  $effect(() => {
    if (!asked) return;
    const away = (e) => !e.target.closest('.asks') && (asked = '');
    const esc = (e) => e.key === 'Escape' && (asked = '');
    addEventListener('pointerdown', away);
    addEventListener('keydown', esc);
    return () => {
      removeEventListener('pointerdown', away);
      removeEventListener('keydown', esc);
    };
  });

  let who = $state('');
  let reach = $state('');
  let people = $state(null);
  let counts = $state({});
  let menu = $state(null);

  const l = $derived(look(seed));
  const label = (s) => (s.id === home ? 'Personal' : s.name.replaceAll('_', ' '));
  // The engine hides a space from anyone outside it; a locked one just has a name for who's inside.
  const shown = $derived(spaces.filter((s) => !(l.shared && l.locked.includes(s.name))));
  const groupOf = (s) => l.groups[s.name] ?? '';

  for (const s of spaces) {
    if (s.id !== home) api(`/auth/spaces/${encodeURIComponent(s.name)}/members`).then((m) => (counts[s.name] = m?.length ?? 0)).catch(() => {});
  }

  function openMenu(e, s) {
    e.preventDefault();
    menu = { space: s, x: Math.min(e.clientX, innerWidth - 200), y: Math.min(e.clientY, innerHeight - 100) };
  }

  $effect(() => {
    if (!menu) return;
    const close = () => (menu = null);
    const esc = (e) => e.key === 'Escape' && close();
    addEventListener('pointerdown', close);
    addEventListener('keydown', esc);
    return () => {
      removeEventListener('pointerdown', close);
      removeEventListener('keydown', esc);
    };
  });

  const nameOf = (id) => {
    const w = archives.find((a) => a.id === id);
    return w ? shortName(w.name, look(id).type) : 'another archive';
  };
  const others = $derived(archives.filter((a) => a.id !== seed));
  const sharedTo = (s) => l.shares[s.name] ?? [];
  const from = $derived(incoming(seed));

  function share(s, id) {
    const to = sharedTo(s);
    setLook(seed, { shares: { ...l.shares, [s.name]: to.includes(id) ? to.filter((x) => x !== id) : [...to, id] } });
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

{#snippet help(title, text)}
  <h3 class="asks">
    {title}
    <button type="button" aria-label="What does {title} mean?" title="What does {title} mean?" aria-expanded={asked === title} onclick={() => (asked = asked === title ? '' : title)}><Icon name="circle-question-mark" /></button>
  </h3>
  {#if asked === title}<p class="note">{text}</p>{/if}
{/snippet}

<aside class="panel" aria-label={name}>
  <button class="avatar" type="button" aria-label="Change type, icon and colour" aria-expanded={picking} title="Change type, icon and colour" onclick={() => (picking = !picking)}>
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
            <Bot {seed} {icon} size={36} bare />
          </button>
        {/each}
      </div>
      <div class="tints">
        {#each TINTS as tint (tint)}
          <button type="button" class:on={l.tint === tint} style="background: {tint}" aria-label="Colour {tint}" onclick={() => setLook(seed, { tint })}></button>
        {/each}
        <label class="any" title="Any colour"><input type="color" value={l.tint} aria-label="Any colour" oninput={(e) => setLook(seed, { tint: e.currentTarget.value })} /></label>
      </div>
    </div>
  {/if}

  {#if spaces.length > 1 || admin}
    <div class="list">
      {@render help('Spaces', 'Rooms inside this archive. Share a room with someone without sharing the rest.')}
      <button class="link" class:on={!current} type="button" onclick={() => onpick('')}><Icon name="layers" /><span>Everything</span></button>
      {#each shown as s (s.id)}
        <div class="line" role="group" oncontextmenu={(e) => s.id !== home && openMenu(e, s)}>
          <button class="link" class:on={s.name === current} type="button" onclick={() => onpick(s.name)}>
            <Icon name={groupOf(s) ? 'lock' : s.id === home ? 'user' : 'folder'} />
            <span class="named">{label(s)}{#if groupOf(s)}<small>{groupOf(s)}</small>{/if}</span>
          </button>
          {#if s.id !== home}
            <button class="count" type="button" aria-label="People in {label(s)}" title="People" onclick={() => (people = s)}><Icon name="users-round" />{counts[s.name] ?? ''}</button>
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
    </div>
  {/if}

  {#if from.length}
    <div class="list">
      <h3>Shared with this archive</h3>
      {#each from as f (f.from + f.space)}
        <div class="link in" title="Stays in {nameOf(f.from)}, which can stop sharing at any time">
          <span class="linked"><Icon name="users-round" /></span>
          <span>{f.space.replaceAll('_', ' ')}<small>From {nameOf(f.from)}</small></span>
        </div>
      {/each}
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
      {@render help('Custodian', 'Someone you trust, who can step in if something happens to you.')}
      {#if l.custodian && !naming}
        <button class="link" type="button" onclick={() => { who = l.custodian.name; reach = l.custodian.reach; naming = true; }}>
          <Icon name="shield-check" /><span>{l.custodian.name}</span>
        </button>
      {:else if naming}
        <form class="form" onsubmit={saveCustodian} {@attach (el) => el.scrollIntoView({ block: 'center' })}>
          <input bind:value={who} placeholder="Their name" aria-label="Custodian name" {@attach (el) => el.focus()} />
          <input bind:value={reach} placeholder="Email or phone" aria-label="Custodian contact" />
          <div class="actions"><button type="button" onclick={() => (naming = false)}>Cancel</button><button class="save" type="submit">Save</button></div>
        </form>
      {:else}
        <button class="link quiet" type="button" onclick={() => (naming = true)}><Icon name="shield-plus" /><span>Add a custodian</span></button>
      {/if}
    </div>
  {/if}
</aside>

{#if menu}
  <div class="menu" role="menu" style="left: {menu.x}px; top: {menu.y}px" onpointerdown={(e) => e.stopPropagation()}>
    <button type="button" role="menuitem" onclick={() => { onpick(menu.space.name); menu = null; }}><Icon name="folder-open" />Open</button>
    <button type="button" role="menuitem" onclick={() => { people = menu.space; menu = null; }}><Icon name="users-round" />People and sharing</button>
  </div>
{/if}

{#if people}
  <Members space={people} title={label(people)} group={groupOf(people)} self={email} {others} sharedTo={sharedTo(people)} {nameOf} onshare={(id) => share(people, id)} ongroup={(g) => setLook(seed, { groups: { ...l.groups, [people.name]: g } })} onchange={(n) => (counts[people.name] = n)} onclose={() => (people = null)} />
{/if}

<style>
  .panel { display: flex; flex: none; flex-direction: column; align-items: center; gap: var(--space-3); width: 280px; max-height: 100svh; padding: var(--space-6) var(--space-4) calc(var(--space-6) + 40px); overflow-y: auto; }
  .avatar { position: relative; flex: none; }
  .edit { position: absolute; right: -6px; bottom: -6px; display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--surface); color: var(--fg-muted); box-shadow: 0 1px 3px rgb(0 0 0 / 20%); }
  .edit :global(.icon) { width: 13px; height: 13px; }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); text-align: center; }
  .kind { margin-top: calc(-1 * var(--space-2)); color: var(--fg-muted); font-size: var(--text-meta); text-align: center; }
  .picker { display: flex; flex: none; flex-direction: column; gap: var(--space-2); width: 100%; padding: var(--space-3); border-radius: var(--radius-item); background: var(--surface); }
  .types { display: flex; gap: var(--space-1); }
  .types button { flex: 1; padding: var(--space-1); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); }
  .types .on { background: var(--fill); color: var(--fg); }
  .icons { display: flex; justify-content: center; gap: var(--space-1); }
  .icons button { display: grid; place-items: center; width: 52px; height: 52px; border-radius: var(--radius-nav); color: var(--fg-soft); }
  .icons .on { background: var(--fill-strong); color: var(--fg); }
  .tints { display: flex; justify-content: space-between; gap: 2px; }
  .tints button { width: 22px; height: 22px; border-radius: 50%; }
  .any { position: relative; width: 22px; height: 22px; overflow: hidden; border-radius: 50%; background: conic-gradient(#e5484d, #f5a524, #30a46c, #0091ff, #8e4ec6, #e5484d); }
  .any input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
  .tints .on { box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--fg-muted); }
  .list { display: flex; flex: none; flex-direction: column; gap: var(--space-0-5); width: 100%; font-size: var(--text-table); }
  .line { position: relative; display: flex; }
  .line .link { padding-right: 52px; }
  .named { display: flex; flex-direction: column; min-width: 0; line-height: var(--leading-snug); }
  .named small { color: var(--fg-muted); font-size: var(--text-meta); text-transform: none; }
  .count { position: absolute; top: 50%; right: var(--space-1); display: flex; align-items: center; gap: 3px; padding: 2px var(--space-1); transform: translateY(-50%); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); }
  .count:hover { background: var(--fill); color: var(--fg); }
  .count :global(.icon) { width: 13px; height: 13px; }
  .menu { position: fixed; z-index: var(--z-nav); display: flex; flex-direction: column; min-width: 180px; padding: var(--space-1); border-radius: var(--radius-item); background: var(--surface-raised); box-shadow: 0 8px 28px rgb(0 0 0 / 22%); }
  .menu button { display: flex; align-items: center; gap: var(--space-1-5); padding: var(--space-1) var(--space-1-5); border-radius: var(--radius-sm); text-align: left; }
  .menu button:hover { background: var(--fill); }
  .menu :global(.icon) { width: 15px; height: 15px; }
  .link { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-1-5); border-radius: var(--radius-nav); text-align: left; text-transform: capitalize; transition: background var(--duration-fast); }
  a.link:hover, button.link:hover:enabled { background: var(--fill-faint); }
  .link.on { background: var(--fill); }
  small { margin-left: auto; font-size: var(--text-meta); }
  .linked { display: grid; place-items: center; flex: none; width: 20px; height: 20px; border-radius: 50%; background: var(--primary); color: #fff; }
  .linked :global(.icon) { width: 12px; height: 12px; }
  .in { align-items: flex-start; }
  .in .linked { margin-left: 0; }
  .in small { display: block; margin: 0; color: var(--fg-muted); text-transform: none; }
  .quiet { color: var(--fg-muted); }
  .failed { color: var(--danger); }
  input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  .form { display: flex; flex-direction: column; gap: var(--space-1); }
  .form input { flex: none; width: 100%; padding: var(--space-1-5) var(--space-2); border-radius: var(--radius-nav); background: var(--surface); }
  .actions { display: flex; justify-content: flex-end; gap: var(--space-1); margin-top: var(--space-0-5); }
  .actions button { padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); color: var(--fg-muted); }
  .actions .save { background: var(--primary); color: var(--on-primary); }
  h3 { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
  .asks { display: flex; align-items: center; gap: var(--space-1); }
  .asks button { display: grid; place-items: center; width: 16px; height: 16px; color: var(--fg-muted); opacity: 0.7; transition: opacity var(--duration-fast); }
  .asks button:hover, .asks [aria-expanded='true'] { opacity: 1; }
  .asks :global(.icon) { width: 14px; height: 14px; }
  .note { padding: 0 var(--space-1-5); color: var(--fg-muted); font-size: var(--text-meta); line-height: var(--leading-snug); }
  @media (max-width: 1023px) { .panel { display: none; } }
</style>
