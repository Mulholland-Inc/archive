// MOCK: made-up data; see index.js for what is still mocked.
export const catalog = [
  { id: 'gdrive', by: 'Google', description: 'Search and read your files, never change them', name: 'Google Drive', category: 'Files', icon: 'drive', scopes: ['Read your files and documents'] },
  { id: 'gmail', by: 'Google', description: 'Read your mail and attachments, never send or change it', name: 'Gmail', category: 'Email', icon: 'mail', scopes: ['Read your mail and its attachments'] },
  { id: 'outlook', by: 'Microsoft', description: 'Read your Microsoft 365 or Outlook.com mail, never send or change it', name: 'Outlook', category: 'Email', icon: 'inbox', scopes: ['Read your mail and its attachments'] },
  { id: 'gcal', by: 'Google', description: 'Read events and add reminders for deadlines', name: 'Google Calendar', category: 'Calendar', icon: 'calendar', scopes: ['Read events', 'Create reminders'] },
  { id: 'dropbox', by: 'Dropbox', description: 'Import files and watch shared folders', name: 'Dropbox', category: 'Files', icon: 'package', scopes: ['Read files you pick', 'Watch folders for new files'] },
  { id: 'onedrive', by: 'Microsoft', description: 'Import files and watch folders from OneDrive', name: 'OneDrive', category: 'Files', icon: 'cloud', scopes: ['Read files you pick', 'Watch folders for new files'] },
  { id: 'box', by: 'Box', description: 'Bring in records and contracts stored in Box', name: 'Box', category: 'Files', icon: 'archive', scopes: ['Read files you pick'] },
  { id: 'icloud', by: 'Apple', description: 'Import albums and new photos as they arrive', name: 'iCloud Photos', category: 'Photos', icon: 'image', scopes: ['Read albums you share', 'Import new photos'] },
  { id: 'slack', by: 'Slack', description: 'Read channels and post workflow results', name: 'Slack', category: 'Messaging', icon: 'chat', scopes: ['Read channels you add Archive to', 'Post workflow results'] },
  { id: 'notion', by: 'Notion', description: 'Search, read and write the pages and databases you allow', name: 'Notion', category: 'Docs', icon: 'notebook', scopes: ['Read and write pages you allow'] },
  { id: 'github', by: 'GitHub', description: 'Read issues, pull requests and files of your repositories', name: 'GitHub', category: 'Code', icon: 'branch', scopes: ['Read repositories you install Archive on'] },
  { id: 'quickbooks', by: 'Intuit', description: 'Read your company’s invoices, customers and books', name: 'QuickBooks', category: 'Finance', icon: 'ledger', scopes: ['Read one company’s accounting'] },
  { id: 'stripe', by: 'Stripe', description: 'Look up customers, payments, invoices and subscriptions', name: 'Stripe', category: 'Finance', icon: 'card', scopes: ['Use your Stripe account as you allow'] },
  { id: 'hubspot', by: 'HubSpot', description: 'Read contacts, companies and deals', name: 'HubSpot', category: 'CRM', icon: 'users', scopes: ['Read contacts, companies and deals'] },
  { id: 'asana', by: 'Asana', description: 'Read workspaces, projects and tasks', name: 'Asana', category: 'Projects', icon: 'list', scopes: ['Read projects, tasks and comments'] },
  { id: 'linear', by: 'Linear', description: 'Search and work with issues, projects and cycles', name: 'Linear', category: 'Projects', icon: 'list', scopes: ['Read and write issues as you'] },
  { id: 'sentry', by: 'Sentry', description: 'Look into issues, events and traces', name: 'Sentry', category: 'Code', icon: 'bug', scopes: ['Read your organization’s issues'] }
];

const connected = {
  okafor: { gdrive: 'nora.okafor@gmail.com', gmail: 'nora.okafor@gmail.com', icloud: 'nora@icloud.com' },
  hale: { outlook: 'nora@halewhitcombe.law', onedrive: 'nora@halewhitcombe.law', box: 'records@halewhitcombe.law' },
  fieldnote: { gdrive: 'nora@fieldnote.studio', slack: 'fieldnote.slack.com', notion: 'Fieldnote', dropbox: 'studio@fieldnote.studio' },
  personal: { github: 'nokafor' }
};

const syncs = ['2 min ago', '14 min ago', '1 hr ago', 'Yesterday'];

export function connectorsFor(archive) {
  return catalog.map((c, i) => {
    const account = connected[archive]?.[c.id];
    return { ...c, connected: Boolean(account), account: account ?? '', lastSync: account ? syncs[i % syncs.length] : '', items: account ? 40 + ((i * 137) % 900) : 0 };
  });
}
