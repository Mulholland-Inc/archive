import { confirmDialog, openDialog } from './app-dialog.js';
import { pages } from './endpoints.js';
import { describeOp, draft, fieldsOf, isUserType, label, loadSpec, typeLabel, userTypes } from './ontology.js';
import { button, field, select, textInput, toggle } from './ui.js';

const namePattern = /^[a-z][a-z0-9_]*$/;

export function draftList(onChange) {
  const ops = draft.get();
  const section = document.createElement('section');
  section.dataset.draft = '';
  if (!ops.length) return section;
  section.innerHTML = '<h2>Waiting for review</h2><ul></ul>';
  section.querySelector('ul').append(
    ...ops.map((op, i) => {
      const li = document.createElement('li');
      const span = document.createElement('span');
      span.textContent = describeOp(op);
      const remove = button('Remove', 'close');
      remove.setAttribute('aria-label', `Remove: ${describeOp(op)}`);
      remove.addEventListener('click', () => {
        draft.set(draft.get().filter((_, j) => j !== i));
        onChange();
      });
      li.append(span, remove);
      return li;
    }),
  );
  return section;
}

function typeText(f) {
  if (f.type === 'reference') return `Reference to ${label(f.to)}`;
  if (f.type === 'enum') return `Choice of ${(f.enum ?? []).join(', ')}`;
  return typeLabel(f.type);
}

class TypeEditor extends HTMLElement {
  async connectedCallback() {
    this.spec = await loadSpec(true);
    this.render();
  }

  render() {
    const type = new URLSearchParams(location.search).get('type');
    if (type === 'new') {
      this.list();
      this.typeDialog();
      history.replaceState(null, '', pages.schema());
    } else if (type && isUserType(this.spec, type)) this.type(type);
    else this.list();
  }

  queue(op) {
    draft.add(op);
    this.render();
  }

  head(title, actions) {
    const header = document.createElement('header');
    header.innerHTML = '<div><h1></h1></div><div></div>';
    header.querySelector('h1').textContent = title;
    header.lastElementChild.append(...actions);
    return header;
  }

  review() {
    const a = document.createElement('a');
    a.className = 'button button-primary button-sm';
    a.href = pages.changes;
    a.textContent = 'Review changes';
    return a;
  }

  list() {
    const add = button('New type', 'plus', draft.get().length ? 'secondary' : 'primary');
    add.addEventListener('click', () => this.typeDialog());
    const actions = draft.get().length ? [add, this.review()] : [add];
    const table = document.createElement('table');
    table.innerHTML = '<thead><tr><th scope="col">Type</th><th scope="col">Based on</th><th scope="col" data-extra>Description</th></tr></thead><tbody></tbody>';
    table.querySelector('tbody').append(
      ...userTypes(this.spec).map((name) => {
        const t = this.spec.types[name];
        const tr = document.createElement('tr');
        tr.innerHTML = '<td><a></a></td><td></td><td data-extra></td>';
        tr.querySelector('a').href = pages.schema(name);
        tr.querySelector('a').textContent = label(name);
        tr.children[1].textContent = t.extends ? label(t.extends) : '';
        tr.children[2].textContent = t.description ?? '';
        return tr;
      }),
    );
    const wrap = document.createElement('div');
    wrap.dataset.scroll = '';
    wrap.append(table);
    this.replaceChildren(this.head('Types', actions), draftList(() => this.render()), wrap);
  }

  type(name) {
    const t = this.spec.types[name];
    const pending = draft.get().length > 0;
    const add = button('Add field', 'plus', pending ? 'secondary' : 'primary');
    add.addEventListener('click', () => this.fieldDialog(name));
    const edit = button('Edit type', 'edit');
    edit.addEventListener('click', () => this.typeDialog(name));
    const remove = button('Remove type', 'trash');
    remove.addEventListener('click', async () => {
      const ok = await confirmDialog({
        title: `Remove ${label(name)}?`,
        text: 'The type and the data in its records will be removed when you apply this change.',
        action: 'Remove type',
      });
      if (ok) this.queue({ op: 'remove_type', type: name });
    });
    const header = this.head(label(name), pending ? [remove, edit, add, this.review()] : [remove, edit, add]);
    const about = header.querySelector('div');
    if (t.description) about.append(Object.assign(document.createElement('p'), { textContent: t.description }));
    if (t.extends) {
      const p = document.createElement('p');
      const a = document.createElement('a');
      a.href = pages.schema(t.extends);
      a.textContent = label(t.extends);
      p.append('Based on ', a);
      about.append(p);
    }

    const table = document.createElement('table');
    table.innerHTML = '<thead><tr><th scope="col">Field</th><th scope="col">Type</th><th scope="col" data-extra>Rules</th><th scope="col"><span class="visually-hidden">Actions</span></th></tr></thead><tbody></tbody>';
    table.querySelector('tbody').append(...fieldsOf(this.spec, name).map((f) => this.fieldRow(name, f)));
    const wrap = document.createElement('div');
    wrap.dataset.scroll = '';
    wrap.append(table);
    const back = document.createElement('p');
    back.innerHTML = '<a></a>';
    back.querySelector('a').href = pages.schema();
    back.querySelector('a').textContent = 'All types';
    this.replaceChildren(header, draftList(() => this.render()), wrap, back);
  }

  fieldRow(type, f) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td></td><td></td><td data-extra></td><td data-actions></td>';
    tr.children[0].textContent = f.key;
    tr.children[1].textContent = typeText(f.field);
    tr.children[2].textContent = [f.field.required && 'Required', f.field.list && 'List', f.field.indexed && 'Indexed'].filter(Boolean).join(', ');
    const cell = tr.children[3];
    if (f.from !== type) {
      const a = document.createElement('a');
      a.href = pages.schema(f.from);
      a.textContent = `From ${label(f.from)}`;
      cell.append(a);
      tr.dataset.inherited = '';
      return tr;
    }
    const edit = button('Edit', 'edit');
    edit.setAttribute('aria-label', `Edit ${f.key}`);
    edit.addEventListener('click', () => this.fieldDialog(type, f));
    const rename = button('Rename', 'rename');
    rename.setAttribute('aria-label', `Rename ${f.key}`);
    rename.addEventListener('click', () => this.renameDialog(type, f));
    const remove = button('Remove', 'trash');
    remove.setAttribute('aria-label', `Remove ${f.key}`);
    remove.addEventListener('click', async () => {
      const ok = await confirmDialog({
        title: `Remove ${f.key}?`,
        text: `The ${f.key} value in every ${label(type).toLowerCase()} will be dropped when you apply this change.`,
        action: 'Remove field',
      });
      if (ok) this.queue({ op: 'remove_field', type, field: f.key });
    });
    cell.append(edit, rename, remove);
    return tr;
  }

  async fieldDialog(type, existing) {
    const def = existing?.field ?? { type: 'text' };
    const builtins = Object.keys(this.spec.types).filter((n) => !isUserType(this.spec, n) && n !== 'reference' && n !== 'enum');
    const kinds = [
      ...builtins.map((n) => ({ value: n, label: typeLabel(n) })),
      { value: 'enum', label: 'Choice' },
      { value: 'reference', label: 'Reference' },
    ].sort((a, b) => a.label.localeCompare(b.label));
    const name = textInput('field-name', existing?.key ?? '');
    const kind = select('Field type', kinds, def.type);
    const to = select('Refers to', userTypes(this.spec).map((n) => ({ value: n, label: label(n) })), def.to ?? userTypes(this.spec)[0]);
    const choices = textInput('field-choices', (def.enum ?? []).join(', '));
    const description = textInput('field-description', def.description ?? '');
    const required = toggle('Required', def.required);
    const list = toggle('List of values', def.list);
    const toRow = field('Refers to', to);
    const choicesRow = field('Choices, separated by commas', choices, 'field-choices');
    const sync = () => {
      toRow.hidden = kind.value !== 'reference';
      choicesRow.hidden = kind.value !== 'enum';
    };
    kind.addEventListener('change', sync);
    const body = document.createElement('div');
    body.append(
      ...(existing ? [] : [field('Name', name, 'field-name')]),
      field('Type', kind),
      toRow,
      choicesRow,
      field('Description, optional', description, 'field-description'),
      required,
      list,
    );
    sync();
    await openDialog({
      title: existing ? `Edit ${existing.key}` : `Add a field to ${label(type)}`,
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: existing ? 'Save' : 'Add field', value: 'ok', kind: 'primary' },
      ],
      onAction: () => {
        const key = existing?.key ?? name.value.trim();
        if (!namePattern.test(key)) throw new Error('Use lowercase letters, digits and underscores, starting with a letter.');
        const next = { type: kind.value };
        if (kind.value === 'reference') next.to = to.value;
        if (kind.value === 'enum') next.enum = choices.value.split(',').map((s) => s.trim()).filter(Boolean);
        if (kind.value === 'enum' && !next.enum.length) throw new Error('Add at least one choice.');
        if (description.value.trim()) next.description = description.value.trim();
        if (required.checked) next.required = true;
        if (list.checked) next.list = true;
        if (def.indexed) next.indexed = true;
        draft.add({ op: existing ? 'change_field' : 'add_field', type, field: key, def: next });
      },
    }).then((v) => v && this.render());
  }

  renameDialog(type, f) {
    const input = textInput('rename-field', f.key);
    openDialog({
      title: `Rename ${f.key}`,
      body: field('New name', input, 'rename-field'),
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Rename', value: 'ok', kind: 'primary' },
      ],
      onAction: () => {
        const to = input.value.trim();
        if (!namePattern.test(to)) throw new Error('Use lowercase letters, digits and underscores, starting with a letter.');
        if (to === f.key) return;
        draft.add({ op: 'rename_field', type, field: f.key, to });
      },
    }).then((v) => v && this.render());
  }

  typeDialog(existing) {
    const t = existing ? this.spec.types[existing] : {};
    const name = textInput('type-name');
    const parents = userTypes(this.spec).filter((n) => n !== existing);
    const extendsSelect = select('Based on', [{ value: '', label: 'Nothing' }, ...parents.map((n) => ({ value: n, label: label(n) }))], t.extends ?? '');
    const description = textInput('type-description', t.description ?? '');
    const body = document.createElement('div');
    body.append(
      ...(existing ? [] : [field('Name', name, 'type-name')]),
      field('Based on', extendsSelect),
      field('Description, optional', description, 'type-description'),
    );
    openDialog({
      title: existing ? `Edit ${label(existing)}` : 'New type',
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: existing ? 'Save' : 'Add type', value: 'ok', kind: 'primary' },
      ],
      onAction: () => {
        const key = existing ?? name.value.trim();
        if (!namePattern.test(key)) throw new Error('Use lowercase letters, digits and underscores, starting with a letter.');
        if (!existing && this.spec.types[key]) throw new Error(`There is already a type called ${key}.`);
        const def = {};
        if (description.value.trim()) def.description = description.value.trim();
        if (extendsSelect.value) def.extends = extendsSelect.value;
        draft.add(existing ? { op: 'change_type', type: key, def } : { op: 'add_type', type: key, def: { ...def, fields: {} } });
      },
    }).then((v) => v && this.render());
  }
}

customElements.define('type-editor', TypeEditor);
