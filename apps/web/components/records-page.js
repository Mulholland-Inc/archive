import { api, currentSpace } from './api.js';
import { confirmDialog, openDialog } from './app-dialog.js';
import { viewParams } from './app-shell.js';
import { endpoints, pages } from './endpoints.js';
import { kind } from './format.js';
import { fieldsOf, isUserType, label, loadSpec, userTypes } from './ontology.js';
import { alertBox, button, field, textInput } from './ui.js';

const prefix = 'where.';

const sameView = (a, b) => {
  const key = (v) => JSON.stringify([v.layout, v.by ?? '', Object.entries(v.where ?? {}).sort()]);
  return key(a) === key(b);
};

const nameDialog = (title, action, value, onName) => {
  const input = textInput('view-name', value);
  return openDialog({
    title,
    body: field('Name', input, 'view-name'),
    actions: [
      { label: 'Cancel', value: '' },
      { label: action, value: 'ok', kind: 'primary' },
    ],
    onAction: () => {
      if (!input.value.trim()) throw new Error('Give the view a name.');
      return onName(input.value.trim());
    },
  });
};

class RecordsPage extends HTMLElement {
  connectedCallback() {
    this.onPop = () => this.render();
    addEventListener('popstate', this.onPop);
    this.render();
  }

  disconnectedCallback() {
    removeEventListener('popstate', this.onPop);
  }

  go(changes) {
    const params = new URLSearchParams(location.search);
    for (const [k, v] of Object.entries(changes)) {
      if (v) params.set(k, v);
      else params.delete(k);
    }
    history.pushState(null, '', `?${params}`);
    this.render();
  }

  async render() {
    const params = new URLSearchParams(location.search);
    const type = params.get('type');
    const spec = await loadSpec();
    if (!type) {
      const first = userTypes(spec)[0];
      if (first) return location.replace(pages.records(first));
      this.innerHTML = `<header><h1>Nothing here yet</h1></header><p>Archive keeps your records in types you define, like invoices or people. Start with your first type.</p><p><a class="button button-primary button-sm" href="${pages.schema('new')}">Create a type</a></p>`;
      return;
    }
    if (!isUserType(spec, type)) {
      this.innerHTML = '<header><h1>Type not found</h1></header><p>This type does not exist, or it was removed.</p>';
      return;
    }

    const fields = fieldsOf(spec, type);
    const choices = {
      board: fields.filter((f) => f.field.type === 'enum' && !f.field.list).map((f) => ({ value: f.key, label: `By ${label(f.key).toLowerCase()}` })),
      calendar: fields.filter((f) => ['date', 'timestamp'].includes(kind(spec, f.field)) && !f.field.list).map((f) => ({ value: f.key, label: `By ${label(f.key).toLowerCase()}` })),
    };
    let view = params.get('view') ?? 'table';
    if (!choices[view]?.length) view = 'table';
    const by = choices[view]?.some((c) => c.value === params.get('by')) ? params.get('by') : choices[view]?.[0]?.value;
    const where = Object.fromEntries([...params].filter(([k]) => k.startsWith(prefix)).map(([k, v]) => [k.slice(prefix.length), v]));
    const config = { type, layout: view, by: view === 'table' ? '' : by, where };
    const saved = params.get('saved') ? await api(endpoints.view(params.get('saved'))).catch(() => null) : null;

    this.innerHTML = `
      <header>
        <div><h1></h1></div>
        <div data-actions>
          <a class="button button-secondary button-sm" data-fields>Edit fields</a>
          <a class="button button-primary button-sm" data-new><app-icon name="plus"></app-icon><span></span></a>
        </div>
      </header>
      <div data-toolbar><view-switch></view-switch><filter-bar></filter-bar></div>`;
    const alert = alertBox();
    this.append(alert);
    this.querySelector('h1').textContent = saved?.name ?? label(type);
    if (saved) {
      const p = document.createElement('p');
      const a = document.createElement('a');
      a.href = pages.records(type);
      a.textContent = `All ${label(type).toLowerCase()} records`;
      p.append(a);
      this.querySelector('header > div').append(p);
    }
    this.querySelector('[data-fields]').href = pages.schema(type);
    this.querySelector('[data-new]').href = pages.record(type);
    this.querySelector('[data-new] span').textContent = `New ${label(type).toLowerCase()}`;
    this.viewActions(saved, config, alert);

    const viewSwitch = this.querySelector('view-switch');
    viewSwitch.show({ view, by, choices });
    viewSwitch.addEventListener('view', (e) => this.go({ view: e.detail.view === 'table' ? '' : e.detail.view, by: e.detail.by, month: '' }));
    const filters = this.querySelector('filter-bar');
    filters.show({ spec, fields, where });
    filters.addEventListener('filter', (e) => {
      const changes = Object.fromEntries(Object.keys(where).map((k) => [prefix + k, '']));
      for (const [k, v] of Object.entries(e.detail)) changes[prefix + k] = v;
      this.go(changes);
    });

    const content = document.createElement(`record-${view}`);
    this.append(content);
    try {
      await content.load({ spec, type, fields, where, by, month: params.get('month'), onMonth: (month) => this.go({ month }) });
    } catch (err) {
      alert.show(err);
      content.remove();
    }
  }

  viewActions(saved, config, alert) {
    const bar = this.querySelector('[data-toolbar]');
    const done = (v) => {
      dispatchEvent(new Event('app-views'));
      history.replaceState(null, '', pages.records(v.type, viewParams(v)));
      this.render();
    };
    if (!saved) {
      const save = button('Save view', 'view');
      save.addEventListener('click', () =>
        nameDialog('Save this view', 'Save view', '', async (name) => {
          const space = (await currentSpace())?.name;
          done(await api(endpoints.views, { method: 'POST', body: { ...config, name, space } }));
        }),
      );
      bar.append(save);
      return;
    }
    const update = button('Save changes', 'check');
    update.hidden = sameView(saved, config);
    update.addEventListener('click', async () => {
      try {
        done(await api(endpoints.view(saved.id), { method: 'PUT', body: { ...config, name: saved.name } }));
      } catch (err) {
        alert.show(err);
      }
    });
    const rename = button('Rename', 'rename');
    rename.addEventListener('click', () =>
      nameDialog('Rename view', 'Rename', saved.name, async (name) => {
        done(await api(endpoints.view(saved.id), { method: 'PUT', body: { type: saved.type, layout: saved.layout, by: saved.by, where: saved.where, name } }));
      }),
    );
    const remove = button('Delete view', 'trash');
    remove.addEventListener('click', async () => {
      if (!(await confirmDialog({ title: `Delete ${saved.name}?`, text: 'The saved view goes away. The records in it stay.', action: 'Delete view' }))) return;
      try {
        await api(endpoints.view(saved.id), { method: 'DELETE' });
        dispatchEvent(new Event('app-views'));
        location.assign(pages.records(saved.type));
      } catch (err) {
        alert.show(err);
      }
    });
    bar.append(update, rename, remove);
  }
}

customElements.define('records-page', RecordsPage);
