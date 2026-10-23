import { api, spaceQuery } from './api.js';
import { confirmDialog } from './app-dialog.js';
import { endpoints, pages } from './endpoints.js';
import { confidenceWord } from './format.js';
import { fieldsOf, isUserType, label, loadSpec, recordTitle } from './ontology.js';

function source(p) {
  if (!p) return '';
  const [method, model] = p.method.split(/:(.*)/);
  let text = method === 'manual' ? 'Entered by hand' : method === 'intake' ? `Read by ${model || 'the intake model'}` : p.method;
  if (p.raw_ids?.length) text += ' from the original source';
  if (p.original) text += `, ${method === 'manual' ? 'typed' : 'read'} as “${p.original}”`;
  return text;
}

class RecordView extends HTMLElement {
  async connectedCallback() {
    const params = new URLSearchParams(location.search);
    this.type = params.get('type');
    this.recordId = params.get('id');
    this.spec = await loadSpec();
    if (!isUserType(this.spec, this.type)) {
      this.innerHTML = '<header><h1>Type not found</h1></header>';
      return;
    }
    if (!this.recordId) return this.create();
    try {
      this.record = await api(endpoints.record(this.type, this.recordId));
    } catch (err) {
      this.innerHTML = `<header><h1>Record not found</h1></header><p></p>`;
      this.querySelector('p').textContent = err.message;
      return;
    }
    this.fields = fieldsOf(this.spec, this.record.type);
    this.innerHTML = `
      <header>
        <div><h1></h1><p><a data-type></a></p></div>
        <div><button type="button" class="button button-secondary button-sm" data-delete><app-icon name="trash"></app-icon><span>Delete</span></button></div>
      </header>
      <dl></dl>`;
    const typeLink = this.querySelector('[data-type]');
    typeLink.href = pages.records(this.record.type);
    typeLink.textContent = label(this.record.type);
    this.querySelector('[data-delete]').addEventListener('click', () => this.confirmDelete());
    this.querySelector('dl').append(...this.fields.map((f) => this.row(f)));
    this.showTitle();
    this.provenance();
  }

  showTitle() {
    const title = recordTitle(this.spec, this.record);
    this.querySelector('h1').textContent = title;
    document.title = `${title}, Archive`;
  }

  async provenance() {
    const list = await api(endpoints.provenance(this.record.type, this.recordId)).catch(() => []);
    const latest = {};
    for (const p of [...(list ?? [])].sort((a, b) => a.id.localeCompare(b.id))) latest[p.field_path.split(/[.[]/)[0]] = p;
    for (const f of this.fields) {
      const row = this.querySelector(`[data-key="${CSS.escape(f.key)}"]`);
      const p = this.record.data[f.key] == null ? null : latest[f.key];
      row.querySelector('[data-source]').textContent = source(p);
      const conf = row.querySelector('[data-confidence]');
      const [word, tone] = p ? confidenceWord(p.confidence) : ['', ''];
      conf.textContent = word;
      conf.dataset.tone = tone;
    }
  }

  row(f) {
    const row = document.createElement('div');
    row.dataset.key = f.key;
    row.innerHTML = `
      <dt></dt>
      <dd data-value><field-value empty="Empty"></field-value></dd>
      <dd data-source></dd>
      <dd data-confidence></dd>
      <dd data-actions><button type="button" class="button button-secondary button-sm" data-edit><app-icon name="edit"></app-icon></button></dd>`;
    row.querySelector('dt').textContent = label(f.key);
    row.querySelector('[data-edit]').setAttribute('aria-label', `Edit ${label(f.key)}`);
    row.querySelector('field-value').show(this.spec, f.field, this.record.data[f.key]);
    row.querySelector('[data-edit]').addEventListener('click', () => this.edit(row, f));
    return row;
  }

  edit(row, f) {
    const cell = row.querySelector('[data-value]');
    const actions = row.querySelector('[data-actions]');
    row.dataset.editing = '';
    const input = document.createElement('field-input');
    cell.replaceChildren(input);
    input.edit(this.spec, f.field, this.record.data[f.key], `edit-${f.key}`);
    row.querySelector('dt').innerHTML = `<label for="edit-${f.key}"></label>`;
    row.querySelector('dt label').textContent = label(f.key);
    actions.innerHTML = '<button type="button" class="button button-secondary button-sm" data-cancel>Cancel</button><button type="button" class="button button-primary button-sm" data-save>Save</button>';
    const close = () => {
      delete row.dataset.editing;
      row.replaceWith(this.row(f));
      this.provenance();
    };
    const initial = JSON.stringify(input.value);
    const save = async () => {
      if (JSON.stringify(input.value) === initial) return close();
      try {
        this.record = await api(endpoints.record(this.record.type, this.recordId), { method: 'PATCH', body: { [f.key]: input.value } });
        this.showTitle();
        close();
      } catch (err) {
        input.error = err.fields?.[f.key] ?? [err.message];
      }
    };
    actions.querySelector('[data-cancel]').addEventListener('click', close);
    actions.querySelector('[data-save]').addEventListener('click', save);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && e.target.tagName === 'INPUT') save();
      if (e.key === 'Escape' && e.target.tagName === 'INPUT') close();
    });
    input.focus();
  }

  async confirmDelete() {
    const title = recordTitle(this.spec, this.record);
    const ok = await confirmDialog({
      title: `Delete ${title}?`,
      text: 'The record, its provenance and its links are deleted. This cannot be undone.',
      action: 'Delete',
    });
    if (!ok) return;
    await api(endpoints.record(this.record.type, this.recordId), { method: 'DELETE' });
    location.assign(pages.records(this.record.type));
  }

  create() {
    this.fields = fieldsOf(this.spec, this.type);
    document.title = `New ${label(this.type).toLowerCase()}, Archive`;
    this.innerHTML = `
      <header><div><h1></h1></div></header>
      <form novalidate>
        <dl></dl>
        <p role="alert" hidden></p>
        <footer>
          <a class="button button-secondary button-sm" data-cancel>Cancel</a>
          <button class="button button-primary button-sm">Create</button>
        </footer>
      </form>`;
    this.querySelector('h1').textContent = `New ${label(this.type).toLowerCase()}`;
    this.querySelector('[data-cancel]').href = pages.records(this.type);
    const inputs = {};
    this.querySelector('dl').append(
      ...this.fields.map((f) => {
        const row = document.createElement('div');
        row.dataset.editing = '';
        row.innerHTML = `<dt><label for="new-${f.key}"></label></dt><dd data-value><field-input></field-input></dd>`;
        row.querySelector('label').textContent = label(f.key) + (f.field.required ? '' : ', optional');
        inputs[f.key] = row.querySelector('field-input');
        inputs[f.key].edit(this.spec, f.field, null, `new-${f.key}`);
        return row;
      }),
    );
    this.querySelector('form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const data = Object.fromEntries(Object.entries(inputs).map(([k, input]) => [k, input.value]).filter(([, v]) => v != null));
      const alert = this.querySelector('[role=alert]');
      alert.hidden = true;
      Object.values(inputs).forEach((input) => (input.error = []));
      try {
        const r = await api(endpoints.records(this.type), { method: 'POST', body: data, query: await spaceQuery() });
        location.assign(pages.record(r.type, r.id));
      } catch (err) {
        let shown = false;
        for (const [k, messages] of Object.entries(err.fields ?? {})) {
          if (inputs[k]) {
            inputs[k].error = messages;
            shown = true;
          }
        }
        if (!shown) {
          alert.textContent = err.message;
          alert.hidden = false;
        }
      }
    });
  }
}

customElements.define('record-view', RecordView);
