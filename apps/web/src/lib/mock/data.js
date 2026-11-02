// MOCK: made-up data; see index.js for what is still mocked.
export const me = {
  id: 'u-nora',
  name: 'Nora Okafor',
  first: 'Nora',
  email: 'nora@okafor.family',
  plan: 'Owner',
  tone: 1
};

export const models = [
  { id: 'pro', name: 'Archive 2 Pro' },
  { id: 'standard', name: 'Archive 2' },
  { id: 'flash', name: 'Archive 2 Flash' }
];

export const suggestions = [
  { id: 's1', label: 'Summarize', icon: 'files', prompt: 'Summarize everything in the 1970s letters folder' },
  { id: 's2', label: 'Find', icon: 'search', prompt: 'Find the deed for the Enugu house' },
  { id: 's3', label: 'Timeline', icon: 'timeline', prompt: 'Build a timeline of the family moving to Leeds' },
  { id: 's4', label: 'Transcribe', icon: 'audio', prompt: 'Transcribe the interview with Uncle Chidi' },
  { id: 's5', label: 'Automate', icon: 'workflow', prompt: 'Every time a scan is added, tag the people in it' }
];

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
  ],
  'exif-rename': [
    {
      id: 'm1',
      role: 'user',
      text: 'I have about 3,000 scanned photos with names like IMG_0042.jpg. Write a script that renames them to the date the photo was taken, like 1988-07-14_001.jpg.'
    },
    {
      id: 'm2',
      role: 'assistant',
      blocks: [
        { type: 'p', text: 'Here is a small Python script that reads the EXIF `DateTimeOriginal` field and renames each file. Scans often lack EXIF data, so it falls back to the file’s modified time and puts those in a separate `undated/` folder for you to check.' },
        {
          type: 'code',
          lang: 'python',
          code: `from pathlib import Path
from datetime import datetime
from PIL import Image

SOURCE = Path("~/Scans").expanduser()
UNDATED = SOURCE / "undated"

def taken(path):
    with Image.open(path) as img:
        raw = img.getexif().get_ifd(0x8769).get(36867)
    if raw:
        return datetime.strptime(raw, "%Y:%m:%d %H:%M:%S")
    return None

counts = {}
for path in sorted(SOURCE.glob("*.jpg")):
    when = taken(path)
    if when is None:
        UNDATED.mkdir(exist_ok=True)
        path.rename(UNDATED / path.name)
        continue
    day = when.strftime("%Y-%m-%d")
    counts[day] = counts.get(day, 0) + 1
    path.rename(SOURCE / f"{day}_{counts[day]:03d}.jpg")`
        },
        { type: 'p', text: 'Install the one dependency and run it from the folder the script is in:' },
        { type: 'code', lang: 'bash', code: 'python3 -m pip install pillow\npython3 rename_scans.py' },
        { type: 'h', text: 'Before you run it' },
        {
          type: 'ul',
          items: [
            'Try it on a **copy** of a few hundred files first. Renames can’t be undone.',
            'If two scans share a timestamp to the second, the counter keeps them apart.',
            'For flatbed scans the date is usually the *scan* date, not the photo date. Those are better dated from what’s in the picture.'
          ]
        },
        { type: 'p', text: 'I can also run this inside the archive instead, so the originals stay untouched and only the display names change.' }
      ]
    }
  ],
  'linden-brief': [
    {
      id: 'm1',
      role: 'user',
      attachments: [{ id: 'a1', name: 'Linden kickoff notes.pdf', kind: 'pdf', size: '480 KB' }],
      text: 'Turn the kickoff notes into a one-page moodboard brief for the team.'
    },
    {
      id: 'm2',
      role: 'assistant',
      worked: '6s',
      task: true,
      steps: [
        { id: 's1', label: 'Reading the six pages of kickoff notes', status: 'done', time: '2s' },
        { id: 's2', label: 'Pulling out goals, audiences and deadlines', status: 'done', time: '3s' },
        { id: 's3', label: 'Writing the one-page brief', status: 'done', time: '1s' }
      ],
      blocks: [
        { type: 'h', text: 'Linden Library rebrand: moodboard brief' },
        { type: 'p', text: 'Linden is a 90-year-old public library becoming a neighbourhood workshop: tools, seeds and sewing machines alongside books. The identity should feel **civic, handmade and durable**, never nostalgic.' },
        { type: 'h', text: 'Collect' },
        {
          type: 'ul',
          items: [
            'Municipal signage from the 1950s and 60s, especially enamel and cast letters',
            'Library stamps, date-due cards and catalogue drawers',
            'Workshop pegboards and tool outlines',
            'Risograph and letterpress prints with visible texture'
          ]
        },
        { type: 'h', text: 'Avoid' },
        { type: 'ul', items: ['Sepia filters and “vintage” pastiche', 'Book-spine clichés', 'Anything that reads as a café'] },
        { type: 'h', text: 'Palette direction' },
        { type: 'p', text: 'Start from the building itself: linden-leaf green, brick, and the pale stone of the entrance. One bright signal colour for wayfinding.' },
        { type: 'p', text: 'Boards are due **Thursday 6 November**, 20 to 30 images each, one board per person.' }
      ]
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
  { id: 'f7', name: 'Lagos wedding, roll 2', kind: 'image', ext: 'JPG', size: '2.8 MB', modified: 'Mon', owner: 'Amara Okafor', folder: 'Photo albums', tone: 4 },
  { id: 'f8', name: 'Aunt Ifeoma’s recipe cards', kind: 'document', ext: 'DOC', size: '22 KB', modified: 'Mon', owner: 'Ifeoma Nwosu', folder: 'Recipes', tone: 1 },
  { id: 'f9', name: 'Grandpa Obi, voice memo', kind: 'recording', ext: 'MP3', size: '6 MB', modified: 'Sun', owner: 'Nora Okafor', folder: 'Interviews', duration: '7:40', tone: 3 },
  { id: 'f10', name: 'Reunion 2019 group photo', kind: 'image', ext: 'HEIC', size: '4.6 MB', modified: 'Sun', owner: 'Tobi Okafor', folder: 'Photo albums', tone: 2 },
  { id: 'f11', name: 'Service record, 1958', kind: 'pdf', ext: 'PDF', size: '880 KB', modified: 'Sat', owner: 'Nora Okafor', folder: 'Military', tone: 4 },
  { id: 'f12', name: 'Igbo proverbs notebook', kind: 'pdf', ext: 'PDF', size: '12 MB', modified: 'Sat', owner: 'Chidi Okafor', folder: 'Notebooks', tone: 1 },
  { id: 'f13', name: 'Household ledger 1966', kind: 'spreadsheet', ext: 'XLSX', size: '140 KB', modified: 'Fri', owner: 'Nora Okafor', folder: 'Ledgers', tone: 3 },
  { id: 'f14', name: 'Christmas in Leeds, 1977', kind: 'image', ext: 'JPG', size: '2.2 MB', modified: 'Oct 22', owner: 'Amara Okafor', folder: 'Photo albums', tone: 1 },
  { id: 'f15', name: 'Cassette 4, side A', kind: 'recording', ext: 'WAV', size: '96 MB', modified: 'Oct 20', owner: 'Nora Okafor', folder: 'Cassettes', duration: '31:05', tone: 4 },
  { id: 'f16', name: 'Birth certificate, Chidi', kind: 'pdf', ext: 'PDF', size: '640 KB', modified: 'Oct 18', owner: 'Emeka Okafor', folder: 'Certificates', tone: 2 },
  { id: 'f17', name: 'Market stall, Onitsha', kind: 'image', ext: 'JPG', size: '1.9 MB', modified: 'Oct 15', owner: 'Nora Okafor', folder: 'Photo albums', tone: 3 },
  { id: 'f18', name: 'Reunion book captions', kind: 'document', ext: 'DOC', size: '31 KB', modified: 'Oct 12', owner: 'Tobi Okafor', folder: 'Reunion book', tone: 4 },
  { id: 'f19', name: 'Polaroid restore, batch 1', kind: 'image', ext: 'PNG', size: '8.4 MB', modified: 'Oct 11', owner: 'Nora Okafor', folder: 'Restorations', tone: 1 },
  { id: 'f20', name: 'Cassette 4, side B', kind: 'recording', ext: 'WAV', size: '92 MB', modified: 'Oct 10', owner: 'Nora Okafor', folder: 'Cassettes', duration: '29:51', tone: 2 },
  { id: 'f21', name: 'Passport stamps, 1972', kind: 'image', ext: 'JPG', size: '1.6 MB', modified: 'Oct 8', owner: 'Emeka Okafor', folder: 'Documents', tone: 3 },
  { id: 'f22', name: 'School report, Amara 1984', kind: 'pdf', ext: 'PDF', size: '420 KB', modified: 'Oct 6', owner: 'Amara Okafor', folder: 'School', tone: 4 },
  { id: 'f23', name: 'Wedding speech draft', kind: 'document', ext: 'DOC', size: '12 KB', modified: 'Oct 2', owner: 'Tobi Okafor', folder: 'Speeches', tone: 1 },
  { id: 'f24', name: 'Harmattan, 1979', kind: 'image', ext: 'JPG', size: '2.0 MB', modified: 'Sep 29', owner: 'Nora Okafor', folder: 'Photo albums', tone: 2 }
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
