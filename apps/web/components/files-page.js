import { api, currentSpace } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { locale } from './format.js';
import { label, loadSpec, userTypes } from './ontology.js';
import { alertBox, button, bytes, el, header, select, when } from './ui.js';
import { upload } from './upload.js';

const names = { 'application/pdf': 'PDF document', 'text/plain': 'Text', 'text/csv': 'Spreadsheet', 'text/html': 'Web page', 'message/rfc822': 'Email' };

export const mimeLabel = (mime = '') => {
  const [top, sub = ''] = mime.split(';')[0].split('/');
  if (names[mime.split(';')[0]]) return names[mime.split(';')[0]];
  if (top === 'image') return `${sub.replace(/^x-/, '').toUpperCase()} image`;
  if (top === 'video') return 'Video';
  if (top === 'audio') return 'Audio';
  return mime;
};

const fileStatus = (s) => ({ pending: ['Waiting', 'muted'], processing: ['Processing', 'warning'], done: ['Ready', 'success'], error: ['Failed', 'destructive'] })[s] ?? [s, 'muted'];

export const statusWord = (s) => {
  const [word, tone] = fileStatus(s);
  const span = el('span', word);
  span.dataset.tone = tone;
  return span;
};

const views = [
  ['table', 'List'],
  ['gallery', 'Gallery'],
];

const takenDay = (f) => String(f.metadata?.taken_at ?? f.created_at).slice(0, 10);

class FilesPage extends HTMLElement {
  async connectedCallback() {
    const [spec, space] = await Promise.all([loadSpec(), currentSpace()]);
    this.space = space;
    const pick = button('Upload files', 'upload', 'primary');
    const intake = select('Read into a type', [{ value: '', label: 'Keep as files' }, ...userTypes(spec).map((t) => ({ value: t, label: `Read into ${label(t).toLowerCase()}` }))], '');
    this.intake = intake;
    this.input = Object.assign(document.createElement('input'), { type: 'file', multiple: true, hidden: true });
    pick.addEventListener('click', () => this.input.click());
    this.input.addEventListener('change', () => {
      this.send([...this.input.files]);
      this.input.value = '';
    });
    this.alert = alertBox();
    this.uploads = el('ul');
    this.uploads.dataset.uploads = '';
    this.uploads.hidden = true;
    this.drop = el('p', 'Drop files anywhere on this page to upload them.');
    this.drop.dataset.drop = '';
    this.content = document.createElement('div');
    this.content.dataset.content = '';
    this.switch = document.createElement('view-switch');
    const toolbar = document.createElement('div');
    toolbar.dataset.toolbar = '';
    toolbar.append(this.switch, intake);
    this.replaceChildren(header('Files', [pick, this.input]), toolbar, this.drop, this.uploads, this.alert, this.content);
    this.switch.addEventListener('view', (e) => {
      history.replaceState(null, '', pages.files({ view: e.detail.view === 'table' ? '' : e.detail.view }));
      this.draw();
    });
    let depth = 0;
    const over = (on) => this.toggleAttribute('data-dragging', on);
    addEventListener('dragenter', (e) => {
      if (!e.dataTransfer?.types.includes('Files')) return;
      depth++;
      over(true);
    });
    addEventListener('dragleave', () => {
      depth = Math.max(0, depth - 1);
      if (!depth) over(false);
    });
    addEventListener('dragover', (e) => e.preventDefault());
    addEventListener('drop', (e) => {
      e.preventDefault();
      depth = 0;
      over(false);
      if (e.dataTransfer?.files.length) this.send([...e.dataTransfer.files]);
    });
    await this.load();
  }

  async load() {
    const all = [];
    for (let after; ; ) {
      const page = await api(endpoints.files, { query: { space: this.space?.name, limit: 1000, after } });
      all.push(...page);
      if (page.length < 1000) break;
      after = page.at(-1).id;
    }
    this.files = all.reverse();
    this.draw();
    clearTimeout(this.timer);
    if (this.files.some((f) => f.status === 'pending' || f.status === 'processing')) this.timer = setTimeout(() => this.load(), 2000);
  }

  draw() {
    const view = new URLSearchParams(location.search).get('view') === 'gallery' ? 'gallery' : 'table';
    this.switch.show({ view, views });
    this.drop.hidden = this.files.length > 0;
    this.content.replaceChildren(view === 'gallery' ? this.gallery() : this.table());
  }

  table() {
    if (!this.files.length) return document.createDocumentFragment();
    const wrap = document.createElement('div');
    wrap.dataset.scroll = '';
    const table = document.createElement('table');
    table.innerHTML = '<thead><tr><th scope="col">Name</th><th scope="col">Status</th><th scope="col" data-extra>Kind</th><th scope="col" data-extra>Size</th><th scope="col" data-extra>Added</th></tr></thead><tbody></tbody>';
    table.querySelector('tbody').append(
      ...this.files.map((f) => {
        const tr = document.createElement('tr');
        const name = document.createElement('td');
        const a = el('a', f.name);
        a.href = pages.file({ id: f.id });
        name.append(a);
        const status = document.createElement('td');
        status.append(statusWord(f.status));
        tr.append(name, status, el('td', mimeLabel(f.mime), { 'data-extra': '' }), el('td', bytes(f.size), { 'data-extra': '' }), el('td', when(f.created_at), { 'data-extra': '' }));
        return tr;
      }),
    );
    wrap.append(table);
    return wrap;
  }

  gallery() {
    const images = this.files.filter((f) => f.mime.startsWith('image/'));
    if (!images.length) return el('p', 'No photos or images in this space yet.');
    const days = Map.groupBy(
      images.toSorted((a, b) => takenDay(b).localeCompare(takenDay(a))),
      takenDay,
    );
    const day = new Intl.DateTimeFormat(locale(), { dateStyle: 'full' });
    const out = document.createElement('div');
    out.dataset.gallery = '';
    for (const [iso, list] of days) {
      const section = document.createElement('section');
      const [y, m, d] = iso.split('-').map(Number);
      const grid = document.createElement('ul');
      grid.append(
        ...list.map((f) => {
          const li = document.createElement('li');
          const a = document.createElement('a');
          a.href = pages.file({ id: f.id });
          const img = document.createElement('img');
          img.alt = f.name;
          img.loading = 'lazy';
          img.src = f.metadata?.thumbnail ? endpoints.fileThumbnail(f.id) : endpoints.fileContent(f.id);
          a.append(img);
          li.append(a);
          return li;
        }),
      );
      section.append(el('h2', day.format(new Date(y, m - 1, d))), grid);
      out.append(section);
    }
    return out;
  }

  async send(files) {
    this.alert.show('');
    this.uploads.hidden = false;
    for (const file of files) {
      const li = document.createElement('li');
      const bar = document.createElement('progress');
      bar.max = 1;
      bar.value = 0;
      bar.setAttribute('aria-label', `Uploading ${file.name}`);
      const pct = el('span', '0%');
      li.append(el('span', file.name), bar, pct);
      this.uploads.append(li);
      try {
        await upload(file, {
          space: this.space?.name,
          intake: this.intake.value,
          onProgress: (p) => {
            bar.value = p;
            pct.textContent = new Intl.NumberFormat(locale(), { style: 'percent' }).format(p);
          },
        });
        li.remove();
      } catch (err) {
        pct.textContent = 'Failed';
        pct.dataset.tone = 'destructive';
        this.alert.show(`${file.name}: ${err.message}`);
      }
    }
    this.uploads.hidden = !this.uploads.children.length;
    await this.load();
  }
}

customElements.define('files-page', FilesPage);
