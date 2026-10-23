import { api, loadMe, spaceLabel } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { label } from './ontology.js';
import { mimeLabel, statusWord } from './files-page.js';
import { alertBox, button, bytes, copy, el, header, link, when } from './ui.js';

const row = (term, value) => {
  const div = document.createElement('div');
  const dd = document.createElement('dd');
  dd.dataset.value = '';
  dd.append(value instanceof Node ? value : String(value));
  div.append(el('dt', term), dd);
  return div;
};

class FileView extends HTMLElement {
  async connectedCallback() {
    const params = new URLSearchParams(location.search);
    let id = params.get('id');
    if (!id && params.get('sha256')) {
      const [rec] = await api(endpoints.records('file'), { query: { sha256: params.get('sha256'), limit: 1 } });
      id = rec?.id;
    }
    try {
      if (!id) throw new Error('There is no file with that content.');
      this.file = await api(endpoints.file(id));
    } catch (err) {
      this.replaceChildren(header('File not found'), el('p', err.message));
      return;
    }
    this.render();
  }

  async render() {
    const f = this.file;
    document.title = `${f.name}, Archive`;
    const content = endpoints.fileContent(f.id);
    const download = link('Download', `${content}${f.metadata?.personal ? '?strip=gps' : ''}`, 'secondary');
    download.download = f.name;
    const share = button('Share link', 'link');
    const alert = alertBox();
    const shared = document.createElement('div');
    shared.dataset.share = '';
    shared.hidden = true;
    share.addEventListener('click', async () => {
      try {
        const l = await api(endpoints.fileLink(f.id), { method: 'POST' });
        const url = el('p', l.url);
        url.dataset.secret = '';
        const c = button('Copy link', 'link');
        c.addEventListener('click', () => copy(c, l.url));
        shared.replaceChildren(el('p', `Anyone with this link can open the file until ${when(l.expires_at)}${f.metadata?.personal ? ', without its location' : ''}.`), url, c);
        shared.hidden = false;
      } catch (err) {
        alert.show(err);
      }
    });
    const back = el('p');
    back.append(link('All files', pages.files()));
    this.replaceChildren(header(f.name, [share, download], [el('p', mimeLabel(f.mime))]), shared, alert, await this.preview(content), this.details(), back);
    if (f.status === 'pending' || f.status === 'processing') {
      setTimeout(async () => {
        this.file = await api(endpoints.file(f.id));
        this.render();
      }, 2000);
    }
  }

  async preview(content) {
    const f = this.file;
    const top = f.mime.split('/')[0];
    const frame = document.createElement('figure');
    frame.dataset.preview = top;
    if (top === 'image') {
      const img = document.createElement('img');
      img.src = content;
      img.alt = f.name;
      frame.append(img);
    } else if (top === 'video' || top === 'audio') {
      const media = document.createElement(top);
      media.controls = true;
      media.preload = 'metadata';
      media.src = content;
      frame.append(media);
    } else if (f.mime === 'application/pdf') {
      const doc = document.createElement('iframe');
      doc.src = content;
      doc.title = f.name;
      frame.append(doc);
    }
    const text = await fetch(endpoints.fileText(f.id)).then((r) => (r.ok ? r.text() : '')).catch(() => '');
    if (text.trim()) {
      const body = el('div', text.trim());
      body.dataset.text = '';
      frame.append(body);
    }
    if (!frame.children.length) frame.append(el('p', f.status === 'done' ? 'Archive cannot show this kind of file. Download it to open it.' : 'Archive is still reading this file.'));
    return frame;
  }

  details() {
    const f = this.file;
    const m = f.metadata ?? {};
    const dl = document.createElement('dl');
    dl.dataset.details = '';
    const status = statusWord(f.status);
    if (f.error) status.textContent += `: ${f.error}`;
    const rows = [
      row('Status', status),
      row('Kind', mimeLabel(f.mime)),
      row('Size', bytes(f.size)),
      row('Added', when(f.created_at)),
      m.pages && row('Pages', m.pages),
      m.width && row('Dimensions', `${m.width} by ${m.height} pixels`),
      m.taken_at && row('Taken', when(m.taken_at)),
      m.camera && row('Camera', m.camera),
      m.personal?.location && row('Location', `${m.personal.location.lat}, ${m.personal.location.lon}`),
      f.intake && row('Read into', label(f.intake)),
    ].filter(Boolean);
    dl.append(...rows);
    loadMe().then(({ spaces, home }) => {
      const sp = spaces.find((s) => s.id === f.space);
      if (sp) dl.append(row('Space', spaceLabel(sp, home)));
    });
    return dl;
  }
}

customElements.define('file-view', FileView);
