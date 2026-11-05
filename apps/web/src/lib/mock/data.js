// MOCK: made-up data; see index.js for what is still mocked.
export const me = {
  id: 'u-nora',
  name: 'Nora Okafor',
  first: 'Nora',
  email: 'nora@okafor.family',
  plan: 'Owner',
  tone: 1
};

export const conversations = {
  'letters-1974': [
    {
      id: 'm1',
      role: 'user',
      attachments: [
        { id: 'a1', name: 'adaeze-letter-03.jpg', kind: 'image', size: '2.4 MB' },
        { id: 'a2', name: 'adaeze-letter-04.jpg', kind: 'image', size: '2.1 MB' },
        { id: 'a3', name: 'envelope-back.jpg', kind: 'image', size: '1.3 MB' }
      ],
      text: 'Can you transcribe these two letters from Grandma Adaeze and tell me who she mentions? The envelope has a postmark I can never read.'
    },
    {
      id: 'm2',
      role: 'assistant',
      worked: '7s',
      steps: [
        { id: 's1', label: 'Reading the handwriting in both letters and on the envelope', status: 'done', time: '4s' },
        { id: 's2', label: 'Looking through 41 other items that mention Adaeze', status: 'done', time: '1s' },
        { id: 's3', label: 'Matching six of the names to people in the family tree', status: 'done', time: '2s' }
      ],
      blocks: [
        { type: 'p', text: 'Both letters are in Adaeze’s handwriting and were written two weeks apart in **March 1974**, from Enugu to your grandfather in Leeds. The postmark reads **ENUGU 18 MR 74**, which matches the date at the top of the second letter.' },
        { type: 'h', text: 'Letter one, 2 March 1974' },
        { type: 'quote', text: 'My dear Obi, the rains came early and the roof over the kitchen has held, thanks to your brother. Chidi is growing tall and asks every evening when Papa will send the picture of the snow.' },
        { type: 'p', text: 'She goes on to describe the harvest, a visit from her sister Ifeoma, and money that arrived safely through the post office. The last paragraph is partly water-damaged; I marked the gaps with brackets in the full transcript.' },
        { type: 'h', text: 'People mentioned' },
        {
          type: 'table',
          head: ['Name', 'Relation', 'In the tree'],
          rows: [
            ['Obi Okafor', 'Husband, recipient', 'Yes'],
            ['Chidi Okafor', 'Son, age 6', 'Yes'],
            ['Ifeoma Nwosu', 'Sister', 'Yes'],
            ['Emeka', 'Brother-in-law', 'Likely Emeka Okafor'],
            ['Mama Ngozi', 'Neighbour', 'Not found']
          ]
        },
        { type: 'h', text: 'What I’d do next' },
        {
          type: 'ol',
          items: [
            'Save both transcripts to **1970s Letters** and link them to the scans.',
            'Add **Mama Ngozi** to the tree as a family friend so later letters can be linked.',
            'Ask Uncle Chidi about the “picture of the snow”; there is an unlabelled 1974 photo of Leeds in the album that may be it.'
          ]
        },
        { type: 'p', text: 'Want me to save the transcripts and create the tree entry?' }
      ]
    },
    {
      id: 'm3',
      role: 'user',
      text: 'Yes, save both. And can you make the transcripts match the line breaks of the originals?'
    },
    {
      id: 'm4',
      role: 'assistant',
      worked: '3s',
      task: true,
      steps: [
        { id: 's4', label: 'Matching the line breaks of the originals', status: 'done', time: '2s' },
        { id: 's5', label: 'Saving both transcripts to 1970s Letters', status: 'done', time: '1s' },
        { id: 's6', label: 'Linking each transcript to its scan', status: 'done', time: '0s' }
      ],
      blocks: [
        { type: 'p', text: 'Done. Both transcripts now keep the original line breaks, with page turns marked as `— page 2 —`. They are saved next to the scans and show up in the letters timeline.' },
        {
          type: 'ul',
          items: ['**Adaeze to Obi, 2 Mar 1974** (3 pages, 412 words)', '**Adaeze to Obi, 16 Mar 1974** (2 pages, 268 words)']
        },
        { type: 'p', text: 'I didn’t add Mama Ngozi to the tree yet, since only owners can change it. You can do that from the letter itself.' }
      ]
    }
  ],
  'mercer-timeline': [
    {
      id: 'm1',
      role: 'user',
      text: 'Build a chronology of the Mercer lease dispute from everything in the matter folder. Flag anything that looks like a notice defect.'
    },
    {
      id: 'm2',
      role: 'assistant',
      worked: '25s',
      steps: [
        { id: 's1', label: 'Opening the 63 files in the Mercer matter folder', status: 'done', time: '1s' },
        { id: 's2', label: 'Pulling 214 dates out of the emails, letters and notes', status: 'done', time: '18s' },
        { id: 's3', label: 'Checking each notice against clause 22.1 of the lease', status: 'done', time: '6s' }
      ],
      blocks: [
        { type: 'p', text: 'Here is the chronology, condensed to the events that matter for the notice question. Full citations are in the matter note I created.' },
        {
          type: 'table',
          head: ['Date', 'Event', 'Source'],
          rows: [
            ['14 Jan 2025', 'Lease signed, 10-year term, break at year 5', 'Lease, p. 1'],
            ['2 Jun 2026', 'Landlord emails intent to redevelop', 'Email, R. Mercer'],
            ['9 Jun 2026', 'Break notice sent by ordinary post', 'Letter LM-0412'],
            ['30 Jun 2026', 'Tenant disputes validity of notice', 'Letter HW-0098'],
            ['15 Jul 2026', 'Landlord re-serves by recorded delivery', 'Royal Mail receipt'],
            ['1 Sep 2026', 'Claim issued, County Court', 'Claim form']
          ]
        },
        { type: 'h', text: 'Possible notice defects' },
        {
          type: 'ol',
          items: [
            '**Method of service.** The 9 June notice went by ordinary post. Clause 22.1 requires recorded delivery, so the first notice is likely invalid.',
            '**Timing of re-service.** The 15 July notice gives 5 months, one short of the 6 required by clause 9.3 for a break at year five.',
            '**Addressee.** Both notices name “Mercer Holdings Ltd”; the tenant entity changed to “Mercer Holdings (UK) Ltd” in March 2026.'
          ]
        },
        { type: 'quote', text: 'Any notice under this Lease shall be in writing and served by hand or by recorded delivery to the registered office of the party to be served.' },
        { type: 'p', text: 'The second and third points are the strongest. Shall I draft a short advice note for the partner on the re-service timing?' }
      ]
    },
    {
      id: 'm3',
      role: 'user',
      text: 'Yes, draft the advice note and check the tenant name change against Companies House.'
    },
    {
      id: 'm4',
      role: 'assistant',
      task: true,
      steps: [
        { id: 's4', label: 'Collecting the two notices and the receipts', status: 'done', time: '2s' },
        { id: 's5', label: 'Checking the tenant name at Companies House', status: 'done', time: '9s' },
        { id: 's6', label: 'Counting the notice period under clause 9.3', status: 'running' },
        { id: 's7', label: 'Writing the advice note for the partner', status: 'pending' }
      ],
      blocks: [{ type: 'p', text: 'The name change is confirmed: the tenant became **Mercer Holdings (UK) Ltd** on 11 March 2026. I’m working through the notice period now and will post the note here.' }]
    }
  ]
};

export const library = [
  { id: 'f1', name: 'Adaeze to Obi, 2 Mar 1974', kind: 'document', ext: 'DOC', size: '18 KB', modified: '12 min ago', owner: 'Nora Okafor', folder: '1970s Letters', tone: 1 },
  { id: 'f2', name: 'adaeze-letter-03.jpg', kind: 'image', ext: 'JPG', size: '2.4 MB', modified: '12 min ago', owner: 'Nora Okafor', folder: '1970s Letters', tone: 4 },
  { id: 'f3', name: 'Uncle Chidi interview, part 1', kind: 'recording', ext: 'M4A', size: '41 MB', modified: '2 hr ago', owner: 'Chidi Okafor', folder: 'Interviews', duration: '48:12', tone: 2 },
  { id: 'f4', name: 'Leeds, winter 1974', kind: 'image', ext: 'JPG', size: '3.1 MB', modified: 'Yesterday', owner: 'Nora Okafor', folder: 'Photo albums', tone: 3 },
  { id: 'f5', name: 'Enugu house deed (1969)', kind: 'pdf', ext: 'PDF', size: '1.2 MB', modified: 'Yesterday', owner: 'Emeka Okafor', folder: 'Property', tone: 1 },
  { id: 'f6', name: 'Family tree export', kind: 'spreadsheet', ext: 'CSV', size: '64 KB', modified: 'Yesterday', owner: 'Nora Okafor', folder: 'Family tree', tone: 2 },
];

export const members = [
  { id: 'u1', user: me.id, name: 'Nora Okafor', email: 'nora@okafor.family', role: 'owner', active: 'Now', status: 'active', tone: 1 },
  { id: 'u2', name: 'Emeka Okafor', email: 'emeka@okafor.family', role: 'owner', active: '2 hr ago', status: 'active', tone: 3 },
  { id: 'u3', name: 'Chidi Okafor', email: 'chidi.okafor@mail.ng', role: 'editor', active: 'Yesterday', status: 'active', tone: 2 },
  { id: 'u4', name: 'Amara Okafor', email: 'amara@okafor.family', role: 'editor', active: 'Today', status: 'active', tone: 4 },
  { id: 'u5', name: 'Ifeoma Nwosu', email: 'ifeoma.nwosu@mail.ng', role: 'editor', active: 'Mon', status: 'active', tone: 5 },
  { id: 'u6', name: 'Tobi Okafor', email: 'tobi@okafor.family', role: 'viewer', active: 'Oct 20', status: 'active', tone: 6 },
  { id: 'u7', name: 'Kelechi Nwosu', email: 'kelechi@nwosu.co.uk', role: 'viewer', active: 'Oct 14', status: 'active', tone: 2 },
  { id: 'u8', name: 'Ada Nwosu', email: 'ada.nwosu@mail.com', role: 'viewer', active: '', status: 'invited', tone: 3 },
  { id: 'u11', name: 'Uche Okafor', email: 'uche@okafor.family', role: 'editor', active: 'Oct 2', status: 'active', tone: 5 }
];

export function initials(name = '') {
  return name
    .split(/\s+/)
    .filter((w) => /^[A-Za-z]/.test(w))
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join('');
}

// Connectors for the component specimen, as the API lists them.
export const connectors = [
  { id: 'gmail', by: 'Google', description: 'Read your mail and attachments, and send, draft or label mail when you ask', name: 'Gmail', category: 'Email', icon: 'mail', scopes: ['Read your mail and its attachments', 'Send, draft and label mail when you ask'] },
  { id: 'quickbooks', by: 'Intuit', description: 'Read your company’s invoices, customers and books', name: 'QuickBooks', category: 'Finance', icon: 'ledger', scopes: ['Read one company’s accounting'] },
  { id: 'github', by: 'GitHub', description: 'Read issues, pull requests and files of your repositories', name: 'GitHub', category: 'Code', icon: 'branch', scopes: ['Read repositories you install Archive on'] }
];
