import { api, ApiError } from './api';
import { reloadOntology, session } from './session.svelte';
import { humanize, objectTypes } from './spec';
import type { Change, Field, Op } from './types';
import type { MenuItem } from './components/Menu.svelte';

export const reserved = ['types', 'changes', 'search', 'sources', 'views', 'files'];

export function checkName(name: string, kind: 'type' | 'field', taken: string[] = []): string {
  if (!name) return `Give the ${kind} a name.`;
  if (!/^[a-z][a-z0-9_]*$/.test(name)) return 'Use lowercase letters, digits and underscores, starting with a letter.';
  if (name.length > 63) return 'Keep it under 64 characters.';
  if (kind === 'type' && reserved.includes(name)) return `${name} is used by Archive itself; pick another name.`;
  if (taken.includes(name)) return `There is already a ${kind} called ${name}.`;
  return '';
}

export function slug(label: string) {
  return label
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .replace(/^(\d)/, '_$1');
}

export async function changeOntology(ops: Op[]): Promise<{ change: Change; applied: boolean }> {
  const change = await api.propose(ops);
  try {
    const applied = await api.apply(change.id);
    await reloadOntology();
    return { change: applied, applied: true };
  } catch (e) {
    if (e instanceof ApiError && (e.status === 403 || e.status === 401)) return { change, applied: false };
    throw e;
  }
}

const common = ['text', 'richtext', 'number', 'integer', 'decimal', 'boolean', 'date', 'timestamp', 'money', 'email', 'phone', 'url', 'address', 'enum', 'reference', 'file'];

export function fieldTypeItems(): MenuItem[] {
  const o = session.ontology;
  if (!o) return [];
  const builtin = o.builtin;
  const first = common.filter((t) => builtin.includes(t));
  const rest = builtin.filter((t) => !first.includes(t)).sort();
  return [
    ...first.map((t) => ({ id: t, label: humanize(t), trailing: o.spec.types[t]?.category ? humanize(o.spec.types[t].category!) : undefined })),
    ...rest.map((t, i) => ({
      id: t,
      label: humanize(t),
      trailing: o.spec.types[t]?.category ? humanize(o.spec.types[t].category!) : undefined,
      separator: i === 0
    }))
  ];
}

export function objectTypeItems(): MenuItem[] {
  return objectTypes(session.ontology).map((t) => ({ id: t, label: humanize(t), icon: 'table' }));
}

export type FieldDraft = { name: string; type: string; required: boolean; list: boolean; to: string; values: string };

export function emptyDraft(): FieldDraft {
  return { name: '', type: 'text', required: false, list: false, to: '', values: '' };
}

export function draftToField(d: FieldDraft): Field {
  const f: Field = { type: d.type };
  if (d.required) f.required = true;
  if (d.list) f.list = true;
  if (d.type === 'reference') f.to = d.to;
  if (d.type === 'enum')
    f.enum = d.values
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean);
  return f;
}

export function checkDraft(d: FieldDraft, taken: string[]): string {
  const n = checkName(d.name, 'field', taken);
  if (n) return n;
  if (d.type === 'reference' && !d.to) return 'Choose which type this field points to.';
  if (d.type === 'enum' && !d.values.trim()) return 'List the allowed values, separated by commas.';
  return '';
}
