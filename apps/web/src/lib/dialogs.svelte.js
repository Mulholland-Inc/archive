export const prompter = $state({ open: false, title: '', label: '', value: '', placeholder: '', confirm: 'Save', resolve: null });
export const confirmer = $state({ open: false, title: '', body: '', confirm: 'Delete', info: false, resolve: null });

export function prompt({ title, label = 'Name', value = '', placeholder = '', confirm = 'Save' }) {
  return new Promise((resolve) => {
    Object.assign(prompter, { open: true, title, label, value, placeholder, confirm, resolve });
  });
}

export function confirmDialog({ title, body = '', confirm = 'Delete' }) {
  return new Promise((resolve) => {
    Object.assign(confirmer, { open: true, title, body, confirm, info: false, resolve });
  });
}

// A message with one button, for errors and for what the app can't do yet.
export function notify({ title, body = '' }) {
  return new Promise((resolve) => {
    Object.assign(confirmer, { open: true, title, body, confirm: 'OK', info: true, resolve });
  });
}
