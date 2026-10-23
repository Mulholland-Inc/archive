import { pages } from './endpoints.js';
import { isoDate, locale, toDate } from './format.js';
import { loadAll, recordTitle } from './ontology.js';

const firstDay = () => {
  const l = new Intl.Locale(locale());
  return (l.getWeekInfo?.() ?? l.weekInfo)?.firstDay ?? 1;
};

const dayOf = (v) => (typeof v === 'string' ? v.slice(0, 10) : v?.utc?.slice(0, 10));

class RecordCalendar extends HTMLElement {
  async load({ spec, type, where, by, month, onMonth }) {
    const start = month && /^\d{4}-\d{2}$/.test(month) ? toDate(`${month}-01`) : new Date(new Date().getFullYear(), new Date().getMonth(), 1);
    const shift = (n) => onMonth(isoDate(new Date(start.getFullYear(), start.getMonth() + n, 1)).slice(0, 7));
    const records = await loadAll(type, where);
    const byDay = Map.groupBy(records, (r) => dayOf(r.data[by]));

    this.innerHTML = `
      <header>
        <h2></h2>
        <div>
          <button type="button" class="button button-secondary button-sm" data-today>Today</button>
          <button type="button" class="button button-secondary button-sm" data-prev aria-label="Previous month"><app-icon name="left"></app-icon></button>
          <button type="button" class="button button-secondary button-sm" data-next aria-label="Next month"><app-icon name="right"></app-icon></button>
        </div>
      </header>
      <ol></ol>
      <p data-empty hidden>Nothing this month.</p>`;
    this.querySelector('h2').textContent = new Intl.DateTimeFormat(locale(), { month: 'long', year: 'numeric' }).format(start);
    this.querySelector('[data-prev]').addEventListener('click', () => shift(-1));
    this.querySelector('[data-next]').addEventListener('click', () => shift(1));
    this.querySelector('[data-today]').addEventListener('click', () => onMonth(''));

    const weekday = new Intl.DateTimeFormat(locale(), { weekday: 'short' });
    const dayName = new Intl.DateTimeFormat(locale(), { weekday: 'long', day: 'numeric', month: 'long' });
    const offset = (start.getDay() - (firstDay() % 7) + 7) % 7;
    const grid = this.querySelector('ol');
    for (let i = 0; i < 7; i++) {
      const li = document.createElement('li');
      li.dataset.weekday = '';
      li.textContent = weekday.format(new Date(2024, 0, 7 + ((firstDay() + i) % 7)));
      grid.append(li);
    }
    const today = isoDate(new Date());
    const days = new Date(start.getFullYear(), start.getMonth() + 1, 0).getDate();
    const cells = Math.ceil((offset + days) / 7) * 7;
    for (let i = 0; i < cells; i++) {
      const date = new Date(start.getFullYear(), start.getMonth(), i - offset + 1);
      const iso = isoDate(date);
      const li = document.createElement('li');
      if (date.getMonth() !== start.getMonth()) li.dataset.outside = '';
      if (iso === today) li.dataset.today = '';
      const n = document.createElement('time');
      n.dateTime = iso;
      n.innerHTML = '<span></span><span data-long></span>';
      n.firstChild.textContent = date.getDate();
      n.lastChild.textContent = dayName.format(date);
      li.append(n);
      for (const r of byDay.get(iso) ?? []) {
        const a = document.createElement('a');
        a.href = pages.record(r.type, r.id);
        a.textContent = recordTitle(spec, r);
        li.append(a);
      }
      if (!li.querySelector('a')) li.dataset.free = '';
      grid.append(li);
    }
    this.querySelector('[data-empty]').hidden = Boolean(grid.querySelector('a'));
  }
}

customElements.define('record-calendar', RecordCalendar);
