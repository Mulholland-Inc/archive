const done = (label, detail, time, tool) => ({ label, detail, status: 'done', time, tool });
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
    name: 'Transcribe new cassettes',
    agent: 'Transcriber',
    trigger: { kind: 'file', label: 'When audio is added to Cassettes' },
    enabled: true,
    steps: ['Find untranscribed audio', 'Transcribe each side', 'Link speakers to the family tree', 'Save transcripts next to the audio'],
    runs: [
      run(7, 'running', '6 min ago', '6m 12s', [
        done('Find untranscribed audio', '4 tapes, 8 sides, 3h 54m', '0:04', 'search'),
        done('Transcribe cassette 5', 'Igbo and English, 2 speakers', '1:48', 'extract'),
        done('Transcribe cassette 6', '3 speakers, music at the start of side B', '1:52', 'extract'),
        at('Transcribe cassette 7', 'running', 'Side A, 18 of 31 minutes'),
        at('Link speakers to the family tree'),
        at('Save transcripts next to the audio')
      ]),
      finished(6, 'Oct 20', '22m 40s', ['Find untranscribed audio', 'Transcribe cassettes 1 to 4', 'Link speakers to the family tree', 'Save transcripts next to the audio'])
    ]
  },
  {
    id: 'wf-faces',
    name: 'Tag people in new photos',
    agent: 'Photo tagger',
    trigger: { kind: 'file', label: 'When photos are added to Photo albums' },
    enabled: true,
    steps: ['Detect faces', 'Match faces to known people', 'Ask about unknown faces', 'Write tags to the photos'],
    runs: [
      run(12, 'waiting', '1 hr ago', '22m 40s', [
        done('Detect faces in 212 photos', '640 faces', '4:10', 'vision'),
        done('Match faces to known people', '588 matched', '17:52', 'tree'),
        at('Ask about 52 unknown faces', 'waiting', 'Waiting for you'),
        at('Write tags to the photos')
      ]),
      finished(11, 'Oct 22', '3m 05s', ['Detect faces', 'Match faces to known people', 'Write tags to the photos'])
    ]
  },
  {
    id: 'wf-digest',
    name: 'Weekly family digest',
    agent: 'Digest',
    trigger: { kind: 'schedule', label: 'Every Sunday at 18:00' },
    enabled: true,
    next: 'Sun, 18:00',
    steps: ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member'],
    runs: [
      run(31, 'succeeded', 'Yesterday', '1m 48s', [
        done('Gather this week’s additions', '38 new items', '0:20', 'search'),
        done('Pick highlights', '5 items', '0:41'),
        done('Write the digest', '420 words', '0:39'),
        done('Email every member', '9 delivered', '0:08')
      ]),
      finished(30, 'Oct 19', '1m 52s', ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member']),
      finished(29, 'Oct 12', '2m 03s', ['Gather this week’s additions', 'Pick highlights', 'Write the digest', 'Email every member'])
    ]
  },
  {
    id: 'wf-ledgers',
    name: 'Index scanned ledgers',
    agent: 'Indexer',
    trigger: { kind: 'manual', label: 'Run by hand' },
    enabled: true,
    steps: ['Read ledger scans', 'Normalize currencies', 'Update the ledger table'],
    runs: [
      run(2, 'succeeded', 'Oct 9', '38m 02s', [
        done('Read 14 ledger scans', '1,120 rows', '21:30', 'extract'),
        done('Normalize currencies', 'Pounds and naira', '6:12'),
        done('Update the ledger table', '1,118 rows, 2 flagged', '10:20', 'files')
      ])
    ]
  },
  {
    id: 'wf-restore',
    name: 'Restore damaged photos',
    agent: 'Restorer',
    trigger: { kind: 'manual', label: 'Run by hand' },
    enabled: true,
    steps: ['Load scans', 'Restore in batches', 'Save restored copies'],
    runs: [
      run(1, 'failed', 'Oct 11', '3m 15s', [
        done('Load 24 Polaroid scans', '24 images', '0:12', 'files'),
        done('Restore batch 1', '12 images', '2:40', 'vision'),
        at('Restore batch 2', 'failed', 'Scans are below 300 dpi', '0:23'),
        at('Save restored copies')
      ])
    ]
  },
  {
    id: 'wf-export',
    name: 'Monthly export to cold storage',
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
    name: 'Check notices against the lease',
    agent: 'Paralegal',
    trigger: { kind: 'email', label: 'When a notice arrives in the Mercer inbox' },
    enabled: true,
    steps: ['Collect notices from the matter', 'Extract service method and date', 'Compare against the lease', 'Write the advice note'],
    runs: [
      run(4, 'running', '14 min ago', '14m 03s', [
        done('Collect notices from the matter', '7 notices', '0:12', 'files'),
        done('Extract service method and date', '7 of 7', '1:30', 'extract'),
        at('Compare against clauses 9.3 and 22.1', 'running', 'Notice 4 of 7'),
        at('Write the advice note')
      ]),
      finished(3, 'Oct 30', '9m 12s', ['Collect notices from the matter', 'Extract service method and date', 'Compare against the lease', 'Write the advice note'])
    ]
  },
  {
    id: 'wf-billing',
    name: 'Draft billing narratives',
    agent: 'Billing',
    trigger: { kind: 'schedule', label: 'Last Friday of every month' },
    enabled: true,
    next: 'Nov 28, 09:00',
    steps: ['Read time entries', 'Group by matter', 'Write narratives'],
    runs: [
      run(10, 'succeeded', 'Oct 31', '4m 10s', [done('Read time entries', '212 entries', '0:40'), done('Group by matter', '9 matters', '0:30'), done('Write narratives', '9 drafts', '3:00')])
    ]
  }
];

const fieldnote = [
  {
    id: 'wf-references',
    name: 'Collect references for moodboards',
    agent: 'Researcher',
    trigger: { kind: 'file', label: 'When images are added to Linden moodboard' },
    enabled: true,
    steps: ['Read new images', 'Find enamel and cast lettering', 'Make a board'],
    runs: [
      run(5, 'running', '3 min ago', '3m 02s', [
        done('Read new images', '64 images', '0:20', 'vision'),
        at('Find enamel and cast lettering', 'running', '18 of 64'),
        at('Make a board')
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
