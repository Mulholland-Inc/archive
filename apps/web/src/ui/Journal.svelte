<script>
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { allowed, day, here, time } from './moment.js';

  let { seed = '', type = 'family', onsave, onclose } = $props();
  const business = type === 'business';
  const MODES = business
    ? [
        { key: 'memo', label: 'Memo', file: 'Memo', prompt: 'What does everyone need to know?', asks: [['to', 'Who is this for?', 'A person, a team, everyone'], ['subject', 'What is it about?', 'A short subject']] },
        { key: 'idea', label: 'Idea', file: 'Idea', prompt: 'What is the idea?', asks: [['why', 'Why does it matter?', 'The problem it solves'], ['who', 'Who should look at this?', 'Names or a team']] },
        { key: 'report', label: 'Report', file: 'Report', prompt: 'What happened, and what did you find?', asks: [['period', 'What period does it cover?', 'Last week, Q3, this project'], ['owner', 'Who is it from?', 'A person or a team']] },
      ]
    : [
        { key: 'now', label: 'Today', file: 'Journal', prompt: 'What happened today?', asks: [] },
        { key: 'old', label: 'An old story', file: 'Story', prompt: 'Tell the story, however you remember it', asks: [['when', 'When did this happen?', 'Summer 1978, or just roughly'], ['where', 'Where was it?', 'A town, a house, a street'], ['who', 'Who else was there?', 'Names, or who they were to you']] },
      ];
  let kind = $state(MODES[0].key);
  const mode = $derived(MODES.find((m) => m.key === kind));
  const answers = $state({});
  let text = $state('');
  let moment = $state(null);
  let finding = $state(false);
  let missing = $state('');

  const now = new Date();
  const today = day(now);
  const clock = time(now);

  async function locate() {
    finding = true;
    missing = '';
    try {
      moment = await here();
    } catch (e) {
      missing = e.message;
    }
    finding = false;
  }

  if (!business) allowed().then((yes) => yes && locate());

  function save() {
    const facts = [
      ...mode.asks.map(([key, q]) => (answers[key]?.trim() ? `${q} ${answers[key].trim()}` : '')),
      `Written: ${today}, ${clock}`,
      kind === 'now' && moment?.place ? `Place: ${moment.place}` : '',
      kind === 'now' && moment?.weather ? `Weather: ${moment.weather}` : '',
    ];
    const body = `${facts.filter(Boolean).join('\n')}\n\n${text.trim()}\n`;
    const name = `${mode.file} ${now.toLocaleDateString(undefined, { dateStyle: 'medium' }).replaceAll('/', '-')} ${clock.replaceAll(':', '.')}.md`;
    onsave(new File([body], name, { type: 'text/markdown' }));
    onclose();
  }

  const esc = (e) => e.key === 'Escape' && onclose();
</script>

<svelte:window onkeydown={esc} />

<button class="scrim" type="button" aria-label="Close" onclick={onclose}></button>
<div class="sheet" role="dialog" aria-modal="true" aria-label="Write it down">
  <header>
    <div class="kinds" role="tablist">
      {#each MODES as m (m.key)}
        <button type="button" role="tab" aria-selected={kind === m.key} onclick={() => (kind = m.key)}>{m.label}</button>
      {/each}
    </div>
    <button class="close" type="button" aria-label="Close" onclick={onclose}><Icon name="x" /></button>
  </header>

  {#if kind === 'now'}
    <div class="meta">
      <span class="today">{today}</span>
      <span class="chips">
        <span class="chip"><Icon name="clock" />{clock}</span>
        {#if moment}
          {#if moment.place}<span class="chip"><Icon name="map-pin" />{moment.place}</span>{/if}
          {#if moment.weather}<span class="chip"><Icon name={moment.icon} />{moment.weather}</span>{/if}
        {:else}
          <button class="chip" type="button" disabled={finding} onclick={locate}><Icon name="map-pin" />{finding ? 'Finding you' : missing || 'Add place and weather'}</button>
        {/if}
      </span>
    </div>
  {/if}

  <textarea bind:value={text} placeholder={mode.prompt} aria-label="Your entry" {@attach (el) => el.focus()}></textarea>

  {#if mode.asks.length}
    <div class="asks">
      <div class="asker"><Bot {seed} size={32} /><span>A few things will help me file this.</span></div>
      {#each mode.asks as [key, q, hint] (key)}
        <label><span>{q}</span><input bind:value={answers[key]} placeholder={hint} /></label>
      {/each}
    </div>
  {/if}

  <footer><button class="save" type="button" disabled={!text.trim()} onclick={save}>Save to archive</button></footer>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 1); width: 100%; background: rgb(0 0 0 / 40%); cursor: default; }
  .sheet { position: fixed; top: 50%; left: 50%; z-index: calc(var(--z-nav) + 2); display: flex; flex-direction: column; gap: var(--space-3); width: min(640px, calc(100vw - 2 * var(--space-3))); max-height: calc(100svh - 2 * var(--space-3)); padding: var(--space-4); overflow-y: auto; transform: translate(-50%, -50%); border-radius: var(--radius-panel); background: var(--surface); box-shadow: 0 12px 40px rgb(0 0 0 / 25%); }
  header { display: flex; align-items: center; justify-content: space-between; }
  .kinds { display: flex; gap: var(--space-1); }
  .kinds button { padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); color: var(--fg-muted); }
  .kinds [aria-selected='true'] { background: var(--fill); color: var(--fg); }
  .close { display: grid; place-items: center; width: 32px; height: 32px; color: var(--fg-muted); }
  .meta { display: flex; flex-direction: column; gap: var(--space-2); }
  .today { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  .chips { display: flex; flex-wrap: wrap; gap: var(--space-1); }
  .chip { display: inline-flex; align-items: center; gap: var(--space-1); padding: var(--space-1) var(--space-2); border-radius: var(--radius-full); background: var(--fill-faint); color: var(--fg-soft); font-size: var(--text-meta); }
  .chip :global(.icon) { width: 14px; height: 14px; }
  textarea { width: 100%; min-height: 200px; padding: 0; border: 0; background: none; color: var(--fg); font: var(--text-lg) / var(--leading-medium, 1.5) var(--font-serif); resize: none; outline: 0; }
  textarea::placeholder { color: var(--fg-muted); }
  .asks { display: flex; flex-direction: column; gap: var(--space-2); padding-top: var(--space-3); border-top: var(--hairline) solid var(--fill); }
  .asker { display: flex; align-items: center; gap: var(--space-2); color: var(--fg-soft); }
  label { display: flex; flex-direction: column; gap: var(--space-0-5); color: var(--fg-muted); font-size: var(--text-meta); }
  label input { padding: var(--space-1-5) var(--space-2); border-radius: var(--radius-nav); background: var(--fill-faint); color: var(--fg); font-size: var(--text-table); outline: 0; }
  footer { display: flex; justify-content: flex-end; }
  .save { padding: var(--space-1-5) var(--space-4); border-radius: var(--radius-full); background: var(--primary); color: var(--on-primary); }
  .save:disabled { opacity: 0.4; }
</style>
