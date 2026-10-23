import { api, spaceQuery } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { label, recordTitle } from './ontology.js';

const pageSize = 50;

class RecordTable extends HTMLElement {
  load({ spec, type, fields, where }) {
    Object.assign(this, { spec, type, fields, where, after: undefined });
    this.innerHTML = '<div data-scroll><table><thead><tr></tr></thead><tbody></tbody></table></div><p data-empty hidden>Nothing here yet.</p><button type="button" class="button button-secondary button-sm" data-more hidden>Load more</button>';
    this.querySelector('thead tr').append(
      ...fields.map((f, i) => {
        const th = document.createElement('th');
        th.scope = 'col';
        th.textContent = label(f.key);
        if (i > 1) th.dataset.extra = '';
        return th;
      }),
    );
    this.querySelector('[data-more]').addEventListener('click', () => this.more());
    return this.more();
  }

  async more() {
    const more = this.querySelector('[data-more]');
    more.disabled = true;
    const rows = await api(endpoints.records(this.type), { query: { ...this.where, ...(await spaceQuery()), limit: pageSize, after: this.after } });
    this.querySelector('tbody').append(...rows.map((r) => this.row(r)));
    this.after = rows.at(-1)?.id ?? this.after;
    more.hidden = rows.length < pageSize;
    more.disabled = false;
    this.querySelector('[data-empty]').hidden = Boolean(this.querySelector('tbody tr'));
  }

  row(record) {
    const tr = document.createElement('tr');
    this.fields.forEach((f, i) => {
      const td = document.createElement('td');
      if (i > 1) td.dataset.extra = '';
      if (i === 0) {
        const a = document.createElement('a');
        a.href = pages.record(record.type, record.id);
        a.textContent = recordTitle(this.spec, record);
        td.append(a);
      } else {
        const v = document.createElement('field-value');
        v.show(this.spec, f.field, record.data[f.key]);
        td.append(v);
      }
      tr.append(td);
    });
    return tr;
  }
}

customElements.define('record-table', RecordTable);
