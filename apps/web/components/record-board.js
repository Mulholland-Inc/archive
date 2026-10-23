import { pages } from './endpoints.js';
import { label, loadAll, recordTitle } from './ontology.js';

export function recordCard(spec, record, fields, skip) {
  const card = document.createElement('article');
  const a = document.createElement('a');
  a.href = pages.record(record.type, record.id);
  a.textContent = recordTitle(spec, record);
  card.append(a);
  fields
    .slice(1)
    .filter((f) => f.key !== skip && record.data[f.key] != null)
    .slice(0, 2)
    .forEach((f) => {
      const v = document.createElement('field-value');
      v.show(spec, f.field, record.data[f.key]);
      card.append(v);
    });
  return card;
}

class RecordBoard extends HTMLElement {
  async load({ spec, type, fields, where, by }) {
    const field = fields.find((f) => f.key === by).field;
    const records = await loadAll(type, where);
    const groups = [...field.enum.map((v) => [v, v]), ['', `No ${label(by).toLowerCase()}`]];
    this.replaceChildren(
      ...groups
        .map(([value, title]) => {
          const items = records.filter((r) => (r.data[by] ?? '') === value);
          if (!value && !items.length) return null;
          const column = document.createElement('section');
          const h = document.createElement('h2');
          h.textContent = title;
          column.append(h, ...items.map((r) => recordCard(spec, r, fields, by)));
          return column;
        })
        .filter(Boolean),
    );
  }
}

customElements.define('record-board', RecordBoard);
