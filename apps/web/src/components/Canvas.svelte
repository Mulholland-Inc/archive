<script>
  import { tick } from 'svelte';
  import { X, Pencil, Share, Trash2, Download, Copy, FileQuestion } from '@lucide/svelte';
  import TopBar from './TopBar.svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import Badge from './Badge.svelte';
  import Tabs from './Tabs.svelte';
  import EmptyState from './EmptyState.svelte';
  import Skeleton from './Skeleton.svelte';
  import { app, canvas, closeFile, editFile, copyFile, downloadFile, openShare, removeFile } from '../lib/app.svelte.js';
  import { blobURL, send, local, setLocal } from '../lib/api.js';
  import { renameFilePrompt } from '../lib/menus.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';

  // One of these per file (App.svelte), so nothing held for one is ever taken for another's.
  const { id } = app.canvas;

  canvas.use(() => id);

  const file = $derived(app.canvas?.file);
  const model = $derived(app.canvas?.model);
  const synced = $derived(Boolean(file?.connection));
  const remove = async () => {
    if (await confirmDialog({ title: `Delete ${file.name}?`, confirm: 'Delete' })) removeFile(file.id);
  };
  const shown = $derived(!file?.held ? '' : file.mime?.startsWith('image/') ? 'image' : file.mime === 'application/pdf' ? 'pdf' : file.mime?.startsWith('audio/') ? 'audio' : file.mime?.startsWith('video/') ? 'video' : file.mime?.startsWith('text/') ? 'text' : '');

  // The pane

  let aside = $state();
  let closer = $state();
  let width = $state(local('archive.canvas', 0));

  function drag(event) {
    event.preventDefault();
    const handle = event.currentTarget;
    handle.setPointerCapture(event.pointerId);
    const right = aside.getBoundingClientRect().right;
    const move = (e) => (width = Math.round(right - e.clientX));
    const up = () => {
      handle.removeEventListener('pointermove', move);
      handle.removeEventListener('pointerup', up);
      keep();
    };
    handle.addEventListener('pointermove', move);
    handle.addEventListener('pointerup', up);
  }

  // What the pane's limits let it be is what is kept.
  async function keep() {
    await tick();
    width = aside.offsetWidth;
    setLocal('archive.canvas', width);
  }

  function nudge(event) {
    const by = { ArrowLeft: 32, ArrowRight: -32 }[event.key];
    if (!by) return;
    event.preventDefault();
    width = aside.offsetWidth + by;
    keep();
  }

  function onkeydown(event) {
    if (event.key === 'Escape' && !event.defaultPrevented && !document.querySelector('dialog[open], :popover-open')) closeFile();
  }

  // Taking over a narrow screen takes the focus along, and closing gives it back.
  $effect(() => {
    const before = document.activeElement;
    if (matchMedia('(max-width: 56rem)').matches) requestAnimationFrame(() => closer?.querySelector('button')?.focus());
    return () => before?.isConnected && before.focus();
  });

  // What the browser shows itself

  let src = $state('');
  let text = $state('');

  $effect(() => {
    const f = file;
    if (!f || !shown) return;
    f.hash;
    if (shown === 'text') send(f.path).then((r) => r.text()).then((t) => (text = t)).catch(() => {});
    else blobURL(f.path).then((u) => (src = u)).catch(() => {});
  });

  // Text a person edits: a paragraph, a table cell's, a shape's. held is what each field being edited started from: a
  // read moves what is under a field, not what is in it, and what is saved is what changed since it started.

  let held = $state({});
  let refused = $state(null);

  // A shape's paragraphs are one line each, a paragraph's text is what it holds.
  const typed = (el) => (el.classList.contains('lines') && el.children.length ? [...el.children].map((c) => c.textContent).join('\n') : el.textContent.replace(/\n$/, ''));

  // A run of text as the file formats it. On a slide it has the slide's own size and color; on a page, the page's type.
  function run(s, slideWidth) {
    const e = document.createElement('span');
    e.textContent = s.text;
    if (s.bold) e.style.fontWeight = 'var(--weight-semibold)';
    if (s.italic) e.style.fontStyle = 'italic';
    if (s.under || s.strike) e.style.textDecoration = [s.under && 'underline', s.strike && 'line-through'].filter(Boolean).join(' ');
    if (slideWidth) {
      e.style.fontSize = `${(s.size / slideWidth) * 100}cqw`;
      if (s.color) e.style.color = s.color;
    } else if (s.link) {
      e.style.color = 'var(--color-accent)';
    } else if (s.color && s.color !== '#000000') {
      e.className = 'inked';
      e.style.setProperty('--ink', s.color);
    }
    return e;
  }

  const media = (part) => (el) => {
    blobURL(`/api/files/${id}/media/${part}`).then((u) => (el.src = u)).catch(() => {});
  };

  // EMU as a share of the slide, for what must scale with it.
  const share = (v, of) => `${(v / of) * 100}%`;
  const boxOf = (b) => (b ? `left:${share(b.x, model.width)};top:${share(b.y, model.height)};width:${share(b.w, model.width)};height:${share(b.h, model.height)};${b.rot ? `transform:rotate(${b.rot}deg);` : ''}` : 'left:5%;top:5%;width:90%;');
  const insetOf = (s) => (s.inset ? [1, 2, 3, 0].map((i) => `${(s.inset[i] / model.width) * 100}cqw`).join(' ') : '');
  const shapeLook = (s) => `${boxOf(s.box)}z-index:${s.z ?? 0};${s.fill ? `background:${s.fill};` : ''}${s.line ? `border:1px solid ${s.line};` : ''}${s.geom === 'ellipse' ? 'border-radius:50%;' : s.geom === 'roundRect' ? `border-radius:${(Math.min(s.box?.w ?? 0, s.box?.h ?? 0) / 6 / model.width) * 100}cqw;` : ''}justify-content:${{ ctr: 'center', b: 'flex-end' }[s.anchor] ?? 'flex-start'};`;

  function drawShape(el, s) {
    const pt = model.width / 12700;
    el.replaceChildren(...s.paragraphs.map((p) => {
      const line = document.createElement('div');
      line.className = 'line';
      if (p.align) line.style.textAlign = p.align === 'both' ? 'justify' : p.align;
      if (p.marker) {
        line.dataset.marker = p.marker;
        line.style.setProperty('--level', p.level ?? 0);
        line.style.setProperty('--ink', p.spans?.[0]?.color ?? 'inherit');
      }
      const size = p.spans?.[0]?.size ?? 18;
      line.style.fontSize = `${(size / pt) * 100}cqw`;
      line.append(...(p.spans ?? [{ text: p.text, size: 18 }]).map((r) => run(r, pt)));
      return line;
    }));
  }

  async function leave(el, key, ops) {
    const base = held[key];
    const next = typed(el);
    if (next === base) {
      if (refused?.key === key) refused = null;
      delete held[key];
      return;
    }
    try {
      await editFile(id, ops(base, next));
      if (refused?.key === key) refused = null;
      delete held[key];
    } catch (e) {
      refused = { key, text: e.message };
    }
  }

  function press(event, key) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      event.currentTarget.blur();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.currentTarget.textContent = held[key];
      event.currentTarget.blur();
    }
  }

  // The smallest replace_text that turns old into next, so the formatting around a change stays. Its find is the
  // first of its kind in old, as replace_text takes the first.
  function change(old, next) {
    const low = (s, i) => /[\uDC00-\uDFFF]/.test(s[i] ?? '');
    let a = 0;
    let z = 0;
    while (a < old.length && a < next.length && old[a] === next[a]) a++;
    if (low(old, a) || low(next, a)) a--;
    while (z < old.length - a && z < next.length - a && old[old.length - 1 - z] === next[next.length - 1 - z]) z++;
    if (z && low(old, old.length - z)) z--;
    while (a > 0 && (a + z === old.length || old.indexOf(old.slice(a, old.length - z)) !== a)) a--;
    while (z > 0 && a + z === old.length) z--;
    return { find: old.slice(a, old.length - z), replace: next.slice(a, next.length - z) };
  }

  function paragraphOps(p, only) {
    return (base, next) => {
      if (base) return [{ op: 'replace_text', para: p.id, ...change(base, next) }];
      const at = p.in?.match(/^t(\d+)\/r(\d+)\/c(\d+)$/);
      if (at && only) return [{ op: 'set_cell_text', table: +at[1], row: +at[2], col: +at[3], text: next }];
      return [{ op: 'insert_paragraph_after', para: p.id, text: next }, { op: 'delete_paragraph', para: p.id }];
    };
  }

  // A document as the model has it: paragraphs in order, those of a table gathered where the table starts.
  const blocks = $derived.by(() => {
    const out = [];
    const tables = new Map();
    for (const p of model?.paragraphs ?? []) {
      const at = p.in?.match(/^t(\d+)\/r(\d+)\/c(\d+)$/);
      if (!at) {
        out.push({ key: p.id, p });
        continue;
      }
      let t = tables.get(at[1]);
      if (!t) {
        t = { key: `t${at[1]}`, rows: [] };
        tables.set(at[1], t);
        out.push(t);
      }
      ((t.rows[+at[2]] ??= [])[+at[3]] ??= []).push(p);
    }
    return out;
  });

  const look = (p) => {
    const s = p.style?.toLowerCase() ?? '';
    return s === 'title' || s === 'subtitle' ? s : (s.match(/^heading([1-6])$/)?.[0] ?? (s.includes('quote') ? 'quote' : ''));
  };

  // A table's cell as it sits on the table's columns.
  const tableAt = (key) => model.tables?.[+key.slice(1)];

  const inlineOf = (p) => (p.spans ? (el) => el.replaceChildren(...p.spans.map((s) => run(s))) : null);

  // A sheet: a grid of what is in it and some room to go on, of which only the rows in view are drawn.

  const ROW = 28;
  const COL = 96;
  const number = /^-?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i;

  let sheetName = $state('');
  let at = $state('');
  let draft = $state(null);
  let pending = $state({});
  let scroller = $state();
  let top = $state(0);
  let tall = $state(600);
  let wide = $state(600);

  const sheet = $derived(model?.sheets?.find((s) => s.name === sheetName) ?? model?.sheets?.[0]);
  const tabs = $derived((model?.sheets ?? []).map((s) => ({ value: s.name, label: s.name })));
  const columnName = (n) => {
    let s = '';
    for (n++; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
    return s;
  };
  const place = (ref) => {
    const [, c, r] = ref.match(/^([A-Z]+)(\d+)$/);
    return { col: [...c].reduce((n, x) => n * 26 + x.charCodeAt(0) - 64, 0) - 1, row: +r - 1 };
  };
  const refOf = (col, row) => columnName(col) + (row + 1);

  const grid = $derived.by(() => {
    const cells = new Map();
    const spans = new Map();
    const hidden = new Set();
    let cols = 0;
    let rows = 0;
    for (const c of sheet?.cells ?? []) {
      cells.set(c.ref, c);
      const { col, row } = place(c.ref);
      cols = Math.max(cols, col + 1);
      rows = Math.max(rows, row + 1);
    }
    for (const m of sheet?.merged ?? []) {
      const [a, b] = m.split(':').map(place);
      spans.set(refOf(a.col, a.row), { cols: b.col - a.col + 1, rows: b.row - a.row + 1 });
      for (let r = a.row; r <= b.row; r++) for (let c = a.col; c <= b.col; c++) if (r !== a.row || c !== a.col) hidden.add(refOf(c, r));
      cols = Math.max(cols, b.col + 1);
      rows = Math.max(rows, b.row + 1);
    }
    return { cells, spans, hidden, used: { cols, rows }, cols: Math.max(cols + 2, Math.ceil(wide / COL)), rows: Math.max(rows + 6, Math.ceil(tall / ROW)) };
  });

  // A column's width in pixels, as Excel turns its characters into them; a hidden one is not drawn.
  const widths = $derived.by(() => {
    const out = new Map();
    for (const c of sheet?.cols ?? []) for (let n = c.min; n <= Math.min(c.max, grid.cols); n++) out.set(n - 1, c.hidden ? 0 : c.width ? Math.round(c.width * 7 + 5) : COL);
    return out;
  });
  const widthOf = (c) => widths.get(c) ?? COL;
  const here = $derived(at ? place(at) : { col: -1, row: -1 });

  // The file's look of a cell. Black and white are the page's, so they follow the theme.
  function cellLook(c) {
    const st = c && model.styles?.[c.style ?? 0];
    if (!c || (!st && !c.ink)) return { css: '', painted: false, inked: false };
    const ink = c.ink || (st?.color !== '#000000' && st?.color) || '';
    const fill = st?.fill !== '#FFFFFF' && st?.fill;
    const edge = (v) => (v === 'auto' ? 'var(--color-border-strong)' : v);
    const css = [
      st?.bold && 'font-weight:var(--weight-semibold)',
      st?.italic && 'font-style:italic',
      (st?.under || st?.strike) && `text-decoration:${[st.under && 'underline', st.strike && 'line-through'].filter(Boolean).join(' ')}`,
      ink && `--ink:${ink}`,
      fill && `--fill:${fill}`,
      ['left', 'center', 'right'].includes(st?.align) && `text-align:${st.align}`,
      st?.indent && `padding-inline-start:calc(var(--space-1-5) + ${st.indent}em)`,
      st?.top && `border-top:1px solid ${edge(st.top)}`,
      st?.right && `border-right:1px solid ${edge(st.right)}`,
      st?.bottom && `border-bottom:1px solid ${edge(st.bottom)}`,
      st?.left && `border-left:1px solid ${edge(st.left)}`
    ].filter(Boolean).join(';');
    return { css, painted: Boolean(fill), inked: Boolean(ink) };
  }

  // Text runs on over the empty cells beside it, as a sheet's does; a number never does.
  const spills = (ref, c, col, row) => Boolean(c?.value) && !number.test(c.value) && !c.formula && !grid.spans.has(ref) && !grid.cells.get(refOf(col + 1, row))?.value;

  const first = $derived(Math.max(0, Math.floor(top / ROW) - 5));
  const last = $derived(Math.min(grid.rows, first + Math.ceil(tall / ROW) + 10));
  const seen = $derived(Array.from({ length: Math.max(0, last - first) }, (_, i) => first + i));
  const columns = $derived(Array.from({ length: grid.cols }, (_, i) => i));

  const keyOf = (ref) => `${sheet?.name}!${ref}`;
  const raw = (ref) => {
    const c = grid.cells.get(ref);
    return c?.formula ? `=${c.formula}` : (c?.value ?? '');
  };
  const face = (ref) => pending[keyOf(ref)] ?? grid.cells.get(ref)?.text ?? grid.cells.get(ref)?.value ?? raw(ref);

  async function select(ref) {
    if (refused && at && refused.key !== keyOf(ref) && refused.key.endsWith(`!${at}`)) revert();
    at = ref;
    await tick();
    scroller?.querySelector(`[data-ref="${ref}"]`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }

  function revert() {
    delete pending[refused.key];
    refused = null;
  }

  function move(dc, dr) {
    if (!at) return select('A1');
    let { col, row } = place(at);
    do {
      col = Math.max(0, col + dc);
      row = Math.max(0, row + dr);
    } while (grid.hidden.has(refOf(col, row)));
    grid.rows > row && grid.cols > col && select(refOf(col, row));
  }

  function write(ref, text) {
    const where = { cell: ref, sheet: sheet.name };
    if (text === '') return { op: 'clear_cell', ...where };
    if (text.startsWith('=')) return { op: 'set_formula', formula: text, ...where };
    return { op: 'set_cell', value: number.test(text) ? Number(text) : text, ...where };
  }

  async function put(ref, text) {
    const key = keyOf(ref);
    if (text === raw(ref)) {
      if (refused?.key === key) revert();
      return;
    }
    pending[key] = text;
    try {
      await editFile(id, [write(ref, text)]);
      if (refused?.key === key) refused = null;
      delete pending[key];
    } catch (e) {
      refused = { key, text: e.message };
      at = ref;
    }
  }

  function begin(text) {
    if (synced) return;
    draft = text;
  }

  function finish(dc = 0, dr = 0) {
    if (draft === null) return;
    const text = draft;
    const ref = at;
    draft = null;
    put(ref, text);
    if (dc || dr) move(dc, dr);
    scroller?.focus();
  }

  function gridKey(event) {
    if (event.target !== scroller) return;
    const moves = { ArrowUp: [0, -1], ArrowDown: [0, 1], ArrowLeft: [-1, 0], ArrowRight: [1, 0], Tab: [event.shiftKey ? -1 : 1, 0] };
    if (moves[event.key]) {
      event.preventDefault();
      move(...moves[event.key]);
    } else if (!at) {
      return;
    } else if (event.key === 'Enter' || event.key === 'F2') {
      event.preventDefault();
      begin(pending[keyOf(at)] ?? raw(at));
    } else if (event.key === 'Backspace' || event.key === 'Delete') {
      event.preventDefault();
      if (!synced) put(at, '');
    } else if (event.key === 'Escape') {
      event.preventDefault();
      if (refused) revert();
      else at = '';
    } else if (event.key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey) {
      event.preventDefault();
      begin(event.key);
    }
  }

  function inputKey(event) {
    if (event.key === 'Enter') {
      event.preventDefault();
      finish(0, 1);
    } else if (event.key === 'Tab') {
      event.preventDefault();
      finish(event.shiftKey ? -1 : 1, 0);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      draft = null;
      scroller?.focus();
    }
  }

  function clicked(event) {
    const ref = event.target.closest('[data-ref]')?.dataset.ref;
    if (ref && ref !== at) select(ref);
  }

  function pressedTwice(event) {
    const ref = event.target.closest('[data-ref]')?.dataset.ref;
    if (ref) begin(pending[keyOf(ref)] ?? raw(ref));
  }

  // The grid's own size, for which rows are drawn.
  const measure = (el) => {
    const seen = new ResizeObserver(() => {
      tall = el.clientHeight;
      wide = el.clientWidth;
    });
    seen.observe(el);
    return () => seen.disconnect();
  };
  const focusEnd = (el) => {
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  };
</script>

<svelte:window {onkeydown} />

{#snippet field(key, value, ops, kind = '', draw = null, attrs = {})}
  <div
    {...attrs}
    class={['field', kind, { refused: refused?.key === key, plain: synced }]}
    role="textbox"
    tabindex={synced ? undefined : 0}
    aria-multiline="true"
    aria-readonly={synced}
    contenteditable={synced ? undefined : 'plaintext-only'}
    {@attach (el) => {
      const text = value;
      if (key in held || refused?.key === key) return;
      if (draw) draw(el);
      else el.textContent = text;
    }}
    onfocus={(e) => (held[key] ??= typed(e.currentTarget))}
    onblur={(e) => key in held && leave(e.currentTarget, key, ops)}
    onkeydown={(e) => press(e, key)}
  ></div>
  {#if refused?.key === key}<p class="error" role="alert">{refused.text}</p>{/if}
{/snippet}

{#snippet paragraph(p, only = false)}
  {#each p.images ?? [] as im (im.part)}
    <img class="figure" alt="" style:width={im.box.w ? `${im.box.w / 9525}px` : undefined} {@attach media(im.part)} />
  {/each}
  {#if p.text || !p.images?.length}
    {@render field(p.id, p.text, paragraphOps(p, only), [look(p), p.marker ? 'item' : ''].join(' ').trim(), inlineOf(p), { 'data-marker': p.marker || undefined, style: `--level:${p.level ?? 0};${p.align ? `text-align:${p.align === 'both' ? 'justify' : p.align};` : ''}` })}
  {/if}
{/snippet}

<aside class="canvas" aria-label={file?.name ?? 'Canvas'} bind:this={aside} style:width={width ? `${width}px` : undefined}>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div class="handle" role="separator" aria-orientation="vertical" aria-label="Resize" aria-valuenow={width} tabindex="0" onpointerdown={drag} onkeydown={nudge}></div>
  <TopBar bordered>
    {#snippet leading()}
      <span class="name" title={file?.name}>{file?.name ?? app.canvas?.name ?? ''}</span>
      {#if synced}<Badge>Read only</Badge>{/if}
    {/snippet}
    {#snippet trailing()}
      {#if synced && file.held}<Button size="sm" icon={Copy} onclick={() => copyFile(file.id)}>Copy</Button>{/if}
      {#if file}
        {#if !file.connection}<IconButton icon={Pencil} label="Rename" placement="bottom" onclick={() => renameFilePrompt(file)} />{/if}
        <IconButton icon={Share} label="Share" placement="bottom" onclick={() => openShare('file', file.id)} />
        {#if file.held}<IconButton icon={Download} label="Download" placement="bottom" onclick={() => downloadFile(file)} />{/if}
        {#if !file.connection}<IconButton icon={Trash2} label="Delete" placement="bottom" onclick={remove} />{/if}
      {/if}
      <span class="close" bind:this={closer}><IconButton icon={X} label="Close" tooltip={false} onclick={closeFile} /></span>
    {/snippet}
  </TopBar>

  {#if !file && app.canvas.error}
    <div class="body center"><EmptyState icon={FileQuestion} title={app.canvas.error} /></div>
  {:else if !file}
    <div class="pad"><Skeleton lines={4} /></div>
  {:else if model?.type === 'xlsx' && sheet}
    {#if refused}<p class="error sheet" role="alert">{refused.text}</p>{/if}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions, a11y_click_events_have_key_events -->
    <div class="grid" role="grid" tabindex="0" aria-label={sheet.name} bind:this={scroller} {@attach measure} onscroll={() => (top = scroller.scrollTop)} onkeydown={gridKey} onclick={clicked} ondblclick={pressedTwice}>
      <table>
        <colgroup>
          <col style:width="2.5rem" />
          {#each columns as c (c)}{#if widthOf(c)}<col style:width={`${widthOf(c)}px`} />{/if}{/each}
        </colgroup>
        <thead>
          <tr>
            <th class="corner"></th>
            {#each columns as c (c)}{#if widthOf(c)}<th class={['col', { on: here.col === c, spare: c >= grid.used.cols }]}>{columnName(c)}</th>{/if}{/each}
          </tr>
        </thead>
        <tbody>
          {#if first}<tr style:height={`${first * ROW}px`}><td class="gap"></td></tr>{/if}
          {#each seen as r (r)}
            <tr>
              <th class={['row', { on: here.row === r, spare: r >= grid.used.rows }]}>{r + 1}</th>
              {#each columns as c (c)}
                {@const ref = refOf(c, r)}
                {#if widthOf(c) && !grid.hidden.has(ref)}
                  {@const span = grid.spans.get(ref)}
                  {@const cell = grid.cells.get(ref)}
                  {@const cl = cellLook(cell)}
                  <td
                    role="gridcell"
                    data-ref={ref}
                    colspan={span?.cols}
                    rowspan={span?.rows}
                    style={cl.css}
                    class={['cell', { on: at === ref, end: number.test(cell?.value ?? face(ref)), refused: refused?.key === keyOf(ref), spare: r >= grid.used.rows || c >= grid.used.cols, spill: spills(ref, cell, c, r), painted: cl.painted, inked: cl.inked }]}
                    aria-selected={at === ref}
                  >
                    {#if draft !== null && at === ref}
                      <input class="input" aria-label={ref} bind:value={draft} onkeydown={inputKey} onblur={() => finish()} {@attach focusEnd} />
                    {:else}
                      {face(ref)}
                    {/if}
                  </td>
                {/if}
              {/each}
            </tr>
          {/each}
          {#if last < grid.rows}<tr style:height={`${(grid.rows - last) * ROW}px`}><td class="gap"></td></tr>{/if}
        </tbody>
      </table>
    </div>
    {#if tabs.length > 1 || at}
      <div class="foot">
        {#if tabs.length > 1}<Tabs variant="pill" items={tabs} value={sheet.name} label="Sheets" onchange={(v) => { sheetName = v; at = ''; top = 0; if (scroller) scroller.scrollTop = 0; }} />{/if}
        {#if at}
          <div class="current">
            <span class="ref">{at}</span>
            <span class="raw" title={raw(at)}>{draft ?? pending[keyOf(at)] ?? raw(at)}</span>
          </div>
        {/if}
      </div>
    {/if}
  {:else if model?.type === 'docx'}
    <div class="body">
      <article class="page">
        {#if model.header}<p class="margin">{model.header}</p>{/if}
        {#each blocks as b (b.key)}
          {#if b.p}
            {@render paragraph(b.p)}
          {:else}
            {@const grid = tableAt(b.key)?.grid}
            <table class="table">
              <tbody>
                {#each b.rows as row, r (r)}
                  <tr>
                    {#each row ?? [] as cell, c (c)}
                      {@const g = grid?.[r]?.[c]}
                      {#if !g?.skip}
                        <td colspan={g?.span > 1 ? g.span : undefined} rowspan={g?.rows > 1 ? g.rows : undefined} class={{ painted: g?.fill && g.fill !== '#FFFFFF' }} style:--fill={g?.fill}>
                          {#each cell ?? [] as p (p.id)}{@render paragraph(p, cell.length === 1)}{/each}
                        </td>
                      {/if}
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        {/each}
        {#if model.footer}<p class="margin foot-note">{model.footer}</p>{/if}
      </article>
    </div>
  {:else if model?.type === 'pptx'}
    <div class="body">
      <div class="deck">
        {#each model.slides as s (s.slide)}
          <figure class={['frame', { off: s.hidden }]}>
            <figcaption class="number">{s.slide}</figcaption>
            <section class="slide" aria-label={`Slide ${s.slide}`} style:aspect-ratio={`${model.width} / ${model.height}`} style:background={s.background ?? '#FFFFFF'}>
              {#if s.backdrop}<img class="backdrop" alt="" {@attach media(s.backdrop)} />{/if}
              {#each s.pictures ?? [] as im, i (i)}
                <img class="picture" alt="" style={`${boxOf(im.box)}z-index:${im.z ?? 0};`} {@attach media(im.part)} />
              {/each}
              {#each s.decor ?? [] as d, i (i)}
                <div class="shape" style={shapeLook(d)}>
                  {#if d.paragraphs?.length}<div class="field plain lines" style:padding={insetOf(d)} {@attach (el) => drawShape(el, d)}></div>{/if}
                </div>
              {/each}
              {#each s.shapes as shape (shape.shape)}
                <div class="shape" style={shapeLook(shape)}>
                  {@render field(`s${s.slide}/${shape.shape}`, shape.paragraphs.map((p) => p.text).join('\n'), (_, next) => [{ op: 'replace_shape_text', slide: s.slide, shape: shape.shape, text: next }], 'lines', (el) => drawShape(el, shape), { style: `padding:${insetOf(shape)}` })}
                </div>
              {/each}
            </section>
          </figure>
        {/each}
      </div>
    </div>
  {:else if app.canvas.error || !shown}
    <div class="body center">
      <EmptyState icon={FileQuestion} title={file.name}>
        {#if app.canvas.error}<p class="error">{app.canvas.error}</p>{:else if file.error || file.sub}<p class="note">{file.error || file.sub}</p>{/if}
        {#if file.held}<Button icon={Download} onclick={() => downloadFile(file)}>Download</Button>{/if}
      </EmptyState>
    </div>
  {:else if shown === 'text'}
    <div class="body"><article class="page"><pre class="text">{text}</pre></article></div>
  {:else if shown === 'image'}
    <div class="body center">{#if src}<img {src} alt={file.name} />{/if}</div>
  {:else if shown === 'pdf'}
    {#if src}<iframe src={`${src}#toolbar=0&view=FitH`} title={file.name}></iframe>{/if}
  {:else if shown === 'audio'}
    <div class="body center">{#if src}<audio {src} controls></audio>{/if}</div>
  {:else if shown === 'video'}
    <div class="body center">{#if src}<!-- svelte-ignore a11y_media_has_caption --><video {src} controls></video>{/if}</div>
  {/if}
</aside>

<style>
  /* Text starts where the title of a TopBar does: the bar's padding and a ghost button's. */
  .canvas {
    --edge: calc(var(--space-2) + var(--space-3));
    --color-canvas: var(--color-surface);
    position: relative;
    display: flex;
    flex-direction: column;
    flex: none;
    width: var(--canvas-width);
    min-width: var(--canvas-min);
    max-width: calc(100% - var(--archive-rail) - var(--sidebar-width) - var(--canvas-min));
    min-height: 0;
    background: var(--color-canvas);
    box-shadow: inset var(--border-width) 0 0 var(--color-border);
  }

  .handle {
    position: absolute;
    inset-block: 0;
    inset-inline-start: calc(var(--space-1) * -1);
    z-index: var(--z-sticky);
    width: var(--space-2);
    outline: none;
    cursor: col-resize;
    touch-action: none;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .handle:hover,
  .handle:focus-visible {
    background: var(--color-border);
  }

  .name {
    padding-inline: var(--space-3);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .close {
    display: inline-flex;
  }

  .pad {
    padding: var(--space-4) var(--edge);
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .center {
    display: grid;
    place-items: center;
    padding: var(--edge);
  }

  img,
  video {
    max-width: 100%;
    max-height: 100%;
    border-radius: var(--radius-md);
  }

  iframe {
    flex: 1;
    width: 100%;
    min-height: 0;
    border: 0;
  }

  .text {
    margin: 0;
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    tab-size: 4;
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
    overflow-wrap: anywhere;
  }

  .note {
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
  }

  /* Text a person edits. A field's own padding reaches out of the page's edge, so its words stay on it. */

  .page {
    --field-edge: var(--space-1-5);
    display: flex;
    flex-direction: column;
    padding: var(--space-4) var(--edge) var(--space-12);
    font-family: var(--font-prose);
    font-size: var(--text-prose);
    line-height: var(--leading-prose);
    letter-spacing: var(--tracking-normal);
  }

  .margin {
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    white-space: pre-wrap;
    margin-block-end: var(--space-6);
  }

  .foot-note {
    margin-block: auto 0;
    padding-block-start: var(--space-6);
  }

  .field {
    position: relative;
    min-height: calc(1em * var(--leading-prose) + var(--space-1));
    margin-block-end: var(--space-1);
    margin-inline: calc(var(--field-edge) * -1);
    padding: var(--space-0-5) var(--field-edge);
    border-radius: var(--radius-xs);
    outline: none;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .field:not(.plain):hover {
    background: var(--color-fill-subtle);
  }

  .field:not(.plain):focus {
    background: var(--color-accent-subtle);
  }

  .field.refused,
  .field.refused:focus {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-danger);
  }

  .title {
    margin-block-end: var(--space-2);
    font-size: var(--text-2xl);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-tight);
    line-height: var(--leading-tight);
  }

  .subtitle {
    margin-block-end: var(--space-4);
    color: var(--color-text-tertiary);
    font-size: var(--text-lg);
    line-height: var(--leading-snug);
  }

  .heading1,
  .heading2,
  .heading3,
  .heading4,
  .heading5,
  .heading6 {
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-snug);
    line-height: var(--leading-snug);
  }

  .heading1 { margin-block: var(--space-6) var(--space-1-5); font-size: var(--text-xl); letter-spacing: var(--tracking-tight); }
  .heading2 { margin-block: var(--space-5) var(--space-1); font-size: var(--text-lg); }
  .heading3 { margin-block: var(--space-4) var(--space-0-5); font-size: var(--text-base); }
  .heading4,
  .heading5,
  .heading6 { margin-block: var(--space-3) var(--space-0-5); }

  .quote {
    margin-block: var(--space-2);
    border-radius: 0;
    box-shadow: inset var(--focus-width) 0 0 var(--color-border-strong);
    color: var(--color-text-secondary);
    padding-inline-start: var(--space-4);
  }

  /* A list item's number or bullet is the file's; it hangs left of the text and is not part of what is edited. */
  .item {
    margin-block-end: 0;
    padding-inline-start: calc(var(--field-edge) + (var(--level, 0) + 1) * 1.5em);
  }

  .item[data-marker]::before,
  :global(.canvas .line[data-marker])::before {
    content: attr(data-marker);
    position: absolute;
    font-variant-numeric: tabular-nums;
  }

  .item[data-marker]::before {
    inset-inline-start: calc(var(--field-edge) + var(--level, 0) * 1.5em);
    color: var(--color-text-tertiary);
  }

  .figure {
    display: block;
    max-width: 100%;
    margin-block: var(--space-2);
    border-radius: var(--radius-sm);
  }

  .table {
    width: 100%;
    margin-block: var(--space-3);
    border-collapse: separate;
    border-spacing: 0;
    border-radius: var(--radius-md);
    box-shadow: var(--ring);
    font-size: var(--text-sm);
    line-height: var(--leading-snug);
  }

  .table td {
    padding: var(--space-2) var(--space-3);
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle), inset var(--border-width) 0 0 var(--color-border-subtle);
    vertical-align: top;
  }

  .table tr:first-child td {
    box-shadow: inset var(--border-width) 0 0 var(--color-border-subtle);
  }

  .table td:first-child {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .table tr:first-child td:first-child {
    box-shadow: none;
  }

  .table .field {
    --field-edge: var(--space-1);
    min-height: 0;
    margin-block-end: 0;
  }

  .painted {
    background: var(--fill);
  }

  :global([data-theme='dark']) .painted {
    background: color-mix(in oklab, var(--fill) 25%, var(--color-surface));
  }

  .page :global(.inked),
  .inked {
    color: var(--ink);
  }

  :global([data-theme='dark']) .page :global(.inked),
  :global([data-theme='dark']) .inked {
    color: color-mix(in oklab, var(--ink) 45%, var(--color-text));
  }

  .deck {
    display: flex;
    flex-direction: column;
  }

  .frame {
    display: flex;
    flex-direction: column;
    margin: 0;
  }

  .off {
    opacity: var(--opacity-disabled);
  }

  .number {
    padding: var(--space-1-5) var(--edge);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  /* Everything on a slide is in the slide's own units, so it scales as one; its colors are the file's in both themes. */
  .slide {
    position: relative;
    container-type: inline-size;
    width: 100%;
    overflow: hidden;
    color: #000000;
    line-height: var(--leading-tight);
  }

  .backdrop {
    position: absolute;
    inset: 0;
    width: 100%;
    max-width: none;
    height: 100%;
    max-height: none;
    border-radius: 0;
    object-fit: cover;
  }

  .picture {
    position: absolute;
    max-width: none;
    max-height: none;
    border-radius: 0;
    object-fit: fill;
  }

  .shape {
    position: absolute;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .shape .field {
    width: 100%;
    min-height: 0;
    margin: 0;
    border-radius: 0;
    line-height: inherit;
  }

  .shape .field:empty {
    min-height: 1lh;
  }

  .shape .field:not(.plain):hover {
    background: none;
    box-shadow: inset 0 0 0 var(--border-width) var(--color-focus);
  }

  .shape .field:not(.plain):focus {
    background: var(--color-accent-subtle);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-focus);
  }

  .shape :global(.line) {
    position: relative;
    min-height: 1lh;
  }

  .shape :global(.line[data-marker]) {
    padding-inline-start: calc((var(--level, 0) + 1) * 1.1em);
  }

  :global(.canvas .line[data-marker])::before {
    inset-inline-start: calc(var(--level, 0) * 1.1em);
    color: var(--ink);
  }

  /* A sheet. */

  .ref {
    flex: none;
    color: var(--color-text-tertiary);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
  }

  .raw {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .error.sheet {
    flex: none;
    margin: var(--space-3) var(--edge) 0;
  }

  .grid {
    flex: 1;
    min-height: 0;
    overflow: auto;
    overscroll-behavior: contain;
    outline: none;
    scroll-padding: 1.75rem 0 0 2.5rem;
    font-size: var(--text-sm);
  }

  .grid table {
    table-layout: fixed;
    width: max-content;
    border-collapse: separate;
    border-spacing: 0;
  }

  .grid th,
  .grid td {
    height: 1.75rem;
  }

  .grid th {
    position: sticky;
    z-index: var(--z-raised);
    padding: 0;
    background: var(--color-surface);
    color: var(--color-text-tertiary);
    font-size: var(--text-2xs);
    font-weight: var(--weight-medium);
    font-variant-numeric: tabular-nums;
    user-select: none;
  }

  .grid th.spare {
    color: var(--color-text-disabled);
  }

  .grid th.on {
    color: var(--color-accent);
  }

  .col {
    inset-block-start: 0;
    box-shadow: inset 0 calc(var(--border-width) * -1) 0 var(--color-border);
  }

  .row {
    inset-inline-start: 0;
    box-shadow: inset calc(var(--border-width) * -1) 0 0 var(--color-border);
  }

  .row.spare {
    box-shadow: none;
  }

  .corner {
    inset-block-start: 0;
    inset-inline-start: 0;
    z-index: var(--z-sticky);
    box-shadow: inset calc(var(--border-width) * -1) calc(var(--border-width) * -1) 0 var(--color-border);
  }

  .cell {
    position: relative;
    padding-inline: var(--space-1-5);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: cell;
    box-shadow: inset calc(var(--border-width) * -1) calc(var(--border-width) * -1) 0 var(--color-border-subtle);
  }

  .cell.spare {
    box-shadow: none;
  }

  .cell.spill {
    overflow: visible;
    text-overflow: clip;
  }

  .end {
    text-align: end;
    font-variant-numeric: tabular-nums;
  }

  .gap {
    height: auto;
    padding: 0;
    box-shadow: none !important;
  }

  .grid:focus .cell.on,
  .cell.on {
    border-radius: var(--radius-xs);
    background: var(--color-accent-subtle);
    box-shadow: inset 0 0 0 var(--focus-width) var(--color-accent);
    overflow: hidden;
  }

  .cell.refused {
    box-shadow: inset 0 0 0 calc(var(--border-width) * 2) var(--color-danger);
  }

  .input {
    position: absolute;
    inset: 0;
    width: 100%;
    padding-inline: var(--space-1-5);
    border: 0;
    outline: none;
    background: var(--color-surface);
    border-radius: var(--radius-xs);
    box-shadow: inset 0 0 0 var(--focus-width) var(--color-accent);
    font: inherit;
    text-align: start;
    color: var(--color-text);
  }

  .foot {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    flex: none;
    min-height: var(--control-md);
    padding: var(--space-1) var(--edge);
    box-shadow: inset 0 var(--border-width) 0 var(--color-border);
    font-size: var(--text-sm);
  }

  .current {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    margin-inline-start: auto;
  }

  @media (max-width: 56rem) {
    .canvas {
      position: fixed;
      inset: 0;
      z-index: var(--z-overlay);
      width: auto !important;
      min-width: 0;
      max-width: none;
      box-shadow: none;
    }

    .handle {
      display: none;
    }
  }
</style>
