export const prompter = $state({ open: false, title: '', label: '', value: '', placeholder: '', confirm: 'Save', resolve: null });
export const confirmer = $state({ open: false, title: '', body: '', confirm: 'Delete', resolve: null });

export function prompt({ title, label = 'Name', value = '', placeholder = '', confirm = 'Save' }) {
  return new Promise((resolve) => {
    Object.assign(prompter, { open: true, title, label, value, placeholder, confirm, resolve });
  });
}

export function confirmDialog({ title, body = '', confirm = 'Delete' }) {
  return new Promise((resolve) => {
    Object.assign(confirmer, { open: true, title, body, confirm, resolve });
  });
}
