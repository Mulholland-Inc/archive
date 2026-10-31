<script>
  import { api } from '../api.js';
  import Icon from '#kit/Icon.svelte';
  import Bot from './Bot.svelte';
  import { clack, ding, whoosh } from './sounds.js';

  // A small clerk in a machine: a capsule comes down the tube and a teleprinter reports what really happened.
  // ids is empty while the files are on their way, and fills once they're safe.
  let { seed = '', files = [], ids = [], failed = false, onclose } = $props();

  const POLL = 1500;
  const PATIENCE = 40;
  const WIDTH = 34;
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const date = new Date().toLocaleDateString('en-US', { month: 'short', day: '2-digit', year: 'numeric' }).toUpperCase();
  const number = Date.now().toString(36).slice(-6).toUpperCase();
  const wait = (ms) => new Promise((done) => setTimeout(done, still ? 0 : ms));

  let paper = $state([]);
  let working = $state(true);
  let arrive;
  const arrived = new Promise((done) => (arrive = done));
  $effect(() => {
    if (ids.length || failed) arrive();
  });

  const size = (n) => (n < 1024 ? `${n} B` : n < 1048576 ? `${Math.round(n / 1024)} KB` : `${(n / 1048576).toFixed(1)} MB`);
  function entry(f) {
    const weight = size(f.size);
    const room = WIDTH - weight.length - 1;
    const name = f.name.toUpperCase();
    return `${(name.length > room ? `${name.slice(0, room - 1)}…` : name).padEnd(room)} ${weight}`;
  }

  async function type(text) {
    paper.push('');
    const at = paper.length - 1;
    for (let i = 0; i < text.length; i++) {
      paper[at] += text[i];
      if (still) continue;
      if (text[i] !== ' ' && i % 2 === 0) clack();
      await wait(19);
    }
    paper[at] = text;
    return at;
  }

  async function reading(names, line, tries = 0) {
    let left;
    try {
      const now = await Promise.all(names.map((id) => api(`/api/files/${id}`)));
      if (now.some((f) => f.status === 'error')) return 'unread';
      left = now.filter((f) => f.status === 'pending').map((f) => f.id);
    } catch {
      return 'waiting';
    }
    if (!left.length) return 'ready';
    if (tries >= PATIENCE) return 'waiting';
    if (paper[line].length < WIDTH) paper[line] += '.';
    clack();
    await wait(POLL);
    return reading(left, line, tries + 1);
  }

  async function run() {
    whoosh();
    await wait(760);
    for (const f of files.slice(0, 3)) await type(entry(f));
    if (files.length > 3) await type(`AND ${files.length - 3} MORE`);
    await arrived;
    if (failed) {
      await type('RETURNED. PLEASE TRY AGAIN.');
    } else {
      const line = await type('CLERK READING');
      const result = await reading(ids, line);
      await type({ ready: 'FILED. ALL IN ORDER.', unread: 'FILED. PART OF IT WOULD NOT READ.', waiting: 'STILL WITH THE CLERK. HE WILL FINISH.' }[result]);
      if (result === 'ready') ding();
    }
    working = false;
    await wait(4200);
    onclose();
  }

  run();
</script>

<div class="machine" class:working role="status" aria-label={paper.at(-1)}>
  <div class="bay">
    <svg viewBox="0 0 96 96" aria-hidden="true"><path class="pipe" d="M96 11 H66 Q48 11 48 29 V46" /><path class="bore" d="M96 11 H66 Q48 11 48 29 V46" /></svg>
    <span class="capsule"></span>
    <span class="clerk"><Bot {seed} size={46} /></span>
    <span class="gauge"><i></i></span>
  </div>
  <div class="slot">
    <div class="paper">
      <header><strong>CENTRAL ARCHIVE SERVICES</strong><span>FORM 27B/6</span></header>
      <p class="ref"><span>No. {number}</span><span>{date}</span></p>
      {#each paper as line, i (i)}<p>{line}{#if working && i === paper.length - 1}<b>▌</b>{/if}</p>{/each}
    </div>
  </div>
  <button type="button" aria-label="Close" onclick={onclose}><Icon name="x" /></button>
</div>

<style>
  .machine { --brass: #b8963e; --steel: #3a404a; --ink: #23262b; --sheet: #f2ebd9; position: fixed; bottom: calc(var(--space-4) + env(safe-area-inset-bottom)); left: 50%; z-index: calc(var(--z-nav) + 5); display: flex; align-items: stretch; width: min(452px, calc(100vw - 2 * var(--space-3))); transform: translateX(-50%); border-radius: 14px; background: linear-gradient(160deg, #2e333b, #1c1f25); box-shadow: inset 0 0 0 1px rgb(255 255 255 / 7%), 0 16px 40px rgb(0 0 0 / 34%); animation: rise 0.3s cubic-bezier(0.2, 0.9, 0.3, 1.1); }
  .bay { position: relative; flex: none; align-self: flex-start; width: 96px; height: 96px; }
  .clerk { z-index: 1; }
  svg { position: absolute; inset: 0; width: 96px; height: 96px; }
  path { fill: none; }
  .pipe { stroke: var(--brass); stroke-width: 13; }
  .bore { stroke: #15171b; stroke-width: 9; }
  .capsule { position: absolute; top: 0; left: 0; width: 13px; height: 7px; border-radius: 3px; background: linear-gradient(#e8cf8a, var(--brass)); offset-path: path('M96 11 H66 Q48 11 48 29 V46'); offset-distance: 100%; opacity: 0; animation: shoot 0.62s cubic-bezier(0.5, 0, 0.9, 0.7) 0.08s; }
  .clerk { position: absolute; top: 40px; left: 25px; display: grid; }
  .working .clerk { animation: work 0.17s steps(2) infinite; }
  .gauge { position: absolute; top: 71px; left: 7px; width: 17px; height: 17px; border: 2px solid var(--brass); border-radius: 50%; background: var(--sheet); }
  .gauge i { position: absolute; bottom: 50%; left: calc(50% - 0.5px); width: 1px; height: 6px; transform-origin: bottom; background: #8a2433; transform: rotate(-60deg); transition: transform 0.5s; }
  .working .gauge i { animation: needle 0.9s ease-in-out infinite; }
  .slot { display: flex; flex: 1; align-items: flex-end; min-width: 0; padding: var(--space-2) 0 0; overflow: hidden; }
  .paper { width: 100%; min-height: 86px; padding: 8px 12px 10px; border-radius: 3px 3px 0 0; background: repeating-linear-gradient(transparent 0 15px, rgb(120 90 40 / 7%) 15px 16px), var(--sheet); box-shadow: 0 -2px 8px rgb(0 0 0 / 28%); color: var(--ink); font: 500 11px / 16px 'Courier New', ui-monospace, monospace; }
  header, .ref { display: flex; justify-content: space-between; gap: var(--space-2); }
  header strong { font-weight: 700; letter-spacing: 0.04em; }
  .ref { margin-bottom: 3px; padding-bottom: 2px; border-bottom: 1px dashed rgb(35 38 43 / 40%); color: #8a2433; }
  p { overflow: hidden; white-space: pre; }
  b { animation: blink 0.5s steps(2) infinite; }
  button { position: absolute; top: 5px; left: 5px; display: grid; place-items: center; width: 22px; height: 22px; color: #8d94a0; }
  button :global(.icon) { width: 13px; height: 13px; }
  @keyframes rise { from { opacity: 0; transform: translate(-50%, 16px); } }
  @keyframes shoot { 0% { offset-distance: 0%; opacity: 1; } 92% { opacity: 1; } 100% { offset-distance: 100%; opacity: 0; } }
  @keyframes work { to { transform: translate(1px, -1px) rotate(1.4deg); } }
  @keyframes needle { 0%, 100% { transform: rotate(10deg); } 40% { transform: rotate(68deg); } 70% { transform: rotate(32deg); } }
  @keyframes blink { 50% { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) {
    .machine, .capsule, .working .clerk, .working .gauge i, b { animation: none; }
  }
</style>
