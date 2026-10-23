const recordViews = [
  ['table', 'Table'],
  ['board', 'Board'],
  ['calendar', 'Calendar'],
];

class ViewSwitch extends HTMLElement {
  show({ view, by, choices = {}, views = recordViews }) {
    this.innerHTML = '<div role="group" aria-label="View"></div>';
    const group = this.firstElementChild;
    for (const [value, text] of views) {
      const b = document.createElement('button');
      b.type = 'button';
      b.innerHTML = `<app-icon name="${value}"></app-icon><span>${text}</span>`;
      b.setAttribute('aria-pressed', String(value === view));
      b.disabled = Boolean(choices[value] && !choices[value].length);
      b.addEventListener('click', () => this.emit(value, choices[value]?.[0]?.value));
      group.append(b);
    }
    const options = choices[view] ?? [];
    if (options.length > 1) {
      const select = document.createElement('app-select');
      select.setAttribute('aria-label', view === 'board' ? 'Group by' : 'Place by');
      this.append(select);
      select.options = options;
      select.value = by;
      select.addEventListener('change', () => this.emit(view, select.value));
    }
  }

  emit(view, by) {
    this.dispatchEvent(new CustomEvent('view', { detail: { view, by }, bubbles: true }));
  }
}

customElements.define('view-switch', ViewSwitch);
