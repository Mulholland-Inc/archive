class WaitlistForm extends HTMLElement {
  connectedCallback() {
    const form = this.querySelector('form');
    const thanks = this.querySelector('.thanks');
    if (!form || !thanks) return;
    form.addEventListener('submit', (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      form.hidden = true;
      thanks.hidden = false;
    });
  }
}

customElements.define('waitlist-form', WaitlistForm);
