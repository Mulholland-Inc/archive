import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type ClipboardEvent, type KeyboardEvent, type PointerEvent as ReactPointerEvent, type ReactNode, type RefObject } from 'react';
import { Button, Icon, Lines, Pic } from './ui';
import type { Issue, TField } from './model';

export type SheetRow = { id: string; sku: string; values: Record<string, string>; issues: Issue[]; defaults?: string[]; locked?: string[] };
export type SheetChange = { row: string; key: string; value: string };
type Props = { fields: TField[]; rows: SheetRow[]; nameKey?: string; skuKey?: string; editable?: boolean; missingOnly?: boolean; pending?: Set<string>; onChange?: (changes: SheetChange[]) => void };

const ROW = 30;
const MULTI = '; ';
const SKU = '__sku';
const NAME = '__name';
const widthOf = (f: TField) => f.type === 'yesno' ? 92 : f.type === 'number' ? 108 : f.type === 'dimensions' ? 164 : f.type === 'select' ? 156 : f.label.length > 22 ? 216 : 164;
const partsOf = (v: string) => v.split(/\s*;\s*/).filter(Boolean);
const optionsOf = (f: TField) => (f.type === 'yesno' ? ['Yes', 'No'] : f.options ?? []);
const typeWord = (f: TField) => f.type === 'select' ? (f.multi ? 'Any of a list' : 'One of a list') : f.type === 'yesno' ? 'Yes or no' : f.type === 'dimensions' ? (f.parts ?? ['W', 'D', 'H']).join(' × ') : f.type === 'number' ? 'A number' : f.type === 'country' ? 'A country' : 'Text';
const asked = (f: TField) => (f.owner === 'lg' ? 'Filled in by the buyer' : f.required ? 'Required' : f.requiredWhen?.length ? 'Required for some products' : 'Optional');
const fieldIssue = (i: Issue) => !i.field.startsWith('image:') && !i.field.startsWith('photo:');
const owes = (r: SheetRow) => r.issues.some(i => i.level === 'error' && fieldIssue(i));
export const cellId = (row: string, key: string) => `${row}\u0000${key}`;
type At = { row: number; col: number };

export function Sheet({ fields, rows, nameKey, skuKey, editable, missingOnly, pending, onChange }: Props) {
  const byKey = useMemo(() => new Map(fields.map(f => [f.key, f])), [fields]);
  const shownRows = useMemo(() => (missingOnly ? rows.filter(owes) : rows), [rows, missingOnly]);
  const shownFields = useMemo(() => {
    const rest = fields.filter(f => f.key !== nameKey && f.key !== skuKey);
    if (!missingOnly) return rest;
    const hot = new Set(shownRows.flatMap(r => r.issues.filter(i => i.level === 'error' && fieldIssue(i)).map(i => i.field)));
    return rest.filter(f => hot.has(f.key));
  }, [fields, nameKey, skuKey, missingOnly, shownRows]);
  const cols = useMemo(() => [{ id: SKU, base: 128 }, { id: NAME, base: 208 }, ...shownFields.map(f => ({ id: f.key, base: widthOf(f) }))], [shownFields]);
  const [resized, setResized] = useState<Record<string, number>>({});
  const widths = cols.map(c => resized[c.id] ?? c.base);
  const lefts = widths.reduce<number[]>((all, w, i) => [...all, (all[i] ?? 0) + w], [0]);
  const pinnedWidth = widths[0] + widths[1];
  const total = lefts[lefts.length - 1];

  const scroll = useRef<HTMLDivElement>(null);
  const [view, setView] = useState({ top: 0, left: 0, w: 0, h: 0 });
  const measure = () => { const el = scroll.current; if (el) setView({ top: el.scrollTop, left: el.scrollLeft, w: el.clientWidth, h: el.clientHeight }); };
  useLayoutEffect(() => {
    measure();
    const seen = new ResizeObserver(measure);
    seen.observe(scroll.current!);
    return () => seen.disconnect();
  }, []);
  const head = useRef<HTMLDivElement>(null);
  const headH = head.current?.offsetHeight ?? ROW * 2;
  const firstRow = Math.max(0, Math.floor(view.top / ROW) - 10);
  const lastRow = Math.min(shownRows.length, Math.ceil((view.top + view.h) / ROW) + 10);
  let firstCol = 2;
  while (firstCol < cols.length - 1 && lefts[firstCol + 1] - pinnedWidth < view.left) firstCol++;
  firstCol = Math.max(2, firstCol - 3);
  let lastCol = firstCol;
  while (lastCol < cols.length && lefts[lastCol] - pinnedWidth < view.left + view.w) lastCol++;
  lastCol = Math.min(cols.length, lastCol + 3);
  const centre = cols.map((c, i) => ({ ...c, i })).slice(firstCol, lastCol);
  const before = (lefts[firstCol] ?? pinnedWidth) - pinnedWidth;
  const after = total - (lefts[lastCol] ?? total);
  const runs: { name: string; width: number }[] = [];
  for (const c of centre) {
    const name = byKey.get(c.id)?.section ?? '';
    const last = runs[runs.length - 1];
    if (last && last.name === name) last.width += widths[c.i]; else runs.push({ name, width: widths[c.i] });
  }

  const [sel, setSel] = useState<{ a: At; b: At } | null>(null);
  const [editing, setEditing] = useState<{ row: string; col: string; seed?: string; all?: boolean } | null>(null);
  const keyOf = (id: string) => (id === NAME ? nameKey ?? '' : id === SKU ? skuKey ?? '' : id);
  const fieldOf = (id: string) => byKey.get(keyOf(id));
  const canEdit = (id: string, r?: SheetRow) => !!editable && (id !== SKU || !!skuKey) && fieldOf(id)?.owner === 'vendor' && !r?.locked?.includes(keyOf(id));
  const textOf = (r: SheetRow, id: string) => (id === SKU && !skuKey ? r.sku : r.values[keyOf(id)] ?? '');
  const write = (changes: SheetChange[]) => { if (changes.length) onChange?.(changes); };
  const clamp = (at: At): At => ({ row: Math.max(0, Math.min(shownRows.length - 1, at.row)), col: Math.max(0, Math.min(cols.length - 1, at.col)) });
  const bring = (at: At) => {
    const el = scroll.current;
    if (!el) return;
    const top = at.row * ROW;
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + ROW + headH > el.scrollTop + el.clientHeight) el.scrollTop = top + ROW + headH - el.clientHeight;
    if (at.col >= 2) {
      const left = lefts[at.col], right = left + widths[at.col];
      if (left < el.scrollLeft + pinnedWidth) el.scrollLeft = left - pinnedWidth;
      else if (right > el.scrollLeft + el.clientWidth) el.scrollLeft = right - el.clientWidth;
    }
  };
  const step = (dir: 'up' | 'down' | 'left' | 'right', extend?: boolean) => setSel(s => {
    if (!s) return s;
    const from = extend ? s.b : s.a;
    const to = clamp({ row: from.row + (dir === 'down' ? 1 : dir === 'up' ? -1 : 0), col: from.col + (dir === 'right' ? 1 : dir === 'left' ? -1 : 0) });
    bring(to);
    return extend ? { a: s.a, b: to } : { a: to, b: to };
  });
  const open = (at: At, seed?: string, all?: boolean) => {
    const r = shownRows[at.row], id = cols[at.col].id;
    if (!r || !canEdit(id, r)) return;
    setSel({ a: at, b: at });
    setEditing({ row: r.id, col: id, seed, all });
  };
  const close = (then?: 'down' | 'right' | 'left') => {
    setEditing(null);
    requestAnimationFrame(() => { scroll.current?.focus({ preventScroll: true }); if (then) step(then); });
  };
  const inRange = (each: (r: SheetRow, c: { id: string }) => void) => {
    if (!sel) return;
    for (const r of shownRows.slice(Math.min(sel.a.row, sel.b.row), Math.max(sel.a.row, sel.b.row) + 1))
      for (const c of cols.slice(Math.min(sel.a.col, sel.b.col), Math.max(sel.a.col, sel.b.col) + 1)) each(r, c);
  };
  const copy = () => {
    if (!sel) return;
    const text = shownRows.slice(Math.min(sel.a.row, sel.b.row), Math.max(sel.a.row, sel.b.row) + 1).map(r => cols.slice(Math.min(sel.a.col, sel.b.col), Math.max(sel.a.col, sel.b.col) + 1)
      .map(c => { const t = textOf(r, c.id); return /[\t\n"]/.test(t) ? `"${t.replace(/"/g, '""')}"` : t; }).join('\t')).join('\n');
    void navigator.clipboard.writeText(text);
  };
  const paste = (e: ClipboardEvent<HTMLDivElement>) => {
    if (!editable || editing || !sel) return;
    e.preventDefault();
    const lines = e.clipboardData.getData('text/plain').replace(/\r/g, '').split('\n');
    if (lines.length > 1 && lines[lines.length - 1] === '') lines.pop();
    const changes: SheetChange[] = [];
    lines.forEach((line, dy) => {
      const r = shownRows[sel.a.row + dy];
      if (r) line.split('\t').forEach((v, dx) => {
        const c = cols[sel.a.col + dx];
        if (c && canEdit(c.id, r)) changes.push({ row: r.id, key: keyOf(c.id), value: v.trim() });
      });
    });
    write(changes);
  };
  const keys = (e: KeyboardEvent<HTMLDivElement>) => {
    if (editing) return;
    const mod = e.metaKey || e.ctrlKey;
    const dir = ({ ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' } as const)[e.key as 'ArrowUp'];
    if (dir) { e.preventDefault(); return step(dir, e.shiftKey); }
    if (e.key === 'Tab') { e.preventDefault(); return step(e.shiftKey ? 'left' : 'right'); }
    if (mod && e.key.toLowerCase() === 'a') { e.preventDefault(); return setSel({ a: { row: 0, col: 0 }, b: { row: shownRows.length - 1, col: cols.length - 1 } }); }
    if (mod && e.key.toLowerCase() === 'c') { e.preventDefault(); return copy(); }
    if (e.key === 'Escape') return setSel(null);
    if (!sel) return;
    if (e.key === 'Enter' || e.key === 'F2') { e.preventDefault(); return open(sel.a); }
    if ((e.key === 'Backspace' || e.key === 'Delete') && editable) {
      e.preventDefault();
      const changes: SheetChange[] = [];
      inRange((r, c) => { if (canEdit(c.id, r) && textOf(r, c.id)) changes.push({ row: r.id, key: keyOf(c.id), value: '' }); });
      return write(changes);
    }
    if (e.key.length === 1 && !mod && canEdit(cols[sel.a.col].id, shownRows[sel.a.row])) { e.preventDefault(); open(sel.a, e.key); }
  };
  const dragging = useRef(false);
  const press = (at: At) => (e: ReactPointerEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest('.sheet__lift')) return;
    setEditing(null);
    setSel(s => (e.shiftKey && s ? { a: s.a, b: at } : { a: at, b: at }));
    dragging.current = true;
    const stop = () => { dragging.current = false; removeEventListener('pointerup', stop); };
    addEventListener('pointerup', stop);
    e.preventDefault();
    scroll.current?.focus({ preventScroll: true });
  };
  const drag = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!dragging.current) return;
    const el = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-row][data-col]');
    if (el) setSel(s => (s ? { a: s.a, b: { row: Number(el.dataset.row), col: Number(el.dataset.col) } } : s));
  };
  const resizer = (id: string, i: number) => (
    <span className="sheet__resize" onDoubleClick={() => setResized(({ [id]: _, ...rest }) => rest)} onPointerDown={e => {
      e.stopPropagation();
      const x0 = e.clientX, w0 = widths[i];
      const move = (ev: PointerEvent) => setResized(was => ({ ...was, [id]: Math.max(72, w0 + ev.clientX - x0) }));
      const up = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', up); };
      addEventListener('pointermove', move);
      addEventListener('pointerup', up);
    }} />
  );
  const pinStyle = (i: number): CSSProperties => ({ width: widths[i], insetInlineStart: lefts[i] });
  const here = sel && shownRows[sel.a.row];
  const hereKey = sel ? keyOf(cols[sel.a.col].id) : '';
  const hereIssue = here?.issues.find(i => i.field === hereKey && fieldIssue(i));
  const hereDefault = here && !hereIssue && here.defaults?.includes(hereKey);

  const cell = (r: SheetRow, rowAt: number, id: string, colAt: number, style: CSSProperties) => {
    const k = keyOf(id);
    const f = byKey.get(k);
    const issue = f && r.issues.find(i => i.field === k);
    const v = textOf(r, id);
    const wrong = issue?.level === 'error' && !!v.trim();
    const known = !f || !optionsOf(f).length || partsOf(v).every(p => optionsOf(f).some(o => o.toLowerCase() === p.toLowerCase()));
    const inside = sel && rowAt >= Math.min(sel.a.row, sel.b.row) && rowAt <= Math.max(sel.a.row, sel.b.row) && colAt >= Math.min(sel.a.col, sel.b.col) && colAt <= Math.max(sel.a.col, sel.b.col);
    const caret = sel && sel.a.row === rowAt && sel.a.col === colAt;
    const edges = inside ? [
      rowAt === Math.min(sel.a.row, sel.b.row) && 'inset 0 var(--pick-width) 0 var(--ring)', rowAt === Math.max(sel.a.row, sel.b.row) && 'inset 0 calc(-1 * var(--pick-width)) 0 var(--ring)',
      colAt === Math.min(sel.a.col, sel.b.col) && 'inset var(--pick-width) 0 0 var(--ring)', colAt === Math.max(sel.a.col, sel.b.col) && 'inset calc(-1 * var(--pick-width)) 0 0 var(--ring)'
    ].filter(Boolean) as string[] : [];
    if (caret) edges.unshift('inset 0 0 0 var(--stroke-strong) var(--ring)');
    if (id === SKU) edges.push(`inset var(--bar-width) 0 0 ${owes(r) ? 'var(--destructive)' : 'var(--success)'}`);
    const cls = ['sheet__cell', !canEdit(id, r) && id !== SKU && 'sheet__cell--fixed', issue && `sheet__cell--${issue.level === 'error' ? 'missing' : 'warn'}`, wrong && 'sheet__cell--wrong',
      f && r.defaults?.includes(k) && !issue && 'sheet__cell--default', pending?.has(cellId(r.id, k)) && 'sheet__cell--pending', inside && 'is-picked', colAt < 2 && 'sheet__cell--pin'].filter(Boolean).join(' ');
    return (
      <div key={id} className={cls} style={{ ...style, boxShadow: edges.join(', ') || undefined }} data-row={rowAt} data-col={colAt} onPointerDown={press({ row: rowAt, col: colAt })} onDoubleClick={() => open({ row: rowAt, col: colAt }, undefined, true)}>
        <span className="sheet__text" title={issue?.message ?? (v.length > 24 ? v : undefined)}>
          {v && issue?.level === 'error' && <Icon name="alert" size="xs" />}
          {v || (issue?.level === 'error' ? <span className="sheet__owed">{f?.type === 'select' || f?.type === 'yesno' ? 'Pick one' : 'Missing'}</span> : '')}
          {v && !known && issue?.level !== 'error' && <Icon name="alert" size="xs" />}
        </span>
      </div>
    );
  };

  return (
    <>
      <div className="sheet" ref={scroll} tabIndex={0} onKeyDown={keys} onPaste={paste} onPointerMove={drag} onScroll={measure}>
        <div style={{ width: total, position: 'relative', minWidth: '100%' }}>
          <div className="sheet__head" ref={head}>
            {runs.some(r => r.name) && (
              <div className="sheet__row">
                {[0, 1].map(i => <div key={i} className="sheet__cell sheet__cell--pin" style={pinStyle(i)} />)}
                {before > 0 && <div style={{ width: before, flex: 'none' }} />}
                {runs.map((r, i) => <div key={i} className="sheet__cell" style={{ width: r.width }}><span className="sheet__text">{r.name}</span></div>)}
                {after > 0 && <div style={{ width: after, flex: 'none' }} />}
              </div>
            )}
            <div className="sheet__row">
              {[SKU, NAME].map((id, i) => <div key={id} className="sheet__cell sheet__cell--pin" style={pinStyle(i)}><span className="sheet__text">{id === SKU ? 'SKU' : 'Product'}</span>{resizer(id, i)}</div>)}
              {before > 0 && <div style={{ width: before, flex: 'none' }} />}
              {centre.map(c => {
                const f = byKey.get(c.id)!;
                return <div key={c.id} className="sheet__cell" style={{ width: widths[c.i] }} title={[f.label, typeWord(f), asked(f), f.help ?? f.note].filter(Boolean).join('\n')}><span className="sheet__text">{f.label}{f.required && f.owner === 'vendor' && <span className="sheet__req">*</span>}</span>{resizer(c.id, c.i)}</div>;
              })}
              {after > 0 && <div style={{ width: after, flex: 'none' }} />}
            </div>
          </div>
          <div style={{ position: 'relative', height: shownRows.length * ROW }}>
            {shownRows.slice(firstRow, lastRow).map((r, n) => {
              const at = firstRow + n;
              return (
                <div key={r.id} className="sheet__row sheet__row--body" style={{ transform: `translateY(${at * ROW}px)` }}>
                  {[SKU, NAME].map((id, i) => cell(r, at, id, i, pinStyle(i)))}
                  {before > 0 && <div style={{ width: before, flex: 'none' }} />}
                  {centre.map(c => cell(r, at, c.id, c.i, { width: widths[c.i] }))}
                  {after > 0 && <div style={{ width: after, flex: 'none' }} />}
                </div>
              );
            })}
            {!shownRows.length && <div className="sheet__nothing">{!missingOnly ? 'No products yet.' : rows.some(r => r.issues.some(i => i.level === 'error')) ? 'Every field is filled in. What is still owed is photographs: see Photos.' : 'Nothing is missing.'}</div>}
          </div>
        </div>
      </div>
      {(hereIssue || hereDefault) && (
        <div className="sheet__said" role="status">
          {hereIssue ? <>{byKey.get(hereKey)?.label ?? hereKey} for {here!.sku}: {hereIssue.message ?? (hereIssue.level === 'error' ? 'Required' : '')}</>
            : <>{byKey.get(hereKey)?.label ?? hereKey} for {here!.sku} is filled in for you. Type over it if yours is different.</>}
        </div>
      )}
      {editing && (
        <Lifted key={cellId(editing.row, editing.col)} scroll={scroll} rowAt={shownRows.findIndex(r => r.id === editing.row)} colAt={cols.findIndex(c => c.id === editing.col)} row={rows.find(r => r.id === editing.row)} field={fieldOf(editing.col)} seed={editing.seed} all={editing.all}
          keep={(v, then) => { const r = rows.find(x => x.id === editing.row); if (r && textOf(r, editing.col) !== v) write([{ row: r.id, key: keyOf(editing.col), value: v }]); close(then); }} cancel={() => close()} />
      )}
    </>
  );
}

function Lifted({ scroll, rowAt, colAt, row, field, seed, all, keep, cancel }: {
  scroll: RefObject<HTMLDivElement | null>; rowAt: number; colAt: number; row?: SheetRow; field?: TField; seed?: string; all?: boolean; keep: (value: string, then?: 'down' | 'right' | 'left') => void; cancel: () => void;
}) {
  const [up, setUp] = useState(false);
  const [at, setAt] = useState<{ left: number; top: number; width: number; height: number } | null>(null);
  const [value, setValue] = useState(seed ?? row?.values[field?.key ?? ''] ?? '');
  const [q, setQ] = useState(seed ?? '');
  const [typed, setTyped] = useState(!!seed);
  const box = useRef<HTMLInputElement>(null);
  const [hot, setHot] = useState(0);
  const ref = useRef<HTMLDivElement>(null);
  const issue = row?.issues.find(i => i.field === field?.key);
  useLayoutEffect(() => {
    const place = () => {
      const cell = scroll.current?.querySelector<HTMLElement>(`[data-row="${rowAt}"][data-col="${colAt}"]`);
      if (!cell) return cancel();
      const b = cell.getBoundingClientRect();
      setAt({ left: b.left, top: b.top, width: b.width, height: b.height });
      setUp(innerHeight - b.bottom < 260 && b.top > innerHeight - b.bottom);
    };
    place();
    const el = scroll.current;
    el?.addEventListener('scroll', place);
    addEventListener('resize', place);
    return () => { el?.removeEventListener('scroll', place); removeEventListener('resize', place); };
  }, [rowAt, colAt]);
  useEffect(() => {
    const out = (e: PointerEvent) => { if (!ref.current?.contains(e.target as Node)) keep(value); };
    addEventListener('pointerdown', out, true);
    return () => removeEventListener('pointerdown', out, true);
  });
  if (!field || !row) return null;
  const opts = optionsOf(field);
  const listed = opts.length > 0;
  const multi = !!field.multi && field.type === 'select';
  const held = partsOf(value);
  const has = (o: string) => held.some(p => p.toLowerCase() === o.toLowerCase());
  const found = opts.filter(o => !typed || !q.trim() || o.toLowerCase().includes(q.trim().toLowerCase()));
  const toggle = (o: string) => {
    const unknown = held.filter(p => !opts.some(x => x.toLowerCase() === p.toLowerCase()));
    setValue([...opts.filter(x => (x === o ? !has(o) : has(x))), ...unknown].join(MULTI));
  };
  const choose = (o: string) => { if (multi) { toggle(o); setTyped(false); setQ(''); box.current?.focus(); } else keep(o, 'down'); };
  const common = (e: KeyboardEvent) => {
    if (e.key === 'Escape') { e.preventDefault(); cancel(); return true; }
    if (e.key === 'Tab') { e.preventDefault(); keep(value, e.shiftKey ? 'left' : 'right'); return true; }
    return false;
  };
  const enterKeeps = (e: KeyboardEvent) => { if (common(e)) return; if (e.key === 'Enter') { e.preventDefault(); keep(value, 'down'); } };
  const style = at ? { left: at.left, top: at.top, minWidth: at.width, '--cell-h': `${at.height}px` } as CSSProperties : { opacity: 0 };
  return (
    <div ref={ref} className={`sheet__lift ${issue?.level === 'error' ? 'sheet__lift--missing' : ''}`} style={style} onKeyDown={e => e.stopPropagation()}>
      {listed ? (
        <>
          <span className="sheet__combo">
            <input ref={box} className="sheet__input" autoFocus value={typed ? q : multi ? held.join(', ') : value} onFocus={e => { if (all || !seed) e.currentTarget.select(); }}
              onChange={e => { setQ(e.target.value); setTyped(true); setHot(0); }}
              onKeyDown={e => {
                if (common(e)) return;
                if (e.key === 'ArrowDown') { e.preventDefault(); setHot(Math.min(hot + 1, found.length - 1)); }
                else if (e.key === 'ArrowUp') { e.preventDefault(); setHot(Math.max(hot - 1, 0)); }
                else if (e.key === 'Enter') { e.preventDefault(); if (found[hot]) choose(found[hot]); else if (multi) keep(value, 'down'); else if (q.trim()) keep(q.trim(), 'down'); else keep(value, 'down'); }
              }} />
            <Icon name="chevron" size="sm" />
          </span>
          <div className={`sheet__drop ${up ? 'sheet__drop--up' : ''}`} role="listbox" aria-multiselectable={multi || undefined}>
            {found.map((o, i) => (
              <button key={o} type="button" role="option" aria-selected={has(o)} className={`menu-item ${i === hot ? 'menu-item--hot' : ''}`} onMouseEnter={() => setHot(i)} onMouseDown={e => e.preventDefault()} onClick={() => choose(o)}>
                <Lines top={o} />{has(o) && <Icon name="check" />}
              </button>
            ))}
            {!found.length && <span className="menu-label">{typed && q.trim() ? `Nothing like “${q.trim()}”. Enter keeps it as typed.` : 'No options'}</span>}
            {multi && <div className="sheet__foot"><span>{held.length ? `${held.length} picked` : 'None picked'}</span><Button variant="primary" onMouseDown={e => e.preventDefault()} onClick={() => keep(value, 'down')}>Done</Button></div>}
          </div>
        </>
      ) : field.type === 'dimensions' && !field.multi ? (
        <Dimensions parts={field.parts ?? ['W', 'D', 'H']} value={value} onChange={setValue} onKey={enterKeeps} />
      ) : (
        <Grow value={value} type={field.type} all={all} onChange={setValue} onKey={enterKeeps} />
      )}
    </div>
  );
}
function Grow({ value, type, all, onChange, onKey }: { value: string; type?: string; all?: boolean; onChange: (v: string) => void; onKey: (e: KeyboardEvent) => void }) {
  const ref = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => { const el = ref.current!; el.focus({ preventScroll: true }); if (all) el.select(); else el.setSelectionRange(el.value.length, el.value.length); }, []);
  return <span className="sheet__grow" data-value={value || ' '}><input ref={ref} className="sheet__input" value={value} inputMode={type === 'number' ? 'decimal' : undefined} placeholder={type === 'number' ? '0' : ''} onChange={e => onChange(e.target.value)} onKeyDown={onKey} /></span>;
}
function Dimensions({ parts, value, onChange, onKey }: { parts: string[]; value: string; onChange: (v: string) => void; onKey: (e: KeyboardEvent) => void }) {
  const got = value.split(/\s*[x×]\s*/i);
  const set = (i: number, v: string) => onChange(parts.map((_, j) => (j === i ? v : got[j] ?? '').trim()).join(' x ').replace(/( x )+$/, ''));
  return (
    <span className="sheet__dims">
      {parts.map((p, i) => <label key={p} className="sheet__dim"><span>{p}</span><input className="sheet__input" autoFocus={i === 0} inputMode="decimal" value={got[i] ?? ''} onChange={e => set(i, e.target.value)} onKeyDown={onKey} /></label>)}
    </span>
  );
}

const PHOTO_DRAG = 'application/x-photo';
export type Slot = { pics: { id: string; file?: string; issue?: { level: string; message: string } }[]; owed: number };
export type PhotoRow = { id: string; name: string; sku: string; slots: Record<string, Slot> };
export type TrayPic = { id: string; name: string; file?: string; placed: boolean };
export function PhotoGrid({ roles, rows, missingOnly, picking, onPlace, onClear }: { roles: { key: string; label: string }[]; rows: PhotoRow[]; missingOnly?: boolean; picking?: string | null; onPlace?: (photo: string, row: string, role: string) => void; onClear?: (photo: string) => void }) {
  const [hot, setHot] = useState('');
  const shown = missingOnly ? rows.filter(r => roles.some(c => r.slots[c.key]?.owed)) : rows;
  return (
    <div className="photo-grid">
      <table>
        <thead><tr><th className="photo-grid__pin">Name</th><th className="photo-grid__pin photo-grid__pin--2">SKU</th>{roles.map(c => <th key={c.key} title={c.label}>{c.label}</th>)}</tr></thead>
        <tbody>
          {shown.map(r => (
            <tr key={r.id}>
              <td className="photo-grid__pin">{r.name}</td><td className="photo-grid__pin photo-grid__pin--2">{r.sku}</td>
              {roles.map(c => {
                const v = r.slots[c.key] ?? { pics: [], owed: 0 };
                const at = `${r.id}|${c.key}`;
                const slot = onPlace ? {
                  onDragOver: (e: React.DragEvent) => { if (e.dataTransfer.types.includes(PHOTO_DRAG)) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; if (hot !== at) setHot(at); } },
                  onDragLeave: () => setHot(''),
                  onDrop: (e: React.DragEvent) => { const id = e.dataTransfer.getData(PHOTO_DRAG); setHot(''); if (id) { e.preventDefault(); e.stopPropagation(); onPlace(id, r.id, c.key); } },
                  onClick: () => { if (picking) onPlace(picking, r.id, c.key); }
                } : {};
                return (
                  <td key={c.key} className={`photo-grid__slot ${hot === at ? 'is-hot' : ''} ${picking ? 'is-picking' : ''}`} aria-label={v.owed ? `${v.owed} owed` : undefined} {...slot}>
                    <span className="photo-grid__tiles">
                      {v.pics.map(p => (
                        <span key={p.id} className={`tile ${p.issue ? `tile--${p.issue.level === 'error' ? 'bad' : 'warn'}` : ''}`} title={p.issue?.message}>
                          <Pic file={p.file} />
                          {onClear && <button type="button" className="unpin" aria-label="Take this photo off" onClick={e => { e.stopPropagation(); onClear(p.id); }}><Icon name="close" size="xs" /></button>}
                        </span>
                      ))}
                      {Array.from({ length: v.owed }, (_, i) => <span key={i} className="tile tile--owed" />)}
                      {!v.pics.length && !v.owed && <span className="tile tile--empty" />}
                    </span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export function PhotoTray({ pics, picked, onPick, onRemove, onFiles, children }: { pics: TrayPic[]; picked?: string | null; onPick?: (id: string | null) => void; onRemove?: (id: string) => void; onFiles?: (files: File[]) => void; children: ReactNode }) {
  const [over, setOver] = useState(false);
  return (
    <div className="traywrap">
      {children}
      <div className={`tray ${over ? 'tray--over' : ''}`} onDragOver={e => { if (onFiles && e.dataTransfer.types.includes('Files')) { e.preventDefault(); setOver(true); } }}
        onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setOver(false); }}
        onDrop={e => { if (!onFiles || !e.dataTransfer.files.length) return; e.preventDefault(); e.stopPropagation(); setOver(false); onFiles([...e.dataTransfer.files]); }}>
        {!pics.length && <span className="tray__empty">{onFiles ? 'Drop photographs here' : 'No photographs yet'}</span>}
        <div className="tray__pics">
          {pics.map(p => (
            <span key={p.id} className={`tray__item ${picked === p.id ? 'tray__item--picked' : ''} ${p.placed ? 'tray__item--placed' : ''}`} title={p.name} draggable={!!onPick}
              onDragStart={e => { e.dataTransfer.setData(PHOTO_DRAG, p.id); e.dataTransfer.effectAllowed = 'move'; }} onClick={() => onPick?.(picked === p.id ? null : p.id)}>
              <Pic file={p.file} alt={p.name} />
              {onRemove && <button type="button" className="unpin" aria-label={`Remove ${p.name}`} onClick={e => { e.stopPropagation(); onRemove(p.id); }}><Icon name="close" size="xs" /></button>}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
