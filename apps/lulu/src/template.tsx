import { useEffect, useState, type ReactNode } from 'react';
import { list, moved, patch, remove, run, useQuery, useRead, type Row } from './api';
import { Actions, Button, CheckField, Combo, Confirm, Dialog, Empty, Field, FormCard, FormGroup, FormLine, FormRow, GONE, IconButton, IconField, Indicator, Labeled, Lines, MultiCombo, Note, Page, PageState, Picker, SearchField, Section, Spacer, Table, Tabs, Tag, Toolbar, toast, useWrite, type Column, type IconName, type Option } from './ui';
import { ANGLES, ICON_NAMES, categoryIcon, isOpen, plural, useNav, type Check, type FieldType, type Fill, type Measure, type Op, type Photo, type Rule, type Spec, type TField, type Test, type When } from './model';
import { DUE, STANDING, bookRead, specOf } from './builds';

const angleOf = (key: string) => ANGLES.find(a => a.key === key);
const angleLabel = (key: string) => angleOf(key)?.label ?? key;
const COUNTRIES = ['Afghanistan', 'Albania', 'Algeria', 'Andorra', 'Angola', 'Antigua and Barbuda', 'Argentina', 'Armenia', 'Australia', 'Austria', 'Azerbaijan', 'Bahamas', 'Bahrain', 'Bangladesh', 'Barbados', 'Belarus', 'Belgium', 'Belize', 'Benin', 'Bhutan', 'Bolivia', 'Bosnia and Herzegovina', 'Botswana', 'Brazil', 'Brunei', 'Bulgaria', 'Burkina Faso', 'Burundi', 'Cambodia', 'Cameroon', 'Canada', 'Cape Verde', 'Central African Republic', 'Chad', 'Chile', 'China', 'Colombia', 'Comoros', 'Congo', 'Costa Rica', 'Croatia', 'Cuba', 'Cyprus', 'Czech Republic', 'Democratic Republic of the Congo', 'Denmark', 'Djibouti', 'Dominica', 'Dominican Republic', 'East Timor', 'Ecuador', 'Egypt', 'El Salvador', 'Equatorial Guinea', 'Eritrea', 'Estonia', 'Eswatini', 'Ethiopia', 'Fiji', 'Finland', 'France', 'Gabon', 'Gambia', 'Georgia', 'Germany', 'Ghana', 'Greece', 'Grenada', 'Guatemala', 'Guinea', 'Guinea-Bissau', 'Guyana', 'Haiti', 'Honduras', 'Hungary', 'Iceland', 'India', 'Indonesia', 'Iran', 'Iraq', 'Ireland', 'Israel', 'Italy', 'Ivory Coast', 'Jamaica', 'Japan', 'Jordan', 'Kazakhstan', 'Kenya', 'Kiribati', 'Kosovo', 'Kuwait', 'Kyrgyzstan', 'Laos', 'Latvia', 'Lebanon', 'Lesotho', 'Liberia', 'Libya', 'Liechtenstein', 'Lithuania', 'Luxembourg', 'Madagascar', 'Malawi', 'Malaysia', 'Maldives', 'Mali', 'Malta', 'Marshall Islands', 'Mauritania', 'Mauritius', 'Mexico', 'Micronesia', 'Moldova', 'Monaco', 'Mongolia', 'Montenegro', 'Morocco', 'Mozambique', 'Myanmar', 'Namibia', 'Nauru', 'Nepal', 'Netherlands', 'New Zealand', 'Nicaragua', 'Niger', 'Nigeria', 'North Korea', 'North Macedonia', 'Norway', 'Oman', 'Pakistan', 'Palau', 'Palestine', 'Panama', 'Papua New Guinea', 'Paraguay', 'Peru', 'Philippines', 'Poland', 'Portugal', 'Puerto Rico', 'Qatar', 'Romania', 'Russia', 'Rwanda', 'Saint Kitts and Nevis', 'Saint Lucia', 'Saint Vincent and the Grenadines', 'Samoa', 'San Marino', 'Sao Tome and Principe', 'Saudi Arabia', 'Senegal', 'Serbia', 'Seychelles', 'Sierra Leone', 'Singapore', 'Slovakia', 'Slovenia', 'Solomon Islands', 'Somalia', 'South Africa', 'South Korea', 'South Sudan', 'Spain', 'Sri Lanka', 'Sudan', 'Suriname', 'Sweden', 'Switzerland', 'Syria', 'Taiwan', 'Tajikistan', 'Tanzania', 'Thailand', 'Togo', 'Tonga', 'Trinidad and Tobago', 'Tunisia', 'Turkey', 'Turkmenistan', 'Tuvalu', 'Uganda', 'Ukraine', 'United Arab Emirates', 'United Kingdom', 'United States', 'Uruguay', 'Uzbekistan', 'Vanuatu', 'Vatican City', 'Venezuela', 'Vietnam', 'Yemen', 'Zambia', 'Zimbabwe'];
const choicesOf = (f?: TField) => !f ? undefined : f.type === 'select' ? f.options ?? [] : f.type === 'yesno' ? ['Yes', 'No'] : f.type === 'country' ? COUNTRIES : undefined;
const multiable = (f: Pick<TField, 'type' | 'pctBreakdown'>) => !f.pctBreakdown && f.type !== 'yesno' && f.type !== 'country';
const withKey = <K extends 'requiredWhen' | 'must' | 'fills'>(f: TField, key: K, v: TField[K]): TField => { const { [key]: _, ...rest } = f; return ((v as unknown[] | undefined)?.length ? { ...rest, [key]: v } : rest) as unknown as TField; };
const canon = (v: unknown) => JSON.stringify(v, (_, x: unknown) => x && typeof x === 'object' && !Array.isArray(x) ? Object.fromEntries(Object.entries(x as Row).filter(([, y]) => y !== undefined).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))) : x);

const OP_WORDS: Record<Op, string> = { is: 'is', not: 'is not', contains: 'contains', above: 'is above', below: 'is below', filled: 'is filled in' };
function askWords(c: Pick<Test, 'op' | 'values'>, voice: 'is' | 'must' = 'is'): string {
  const vs = c.values ?? [];
  const be = voice === 'is' ? 'is' : 'be';
  switch (c.op) {
    case 'filled': return `${be} filled in`;
    case 'above': return `${be} above ${vs[0] ?? ''}`;
    case 'below': return `${be} below ${vs[0] ?? ''}`;
    case 'contains': return `${voice === 'is' ? 'contains' : 'contain'} ${vs[0] ?? ''}`;
    case 'not': return `${voice === 'is' ? 'is not' : 'not be'} ${vs.join(' or ')}`;
    default: return `${be} ${vs.join(' or ')}`;
  }
}
const MEASURE_WORDS: Record<Measure, string> = { longest: 'longest side', shortest: 'shortest side', girth: 'length plus girth' };
const whenWords = (fields: TField[], w: When) => w.map(c => `${fields.find(f => f.key === c.field)?.label || c.field}${c.measure ? `’s ${MEASURE_WORDS[c.measure]}` : ''} ${askWords(c)}`).join(' and ');
const fillWords = (fields: TField[], fills: Fill[]) => fills.map(x => (x.when?.length ? `${x.value} when ${whenWords(fields, x.when)}` : `otherwise ${x.value}`)).join('; ');
const requiredWords = (fields: TField[], f: TField) => f.required ? 'Required' : !f.requiredWhen?.length ? 'Optional' : `Required when ${f.requiredWhen.map(w => whenWords(fields, w)).join(', or when ')}`;
const checkWords = (fields: TField[], c: Check) => c.message?.trim() || `Must ${askWords(c, 'must')}${c.when?.length ? ` when ${whenWords(fields, c.when)}` : ''}`;
const needOf = (f: TField, hidden: boolean) => hidden ? 'hidden' : f.owner === 'lg' ? 'internal' : f.required ? 'always' : f.requiredWhen?.length ? 'when' : 'optional';
const NEED_WORD: Record<string, string> = { hidden: '—', internal: 'Internal', always: 'Always', when: 'When…', optional: 'Optional' };
const NEED_FILTER = [{ value: 'always', label: 'Always' }, { value: 'when', label: 'Conditional' }, { value: 'optional', label: 'Optional' }, { value: 'check', label: 'Has a check' }, { value: 'internal', label: 'Internal' }, { value: 'hidden', label: 'Hidden here' }];
function typeSummary(f: TField): string {
  const several = f.multi ? ', several' : '';
  if (f.type === 'select') return `${plural(f.options?.length ?? 0, 'option')}${several}`;
  if (f.type === 'dimensions') return (f.parts ?? ['Width', 'Depth', 'Height']).map(p => p[0]?.toUpperCase() ?? '').join(' × ') + several;
  if (f.type === 'text' && f.pctBreakdown) return 'Percentages';
  return ({ text: 'Text', number: 'Number', yesno: 'Yes / No', country: 'Country' } as Record<string, string>)[f.type] + several;
}
const howMany = (r: Rule) => (angleOf(r.role)?.many ? `Min ${r.min}` : (r.max ?? 0) > r.min ? `Max ${r.max}` : '1');

const renamed = (w: When, field: string, a: string, b: string): When => w.map(c => (c.field === field && c.values?.includes(a) ? { ...c, values: c.values.map(v => (v === a ? b : v)) } : c));
function renamedIn(f: TField, key: string, a: string, b: string): TField {
  let out = f.requiredWhen ? withKey(f, 'requiredWhen', f.requiredWhen.map(w => renamed(w, key, a, b))) : f;
  if (f.fills) out = withKey(out, 'fills', f.fills.map(x => ({ ...x, ...(f.key === key && x.value === a ? { value: b } : {}), ...(x.when ? { when: renamed(x.when, key, a, b) } : {}) })));
  if (f.must) out = withKey(out, 'must', f.must.map(c => ({ ...c, ...(f.key === key && c.values ? { values: c.values.map(v => (v === a ? b : v)) } : {}), ...(c.when ? { when: renamed(c.when, key, a, b) } : {}) })));
  return out;
}
function stripped(w: When, field: string, gone: string[]): When | undefined {
  const out: When = [];
  for (const c of w) {
    if (c.field !== field || (c.op !== 'is' && c.op !== 'not')) { out.push(c); continue; }
    const values = (c.values ?? []).filter(v => !gone.includes(v));
    if (values.length) out.push({ ...c, values }); else if (c.op === 'is') return undefined;
  }
  return out.length ? out : undefined;
}
function strippedIn(f: TField, key: string, gone: string[]): TField {
  let out = f.requiredWhen ? withKey(f, 'requiredWhen', f.requiredWhen.flatMap(w => { const s = stripped(w, key, gone); return s ? [s] : []; })) : f;
  if (f.fills) out = withKey(out, 'fills', f.fills.flatMap(x => {
    if (f.key === key && gone.includes(x.value)) return [];
    if (!x.when) return [x];
    const when = stripped(x.when, key, gone);
    return when ? [{ ...x, when }] : [];
  }));
  if (f.must) out = withKey(out, 'must', f.must.flatMap(c => {
    let next = c;
    if (f.key === key && (c.op === 'is' || c.op === 'not')) {
      const values = (c.values ?? []).filter(v => !gone.includes(v));
      if (!values.length) return [];
      next = { ...next, values };
    }
    if (c.when) { const when = stripped(c.when, key, gone); if (!when) return []; next = { ...next, when }; }
    return [next];
  }));
  return out;
}

type Draft = { fields: TField[]; hides: string[]; images: Rule[]; photo?: Photo; own: Set<string> };
type Line = { id: string; f: TField; hidden: boolean; from: string; changed: boolean };

export function TemplatePage({ id }: { id: string }) {
  const { go, swap, brand } = useNav();
  const read = async () => {
    const [rows, spec, own] = await Promise.all([list('template'), run<Partial<Spec>>('buyer_template_spec', { id }), list('field', { template: id })]);
    const row = rows.find(r => r.id === id);
    const parent = row?.parent ? specOf(await run<Partial<Spec>>('buyer_template_spec', { id: row.parent })) : undefined;
    const chain: Row[] = [];
    for (let at = row?.parent; at; at = rows.find(r => r.id === at)?.parent) { const t = rows.find(r => r.id === at); if (!t) break; chain.push(t); }
    const fieldsOf = new Map<string, Set<string>>();
    if (chain.length) for (const f of await list('field')) { const k = String(f.template); fieldsOf.set(k, (fieldsOf.get(k) ?? new Set()).add(String(f.key))); }
    return { rows, row, spec: specOf(spec), parent, chain, own: new Set(own.map(f => String(f.key))), fieldsOf };
  };
  const { data, error, reload } = useRead(`template|${id}`, read, ['template', 'field']);
  const books = useRead('build_book', bookRead, ['build', 'build_row', 'template', 'vendor']);
  const [draft, setDraft] = useState<Draft | null>(null);
  const { busy, run: write } = useWrite(() => Promise.all([reload(), books.reload()]), undefined, moved);
  const [q, setQ] = useQuery('q');
  const [need, setNeed] = useQuery('need');
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [photo, setPhoto] = useState<number | 'new' | null>(null);
  const [bulkWhen, setBulkWhen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [refused, setRefused] = useState('');
  useEffect(() => { if (data?.row) document.title = String(data.row.name); }, [data?.row?.name]);
  if (!data) return <PageState title="Template" error={error} />;
  const { row, rows, spec, parent, chain, own, fieldsOf } = data;
  if (!row) return <PageState title="Template" error="No such template." />;
  const start = (): Draft => ({ fields: spec.fields, hides: row.hides ?? [], images: spec.images, photo: spec.photo ?? undefined, own });
  const shown = draft ?? start();
  const change = (fn: (was: Draft) => Draft) => setDraft(was => fn(was ?? start()));
  const isRoot = !row.parent;
  const fromOf = (k: string) => chain.find(t => fieldsOf.get(String(t.id))?.has(k));
  const hiddenFields = shown.hides.flatMap(k => parent?.fields.filter(f => f.key === k) ?? []);
  const children = rows.filter(x => x.parent === id);
  const open = (books.data ?? []).filter(r => r.template === id && isOpen(r));
  const lines: Line[] = [
    ...shown.fields.map(f => { const was = parent?.fields.some(x => x.key === f.key); return { id: f.key, f, hidden: false, from: was ? String(fromOf(f.key)?.id ?? '') : '', changed: !!was && shown.own.has(f.key) }; }),
    ...hiddenFields.map(f => ({ id: f.key, f, hidden: true, from: String(fromOf(f.key)?.id ?? ''), changed: false }))
  ];
  const needle = q.trim().toLowerCase();
  const found = lines.filter(l => (!needle || l.f.label.toLowerCase().includes(needle) || l.f.key.toLowerCase().includes(needle)) && (!need || (need === 'check' ? !!l.f.must?.length : needOf(l.f, l.hidden) === need)));
  const sections = [...new Set(shown.fields.map(f => f.section))];
  const patchFields = (keys: string[], p: (f: TField) => TField) => change(was => ({ ...was, fields: was.fields.map(f => (keys.includes(f.key) ? p(f) : f)), own: new Set([...was.own, ...keys]) }));
  const saveField = (next: TField, renames: [string, string][], gone: string[], isNew: boolean) => change(was => {
    let fields = isNew ? [...was.fields, next] : was.fields.map(f => (f.key === next.key ? next : f));
    let images = was.images;
    for (const [a, b] of renames) {
      fields = fields.map(f => renamedIn(f, next.key, a, b));
      images = images.map(r => (r.when ? { ...r, when: renamed(r.when, next.key, a, b) } : r));
    }
    if (gone.length) {
      fields = fields.map(f => strippedIn(f, next.key, gone));
      images = images.flatMap(r => { if (!r.when) return [r]; const when = stripped(r.when, next.key, gone); return when ? [{ ...r, when }] : []; });
    }
    return { ...was, fields, images, own: new Set([...was.own, next.key]) };
  });
  const dropFields = (keys: string[]) => change(was => ({
    ...was,
    fields: was.fields.filter(f => !keys.includes(f.key)).map(f => {
      const reads = (w: When | undefined) => (w ?? []).some(c => keys.includes(c.field));
      const kept = f.requiredWhen ? withKey(f, 'requiredWhen', f.requiredWhen.filter(w => !reads(w))) : f;
      const checked = f.must ? withKey(kept, 'must', f.must.filter(c => !reads(c.when))) : kept;
      return f.fills ? withKey(checked, 'fills', f.fills.filter(x => !reads(x.when))) : checked;
    }),
    hides: [...new Set([...was.hides.filter(k => !keys.includes(k)), ...keys.filter(k => parent?.fields.some(f => f.key === k))])],
    images: was.images.filter(r => !(r.when ?? []).some(c => keys.includes(c.field)))
  }));
  const revertField = (k: string) => { const was = parent?.fields.find(f => f.key === k); if (was) change(d => ({ ...d, fields: d.fields.map(f => (f.key === k ? { ...was } : f)), own: new Set([...d.own].filter(x => x !== k)) })); };
  const unhideField = (k: string) => change(was => {
    const back = parent?.fields.find(f => f.key === k);
    return { ...was, fields: back && !was.fields.some(f => f.key === k) ? [...was.fields, back] : was.fields, hides: was.hides.filter(x => x !== k) };
  });
  const publish = () => write(async () => {
    if (!draft) return;
    await run('publish_template', { key: row.key, definition: { fields: draft.fields, hides: draft.hides, images: draft.images, photo: draft.photo, export: row.export ?? {} }, name: row.name, icon: row.icon ?? '', sheet: row.sheet });
    setDraft(null);
    setPicked(new Set());
    return `Published ${row.name}.`;
  });
  const saveDetails = (values: Row) => write(async () => { await patch('template', id, values); });
  const duplicate = () => write(async () => { const out = await run('duplicate_template', { id }); go(`/templates/${out.id}`); return GONE; });
  const destroy = () => write(async () => {
    try { await remove('template', id); } catch (e) { setRefused((e as Error).message); return; }
    toast(`Deleted ${row.name}.`, 'notice');
    swap('/templates');
    return GONE;
  });
  const live = [...picked].filter(k => shown.fields.some(f => f.key === k));
  const clear = () => setPicked(new Set());
  const columns: Column<Line>[] = [
    { key: 'f', label: 'Field', fill: true, sort: l => l.f.label, render: l => (
      <span className="named"><span>{l.f.label || 'Untitled field'}</span>
        {!l.hidden && l.changed && <Indicator accent icon="pencil" label="Changed on this template from what its parent says" />}
        {!!l.f.must?.length && <Indicator icon="shield" label={`Its answer must pass ${plural(l.f.must.length, 'check')}`} />}
        {!!l.f.fills?.length && <Indicator icon="star" label={`Fills itself: ${fillWords(shown.fields, l.f.fills)}`} />}
      </span>
    ) },
    { key: 't', label: 'Type', soft: true, short: true, render: l => typeSummary(l.f) },
    { key: 'h', label: 'Hidden', soft: true, sort: l => (l.hidden ? 0 : 1), render: l => (l.hidden ? <Indicator icon="eyeOff" label="Hidden on this template: vendors are not asked it" /> : null) },
    { key: 'i', label: 'Inherited', soft: true, sort: l => (l.from ? 0 : 1), render: l => (l.from ? <Indicator icon="check" label={`From ${chain.find(x => String(x.id) === l.from)?.name ?? 'a parent'}`} /> : null) },
    { key: 'r', label: 'Required', soft: true, num: true, sort: l => needOf(l.f, l.hidden), render: l => <span className="explained" title={l.hidden ? 'Hidden on this template' : l.f.owner === 'lg' ? `Filled in by ${brand}` : requiredWords(shown.fields, l.f)}>{NEED_WORD[needOf(l.f, l.hidden)]}</span> }
  ];
  const editingField = editing === null || editing === 'new' ? undefined : shown.fields.find(f => f.key === editing) ?? hiddenFields.find(f => f.key === editing);
  const imagesFrom = parent && draft === null && !row.images ? chain.find(t => t.images)?.name : undefined;
  const photoFrom = parent?.photo && canon(shown.photo ?? null) === canon(parent.photo) ? chain.find(t => t.photo)?.name : undefined;
  return (
    <Page wide title={String(row.name)} aside={<Actions items={[{ key: 'duplicate', label: 'Duplicate', icon: 'copy', disabled: busy, onClick: duplicate }, { key: 'delete', label: 'Delete', disabled: busy, onClick: () => { setRefused(''); setDeleting(true); } }]} />}>
      {draft && (
        <div className="banner"><span>Unpublished changes. Builds keep reading the published template until you publish.</span>
          <Button disabled={busy} onClick={() => setDraft(null)}>Discard</Button><Button variant="primary" disabled={busy} onClick={publish}>{busy ? 'Publishing…' : 'Publish'}</Button>
        </div>
      )}
      <Details key={`${row.name}|${row.parent}|${row.icon}`} row={row} isRoot={isRoot} busy={busy} parents={rows.filter(x => { for (let at: Row | undefined = x, n = 0; at && n < 64; at = rows.find(r => r.id === at!.parent), n++) if (at.id === id) return false; return true; })} under={children} onSave={saveDetails} />
      <Section>
        <Toolbar>
          {picked.size > 0 ? <>
            <Button disabled={!live.length} onClick={() => { patchFields(live, f => withKey({ ...f, required: true }, 'requiredWhen', undefined)); clear(); }}>Required</Button>
            <Button disabled={!live.length} onClick={() => setBulkWhen(true)}>Required when…</Button>
            <Button disabled={!live.length} onClick={() => { patchFields(live, f => withKey({ ...f, required: false }, 'requiredWhen', undefined)); clear(); }}>Optional</Button>
            <Button disabled={!live.length} onClick={() => { dropFields(live); clear(); }}>Remove</Button>
            <Spacer /><Button variant="quiet" onClick={clear}>Clear</Button>
          </> : <>
            <SearchField placeholder={`Search ${plural(lines.length, 'field')}`} value={q} onChange={setQ} />
            <Picker label="Required" value={need} options={NEED_FILTER} onChange={setNeed} />
            <Button icon="plus" onClick={() => setEditing('new')}>Add field</Button>
          </>}
        </Toolbar>
        {found.length ? <Table<Line> groups={[...new Set(found.map(l => l.f.section))].map(s => ({ title: s || 'No section', rows: found.filter(l => l.f.section === s) }))} columns={columns} select={{ ids: picked, idOf: l => l.id, onChange: setPicked }}
          rowClass={l => (l.hidden ? 'is-muted' : '')} onRow={l => setEditing(l.f.key)} /> : <Empty>{needle || need ? 'No field matches.' : 'No fields yet.'}</Empty>}
      </Section>
      <Section title="Photographs" action={<Button icon="plus" onClick={() => setPhoto('new')}>Add photo</Button>}>
        {!shown.images.length ? <Empty>No photographs are asked for.</Empty> : <Table<{ r: Rule; i: number }> rows={shown.images.map((r, i) => ({ r, i }))} onRow={x => setPhoto(x.i)} columns={[
          { key: 'a', label: 'Angle', fill: true, sort: x => ANGLES.findIndex(a => a.key === x.r.role), render: x => angleLabel(x.r.role) },
          { key: 'n', label: 'How many', soft: true, render: x => howMany(x.r) },
          { key: 'i', label: 'Inherited', soft: true, render: () => (imagesFrom ? <Indicator icon="check" label={`From ${imagesFrom}`} /> : null) },
          { key: 'w', label: 'Required', soft: true, num: true, render: x => (x.r.when?.length ? <span className="explained" title={`When ${whenWords(shown.fields, x.r.when)}`}>When…</span> : 'Always') }
        ]} />}
      </Section>
      <PhotoSpec photo={shown.photo} from={photoFrom} parent={parent?.photo ?? undefined} parentName={String(chain[0]?.name ?? '')} onChange={p => change(was => ({ ...was, photo: p }))} />
      <Section title="Open builds">
        {open.length ? <Table<Row> rows={open} onRow={r => go(`/builds/${r.id}`)} columns={[{ key: 'n', label: 'Build', fill: true, sort: r => r.name, render: r => <Lines top={r.name} sub={r.vendor} /> }, DUE, STANDING]} />
          : <Empty>{books.data ? 'No build is open under this template.' : books.error ?? 'Reading…'}</Empty>}
      </Section>
      {editing !== null && (editingField && shown.hides.includes(editingField.key)
        ? <Confirm title={editingField.label} note={`Hidden on ${row.name}. ${fromOf(editingField.key)?.name ?? 'Its parent'} asks for it; unhide it to ask for it here too.`} no="Cancel" yes="Unhide" onYes={() => { unhideField(editingField.key); setEditing(null); }} onClose={() => setEditing(null)} />
        : <FieldForm key={editing} field={editingField} fields={shown.fields} sections={sections} templateName={String(row.name)} brand={brand} keys={[...shown.fields, ...hiddenFields].map(f => f.key)} parentName={editingField && parent?.fields.some(f => f.key === editingField.key) ? String(fromOf(editingField.key)?.name ?? '') : undefined}
            changed={!!editingField && !!lines.find(l => l.id === editingField.key)?.changed} onClose={() => setEditing(null)} onSave={(f, renames, gone) => { saveField(f, renames, gone, editing === 'new'); setEditing(null); }}
            onDrop={() => { if (editingField) dropFields([editingField.key]); setEditing(null); }} onRevert={() => { if (editingField) revertField(editingField.key); setEditing(null); }} />)}
      {photo !== null && <PhotoForm key={String(photo)} rule={photo === 'new' ? undefined : shown.images[photo]} others={shown.images.filter((_, i) => i !== photo)} fields={shown.fields} onClose={() => setPhoto(null)}
        onSave={r => { change(was => ({ ...was, images: photo === 'new' ? [...was.images, r] : was.images.map((x, i) => (i === photo ? r : x)) })); setPhoto(null); }}
        onRemove={() => { change(was => ({ ...was, images: was.images.filter((_, i) => i !== photo) })); setPhoto(null); }} />}
      {bulkWhen && <BulkWhen fields={shown.fields} count={live.length} onClose={() => setBulkWhen(false)} onSave={w => {
        patchFields(live, f => { const has = f.requiredWhen ?? []; return withKey({ ...f, required: false }, 'requiredWhen', has.some(x => canon(x) === canon(w)) ? has : [...has, w]); });
        setBulkWhen(false); clear();
      }} />}
      {deleting && <Confirm title={`Delete ${row.name}?`} no="Keep it" yes="Delete" busy={busy || isRoot} onYes={() => void destroy()} onClose={() => setDeleting(false)}
        note={isRoot ? `${row.name} can’t be deleted: every other template starts from it.` : children.length ? `${children.map(c => c.name).join(', ')} ${children.length === 1 ? 'inherits' : 'inherit'} from it.` : open.length ? `${plural(open.length, 'open build')} still ${open.length === 1 ? 'uses' : 'use'} it.` : 'Nothing else uses it.'}>
        {refused && <Tag tone="danger">{refused}</Tag>}
      </Confirm>}
    </Page>
  );
}

function Details({ row, isRoot, busy, parents, under, onSave }: { row: Row; isRoot: boolean; busy: boolean; parents: Row[]; under: Row[]; onSave: (values: Row) => void }) {
  const { go } = useNav();
  const [name, setName] = useState(String(row.name));
  const keep = () => { const n = name.trim(); if (!n) setName(String(row.name)); else if (n !== row.name) onSave({ name: n }); };
  return (
    <Section>
      <FormRow>
        <Field label="Name" value={name} disabled={busy} onChange={setName} onLeave={keep} onEscape={() => setName(String(row.name))} />
        {!isRoot && <Combo label="Inherits" placeholder="A template" value={row.parent ?? ''} onChange={v => v !== (row.parent ?? '') && onSave({ parent: v || null })} options={parents.map(x => ({ value: String(x.id), label: String(x.name), icon: categoryIcon(x.key, x.icon) }))} />}
        <IconField label="Icon" value={row.icon ?? ''} icon={categoryIcon(row.key, row.icon)} disabled={busy} onChange={icon => onSave({ icon })} options={ICON_NAMES.map(n => ({ value: n, icon: categoryIcon(undefined, n) as IconName }))} />
      </FormRow>
      {under.length > 0 && <Labeled label="Templates under it"><div className="toolbar toolbar--wrap">{under.map(c => <Button key={String(c.id)} icon={categoryIcon(c.key, c.icon)} onClick={() => go(`/templates/${c.id}`)}>{c.name}</Button>)}</div></Labeled>}
    </Section>
  );
}

function opsFor(f: TField | undefined, now?: Op, measure?: Measure): Option[] {
  const ops: Op[] = choicesOf(f) ? ['is', 'not', 'filled'] : f && (f.type === 'number' || f.type === 'text') ? ['contains', 'above', 'below', 'filled'] : ['contains', 'filled'];
  if (now && !measure && !ops.includes(now)) ops.push(now);
  const out: Option[] = ops.map(o => ({ value: o, label: OP_WORDS[o] }));
  if (f?.type === 'dimensions' || measure) for (const m of Object.keys(MEASURE_WORDS) as Measure[]) for (const o of ['above', 'below'] as Op[]) out.push({ value: `${o}:${m}`, label: `${MEASURE_WORDS[m]} ${OP_WORDS[o]}` });
  return out;
}
const listed = (op: Op) => op === 'is' || op === 'not';
const startTest = (field: string, fields: TField[]): Test => (choicesOf(fields.find(f => f.key === field)) ? { field, op: 'is', values: [] } : { field, op: 'filled' });
const blankTest = (): Test => ({ field: '', op: 'filled' });
function complete(c: Test): boolean {
  if (!c.field) return false;
  const v = (c.values?.[0] ?? '').trim();
  if (listed(c.op)) return !!c.values?.length;
  if (c.op === 'contains') return v !== '';
  if (c.op === 'above' || c.op === 'below') return v !== '' && Number.isFinite(Number(v));
  return true;
}
const RuleLine = ({ word, end, children }: { word: string; end?: ReactNode; children: ReactNode }) => <div className="rules__line"><span className="note">{word}</span>{children}{end ?? <span />}</div>;
const Boxed = ({ children }: { children: ReactNode }) => <div className="rules rules--boxed">{children}</div>;
const More = ({ children }: { children: ReactNode }) => <span className="rules__more">{children}</span>;

function TestLine({ word, field, test, pick, onChange, onDrop }: { word: string; field?: TField; test: Test; pick: ReactNode; onChange: (t: Test) => void; onDrop: () => void }) {
  const choices = choicesOf(field);
  return (
    <RuleLine word={word} end={<IconButton icon="close" label="Take this test out" onClick={onDrop} />}>
      <div className="rules__test">
        {typeof pick === 'string' ? <span className="rules__name">{pick}</span> : pick}
        <Combo bare label="Test" value={test.measure ? `${test.op}:${test.measure}` : test.op} options={opsFor(field, test.op, test.measure)} onChange={v => {
          const [op, measure] = v.split(':') as [Op, Measure | undefined];
          const keep = (listed(op) && listed(test.op)) || ((op === 'above' || op === 'below') && (test.op === 'above' || test.op === 'below'));
          onChange({ field: test.field, op, ...(keep ? { values: test.values } : {}), ...(measure ? { measure } : {}) });
        }} />
        {test.op === 'filled' ? <span /> : !listed(test.op) || !choices
          ? <Field bare label="Value" placeholder={test.op === 'above' || test.op === 'below' ? 'A number' : 'A word'} value={test.values?.[0] ?? ''} onChange={v => onChange({ ...test, values: [v] })} />
          : <MultiCombo label="Option" placeholder={test.values?.length ? 'or…' : 'An option'} values={test.values ?? []} options={choices} onChange={values => onChange({ ...test, values })} />}
      </div>
    </RuleLine>
  );
}
function WhenEdit({ lead, when, fields, onChange, onDrop, more }: { lead: string; when: When; fields: TField[]; onChange: (w: When) => void; onDrop: () => void; more?: ReactNode }) {
  const at = (i: number, c: Test) => onChange(when.map((x, j) => (j === i ? c : x)));
  return (
    <div className="rules__alt">
      {when.map((c, i) => <TestLine key={i} word={i === 0 ? lead : 'and'} field={fields.find(f => f.key === c.field)} test={c} onChange={nt => at(i, nt)} onDrop={() => (when.length === 1 ? onDrop() : onChange(when.filter((_, j) => j !== i)))}
        pick={<Combo bare label="Field" placeholder="A field" value={c.field} options={fields.map(f => ({ value: f.key, label: f.label || f.key }))} onChange={k => at(i, k ? startTest(k, fields) : blankTest())} />} />)}
      <More><Button variant="quiet" icon="plus" onClick={() => onChange([...when, blankTest()])}>And</Button>{more}</More>
    </div>
  );
}
function Whens({ whens, fields, single, onChange }: { whens: When[]; fields: TField[]; single?: boolean; onChange: (w: When[]) => void }) {
  return (
    <div className="rules">
      {whens.map((w, i) => <WhenEdit key={i} lead={i === 0 ? 'When' : 'Or when'} when={w} fields={fields} onChange={next => onChange(whens.map((x, j) => (j === i ? next : x)))} onDrop={() => onChange(whens.filter((_, j) => j !== i))}
        more={!single && i === whens.length - 1 ? <Button variant="quiet" icon="plus" onClick={() => onChange([...whens, [blankTest()]])}>Or when</Button> : undefined} />)}
    </div>
  );
}
function CheckEdit({ field, check, fields, others, onChange, onDrop }: { field: TField; check: Check; fields: TField[]; others: TField[]; onChange: (c: Check) => void; onDrop: () => void }) {
  const when = check.when ?? [];
  const setWhen = (w: When) => { const { when: _, ...rest } = check; onChange(w.length ? { ...rest, when: w } : rest); };
  return (
    <Boxed>
      <div className="rules__alt"><TestLine word="Check" field={field} test={{ field: field.key || 'self', op: check.op, values: check.values }} onDrop={onDrop} pick={field.label || 'This field'}
        onChange={next => { const { values: _, ...rest } = check; onChange({ ...rest, op: next.op, ...(next.values ? { values: next.values } : {}) }); }} /></div>
      {when.length ? <WhenEdit lead="when" when={when} fields={others} onChange={setWhen} onDrop={() => setWhen([])} /> : <More><Button variant="quiet" icon="plus" onClick={() => setWhen([blankTest()])}>Add condition</Button></More>}
      <RuleLine word="Says"><Field bare label="Message" placeholder={checkWords(fields, { ...check, message: undefined })} value={check.message ?? ''} onChange={v => { const { message: _, ...rest } = check; onChange(v ? { ...rest, message: v } : rest); }} /></RuleLine>
    </Boxed>
  );
}
function FillEdit({ field, fill, others, onChange, onDrop }: { field: TField; fill: Fill; others: TField[]; onChange: (x: Fill) => void; onDrop: () => void }) {
  const when = fill.when ?? [];
  const setWhen = (w: When) => { const { when: _, ...rest } = fill; onChange(w.length ? { ...rest, when: w } : rest); };
  return (
    <Boxed>
      <RuleLine word={when.length ? 'Set to' : 'Otherwise'} end={<IconButton icon="close" label="Take this out" onClick={onDrop} />}><Combo bare label="Value" placeholder="An option" value={fill.value} options={(field.options ?? []).map(o => ({ value: o, label: o }))} onChange={value => onChange({ ...fill, value })} /></RuleLine>
      {when.length ? <WhenEdit lead="when" when={when} fields={others} onChange={setWhen} onDrop={() => setWhen([])} /> : <More><Button variant="quiet" icon="plus" onClick={() => setWhen([blankTest()])}>Add condition</Button></More>}
    </Boxed>
  );
}

const MEASURES = [['Width', 'Depth', 'Height'], ['Width', 'Length', 'Height'], ['Width', 'Depth'], ['Width', 'Length'], ['Diameter', 'Height']];
const dims = (parts: string[]) => `dimensions:${parts.join('|')}`;
function fieldTypes(f: TField): Option[] {
  const measures = [...MEASURES];
  if (f.type === 'dimensions' && f.parts && !measures.some(m => dims(m) === dims(f.parts!))) measures.push(f.parts);
  return [{ value: 'text', label: 'Text' }, { value: 'percentages', label: 'Percentages', sub: '70% cotton, 30% linen' }, { value: 'number', label: 'Number' }, { value: 'select', label: 'Choice' }, { value: 'yesno', label: 'Yes / No' }, { value: 'country', label: 'Country' }, ...measures.map(m => ({ value: dims(m), label: m.join(' × ') }))];
}
const typeValue = (f: TField) => f.type === 'text' && f.pctBreakdown ? 'percentages' : f.type === 'dimensions' ? dims(f.parts ?? MEASURES[0]) : f.type;
function typePatch(v: string): Partial<TField> {
  if (v === 'percentages') return { type: 'text', pctBreakdown: true, parts: undefined, multi: undefined };
  if (v.startsWith('dimensions:')) return { type: 'dimensions', parts: v.slice('dimensions:'.length).split('|'), pctBreakdown: undefined };
  const p: Partial<TField> = { type: v as FieldType, pctBreakdown: undefined, parts: undefined };
  return multiable(p as TField) ? p : { ...p, multi: undefined };
}
const splitParts = (s: string) => s.split(/\s+x\s+|\s*[×,]\s*/i).map(p => p.trim()).filter(Boolean);
const slug = (label: string) => label.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '') || 'field';
const freeKey = (taken: string[], want: string) => { let key = want, n = 2; while (taken.includes(key)) key = `${want}_${n++}`; return key; };

function FieldForm({ field, fields, sections, keys, templateName, brand, parentName, changed, onClose, onSave, onDrop, onRevert }: {
  field?: TField; fields: TField[]; sections: string[]; keys: string[]; templateName: string; brand: string; parentName?: string; changed: boolean; onClose: () => void; onSave: (f: TField, renames: [string, string][], gone: string[]) => void; onDrop: () => void; onRevert: () => void;
}) {
  const isNew = !field;
  const [f, setF] = useState<TField>(field ?? { key: '', label: '', type: 'text', section: sections[0] ?? 'Product', owner: 'vendor', required: false });
  const [renames, setRenames] = useState<[string, string][]>([]);
  const [extra, setExtra] = useState<string[]>([]);
  const [partsText, setPartsText] = useState((field?.parts ?? []).join(' × '));
  const set = (p: Partial<TField>) => setF(was => ({ ...was, ...p }));
  const others = fields.filter(x => x.key !== f.key);
  const whens = f.requiredWhen ?? [], must = f.must ?? [], fills = f.fills ?? [], parts = f.parts ?? [];
  const mode = f.required ? 'always' : whens.length ? 'when' : 'never';
  const internal = f.owner === 'lg';
  const ready = f.label.trim() !== '' && (f.type !== 'select' || (f.options?.length ?? 0) > 0) && (f.type !== 'dimensions' || (parts.length >= 2 && parts.length <= 4)) && whens.every(w => w.length && w.every(complete))
    && must.every(c => complete({ field: f.key || 'new', op: c.op, values: c.values }) && (c.when ?? []).every(complete)) && fills.every(x => (f.options ?? []).includes(x.value) && (x.when ?? []).every(complete));
  const done = () => {
    const out = { ...f, label: f.label.trim() };
    if (isNew) out.key = freeKey(keys, slug(out.label));
    let expected = field?.type === 'select' ? [...(field.options ?? [])] : [];
    for (const [a, b] of renames) expected = expected.map(o => (o === a ? b : o));
    const now = out.type === 'select' ? out.options ?? [] : [];
    const gone = choicesOf(field) && !choicesOf(out) ? choicesOf(field)! : expected.filter(o => !now.includes(o));
    const next = withKey(withKey(withKey(out, 'requiredWhen', whens), 'must', must), 'fills', internal && out.type === 'select' && !out.multi ? fills : undefined);
    if (field && canon(field) === canon(next) && !renames.length) return onClose();
    onSave(next, renames, gone);
  };
  const setMust = (next: Check[]) => setF(was => withKey(was, 'must', next));
  const setFills = (next: Fill[]) => setF(was => withKey(was, 'fills', next));
  return (
    <Dialog wide title={isNew ? 'New field' : field.label || 'Untitled field'} onClose={onClose} foot={<>
      {!isNew && <Button variant="quiet" onClick={onDrop}>{parentName ? `Hide on ${templateName}` : 'Remove field'}</Button>}
      {changed && parentName && <Button variant="quiet" onClick={onRevert}>{`Use ${parentName}’s`}</Button>}
      <Spacer /><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={!ready} onClick={done}>{isNew ? 'Add field' : 'Done'}</Button>
    </>}>
      {parentName && <Note block>{changed ? `Changed here from ${parentName}’s.` : `From ${parentName}. A change here is ${templateName}’s own.`}</Note>}
      <Field label="Name" autoFocus={isNew} value={f.label} onChange={label => set({ label })} />
      <FormRow>
        <Combo label="Type" value={typeValue(f)} options={fieldTypes(f)} onChange={v => { const p = typePatch(v); set(p); setPartsText((p.parts ?? []).join(' × ')); }} />
        <Combo label="Section" placeholder="None" value={f.section} options={[...new Set([...sections, ...extra, f.section])].filter(Boolean).map(s => ({ value: s, label: s }))} onChange={section => section && set({ section })} onAdd={section => { setExtra(x => [...x, section]); set({ section }); }} />
      </FormRow>
      {f.type === 'dimensions' && <Field label="Parts" value={partsText} placeholder="Width × Depth × Height" hint={parts.length < 2 || parts.length > 4 ? 'Two to four parts, with × between them.' : `A vendor answers ${parts.join(' × ')}, in inches.`} onChange={v => { setPartsText(v); set({ parts: splitParts(v) }); }} />}
      {f.type === 'select' && <Labeled label="Options">
        <OptionsEdit options={f.options ?? []} onChange={options => set({ options })} onRename={(a, b) => setRenames(r => [...r, [a, b]])} />
        <CheckField label="Multiple" checked={!!f.multi} onChange={v => set({ multi: v || undefined })}>Multiple: a product can be several of these at once</CheckField>
      </Labeled>}
      {f.type !== 'select' && multiable(f) && <CheckField label="One per part" checked={!!f.multi} onChange={v => set({ multi: v || undefined })}>One per part, separated by ;</CheckField>}
      <Field label="Help" lines={2} value={f.help ?? ''} placeholder="What the vendor is told about it" onChange={v => set({ help: v || undefined })} />
      <FormGroup>
        <FormRow>
          <Labeled label="Filled by"><Tabs value={f.owner} onChange={owner => set({ owner })} tabs={[{ id: 'vendor', label: 'Vendor' }, { id: 'lg', label: brand }]} /></Labeled>
          {!internal && <Labeled label="Required"><Tabs value={mode} onChange={m => setF(was => withKey({ ...was, required: m === 'always' }, 'requiredWhen', m === 'when' ? (whens.length ? whens : [[blankTest()]]) : []))} tabs={[{ id: 'always', label: 'Always' }, { id: 'never', label: 'Never' }, { id: 'when', label: 'When' }]} /></Labeled>}
        </FormRow>
        {!internal && mode === 'when' && <Whens whens={whens} fields={others} onChange={next => setF(was => withKey({ ...was, required: false }, 'requiredWhen', next))} />}
      </FormGroup>
      {!internal && <FormGroup label="Checks">
        {must.map((c, i) => <CheckEdit key={i} field={f} check={c} fields={fields} others={others} onChange={next => setMust(must.map((x, j) => (j === i ? next : x)))} onDrop={() => setMust(must.filter((_, j) => j !== i))} />)}
        <div><Button variant="quiet" icon="plus" onClick={() => setMust([...must, choicesOf(f) ? { op: 'is', values: [] } : { op: 'filled' }])}>Add check</Button></div>
      </FormGroup>}
      {internal && f.type === 'select' && !f.multi && <FormGroup label="Fills itself">
        <Note block>The first that holds fills it on every product, marked as a default. A value {brand} picks on a build stays.</Note>
        {fills.map((x, i) => <FillEdit key={i} field={f} fill={x} others={others} onChange={next => setFills(fills.map((y, j) => (j === i ? next : y)))} onDrop={() => setFills(fills.filter((_, j) => j !== i))} />)}
        <div><Button variant="quiet" icon="plus" onClick={() => setFills([...fills, { value: f.options?.[0] ?? '', when: [blankTest()] }])}>Add rule</Button></div>
      </FormGroup>}
    </Dialog>
  );
}
function OptionsEdit({ options, onChange, onRename }: { options: string[]; onChange: (o: string[]) => void; onRename: (from: string, to: string) => void }) {
  const [editing, setEditing] = useState<{ at: number; text: string } | null>(null);
  const [adding, setAdding] = useState('');
  const commit = () => {
    if (!editing) return;
    const was = options[editing.at], text = editing.text.trim();
    setEditing(null);
    if (!text || text === was || options.includes(text)) return;
    onChange(options.map((o, i) => (i === editing.at ? text : o)));
    onRename(was, text);
  };
  const add = () => { const o = adding.trim(); if (o && !options.includes(o)) onChange([...options, o]); setAdding(''); };
  return (
    <FormCard>
      {options.map((o, i) => <FormLine key={i}><Field bare label={`Option ${i + 1}`} value={editing?.at === i ? editing.text : o} onFocus={() => setEditing({ at: i, text: o })} onChange={text => setEditing({ at: i, text })} onLeave={commit} /><IconButton icon="close" label={`Remove ${o}`} onClick={() => onChange(options.filter((_, j) => j !== i))} /></FormLine>)}
      <FormLine><Field bare label="New option" value={adding} placeholder={options.length ? 'Another option' : 'The first option'} onChange={setAdding} onEnter={add} /><Button variant="quiet" icon="plus" disabled={!adding.trim()} onClick={add}>Add</Button></FormLine>
    </FormCard>
  );
}
function PhotoForm({ rule, others, fields, onClose, onSave, onRemove }: { rule?: Rule; others: Rule[]; fields: TField[]; onClose: () => void; onSave: (r: Rule) => void; onRemove: () => void }) {
  const isNew = !rule;
  const [r, setR] = useState<Rule>(rule ?? { role: ANGLES.find(a => !others.some(x => x.role === a.key && !x.when))?.key ?? 'detail', min: 1 });
  const [count, setCount] = useState<string | null>(null);
  const angle = angleOf(r.role);
  const when = r.when ?? [];
  const setWhen = (w: When) => { const { when: _, ...rest } = r; setR(w.length ? { ...rest, when: w } : rest); };
  const twice = others.some(x => x.role === r.role && canon(x.when ?? null) === canon(r.when ?? null));
  const whole = (v: string) => Math.max(1, Math.floor(Number(v) || 1));
  return (
    <Dialog wide title={isNew ? 'Add photo' : angleLabel(r.role)} onClose={onClose} foot={<>{!isNew && <Button variant="quiet" onClick={onRemove}>Remove</Button>}<Spacer /><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={!angle || !when.every(complete) || twice} onClick={() => onSave(r)}>{isNew ? 'Add' : 'Done'}</Button></>}>
      <FormRow>
        <Combo label="Angle" value={r.role} options={ANGLES.map(a => ({ value: a.key, label: a.label, sub: a.hint }))} onChange={role => { const { max: _, ...rest } = r; setCount(null); setR(angleOf(role)?.many ? { ...rest, role } : { ...rest, role, min: 1 }); }} />
        <Field label={angle?.many ? 'Min' : 'Max'} value={count ?? (angle?.many ? String(r.min) : String(Math.max(r.min, r.max ?? 0)))} hint={angle?.many ? 'At least this many a product.' : 'One a product, and room for this many.'} onChange={v => {
          const digits = v.replace(/\D/g, '');
          setCount(digits);
          if (!digits) return;
          if (angle?.many) return setR({ ...r, min: whole(digits) });
          const { max: _, ...rest } = r;
          const max = whole(digits);
          setR(max > r.min ? { ...rest, max } : rest);
        }} />
      </FormRow>
      {angle && <Note block>{`${angle.label}: ${angle.hint}.`}</Note>}
      <Labeled label="Required"><Tabs value={when.length ? 'when' : 'always'} onChange={m => setWhen(m === 'when' ? (when.length ? when : [blankTest()]) : [])} tabs={[{ id: 'always', label: 'Always' }, { id: 'when', label: 'When' }]} /></Labeled>
      {when.length > 0 && <Whens single whens={[when]} fields={fields} onChange={next => setWhen(next[0] ?? [])} />}
      {twice && <Tag tone="danger">{`${angleLabel(r.role)} is already asked for under this condition.`}</Tag>}
    </Dialog>
  );
}
function BulkWhen({ fields, count, onClose, onSave }: { fields: TField[]; count: number; onClose: () => void; onSave: (w: When) => void }) {
  const [w, setW] = useState<When>([blankTest()]);
  return (
    <Dialog wide title="Required when" onClose={onClose} foot={<><Button onClick={onClose}>Cancel</Button><Button variant="primary" disabled={!w.length || !w.every(complete)} onClick={() => onSave(w)}>{`Apply to ${plural(count, 'field')}`}</Button></>}>
      <div className="rules"><WhenEdit lead="When" when={w} fields={fields} onChange={setW} onDrop={() => setW([blankTest()])} /></div>
    </Dialog>
  );
}
const FORMATS: [string, string][] = [['jpeg', 'JPG'], ['tiff', 'TIFF'], ['png', 'PNG']];
function tidyPhoto(p: Photo): Photo {
  const out: Photo = {};
  if (p.minWidth) out.minWidth = p.minWidth;
  if (p.minHeight) out.minHeight = p.minHeight;
  const formats = FORMATS.map(f => f[0]).filter(v => p.formats?.includes(v));
  if (formats.length) out.formats = formats;
  if (p.transparentPng) out.transparentPng = true;
  if (p.fill) out.fill = p.fill;
  return out;
}
function PhotoSpec({ photo, from, parent, parentName, onChange }: { photo?: Photo; from?: string; parent?: Photo; parentName: string; onChange: (p: Photo | undefined) => void }) {
  const p = photo ?? {};
  const set = (next: Partial<Photo>) => onChange(tidyPhoto({ ...p, ...next }));
  const number = (v: string) => Math.max(0, Math.floor(Number(v.replace(/\D/g, '')) || 0));
  return (
    <Section title="Photo spec" action={from ? <Note>{`From ${from}`}</Note> : parent ? <Button variant="quiet" onClick={() => onChange(parent)}>{`Use ${parentName}’s`}</Button> : undefined}>
      <div className="card form-box">
        <FormRow>
          <Field label="Min width, px" value={p.minWidth ? String(p.minWidth) : ''} onChange={v => set({ minWidth: number(v) })} />
          <Field label="Min height, px" value={p.minHeight ? String(p.minHeight) : ''} onChange={v => set({ minHeight: number(v) })} />
          <Field label="Fill, %" value={p.fill ? String(Math.round(p.fill * 100)) : ''} hint="How much of the frame a studio shot’s product spans." onChange={v => set({ fill: Math.min(100, number(v)) / 100 })} />
        </FormRow>
        <Labeled label="Formats">
          <div className="toolbar toolbar--wrap">
            {FORMATS.map(([value, label]) => <CheckField key={value} label={label} checked={!!p.formats?.includes(value)} onChange={on => set({ formats: on ? [...(p.formats ?? []), value] : (p.formats ?? []).filter(v => v !== value) })} />)}
            <CheckField label="Transparent PNG only" checked={!!p.transparentPng} onChange={on => set({ transparentPng: on })} />
          </div>
        </Labeled>
      </div>
    </Section>
  );
}
