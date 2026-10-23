class AppDialog extends HTMLElement {
  open({ title, body, actions, onAction }) {
    this.innerHTML = '<dialog><form method="dialog"><h2></h2><div data-body></div><p data-error role="alert" hidden></p><footer></footer></form></dialog>';
    const dialog = this.querySelector('dialog');
    const error = this.querySelector('[data-error]');
    this.querySelector('h2').textContent = title;
    const slot = this.querySelector('[data-body]');
    if (typeof body === 'string') {
      const p = document.createElement('p');
      p.textContent = body;
      slot.append(p);
    } else if (body) slot.append(body);
    this.querySelector('footer').append(
      ...actions.map((a) => {
        const b = document.createElement('button');
        b.className = `button button-sm ${a.kind === 'primary' || a.kind === 'danger' ? 'button-primary' : 'button-secondary'}`;
        if (a.kind === 'danger') b.dataset.danger = '';
        b.value = a.value ?? '';
        b.textContent = a.label;
        if (!b.value) {
          b.type = 'button';
          b.addEventListener('click', () => dialog.close(''));
        }
        return b;
      }),
    );
    document.body.append(this);
    return new Promise((resolve) => {
      this.querySelector('form').addEventListener('submit', async (e) => {
        const value = e.submitter?.value ?? '';
        if (!value || !onAction) return;
        e.preventDefault();
        error.hidden = true;
        const buttons = this.querySelectorAll('footer button');
        buttons.forEach((b) => (b.disabled = true));
        try {
          if ((await onAction(value)) !== false) dialog.close(value);
        } catch (err) {
          error.textContent = err.message;
          error.hidden = false;
        }
        buttons.forEach((b) => (b.disabled = false));
      });
      dialog.addEventListener('close', () => {
        resolve(dialog.returnValue || null);
        this.remove();
      });
      dialog.showModal();
    });
  }
}

customElements.define('app-dialog', AppDialog);

export const openDialog = (options) => document.createElement('app-dialog').open(options);

export const confirmDialog = ({ title, text, action }) =>
  openDialog({
    title,
    body: text,
    actions: [
      { label: 'Cancel', value: '' },
      { label: action, value: 'ok', kind: 'danger' },
    ],
  }).then((v) => v === 'ok');
