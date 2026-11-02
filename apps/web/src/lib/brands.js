import { BRANDS } from '#kit/brands.js';

const names = {
  gdrive: 'Google Drive',
  gmail: 'Gmail',
  gcal: 'Google Calendar',
  dropbox: 'Dropbox',
  box: 'Box',
  icloud: 'iCloud',
  notion: 'Notion',
  github: 'GitHub',
  outlook: 'Outlook',
  onedrive: 'OneDrive',
  slack: 'Slack',
  google: 'Google',
  okta: 'Okta',
  microsoft: 'Microsoft',
  entra: 'Microsoft Entra'
};

export const brandFor = (id) => BRANDS[names[id]];
