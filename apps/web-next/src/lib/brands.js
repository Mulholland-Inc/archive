import { siGoogledrive, siGmail, siGooglecalendar, siDropbox, siBox, siIcloud, siNotion, siGithub } from 'simple-icons';

export const brands = {
  gdrive: { path: siGoogledrive.path, hex: siGoogledrive.hex },
  gmail: { path: siGmail.path, hex: siGmail.hex },
  gcal: { path: siGooglecalendar.path, hex: siGooglecalendar.hex },
  dropbox: { path: siDropbox.path, hex: siDropbox.hex },
  box: { path: siBox.path, hex: siBox.hex },
  icloud: { path: siIcloud.path, hex: siIcloud.hex },
  notion: { path: siNotion.path, hex: siNotion.hex },
  github: { path: siGithub.path, hex: siGithub.hex },
  outlook: { glyph: 'mail', hex: '0078D4' },
  onedrive: { glyph: 'cloud', hex: '0078D4' },
  slack: { glyph: 'hash', hex: '4A154B' }
};

export function isDark(hex) {
  const n = parseInt(hex, 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return 0.2126 * r + 0.7152 * g + 0.0722 * b < 60;
}
