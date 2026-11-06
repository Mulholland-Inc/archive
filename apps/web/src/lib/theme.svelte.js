const KEY = 'archive.theme';

export const theme = $state({
  choice: (typeof localStorage !== 'undefined' && localStorage.getItem(KEY)) || 'system',
  resolved: 'light'
});

const media = typeof matchMedia !== 'undefined' ? matchMedia('(prefers-color-scheme: dark)') : null;

function apply() {
  const dark = theme.choice === 'dark' || (theme.choice === 'system' && media?.matches);
  theme.resolved = dark ? 'dark' : 'light';
  document.documentElement.dataset.theme = theme.resolved;
}

export function setTheme(choice) {
  theme.choice = choice;
  localStorage.setItem(KEY, choice);
  apply();
}

export function toggleTheme() {
  setTheme(theme.resolved === 'dark' ? 'light' : 'dark');
}

if (typeof document !== 'undefined') {
  apply();
  media?.addEventListener('change', apply);
}
