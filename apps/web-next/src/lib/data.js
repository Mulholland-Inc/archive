export const me = {
  id: 'u-nora',
  name: 'Nora Okafor',
  first: 'Nora',
  email: 'nora@okafor.family',
  plan: 'Owner',
  tone: 1
};

export const workspaces = [
  { id: 'okafor', name: 'Okafor Family Archive', kind: 'Family', members: 9, tone: 1 },
  { id: 'hale', name: 'Hale & Whitcombe LLP', kind: 'Law firm', members: 42, tone: 3 },
  { id: 'fieldnote', name: 'Fieldnote Studio', kind: 'Studio', members: 14, tone: 2 },
  { id: 'personal', name: 'Personal', kind: 'Just you', members: 1, tone: 5 }
];

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

export const chatGroups = ['Starred', 'Today', 'Yesterday', 'Previous 7 days', 'Previous 30 days', 'Earlier'];

export const chats = [
  { id: 'letters-1974', title: "Transcribe Grandma Adaeze's 1974 letters", group: 'Today', starred: true, project: 'p-letters', updated: '12 min ago' },
  { id: 'mercer-timeline', title: 'Timeline for the Mercer lease dispute', group: 'Today', project: 'p-mercer', updated: '1 hr ago' },
  { id: 'exif-rename', title: 'Rename scanned photos by EXIF date', group: 'Today', updated: '3 hr ago' },
  { id: 'linden-brief', title: 'Moodboard brief for the Linden rebrand', group: 'Yesterday', starred: true, project: 'p-linden', updated: 'Yesterday' },
  { id: 'c5', title: 'Who is in the Lagos wedding photos', group: 'Yesterday', project: 'p-photos', updated: 'Yesterday' },
  { id: 'c6', title: 'Compare the two Enugu deeds', group: 'Yesterday', updated: 'Yesterday' },
  { id: 'c7', title: 'Recipe cards from Aunt Ifeoma', group: 'Yesterday', updated: 'Yesterday' },
  { id: 'c8', title: 'Clause 14.2 indemnity language', group: 'Previous 7 days', project: 'p-mercer', updated: 'Mon' },
  { id: 'c9', title: 'Deposition prep checklist', group: 'Previous 7 days', updated: 'Mon' },
  { id: 'c10', title: 'Tag faces in the 1988 album', group: 'Previous 7 days', project: 'p-photos', updated: 'Sun' },
  { id: 'c11', title: 'Summarize the zoning board minutes', group: 'Previous 7 days', updated: 'Sun' },
  { id: 'c12', title: 'Letterpress vendors in Rotterdam', group: 'Previous 7 days', project: 'p-linden', updated: 'Sat' },
  { id: 'c13', title: 'Translate the Igbo proverbs notebook', group: 'Previous 7 days', updated: 'Sat' },
  { id: 'c14', title: 'Billing narrative for October', group: 'Previous 7 days', updated: 'Fri' },
  { id: 'c15', title: 'Type scale options for Linden', group: 'Previous 7 days', project: 'p-linden', updated: 'Fri' },
  { id: 'c16', title: 'Family tree: the Nwosu side', group: 'Previous 30 days', updated: 'Oct 19' },
  { id: 'c17', title: 'Discovery request: email retention', group: 'Previous 30 days', project: 'p-mercer', updated: 'Oct 18' },
  { id: 'c18', title: 'Clean up the cassette transcripts', group: 'Previous 30 days', updated: 'Oct 16' },
  { id: 'c19', title: 'Draft a thank-you to the archivist', group: 'Previous 30 days', updated: 'Oct 15' },
  { id: 'c20', title: 'Brand voice principles, round two', group: 'Previous 30 days', project: 'p-linden', updated: 'Oct 14' },
  { id: 'c21', title: 'Conflict check for Brightwater', group: 'Previous 30 days', updated: 'Oct 12' },
  { id: 'c22', title: 'Restore water-damaged Polaroids', group: 'Previous 30 days', project: 'p-photos', updated: 'Oct 11' },
  { id: 'c23', title: 'Index the 1960s ledgers', group: 'Previous 30 days', updated: 'Oct 9' },
  { id: 'c24', title: 'Studio rate card for 2027', group: 'Previous 30 days', updated: 'Oct 8' },
  { id: 'c25', title: "Grandpa Obi's service record", group: 'Previous 30 days', updated: 'Oct 6' },
  { id: 'c26', title: 'Precedent search: quiet enjoyment', group: 'Previous 30 days', project: 'p-mercer', updated: 'Oct 4' },
  { id: 'c27', title: 'Photo captions for the reunion book', group: 'Earlier', updated: 'Sep 28' },
  { id: 'c28', title: 'Retainer agreement template', group: 'Earlier', updated: 'Sep 24' },
  { id: 'c29', title: 'Digitize the VHS tapes plan', group: 'Earlier', updated: 'Sep 21' },
  { id: 'c30', title: 'Signage wayfinding concepts', group: 'Earlier', updated: 'Sep 17' },
  { id: 'c31', title: 'Explain probate in plain English', group: 'Earlier', updated: 'Sep 12' },
  { id: 'c32', title: 'Naming ideas for the cookbook', group: 'Earlier', updated: 'Sep 9' },
  { id: 'c33', title: 'Witness statement summary', group: 'Earlier', updated: 'Sep 3' },
  { id: 'c34', title: 'Color study from the archive scans', group: 'Earlier', updated: 'Aug 30' },
  { id: 'c35', title: 'Birth certificates checklist', group: 'Earlier', updated: 'Aug 22' }
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
      steps: [
        { id: 's1', label: 'Read 3 images', detail: 'adaeze-letter-03.jpg, adaeze-letter-04.jpg, envelope-back.jpg', tool: 'vision', status: 'done', time: '4s' },
        { id: 's2', label: 'Searched the archive for “Adaeze”', detail: '41 matches across letters, photos and the family tree', tool: 'search', status: 'done', time: '1s' },
        { id: 's3', label: 'Matched names to the family tree', detail: '6 people linked, 1 unresolved', tool: 'tree', status: 'done', time: '2s' }
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
      steps: [
        { id: 's4', label: 'Created 2 documents in 1970s Letters', detail: 'Adaeze to Obi, 2 Mar 1974 · Adaeze to Obi, 16 Mar 1974', tool: 'files', status: 'done', time: '1s' },
        { id: 's5', label: 'Linked transcripts to scans', detail: '2 links', tool: 'files', status: 'done', time: '0s' }
      ],
      blocks: [
        { type: 'p', text: 'Done. Both transcripts now keep the original line breaks, with page turns marked as `— page 2 —`. They are saved next to the scans and show up in the letters timeline.' },
        {
          type: 'ul',
          items: ['**Adaeze to Obi, 2 Mar 1974** · 3 pages, 412 words', '**Adaeze to Obi, 16 Mar 1974** · 2 pages, 268 words']
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
      steps: [
        { id: 's1', label: 'Listed the Mercer matter folder', detail: '63 files · 18 emails, 22 PDFs, 9 letters, 14 notes', tool: 'files', status: 'done', time: '1s' },
        { id: 's2', label: 'Extracted dates from 63 files', detail: '214 dated events found, 37 relevant', tool: 'extract', status: 'done', time: '18s' },
        { id: 's3', label: 'Checked notices against lease §22', detail: 'Clause 22.1 requires written notice by recorded delivery', tool: 'review', status: 'done', time: '6s' }
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
      steps: [{ id: 's1', label: 'Read Linden kickoff notes.pdf', detail: '6 pages', tool: 'files', status: 'done', time: '2s' }],
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

export const streamReply = [
  { type: 'p', text: 'I looked through the archive for that. There are **14 items** that match, most of them from the 1970s letters and the Leeds photo album.' },
  { type: 'ul', items: ['9 letters between Enugu and Leeds, 1973 to 1976', '4 photos with handwritten captions on the back', '1 cassette recording, partly transcribed'] },
  { type: 'p', text: 'Would you like a short summary of each, or should I build a timeline from them?' }
];

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

export const projects = [
  { id: 'p-letters', name: '1970s Letters', updated: '12 min ago', files: 86, chats: 14, members: ['Nora Okafor', 'Chidi Okafor', 'Amara Okafor'], instructions: 'Transcribe faithfully, keep original spelling, and mark unreadable passages with [brackets]. Link every person to the family tree.', starred: true },
  { id: 'p-photos', name: 'Photo albums', updated: 'Yesterday', files: 1240, chats: 9, members: ['Nora Okafor', 'Tobi Okafor'], instructions: 'Date photos from their contents when there is no caption. Never guess names; ask.' },
  { id: 'p-mercer', name: 'Mercer v. Brightwater', updated: '1 hr ago', files: 63, chats: 22, members: ['Ruth Hale', 'Daniel Whitcombe', 'Priya Raman', 'Nora Okafor'], instructions: 'Cite the source document for every fact. Use British spelling and the firm’s citation style.' },
  { id: 'p-linden', name: 'Linden Library rebrand', updated: 'Yesterday', files: 212, chats: 17, members: ['Jun Park', 'Lea Fontaine', 'Nora Okafor'], instructions: 'Write in the studio voice: short, concrete, no superlatives.' },
  { id: 'p-tree', name: 'Family tree', updated: 'Mon', files: 18, chats: 6, members: ['Nora Okafor', 'Emeka Okafor'], instructions: 'Only owners change the tree. Suggest edits as comments.' },
  { id: 'p-cassettes', name: 'Cassette tapes', updated: 'Oct 20', files: 48, chats: 4, members: ['Nora Okafor'], instructions: 'Transcribe in both Igbo and English, side by side.' },
  { id: 'p-recipes', name: 'Family cookbook', updated: 'Oct 12', files: 37, chats: 8, members: ['Ifeoma Nwosu', 'Nora Okafor', 'Amara Okafor'], instructions: 'Convert to metric and keep the original notes in italics.' },
  { id: 'p-property', name: 'Enugu property', updated: 'Sep 30', files: 24, chats: 3, members: ['Emeka Okafor', 'Nora Okafor'], instructions: 'Summaries must be plain English for the whole family.' }
];

export const tasks = [
  {
    id: 't-cassettes',
    title: 'Transcribe cassettes 5 to 8',
    status: 'running',
    started: '6 min ago',
    elapsed: '6m 12s',
    agent: 'Transcriber',
    progress: 0.62,
    steps: [
      { id: 'a', label: 'Find untranscribed cassettes', detail: '4 tapes, 8 sides, 3h 54m of audio', status: 'done', time: '0:04' },
      { id: 'b', label: 'Transcribe cassette 5', detail: 'Igbo and English, 2 speakers', status: 'done', time: '1:48' },
      { id: 'c', label: 'Transcribe cassette 6', detail: '3 speakers, music at the start of side B', status: 'done', time: '1:52' },
      { id: 'd', label: 'Transcribe cassette 7', detail: 'Side A, 18 of 31 minutes', status: 'running', time: '2:28' },
      { id: 'e', label: 'Transcribe cassette 8', detail: '', status: 'pending', time: '' },
      { id: 'f', label: 'Link speakers to the family tree', detail: '', status: 'pending', time: '' }
    ]
  },
  {
    id: 't-mercer',
    title: 'Check every Mercer notice against the lease',
    status: 'running',
    started: '14 min ago',
    elapsed: '14m 03s',
    agent: 'Paralegal',
    progress: 0.4,
    steps: [
      { id: 'a', label: 'Collect notices from the matter folder', detail: '7 notices', status: 'done', time: '0:12' },
      { id: 'b', label: 'Extract service method and date', detail: '7 of 7', status: 'done', time: '1:30' },
      { id: 'c', label: 'Compare against clauses 9.3 and 22.1', detail: 'Notice 4 of 7', status: 'running', time: '12:21' },
      { id: 'd', label: 'Write the advice note', detail: '', status: 'pending', time: '' }
    ]
  },
  {
    id: 't-faces',
    title: 'Tag people in the 1988 album',
    status: 'waiting',
    started: '1 hr ago',
    elapsed: '22m 40s',
    agent: 'Photo tagger',
    progress: 0.8,
    steps: [
      { id: 'a', label: 'Detect faces in 212 photos', detail: '640 faces', status: 'done', time: '4:10' },
      { id: 'b', label: 'Match faces to known people', detail: '588 matched', status: 'done', time: '17:52' },
      { id: 'c', label: 'Ask about 52 unknown faces', detail: 'Waiting for Nora', status: 'waiting', time: '' },
      { id: 'd', label: 'Write tags to the photos', detail: '', status: 'pending', time: '' }
    ]
  },
  {
    id: 't-digest',
    title: 'Weekly digest for the family',
    status: 'completed',
    started: 'Yesterday',
    elapsed: '1m 48s',
    agent: 'Digest',
    progress: 1,
    steps: [
      { id: 'a', label: 'Gather this week’s additions', detail: '38 new items', status: 'done', time: '0:20' },
      { id: 'b', label: 'Pick highlights', detail: '5 items', status: 'done', time: '0:41' },
      { id: 'c', label: 'Write the digest', detail: '420 words', status: 'done', time: '0:39' },
      { id: 'd', label: 'Email 9 members', detail: 'Delivered', status: 'done', time: '0:08' }
    ]
  },
  {
    id: 't-ledger',
    title: 'Index the 1960s household ledgers',
    status: 'completed',
    started: 'Oct 9',
    elapsed: '38m 02s',
    agent: 'Indexer',
    progress: 1,
    steps: [
      { id: 'a', label: 'Read 14 ledger scans', detail: '1,120 rows', status: 'done', time: '21:30' },
      { id: 'b', label: 'Normalize currencies', detail: 'Pounds and naira', status: 'done', time: '6:12' },
      { id: 'c', label: 'Create the ledger table', detail: '1,118 rows, 2 flagged', status: 'done', time: '10:20' }
    ]
  },
  {
    id: 't-restore',
    title: 'Restore water-damaged Polaroids',
    status: 'failed',
    started: 'Oct 11',
    elapsed: '3m 15s',
    agent: 'Restorer',
    progress: 0.3,
    steps: [
      { id: 'a', label: 'Load 24 Polaroid scans', detail: '24 images', status: 'done', time: '0:12' },
      { id: 'b', label: 'Restore batch 1', detail: '12 images', status: 'done', time: '2:40' },
      { id: 'c', label: 'Restore batch 2', detail: 'Scans are below 300 dpi', status: 'failed', time: '0:23' }
    ]
  },
  {
    id: 't-backup',
    title: 'Monthly export to cold storage',
    status: 'scheduled',
    started: 'Nov 1, 02:00',
    elapsed: '',
    agent: 'Exporter',
    progress: 0,
    steps: [
      { id: 'a', label: 'Export everything added in October', detail: '', status: 'pending', time: '' },
      { id: 'b', label: 'Verify checksums', detail: '', status: 'pending', time: '' }
    ]
  }
];

export const groups = [
  { id: 'g1', name: 'Okafor siblings', members: 4 },
  { id: 'g2', name: 'Nwosu cousins', members: 3 },
  { id: 'g3', name: 'Archivists', members: 2 },
  { id: 'g4', name: 'Guests', members: 2 }
];

export const members = [
  { id: 'u1', name: 'Nora Okafor', email: 'nora@okafor.family', role: 'owner', groups: ['Okafor siblings', 'Archivists'], active: 'Now', status: 'active', tone: 1 },
  { id: 'u2', name: 'Emeka Okafor', email: 'emeka@okafor.family', role: 'owner', groups: ['Okafor siblings'], active: '2 hr ago', status: 'active', tone: 3 },
  { id: 'u3', name: 'Chidi Okafor', email: 'chidi.okafor@mail.ng', role: 'editor', groups: ['Okafor siblings'], active: 'Yesterday', status: 'active', tone: 2 },
  { id: 'u4', name: 'Amara Okafor', email: 'amara@okafor.family', role: 'editor', groups: ['Okafor siblings', 'Archivists'], active: 'Today', status: 'active', tone: 4 },
  { id: 'u5', name: 'Ifeoma Nwosu', email: 'ifeoma.nwosu@mail.ng', role: 'editor', groups: ['Nwosu cousins'], active: 'Mon', status: 'active', tone: 5 },
  { id: 'u6', name: 'Tobi Okafor', email: 'tobi@okafor.family', role: 'viewer', groups: [], active: 'Oct 20', status: 'active', tone: 6 },
  { id: 'u7', name: 'Kelechi Nwosu', email: 'kelechi@nwosu.co.uk', role: 'viewer', groups: ['Nwosu cousins'], active: 'Oct 14', status: 'active', tone: 2 },
  { id: 'u8', name: 'Ada Nwosu', email: 'ada.nwosu@mail.com', role: 'viewer', groups: ['Nwosu cousins'], active: '', status: 'invited', tone: 3 },
  { id: 'u9', name: 'Margaret Ellis', email: 'm.ellis@leedsarchives.org', role: 'viewer', groups: ['Guests'], active: 'Sep 30', status: 'active', tone: 4 },
  { id: 'u10', name: 'Samuel Ibe', email: 'samuel.ibe@mail.com', role: 'viewer', groups: ['Guests'], active: '', status: 'invited', tone: 1 },
  { id: 'u11', name: 'Uche Okafor', email: 'uche@okafor.family', role: 'editor', groups: ['Okafor siblings'], active: 'Oct 2', status: 'active', tone: 5 }
];

export const people = ['Ruth Hale', 'Daniel Whitcombe', 'Priya Raman', 'Jun Park', 'Lea Fontaine', 'Chidi Okafor', 'Amara Okafor', 'Tobi Okafor', 'Emeka Okafor', 'Ifeoma Nwosu'];

export function initials(name = '') {
  return name
    .split(/\s+/)
    .filter((w) => /^[A-Za-z]/.test(w))
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join('');
}

export function toneFor(name = '') {
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return (h % 6) + 1;
}
