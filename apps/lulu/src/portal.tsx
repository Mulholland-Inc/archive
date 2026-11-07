import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { boot, list, moved, patch, remove, run, saveFile, setBearer, upload, uploadMany, useQuery, useRead, type Row } from './api';
import { Actions, Button, Confirm, Dialog, DropRegion, DropZone, FilePicker, Icon, IconButton, Lines, Loading, Meter, Note, Page, Section, ShellCtx, Table, Tabs, Toasts, toast } from './ui';
import { MAX_FILE, angleRank, byPosition, categoryIcon, fixing, idle, isOpen, issuesOf, LISTED_BY, nameKeyOf, plural, sentence, show, SKU_KEY, vendorFields, whenOf, late, tooBig, type Spec } from './model';
import { PhotoGrid, PhotoTray, Sheet, cellId, type PhotoRow, type SheetChange, type SheetRow, type TrayPic } from './sheet';

const TABLES = ['build', 'build_row', 'build_image'];
const specOf = (s: Partial<Spec>): Spec => ({ fields: s.fields ?? [], images: s.images ?? [], photo: s.photo ?? null });

function ThemeSwitch() {
  const [dark, setDark] = useState(() => document.documentElement.dataset.theme === 'dark');
  const flip = () => { const next = dark ? 'light' : 'dark'; document.documentElement.dataset.theme = next; localStorage.setItem('look', next); setDark(!dark); };
  return <IconButton icon={dark ? 'sun' : 'moon'} label={dark ? 'Light' : 'Dark'} onClick={flip} />;
}
const route = () => {
  const [, token = '', number = ''] = location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
  return { token, number };
};
const greeting = (name: string) => {
  const h = Number(new Intl.DateTimeFormat('en-US', { timeZone: 'America/Los_Angeles', hour: 'numeric', hour12: false }).format(new Date()));
  return `${h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'}, ${name}.`;
};
const Front = ({ brand, children }: { brand: string; children: ReactNode }) => (
  <main className="front"><span className="front__corner"><ThemeSwitch /></span><div className="front__inner"><h1 className="front__title">{brand}</h1>{children}</div></main>
);

export function VendorPortal() {
  const [at, setAt] = useState(() => { const r = route(); setBearer(r.token); return r; });
  useEffect(() => {
    const on = () => setAt(route());
    addEventListener('popstate', on);
    return () => removeEventListener('popstate', on);
  }, []);
  const who = useRead(`portal|${at.token}`, () => boot(), []);
  const brand = String(who.data?.me?.workspaces?.[0]?.name ?? '');
  useEffect(() => { if (brand) document.title = brand; }, [brand]);
  const go = (number?: unknown) => { history.pushState(null, '', `/v/${encodeURIComponent(at.token)}${number ? `/${number}` : ''}${location.search.includes('buyer=1') ? '?buyer=1' : ''}`); setAt(route()); };
  if (!at.token || (!who.data && who.error)) return <Front brand=""><Note block>This link is not active. Ask the person who sent it for a new one.</Note></Front>;
  if (!who.data) return <Front brand=""><Loading /></Front>;
  return <>
    {at.number ? <PortalBuild key={at.number} number={at.number} brand={brand} home={() => go()} /> : <PortalHome who={who.data} brand={brand} open={go} />}
    <Toasts />
  </>;
}

function PortalHome({ who, brand, open }: { who: Row; brand: string; open: (n: unknown) => void }) {
  const { data, error } = useRead('portal-builds', () => list('portal_book').then(all => all.filter(b => !idle(b)).sort((a, b) => rank(a) - rank(b))), TABLES);
  if (!data) return <Front brand={brand}>{error ? <Note block>{error}</Note> : <Loading />}</Front>;
  const opened = data.filter(isOpen);
  const left = opened.reduce((n, r) => n + (r.skus ?? 0) - (r.clean ?? 0), 0);
  return (
    <Front brand={brand}>
      <Note block>{greeting(String(who.me?.actor?.name || 'there'))} {opened.length ? `You have ${plural(opened.length, 'build')} open with ${plural(left, 'product')} still to fill in.` : 'Nothing is waiting on you.'}</Note>
      {data.length > 0 && <Table<Row> rows={data} onRow={r => open(r.number)} columns={[
        { key: 'n', label: 'Build', fill: true, render: r => <span className="row-icon row-icon--tall"><Icon name={categoryIcon(r.category, r.category_icon)} size="lg" /><Lines top={r.name} sub={r.category_name} /></span> },
        { key: 'data', label: 'Data', num: true, render: r => `${r.data_clean}/${r.skus ?? 0}` },
        { key: 'photos', label: 'Photos', num: true, render: r => (r.wants_photos ? `${r.photos_clean}/${r.skus ?? 0}` : '') },
        { key: 'd', label: 'When', num: true, render: r => <Note danger={late(r)}>{whenOf(r)}</Note> }
      ]} />}
      <Note block>Stuck on a build? Reply to the email, and someone answers around the clock.</Note>
    </Front>
  );
}
const rank = (r: Row) => (r.stage === 'sent' || r.stage === 'in_progress' ? 0 : r.stage === 'review' ? 1 : 2);

function PortalBuild({ number, brand, home }: { number: string; brand: string; home: () => void }) {
  const read = async () => {
    const build = (await list('build', { number }))[0];
    if (!build) return null;
    const id = String(build.id);
    const [spec, rows, images] = await Promise.all([run<Partial<Spec>>('build_spec', { id }), list('build_row', { build: id }), list('build_image', { build: id })]);
    return { build, rows: rows.sort(byPosition), images: images.sort(byPosition), spec: specOf(spec) };
  };
  const { data, error } = useRead(`portal-build|${number}`, read, TABLES);
  if (data === null) return <Front brand={brand}><Note block>This link is not active. Ask the person who sent it for a new one.</Note></Front>;
  if (!data) return error ? <Front brand={brand}><Note block>{error}</Note></Front> : <Front brand={brand}><Loading /></Front>;
  return <ShellCtx.Provider value={{ back: home }}><PortalSheet b={data} brand={brand} /></ShellCtx.Provider>;
}
type Built = { build: Row; rows: Row[]; images: Row[]; spec: Spec };

function useEdits() {
  const [over, setOver] = useState<Map<string, string>>(new Map());
  const [saving, setSaving] = useState<'idle' | 'saving' | 'saved'>('idle');
  const queue = useRef(new Map<string, SheetChange>());
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typed = useRef(new Map<string, number>());
  const landed = useRef(new Map<string, number>());
  const at = (c: SheetChange) => cellId(c.row, c.key);
  const flush = useCallback(async () => {
    timer.current = null;
    const changes = [...queue.current.values()];
    queue.current = new Map();
    if (!changes.length) return;
    setSaving('saving');
    const byRow = new Map<string, SheetChange[]>();
    for (const c of changes) byRow.set(c.row, [...(byRow.get(c.row) ?? []), c]);
    const told = await Promise.allSettled([...byRow].map(([id, cs]) => patch('build_row', id, Object.fromEntries(cs.map(c => [c.key, c.value])))));
    const refused: SheetChange[] = [];
    [...byRow.values()].forEach((cs, i) => {
      const t = told[i];
      if (t.status === 'fulfilled') for (const c of cs) landed.current.set(at(c), typed.current.get(at(c)) ?? 0);
      else { refused.push(...cs); toast(`Not saved: ${(t.reason as Error).message}`); }
    });
    if (refused.length) setOver(was => { const n = new Map(was); for (const c of refused) if (!queue.current.has(at(c))) n.delete(at(c)); return n; });
    setSaving(refused.length === changes.length ? 'idle' : 'saved');
    moved();
  }, []);
  const change = useCallback((changes: SheetChange[]) => {
    setOver(was => { const next = new Map(was); for (const c of changes) next.set(at(c), c.value); return next; });
    for (const c of changes) { queue.current.set(at(c), c); typed.current.set(at(c), (typed.current.get(at(c)) ?? 0) + 1); }
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => void flush(), 400);
  }, [flush]);
  const settle = useCallback(() => setOver(was => {
    if (!was.size) return was;
    const next = new Map(was);
    for (const k of was.keys()) {
      const n = landed.current.get(k);
      if (n !== undefined && n === typed.current.get(k) && !queue.current.has(k)) { next.delete(k); landed.current.delete(k); }
    }
    return next.size === was.size ? was : next;
  }), []);
  useEffect(() => () => { if (timer.current) { clearTimeout(timer.current); void flush(); } }, [flush]);
  return { over, change, settle, saving };
}

const seededRow = (r: Row, keys: string[], over: Map<string, string>): SheetRow => {
  const values: Record<string, string> = {};
  for (const k of keys) values[k] = r[k] == null ? '' : String(r[k]);
  for (const [at, v] of over) { const [id, key] = at.split('\u0000'); if (id === String(r.id)) values[key] = v; }
  const seeded = (r.seeded ?? {}) as Row;
  return { id: String(r.id), sku: String(r.sku || r.name || '—'), values, issues: issuesOf(r), defaults: Object.keys(r.defaults ?? {}), locked: LISTED_BY.filter(k => String(seeded[k] ?? '').trim()) };
};

function photosOf({ build, rows, images, spec }: Built) {
  const roles = build.photos ? [...new Set([...spec.images.map(i => i.role), ...images.filter(i => i.build_row && i.role).map(i => String(i.role))])].sort((a, b) => angleRank(a) - angleRank(b)) : [];
  const nameKey = nameKeyOf(spec);
  const held = new Map<string, Row[]>();
  for (const i of images) if (i.build_row && i.role) held.set(`${i.build_row}|${i.role}`, [...(held.get(`${i.build_row}|${i.role}`) ?? []), i]);
  const grid: PhotoRow[] = rows.map(r => {
    const issues = issuesOf(r);
    const slots: PhotoRow['slots'] = {};
    for (const role of roles) {
      const said = issues.find(i => i.field === `image:${role}`);
      slots[role] = {
        pics: (held.get(`${r.id}|${role}`) ?? []).map(i => { const bad = issues.find(x => x.field === `photo:${i.id}`); return { id: String(i.id), file: String(i.file), issue: bad && { level: bad.level, message: bad.message ?? '' } }; }),
        owed: said ? Number(/(\d+) more/.exec(said.message ?? '')?.[1] ?? 1) : 0
      };
    }
    return { id: String(r.id), name: show(nameKey ? r[nameKey] : '') || r.name || '—', sku: r.sku || 'No SKU yet', slots };
  });
  const pool: TrayPic[] = [...images.filter(i => !i.build_row), ...images.filter(i => i.build_row)].map(i => ({ id: String(i.id), name: String(i.name ?? 'Photograph'), file: String(i.file), placed: !!i.build_row }));
  return { roles: roles.map(r => ({ key: r, label: r === 'three_quarter' ? '¾' : sentence(r) })), grid, pool };
}

function PortalSheet({ b, brand }: { b: Built; brand: string }) {
  const [viewWord, setView] = useQuery('show', 'sheet');
  const view = viewWord as 'sheet' | 'photos';
  const [missing, setMissing] = useState(false);
  const [sending, setSending] = useState<{ sent: number; total: number } | null>(null);
  const [resetting, setResetting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [fetching, setFetching] = useState(false);
  const [picked, setPicked] = useState<string | null>(null);
  const [assigning, setAssigning] = useState(false);
  const picker = useRef<HTMLInputElement>(null);
  const { build, spec } = b;
  const id = String(build.id);
  const locked = build.stage === 'complete' || build.stage === 'cancelled';
  const edits = useEdits();
  const fields = useMemo(() => vendorFields(spec), [spec]);
  const keys = useMemo(() => [...fields.map(f => f.key)], [fields]);
  const nameKey = nameKeyOf(spec);
  useEffect(() => edits.settle(), [b.rows]);
  const rows = useMemo(() => b.rows.map(r => seededRow(r, keys, edits.over)), [b.rows, keys, edits.over]);
  const pending = useMemo(() => new Set(edits.over.keys()), [edits.over]);
  useEffect(() => { document.title = String(build.name ?? ''); }, [build.name]);
  useEffect(() => {
    if (location.search.includes('buyer=1')) return;
    const kinds = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const;
    const touched = () => { kinds.forEach(k => window.removeEventListener(k, touched)); run('open_build', { id }).catch((e: Error) => toast(e.message)); };
    kinds.forEach(k => window.addEventListener(k, touched, { passive: true }));
    return () => kinds.forEach(k => window.removeEventListener(k, touched));
  }, [id]);
  const write = async (what: () => Promise<unknown>, told?: string) => {
    try { await what(); if (told) toast(told, 'notice'); } catch (e) { toast((e as Error).message); }
    moved();
  };
  const place = (photo: string, row: string, role: string) => { setPicked(null); void write(() => patch('build_image', photo, { build_row: row, role })); };
  const sendPhotos = async (files: File[]) => {
    const pictures = files.filter(f => /^(image|video)\//.test(f.type) || /\.(jpe?g|png|webp|gif|heic|tiff?|mp4|mov)$/i.test(f.name));
    if (!pictures.length) return toast('None of those are photographs.');
    for (const f of pictures) if (f.size > MAX_FILE) toast(`${f.name} is larger than 25 MB, the most one photo can be. Export it smaller and drop it again.`);
    const shots = pictures.filter(f => f.size <= MAX_FILE);
    if (!shots.length) return;
    try {
      setSending({ sent: 0, total: shots.length });
      const up = await uploadMany(shots, (sent, total) => setSending({ sent, total }));
      setSending(null);
      if (up.stored.length) {
        const out = await run<{ uploaded?: number; matched?: number; skipped?: string[] }>('attach_photos', { id, photos: up.stored.map(f => ({ file: f.id, name: f.name })) });
        toast(`Took ${plural(out?.uploaded ?? up.stored.length, 'photo')}${out?.matched ? `, ${out.matched} placed by their file names` : ''}.`, 'notice');
        if (out?.skipped?.length) toast(`Not photographs: ${out.skipped.join(', ')}.`);
      }
      for (const r of up.refused) toast(`${r.name}: ${r.error}`);
    } catch (e) { setSending(null); toast((e as Error).message); }
    moved();
  };
  const download = async () => {
    setFetching(true);
    try { const t = await run<{ file: string; name: string }>('blank_build', { id }); await saveFile(t.file, t.name); } catch (e) { toast(`The template could not be made just now: ${(e as Error).message}. Try again in a minute.`); }
    setFetching(false);
  };
  const owesPhoto = b.rows.some(r => issuesOf(r).some(i => i.field.startsWith('image:')));
  const canAssign = owesPhoto && b.images.some(i => !i.build_row);
  const assign = async () => {
    setAssigning(true);
    await write(async () => { const out = await run<{ matched?: number }>('sort_photos', { id }); toast(out?.matched ? `Placed ${plural(out.matched, 'photo')}.` : 'No photo could be placed by its file name.', 'notice'); });
    setAssigning(false);
  };
  const waiting = build.stage === 'in_progress' && !!build.returned_at;
  const ready = (build.skus ?? 0) > 0 && build.clean === build.skus;
  const s = photosOf(b);
  const done = build.skus ? (build.clean ?? 0) / build.skus : 0;
  return (
    <div className="portal">
      <Page flush title={String(build.name ?? '')} tools={<>
        {build.photos && <Tabs value={view} onChange={setView} tabs={[{ id: 'sheet', label: 'Sheet', icon: 'table' }, { id: 'photos', label: 'Photos', icon: 'camera' }]} />}
        <Button icon="alert" pressed={missing} onClick={() => setMissing(!missing)}>Needs fixing</Button>
        {edits.saving !== 'idle' && <Note>{edits.saving === 'saving' ? 'Saving…' : 'All changes saved'}</Note>}
      </>} aside={<Actions items={locked ? [] : [
        ...(view === 'photos' ? [
          { key: 'photos', label: sending ? `Uploading ${sending.sent} of ${sending.total}` : 'Upload photos', icon: 'upload' as const, disabled: !!sending, onClick: () => picker.current?.click() },
          { key: 'assign', label: assigning ? 'Assigning…' : 'Auto-assign', disabled: !!sending || assigning || !canAssign, onClick: assign }
        ] : [
          { key: 'download', label: fetching ? 'Downloading…' : 'Download template', icon: 'download' as const, disabled: fetching, onClick: download },
          { key: 'upload', label: 'Upload sheet', icon: 'upload' as const, onClick: () => setUploading(true) }
        ]),
        { key: 'reset', label: 'Reset', onClick: () => setResetting(true) },
        ...(waiting ? [{ key: 'submit', label: 'Submit', icon: 'send' as const, primary: true, disabled: !ready, onClick: () => write(() => run('submit_build', { id }), `Submitted. ${brand} will review it.`) }] : [])
      ]} after={<><Meter value={done} label={`${build.clean ?? 0} of ${build.skus ?? 0} products, ${whenOf(build)}`} /><ThemeSwitch /></>} />}>
        <FilePicker inputRef={picker} multiple accept="image/*,video/*" onFiles={sendPhotos} />
        {locked && <Note block>{build.stage === 'complete' ? 'This build is approved. Thank you, nothing more is needed.' : 'This build was withdrawn. Nothing you entered is lost.'}</Note>}
        {build.return_note && <Section title={`A note from ${brand}`} action={waiting ? undefined : <Button variant="quiet" onClick={() => write(() => run('dismiss_note', { id }))}>Dismiss</Button>}><Note block>{build.return_note}</Note></Section>}
        {waiting && <Note block>{ready ? `Every product is complete. Press Submit when you have made the changes asked for, and ${brand} will review it.` : 'Submit comes on once every product is complete.'}</Note>}
        {view === 'photos'
          ? <DropRegion enabled={!locked} onFiles={sendPhotos}>
              <PhotoTray pics={s.pool} picked={locked ? null : picked} onPick={locked ? undefined : setPicked} onFiles={locked ? undefined : sendPhotos} onRemove={locked ? undefined : img => write(() => remove('build_image', img))}>
                <PhotoGrid roles={s.roles} rows={s.grid} missingOnly={missing} picking={locked ? null : picked} onPlace={locked ? undefined : place} onClear={locked ? undefined : img => write(() => patch('build_image', img, { build_row: null, role: '' }))} />
              </PhotoTray>
            </DropRegion>
          : <Sheet editable={!locked} missingOnly={missing} fields={fields} nameKey={nameKey} skuKey={spec.fields.some(f => f.key === SKU_KEY) ? SKU_KEY : undefined} rows={rows} pending={pending} onChange={edits.change} />}
      </Page>
      {uploading && <UploadSheet id={id} onClose={() => setUploading(false)} />}
      {resetting && <Confirm title="Start this build over?" note="Every value and photograph entered on this build is cleared. The products stay on it." no="Keep it" yes="Start over"
        onYes={() => { setResetting(false); void write(() => run('reset_build', { id }), 'Everything entered is cleared. The SKUs stay.'); }} onClose={() => setResetting(false)} />}
    </div>
  );
}

type Question = { id: string; kind: string; ask: string; raw: string; field: string; guess: string; options: { value: string; label: string }[]; why?: string };
type AutoRun = { source: string; rows: Row[]; questions: Question[]; notice?: string; dropped?: string[] };
const askKey = (q: Question) => `${q.kind} ${q.field} ${q.raw}`;
type Step = { at: 'drop' } | { at: 'working'; file: string; said: string } | { at: 'asking'; file: string; run: AutoRun };

function UploadSheet({ id, onClose }: { id: string; onClose: () => void }) {
  const [step, setStep] = useState<Step>({ at: 'drop' });
  const [picked, setPicked] = useState<Record<string, string>>({});
  const confirm = async (file: string, got: AutoRun, answers: Record<string, string>) => {
    setStep({ at: 'working', file, said: 'Putting it on the sheet…' });
    try {
      const done = await run<{ notice?: string }>('auto_confirm', { id, source: got.source, rows: got.rows, answers: got.questions.map(q => ({ id: q.id, kind: q.kind, field: q.field, raw: q.raw, value: answers[askKey(q)] ?? q.guess })) });
      if (done?.notice) toast(done.notice, 'notice');
      moved();
      onClose();
      const f = fixing(await list('build_row', { build: id }));
      toast(f.values ? `${f.words}. Needs fixing shows them.` : 'Every value from the sheet is in.', f.values ? 'error' : 'notice');
    } catch (e) { toast((e as Error).message); setStep({ at: 'asking', file, run: got }); }
  };
  const onFile = async (f: File) => {
    if (tooBig(f)) return toast(tooBig(f));
    try {
      setStep({ at: 'working', file: f.name, said: 'Sending your sheet…' });
      const up = await upload(f);
      setStep({ at: 'working', file: f.name, said: 'Reading your columns…' });
      const out = await run<AutoRun>('auto_import', { id, file: up.id });
      const got = { ...out, rows: out.rows ?? [], questions: out.questions ?? [] };
      if (got.dropped?.length) toast(`Skipped ${plural(got.dropped.length, 'row')} for products not on this build: ${got.dropped.join('; ')}.`);
      if (!got.questions.length) return confirm(f.name, got, {});
      setPicked(Object.fromEntries(got.questions.map(q => [askKey(q), q.guess])));
      setStep({ at: 'asking', file: f.name, run: got });
    } catch (e) { toast((e as Error).message); setStep({ at: 'drop' }); }
  };
  const cancel = <Button onClick={onClose}>Cancel</Button>;
  if (step.at === 'drop') return <Dialog title="Upload sheet" onClose={onClose} foot={cancel}><DropZone title="Drop your sheet here, or click to choose one" hint="The template you downloaded, or your own Excel or CSV in your own words, up to 25 MB" accept=".xlsx,.xls,.csv,.tsv" onFile={onFile} /></Dialog>;
  if (step.at === 'working') return <Dialog title={step.file} onClose={onClose} foot={cancel}><Note block>{step.said}</Note></Dialog>;
  const got = step.run;
  return (
    <Dialog title={step.file} onClose={onClose} foot={<>{cancel}<Button variant="primary" icon="check" onClick={() => confirm(step.file, got, picked)}>Confirm</Button></>}>
      {got.notice && <Note block>{got.notice}</Note>}
      {got.questions.map(q => (
        <div key={askKey(q)} className="choice">
          <span>{q.ask}</span>{q.why && <Note block>{q.why}</Note>}
          <div className="toolbar toolbar--wrap">{q.options.map(o => <Button key={o.value || 'none'} pressed={o.value === (picked[askKey(q)] ?? q.guess)} onClick={() => setPicked({ ...picked, [askKey(q)]: o.value })}>{o.label}</Button>)}</div>
        </div>
      ))}
    </Dialog>
  );
}
