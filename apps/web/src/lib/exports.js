// Where a person asks each service for a copy of their data, for the Import connector's dialog. The full steps and wait times
// are in the setup guide; keep the two in step.
export const exportSources = [
  { name: 'Google Takeout', how: 'Choose .zip, and JSON for My Activity. Hours to days.', url: 'https://takeout.google.com' },
  { name: 'Apple', how: 'Request a copy of your data. Up to a week.', url: 'https://privacy.apple.com' },
  { name: 'Amazon orders', how: 'Request your data, then Your Orders. A few days.', url: 'https://www.amazon.com/hz/privacy-central/data-requests/preview.html' },
  { name: 'Facebook and Instagram', how: 'Download your information, as JSON. Hours to days.', url: 'https://accountscenter.facebook.com/info_and_permissions/dyi' },
  { name: 'ChatGPT', how: 'Data controls, then Export data. Minutes.', url: 'https://chatgpt.com/#settings/DataControls' },
  { name: 'Claude', how: 'Privacy, then Export data. Minutes.', url: 'https://claude.ai/settings/data-privacy-controls' },
  { name: 'Goodreads', how: 'Export Library. Minutes.', url: 'https://www.goodreads.com/review/import' },
  { name: 'Letterboxd', how: 'Export your data. Right away.', url: 'https://letterboxd.com/settings/data/' },
  { name: 'Family tree', how: 'Export a GEDCOM from Ancestry, MyHeritage or your tree app.' },
  { name: 'Raw DNA', how: 'Download raw data from 23andMe, AncestryDNA or MyHeritage.' },
  { name: 'WhatsApp', how: 'In a chat, Export Chat. Right away.' },
  { name: 'iMessage', how: 'Copy chat.db from ~/Library/Messages on a Mac.' }
];
