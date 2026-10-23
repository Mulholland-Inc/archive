import { api, currentSpace, loadMe, setSpace } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { label, loadSpec, userTypes } from './ontology.js';

const link = (href, icon, text, current) => {
  const li = document.createElement('li');
  const a = document.createElement('a');
  a.href = href;
  a.innerHTML = `<app-icon name="${icon}"></app-icon><span></span>`;
  a.querySelector('span').textContent = text;
  if (current) a.setAttribute('aria-current', 'page');
  li.append(a);
  return li;
};

export const viewParams = (v) => ({
  view: v.layout === 'table' ? '' : v.layout,
  by: v.by,
  saved: v.id,
  ...Object.fromEntries(Object.entries(v.where ?? {}).map(([k, val]) => [`where.${k}`, val])),
});

class AppShell extends HTMLElement {
  connectedCallback() {
    if (this.frame) return;
    const content = [...this.childNodes];
    const params = new URLSearchParams(location.search);
    const here = location.pathname.split('/').pop() || 'index.html';
    this.type = ['records.html', 'record.html', 'index.html'].includes(here) ? params.get('type') : null;
    this.saved = here === 'records.html' ? params.get('saved') : null;

    this.innerHTML = `
      <a class="skip" href="#main">Skip to content</a>
      <aside id="shell-sidebar" aria-label="Sidebar">
        <a href="${pages.home}" aria-label="Archive home"><archive-wordmark></archive-wordmark></a>
        <app-select data-space aria-label="Space"></app-select>
        <nav aria-label="Types"><ul data-types></ul></nav>
        <nav aria-label="Archive"><ul data-app></ul></nav>
      </aside>
      <div data-scrim></div>
      <div data-frame>
        <header>
          <button type="button" data-menu aria-expanded="false" aria-controls="shell-sidebar" aria-label="Menu"><app-icon name="menu"></app-icon></button>
          <form role="search" action="${pages.search()}">
            <app-icon name="search"></app-icon>
            <input type="text" name="q" role="searchbox" aria-label="Search everything" placeholder="Search everything" autocomplete="off">
          </form>
        </header>
        <main id="main" tabindex="-1"></main>
      </div>`;
    this.frame = this.querySelector('[data-frame]');
    this.querySelector('main').append(...content);
    this.querySelector('[data-app]').append(
      link(pages.files(), 'file', 'Files', ['files.html', 'file.html'].includes(here)),
      link(pages.sources, 'source', 'Sources', here === 'sources.html'),
      link(pages.changes, 'changes', 'Changes', ['changes.html', 'schema.html'].includes(here)),
      link(pages.settings, 'settings', 'Settings', here === 'settings.html'),
    );

    const menu = this.querySelector('[data-menu]');
    const toggle = (open) => {
      this.toggleAttribute('data-open', open);
      menu.setAttribute('aria-expanded', String(open));
    };
    menu.addEventListener('click', () => toggle(!this.hasAttribute('data-open')));
    this.querySelector('[data-scrim]').addEventListener('click', () => toggle(false));
    this.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && this.hasAttribute('data-open')) {
        toggle(false);
        menu.focus();
      }
    });

    this.spaces();
    this.types();
    addEventListener('app-views', () => this.types());
    this.search(here === 'search.html');
  }

  async spaces() {
    const picker = this.querySelector('[data-space]');
    const [{ spaces }, current] = await Promise.all([loadMe(), currentSpace()]);
    picker.options = spaces.map((s) => ({ value: s.name, label: label(s.name) }));
    picker.value = current?.name ?? '';
    picker.addEventListener('change', () => {
      setSpace(picker.value);
      location.reload();
    });
  }

  async types() {
    const [spec, views, space] = await Promise.all([loadSpec(), api(endpoints.views).catch(() => []), currentSpace()]);
    this.spec = spec;
    const list = this.querySelector('[data-types]');
    list.replaceChildren(
      ...userTypes(spec).map((t) => {
        const li = link(pages.records(t), 'record', label(t), t === this.type && !this.saved);
        const mine = views.filter((v) => v.type === t && v.space === space?.id);
        if (mine.length) {
          const ul = document.createElement('ul');
          ul.append(...mine.map((v) => link(pages.records(t, viewParams(v)), 'view', v.name, v.id === this.saved)));
          li.append(ul);
        }
        return li;
      }),
      link(pages.schema('new'), 'plus', 'New type', false),
    );
  }

  search(live) {
    const form = this.querySelector('[role=search]');
    const input = form.querySelector('input');
    input.value = new URLSearchParams(location.search).get('q') ?? '';
    if (!live) return;
    form.addEventListener('submit', (e) => e.preventDefault());
    let timer;
    input.addEventListener('input', () => {
      clearTimeout(timer);
      timer = setTimeout(() => dispatchEvent(new CustomEvent('app-search', { detail: input.value.trim() })), 250);
    });
  }
}

customElements.define('app-shell', AppShell);
