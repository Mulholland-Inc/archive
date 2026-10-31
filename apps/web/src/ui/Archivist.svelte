<script>
  import Icon from '#kit/Icon.svelte';
  import { workspace } from '../api.js';
  import Bot from './Bot.svelte';
  import Composer from './Composer.svelte';

  let { name = '', email = '', archive = '', kind = 'personal' } = $props();

  // apps/help carries this to a Discord forum. Without its address the chat is a demo, and nobody answers.
  const HELP = import.meta.env.VITE_HELP_URL ?? '';
  const ARCHIVIST = import.meta.env.VITE_ARCHIVIST ?? 'Sam';
  const BOOK = import.meta.env.VITE_BOOK_URL ?? '';
  const ASK = {
    family: 'Old photos, tangled family trees, what to keep: ask away.',
    business: 'Your records, your contracts, what to keep and how: ask away.',
    personal: 'Your papers, your photos, what to keep: ask away.',
  };
  const NOTED = { mine: false, who: ARCHIVIST, text: "Thanks. I'll answer you right here." };
  const key = `help:${workspace()}`;

  function read() {
    try {
      return JSON.parse(localStorage[key] ?? '{}');
    } catch {
      return {};
    }
  }

  const saved = read();
  let open = $state(false);
  let draft = $state('');
  let root = $state();
  let unread = $state(false);
  let ticket = $state(saved.ticket ?? '');
  let after = saved.after ?? '';
  let turns = $state(saved.turns ?? []);

  function save() {
    try {
      localStorage[key] = JSON.stringify({ ticket, after, turns });
    } catch {}
  }

  async function send(text) {
    const said = text.trim();
    turns.push({ mine: true, text: said });
    draft = '';
    if (!HELP) return void setTimeout(() => turns.push(NOTED), 700);
    try {
      const r = await fetch(`${HELP}/messages`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: said, ticket, name, email, archive }) });
      if (!r.ok) throw new Error();
      const first = !ticket;
      ticket = (await r.json()).ticket;
      if (first) turns.push(NOTED);
    } catch {
      turns.push({ mine: false, text: "That didn't send. Please try again." });
    }
    save();
  }

  async function check() {
    try {
      const r = await fetch(`${HELP}/messages?ticket=${encodeURIComponent(ticket)}${after ? `&after=${after}` : ''}`);
      if (!r.ok) return;
      const fresh = await r.json();
      if (!fresh.length) return;
      turns.push(...fresh.map((m) => ({ mine: false, who: m.name, text: m.text })));
      after = fresh.at(-1).id;
      if (!open) unread = true;
      save();
    } catch {}
  }

  function toggle() {
    open = !open;
    unread = false;
  }

  $effect(() => {
    if (!HELP || !ticket) return;
    const timer = setInterval(check, open ? 5000 : 30000);
    return () => clearInterval(timer);
  });

  $effect(() => {
    if (!open) return;
    const away = (e) => !root.contains(e.target) && (open = false);
    const esc = (e) => e.key === 'Escape' && (open = false);
    addEventListener('pointerdown', away);
    addEventListener('keydown', esc);
    return () => {
      removeEventListener('pointerdown', away);
      removeEventListener('keydown', esc);
    };
  });
</script>

<div class="archivist" bind:this={root}>
  <button class="mark" class:unread type="button" aria-label="Help: ask a Mulholland archivist" aria-expanded={open} title="Ask a Mulholland archivist" onclick={toggle}><span></span>Help</button>
  {#if open}
    <div class="card" role="dialog" aria-label="Ask a Mulholland archivist">
      <header>
        <Bot seed="archivist" icon="archivist" tint="#e8c39e" size={64} bare />
        <div><h2>Ask a Mulholland archivist</h2><p>{HELP ? 'Talk to a real person, 24/7' : 'Demo'}</p></div>
        <button class="close" type="button" aria-label="Close" onclick={() => (open = false)}><Icon name="x" /></button>
      </header>
      <div class="turns">
        <p><small>{ARCHIVIST}</small>Hi, I'm {ARCHIVIST}, your Mulholland archivist. {ASK[kind] ?? ASK.personal}</p>
        {#each turns as t, i (i)}
          <p class:mine={t.mine}>{#if t.who}<small>{t.who}</small>{/if}{t.text}</p>
        {/each}
      </div>
      {#if BOOK}<a class="book" href={BOOK} target="_blank" rel="noreferrer"><Icon name="video" />Book a call with {ARCHIVIST}</a>{/if}
      <div class="box"><Composer bind:value={draft} onsend={send} onstop={() => {}} placeholder="Ask anything" aria-label="Ask a Mulholland archivist" /></div>
    </div>
  {/if}
</div>

<style>
  .archivist { position: fixed; top: var(--space-3); right: var(--space-3); z-index: var(--z-nav); }
  .mark { position: relative; display: flex; align-items: center; gap: var(--space-1); padding: var(--space-1) var(--space-2); border-radius: var(--radius-full); color: var(--fg-muted); font-size: var(--text-meta); transition: background var(--duration-fast), color var(--duration-fast); }
  .unread::after { content: ''; position: absolute; top: 4px; right: 6px; width: 8px; height: 8px; border-radius: 50%; background: #ff3b30; }
  .mark:hover, .mark[aria-expanded='true'] { background: var(--fill); color: var(--fg); }
  .mark span { width: 22px; height: 16px; background: currentColor; -webkit-mask: url(/mulholland-mark.svg) center / contain no-repeat; mask: url(/mulholland-mark.svg) center / contain no-repeat; }
  .card { position: absolute; top: calc(100% + var(--space-2)); right: 0; display: flex; flex-direction: column; width: min(340px, calc(100vw - 2 * var(--space-3))); max-height: min(520px, calc(100svh - 80px)); overflow: hidden; border-radius: var(--radius-panel); background: var(--surface-raised); box-shadow: 0 16px 48px rgb(0 0 0 / 28%); }
  header { position: relative; display: flex; align-items: center; gap: var(--space-5); padding: var(--space-5) var(--space-4) var(--space-5) var(--space-5); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  header p { color: var(--fg-muted); font-size: var(--text-meta); }
  .close { position: absolute; top: var(--space-2); right: var(--space-2); display: grid; place-items: center; width: 28px; height: 28px; color: var(--fg-muted); }
  .turns { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-height: 120px; padding: 0 var(--space-4) var(--space-3); overflow-y: auto; }
  .turns p { align-self: flex-start; max-width: 90%; line-height: var(--leading-snug); }
  .turns small { display: block; color: var(--fg-muted); font-size: var(--text-meta); }
  .turns .mine { align-self: flex-end; padding: var(--space-1) var(--space-2); border-radius: var(--radius-item); background: var(--fill); }
  .book { display: flex; flex: none; align-items: center; gap: var(--space-1); margin: 0 var(--space-4); padding: var(--space-1) 0; color: var(--fg-muted); font-size: var(--text-meta); }
  .book:hover { color: var(--fg); }
  .book :global(.icon) { width: 15px; height: 15px; }
  .box { flex: none; padding: var(--space-2); }
  @media (max-width: 767px) { .archivist { right: calc(var(--space-3) + 48px); } }
</style>
