// How the app shows each connector the API has (src/connect), personal or the archive's integration; one the API
// lists that is not here still shows, plainly.
export const catalog = [
  { id: 'gdrive', by: 'Google', description: 'Search and read your files, never change them', name: 'Google Drive', category: 'Files', icon: 'drive', scopes: ['Read your files and documents'] },
  { id: 'gmail', by: 'Google', description: 'Read your mail and attachments, never send or change it', name: 'Gmail', category: 'Email', icon: 'mail', scopes: ['Read your mail and its attachments'] },
  { id: 'sharepoint', by: 'Microsoft', description: 'Search and read the files of your SharePoint sites and OneDrive, never change them', name: 'SharePoint', category: 'Files', icon: 'drive', scopes: ['Read files in every site and library you can read'] },
  { id: 'outlook', by: 'Microsoft', description: 'Read your Microsoft 365 or Outlook.com mail, never send or change it', name: 'Outlook', category: 'Email', icon: 'inbox', scopes: ['Read your mail and its attachments'] },
  { id: 'notion', by: 'Notion', description: 'Search, read and write the pages and databases you allow', name: 'Notion', category: 'Docs', icon: 'notebook', scopes: ['Read and write pages you allow'] },
  { id: 'github', by: 'GitHub', description: 'Read issues, pull requests and files of your repositories', name: 'GitHub', category: 'Code', icon: 'branch', scopes: ['Read repositories you install Archive on'] },
  { id: 'quickbooks', by: 'Intuit', description: 'Read your company’s invoices, customers and books', name: 'QuickBooks', category: 'Finance', icon: 'ledger', scopes: ['Read one company’s accounting'] },
  { id: 'stripe', by: 'Stripe', description: 'Look up customers, payments, invoices and subscriptions', name: 'Stripe', category: 'Finance', icon: 'card', scopes: ['Use your Stripe account as you allow'] },
  { id: 'hubspot', by: 'HubSpot', description: 'Read contacts, companies and deals', name: 'HubSpot', category: 'CRM', icon: 'users', scopes: ['Read contacts, companies and deals'] },
  { id: 'asana', by: 'Asana', description: 'Read workspaces, projects and tasks', name: 'Asana', category: 'Projects', icon: 'list', scopes: ['Read projects, tasks and comments'] },
  { id: 'linear', by: 'Linear', description: 'Search and work with issues, projects and cycles', name: 'Linear', category: 'Projects', icon: 'list', scopes: ['Read and write issues as you'] },
  { id: 'sentry', by: 'Sentry', description: 'Look into issues, events and traces', name: 'Sentry', category: 'Code', icon: 'bug', scopes: ['Read your organization’s issues'] },
  { id: 'gusto', by: 'Gusto', description: 'Read the company, its people and its payrolls', name: 'Gusto', category: 'Payroll', icon: 'users', scopes: ['Read the company, employees, contractors and payrolls'] }
];
