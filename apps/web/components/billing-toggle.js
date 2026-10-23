class BillingToggle extends HTMLElement {
  connectedCallback() {
    const buttons = [...this.querySelectorAll('button')];
    buttons.forEach((button) =>
      button.addEventListener('click', () => {
        buttons.forEach((other) => other.setAttribute('aria-pressed', String(other === button)));
        const yearly = button.value === 'yearly';
        document.querySelectorAll('[data-period]').forEach((label) => {
          label.textContent = yearly ? '/yr' : '/mo';
        });
        document.querySelectorAll('[data-billed]').forEach((note) => {
          note.textContent = yearly ? 'Billed yearly, two months free' : 'Billed monthly';
        });
      }),
    );
  }
}

customElements.define('billing-toggle', BillingToggle);
