<script>
  import SchemaNode from './SchemaNode.svelte';
  import SchemaEdge from './SchemaEdge.svelte';
  import ZoomControls from './ZoomControls.svelte';

  let { types = [], query = '', onopen } = $props();

  const GAP_X = 96;
  const GAP_Y = 32;
  const LOOP = 28;
  const MIN = 0.25;
  const MAX = 2;

  let svg = $state();
  let vw = $state(0);
  let vh = $state(0);
  let view = $state({ x: 0, y: 0, k: 1 });
  let sizes = $state({});
  let hover = $state(null);
  let fitted = $state(false);

  const ids = $derived(new Set(types.map((t) => t.id)));
  const names = $derived(Object.fromEntries(types.map((t) => [t.id, t.name])));
  const edges = $derived(
    types.flatMap((t) => t.links.filter((l) => ids.has(l.to)).map((l) => ({ id: `${t.id}.${l.column}`, from: t.id, column: l.column, to: l.to })))
  );

  const layers = $derived.by(() => {
    const out = {};
    const visiting = new Set();
    const depth = (id) => {
      if (id in out) return out[id];
      if (visiting.has(id)) return 0;
      visiting.add(id);
      const t = types.find((x) => x.id === id);
      const targets = t.links.map((l) => l.to).filter((to) => to !== id && ids.has(to));
      out[id] = targets.length ? 1 + Math.max(...targets.map(depth)) : 0;
      visiting.delete(id);
      return out[id];
    };
    types.forEach((t) => depth(t.id));
    const cols = [];
    for (const t of types) (cols[out[t.id]] ??= []).push(t.id);
    return cols.filter(Boolean);
  });

  const size = (id) => sizes[id] ?? { w: 240, h: 200, anchors: { '': 20 } };

  const layout = $derived.by(() => {
    const order = layers.map((c) => [...c]);
    const neighbors = (id) => edges.filter((e) => e.from === id || e.to === id).map((e) => (e.from === id ? e.to : e.from));
    for (let pass = 0; pass < 2; pass++) {
      for (let i = 1; i < order.length; i++) {
        const prev = order[i - 1];
        const score = (id) => {
          const n = neighbors(id).map((x) => prev.indexOf(x)).filter((x) => x >= 0);
          return n.length ? n.reduce((a, b) => a + b, 0) / n.length : prev.length / 2;
        };
        order[i].sort((a, b) => score(a) - score(b));
      }
    }
    const pos = {};
    let x = 0;
    for (const col of order) {
      const w = Math.max(...col.map((id) => size(id).w));
      const total = col.reduce((a, id) => a + size(id).h, 0) + GAP_Y * (col.length - 1);
      let y = -total / 2;
      for (const id of col) {
        pos[id] = { x, y };
        y += size(id).h + GAP_Y;
      }
      x += w + GAP_X;
    }
    return pos;
  });

  const bounds = $derived.by(() => {
    const list = Object.entries(layout);
    if (!list.length) return { x: 0, y: 0, w: 1, h: 1 };
    const x0 = Math.min(...list.map(([, p]) => p.x)) - LOOP;
    const y0 = Math.min(...list.map(([, p]) => p.y));
    const x1 = Math.max(...list.map(([id, p]) => p.x + size(id).w));
    const y1 = Math.max(...list.map(([id, p]) => p.y + size(id).h));
    return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
  });

  function path(e) {
    const a = layout[e.from];
    const b = layout[e.to];
    if (!a || !b) return '';
    const sa = size(e.from);
    const sb = size(e.to);
    const y1 = a.y + (sa.anchors[e.column] ?? sa.anchors['']);
    const y2 = b.y + sb.anchors[''];
    if (e.from === e.to) {
      return `M ${a.x} ${y1} C ${a.x - LOOP} ${y1}, ${a.x - LOOP} ${y2}, ${a.x} ${y2}`;
    }
    const left = b.x + sb.w <= a.x;
    const x1 = left ? a.x : a.x + sa.w;
    const x2 = left ? b.x + sb.w : b.x;
    const dx = Math.max(40, Math.abs(x2 - x1) / 2) * (left ? -1 : 1);
    return `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
  }

  const q = $derived(query.trim().toLowerCase());
  const matches = $derived(q ? new Set(types.filter((t) => t.name.toLowerCase().includes(q) || t.columns.some((c) => c.name.includes(q))).map((t) => t.id)) : null);
  const related = $derived.by(() => {
    if (!hover) return null;
    const set = new Set([hover]);
    for (const e of edges) if (e.from === hover || e.to === hover) set.add(e.from).add(e.to);
    return set;
  });

  function nodeState(id) {
    if (matches) return matches.has(id) ? 'match' : 'dim';
    if (related) return id === hover ? 'active' : related.has(id) ? 'idle' : 'dim';
    return 'idle';
  }

  function edgeState(e) {
    if (matches) return matches.has(e.from) || matches.has(e.to) ? 'idle' : 'dim';
    if (hover) return e.from === hover || e.to === hover ? 'active' : 'dim';
    return 'idle';
  }

  const clamp = (k) => Math.min(MAX, Math.max(MIN, k));

  function fit(ids = null) {
    if (!vw || !vh) return;
    let b = bounds;
    if (ids?.size) {
      const list = [...ids].filter((id) => layout[id]);
      const x0 = Math.min(...list.map((id) => layout[id].x));
      const y0 = Math.min(...list.map((id) => layout[id].y));
      const x1 = Math.max(...list.map((id) => layout[id].x + size(id).w));
      const y1 = Math.max(...list.map((id) => layout[id].y + size(id).h));
      b = { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
    }
    const pad = 48;
    const k = clamp(Math.min((vw - pad * 2) / b.w, (vh - pad * 2) / b.h, 1));
    view = { k, x: vw / 2 - (b.x + b.w / 2) * k, y: vh / 2 - (b.y + b.h / 2) * k };
  }

  function zoomAt(k, cx = vw / 2, cy = vh / 2) {
    const next = clamp(k);
    const r = next / view.k;
    view = { k: next, x: cx - (cx - view.x) * r, y: cy - (cy - view.y) * r };
  }

  $effect(() => {
    const measured = types.every((t) => sizes[t.id]);
    if (!fitted && measured && vw && vh) {
      fit();
      fitted = true;
    }
  });

  $effect(() => {
    matches;
    if (fitted) fit(matches);
  });

  const pointers = new Map();
  let start = null;
  let moved = false;

  function down(event) {
    pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    moved = false;
    start = { view: { ...view }, points: new Map(pointers) };
  }

  function move(event) {
    if (!pointers.has(event.pointerId)) return;
    pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    const [p1, p2] = [...pointers.values()];
    const [s1, s2] = [...start.points.values()];
    if (!s1) return;
    if (p2 && s2) {
      const rect = svg.getBoundingClientRect();
      const d0 = Math.hypot(s2.x - s1.x, s2.y - s1.y) || 1;
      const d1 = Math.hypot(p2.x - p1.x, p2.y - p1.y);
      const k = clamp(start.view.k * (d1 / d0));
      const c0 = { x: (s1.x + s2.x) / 2 - rect.left, y: (s1.y + s2.y) / 2 - rect.top };
      const c1 = { x: (p1.x + p2.x) / 2 - rect.left, y: (p1.y + p2.y) / 2 - rect.top };
      const r = k / start.view.k;
      view = { k, x: c1.x - (c0.x - start.view.x) * r, y: c1.y - (c0.y - start.view.y) * r };
      moved = true;
      return;
    }
    const dx = p1.x - s1.x;
    const dy = p1.y - s1.y;
    if (!moved && Math.hypot(dx, dy) < 4) return;
    if (!moved) svg.setPointerCapture(event.pointerId);
    moved = true;
    view = { ...view, x: start.view.x + dx, y: start.view.y + dy };
  }

  function up(event) {
    pointers.delete(event.pointerId);
    start = { view: { ...view }, points: new Map(pointers) };
  }

  function open(id) {
    if (moved) return;
    onopen?.(id);
  }

  $effect(() => {
    if (!svg) return;
    const wheel = (event) => {
      event.preventDefault();
      const rect = svg.getBoundingClientRect();
      const factor = Math.exp(-event.deltaY * (event.ctrlKey ? 0.01 : 0.002));
      zoomAt(view.k * factor, event.clientX - rect.left, event.clientY - rect.top);
    };
    svg.addEventListener('wheel', wheel, { passive: false });
    return () => svg.removeEventListener('wheel', wheel);
  });
</script>

<div class="canvas" bind:clientWidth={vw} bind:clientHeight={vh}>
  <svg
    bind:this={svg}
    class={['graph', { ready: fitted }]}
    role="application"
    aria-label="Tables and how they link"
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={up}
  >
    <g transform={`translate(${view.x} ${view.y}) scale(${view.k})`}>
      {#each edges as e (e.id)}
        <SchemaEdge d={path(e)} mode={edgeState(e)} />
      {/each}
      {#each types as t (t.id)}
        {@const p = layout[t.id] ?? { x: 0, y: 0 }}
        {@const s = sizes[t.id]}
        <foreignObject class={{ dim: nodeState(t.id) === 'dim' }} x={p.x} y={p.y} width={s?.w ?? 400} height={s?.h ?? 800}>
          <SchemaNode type={t} {names} mode={nodeState(t.id)} onclick={() => open(t.id)} onhover={(id) => (hover = id)} onmeasure={(m) => (sizes[t.id] = m)} />
        </foreignObject>
      {/each}
    </g>
  </svg>
  <div class="controls">
    <ZoomControls zoom={view.k} onin={() => zoomAt(view.k * 1.2)} onout={() => zoomAt(view.k / 1.2)} onfit={() => fit()} />
  </div>
</div>

<style>
  .canvas {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .graph {
    display: block;
    width: 100%;
    height: 100%;
    touch-action: none;
    cursor: grab;
    opacity: 0;
  }

  .graph:active {
    cursor: grabbing;
  }

  .ready {
    opacity: 1;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  foreignObject {
    overflow: visible;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .dim {
    opacity: var(--opacity-dim);
  }

  .controls {
    position: absolute;
    inset-block-end: var(--space-4);
    inset-inline-end: var(--space-4);
  }
</style>
