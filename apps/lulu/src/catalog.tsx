import { Fragment, useState, type ReactNode } from 'react';
import { create, fileUrl, get, list, moved, patch, remove, run, useQuery, useRead, type Row } from './api';
import { Actions, Button, Confirm, CopyButton, Dialog, Empty, Field, FormCard, FormLine, GONE, IconButton, Lines, List, Note, Page, PageState, Pic, Picker, Problem, SearchField, Section, Spacer, Table, Tag, Toolbar, toast, useWrite, type Item } from './ui';
import { ANGLES, angleRank, idle, issuesOf, listedKey, plural, sentence, show, useNav, whenOf, type TField } from './model';
import { COMPLETE, STANDING, bookRead, choices, eachRow, isEmail, linkPath, marked, notEmail, specOf, takeExport } from './builds';

const BOOK = ['build', 'build_row', 'build_image', 'vendor', 'template'];
const VENDORS = ['vendor', 'build'];

export function Products() {
  const { go } = useNav();
  const { data, error } = useRead('build_sku', () => list('build_sku'), ['build_row', 'build', 'build_image']);
  const books = useRead('build_book', bookRead, BOOK);
  const [sku, setSku] = useQuery('sku');
  const [name, setName] = useQuery('name');
  const [build, setBuild] = useQuery('build');
  const [vendor, setVendor] = useQuery('vendor');
  const [category, setCategory] = useQuery('category');
  const [status, setStatus] = useQuery('status');
  if (!data || !books.data) return <PageState title="Products" error={error ?? books.error} />;
  const bookOf = new Map(books.data.map(b => [String(b.id), b]));
  const all: Row[] = data.map(r => {
    const b = bookOf.get(String(r.build));
    return { ...r, category: b?.category, category_name: b?.category_name, standing: !b || idle(b) ? '' : (r.owes as string[]).length ? 'missing' : 'complete' };
  });
  const has = (word: unknown, part: string) => String(word ?? '').toLowerCase().includes(part.trim().toLowerCase());
  const filtered = !!(sku || name || build || vendor || category || status);
  const clear = () => { setSku(''); setName(''); setBuild(''); setVendor(''); setCategory(''); setStatus(''); };
  const shown = all.filter(r => has(r.sku, sku) && has(r.product, name)).filter(r => !build || String(r.build) === build).filter(r => !vendor || r.vendor === vendor).filter(r => !category || String(r.category) === category).filter(r => !status || r.standing === status)
    .sort((a, b) => Number(b.build_number) - Number(a.build_number) || Number(a.position) - Number(b.position));
  return (
    <Page title="Products" list wide>
      <Toolbar><SearchField placeholder="Search by SKU" value={sku} onChange={setSku} /><SearchField placeholder="Search by name" value={name} onChange={setName} /></Toolbar>
      <Toolbar wrap>
        <Picker search label="Build" value={build} options={choices(all, 'build', 'build_name')} onChange={setBuild} />
        <Picker search label="Vendor" value={vendor} options={choices(all, 'vendor')} onChange={setVendor} />
        <Picker label="Category" value={category} options={choices(all, 'category', 'category_name')} onChange={setCategory} />
        <Picker label="Status" value={status} options={[{ value: 'missing', label: 'Needs fixing' }, { value: 'complete', label: 'Complete' }]} onChange={setStatus} />
        <Spacer /><Note>{plural(shown.length, 'product')}</Note>
      </Toolbar>
      {shown.length ? <Table<Row> rows={shown} onRow={r => go(`/products/${r.id}`)} columns={[
        { key: 'product', label: 'Product', fill: true, render: r => r.product },
        { key: 'sku', label: 'SKU', render: r => r.sku },
        { key: 'build_name', label: 'Build', soft: true, render: r => r.build_name },
        { key: 'vendor', label: 'Vendor', soft: true, render: r => r.vendor },
        { key: 'status', label: 'Status', soft: true, num: true, sort: r => (r.owes as string[]).length, render: r => (r.standing === 'missing' ? <Tag tone="warning" title={(r.owes as string[]).join(', ')}>{(r.owes as string[]).length} missing</Tag> : r.standing === 'complete' ? <Tag tone="success">Complete</Tag> : '') }
      ]} /> : <Toolbar><Note>{filtered ? 'No product matches these filters.' : 'No products yet.'}</Note>{filtered && <Button onClick={clear}>Clear filters</Button>}</Toolbar>}
    </Page>
  );
}

export function ProductPage({ id }: { id: string }) {
  const { go } = useNav();
  const read = async () => {
    const row = await get('build_row', id);
    const build = String(row.build);
    const [images, book, spec, elsewhere] = await Promise.all([list('build_image', { build_row: id }), list('build_book'), run('buyer_build_spec', { id: build }), list('sku_elsewhere', { build })]);
    return { row, images, book: book.find(b => b.id === build), fields: specOf(spec).fields, elsewhere };
  };
  const { data, error } = useRead(`product|${id}`, read, ['build', 'build_row', 'build_image']);
  const [taking, setTaking] = useState(false);
  if (!data) return <PageState title="Product" error={error && /not found|404/i.test(error) ? 'No such product.' : error} />;
  const { row, images, book, fields, elsewhere } = data;
  const name = String(row.name || row.sku || 'Product');
  const labels = new Map(fields.map((f: TField) => [f.key, f.label]));
  const issues = issuesOf(row);
  const labelOf = (field: string) => (field.startsWith('image:') ? `${sentence(field.slice(6))} photo` : labels.get(field) ?? sentence(field));
  const owes = issues.filter(i => i.level === 'error');
  const notes = issues.filter(i => i.level !== 'error');
  const standing = !book || idle(book) ? null : owes.length ? <Tag tone="warning">{owes.length} missing</Tag> : <Tag tone="success">Complete</Tag>;
  const sections = [...new Set(fields.map((f: TField) => f.section ?? ''))].map(title => ({ title, items: fields.filter((f: TField) => (f.section ?? '') === title && show(row[f.key])) })).filter(s => s.items.length);
  const pics = [...images].sort((a, b) => angleRank(String(a.role)) - angleRank(String(b.role)) || Number(a.position) - Number(b.position));
  const sameKey = listedKey({ sku: String(row.sku ?? ''), name: String(row.name ?? '') });
  const also = elsewhere.filter(e => listedKey({ sku: String(e.sku ?? ''), name: String(e.name ?? '') }) === sameKey);
  const take = async () => {
    setTaking(true);
    try { toast(`Exported ${await takeExport('export_product', id)}.`, 'notice'); } catch (e) { toast((e as Error).message); }
    setTaking(false);
  };
  const details: Item[] = [
    ...(row.sku ? [{ key: 'sku', left: 'SKU', right: <span className="list__end">{String(row.sku)}<CopyButton value={String(row.sku)} label="Copy SKU" /></span> }] : []),
    { key: 'build', left: 'Build', right: String(book?.name ?? ''), onOpen: () => go(`/builds/${row.build}`) },
    ...(book?.vendor ? [{ key: 'vendor', left: 'Vendor', right: String(book.vendor), onOpen: () => go(`/vendors/${book.vendor_id}`) }] : []),
    ...(book?.category_name ? [{ key: 'category', left: 'Category', right: marked(book.category, book.category_icon, String(book.category_name)) }] : [])
  ];
  const issueItems = (all: typeof issues): Item[] => all.map((i, n) => ({ key: `${i.field}${n}`, left: labelOf(i.field), below: i.message }));
  return (
    <Page title={name} tools={standing} aside={<Button variant="primary" icon="download" disabled={taking} onClick={take}>{taking ? 'Exporting…' : 'Export'}</Button>}>
      <Section><List items={details} /></Section>
      {owes.length > 0 && <Section title="Still owed"><List items={issueItems(owes)} /></Section>}
      {notes.length > 0 && <Section title="Worth a look"><List items={issueItems(notes)} /></Section>}
      {also.length > 0 && <Section title="Also on"><List items={also.map(e => ({ key: String(e.other_build), left: String(e.other_name), right: String(e.other_vendor), onOpen: () => go(`/builds/${e.other_build}`) }))} /></Section>}
      {pics.length > 0 && <Section title="Photos"><div className="photos">{pics.map(p => (
        <a key={String(p.id)} className="photos__item" href="#" onClick={e => { e.preventDefault(); void fileUrl(String(p.file), 'content').then(u => window.open(u, '_blank', 'noopener')); }}>
          <Pic file={String(p.file)} alt={String(p.role ?? '')} /><span>{ANGLES.find(a => a.key === p.role)?.label ?? sentence(String(p.role ?? 'Photo'))}</span>
        </a>
      ))}</div></Section>}
      {sections.map(s => <Section key={s.title} title={s.title}><List items={s.items.map((f: TField) => { const said = show(row[f.key]); return said.length > 80 ? { key: f.key, left: f.label, below: said } : { key: f.key, left: f.label, right: said }; })} /></Section>)}
      {!sections.length && <Note block>Nothing filled in yet.</Note>}
    </Page>
  );
}

export function Templates() {
  const { go } = useNav();
  const { data, error } = useRead('template_book', () => list('template_book'), ['template', 'build']);
  const [q, setQ] = useQuery('q');
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState('');
  const [problem, setProblem] = useState('');
  if (!data) return <PageState title="Templates" error={error} />;
  const make = async () => {
    if (!name.trim()) return;
    setProblem('');
    try { const t = await create('template', { name: name.trim() }); moved(); go(`/templates/${t.id}`); } catch (e) { setProblem((e as Error).message); }
  };
  const rows = data.filter(r => !q || `${r.name} ${r.parent_name}`.toLowerCase().includes(q.toLowerCase())).sort((a, b) => String(a.name).localeCompare(String(b.name)));
  return (
    <Page title="Templates" list>
      <Toolbar><SearchField placeholder="Search templates" value={q} onChange={setQ} /><Button variant="primary" icon="plus" onClick={() => { setName(''); setAdding(true); }}>New template</Button></Toolbar>
      {rows.length ? <Table<Row> rows={rows} onRow={r => go(`/templates/${r.id}`)} columns={[
        { key: 'name', label: 'Template', fill: true, sort: r => r.name, render: r => marked(r.key, r.icon, String(r.name ?? '')) },
        { key: 'parent', label: 'Parent', soft: true, render: r => r.parent_name ?? '' },
        { key: 'open', label: 'Open builds', soft: true, num: true, render: r => r.open_builds || '' }
      ]} /> : <Toolbar><Note>{q ? 'No template matches.' : 'No templates yet.'}</Note></Toolbar>}
      {adding && (
        <Dialog title="New template" onClose={() => setAdding(false)} foot={<><Button onClick={() => setAdding(false)}>Cancel</Button><Button variant="primary" disabled={!name.trim()} onClick={make}>Create</Button></>}>
          {problem && <Problem>{problem}</Problem>}
          <Field label="Name" autoFocus value={name} onChange={setName} onEnter={make} placeholder="e.g. Outdoor furniture" />
        </Dialog>
      )}
    </Page>
  );
}

function VendorForm({ title, v, yes, onSave, onClose }: { title: string; v?: Row; yes: string; onSave: (values: { name: string; country: string }) => Promise<void>; onClose: () => void }) {
  const [name, setName] = useState(String(v?.name ?? ''));
  const [country, setCountry] = useState(String(v?.country ?? ''));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const save = async () => {
    if (!name.trim() || busy) return;
    setBusy(true); setError('');
    try { await onSave({ name: name.trim(), country: country.trim() }); } catch (e) { setError((e as Error).message); setBusy(false); }
  };
  return (
    <Dialog title={title} onClose={onClose} foot={<><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={!name.trim() || busy} onClick={save}>{yes}</Button></>}>
      {error && <Problem>{error}</Problem>}
      <Field label="Name" autoFocus value={name} onChange={setName} onEnter={save} />
      <Field label="Country" value={country} onChange={setCountry} onEnter={save} />
    </Dialog>
  );
}

export function Vendors() {
  const { go } = useNav();
  const { data, error, reload } = useRead('vendor_book', () => list('vendor_book'), VENDORS);
  const [q, setQ] = useQuery('q');
  const [adding, setAdding] = useState(false);
  const [ids, setIds] = useState<Set<string>>(new Set());
  const [deleting, setDeleting] = useState(false);
  const [busy, setBusy] = useState(false);
  if (!data) return <PageState title="Vendors" error={error} />;
  const rows = data.filter(r => !q || `${r.name} ${r.country}`.toLowerCase().includes(q.toLowerCase())).sort((a, b) => String(a.name).localeCompare(String(b.name)));
  const picked = rows.filter(r => ids.has(String(r.id)));
  const drop = async () => {
    setBusy(true);
    const { took, said } = await eachRow(picked, r => String(r.name), async r => { await remove('vendor', String(r.id)); });
    await reload();
    setBusy(false); setDeleting(false); setIds(new Set());
    toast([`Deleted ${plural(took, 'vendor')}.`, ...said].join(' '), said.length ? 'error' : 'notice');
  };
  return (
    <Page title="Vendors" list>
      <Toolbar>
        <SearchField placeholder="Search vendors" value={q} onChange={setQ} />
        {picked.length > 0 && <Button onClick={() => setDeleting(true)}>Delete ({picked.length})</Button>}
        <Button variant="primary" icon="plus" onClick={() => setAdding(true)}>New vendor</Button>
      </Toolbar>
      {!rows.length && <Toolbar><Note>{q ? 'No vendor matches.' : 'No vendors yet.'}</Note></Toolbar>}
      {rows.length > 0 && <Table<Row> rows={rows} onRow={r => go(`/vendors/${r.id}`)} select={{ ids, idOf: r => String(r.id), onChange: setIds }} columns={[
        { key: 'name', label: 'Vendor', fill: true, render: r => r.name },
        { key: 'country', label: 'Country', soft: true, render: r => r.country },
        { key: 'contacts', label: 'Contacts', soft: true, num: true, render: r => r.contact_count || '' },
        { key: 'open', label: 'Open builds', soft: true, num: true, render: r => r.open_builds || '' },
        { key: 'sent', label: 'Last sent', soft: true, num: true, sort: r => r.last_sent_at, render: r => show(r.last_sent_at) }
      ]} />}
      {adding && <VendorForm title="New vendor" yes="Create" onClose={() => setAdding(false)} onSave={async ({ name, country }) => { const v = await create('vendor', { name, ...(country ? { country } : {}) }); moved(); setAdding(false); go(`/vendors/${v.id}`); }} />}
      {deleting && <Confirm title={`Delete ${plural(picked.length, 'vendor')}?`} note="A vendor who still has builds is kept." no="Keep them" yes={busy ? 'Deleting…' : 'Delete'} busy={busy} onYes={drop} onClose={() => setDeleting(false)} />}
    </Page>
  );
}

const rankOpen = (r: Row) => (r.stage === 'sent' || r.stage === 'in_progress' ? 0 : r.stage === 'review' ? 1 : 2);

export function VendorPage({ id }: { id: string }) {
  const { go, swap } = useNav();
  const page = useRead(`vendor-book|${id}`, () => list('vendor_book').then(r => r.find(v => v.id === id) ?? null), VENDORS);
  const books = useRead(`vendor-builds|${id}`, () => list('build_book', { vendor_id: id }), BOOK);
  const link = useRead(`vendor-link|${id}`, () => run<{ url: string; minted_at?: string }>('vendor_link', { vendor: id }), ['vendor']);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [asking, setAsking] = useState<'rotate' | 'delete' | 'edit' | null>(null);
  const { busy, run: write } = useWrite(() => Promise.all([page.reload(), link.reload(), books.reload()]), () => setAsking(null), moved);
  if (page.data === null) return <PageState title="Vendor" error="No such vendor." />;
  if (!page.data) return <PageState title="Vendor" error={page.error} />;
  const v = page.data;
  const contacts: Row[] = v.contacts ?? [];
  const theirs = [...(books.data ?? [])].sort((a, b) => rankOpen(a) - rankOpen(b));
  const remember = (next: Row[]) => write(async () => { await patch('vendor', id, { contacts: next.map(c => ({ name: c.name ?? '', email: c.email })) }); });
  const add = () => {
    const e = email.trim();
    if (!e) return;
    if (!isEmail(e)) return toast(notEmail(e));
    remember([...contacts.filter(c => c.email !== e), { name: name.trim(), email: e }]);
    setName(''); setEmail('');
  };
  const url = link.data?.url ?? '';
  const facts: [string, ReactNode][] = [
    ['Name', v.name], ['Country', v.country], ['Builds', `${v.builds ?? theirs.length}${v.open_builds ? `, ${v.open_builds} open` : ''}`], ['Last sent a build', show(v.last_sent_at) || 'Never'],
    ['Portal link', url ? (link.data?.minted_at ? `Made ${show(link.data.minted_at)}` : 'Made') : link.error ? <Note>{link.error}</Note> : link.data ? 'No link yet: it is made when their first build is sent' : 'Reading…']
  ];
  return (
    <Page title={String(v.name ?? '')} aside={<Actions items={[
      ...(url ? [{ key: 'open', label: 'Open', icon: 'open' as const, onClick: () => window.open(`${linkPath(url)}?buyer=1`, '_blank', 'noopener') }, { key: 'rotate', label: 'Rotate link', disabled: busy, onClick: () => setAsking('rotate') }] : []),
      { key: 'edit', label: 'Edit', disabled: busy, onClick: () => setAsking('edit') },
      { key: 'delete', label: 'Delete', disabled: busy, onClick: () => setAsking('delete') }
    ]} />}>
      <Section title="Details"><List items={facts.filter(f => f[1]).map(([k, val]) => ({ key: k, left: k, right: val }))} /></Section>
      <Section title="Contacts">
        <FormCard>
          {contacts.map(c => <FormLine key={String(c.email)} words={c.name || c.email} note={c.name && c.email}><IconButton icon="close" label={`Remove ${c.email}`} onClick={() => remember(contacts.filter(x => x !== c))} /></FormLine>)}
          <FormLine>
            <Field bare label="Name" value={name} onChange={setName} onEnter={add} placeholder="Name" />
            <Field bare label="Email" type="email" value={email} onChange={setEmail} onEnter={add} placeholder={contacts.length ? 'Email' : 'Email. Nobody is remembered yet'} />
            <Button variant="quiet" icon="plus" disabled={!email.trim() || busy} onClick={add}>Add</Button>
          </FormLine>
        </FormCard>
      </Section>
      <LeadTimes vendor={id} />
      <Section title="Builds" action={<Button icon="plus" onClick={() => go(`/builds/new/${id}`)}>New build</Button>}>
        {theirs.length ? <Table<Row> rows={theirs} onRow={r => go(`/builds/${r.id}`)} columns={[
          { key: 'n', label: 'Build', fill: true, sort: r => r.name, render: r => marked(r.category, r.category_icon, <Lines top={r.name} sub={r.category_name} />) },
          COMPLETE,
          { key: 'when', label: 'When', soft: true, num: true, sort: r => (idle(r) ? null : r.submitted_at ?? r.approved_at ?? r.due_at), render: r => (idle(r) ? '' : whenOf(r)) },
          STANDING
        ]} /> : <Empty>{books.data ? 'No builds yet.' : books.error ?? 'Reading…'}</Empty>}
      </Section>
      {asking === 'rotate' && <Confirm title="Rotate the portal link?" note={`The link ${String(v.name)} has stops working at once, in every email they were sent. Send them the new one.`} no="Keep it" yes="Rotate" busy={busy}
        onYes={() => write(async () => { await run('rotate_vendor_link', { vendor: id }); return 'A new link is made. The old one no longer opens anything.'; })} onClose={() => setAsking(null)} />}
      {asking === 'delete' && <Confirm title={`Delete ${v.name}?`} note="A vendor who still has builds is kept." no="Keep them" yes="Delete" busy={busy}
        onYes={() => write(async () => { await remove('vendor', id); toast(`Deleted ${v.name}.`, 'notice'); swap('/vendors'); return GONE; })} onClose={() => setAsking(null)} />}
      {asking === 'edit' && <VendorForm title={`Edit ${v.name}`} v={v} yes="Save" onClose={() => setAsking(null)} onSave={async values => { await patch('vendor', id, values); moved(); await page.reload(); setAsking(null); }} />}
    </Page>
  );
}

const UNITS = [{ value: 'business_days', label: 'Business days' }, { value: 'days', label: 'Days' }, { value: 'weeks', label: 'Weeks' }];
const SHIP_METHOD = /(^|[^a-z])ship(ping)?[\s_-]*(method|via)($|[^a-z])/i;
type LeadRule = { id?: string; ship_method: string; min_time: string; max_time: string; unit: string; note: string };

function LeadTimes({ vendor }: { vendor: string }) {
  const rules = useRead(`lead-times|${vendor}`, () => list('lead_time_rule', { vendor }), ['lead_time_rule']);
  const fields = useRead('field-ship', () => list('field', { type: 'select' }), ['field']);
  const methods = [...new Set((fields.data ?? []).filter(f => SHIP_METHOD.test(String(f.key)) || SHIP_METHOD.test(String(f.name))).flatMap(f => (f.options ?? []) as string[]))];
  const [editing, setEditing] = useState<LeadRule | null>(null);
  const [busy, setBusy] = useState(false);
  const [said, setSaid] = useState('');
  const act = async (what: () => Promise<unknown>) => {
    setBusy(true); setSaid('');
    try { await what(); moved(); await rules.reload(); setEditing(null); } catch (e) { setSaid((e as Error).message); }
    setBusy(false);
  };
  const save = () => {
    if (!editing || busy) return;
    const body = { ship_method: editing.ship_method || null, min_time: editing.min_time.trim(), max_time: (editing.max_time || editing.min_time).trim(), unit: editing.unit, note: editing.note.trim() };
    void act(() => (editing.id ? patch('lead_time_rule', editing.id, body) : create('lead_time_rule', { vendor, ...body })));
  };
  const form = editing && (
    <FormLine wrap>
      <Picker all={false} label={editing.ship_method || 'Any method'} value={editing.ship_method} options={[{ value: '', label: 'Any method' }, ...methods.map(m => ({ value: m, label: m }))]} onChange={v => setEditing({ ...editing, ship_method: v })} />
      <Field bare short label="From" value={editing.min_time} onChange={v => setEditing({ ...editing, min_time: v })} onEnter={save} placeholder="From" />
      <Field bare short label="To" value={editing.max_time} onChange={v => setEditing({ ...editing, max_time: v })} onEnter={save} placeholder="To" />
      <Picker all={false} label="Weeks" value={editing.unit} options={UNITS} onChange={v => setEditing({ ...editing, unit: v })} />
      <Field bare label="Note" value={editing.note} onChange={v => setEditing({ ...editing, note: v })} onEnter={save} placeholder="Note, if it needs one" />
      <Button variant="quiet" onClick={() => { setEditing(null); setSaid(''); }}>Cancel</Button>
      <Button variant="primary" disabled={busy || !editing.min_time.trim()} onClick={save}>{editing.id ? 'Save' : 'Add'}</Button>
    </FormLine>
  );
  const items = [...(rules.data ?? [])].sort((a, b) => String(a.name).localeCompare(String(b.name)));
  return (
    <Section title="Lead times" action={!editing && <Button icon="plus" onClick={() => { setSaid(''); setEditing({ ship_method: '', min_time: '', max_time: '', unit: 'weeks', note: '' }); }}>Add lead time</Button>}>
      {said && <Problem>{said}</Problem>}
      {!items.length && !editing ? <Empty>{rules.data ? 'None yet. An empty lead time on this vendor’s builds stays empty until one is added.' : rules.error ?? 'Reading…'}</Empty> : <FormCard>
        {items.map(r => editing?.id === String(r.id) ? <Fragment key={String(r.id)}>{form}</Fragment> : (
          <FormLine key={String(r.id)} words={String(r.name)} note={r.note}>
            <IconButton icon="pencil" label={`Change ${r.name}`} disabled={busy} onClick={() => { setSaid(''); setEditing({ id: String(r.id), ship_method: r.ship_method ?? '', min_time: String(r.min_time), max_time: String(r.max_time), unit: String(r.unit), note: String(r.note ?? '') }); }} />
            <IconButton icon="close" label={`Remove ${r.name}`} disabled={busy} onClick={() => void act(() => remove('lead_time_rule', String(r.id)))} />
          </FormLine>
        ))}
        {editing && !editing.id && form}
      </FormCard>}
    </Section>
  );
}

