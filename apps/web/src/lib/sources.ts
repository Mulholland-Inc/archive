export const kinds = [
  { id: 'gmail', label: 'Gmail', icon: 'mail', description: 'Mail and attachments, read-only, through Google sign-in' },
  { id: 'drive', label: 'Google Drive', icon: 'cloud', description: 'Documents and files, read-only, through Google sign-in' },
  { id: 'imap', label: 'Email over IMAP', icon: 'inbox', description: 'Fastmail, iCloud, Outlook or any IMAP server, with an app password' },
  { id: 'folder', label: 'Folder', icon: 'folder', description: 'A folder on the machine Archive runs on' },
  { id: 'external', label: 'External connector', icon: 'terminal', description: 'Any program that speaks the JSON-lines or Singer protocol' }
];

export function kindLabel(kind: string) {
  return kinds.find((k) => k.id === kind)?.label ?? kind;
}

export function kindIcon(kind: string) {
  return kinds.find((k) => k.id === kind)?.icon ?? 'plug';
}

export const schedules = [
  { id: '', label: 'Only when I run it' },
  { id: '15m', label: 'Every 15 minutes' },
  { id: '1h', label: 'Every hour' },
  { id: '6h', label: 'Every 6 hours' },
  { id: '24h', label: 'Every day' }
];

export function scheduleLabel(s?: string) {
  if (!s) return 'Manual';
  return schedules.find((x) => x.id === s)?.label ?? `Every ${s}`;
}

export function statusLabel(s: string) {
  return (
    { new: 'Not run yet', needs_auth: 'Needs sign-in', running: 'Syncing', ok: 'Up to date', error: 'Failing' } as { [k: string]: string }
  )[s] ?? s;
}

export function statusTone(s: string): 'success' | 'warning' | 'danger' | 'muted' | undefined {
  if (s === 'ok') return 'success';
  if (s === 'error') return 'danger';
  if (s === 'needs_auth') return 'warning';
  if (s === 'new') return 'muted';
  return undefined;
}
