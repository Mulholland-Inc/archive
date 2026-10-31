<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import Composer from './Composer.svelte';

  // Front end only for now: nobody is on the other end yet.
  let open = $state(false);
  let draft = $state('');
  let root = $state();
  let turns = $state([{ mine: false, text: "Good day. I'm your archivist. Old photos, tangled family trees, what to keep and how to keep it: ask me anything." }]);

  function send(text) {
    turns.push({ mine: true, text: text.trim() });
    draft = '';
    setTimeout(() => turns.push({ mine: false, text: 'Noted. An archivist from Mulholland will answer you right here.' }), 700);
  }

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
  <button class="mark" type="button" aria-label="Talk to an archivist" aria-expanded={open} title="Talk to an archivist" onclick={() => (open = !open)}><span></span></button>
  {#if open}
    <div class="card" role="dialog" aria-label="Talk to an archivist">
      <header>
        <Bot seed="archivist" icon="archivist" tint="#e8c39e" size={64} bare />
        <div><h2>Your archivist</h2><p>Mulholland · Demo</p></div>
        <button class="close" type="button" aria-label="Close" onclick={() => (open = false)}><Icon name="x" /></button>
      </header>
      <div class="turns">
        {#each turns as t, i (i)}<p class:mine={t.mine}>{t.text}</p>{/each}
      </div>
      <div class="box"><Composer bind:value={draft} onsend={send} onstop={() => {}} placeholder="Ask your archivist" aria-label="Ask your archivist" /></div>
    </div>
  {/if}
</div>

<style>
  .archivist { position: fixed; top: var(--space-3); right: var(--space-3); z-index: var(--z-nav); }
  .mark { display: grid; place-items: center; width: 40px; height: 40px; border-radius: 50%; color: var(--fg-muted); transition: background var(--duration-fast), color var(--duration-fast); }
  .mark:hover, .mark[aria-expanded='true'] { background: var(--fill); color: var(--fg); }
  .mark span { width: 26px; height: 19px; background: currentColor; -webkit-mask: url(/mulholland-mark.svg) center / contain no-repeat; mask: url(/mulholland-mark.svg) center / contain no-repeat; }
  .card { position: absolute; top: calc(100% + var(--space-2)); right: 0; display: flex; flex-direction: column; width: min(340px, calc(100vw - 2 * var(--space-3))); max-height: min(520px, calc(100svh - 80px)); overflow: hidden; border-radius: var(--radius-panel); background: var(--surface-raised); box-shadow: 0 16px 48px rgb(0 0 0 / 28%); }
  header { position: relative; display: flex; align-items: center; gap: var(--space-5); padding: var(--space-5) var(--space-4) var(--space-5) var(--space-5); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  header p { color: var(--fg-muted); font-size: var(--text-meta); }
  .close { position: absolute; top: var(--space-2); right: var(--space-2); display: grid; place-items: center; width: 28px; height: 28px; color: var(--fg-muted); }
  .turns { display: flex; flex: 1; flex-direction: column; gap: var(--space-2); min-height: 120px; padding: 0 var(--space-4) var(--space-3); overflow-y: auto; }
  .turns p { align-self: flex-start; max-width: 90%; line-height: var(--leading-snug); }
  .turns .mine { align-self: flex-end; padding: var(--space-1) var(--space-2); border-radius: var(--radius-item); background: var(--fill); }
  .box { flex: none; padding: var(--space-2); }
  @media (max-width: 767px) { .archivist { right: calc(var(--space-3) + 48px); } }
</style>
