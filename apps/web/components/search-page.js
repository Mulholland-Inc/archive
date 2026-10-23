import { api, currentSpace, loadMe, spaceLabel } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { isoDate } from './format.js';
import { fieldsOf, label, loadSpec, recordTitle, userTypes } from './ontology.js';
import { alertBox, el, header, select, snippet, when } from './ui.js';

const ranges = [
  ['', 'Any time'],
  ['1', 'Today'],
  ['7', 'Past week'],
  ['31', 'Past month'],
  ['366', 'Past year'],
];

const filters = ['type', 'space', 'source', 'from', 'to'];

class SearchPage extends HTMLElement {
  async connectedCallback() {
    const [spec, me, space, sources] = await Promise.all([loadSpec(), loadMe(), currentSpace(), api(endpoints.sources).catch(() => [])]);
    Object.assign(this, { spec, spaces: me.spaces, home: me.home, sources });
    const params = new URLSearchParams(location.search);
    if (!params.has('space') && !params.has('all') && space) params.set('space', space.name);
    this.params = params;
    this.alert = alertBox();
    this.toolbar = document.createElement('div');
    this.toolbar.dataset.toolbar = '';
    this.results = document.createElement('ol');
    this.results.dataset.results = '';
    this.summary = el('p');
    this.summary.dataset.summary = '';
    this.replaceChildren(header('Search'), this.toolbar, this.alert, this.summary, this.results);
    this.filters();
    addEventListener('app-search', (e) => this.go({ q: e.detail }));
    this.run();
  }

  go(changes) {
    for (const [k, v] of Object.entries(changes)) {
      if (v) this.params.set(k, v);
      else this.params.delete(k);
    }
    if ('space' in changes) {
      if (changes.space) this.params.delete('all');
      else this.params.set('all', '1');
    }
    history.replaceState(null, '', pages.search(Object.fromEntries(this.params)));
    this.run();
  }

  filters() {
    const p = this.params;
    const make = (aria, options, key) => {
      const s = select(aria, options, p.get(key) ?? '');
      s.addEventListener('change', () => this.go({ [key]: s.value }));
      return s;
    };
    const type = make('Type', [{ value: '', label: 'All types' }, ...userTypes(this.spec).map((t) => ({ value: t, label: label(t) })), { value: 'file', label: 'Files' }], 'type');
    const space = make('Space', [{ value: '', label: 'All spaces' }, ...this.spaces.map((s) => ({ value: s.name, label: spaceLabel(s, this.home) }))], 'space');
    const source = make('Source', [{ value: '', label: 'All sources' }, ...this.sources.map((s) => ({ value: s.id, label: s.name }))], 'source');
    const days = ranges.find(([d]) => d && p.get('from') === isoDate(new Date(Date.now() - (Number(d) - 1) * 864e5)))?.[0] ?? '';
    const custom = !days && (p.get('from') || p.get('to'));
    const options = ranges.map(([value, text]) => ({ value, label: text }));
    if (custom) options.push({ value: 'custom', label: [p.get('from') && `From ${p.get('from')}`, p.get('to') && `until ${p.get('to')}`].filter(Boolean).join(' ') });
    const range = select('Date', options, custom ? 'custom' : days);
    range.addEventListener('change', () => {
      if (range.value === 'custom') return;
      this.go({ from: range.value ? isoDate(new Date(Date.now() - (Number(range.value) - 1) * 864e5)) : '', to: '' });
    });
    this.toolbar.replaceChildren(type, space, source, range);
  }

  async run() {
    const q = (this.params.get('q') ?? '').trim();
    this.alert.show('');
    if (!q) {
      this.summary.hidden = false;
      this.summary.textContent = 'Type in the search box to find records, files and synced items by their words and meaning.';
      this.results.replaceChildren();
      return;
    }
    const query = { q, limit: 50, ...Object.fromEntries(filters.map((k) => [k, this.params.get(k)])) };
    const seq = (this.seq = (this.seq ?? 0) + 1);
    try {
      const hits = await api(endpoints.search, { query });
      if (seq !== this.seq) return;
      this.summary.textContent = hits.length ? '' : 'Nothing matches. Try other words or fewer filters.';
      this.summary.hidden = hits.length > 0;
      this.results.replaceChildren(...hits.map((h) => this.hit(h)));
    } catch (err) {
      if (seq !== this.seq) return;
      this.results.replaceChildren();
      this.alert.show(err);
    }
  }

  hit(h) {
    const li = document.createElement('li');
    const data = h.data ?? {};
    const space = this.spaces.find((s) => s.id === h.space);
    let title;
    let kind;
    if (h.kind === 'raw') {
      const src = this.sources.find((s) => h.source === `${s.kind}:${s.id}` || h.source === `folder:${s.config?.path}`);
      title = el('p', src ? `Synced from ${src.name}` : 'Synced item');
      kind = 'Not mapped into a type yet';
    } else if (h.type === 'file') {
      title = el('a', data.name);
      title.href = pages.file({ id: h.id });
      kind = 'File';
    } else {
      title = el('a', this.spec.types[h.type] ? recordTitle(this.spec, h) : label(h.type));
      title.href = pages.record(h.type, h.id);
      kind = label(h.type);
    }
    title.dataset.title = '';
    const meta = el('p', `${kind}${space ? ` in ${spaceLabel(space, this.home)}` : ''}, ${when(h.date, { dateStyle: 'medium' })}`);
    meta.dataset.meta = '';
    li.append(title, meta);
    if (h.snippet) li.append(snippet(h.snippet));
    else if (h.kind === 'record' && h.type !== 'file' && this.spec.types[h.type]) {
      const values = fieldsOf(this.spec, h.type)
        .slice(1)
        .filter((f) => data[f.key] != null)
        .slice(0, 3);
      if (values.length) {
        const p = document.createElement('p');
        p.dataset.snippet = '';
        values.forEach((f, i) => {
          const v = document.createElement('field-value');
          v.setAttribute('plain', '');
          v.show(this.spec, f.field, data[f.key]);
          p.append(...(i ? [', ', v] : [v]));
        });
        li.append(p);
      }
    }
    return li;
  }
}

customElements.define('search-page', SearchPage);
