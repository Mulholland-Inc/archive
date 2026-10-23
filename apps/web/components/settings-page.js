import { api } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { setLocale } from './format.js';

const locales = [
  ['', 'Same as this browser'],
  ['en-US', 'English (United States)'],
  ['en-GB', 'English (United Kingdom)'],
  ['nl-NL', 'Nederlands'],
  ['de-DE', 'Deutsch'],
  ['fr-FR', 'Français'],
];

class SettingsPage extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `
      <header><div><h1>Settings</h1></div></header>
      <dl>
        <div data-account hidden><dt>Signed in as</dt><dd></dd></div>
        <div><dt>Dates and amounts</dt><dd><app-select aria-label="Dates and amounts"></app-select></dd></div>
      </dl>
      <footer><button type="button" class="button button-secondary button-sm">Sign out</button></footer>`;
    const select = this.querySelector('app-select');
    select.options = locales.map(([value, text]) => ({ value, label: text }));
    select.value = localStorage.getItem('archive.locale') ?? '';
    select.addEventListener('change', () => setLocale(select.value));
    this.querySelector('footer button').addEventListener('click', async () => {
      await api(endpoints.signOut, { method: 'POST', redirect: false }).catch(() => {});
      location.assign(pages.signIn);
    });
    api(endpoints.me, { redirect: false })
      .then((me) => {
        if (!me?.email) return;
        const row = this.querySelector('[data-account]');
        row.querySelector('dd').textContent = me.name ? `${me.name}, ${me.email}` : me.email;
        row.hidden = false;
      })
      .catch(() => {});
  }
}

customElements.define('settings-page', SettingsPage);
