// MOCK: made-up data; see index.js for what is still mocked.
const done = (label, detail, time) => ({ label, detail, status: 'done', time });
const at = (label, status = 'pending', detail = '', time = '') => ({ label, detail, status, time });

function run(number, status, started, elapsed, steps) {
  return {
    id: `r${number}`,
    number,
    status,
    started,
    elapsed,
    seconds: [...elapsed.matchAll(/(\d+)([ms])/g)].reduce((n, [, v, u]) => n + Number(v) * (u === 'm' ? 60 : 1), 0),
    steps: steps.map((s, i) => ({ id: `s${i}`, ...s })),
    log: []
  };
}

function finished(number, started, elapsed, labels) {
  return run(number, 'succeeded', started, elapsed, labels.map((l) => done(l, '', '')));
}

const okafor = [
  {
    id: 'wf-cassettes',
    name: 'Transcribe each recording added to Cassettes and file the transcript next to it',
    agent: 'Transcriber',
    trigger: { kind: 'file', label: 'When audio is added to Cassettes' },
    enabled: true,
    steps: ['Find untranscribed audio', 'Transcribe each side', 'Link speakers to the family tree', 'Save transcripts next to the audio'],
    runs: [
      run(7, 'running', '6 min ago', '6m 12s', [
        done('Finding the tapes nobody has transcribed yet', '4 tapes, 8 sides, 3h 54m of audio', '0:04'),
        done('Transcribing cassette 5', 'Igbo and English, two speakers', '1:48'),
        done('Transcribing cassette 6', 'Three speakers, music at the start of side B', '1:52'),
        at('Transcribing cassette 7', 'running', 'Side A, 18 of 31 minutes'),
        at('Linking each speaker to the family tree'),
        at('Saving the transcripts next to the audio')
      ]),
      finished(6, 'Oct 20', '22m 40s', ['Find untranscribed audio', 'Transcribe cassettes 1 to 4', 'Link speakers to the family tree', 'Save transcripts next to the audio'])
    ]
  },
  {
    id: 'wf-faces',
    name: 'Tag who is in each new photo from iCloud Photos and ask when it isn\'t sure',
    agent: 'Photo tagger',
    trigger: { kind: 'file', label: 'When photos arrive from iCloud Photos', connector: 'icloud' },
    enabled: true,
    steps: ['Detect faces', 'Match faces to known people', 'Ask about unknown faces', 'Write tags to the photos'],
    runs: [
      run(12, 'waiting', '1 hr ago', '22m 40s', [
        done('Finding faces in the 212 new photos', '640 faces', '4:10'),
        done('Matching faces to people you have already named', '588 matched', '17:52'),
        at('Asking you about 52 faces it doesn’t recognise', 'waiting', 'Waiting for you'),
        at('Writing the names onto the photos')
      ]),
      finished(11, 'Oct 22', '3m 05s', ['Detect faces', 'Match faces to known people', 'Write tags to the photos'])
    ]
  },
  {
    id: 'wf-digest',
    name: 'Every Sunday evening, email the family a digest of what was added that week',
    agent: 'Digest',
    trigger: { kind: 'schedule', label: 'Every Sunday at 18:00' },
    enabled: true,
    next: 'Sun, 18:00',
    steps: ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member'],
    runs: [
      run(31, 'succeeded', 'Yesterday', '1m 48s', [
        done('Gathering what was added this week', '38 new items', '0:20'),
        done('Picking five highlights', '', '0:41'),
        done('Writing the digest', '420 words', '0:39'),
        done('Emailing it to everyone', '9 delivered', '0:08')
      ]),
      finished(30, 'Oct 19', '1m 52s', ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member']),
      finished(29, 'Oct 12', '2m 03s', ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member'])
    ]
  },
  {
    id: 'wf-ledgers',
    name: 'Read the scanned ledgers and turn every entry into a searchable row',
    agent: 'Indexer',
    trigger: { kind: 'manual', label: 'Run by hand' },
    enabled: true,
    steps: ['Read ledger scans', 'Normalize currencies', 'Update the ledger table'],
    runs: [
      run(2, 'succeeded', 'Oct 9', '38m 02s', [
        done('Reading the 14 ledger scans', '1,120 rows', '21:30'),
        done('Converting pounds and naira to one currency', '', '6:12'),
        done('Updating the ledger table', '1,118 rows, 2 flagged for you', '10:20')
      ])
    ]
  },
  {
    id: 'wf-restore',
    name: 'Repair scratches and fading in old photos and keep the originals untouched',
    agent: 'Restorer',
    trigger: { kind: 'manual', label: 'Run by hand' },
    enabled: true,
    steps: ['Load scans', 'Restore in batches', 'Save restored copies'],
    runs: [
      run(1, 'failed', 'Oct 11', '3m 15s', [
        done('Opening the 24 Polaroid scans', '', '0:12'),
        done('Restoring the first 12', '', '2:40'),
        at('Restoring the other 12', 'failed', 'These scans are below 300 dpi', '0:23'),
        at('Saving the restored copies')
      ])
    ]
  },
  {
    id: 'wf-export',
    name: 'On the 1st of every month, export the whole archive to cold storage',
    agent: 'Exporter',
    trigger: { kind: 'schedule', label: '1st of every month, 02:00' },
    enabled: true,
    next: 'Nov 1, 02:00',
    steps: ['Export everything added last month', 'Verify checksums', 'Send a receipt'],
    runs: [finished(9, 'Oct 1', '12m 30s', ['Export everything added last month', 'Verify checksums', 'Send a receipt'])]
  }
];

const hale = [
  {
    id: 'wf-notices',
    name: 'Check each notice that arrives in the Mercer inbox against the lease and flag conflicts',
    agent: 'Paralegal',
    trigger: { kind: 'email', label: 'When a notice arrives in the Mercer inbox', connector: 'outlook' },
    enabled: true,
    steps: ['Collect notices from the matter', 'Extract service method and date', 'Compare against the lease', 'Write the advice note'],
    runs: [
      run(4, 'running', '14 min ago', '14m 03s', [
        done('Collecting every notice in the matter', '7 notices', '0:12'),
        done('Pulling out how and when each was served', '7 of 7', '1:30'),
        at('Checking each against clauses 9.3 and 22.1', 'running', 'Notice 4 of 7'),
        at('Writing the advice note')
      ]),
      finished(3, 'Oct 30', '9m 12s', ['Collect notices from the matter', 'Extract service method and date', 'Compare against the lease', 'Write the advice note'])
    ]
  },
  {
    id: 'wf-billing',
    name: 'On the last Friday of the month, draft billing narratives from the month\'s time entries',
    agent: 'Billing',
    trigger: { kind: 'schedule', label: 'Last Friday of every month' },
    enabled: true,
    next: 'Nov 28, 09:00',
    steps: ['Read time entries', 'Group by matter', 'Write narratives'],
    runs: [
      run(10, 'succeeded', 'Oct 31', '4m 10s', [done('Reading 212 time entries', '', '0:40'), done('Grouping them by matter', '9 matters', '0:30'), done('Drafting a narrative for each', '9 drafts', '3:00')])
    ]
  }
];

const fieldnote = [
  {
    id: 'wf-references',
    name: 'Sort each image added to the Linden folder in Google Drive into the right moodboard',
    agent: 'Researcher',
    trigger: { kind: 'file', label: 'When images are added to the Linden folder in Google Drive', connector: 'gdrive' },
    enabled: true,
    steps: ['Read new images', 'Find enamel and cast lettering', 'Make a board'],
    runs: [
      run(5, 'running', '3 min ago', '3m 02s', [
        done('Looking at the 64 new images', '', '0:20'),
        at('Picking out enamel and cast lettering', 'running', '18 of 64'),
        at('Laying them out on a board')
      ])
    ]
  }
];

export const workflowSeed = { okafor, hale, fieldnote, personal: [] };

export const triggerKinds = [
  { value: 'manual', label: 'Run by hand' },
  { value: 'schedule', label: 'On a schedule' },
  { value: 'file', label: 'When a file is added' },
  { value: 'email', label: 'When an email arrives' }
];
