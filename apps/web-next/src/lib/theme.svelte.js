const KEY = 'archive.theme';
const FONT = 'archive.chatFont';

export const theme = $state({
  choice: (typeof localStorage !== 'undefined' && localStorage.getItem(KEY)) || 'system',
  resolved: 'light',
  chatFont: (typeof localStorage !== 'undefined' && localStorage.getItem(FONT)) || 'sans'
});

const media = typeof matchMedia !== 'undefined' ? matchMedia('(prefers-color-scheme: dark)') : null;

function apply() {
  const dark = theme.choice === 'dark' || (theme.choice === 'system' && media?.matches);
  theme.resolved = dark ? 'dark' : 'light';
  document.documentElement.dataset.theme = theme.resolved;
  document.documentElement.dataset.chatFont = theme.chatFont;
}

export function setTheme(choice) {
  theme.choice = choice;
  localStorage.setItem(KEY, choice);
  apply();
}

export function setChatFont(font) {
  theme.chatFont = font;
  localStorage.setItem(FONT, font);
  apply();
}

export function toggleTheme() {
  setTheme(theme.resolved === 'dark' ? 'light' : 'dark');
}

if (typeof document !== 'undefined') {
  apply();
  media?.addEventListener('change', apply);
}
