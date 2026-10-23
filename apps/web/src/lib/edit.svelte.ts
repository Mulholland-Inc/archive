import { api, ApiError } from './api';
import { remember } from './refs.svelte';
import { session } from './session.svelte';
import { inputValue, isSecret, kindOf, parseInput } from './spec';
import type { ArchiveRecord, Field } from './types';

export class RecordEdit {
  active = $state(false);
  busy = $state(false);
  values = $state<{ [k: string]: string }>({});
  checks = $state<{ [k: string]: boolean }>({});
  errors = $state<{ [k: string]: string }>({});
  failure = $state('');
  #record: ArchiveRecord | null = null;
  #fields: [string, Field][] = [];
  #original: { [k: string]: string | boolean } = {};

  begin(record: ArchiveRecord, fields: [string, Field][]) {
    const o = session.ontology;
    this.#record = record;
    this.#fields = fields;
    const v: { [k: string]: string } = {};
    const c: { [k: string]: boolean } = {};
    const orig: { [k: string]: string | boolean } = {};
    for (const [name, f] of fields) {
      v[name] = '';
      c[name] = false;
      if (kindOf(o, f.type) === 'boolean' && !f.list) orig[name] = c[name] = record.data[name] === true;
      else orig[name] = v[name] = inputValue(o, f, record.data[name]);
    }
    this.values = v;
    this.checks = c;
    this.#original = orig;
    this.errors = {};
    this.failure = '';
    this.active = true;
  }

  cancel() {
    this.active = false;
    this.errors = {};
    this.failure = '';
  }

  async save(): Promise<ArchiveRecord | null> {
    const record = this.#record;
    if (!record) return null;
    const o = session.ontology;
    const data: { [k: string]: unknown } = {};
    for (const [name, f] of this.#fields) {
      const isBool = kindOf(o, f.type) === 'boolean' && !f.list;
      const now = isBool ? this.checks[name] : this.values[name];
      if (now === this.#original[name]) continue;
      if (!isBool && now === '' && isSecret(record.data[name])) continue;
      data[name] = isBool ? now : parseInput(o, f, now as string);
    }
    if (!Object.keys(data).length) {
      this.cancel();
      return null;
    }
    this.busy = true;
    this.errors = {};
    this.failure = '';
    try {
      const saved = await api.patch(record.type, record.id, data);
      remember(saved);
      this.active = false;
      return saved;
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) {
        const errors: { [k: string]: string } = {};
        for (const f of err.fields) {
          const key = f.path.split(/[.[]/)[0];
          if (this.#fields.some(([n]) => n === key)) errors[key] = f.message;
          else this.failure = `${f.path}: ${f.message}`;
        }
        this.errors = errors;
      } else this.failure = (err as Error).message;
      return null;
    } finally {
      this.busy = false;
    }
  }
}

export function secretTypes() {
  return new Set(Object.entries(session.ontology?.spec.types ?? {}).filter(([, t]) => t.sensitivity === 'secret').map(([n]) => n));
}
