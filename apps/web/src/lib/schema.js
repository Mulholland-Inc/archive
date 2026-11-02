// Tables as the Database screens show them, from the catalog (/api/types) and the DDL (/api/schema).

import { relative } from './live.js';

const sql = { id: 'uuid', text: 'text', long: 'text', date: 'date', money: 'numeric(12,2)', number: 'integer', link: 'uuid', file: 'uuid', email: 'text', yes: 'boolean', place: 'text' };
export const typeLabels = { id: 'ID', text: 'Text', long: 'Long text', date: 'Date', money: 'Money', number: 'Number', link: 'Link', file: 'File', email: 'Email', yes: 'Yes or no', place: 'Place', choice: 'Choice' };

export const typeLabel = (col) => col.label ?? typeLabels[col.type] ?? col.type;

export function ddl(t) {
  if (t.ddl) return t.ddl;
  const lines = t.columns.map((x) => {
    const base = x.name === 'id' ? 'uuid primary key' : x.to ? `uuid references ${x.to}(id)` : sql[x.type];
    return `  ${x.name} ${base}${x.required && x.name !== 'id' ? ' not null' : ''}`;
  });
  return `create table ${t.id} (\n${lines.join(',\n')}\n);`;
}

const kinds = {
  text: 'text', citext: 'text', varchar: 'text', bpchar: 'text', richtext: 'long', uuid: 'id',
  date: 'date', timestamptz: 'date', timestamp: 'date', time: 'date', interval: 'text',
  int2: 'number', int4: 'number', int8: 'number', float4: 'number', float8: 'number', numeric: 'number',
  bool: 'yes', money: 'money', email: 'email', address: 'place', geo: 'place', country: 'place'
};

const own = new Set(['id', 'access', 'created_at', 'updated_at', 'search']);

function column(c, types) {
  const enumType = types[`${c.schema}.${c.type}`]?.enum?.length;
  const ref = c.ref?.replace(/^public\./, '');
  const type = c.ref === 'archive.file' ? 'file' : ref ? 'link' : enumType ? 'choice' : (kinds[c.type] ?? 'text');
  const label = !ref && !enumType && !kinds[c.type] && c.schema === 'archive' ? c.type.charAt(0).toUpperCase() + c.type.slice(1).replaceAll('_', ' ') : undefined;
  return {
    name: c.name,
    type,
    label: c.list ? `List of ${(label ?? typeLabels[type]).toLowerCase()}` : label,
    required: Boolean(c.not_null && !c.default),
    notes: own.has(c.name) ? 'Set by the archive' : c.description,
    to: ref && !ref.startsWith('archive.') ? ref : undefined
  };
}

// counts: { [table]: { rows, updated } }, from one query; workflows: the real workflows, to say which
// tables start one.
export function toTables(catalog, schema = [], counts = {}, workflows = []) {
  const ddls = Object.fromEntries(schema.map((s) => [s.name, s.ddl]));
  return Object.values(catalog.tables ?? {})
    .filter((t) => t.schema === 'public')
    .map((t) => {
      const columns = t.columns.filter((c) => c.type !== 'tsvector').map((c) => column(c, catalog.types ?? {}));
      return {
        id: t.name,
        name: t.name.replaceAll('_', ' ').replace(/^./, (s) => s.toUpperCase()),
        kind: t.kind,
        icon: t.icon,
        description: t.description,
        rows: counts[t.name]?.rows ?? null,
        updated: counts[t.name]?.updated ? relative(counts[t.name].updated) : '',
        access: { edit: ['editor'], view: ['viewer'] },
        workflows: workflows.filter((w) => w.on?.table === t.name).map((w) => w.name),
        columns,
        links: columns.filter((x) => x.to).map((x) => ({ column: x.name, to: x.to })),
        sample: [],
        ddl: ddls[t.name] ?? ''
      };
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

// One read-only query for every table's row count and last change.
export function countsQuery(catalog) {
  const tables = Object.values(catalog.tables ?? {}).filter((t) => t.schema === 'public');
  if (!tables.length) return '';
  const id = (s) => `"${s.replaceAll('"', '""')}"`;
  const parts = tables.map((t) => {
    const updated = t.columns.some((c) => c.name === 'updated_at') ? 'max(updated_at)' : 'null';
    return `select '${t.name.replaceAll("'", "''")}' as name, count(*)::int as rows, ${updated} as updated from ${id(t.name)}`;
  });
  return parts.join(' union all ');
}
