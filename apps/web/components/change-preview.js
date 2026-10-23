import { api } from './api.js';
import { confirmDialog } from './app-dialog.js';
import { endpoints, pages } from './endpoints.js';
import { describeOp, draft, label, loadRecord, loadSpec, recordTitle } from './ontology.js';
import { draftList } from './type-editor.js';
import { alertBox, button, el, listItem, when } from './ui.js';

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

const stable = (v) =>
  Array.isArray(v) ? `[${v.map(stable)}]` : v && typeof v === 'object' ? `{${Object.keys(v).sort().map((k) => `${k}:${stable(v[k])}`)}}` : JSON.stringify(v);

const show = (v) => (v == null ? 'Empty' : typeof v === 'object' ? JSON.stringify(v) : String(v));

const statusWords = {
  applied: ['Applied', 'success'],
  reverted: ['Reverted', 'muted'],
  proposed: ['Waiting for review', 'warning'],
};

class ChangePreview extends HTMLElement {
  async connectedCallback() {
    this.render();
  }

  async render(preview) {
    [this.spec, this.changes] = await Promise.all([loadSpec(true), api(endpoints.changes)]);
    this.innerHTML = `
      <header><div><h1>Changes</h1></div><div><a class="button button-secondary button-sm" data-types><app-icon name="types"></app-icon><span>Edit types</span></a></div></header>
      <div data-draft></div>
      <section data-preview hidden></section>
      <section data-history hidden><h2>History</h2><ul></ul></section>`;
    this.alert = alertBox();
    this.querySelector('[data-draft]').after(this.alert);
    this.querySelector('[data-types]').href = pages.schema();
    const ops = draft.get();
    const slot = this.querySelector('[data-draft]');
    if (ops.length) {
      slot.append(draftList(() => this.render()));
      const actions = document.createElement('footer');
      const discard = button('Discard');
      discard.addEventListener('click', () => {
        draft.set([]);
        this.render();
      });
      const run = button('Preview', null, 'primary');
      run.addEventListener('click', () => this.preview());
      actions.append(discard, run);
      slot.append(actions);
    } else {
      slot.innerHTML = '<p>No changes are waiting. Edit your types to propose one; you will see what it does to your records before anything changes.</p>';
    }
    if (preview) this.showPreview(preview);
    this.showHistory();
  }

  async preview() {
    try {
      const ops = stable(draft.get());
      const open = this.changes.find((c) => c.status === 'proposed' && c.from_version === this.spec.version && stable(c.ops) === ops);
      const change = open ?? (await api(endpoints.changes, { method: 'POST', body: { ops: draft.get() } }));
      this.render(await api(endpoints.preview(change.id), { method: 'POST' }));
    } catch (err) {
      this.alert.show(err);
    }
  }

  async review(change) {
    try {
      this.render(await api(endpoints.preview(change.id), { method: 'POST' }));
    } catch (err) {
      this.alert.show(err);
    }
  }

  recordLink(r) {
    const a = document.createElement('a');
    a.href = pages.record(r.type, r.id);
    a.textContent = label(r.type);
    loadRecord(r.type, r.id).then((rec) => rec && (a.textContent = recordTitle(this.spec, rec)));
    return a;
  }

  showPreview(p) {
    const section = this.querySelector('[data-preview]');
    section.hidden = false;
    section.innerHTML = '<h2>Preview</h2><ul data-ops></ul><p data-summary></p>';
    section.querySelector('[data-ops]').append(...p.change.ops.map((op) => el('li', describeOp(op))));
    section.querySelector('[data-summary]').textContent = p.touched
      ? `This change touches ${plural(p.touched, 'record', 'records')}. ${p.failing === 1 ? 'One of them would no longer be valid; it keeps its old version.' : p.failing ? `${p.failing} of them would no longer be valid; they keep their old version.` : 'All of them stay valid.'}`
      : 'This change touches no records.';
    if (p.failures?.length) {
      const ul = document.createElement('ul');
      ul.dataset.failures = '';
      for (const f of p.failures) {
        const why = el('span', f.errors.map((e) => (e.path ? `${e.path}: ${e.message}` : e.message)).join(', '));
        why.dataset.tone = 'destructive';
        ul.append(listItem([this.recordLink(f), why]));
      }
      section.append(el('h3', 'Would fail validation'), ul);
    }
    const changed = (p.samples ?? [])
      .map((s) => ({
        s,
        keys: [...new Set([...Object.keys(s.before ?? {}), ...Object.keys(s.after ?? {})])].filter(
          (k) => JSON.stringify(s.before?.[k]) !== JSON.stringify(s.after?.[k]),
        ),
      }))
      .filter((x) => x.keys.length);
    if (changed.length) {
      section.append(el('h3', 'Before and after'));
      for (const { s, keys } of changed) {
        const block = document.createElement('div');
        block.dataset.sample = '';
        const title = document.createElement('p');
        title.append(this.recordLink(s));
        const table = document.createElement('table');
        table.innerHTML = '<thead><tr><th scope="col">Field</th><th scope="col">Before</th><th scope="col">After</th></tr></thead><tbody></tbody>';
        table.querySelector('tbody').append(
          ...keys.map((k) => {
            const tr = document.createElement('tr');
            tr.append(el('td', k), el('td', show(s.before?.[k])), el('td', show(s.after?.[k])));
            return tr;
          }),
        );
        block.append(title, table);
        section.append(block);
      }
    }
    const footer = document.createElement('footer');
    const apply = button('Apply change', null, 'primary');
    apply.addEventListener('click', () => this.apply(p.change));
    footer.append(apply);
    section.append(footer);
  }

  async apply(change) {
    try {
      await api(endpoints.apply(change.id), { method: 'POST' });
      if (stable(change.ops) === stable(draft.get())) draft.set([]);
      location.reload();
    } catch (err) {
      this.alert.show(err);
    }
  }

  async revert(change) {
    const ok = await confirmDialog({
      title: 'Revert this change?',
      text: 'Archive applies the opposite change as a new version. Values in fields or types the revert removes are not kept.',
      action: 'Revert',
    });
    if (!ok) return;
    try {
      await api(endpoints.revert(change.id), { method: 'POST' });
      location.reload();
    } catch (err) {
      this.alert.show(err);
    }
  }

  showHistory() {
    const list = this.changes.filter((c) => c.status !== 'proposed' || c.from_version === this.spec.version);
    const section = this.querySelector('[data-history]');
    section.hidden = !list.length;
    section.querySelector('ul').append(
      ...list.map((c) => {
        const what = c.proposed_by.startsWith('revert:') ? [el('p', 'Revert of an earlier change')] : c.ops.map((op) => el('p', describeOp(op)));
        const by = c.proposed_by.startsWith('revert:') ? '' : c.proposed_by;
        const meta = el('p', [when(c.created_at), by && `by ${by}`].filter(Boolean).join(' '));
        meta.dataset.meta = '';
        const [word, tone] = statusWords[c.status] ?? [c.status, 'muted'];
        const status = el('span', word);
        status.dataset.tone = tone;
        const actions = [status];
        if (c.status === 'applied' && c.to_version === this.spec.version) {
          const b = button('Revert');
          b.addEventListener('click', () => this.revert(c));
          actions.push(b);
        }
        if (c.status === 'proposed') {
          const b = button('Review');
          b.addEventListener('click', () => this.review(c));
          actions.push(b);
        }
        return listItem([...what, meta], actions);
      }),
    );
  }
}

customElements.define('change-preview', ChangePreview);
