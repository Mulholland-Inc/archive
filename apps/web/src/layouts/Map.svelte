<script>
  import { Plus, Minus } from '@lucide/svelte';
  import Text from '../components/Text.svelte';
  import IconButton from '../components/IconButton.svelte';
  import Table from '../components/Table.svelte';
  import { fill } from './fill.js';
  import { titleOf } from '../lib/app.svelte.js';
  import { theme } from '../lib/theme.svelte.js';

  // A map: each row a pin at the view's by place (an archive.geo), beside a table of them that scrolls. A pin reads the row's
  // first money, or failing that its first number; the list each row's name and its other columns.
  let { view, rows, shown, text, pick } = $props();

  // A tile's side, as --view-map-tile.
  const TILE = 256;
  const at = (r) => {
    const g = r[view.by];
    return g && Number.isFinite(g.lat) && Number.isFinite(g.lon) ? g : null;
  };
  const placed = $derived(rows.filter(at));
  const label = $derived(shown.find((c) => c.type === 'money') ?? shown.find((c) => c.type === 'number'));
  const details = $derived(shown.filter((c) => c.name !== view.by && c !== label).slice(0, 3));

  // A place in pixels of the whole world at zoom z (web mercator, as the tiles are drawn).
  const world = (z) => TILE * 2 ** z;
  const project = ({ lat, lon }, z) => {
    const s = Math.sin((Math.max(-85, Math.min(85, lat)) * Math.PI) / 180);
    return [((lon + 180) / 360) * world(z), (0.5 - Math.log((1 + s) / (1 - s)) / (4 * Math.PI)) * world(z)];
  };

  let width = $state(0);
  let height = $state(0);
  let zoom = $state(12);
  // The map's middle, in world pixels at zoom.
  let center = $state([0, 0]);
  let fitted = false;

  // At first, and when the places change, the map fits them all.
  function fit() {
    if (!width || !height || !placed.length) return;
    let z = 17;
    for (; z > 1; z--) {
      const ps = placed.map((r) => project(at(r), z));
      const xs = ps.map((p) => p[0]);
      const ys = ps.map((p) => p[1]);
      if (Math.max(...xs) - Math.min(...xs) < width - 120 && Math.max(...ys) - Math.min(...ys) < height - 120) {
        zoom = z;
        center = [(Math.max(...xs) + Math.min(...xs)) / 2, (Math.max(...ys) + Math.min(...ys)) / 2];
        return;
      }
    }
  }
  $effect(() => {
    placed;
    if (width && height && !fitted) {
      fit();
      fitted = true;
    }
  });

  const left = $derived(center[0] - width / 2);
  const top = $derived(center[1] - height / 2);
  // The tiles that cover the map at zoom z, each drawn at the map's zoom. Under the map's own tiles are those of
  // the zoom before, already loaded, so a zoom shows them blurred until its own arrive rather than nothing.
  function cover(z) {
    const n = 2 ** z;
    const size = TILE * 2 ** (zoom - z);
    const out = [];
    for (let y = Math.floor(top / size); y <= Math.floor((top + height) / size); y++) {
      if (y < 0 || y >= n) continue;
      for (let x = Math.floor(left / size); x <= Math.floor((left + width) / size); x++) {
        out.push({ key: `${z}/${x}/${y}`, x: x * size - left, y: y * size - top, scale: 2 ** (zoom - z), src: `${z}/${y}/${((x % n) + n) % n}` });
      }
    }
    return out;
  }
  const tiles = $derived([...cover(zoom - 1), ...cover(zoom)]);
  const style = $derived(theme.resolved === 'dark' ? 'World_Dark_Gray_Base' : 'World_Light_Gray_Base');

  // Zooming keeps the point under the cursor, or the middle, where it is.
  function zoomBy(d, px = width / 2, py = height / 2) {
    const z = Math.max(2, Math.min(18, zoom + d));
    if (z === zoom) return;
    const k = 2 ** (z - zoom);
    center = [(left + px) * k - px + width / 2, (top + py) * k - py + height / 2];
    zoom = z;
  }

  let drag = null;
  let moved = false;
  function down(e) {
    if (e.button !== 0 || e.target.closest('button')) return;
    drag = { x: e.clientX, y: e.clientY, center };
    moved = false;
    e.currentTarget.setPointerCapture(e.pointerId);
  }
  function move(e) {
    if (!drag) return;
    const dx = e.clientX - drag.x;
    const dy = e.clientY - drag.y;
    if (Math.abs(dx) + Math.abs(dy) > 3) moved = true;
    center = [drag.center[0] - dx, drag.center[1] - dy];
  }
  function up() {
    drag = null;
    if (!moved) selected = null;
  }
  let wheelAt = 0;
  function wheel(e) {
    e.preventDefault();
    if (Date.now() - wheelAt < 250 || Math.abs(e.deltaY) < 4) return;
    wheelAt = Date.now();
    const box = e.currentTarget.getBoundingClientRect();
    zoomBy(e.deltaY < 0 ? 1 : -1, e.clientX - box.left, e.clientY - box.top);
  }

  let hovered = $state(null);
  let selected = $state(null);
  const short = (c, r) => {
    const v = r[c.name];
    if (c.type === 'money' && v?.amount != null) return new Intl.NumberFormat('en-US', { style: 'currency', currency: v.currency || 'USD', notation: 'compact', maximumFractionDigits: 2 }).format(v.amount);
    return text(c, r);
  };
  const pins = $derived(
    placed
      .map((r) => {
        const [x, y] = project(at(r), zoom);
        return { r, x: x - left, y: y - top };
      })
      .sort((a, b) => a.y - b.y)
  );
  const name = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());
  // Beside the map, its rows as a table, each column the same as in a table view's; money and numbers line up on
  // the right. The row of the pin that is open is selected.
  const money = (v) => new Intl.NumberFormat('en-US', { style: 'currency', currency: v.currency || 'USD', maximumFractionDigits: 0 }).format(v.amount);
  const cols = $derived([...(label ? [label] : []), ...shown.filter((c) => c.name !== view.by && c !== label)]);
  const columns = $derived([{ key: 'name', label: 'Name' }, ...cols.map((c) => ({ key: c.name, label: name(c), align: c.type === 'money' || c.type === 'number' ? 'end' : undefined }))]);
  const cells = $derived(rows.map((r, i) => ({ id: r.id ?? i, row: r, name: titleOf(r, view), ...Object.fromEntries(cols.map((c) => [c.name, c.type === 'money' && r[c.name]?.amount != null ? money(r[c.name]) : c.type === 'number' && typeof r[c.name] === 'number' ? r[c.name].toLocaleString('en-US') : text(c, r)])) })));
  // The open pin's row scrolls into the table's view.
  function follow(node) {
    $effect(() => {
      if (selected) node.querySelector('tr.selected')?.scrollIntoView({ block: 'nearest' });
    });
  }
  const focus = (r) => r === hovered || r === selected;
</script>

<!-- The map and its table fill the window, so the page does not scroll and the wheel over the map zooms it. -->
<div class="map-view" {@attach fill()}>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="map" bind:clientWidth={width} bind:clientHeight={height} onpointerdown={down} onpointermove={move} onpointerup={up} onwheel={wheel} class:dragging={drag}>
    {#each tiles as t (t.key)}
      <img src={`https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/${style}/MapServer/tile/${t.src}`} alt="" draggable="false" style:--x={`${t.x}px`} style:--y={`${t.y}px`} style:--scale={t.scale} />
    {/each}
    {#each pins as p (p.r.id)}
      <button
        type="button"
        class={['pin', { focus: focus(p.r), dot: !label }]}
        style:--x={`${p.x}px`}
        style:--y={`${p.y}px`}
        onclick={() => (selected = selected === p.r ? null : p.r)}
        onmouseenter={() => (hovered = p.r)}
        onmouseleave={() => (hovered = null)}
        aria-label={titleOf(p.r, view)}
      >
        {#if label}{short(label, p.r)}{/if}
      </button>
    {/each}
    {#if selected && pins.find((p) => p.r === selected)}
      {@const p = pins.find((p) => p.r === selected)}
      <button type="button" class="popup" style:--x={`${p.x}px`} style:--y={`${p.y}px`} onclick={() => pick(selected)}>
        <Text size="sm" weight="medium">{titleOf(selected, view)}</Text>
        {#if label}<Text size="sm">{short(label, selected)}</Text>{/if}
        {#if details.some((c) => text(c, selected))}
          <dl>
            {#each details.filter((c) => text(c, selected)) as c (c.name)}<dt>{name(c)}</dt><dd>{text(c, selected)}</dd>{/each}
          </dl>
        {/if}
      </button>
    {/if}
    <div class="zoom">
      <IconButton icon={Plus} label="Zoom in" variant="secondary" size="sm" onclick={() => zoomBy(1)} />
      <IconButton icon={Minus} label="Zoom out" variant="secondary" size="sm" onclick={() => zoomBy(-1)} />
    </div>
    <span class="credit">Esri, HERE, Garmin, © OpenStreetMap</span>
  </div>

  <div class="side" {@attach follow}>
    <Table {columns} rows={cells} label={view.name} selected={selected ? [selected.id] : []} onrowclick={(c) => pick(c.row)} />
  </div>
</div>

<style>
  .map-view {
    display: grid;
    height: var(--view-fill);
    grid-template-columns: minmax(0, 1fr) minmax(0, var(--view-map-aside));
    gap: var(--space-4);
  }

  .map {
    position: relative;
    overflow: hidden;
    border-radius: var(--radius-lg);
    background: var(--color-fill);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
    touch-action: none;
    cursor: grab;
    user-select: none;
  }

  /* The table scrolls in its own box, its header row staying at the top. */
  .side {
    min-height: 0;
  }

  .side :global(.wrap) {
    height: 100%;
    overflow: auto;
  }

  .side :global(th) {
    position: sticky;
    top: 0;
    z-index: var(--z-raised);
    background: var(--color-canvas);
  }

  .dragging {
    cursor: grabbing;
  }

  /* Tiles, pins and the popup are placed by the --x and --y the map works out for each. */
  img {
    position: absolute;
    top: 0;
    left: 0;
    width: calc(var(--view-map-tile) * var(--scale));
    height: calc(var(--view-map-tile) * var(--scale));
    transform: translate(var(--x), var(--y));
    pointer-events: none;
  }

  .pin {
    position: absolute;
    top: 0;
    left: 0;
    transform: translate(var(--x), var(--y));
    translate: -50% -100%;
    margin-top: calc(var(--view-pin-tip) / -2);
    padding: var(--space-0-5) var(--space-2);
    border-radius: var(--radius-full);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
    color: var(--color-text);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  /* The pin points at its place. */
  .pin::after {
    content: '';
    position: absolute;
    left: 50%;
    bottom: calc(var(--view-pin-tip) / -2);
    width: var(--view-pin-tip);
    height: var(--view-pin-tip);
    translate: -50% 0;
    rotate: 45deg;
    background: inherit;
    box-shadow: inset calc(var(--border-width) * -1) calc(var(--border-width) * -1) 0 0 var(--color-border-strong);
    z-index: -1;
  }

  .pin.dot {
    width: var(--view-pin-dot);
    height: var(--view-pin-dot);
    padding: 0;
    translate: -50% -50%;
    margin: 0;
  }

  .pin.dot::after {
    display: none;
  }

  .pin.focus {
    z-index: var(--z-raised);
    background: var(--color-inverse);
    color: var(--color-text-inverse);
  }

  .popup {
    position: absolute;
    top: 0;
    left: 0;
    transform: translate(var(--x), var(--y));
    z-index: var(--z-sticky);
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
    width: var(--view-pin-popup);
    translate: -50% calc(-100% - var(--control-lg));
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background: var(--color-surface-raised);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    text-align: start;
    cursor: pointer;
  }

  dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: var(--space-0-5) var(--space-3);
    margin-block-start: var(--space-1-5);
    font-size: var(--text-sm);
  }

  dt {
    color: var(--color-text-tertiary);
  }

  dd {
    color: var(--color-text);
  }

  .zoom {
    position: absolute;
    right: var(--space-3);
    bottom: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .credit {
    position: absolute;
    left: var(--space-2);
    bottom: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-size: var(--text-2xs);
  }

  /* On a narrow page the table goes under the map, each half the height. */
  @container (max-width: 48rem) {
    .map-view {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(0, 1fr) minmax(0, 1fr);
    }
  }
</style>
