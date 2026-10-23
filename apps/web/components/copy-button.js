class CopyButton extends HTMLElement {
  connectedCallback() {
    const button = this.querySelector('button');
    const source = document.getElementById(this.getAttribute('for'));
    if (!button || !source || !navigator.clipboard) return;
    const label = button.textContent;
    this.hidden = false;
    button.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(source.textContent.trim());
        button.textContent = 'Copied';
      } catch {
        button.textContent = 'Press Ctrl+C';
      }
      clearTimeout(this.timer);
      this.timer = setTimeout(() => (button.textContent = label), 1800);
    });
  }
}

customElements.define('copy-button', CopyButton);
