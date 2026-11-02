// MOCK: made-up data; see index.js for what is still mocked.
export const catalog = [
  { id: 'gdrive', by: 'Google', description: 'Find and import documents, scans and photos from your Drive', name: 'Google Drive', category: 'Files', icon: 'drive', scopes: ['Read files you pick', 'Watch folders for new files'] },
  { id: 'gmail', by: 'Google', description: 'Pull letters and attachments from labelled mail', name: 'Gmail', category: 'Email', icon: 'mail', scopes: ['Read messages with a label you choose', 'Save attachments to the library'] },
  { id: 'outlook', by: 'Microsoft', description: 'Save attachments and notices from chosen folders', name: 'Outlook', category: 'Email', icon: 'inbox', scopes: ['Read messages in folders you choose', 'Save attachments to the library'] },
  { id: 'gcal', by: 'Google', description: 'Read events and add reminders for deadlines', name: 'Google Calendar', category: 'Calendar', icon: 'calendar', scopes: ['Read events', 'Create reminders'] },
  { id: 'dropbox', by: 'Dropbox', description: 'Import files and watch shared folders', name: 'Dropbox', category: 'Files', icon: 'package', scopes: ['Read files you pick', 'Watch folders for new files'] },
  { id: 'onedrive', by: 'Microsoft', description: 'Import files and watch folders from OneDrive', name: 'OneDrive', category: 'Files', icon: 'cloud', scopes: ['Read files you pick', 'Watch folders for new files'] },
  { id: 'box', by: 'Box', description: 'Bring in records and contracts stored in Box', name: 'Box', category: 'Files', icon: 'archive', scopes: ['Read files you pick'] },
  { id: 'icloud', by: 'Apple', description: 'Import albums and new photos as they arrive', name: 'iCloud Photos', category: 'Photos', icon: 'image', scopes: ['Read albums you share', 'Import new photos'] },
  { id: 'slack', by: 'Slack', description: 'Read channels and post workflow results', name: 'Slack', category: 'Messaging', icon: 'chat', scopes: ['Read channels you add Archive to', 'Post workflow results'] },
  { id: 'notion', by: 'Notion', description: 'Read shared pages and create new ones', name: 'Notion', category: 'Docs', icon: 'notebook', scopes: ['Read pages you share', 'Create pages'] },
  { id: 'github', by: 'GitHub', description: 'Read repositories and open issues', name: 'GitHub', category: 'Code', icon: 'branch', scopes: ['Read repositories you pick', 'Open issues'] }
];

const connected = {
  okafor: { gdrive: 'nora.okafor@gmail.com', gmail: 'nora.okafor@gmail.com', icloud: 'nora@icloud.com' },
  hale: { outlook: 'nora@halewhitcombe.law', onedrive: 'nora@halewhitcombe.law', box: 'records@halewhitcombe.law' },
  fieldnote: { gdrive: 'nora@fieldnote.studio', slack: 'fieldnote.slack.com', notion: 'Fieldnote', dropbox: 'studio@fieldnote.studio' },
  personal: { github: 'nokafor' }
};

const syncs = ['2 min ago', '14 min ago', '1 hr ago', 'Yesterday'];

export function connectorsFor(vault) {
  return catalog.map((c, i) => {
    const account = connected[vault]?.[c.id];
    return { ...c, connected: Boolean(account), account: account ?? '', lastSync: account ? syncs[i % syncs.length] : '', items: account ? 40 + ((i * 137) % 900) : 0 };
  });
}
