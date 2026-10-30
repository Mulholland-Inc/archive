<script module>
  import { fetchRows } from './filter.js';

  const NEEDS = ['name', 'sex', 'born_year', 'died_year', 'home', 'living', 'parents'];
  export const isTree = (view) => view.layout === 'tree' && NEEDS.every((n) => view.columns.some((c) => c.name === n));

  function lifespan(p) {
    if (p.living) return p.born_year ? `${p.born_year}–Living` : 'Living';
    if (!p.born_year && !p.died_year) return '';
    return `${p.born_year ?? '?'}–${p.died_year ?? '?'}`;
  }

  export function toBook(rows, partnerships) {
    const people = {};
    const unions = {};
    for (const p of rows) people[p.id] = { name: p.name, sex: p.sex, years: lifespan(p), living: p.living, photo: null, records: 0, parents: [], unions: [] };
    for (const u of partnerships) unions[u.id] = { a: people[u.a] ? u.a : null, b: people[u.b] ? u.b : null, kids: [] };
    for (const p of [...rows].sort((x, y) => (x.born_year ?? Infinity) - (y.born_year ?? Infinity))) {
      for (const u of p.parents ?? []) {
        if (!unions[u]) continue;
        unions[u].kids.push(p.id);
        people[p.id].parents.push(u);
      }
    }
    for (const [id, u] of Object.entries(unions)) for (const q of [u.a, u.b]) if (q) people[q].unions.push(id);
    return { people, unions };
  }

  export async function loadBook(view) {
    const [people, unions] = await Promise.all([fetchRows(view.name, { all: true }), fetchRows('partnership', { all: true })]);
    return { book: toBook(people.rows, unions.rows), home: (people.rows.find((p) => p.home) ?? people.rows[0])?.id ?? null };
  }
</script>

<script>
  import { untrack } from 'svelte';
  import Icon from '#kit/Icon.svelte';
  import { FV, familyLayout, startState, toggle } from './family.js';

  let { book, home } = $props();

  let s = $state(startState(untrack(() => book), untrack(() => home)));
  const l = $derived(familyLayout(book, s, home));
  let t = $state({ k: 1, x: 0, y: 0 });
  let frame = $state();
  let live = $state(false);

  const PAD = 32;
  const FAR = 0.5;
  const MIN = 0.12;
  const MAX = 2;
  const SILHOUETTE = 'M17.5 22a10.5 10.5 0 1 0 21 0a10.5 10.5 0 1 0-21 0ZM7 58c0-12.5 9.4-21 21-21s21 8.5 21 21Z';
  const CHEVRON = { up: 'M6 13l4-4 4 4', down: 'M6 8l4 4 4-4', left: 'M12 6l-4 4 4 4', right: 'M8 6l4 4-4 4' };

  const box = $derived(l.bounds);
  const clamp = (k) => Math.min(MAX, Math.max(MIN, k));
  const fitK = () => Math.min(frame.clientWidth / (box.w + PAD * 2), frame.clientHeight / (box.h + PAD * 2), 1);

  function zoomAt(by, x = frame.clientWidth / 2, y = frame.clientHeight / 2) {
    const k = clamp(t.k * by);
    t = { k, x: x - ((x - t.x) * k) / t.k, y: y - ((y - t.y) * k) / t.k };
  }

  function fit() {
    const k = fitK();
    t = { k, x: frame.clientWidth / 2 - (box.x + box.w / 2) * k, y: frame.clientHeight / 2 - (box.y + box.h / 2) * k };
  }

  function start() {
    if (fitK() >= FAR) return fit();
    const h = frame.clientHeight;
    const foot = l.focus.y + FV.h + FV.off + FV.chevron + 8;
    const k = Math.min(1, Math.max(FAR, (h - PAD * 2) / (foot - box.y)));
    t = { k, x: frame.clientWidth / 2, y: Math.min(h - PAD - foot * k, PAD - box.y * k) };
  }

  function press(act, anchor) {
    const before = l.cards.find((c) => c.id === anchor);
    s = toggle(book, s, act);
    const after = l.cards.find((c) => c.id === anchor);
    if (before && after) t = { ...t, x: t.x + (before.x - after.x) * t.k, y: t.y + (before.y - after.y) * t.k };
  }

  // Pan with one finger or the mouse, pinch with two. A drag that starts on a card must not open it.
  const fingers = new Map();
  let dragged = false;

  function down(e) {
    if (e.button || e.target.closest('button')) return;
    fingers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    dragged = false;
  }

  function move(e) {
    const was = fingers.get(e.pointerId);
    if (!was) return;
    const now = { x: e.clientX, y: e.clientY };
    if (fingers.size === 2) {
      const other = [...fingers].find(([id]) => id !== e.pointerId)[1];
      const r = frame.getBoundingClientRect();
      zoomAt(Math.hypot(now.x - other.x, now.y - other.y) / Math.hypot(was.x - other.x, was.y - other.y), (now.x + other.x) / 2 - r.left, (now.y + other.y) / 2 - r.top);
    } else {
      t = { ...t, x: t.x + now.x - was.x, y: t.y + now.y - was.y };
    }
    fingers.set(e.pointerId, now);
    if (Math.abs(now.x - was.x) + Math.abs(now.y - was.y) > 1) {
      dragged = true;
      if (!frame.hasPointerCapture(e.pointerId)) frame.setPointerCapture(e.pointerId);
    }
  }

  const up = (e) => fingers.delete(e.pointerId);

  function click(e) {
    if (!dragged) return;
    e.preventDefault();
    e.stopPropagation();
    dragged = false;
  }

  function wheel(e) {
    e.preventDefault();
    if (e.ctrlKey || e.metaKey) {
      const r = frame.getBoundingClientRect();
      zoomAt(2 ** (-e.deltaY / 300), e.clientX - r.left, e.clientY - r.top);
    } else {
      t = { ...t, x: t.x - e.deltaX, y: t.y - e.deltaY };
    }
  }

  $effect(() => {
    frame.addEventListener('wheel', wheel, { passive: false });
    frame.addEventListener('click', click, true);
    untrack(start);
    live = true;
    // The frame can shrink as things load above it: keep what stood at the bottom in view.
    let height = frame.clientHeight;
    const held = new ResizeObserver(() => {
      t = { ...t, y: t.y + frame.clientHeight - height };
      height = frame.clientHeight;
    });
    held.observe(frame);
    return () => {
      held.disconnect();
      frame.removeEventListener('wheel', wheel);
      frame.removeEventListener('click', click, true);
    };
  });
</script>

<div class="tree" class:live class:far={t.k < FAR} bind:this={frame} role="group" aria-label="Family tree" onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={up}>
  <div class="world" style="transform: translate({t.x}px, {t.y}px) scale({t.k})">
    <svg class="lines" style="left: {box.x}px; top: {box.y}px" width={box.w} height={box.h} viewBox="{box.x} {box.y} {box.w} {box.h}" aria-hidden="true">
      {#each l.lines as ln (ln.key)}<path class={ln.kind} d={ln.d} />{/each}
    </svg>
    {#each l.labels as lb (lb.key)}<span class="label" style="left: {lb.x}px; top: {lb.y}px">{lb.text}</span>{/each}
    {#each l.cards as c (c.id)}
      <a class="card {c.sex}" class:focus={c.focus} href="/person/{c.id}" draggable="false" style="left: {c.x}px; top: {c.y}px" title={c.years ? `${c.name}, ${c.years}` : c.name}>
        <span class="portrait"><svg viewBox="0 0 56 56" aria-hidden="true"><path d={SILHOUETTE} /></svg></span>
        <span class="name">{c.name}</span>
        <span class="years">{c.years}</span>
      </a>
    {/each}
    {#each l.chevrons as ch (ch.key)}
      <button class="chev" style="left: {ch.x}px; top: {ch.y}px" onclick={() => press(ch.act, ch.anchor)} aria-label={ch.label} title={ch.label} aria-expanded={ch.open}>
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d={CHEVRON[ch.dir]} /></svg>
      </button>
    {/each}
    {#each l.switches as sw (sw.key)}
      <button class="chev switch" style="left: {sw.x}px; top: {sw.y}px" onclick={() => press(sw.act, sw.anchor)} aria-label={sw.label} title={sw.label}><Icon name="users" /></button>
    {/each}
  </div>
  <div class="tools">
    <button onclick={() => zoomAt(1.4)} aria-label="Zoom in" title="Zoom in"><Icon name="plus" /></button>
    <button onclick={() => zoomAt(1 / 1.4)} aria-label="Zoom out" title="Zoom out"><Icon name="minus" /></button>
    <button onclick={fit} aria-label="See the whole tree" title="See the whole tree"><Icon name="maximize" /></button>
    <button onclick={start} aria-label="Back to the start" title="Back to the start"><Icon name="user" /></button>
  </div>
</div>

<style>
  .tree {
    --canvas: #2d3048;
    --line: #8a8fb2;
    --chev: #464c6e;
    --chev-hover: #5a6188;
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    border-radius: var(--radius-panel);
    background: var(--canvas);
    cursor: grab;
    touch-action: none;
    user-select: none;
  }
  .tree:active { cursor: grabbing; }
  .tree:not(.live) .world { visibility: hidden; }
  .world { position: absolute; top: 0; left: 0; transform-origin: 0 0; }
  .lines { position: absolute; max-width: none; overflow: visible; pointer-events: none; }
  .lines path { fill: none; stroke: var(--line); stroke-width: 2; }
  .lines .husband { stroke: #9db2ff; stroke-width: 3; }
  .lines .wife { stroke: #eba591; stroke-width: 3; }
  .label { position: absolute; width: max-content; max-width: 220px; padding: 2px 6px; transform: translate(-50%, -50%); background: var(--canvas); color: #e1e3f0; font: 700 17px / 20px var(--font-serif); text-align: center; pointer-events: none; }
  .card { position: absolute; display: flex; flex-direction: column; align-items: center; box-sizing: border-box; width: 100px; height: 152px; padding: 6px 6px 0; border-radius: 3px; background: #fff; box-shadow: 0 1px 2px rgb(0 0 20 / 45%); color: #000080; text-align: center; }
  .card.focus { box-shadow: 0 0 0 3px #e6e6fa, 0 2px 6px rgb(0 0 20 / 50%); }
  .portrait { flex: none; width: 88px; height: 88px; overflow: hidden; border-radius: 2px; background: #ececf4; }
  .portrait svg { display: block; width: 100%; height: 100%; fill: #9a9ab8; }
  .m .portrait { background: #dfe5f6; }
  .m .portrait svg { fill: #7f93c6; }
  .f .portrait { background: #f6e3dc; }
  .f .portrait svg { fill: #c27a62; }
  .name { display: -webkit-box; margin-top: 6px; overflow: hidden; font: 700 14px / 15px var(--font-serif); overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .years { margin-top: 2px; color: #4a4a78; font: 13px / 15px var(--font-serif); white-space: nowrap; }
  .far .name, .far .years, .far .label { font-family: var(--font-sans); }
  .chev { position: absolute; display: grid; place-items: center; width: 22px; height: 22px; margin: -11px 0 0 -11px; border-radius: 50%; background: var(--chev); color: #f2f3fa; cursor: pointer; }
  .chev:hover { background: var(--chev-hover); }
  .chev::after { content: ''; position: absolute; inset: -9px; border-radius: 50%; }
  .chev svg { width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  .switch { margin: -8px 0 0 -8px; }
  .tools { position: absolute; right: var(--space-3); bottom: var(--space-3); display: flex; flex-direction: column; gap: var(--space-1); }
  .tools button { display: grid; place-items: center; width: var(--size-control-lg); height: var(--size-control-lg); border-radius: var(--radius-nav); background: #fff; color: #000080; cursor: pointer; }
  .tools button:hover { background: #ededf6; }
</style>
