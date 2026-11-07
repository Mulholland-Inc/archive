import { useEffect, useRef, useState, type ClipboardEvent, type KeyboardEvent, type ReactNode } from 'react';
import { create, get, list, moved, patch, run, saveFile, upload, useQuery, useRead, type Row } from './api';
import {
  Actions, Button, CheckField, Combo, Confirm, Dialog, DropRegion, DropZone, Empty, Field, FormCard, FormLine, FormRow, GONE, Icon, IconButton, IconLabel, Labeled, Lines, LinkButton, List, Log, Note, Page, PageState, Pic, Picker, Problem, SearchField, Section, Spacer, Stages, Table, Tabs, Tag, Toolbar, toast, useWrite, type Column
} from './ui';
import { NAME_KEY, SKU_KEY, STATUS, ago, byPosition, categoryIcon, idle, isOpen, issuesOf, listedKey, nameKeyOf, plural, sentence, show, stageTone, standingOf, statusName, statusRank, tone, tooBig, useNav, type Spec, type TField } from './model';

export const bookRead = () => list('build_book');
export const specOf = (s: Partial<Spec>): Spec => ({ fields: s.fields ?? [], images: s.images ?? [], photo: s.photo ?? null });
const BOOK = ['build', 'build_row', 'build_image', 'build_activity', 'vendor', 'template'];

export const marked = (key: unknown, icon: unknown, words: ReactNode) => <IconLabel icon={categoryIcon(key, icon)}>{words}</IconLabel>;
export const COMPLETE: Column<Row> = { key: 'done', label: 'Complete', soft: true, num: true, render: r => (idle(r) ? '' : `${r.clean ?? 0}/${r.skus ?? 0}`) };
export const DUE: Column<Row> = { key: 'due', label: 'Due', soft: true, num: true, sort: r => (r.stage === 'cancelled' ? null : r.due_at), render: r => (r.stage === 'cancelled' ? '' : show(r.due_at)) };
export const STANDING: Column<Row> = { key: 'status', label: 'Status', num: true, sort: r => r.standing, render: r => <Tag tone={tone(r.tone)}>{r.standing}</Tag> };

type Bulk = 'send' | 'approve' | 'cancel' | 'delete';
const BULKS: Record<Bulk, { label: string; icon?: 'check'; done: string; action: string; applies: (b: Row) => boolean; ask: (n: number) => [string, string, string] }> = {
  send: { label: 'Send', done: 'Sent', action: 'send_build', applies: b => b.stage === 'draft' && (b.emails?.length ?? 0) > 0, ask: n => [`Send ${plural(n, 'build')}?`, 'Each vendor gets their link by email.', 'Send'] },
  approve: { label: 'Approve', icon: 'check', done: 'Approved', action: 'approve_build', applies: b => b.stage === 'sent' || b.stage === 'in_progress' || b.stage === 'review', ask: n => [`Approve ${plural(n, 'build')}?`, 'Their products are saved and each vendor is thanked by email. Export them from each build’s page.', 'Approve'] },
  cancel: { label: 'Cancel', done: 'Cancelled', action: 'cancel_build', applies: b => b.stage !== 'complete' && b.stage !== 'cancelled', ask: n => [`Cancel ${plural(n, 'build')}?`, 'Final. A vendor who already has one gets an email that it is withdrawn. Nothing they entered is lost.', 'Cancel builds'] },
  delete: { label: 'Delete', done: 'Deleted', action: 'delete_build', applies: b => b.stage === 'draft' || b.stage === 'cancelled' || b.stage === 'complete', ask: n => [`Delete ${plural(n, 'build')}?`, 'A draft or a cancelled build goes for good, with its products, photos and history. An approved one’s products stay in the catalogue.', 'Delete'] }
};

export async function eachRow(rows: Row[], name: (r: Row) => string, one: (r: Row) => Promise<string | void>) {
  let took = 0;
  const said: string[] = [];
  for (const r of rows) {
    try { const besides = await one(r); took++; if (besides) said.push(`${name(r)}: ${besides}`); } catch (e) { said.push(`${name(r)}: ${(e as Error).message}`); }
  }
  moved();
  return { took, said };
}
export const choices = (rows: Row[], value: string, label = value) =>
  [...new Map(rows.filter(r => r[value]).map(r => [String(r[value]), String(r[label] ?? r[value])])).entries()].map(([v, l]) => ({ value: v, label: l })).sort((a, b) => a.label.localeCompare(b.label));

export function Builds({ me }: { me: string }) {
  const { go } = useNav();
  const { data, error, reload } = useRead('build_book', bookRead, BOOK);
  const [view, setView] = useQuery('view');
  const [q, setQ] = useQuery('q');
  const [stage, setStage] = useQuery('stage');
  const [category, setCategory] = useQuery('category');
  const [vendor, setVendor] = useQuery('vendor');
  const [by, setBy] = useQuery('by');
  const [mine, setMine] = useQuery('mine');
  const [ids, setIds] = useState<Set<string>>(new Set());
  const [asking, setAsking] = useState<Bulk | null>(null);
  const [busy, setBusy] = useState(false);
  if (!data) return <PageState title="Builds" error={error} />;
  const filtered = !!(stage || category || vendor || by || mine || q);
  const clear = () => { setStage(''); setCategory(''); setVendor(''); setBy(''); setMine(''); setQ(''); };
  const shown = data
    .filter(r => !stage || r.status === stage).filter(r => !category || String(r.category) === category).filter(r => !vendor || String(r.vendor_slug) === vendor).filter(r => !by || r.created_by_name === by)
    .filter(r => !mine || r.created_by === me).filter(r => !q || `${r.name} ${r.vendor} ${r.category_name}`.toLowerCase().includes(q.toLowerCase()));
  const grouped = view === 'grouped';
  const groups = grouped
    ? STATUS.map(([id, title]) => ({ title, tone: tone(shown.find(r => r.status === id)?.tone), rows: shown.filter(r => r.status === id) })).filter(g => g.rows.length)
    : [{ rows: [...shown].sort((a, b) => statusRank(a) - statusRank(b) || String(a.due_at ?? '').localeCompare(String(b.due_at ?? ''))) }];
  const selected = shown.filter(r => ids.has(String(r.id)));
  const applying = (k: Bulk) => selected.filter(BULKS[k].applies);
  const asked = asking && BULKS[asking].ask(applying(asking).length);
  const confirm = async () => {
    const k = asking!, on = applying(k), skipped = selected.length - on.length;
    setBusy(true);
    const { took, said } = await eachRow(on, r => `“${r.name}”`, async r => { const out = await run(BULKS[k].action, { id: String(r.id) }); return out?.invite_error || out?.mail_error; });
    await reload();
    setBusy(false);
    toast([`${BULKS[k].done} ${plural(took, 'build')}.`, ...said, ...(skipped ? [`Skipped ${skipped} that cannot be ${BULKS[k].done.toLowerCase()}.`] : [])].join(' '), said.length ? 'error' : 'notice');
    setAsking(null); setIds(new Set());
  };
  const name: Column<Row> = { key: 'name', label: 'Build', fill: true, sort: r => r.name, render: r => (grouped ? marked(r.category, r.category_icon, String(r.name ?? '')) : r.name) };
  const vendorCol: Column<Row> = { key: 'vendor', label: 'Vendor', soft: true, render: r => r.vendor };
  const columns: Column<Row>[] = grouped ? [name, vendorCol, COMPLETE, DUE] : [name, vendorCol,
    { key: 'category', label: 'Category', soft: true, sort: r => r.category_name, render: r => marked(r.category, r.category_icon, String(r.category_name ?? '')) },
    { key: 'status', label: 'Status', sort: statusRank, render: r => <Tag tone={tone(r.tone)}>{statusName(r)}</Tag> }, COMPLETE, DUE,
    { key: 'last', label: 'Last activity', soft: true, num: true, sort: r => (r.last_activity_at ? -new Date(r.last_activity_at).getTime() : null), render: r => ago(r.last_activity_at) }];
  return (
    <Page title="Builds" list wide={!grouped}>
      <Toolbar><SearchField placeholder="Search builds" value={q} onChange={setQ} /><Button variant="primary" icon="plus" onClick={() => go('/builds/new')}>New build</Button></Toolbar>
      {selected.length ? (
        <Toolbar wrap>
          <Note>{plural(selected.length, 'build')} selected</Note><Spacer />
          {(Object.keys(BULKS) as Bulk[]).map(k => <Button key={k} icon={BULKS[k].icon} disabled={!applying(k).length} onClick={() => setAsking(k)}>{applying(k).length ? `${BULKS[k].label} (${applying(k).length})` : BULKS[k].label}</Button>)}
        </Toolbar>
      ) : (
        <Toolbar wrap>
          <Picker label="Status" value={stage} options={STATUS.map(([value, label]) => ({ value, label }))} onChange={setStage} />
          <Picker label="Category" value={category} options={choices(data, 'category', 'category_name')} onChange={setCategory} />
          <Picker label="Vendor" value={vendor} options={choices(data, 'vendor_slug', 'vendor')} onChange={setVendor} />
          <Picker label="Created by" value={by} options={choices(data, 'created_by_name')} onChange={setBy} />
          <Button pressed={!!mine} onClick={() => setMine(mine ? '' : '1')}>Mine</Button>
          <Spacer />
          <Tabs value={grouped ? 'grouped' : 'table'} onChange={v => setView(v === 'table' ? '' : v)} tabs={[{ id: 'table', label: 'Table', icon: 'table' }, { id: 'grouped', label: 'By status', icon: 'list' }]} />
        </Toolbar>
      )}
      {shown.length ? <Table<Row> groups={groups} columns={columns} select={{ ids, idOf: r => String(r.id), onChange: setIds }} onRow={r => go(`/builds/${r.id}`)} />
        : <Toolbar><Note>{filtered ? 'No build matches these filters.' : 'No builds yet.'}</Note>{filtered && <Button onClick={clear}>Clear filters</Button>}</Toolbar>}
      {asked && <Confirm title={asked[0]} note={asked[1]} yes={busy ? 'Working…' : asked[2]} busy={busy} onYes={confirm} onClose={() => setAsking(null)} />}
    </Page>
  );
}

const NOISE = new Set(['llc', 'inc', 'ltd', 'co', 'corp', 'company', 'the', 'and']);
const namesVendor = (file: string, vendor: string) => {
  const inFile = new Set(file.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean));
  return vendor.toLowerCase().split(/[^a-z0-9]+/).some(w => w.length >= 3 && !NOISE.has(w) && inFile.has(w));
};
type Listed = { id: string; sku: string; name: string };
type Elsewhere = { sku: string; product: string; build: string; number: number; name: string; vendor: string; stage: string };
const newId = () => crypto.randomUUID();
const distinct = (all: Listed[]) => { const seen = new Set<string>(); return all.filter(p => { const k = listedKey(p); return !!k && !seen.has(k) && !!seen.add(k); }); };
const asRow = (p: Listed) => ({ sku: p.sku.trim(), name: p.name.trim(), values: {} });
const joined = (all: Listed[], more: Listed[]) => { const have = new Set(all.map(listedKey)); return [...all.filter(p => listedKey(p)), ...more.filter(p => !have.has(listedKey(p)))]; };

function useElsewhere(items: { sku: string; name: string }[], build?: string): Elsewhere[] {
  const [found, setFound] = useState<Elsewhere[]>([]);
  const skus = items.map(p => p.sku.trim()).filter(Boolean);
  const names = items.filter(p => !p.sku.trim()).map(p => p.name.trim()).filter(Boolean);
  const key = `${skus.join('\n')}\u0000${names.join('\n')}`;
  useEffect(() => {
    if (!skus.length && !names.length) { setFound([]); return; }
    let live = true;
    const t = setTimeout(() => { run<{ elsewhere?: Elsewhere[] }>('skus_elsewhere', { skus, names, ...(build ? { build } : {}) }).then(out => { if (live) setFound(out?.elsewhere ?? []); }, e => toast((e as Error).message)); }, 400);
    return () => { live = false; clearTimeout(t); };
  }, [key, build]);
  return found;
}
const alsoOnOf = (all: Listed[], found: Elsewhere[]): Record<string, string> => Object.fromEntries(all.flatMap(p => {
  const on = found.filter(f => listedKey({ sku: f.sku, name: f.sku ? '' : f.product }) === listedKey(p));
  return on.length ? [[p.id, `Also on ${on.map(f => `build ${f.number}, ${f.name}`).join('; ')}`]] : [];
}));

function ProductTable({ rows, onChange, marks, closed }: { rows: Listed[]; onChange: (next: Listed[]) => void; marks: Record<string, string>; closed?: boolean }) {
  const [draft, setDraft] = useState(newId);
  const box = useRef<HTMLDivElement>(null);
  const COLS = [{ key: 'sku' as const, label: 'SKU', placeholder: 'e.g. 254180-002' }, { key: 'name' as const, label: 'Product name', placeholder: 'e.g. Kadner Dining Chair, Cocoa' }];
  const lines = closed ? rows : [...rows, { id: draft, sku: '', name: '' }];
  const grow = (next: Listed[]) => { if (next.some(r => r.id === draft)) setDraft(newId()); onChange(next); };
  const write = (id: string, key: 'sku' | 'name', value: string) => grow(rows.some(r => r.id === id) ? rows.map(r => (r.id === id ? { ...r, [key]: value } : r)) : [...rows, { id, sku: '', name: '', [key]: value }]);
  const paste = (line: number, col: number) => (e: ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData('text/plain').replace(/\r/g, '');
    if (!/[\n\t]/.test(text)) return;
    e.preventDefault();
    const pasted = text.split('\n');
    if (pasted.length > 1 && pasted[pasted.length - 1] === '') pasted.pop();
    const next = lines.map(r => ({ ...r }));
    pasted.forEach((words, dy) => {
      while (next.length <= line + dy) next.push({ id: newId(), sku: '', name: '' });
      words.split('\t').forEach((v, dx) => { const c = COLS[col + dx]; if (c) next[line + dy][c.key] = v.trim(); });
    });
    grow(next.filter((r, i) => i < rows.length || r.sku.trim() || r.name.trim()));
  };
  const down = (line: number, col: number) => (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    box.current?.querySelector<HTMLInputElement>(`[data-at="${line + 1}:${col}"]`)?.focus();
  };
  return (
    <div className="card form-table" ref={box}>
      <div className="form-table__head">{COLS.map(c => <span key={c.key}>{c.label}</span>)}<span /></div>
      {lines.map((r, line) => (
        <div key={r.id} className="form-table__line">
          {COLS.map((c, col) => (
            <span key={c.key} className="form-table__cell">
              {col === 0 && marks[r.id] && <span className="form-table__mark" title={marks[r.id]}><Icon name="alert" size="xs" /></span>}
              <input aria-label={c.label} data-at={`${line}:${col}`} value={r[c.key]} placeholder={r.id === draft ? c.placeholder : undefined} onChange={e => write(r.id, c.key, e.target.value)} onPaste={paste(line, col)} onKeyDown={down(line, col)} />
            </span>
          ))}
          {r.id === draft ? <IconButton icon="plus" label="Add a product" onClick={() => box.current?.querySelector<HTMLInputElement>(`[data-at="${line}:0"]`)?.focus()} />
            : <IconButton reveal icon="close" label="Remove this product" onClick={() => onChange(rows.filter(x => x.id !== r.id))} />}
        </div>
      ))}
    </div>
  );
}

type Kind = 'sku' | 'name' | '';
const gridOf = (text: string) => text.replace(/\r/g, '').split('\n').map(l => l.split('\t').map(c => c.trim())).filter(r => r.some(Boolean));
const headed = (cell: string): Kind =>
  /^(manufacturer |manu |mfg |vendor )?(sku|item( no\.?| number| #)?|style( no\.?| number| #)?|model( number)?|part( number)?)$/i.test(cell) ? 'sku' : /^((vendor |manufacturer )?product( name)?|name|title|description|item name)$/i.test(cell) ? 'name' : '';
const headedGrid = (grid: string[][]) => grid.length > 1 && grid[0].some(c => headed(c));
function guessKinds(grid: string[][]): Kind[] {
  const width = Math.max(0, ...grid.map(r => r.length));
  const head = headedGrid(grid);
  const body = head ? grid.slice(1) : grid;
  const kinds: Kind[] = [];
  for (let i = 0; i < width; i++) {
    const guess: Kind = (head ? headed(grid[0][i] ?? '') : '') || (body.some(r => /\s/.test(r[i] ?? '')) ? 'name' : 'sku');
    kinds.push(kinds.includes(guess) ? (kinds.includes(guess === 'sku' ? 'name' : 'sku') ? '' : guess === 'sku' ? 'name' : 'sku') : guess);
  }
  return kinds;
}
const csvAsPasted = (text: string) => text.replace(/\r/g, '').split('\n').map(line => {
  const cells: string[] = [];
  let cell = '', quoted = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === '"' && quoted && line[i + 1] === '"') { cell += '"'; i++; } else if (c === '"') quoted = !quoted; else if (c === ',' && !quoted) { cells.push(cell); cell = ''; } else cell += c;
  }
  return [...cells, cell].join('\t');
}).join('\n');
function pastedProducts(grid: string[][], kinds: Kind[]): Listed[] {
  const body = headedGrid(grid) ? grid.slice(1).filter(r => r.join('\t') !== grid[0].join('\t')) : grid;
  const sku = kinds.indexOf('sku'), name = kinds.indexOf('name');
  const split = (s: string) => s.split(/[\s,;]+/).filter(Boolean).map(one => ({ id: newId(), sku: one, name: '' }));
  if (kinds.length === 1 && sku === 0) return body.flatMap(r => split(r[0]));
  return body.flatMap(r => {
    const s = sku >= 0 ? r[sku] ?? '' : '', n = name >= 0 ? r[name] ?? '' : '';
    return !n && /[,;]/.test(s) ? split(s) : [{ id: newId(), sku: s, name: n }];
  }).filter(p => p.sku || p.name);
}
async function unzipped(buf: ArrayBuffer, want: (name: string) => boolean): Promise<Map<string, string>> {
  const v = new DataView(buf);
  const words = new TextDecoder();
  let end = buf.byteLength - 22;
  while (end >= 0 && v.getUint32(end, true) !== 0x06054b50) end--;
  if (end < 0) throw new Error('That file could not be opened.');
  const out = new Map<string, string>();
  let at = v.getUint32(end + 16, true);
  for (let i = v.getUint16(end + 10, true); i > 0; i--) {
    const method = v.getUint16(at + 10, true), size = v.getUint32(at + 20, true), named = v.getUint16(at + 28, true), local = v.getUint32(at + 42, true);
    const name = words.decode(new Uint8Array(buf, at + 46, named));
    at += 46 + named + v.getUint16(at + 30, true) + v.getUint16(at + 32, true);
    if (!want(name)) continue;
    const raw = new Uint8Array(buf, local + 30 + v.getUint16(local + 26, true) + v.getUint16(local + 28, true), size);
    out.set(name, words.decode(method === 0 ? raw : new Uint8Array(await new Response(new Blob([raw]).stream().pipeThrough(new DecompressionStream('deflate-raw'))).arrayBuffer())));
  }
  return out;
}
async function slidesAsPasted(f: File): Promise<string> {
  const slides = await unzipped(await f.arrayBuffer(), n => /^ppt\/slides\/slide\d+\.xml$/.test(n));
  const order = [...slides.keys()].sort((a, b) => Number(/(\d+)\.xml$/.exec(a)![1]) - Number(/(\d+)\.xml$/.exec(b)![1]));
  const text = (el: Element) => [...el.getElementsByTagNameNS('*', 't')].map(t => t.textContent ?? '').join('').trim();
  const lines: string[] = [];
  for (const name of order) {
    const doc = new DOMParser().parseFromString(slides.get(name)!, 'application/xml');
    for (const table of doc.getElementsByTagNameNS('*', 'tbl')) for (const row of table.getElementsByTagNameNS('*', 'tr')) lines.push([...row.getElementsByTagNameNS('*', 'tc')].map(text).join('\t'));
    for (const p of doc.getElementsByTagNameNS('*', 'p')) if (!p.closest('tbl') && text(p)) lines.push(text(p));
  }
  return lines.join('\n');
}

type Prefill = { file: string; rows: Listed[]; mapping: Row; columns: string[]; column_map: Row };
type Brought = { rows: Listed[]; sheet?: Prefill };
const SHEETS = '.xlsx,.xls,.csv,.tsv,.txt,.pptx';
async function readSheet(f: File, against: Row): Promise<Prefill> {
  const up = await upload(f);
  const r = await run('import_file', { file: up.id, ...against });
  const skuHeader = r.mapping[SKU_KEY] ?? '', nameHeader = r.mapping[NAME_KEY] ?? '';
  const seen = new Set<string>();
  const rows: Listed[] = [];
  for (const row of r.rows as Row[]) {
    const p = { id: newId(), sku: String((skuHeader && row[skuHeader]) ?? '').trim(), name: String((nameHeader && row[nameHeader]) ?? '').trim() };
    const k = listedKey(p);
    if (k && !seen.has(k)) { seen.add(k); rows.push(p); }
  }
  if (!rows.length) throw new Error(`We found no products in ${f.name}.`);
  return { file: r.source || f.name, rows, mapping: r.mapping, columns: r.columns ?? [], column_map: r.column_map ?? {} };
}

function ProductImport({ title, against, have = [], build, file, busy, onDone, onClose }: { title: string; against: Row; have?: Listed[]; build?: string; file?: File; busy?: boolean; onDone: (brought: Brought) => void; onClose: () => void }) {
  const [text, setText] = useState('');
  const [preview, setPreview] = useState<Brought | null>(null);
  const [error, setError] = useState('');
  const [reading, setReading] = useState('');
  const known = new Set(have.map(listedKey));
  const names = new Set(have.map(p => p.name.trim().toLowerCase()).filter(Boolean));
  const there = (p: Listed) => known.has(listedKey(p)) || (!p.sku.trim() && names.has(p.name.trim().toLowerCase()));
  const read = (words: string) => pastedProducts(gridOf(words), guessKinds(gridOf(words)));
  const typed = read(text);
  const showWords = (words: string) => { const rows = read(words); if (rows.length) { setError(''); setPreview({ rows }); } else { setText(words); setError('There are no SKUs or product names in that.'); } };
  const onFile = async (f: File) => {
    if (/\.(csv|tsv|txt)$/i.test(f.name) || /^text\//.test(f.type)) { const words = await f.text(); return showWords(/\.csv$/i.test(f.name) || f.type === 'text/csv' ? csvAsPasted(words) : words); }
    try {
      if (/\.pptx$/i.test(f.name)) return showWords(await slidesAsPasted(f));
      if (tooBig(f)) return setError(tooBig(f));
      setReading(`${f.name}: Reading its columns…`);
      const sheet = await readSheet(f, against);
      setPreview({ rows: sheet.rows, sheet });
    } catch (e) { setError((e as Error).message); }
    setReading('');
  };
  useEffect(() => { if (file) void onFile(file); }, [file]);
  const rows = preview ? distinct(preview.rows) : [];
  const fresh = rows.filter(p => !there(p));
  const elsewhere = useElsewhere(fresh, build);
  const marks = alsoOnOf(fresh, elsewhere);
  const unique = fresh.filter(p => !marks[p.id]);
  if (!preview) return (
    <Dialog wide title={title} onClose={onClose} foot={<><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={!typed.length || !!reading} onClick={() => setPreview({ rows: typed })}>Continue</Button></>}>
      <div onPasteCapture={e => { const words = e.clipboardData.getData('text/plain'); if (words.trim()) { e.preventDefault(); showWords(words); } }}>
        <Field bare label="Paste from your sheet" lines={6} autoFocus value={text} onChange={v => { setText(v); setError(''); }} placeholder="Copy the SKU column, the product name column, or both side by side, and paste them here" />
      </div>
      {error && <Problem>{error}</Problem>}
      {reading ? <Note block>{reading}</Note> : <DropZone hint="Or drop the file with the list, or click to choose it: a spreadsheet, csv, txt or slides (pptx)." accept={SHEETS} onFile={onFile} />}
    </Dialog>
  );
  return (
    <Dialog wide title={title} onClose={onClose} foot={<>
      {unique.length < fresh.length && <Button disabled={!unique.length || busy} onClick={() => onDone({ rows: unique, sheet: preview.sheet })}>{`Only add ${plural(unique.length, 'unique product')}`}</Button>}
      <Button variant="primary" disabled={!fresh.length || busy} onClick={() => onDone({ rows: fresh, sheet: preview.sheet })}>{busy ? 'Adding…' : `Add ${unique.length < fresh.length ? 'all ' : ''}${plural(fresh.length, 'product')}`}</Button>
    </>}>
      <ProductTable closed rows={fresh} marks={marks} onChange={next => setPreview({ ...preview, rows: next })} />
    </Dialog>
  );
}

export const isEmail = (e: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e);
export const notEmail = (e: string) => `“${e}” is not an email address.`;
const inDays = (n: number) => { const d = new Date(); d.setDate(d.getDate() + n); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`; };
const REMIND = ['2', '3', '5', '7'].map(d => ({ value: d, label: `${d} days` }));
const BUILD_TYPES = ['Owned', 'Dropship', 'MTO'].map(t => ({ value: t, label: t }));

function Recipients({ emails, onChange, onCheck, none, busy, locked }: { emails: string[]; onChange: (next: string[]) => void; onCheck: (why: string) => void; none: string; busy?: boolean; locked?: boolean }) {
  const [typed, setTyped] = useState('');
  const add = () => {
    const e = typed.trim();
    if (!e) return;
    if (!isEmail(e)) return onCheck(notEmail(e));
    onCheck('');
    setTyped('');
    if (!emails.some(x => x.toLowerCase() === e.toLowerCase())) onChange([...emails, e]);
  };
  return (
    <FormCard>
      {emails.map(e => <FormLine key={e} words={e}>{!locked && <IconButton reveal icon="close" label={`Remove ${e}`} onClick={() => onChange(emails.filter(x => x !== e))} />}</FormLine>)}
      {!locked && <FormLine><Field bare label="Add an email" type="email" value={typed} onChange={setTyped} onEnter={add} placeholder={emails.length ? 'Another address' : none} /><Button variant="quiet" icon="plus" disabled={!typed.trim() || busy} onClick={add}>Add</Button></FormLine>}
    </FormCard>
  );
}

function MailPreview({ ask, send, busy, onSend, onClose }: { ask: Row; send: string; busy?: boolean; onSend: () => void; onClose: () => void }) {
  const key = JSON.stringify(ask);
  const mail = useRead(`mail|${key}`, () => run<{ to: string[]; replyTo?: string; subject: string; html: string }>('preview_mail', ask), []);
  const to = mail.data?.to ?? [];
  return (
    <Dialog wide title="Preview email" onClose={onClose} foot={<><Button onClick={onClose}>Not now</Button><Button variant="primary" icon="send" disabled={!to.length || busy} onClick={onSend}>{busy ? 'Sending…' : send}</Button></>}>
      {!mail.data ? <Note block>{mail.error ?? 'Reading…'}</Note> : <>
        <div className="draft">
          <div><span>To</span><span>{to.join(', ') || 'Nobody yet: add an address first'}</span></div>
          {mail.data.replyTo && <div><span>Reply to</span><span>{mail.data.replyTo}</span></div>}
          <div><span>Subject</span><span>{mail.data.subject}</span></div>
        </div>
        <iframe className="mail-frame" title="The email" sandbox="" srcDoc={mail.data.html} />
      </>}
    </Dialog>
  );
}

export function NewBuild({ vendor }: { vendor?: string }) {
  const { go } = useNav();
  const tpl = useRead('template', () => list('template'), ['template']);
  const book = useRead('vendor_book', () => list('vendor_book'), ['vendor', 'build']);
  const builds = useRead('build_book', bookRead, BOOK);
  const [items, setItems] = useState<Listed[]>([]);
  const [prefill, setPrefill] = useState<Prefill | null>(null);
  const [importing, setImporting] = useState(false);
  const products = distinct(items);
  const elsewhere = useElsewhere(products);
  const [name, setName] = useState('');
  const [templateKey, setTemplateKey] = useState('furniture');
  const [vendorId, setVendorId] = useState('');
  const [emails, setEmails] = useState<string[]>([]);
  const started = useRef(false);
  const prefilled = useRef(true);
  const [due, setDue] = useState(() => inDays(7));
  const [every, setEvery] = useState('3');
  const [photos, setPhotos] = useState(true);
  const [buildType, setBuildType] = useState('Dropship');
  const [error, setError] = useState('');
  const [preview, setPreview] = useState(false);
  const [busy, setBusy] = useState('');
  const vendors = [...(book.data ?? [])].sort((a, b) => String(a.name).localeCompare(String(b.name)));
  const prefillFor = (id: string, category = templateKey) => {
    const saved = ((vendors.find(v => String(v.id) === id)?.contacts ?? []) as Row[]).map(c => String(c.email ?? '')).filter(Boolean);
    if (saved.length) return saved;
    const theirs = (builds.data ?? []).filter(b => String(b.vendor_id) === id && b.emails?.length)
      .sort((a, b) => Number(String(b.category) === category) - Number(String(a.category) === category) || String(b.created_at).localeCompare(String(a.created_at)));
    return (theirs[0]?.emails ?? []) as string[];
  };
  const pickVendor = (id: string) => { setError(''); setVendorId(id); setEmails(prefillFor(id)); prefilled.current = true; };
  useEffect(() => {
    if (!vendor || !book.data || !builds.data || started.current) return;
    started.current = true;
    pickVendor(vendor);
  }, [vendor, book.data, builds.data]);
  if (!tpl.data || !book.data) return <PageState title="New build" error={tpl.error ?? book.error} />;
  const categories = tpl.data.filter(t => t.key !== 'base').sort((a, b) => String(a.name).localeCompare(String(b.name)));
  const template = categories.find(t => t.key === templateKey) ?? categories[0];
  const known = vendors.find(v => String(v.id) === vendorId);
  const imported = ({ rows, sheet }: Brought) => {
    setImporting(false); setError('');
    setItems(joined(items, rows));
    if (!sheet) return;
    setPrefill(sheet);
    const guess = !vendorId && vendors.find(v => namesVendor(sheet.file, String(v.name)));
    if (guess) pickVendor(String(guess.id));
  };
  const addVendor = async (vendorName: string) => {
    setError('');
    try { const v = await create('vendor', { name: vendorName.trim() }); moved(); await book.reload(); setVendorId(String(v.id)); setEmails([]); } catch (e) { setError((e as Error).message); }
  };
  const lacking = (send: boolean) => !name.trim() ? 'Give the build a name. The vendor sees it too.' : !vendorId ? 'Which vendor is this for?'
    : send && !emails.length ? `${known?.name ?? 'This vendor'} has no contact on file yet. Add one before sending.` : !products.length ? 'Add at least one product by its SKU or its name: paste them, or drop the vendor’s sheet.' : '';
  const rows = products.map(asRow);
  const finish = async (send: boolean) => {
    const lack = lacking(send);
    if (lack) return setError(lack);
    setError('');
    if (send && !preview) return setPreview(true);
    setBusy(send ? 'Sending…' : 'Saving…');
    try {
      const out = await run<{ id: string; invite_error?: string }>('create_build', { template: template?.key, vendor: vendorId, emails, name: name.trim(), due, remind_every_days: Number(every), rows, ...(prefill ? { source: prefill.file, mapping: prefill.mapping, columns: prefill.columns, column_map: prefill.column_map } : {}), photos, build_type: buildType, send });
      moved();
      toast(out.invite_error ? `Saved, but the email did not go: ${out.invite_error}` : send ? `Sent “${name.trim()}” to ${known?.name ?? 'the vendor'}.` : `Saved “${name.trim()}” as a draft.`, out.invite_error ? 'error' : 'notice');
      go(`/builds/${out.id}`);
    } catch (e) { setError((e as Error).message); setBusy(''); setPreview(false); }
  };
  return (
    <Page title="New build">
      <Field label="Build name" autoFocus value={name} onChange={v => { setName(v); setError(''); }} placeholder="e.g. Fall 2026 upholstery" />
      <Combo label="Category" value={template?.key ?? ''} options={categories.map(t => ({ value: t.key, label: t.name, icon: categoryIcon(t.key, t.icon) }))}
        onChange={k => { setTemplateKey(k); setEvery(k.startsWith('made_to_order') ? '7' : '3'); if (vendorId && prefilled.current) setEmails(prefillFor(vendorId, k)); }} />
      <Combo label="Build type" value={buildType} options={BUILD_TYPES} onChange={setBuildType} />
      <Combo label="Vendor" placeholder="Pick a vendor, or type a new one" value={vendorId} options={vendors.map(v => ({ value: String(v.id), label: String(v.name), sub: v.country || undefined }))} onChange={pickVendor} onAdd={addVendor} />
      {vendorId && <Labeled label="Goes to"><Recipients emails={emails} none="No contact on file for this vendor yet: add one" onChange={next => { prefilled.current = false; setEmails(next); }} onCheck={why => { setError(why); if (!why) prefilled.current = false; }} /></Labeled>}
      <Labeled label="Products" aside={<>{products.length > 0 && <Note>{plural(products.length, 'product')}</Note>}<Button icon="plus" onClick={() => setImporting(true)}>Add products</Button></>}>
        <ProductTable rows={items} marks={alsoOnOf(items, elsewhere)} onChange={next => { setItems(next); setError(''); }} />
      </Labeled>
      <FormRow><Field label="Due" type="date" value={due} onChange={setDue} /><Combo label="Remind every" value={every} options={REMIND} onChange={setEvery} /></FormRow>
      <CheckField label="Ask for photographs" checked={photos} onChange={setPhotos} />
      {error && <Problem>{error}</Problem>}
      <div className="form-foot">
        <Button disabled={!!busy} onClick={() => finish(false)}>{busy && !preview ? busy : 'Save as draft'}</Button>
        <Button variant="primary" icon="send" disabled={!!busy} onClick={() => finish(true)}>Send to vendor</Button>
      </div>
      {importing && <ProductImport title="Add products" against={{ template: template?.key }} have={items} onDone={imported} onClose={() => setImporting(false)} />}
      {preview && <MailPreview ask={{ kind: 'invite', template: template?.key, vendor: vendorId, emails, name: name.trim(), due, remind_every_days: Number(every), rows }} send="Send it" busy={!!busy} onSend={() => finish(true)} onClose={() => setPreview(false)} />}
    </Page>
  );
}

export async function takeExport(action: string, id: string): Promise<string> {
  const out = await run<{ url: string; filename: string }>(action, { id });
  const m = /\/api\/file\/([^/]+)\/(.+)$/.exec(out.url);
  if (!m) throw new Error(`${action} answered an address that is not a file.`);
  await saveFile(m[1], out.filename, m[2]);
  return out.filename;
}
const NEXT: Record<string, Bulk> = { draft: 'send', sent: 'approve', in_progress: 'approve', review: 'approve' };
const PER_PAGE = 50;
const RECENT = 6;
type Product = Listed & { owes: string[]; img?: string; values: Row; defaults: string[] };
export const linkPath = (url: string) => new URL(url, location.origin).pathname;

export function BuildPage({ id }: { id: string }) {
  const read = async () => {
    const [build, rows, images, activity, spec] = await Promise.all([get('build', id), list('build_row', { build: id }), list('build_image', { build: id }), list('build_activity', { build: id }), run<Partial<Spec>>('buyer_build_spec', { id })]);
    const [template, vendor] = await Promise.all([build.template ? get('template', String(build.template)) : undefined, get('vendor', String(build.vendor))]);
    return { build, rows: rows.sort(byPosition), images: images.sort(byPosition), activity: activity.sort((a, b) => String(b.at).localeCompare(String(a.at))), spec: specOf(spec), template, vendor };
  };
  const { data, error, reload } = useRead(`build|${id}`, read, BOOK);
  const also = useRead(`elsewhere|${id}`, () => list('sku_elsewhere', { build: id }), ['build_row', 'build']);
  if (!data) return <PageState title="Build" error={error && /not found|404/i.test(error) ? 'No such build.' : error} />;
  return <BuildView b={data} also={also.data ?? []} reload={reload} />;
}

function BuildView({ b, also, reload }: { b: { build: Row; rows: Row[]; images: Row[]; activity: Row[]; spec: Spec; template?: Row; vendor: Row }; also: Row[]; reload: () => Promise<void> }) {
  const { go, swap } = useNav();
  const { build, spec } = b;
  const id = String(build.id);
  const link = useRead(`vendor-link|${build.vendor}`, () => run<{ url: string }>('vendor_link', { vendor: build.vendor }), ['vendor']);
  const [asking, setAsking] = useState<Bulk | 'photos' | null>(null);
  const { busy, run: write } = useWrite(() => Promise.all([reload(), link.reload()]), () => setAsking(null), moved);
  const [q, setQ] = useQuery('q');
  const [status, setStatus] = useQuery('status');
  const [pageWord, setPageWord] = useQuery('page');
  const page = Math.max(0, Number(pageWord) || 0);
  const setPage = (n: number) => setPageWord(n > 0 ? String(n) : '');
  const [dropping, setDropping] = useState<Product | null>(null);
  const [adding, setAdding] = useState(false);
  const [dropped, setDropped] = useState<File>();
  const [whole, setWhole] = useState(false);
  const [exporting, setExporting] = useState(false);
  const emails: string[] = Array.isArray(build.emails) ? build.emails : [];
  const open = isOpen(build);
  const next = NEXT[String(build.stage)];
  const mailed = asking === 'send';
  const asked = asking && asking !== 'photos' && !mailed && BULKS[asking].ask(1);
  const errors = b.rows.filter(r => issuesOf(r).some(i => i.level === 'error')).length;
  const verb = (k: Bulk) => write(async () => {
    const out = await run(BULKS[k].action, { id });
    const mail = out?.invite_error || out?.mail_error;
    if (k === 'delete') { toast(`Deleted “${build.name}”.`, 'notice'); swap('/builds'); return GONE; }
    const done = `${BULKS[k].done} “${build.name}”.${mail ? ` The email did not go: ${mail}` : ''}`;
    if (k !== 'approve') return done;
    try { return `${done} Exported ${await takeExport('export_build', id)}.`; } catch (e) { return `${done} The export did not run: ${(e as Error).message} Export it again from this page.`; }
  });
  const labels = new Map(spec.fields.map(f => [f.key, f.label]));
  const nameKey = nameKeyOf(spec);
  const hero = new Map(b.images.filter(i => i.build_row && i.role === 'full').map(i => [String(i.build_row), String(i.file)]));
  const untouched = build.stage === 'sent' || build.stage === 'draft' || (build.stage === 'cancelled' && !build.opened_at);
  const products: Product[] = b.rows.map(r => ({
    id: String(r.id), sku: String(r.sku ?? ''), name: show(nameKey ? r[nameKey] : '') || r.name || '', img: hero.get(String(r.id)),
    owes: issuesOf(r).filter(i => i.level === 'error' || i.field.startsWith('image:')).map(i => (i.field.startsWith('image:') ? `${sentence(i.field.slice(6))} photo` : labels.get(i.field) ?? sentence(i.field))),
    values: r, defaults: [...Object.keys(r.defaults ?? {}), ...Object.keys(r.lg_defaults ?? {})]
  }));
  const settable = build.stage !== 'complete' && build.stage !== 'cancelled';
  const fillColumn = (f: TField): Column<Product> => ({
    key: `f:${f.key}`, label: f.label, sort: r => r.values[f.key] ?? '',
    render: r => {
      const ruled = r.defaults.includes(f.key);
      const now = r.values[f.key] ?? '';
      if (!settable) return now || '—';
      return <span className="toolbar">
        <Picker quiet all={false} label={now || f.label} value={now} options={[...(ruled || !now ? [] : [{ value: '', label: 'By the rules' }]), ...(f.options ?? []).map(o => ({ value: o, label: o }))]}
          onChange={v => write(async () => { await patch('build_row', r.id, { [f.key]: v }); })} />
        {!ruled && now && <Tag quiet title="Set by the buyer, kept over the rules">Set</Tag>}
      </span>;
    }
  });
  const clean = products.filter(x => !x.owes.length).length;
  const found = products.filter(x => untouched || !status || (status === 'missing') === x.owes.length > 0).filter(x => !q || `${x.name} ${x.sku}`.toLowerCase().includes(q.toLowerCase()));
  const pages = Math.max(1, Math.ceil(found.length / PER_PAGE));
  const at = Math.min(page, pages - 1);
  const shown = found.slice(at * PER_PAGE, (at + 1) * PER_PAGE);
  const alsoOn = new Map<string, Row[]>();
  for (const e of also) { const k = listedKey({ sku: String(e.sku ?? ''), name: String(e.name ?? '') }); alsoOn.set(k, [...(alsoOn.get(k) ?? []), e]); }
  const closeAdd = () => { setAdding(false); setDropped(undefined); };
  const addProducts = ({ rows, sheet }: Brought) => write(async () => {
    const out = await run<{ added: string[] }>('edit_build_skus', { id, add: rows.map(asRow), remove: [] });
    closeAdd();
    return `Added ${plural(out.added?.length ?? rows.length, 'product')}${sheet ? ` from ${sheet.file}` : ''}.`;
  });
  const dropProduct = (x: Product) => write(async () => { setDropping(null); await run('edit_build_skus', { id, add: [], remove: [x.id] }); return `Dropped ${x.sku || x.name}.`; });
  const setEmails = (next: string[]) => write(async () => {
    const out = await run<{ added?: string[]; mail_error?: string }>('set_build_emails', { id, emails: next });
    if (out.mail_error) throw new Error(`Saved, but the email did not go: ${out.mail_error}`);
    return out.added?.length && build.stage !== 'draft' ? `Link emailed to ${out.added.join(', ')}.` : undefined;
  });
  const setPhotos = (photos: boolean) => write(async () => { await run('set_build_photos', { id, photos }); });
  const steps: [string, unknown][] = [['Sent', build.sent_at], ['Opened', build.opened_at], ['Submitted', build.submitted_at], [build.cancelled_at ? 'Cancelled' : 'Approved', build.cancelled_at ?? build.approved_at]];
  const reached = steps.reduce((n, x, i) => (x[1] ? i : n), -1);
  const share = link.data?.url || '';
  const facts: [string, ReactNode][] = [
    ['Category', b.template && <LinkButton onClick={() => go(`/templates/${b.template!.id}`)}>{marked(b.template.key, b.template.icon, b.template.name)}</LinkButton>],
    ['Vendor', <LinkButton onClick={() => go(`/vendors/${b.vendor.id}`)}>{b.vendor.name}</LinkButton>],
    ['Due', [show(build.due_at), build.remind_every_days ? `reminded every ${build.remind_every_days} days` : '', build.reminders_sent ? `${build.reminders_sent} sent` : ''].filter(Boolean).join(', ')],
    ['Photographs', open ? <CheckField label="Ask for photographs" checked={!!build.photos} onChange={v => (v || !b.images.length ? setPhotos(v) : setAsking('photos'))}>{build.photos ? 'Asked for' : 'Not asked for'}</CheckField> : build.photos ? 'Asked for' : 'Not asked for'],
    ['Build type', build.build_type], ['Created by', build.created_by_name]
  ];
  const exportNow = async () => {
    setExporting(true);
    try { toast(`Exported ${await takeExport('export_build', id)}.`, 'notice'); } catch (e) { toast(`Not exported: ${(e as Error).message}`); }
    setExporting(false);
  };
  const activity = whole ? b.activity : b.activity.slice(0, RECENT);
  return (
    <Page title={String(build.name ?? '')} tools={<Tag tone={stageTone(build)}>{standingOf(build)}</Tag>} aside={<Actions items={[
      ...(share ? [{ key: 'open', label: 'Open', icon: 'open' as const, onClick: () => window.open(`${linkPath(share)}${build.number ? `/${build.number}` : ''}?buyer=1`, '_blank', 'noopener') }] : []),
      ...(BULKS.cancel.applies(build) ? [{ key: 'cancel', label: 'Cancel', disabled: busy, onClick: () => setAsking('cancel') }] : []),
      ...(BULKS.delete.applies(build) ? [{ key: 'delete', label: 'Delete', disabled: busy, onClick: () => setAsking('delete') }] : []),
      ...(build.stage !== 'draft' ? [{ key: 'export', label: exporting ? 'Exporting…' : 'Export', icon: 'download' as const, disabled: exporting, onClick: exportNow }] : []),
      ...(next ? [{ key: 'next', label: BULKS[next].label, icon: BULKS[next].icon, primary: true, disabled: busy, onClick: () => setAsking(next) }] : [])
    ]} />}>
      {build.return_note && <Section title="Sent back with"><Note block>{build.return_note}</Note></Section>}
      <Stages stages={steps.map(([k, day]) => (day ? `${k} ${show(day)}` : k))} at={reached} />
      {also.length > 0 && <Section title={`${(n => (n === 1 ? '1 product is' : `${n} products are`))(alsoOn.size)} also on ${also.length === 1 ? 'another build' : 'other builds'}`}>
        <List items={also.map(e => ({ key: `${e.sku}|${e.name}|${e.other_build}`, left: <IconLabel icon="alert">{String(e.sku || e.name)}</IconLabel>, right: <LinkButton onClick={() => go(`/builds/${e.other_build}`)}>{e.other_name}</LinkButton> }))} />
      </Section>}
      <DropRegion enabled={open} label="Drop the line sheet to add its products" onFiles={f => { setDropped(f[0]); setAdding(true); }}>
        <Section>
          <Toolbar wrap>
            <SearchField placeholder="Search products" value={q} onChange={v => { setQ(v); setPage(0); }} />
            <Note>{found.length ? `${at * PER_PAGE + 1}–${Math.min((at + 1) * PER_PAGE, found.length)} of ${found.length}` : 'None match'}</Note>
            <IconButton icon="back" label="Previous" disabled={at === 0} onClick={() => setPage(at - 1)} />
            <IconButton icon="chevronRight" label="Next" disabled={at >= pages - 1} onClick={() => setPage(at + 1)} />
            {!untouched && <Picker label="Status" value={status} options={[{ value: 'missing', label: 'Needs fixing' }, { value: 'complete', label: 'Complete' }]} onChange={v => { setStatus(v); setPage(0); }} />}
            {open && <Button icon="plus" disabled={busy} onClick={() => setAdding(true)}>Add products</Button>}
          </Toolbar>
          {!shown.length && <Empty>{products.length ? 'No product matches.' : 'No products yet. Add some to start.'}</Empty>}
          {shown.length > 0 && <Table<Product> rows={shown} columns={[
            { key: 'p', label: q || (status && !untouched) ? `${found.length} of ${plural(products.length, 'product')}` : untouched ? plural(products.length, 'product') : `${clean} of ${products.length} complete`, fill: true, sort: r => r.name || r.sku,
              render: r => <IconLabel lead={<span className="thumb"><Pic file={r.img} /></span>}><Lines top={r.name || r.sku} sub={r.name ? r.sku || 'No SKU yet' : undefined} /></IconLabel> },
            ...spec.fields.filter(f => f.owner === 'lg' && f.fills?.length).map(fillColumn),
            ...(alsoOn.size ? [{ key: 'o', label: 'Also on', soft: true, render: (r: Product) => (alsoOn.get(listedKey(r)) ?? []).map(e => `build ${e.other_number}`).join(', ') }] : []),
            ...(open ? [{ key: 'x', label: '', num: true, render: (r: Product) => <IconButton reveal icon="close" label={`Drop ${r.sku || r.name}`} onClick={() => setDropping(r)} /> }] : []),
            ...(untouched ? [] : [{ key: 's', label: 'Status', num: true, sort: (r: Product) => r.owes.length, render: (r: Product) => (r.owes.length ? <Tag tone="warning" title={r.owes.join(', ')}>{r.owes.length} missing</Tag> : <Tag tone="success">Complete</Tag>) }])
          ]} />}
        </Section>
      </DropRegion>
      <Section title={build.stage === 'draft' ? 'Goes to' : 'Sent to'}><Recipients emails={emails} onChange={setEmails} onCheck={why => toast(why)} busy={busy} locked={!open} none="Nobody gets this build’s emails yet: add an address" /></Section>
      <Section title="Details"><List items={facts.filter(f => f[1]).map(([k, v]) => ({ key: k, left: k, right: v }))} /></Section>
      {b.activity.length > 0 && <Section title="Activity">
        <Log lines={activity.map(a => ({ key: String(a.id), when: show(a.at), text: a.text }))} />
        {b.activity.length > RECENT && <div><Button variant="quiet" onClick={() => setWhole(!whole)}>{whole ? 'Show less' : `Show all ${b.activity.length}`}</Button></div>}
      </Section>}
      {asked && <Confirm title={asked[0].replace('1 build', `“${build.name}”`)} busy={busy} onYes={() => verb(asking as Bulk)} onClose={() => setAsking(null)} yes={busy ? 'Working…' : asking === 'approve' && errors ? 'Approve anyway' : asked[2].replace(/builds$/, 'build')}
        note={asking === 'approve' ? `${errors ? `${plural(errors, 'product')} still ${errors === 1 ? 'has' : 'have'} open issues. ` : ''}Its products are saved and the vendor is thanked by email. The workbook downloads as soon as it is approved.` : asked[1]} />}
      {asking === 'photos' && <Confirm title="Stop asking for photographs?" note={`The vendor has already sent ${plural(b.images.length, 'photograph')}. They are kept, and no longer owed.`} no="Keep asking" yes="Stop asking" busy={busy} onYes={() => setPhotos(false)} onClose={() => setAsking(null)} />}
      {mailed && <MailPreview ask={{ kind: 'invite', build: id }} send="Send it" busy={busy} onSend={() => verb('send')} onClose={() => setAsking(null)} />}
      {adding && <ProductImport title="Add products to this build" against={{ build: id }} have={products} build={id} file={dropped} busy={busy} onDone={addProducts} onClose={closeAdd} />}
      {dropping && <Confirm title={`Drop ${dropping.sku || dropping.name}?`} note="The product leaves the build, with whatever the vendor entered for it and its photographs." no="Keep it" yes="Drop it" busy={busy} onYes={() => dropProduct(dropping)} onClose={() => setDropping(null)} />}
    </Page>
  );
}
