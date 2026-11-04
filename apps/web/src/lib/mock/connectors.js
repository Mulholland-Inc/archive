// How the app shows each connector the API has (src/connect), personal or the archive's integration; one the API
// lists that is not here still shows, plainly.
export const catalog = [
  { id: 'gdrive', by: 'Google', description: 'Search and read your files, and make or change them when you ask', name: 'Google Drive', category: 'Files', icon: 'drive', scopes: ['Read your files and documents', 'Make, change and trash files when you ask'] },
  { id: 'gcal', by: 'Google', description: 'Read your calendars’ events and who is invited, and book or change events when you ask', name: 'Google Calendar', category: 'Calendar', icon: 'calendar', scopes: ['Read your calendars and events', 'Make, change and cancel events when you ask'] },
  { id: 'gmail', by: 'Google', description: 'Read your mail and attachments, and send, draft or label mail when you ask', name: 'Gmail', category: 'Email', icon: 'mail', scopes: ['Read your mail and its attachments', 'Send, draft and label mail when you ask'] },
  { id: 'sharepoint', by: 'Microsoft', description: 'Search and read the files of your SharePoint sites and OneDrive, never change them', name: 'SharePoint', category: 'Files', icon: 'drive', scopes: ['Read files in every site and library you can read'] },
  { id: 'outlook', by: 'Microsoft', description: 'Read your Microsoft 365 or Outlook.com mail and calendar, never send or change them', name: 'Outlook', category: 'Email', icon: 'inbox', scopes: ['Read your mail and its attachments', 'Read your calendar'] },
  { id: 'teams', by: 'Microsoft', description: 'Read your Teams chats and the channels of your teams, never post or change them', name: 'Microsoft Teams', category: 'Chat', icon: 'chat', scopes: ['Read your chats', 'Read the channels of your teams'] },
  { id: 'notion', by: 'Notion', description: 'Search, read and write the pages and databases you allow', name: 'Notion', category: 'Docs', icon: 'notebook', scopes: ['Read and write pages you allow'] },
  { id: 'github', by: 'GitHub', description: 'Read issues, pull requests and files of your repositories', name: 'GitHub', category: 'Code', icon: 'branch', scopes: ['Read repositories you install Archive on'] },
  { id: 'quickbooks', by: 'Intuit', description: 'Read your company’s invoices, customers and books', name: 'QuickBooks', category: 'Finance', icon: 'ledger', scopes: ['Read one company’s accounting'] },
  { id: 'stripe', by: 'Stripe', description: 'Look up customers, payments, invoices and subscriptions', name: 'Stripe', category: 'Finance', icon: 'card', scopes: ['Use your Stripe account as you allow'] },
  { id: 'salesforce', by: 'Salesforce', description: 'Read accounts, contacts, leads and opportunities', name: 'Salesforce', category: 'CRM', icon: 'users', scopes: ['Read your org’s records through the API'] },
  { id: 'klaviyo', by: 'Klaviyo', description: 'Read profiles, lists and campaigns', name: 'Klaviyo', category: 'Marketing', icon: 'mail', scopes: ['Read profiles, lists, campaigns and metrics'] },
  { id: 'karbon', by: 'Karbon', description: 'Read the firm’s clients, people and work items', name: 'Karbon', category: 'Practice', icon: 'list', scopes: ['Read contacts, organizations and work with the firm’s API keys'] },
  { id: 'shopify', by: 'Shopify', description: 'Read the store’s orders, products and customers', name: 'Shopify', category: 'Commerce', icon: 'card', scopes: ['Read orders, products and customers with the store’s app token'] },
  { id: 'meta_ads', by: 'Meta', description: 'Read an ad account’s campaigns and what they spent and got each day', name: 'Meta Ads', category: 'Marketing', icon: 'users', scopes: ['Read the ad account’s campaigns and results'] },
  { id: 'opendental', by: 'Open Dental', description: 'Read the practice’s patients, appointments, procedures and payments', name: 'Open Dental', category: 'Dental', icon: 'users', scopes: ['Read practice data with the practice’s customer key'] },
  { id: 'hubspot', by: 'HubSpot', description: 'Read contacts, companies and deals', name: 'HubSpot', category: 'CRM', icon: 'users', scopes: ['Read contacts, companies and deals'] },
  { id: 'asana', by: 'Asana', description: 'Read workspaces, projects and tasks', name: 'Asana', category: 'Projects', icon: 'list', scopes: ['Read projects, tasks and comments'] },
  { id: 'linear', by: 'Linear', description: 'Search and work with issues, projects and cycles', name: 'Linear', category: 'Projects', icon: 'list', scopes: ['Read and write issues as you'] },
  { id: 'sentry', by: 'Sentry', description: 'Look into issues, events and traces', name: 'Sentry', category: 'Code', icon: 'bug', scopes: ['Read your organization’s issues'] },
  { id: 'gusto', by: 'Gusto', description: 'Read the company, its people and its payrolls', name: 'Gusto', category: 'Payroll', icon: 'users', scopes: ['Read the company, employees, contractors and payrolls'] }
];
