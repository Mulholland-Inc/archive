let count = 0;

class AppSelect extends HTMLElement {
  #options = [];
  #value = '';

  static observedAttributes = ['disabled'];

  attributeChangedCallback() {
    this.render();
  }

  connectedCallback() {
    if (this.button) return;
    this.button = document.createElement('button');
    this.button.type = 'button';
    this.button.setAttribute('aria-haspopup', 'listbox');
    this.button.setAttribute('aria-expanded', 'false');
    if (this.hasAttribute('aria-label')) this.button.setAttribute('aria-label', this.getAttribute('aria-label'));
    this.button.innerHTML = '<span></span><app-icon name="down"></app-icon>';
    this.list = document.createElement('ul');
    this.list.id = `app-select-${++count}`;
    this.list.setAttribute('role', 'listbox');
    this.list.popover = 'auto';
    this.button.setAttribute('aria-controls', this.list.id);
    this.append(this.button, this.list);
    this.button.addEventListener('click', () => this.open());
    this.button.addEventListener('keydown', (e) => {
      if (['ArrowDown', 'ArrowUp'].includes(e.key)) {
        e.preventDefault();
        this.open();
      }
    });
    this.list.addEventListener('toggle', (e) => {
      this.button.setAttribute('aria-expanded', String(e.newState === 'open'));
      if (e.newState === 'closed') removeEventListener('scroll', this.close, true);
    });
    this.list.addEventListener('click', (e) => {
      const li = e.target.closest('[role=option]');
      if (li) this.pick(li.dataset.value);
    });
    this.list.addEventListener('keydown', (e) => this.key(e));
    this.close = (e) => {
      if (!this.list.contains(e?.target)) this.list.hidePopover();
    };
    this.render();
  }

  get options() {
    return this.#options;
  }

  set options(list) {
    this.#options = list;
    this.render();
  }

  get value() {
    return this.#value;
  }

  set value(v) {
    this.#value = v ?? '';
    this.render();
  }

  get disabled() {
    return this.hasAttribute('disabled');
  }

  render() {
    if (!this.button) return;
    const current = this.#options.find((o) => o.value === this.#value);
    this.button.querySelector('span').textContent = current?.label ?? this.getAttribute('placeholder') ?? '';
    this.button.toggleAttribute('data-empty', !current || current.value === '');
    this.button.disabled = this.disabled;
    this.list.replaceChildren(
      ...this.#options.map((o) => {
        const li = document.createElement('li');
        li.setAttribute('role', 'option');
        li.tabIndex = -1;
        li.dataset.value = o.value;
        li.setAttribute('aria-selected', String(o.value === this.#value));
        li.innerHTML = '<span></span><app-icon name="check"></app-icon>';
        li.querySelector('span').textContent = o.label;
        return li;
      }),
    );
  }

  open() {
    if (this.disabled || !this.#options.length) return;
    const r = this.button.getBoundingClientRect();
    Object.assign(this.list.style, { left: `${r.left}px`, minWidth: `${r.width}px`, top: `${r.bottom}px`, bottom: 'auto' });
    this.list.showPopover();
    const space = innerHeight - r.bottom;
    if (space < this.list.offsetHeight && r.top > space) Object.assign(this.list.style, { top: 'auto', bottom: `${innerHeight - r.top}px` });
    addEventListener('scroll', this.close, true);
    (this.list.querySelector('[aria-selected=true]') ?? this.list.firstElementChild)?.focus();
  }

  key(e) {
    const items = [...this.list.children];
    const i = items.indexOf(document.activeElement);
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      items[Math.max(0, Math.min(items.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1)))]?.focus();
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (items[i]) this.pick(items[i].dataset.value);
    } else if (e.key === 'Tab') {
      this.close();
    }
  }

  pick(value) {
    this.close();
    this.button.focus();
    if (value === this.#value) return;
    this.value = value;
    this.dispatchEvent(new Event('change', { bubbles: true }));
  }
}

customElements.define('app-select', AppSelect);
