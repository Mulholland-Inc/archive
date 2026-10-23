class AppSwitch extends HTMLElement {
  #checked = false;

  connectedCallback() {
    if (this.button) return;
    this.button = document.createElement('button');
    this.button.type = 'button';
    this.button.setAttribute('role', 'switch');
    this.button.innerHTML = '<i></i><span></span>';
    this.button.querySelector('span').textContent = this.getAttribute('label') ?? '';
    this.append(this.button);
    if (this.hasAttribute('checked')) this.#checked = true;
    this.render();
    this.button.addEventListener('click', () => {
      this.checked = !this.#checked;
      this.dispatchEvent(new Event('change', { bubbles: true }));
    });
  }

  get checked() {
    return this.#checked;
  }

  set checked(v) {
    this.#checked = Boolean(v);
    this.render();
  }

  render() {
    this.button?.setAttribute('aria-checked', String(this.#checked));
  }
}

customElements.define('app-switch', AppSwitch);
