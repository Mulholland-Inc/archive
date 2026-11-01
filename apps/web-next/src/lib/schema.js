const sql = { text: 'text', long: 'text', date: 'date', money: 'numeric(12,2)', number: 'integer', link: 'uuid', file: 'text', email: 'text', yes: 'boolean', place: 'text' };
export const typeLabels = { text: 'Text', long: 'Long text', date: 'Date', money: 'Money', number: 'Number', link: 'Link', file: 'File', email: 'Email', yes: 'Yes or no', place: 'Place' };

const c = (name, type, required = false, notes = '', to) => ({ name, type, required, notes, to });

function table(id, name, icon, rows, updated, access, workflows, columns, sample) {
  const all = [c('id', 'text', true, 'Set by the vault'), ...columns, c('created_at', 'date', true, 'Set by the vault')];
  return {
    id,
    name,
    icon,
    rows,
    updated,
    access,
    workflows,
    columns: all,
    links: columns.filter((x) => x.to).map((x) => ({ column: x.name, to: x.to })),
    sample: sample.map((r, i) => ({ id: `${id}-${i}`, ...Object.fromEntries(columns.slice(0, 4).map((x, j) => [x.name, r[j]])) }))
  };
}

export function ddl(t) {
  const lines = t.columns.map((x) => {
    const base = x.name === 'id' ? 'uuid primary key' : x.to ? `uuid references ${x.to}(id)` : sql[x.type];
    return `  ${x.name} ${base}${x.required && x.name !== 'id' ? ' not null' : ''}`;
  });
  return `create table ${t.id} (\n${lines.join(',\n')}\n);`;
}

const family = { owners: ['Archivists'], editors: ['Okafor siblings'], viewers: ['Nwosu cousins', 'Guests'] };
const familyPrivate = { owners: ['Archivists'], editors: ['Okafor siblings'], viewers: [] };
const firm = { owners: ['Partners'], editors: ['Litigation'], viewers: [] };
const firmBilling = { owners: ['Partners'], editors: [], viewers: ['Litigation'] };
const studio = { owners: ['Design'], editors: ['Research'], viewers: [] };
const solo = { owners: ['You'], editors: [], viewers: [] };

const okafor = [
  table('person', 'Person', 'person', 214, 'Today', family, ['Tag who is in each new photo from iCloud Photos and ask when it isn\'t sure'], [
    c('full_name', 'text', true, 'As written on documents'),
    c('born', 'date', false, 'Year only is fine'),
    c('birthplace', 'link', false, '', 'place'),
    c('parent', 'link', false, 'Up to two', 'person'),
    c('died', 'date'),
    c('notes', 'long')
  ], [
    ['Adaeze Okafor', '1931', 'Enugu', 'Chukwuma Okafor'],
    ['Emeka Okafor', '1958', 'Lagos', 'Adaeze Okafor'],
    ['Ngozi Nwosu', '1962', 'Lagos', 'Adaeze Okafor']
  ]),
  table('letter', 'Letter', 'letter', 386, 'Yesterday', family, ['Read the scanned ledgers and turn every entry into a searchable row'], [
    c('title', 'text', true),
    c('sender', 'link', true, '', 'person'),
    c('recipient', 'link', false, '', 'person'),
    c('sent_on', 'date', false, 'From the postmark'),
    c('scan', 'file', true, 'Front and back'),
    c('transcript', 'long')
  ], [
    ['Letter from Enugu', 'Adaeze Okafor', 'Emeka Okafor', 'Mar 1979'],
    ['Wedding thanks', 'Ngozi Nwosu', 'Adaeze Okafor', 'Jun 1984'],
    ['News from London', 'Emeka Okafor', 'Adaeze Okafor', 'Nov 1981']
  ]),
  table('photo', 'Photo', 'photo', 1842, '2 hr ago', family, ['Tag who is in each new photo from iCloud Photos and ask when it isn\'t sure', 'Repair scratches and fading in old photos and keep the originals untouched'], [
    c('caption', 'text', true),
    c('taken_on', 'date'),
    c('place', 'link', false, '', 'place'),
    c('event', 'link', false, '', 'event'),
    c('people', 'link', false, 'Tagged faces', 'person'),
    c('image', 'file', true)
  ], [
    ['Wedding, front steps', '1981', 'Lagos', 'Wedding, 1981'],
    ['Christmas at Enugu', '1975', 'Enugu', 'Christmas 1975'],
    ['First day of school', '1966', 'Enugu', '']
  ]),
  table('recording', 'Recording', 'recording', 48, '6 min ago', family, ['Transcribe each recording added to Cassettes and file the transcript next to it'], [
    c('title', 'text', true),
    c('recorded_on', 'date'),
    c('length', 'text', false, 'Minutes and seconds'),
    c('speaker', 'link', false, '', 'person'),
    c('audio', 'file', true),
    c('transcript', 'long', false, 'Filled by Transcriber')
  ], [
    ['Cassette 4, side A', '1988', '46:20', 'Adaeze Okafor'],
    ['Cassette 4, side B', '1988', '44:05', 'Adaeze Okafor'],
    ['Birthday songs', '1992', '12:41', 'Emeka Okafor']
  ]),
  table('place', 'Place', 'place', 63, 'Sep 28', family, [], [
    c('name', 'text', true),
    c('city', 'text'),
    c('country', 'text', true),
    c('lived_from', 'date'),
    c('location', 'place', false, 'Pin on the map')
  ], [
    ['Family house', 'Enugu', 'Nigeria', '1952'],
    ['Surulere flat', 'Lagos', 'Nigeria', '1977'],
    ['Peckham terrace', 'London', 'United Kingdom', '1981']
  ]),
  table('event', 'Event', 'event', 129, 'Sep 24', family, ['Every Sunday evening, email the family a digest of what was added that week'], [
    c('name', 'text', true),
    c('happened_on', 'date', true),
    c('place', 'link', false, '', 'place'),
    c('host', 'link', false, '', 'person'),
    c('notes', 'long')
  ], [
    ['Wedding, 1981', 'Jun 1981', 'Lagos', 'Adaeze Okafor'],
    ['Christmas 1975', 'Dec 1975', 'Enugu', 'Chukwuma Okafor'],
    ['Graduation', 'Jul 1986', 'London', 'Emeka Okafor']
  ]),
  table('recipe', 'Recipe', 'recipe', 37, 'Aug 30', familyPrivate, [], [
    c('name', 'text', true),
    c('cook', 'link', false, 'Whose recipe', 'person'),
    c('serves', 'number'),
    c('card', 'file', false, 'Scan of the handwritten card'),
    c('steps', 'long', true)
  ], [
    ['Jollof rice', 'Adaeze Okafor', '8', 'jollof.jpg'],
    ['Egusi soup', 'Adaeze Okafor', '6', 'egusi.jpg'],
    ['Chin chin', 'Ngozi Nwosu', '12', '']
  ]),
  table('document', 'Document', 'document', 512, 'Today', familyPrivate, ['Read the scanned ledgers and turn every entry into a searchable row', 'On the 1st of every month, export the whole archive to cold storage'], [
    c('title', 'text', true),
    c('kind', 'text', false, 'Deed, ledger, certificate'),
    c('dated', 'date'),
    c('about', 'link', false, '', 'person'),
    c('scan', 'file', true)
  ], [
    ['Land deed, Enugu', 'Deed', '1958', 'Chukwuma Okafor'],
    ['Shop ledger, 1972', 'Ledger', '1972', 'Adaeze Okafor'],
    ['Birth certificate', 'Certificate', '1958', 'Emeka Okafor']
  ])
];

const hale = [
  table('client', 'Client', 'briefcase', 142, 'Today', firm, [], [
    c('name', 'text', true),
    c('contact', 'email', true),
    c('since', 'date'),
    c('partner', 'text', true, 'Responsible partner'),
    c('conflict_checked', 'yes', true)
  ], [
    ['Mercer Holdings', 'legal@mercer.com', 'Mar 2019', 'Ruth Hale'],
    ['Brightwater Ltd', 'ops@brightwater.co', 'Jan 2022', 'Daniel Whitcombe'],
    ['Cole Family Trust', 'j.cole@mail.com', 'Aug 2024', 'Ruth Hale']
  ]),
  table('matter', 'Matter', 'folder', 318, '1 hr ago', firm, ['Check each notice that arrives in the Mercer inbox against the lease and flag conflicts'], [
    c('title', 'text', true),
    c('client', 'link', true, '', 'client'),
    c('opened_on', 'date', true),
    c('status', 'text', true, 'Open, on hold, closed'),
    c('budget', 'money')
  ], [
    ['Mercer v. Brightwater', 'Mercer Holdings', 'Feb 2026', 'Open'],
    ['Brightwater lease review', 'Brightwater Ltd', 'Jul 2025', 'Closed'],
    ['Cole estate', 'Cole Family Trust', 'Sep 2026', 'Open']
  ]),
  table('contract', 'Contract', 'document', 904, '1 hr ago', firm, ['Check each notice that arrives in the Mercer inbox against the lease and flag conflicts'], [
    c('title', 'text', true),
    c('matter', 'link', true, '', 'matter'),
    c('signed_on', 'date'),
    c('ends_on', 'date', false, 'Used for reminders'),
    c('file', 'file', true)
  ], [
    ['Mercer lease (signed)', 'Mercer v. Brightwater', 'Apr 2021', 'Apr 2031'],
    ['Retainer, Cole', 'Cole estate', 'Sep 2026', ''],
    ['Side letter, unit 4', 'Mercer v. Brightwater', 'May 2021', '']
  ]),
  table('filing', 'Filing', 'box', 1260, 'Today', firm, [], [
    c('title', 'text', true),
    c('matter', 'link', true, '', 'matter'),
    c('filed_on', 'date', true),
    c('court', 'text'),
    c('file', 'file', true)
  ], [
    ['Particulars of claim', 'Mercer v. Brightwater', 'Mar 2026', 'County Court'],
    ['Defence', 'Mercer v. Brightwater', 'Apr 2026', 'County Court'],
    ['Witness statement, J. Cole', 'Mercer v. Brightwater', 'Sep 2026', 'County Court']
  ]),
  table('invoice', 'Invoice', 'invoice', 2214, 'Fri', firmBilling, ['On the last Friday of the month, draft billing narratives from the month\'s time entries'], [
    c('number', 'text', true),
    c('matter', 'link', true, '', 'matter'),
    c('issued_on', 'date', true),
    c('amount', 'money', true),
    c('paid', 'yes')
  ], [
    ['HW-2026-1041', 'Mercer v. Brightwater', 'Oct 31', '$18,420.00'],
    ['HW-2026-1040', 'Cole estate', 'Oct 31', '$3,150.00'],
    ['HW-2026-0988', 'Mercer v. Brightwater', 'Sep 30', '$22,775.00']
  ]),
  table('hearing', 'Hearing', 'gavel', 87, 'Mon', firm, [], [
    c('title', 'text', true),
    c('matter', 'link', true, '', 'matter'),
    c('held_on', 'date', true),
    c('court', 'text', true),
    c('notes', 'long')
  ], [
    ['Case management', 'Mercer v. Brightwater', 'Nov 18', 'County Court'],
    ['Interim application', 'Mercer v. Brightwater', 'Jan 9', 'County Court'],
    ['Probate directions', 'Cole estate', 'Feb 2', 'High Court']
  ])
];

const fieldnote = [
  table('project', 'Project', 'folder', 64, 'Yesterday', studio, [], [
    c('name', 'text', true),
    c('client', 'link', true, '', 'client'),
    c('starts_on', 'date'),
    c('lead', 'text', true),
    c('fee', 'money')
  ], [
    ['Linden library identity', 'Linden Public Library', 'Oct 2026', 'Jun Park'],
    ['Harbor wayfinding', 'Port of Rotterdam', 'Jun 2026', 'Mika Sato'],
    ['Studio site', 'Fieldnote', 'Jan 2026', 'Lea Fontaine']
  ]),
  table('client', 'Client', 'briefcase', 41, 'Oct 8', studio, [], [
    c('name', 'text', true),
    c('contact', 'email', true),
    c('city', 'text'),
    c('since', 'date')
  ], [
    ['Linden Public Library', 'director@linden.org', 'Linden', '2026'],
    ['Port of Rotterdam', 'brand@port.nl', 'Rotterdam', '2025'],
    ['Fieldnote', 'hello@fieldnote.studio', 'Brooklyn', '2019']
  ]),
  table('asset', 'Asset', 'box', 3120, 'Yesterday', studio, ['Sort each image added to the Linden folder in Google Drive into the right moodboard'], [
    c('name', 'text', true),
    c('project', 'link', true, '', 'project'),
    c('kind', 'text', false, 'Photo, scan, type, sketch'),
    c('added_by', 'text'),
    c('file', 'file', true)
  ], [
    ['Enamel sign, Rotterdam', 'Linden library identity', 'Photo', 'Lea Fontaine'],
    ['Date-due card scan', 'Linden library identity', 'Scan', 'Sam Ortiz'],
    ['Pictogram sketches', 'Harbor wayfinding', 'Sketch', 'Mika Sato']
  ]),
  table('brief', 'Brief', 'letter', 58, 'Mon', studio, [], [
    c('title', 'text', true),
    c('project', 'link', true, '', 'project'),
    c('due_on', 'date'),
    c('owner', 'text'),
    c('body', 'long', true)
  ], [
    ['Identity brief', 'Linden library identity', 'Nov 20', 'Jun Park'],
    ['Signage brief v2', 'Harbor wayfinding', 'Sep 1', 'Mika Sato'],
    ['Site copy brief', 'Studio site', 'Dec 5', 'Lea Fontaine']
  ]),
  table('invoice', 'Invoice', 'invoice', 412, 'Oct 31', { owners: ['Design'], editors: [], viewers: [] }, [], [
    c('number', 'text', true),
    c('project', 'link', true, '', 'project'),
    c('issued_on', 'date', true),
    c('amount', 'money', true),
    c('paid', 'yes')
  ], [
    ['FN-0412', 'Linden library identity', 'Oct 31', '$12,000.00'],
    ['FN-0408', 'Harbor wayfinding', 'Sep 30', '$8,400.00'],
    ['FN-0401', 'Harbor wayfinding', 'Aug 31', '$8,400.00']
  ])
];

const personal = [
  table('note', 'Note', 'note', 731, '3 hr ago', solo, [], [
    c('title', 'text', true),
    c('written_on', 'date', true),
    c('tags', 'text'),
    c('book', 'link', false, '', 'book'),
    c('body', 'long')
  ], [
    ['VHS digitizing plan', 'Sep 21', 'plans', ''],
    ['Quotes to keep', 'Sep 12', 'reading', 'The Overstory'],
    ['Desk setup', 'Sep 2', 'home', '']
  ]),
  table('payment', 'Payment', 'coin', 1388, 'Today', solo, [], [
    c('payee', 'text', true),
    c('paid_on', 'date', true),
    c('amount', 'money', true),
    c('subscription', 'link', false, '', 'subscription'),
    c('trip', 'link', false, '', 'trip')
  ], [
    ['Con Edison', 'Oct 28', '$84.12', ''],
    ['Spotify', 'Oct 25', '$11.99', 'Spotify'],
    ['Hotel Alfama', 'Oct 3', '$412.00', '']
  ]),
  table('subscription', 'Subscription', 'event', 14, 'Oct 25', solo, [], [
    c('name', 'text', true),
    c('renews_on', 'date', true),
    c('price', 'money', true),
    c('cadence', 'text', false, 'Monthly or yearly')
  ], [
    ['Spotify', 'Nov 25', '$11.99', 'Monthly'],
    ['iCloud', 'Nov 14', '$2.99', 'Monthly'],
    ['The Atlantic', 'Mar 2', '$79.99', 'Yearly']
  ]),
  table('trip', 'Trip', 'suitcase', 22, 'Oct 9', solo, [], [
    c('name', 'text', true),
    c('leaves_on', 'date', true),
    c('returns_on', 'date'),
    c('city', 'text', true),
    c('budget', 'money')
  ], [
    ['Lisbon', 'Sep 28', 'Oct 6', 'Lisbon'],
    ['Montreal', 'Jun 12', 'Jun 16', 'Montreal'],
    ['Mexico City', 'Feb 3', 'Feb 11', 'Mexico City']
  ]),
  table('book', 'Book', 'book', 186, 'Sep 12', solo, [], [
    c('title', 'text', true),
    c('author', 'text', true),
    c('finished_on', 'date'),
    c('rating', 'number', false, 'One to five')
  ], [
    ['The Overstory', 'Richard Powers', 'Sep 10', '5'],
    ['Piranesi', 'Susanna Clarke', 'Aug 2', '4'],
    ['Pachinko', 'Min Jin Lee', 'Jun 21', '5']
  ])
];

const schemas = { okafor, hale, fieldnote, personal };

export function schemaFor(id) {
  return structuredClone(schemas[id] ?? []);
}
