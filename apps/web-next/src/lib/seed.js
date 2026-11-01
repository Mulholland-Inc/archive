import { chats, conversations, library, members, groups, me } from './data.js';
import { workflowSeed } from './workflows.js';
import { connectorsFor } from './connectors.js';

const pick = (list, ids) => list.filter((x) => ids.includes(x.id));

const haleChats = ['mercer-timeline', 'c8', 'c9', 'c14', 'c17', 'c21', 'c26', 'c28', 'c31', 'c33'];
const fieldChats = ['linden-brief', 'c12', 'c15', 'c20', 'c24', 'c30', 'c32', 'c34'];
const personalChats = ['exif-rename', 'c19', 'c29'];

const haleLibrary = [
  { id: 'h1', name: 'Mercer lease (signed)', kind: 'pdf', ext: 'PDF', size: '2.1 MB', modified: '1 hr ago', owner: 'Ruth Hale', folder: 'Mercer v. Brightwater', tone: 3 },
  { id: 'h2', name: 'Break notice LM-0412', kind: 'pdf', ext: 'PDF', size: '220 KB', modified: '1 hr ago', owner: 'Priya Raman', folder: 'Mercer v. Brightwater', tone: 1 },
  { id: 'h3', name: 'Chronology draft', kind: 'document', ext: 'DOC', size: '44 KB', modified: 'Today', owner: 'Nora Okafor', folder: 'Mercer v. Brightwater', tone: 2 },
  { id: 'h4', name: 'Client call, 14 Oct', kind: 'recording', ext: 'M4A', size: '28 MB', modified: 'Mon', owner: 'Daniel Whitcombe', folder: 'Calls', duration: '32:10', tone: 4 },
  { id: 'h5', name: 'Billing October', kind: 'spreadsheet', ext: 'XLSX', size: '96 KB', modified: 'Fri', owner: 'Tom Ashby', folder: 'Billing', tone: 2 },
  { id: 'h6', name: 'Retainer template', kind: 'document', ext: 'DOC', size: '31 KB', modified: 'Sep 24', owner: 'Ruth Hale', folder: 'Templates', tone: 1 },
  { id: 'h7', name: 'Site photos, Mercer unit 4', kind: 'image', ext: 'JPG', size: '3.4 MB', modified: 'Sep 20', owner: 'Grace Liu', folder: 'Mercer v. Brightwater', tone: 3 },
  { id: 'h8', name: 'Witness statement, J. Cole', kind: 'pdf', ext: 'PDF', size: '510 KB', modified: 'Sep 3', owner: 'Priya Raman', folder: 'Brightwater', tone: 4 }
];

const fieldLibrary = [
  { id: 's1', name: 'Linden kickoff notes', kind: 'pdf', ext: 'PDF', size: '480 KB', modified: 'Yesterday', owner: 'Jun Park', folder: 'Linden', tone: 1 },
  { id: 's2', name: 'Enamel sign, Rotterdam', kind: 'image', ext: 'JPG', size: '4.2 MB', modified: 'Yesterday', owner: 'Lea Fontaine', folder: 'Linden moodboard', tone: 2 },
  { id: 's3', name: 'Date-due card scan', kind: 'image', ext: 'PNG', size: '1.8 MB', modified: 'Mon', owner: 'Sam Ortiz', folder: 'Linden moodboard', tone: 4 },
  { id: 's4', name: 'Type scale options', kind: 'document', ext: 'DOC', size: '22 KB', modified: 'Fri', owner: 'Nora Okafor', folder: 'Linden', tone: 3 },
  { id: 's5', name: 'Rate card 2027', kind: 'spreadsheet', ext: 'XLSX', size: '58 KB', modified: 'Oct 8', owner: 'Jun Park', folder: 'Studio', tone: 2 },
  { id: 's6', name: 'Workshop interview, librarian', kind: 'recording', ext: 'M4A', size: '36 MB', modified: 'Oct 6', owner: 'Lea Fontaine', folder: 'Linden research', duration: '41:02', tone: 1 },
  { id: 's7', name: 'Wayfinding concepts v2', kind: 'pdf', ext: 'PDF', size: '9.6 MB', modified: 'Sep 17', owner: 'Mika Sato', folder: 'Signage', tone: 3 }
];

const personalLibrary = [
  { id: 'q1', name: 'rename_scans.py', kind: 'document', ext: 'PY', size: '2 KB', modified: '3 hr ago', owner: me.name, folder: 'Scripts', tone: 2 },
  { id: 'q2', name: 'VHS digitizing plan', kind: 'document', ext: 'DOC', size: '14 KB', modified: 'Sep 21', owner: me.name, folder: 'Plans', tone: 1 },
  { id: 'q3', name: 'Desk, morning light', kind: 'image', ext: 'JPG', size: '2.6 MB', modified: 'Sep 2', owner: me.name, folder: 'Photos', tone: 3 }
];

const haleMembers = [
  { id: 'hm1', name: 'Ruth Hale', email: 'ruth@halewhitcombe.law', role: 'owner', groups: ['Partners'], active: 'Now', status: 'active', tone: 3 },
  { id: 'hm2', name: 'Daniel Whitcombe', email: 'daniel@halewhitcombe.law', role: 'owner', groups: ['Partners'], active: '1 hr ago', status: 'active', tone: 1 },
  { id: 'hm3', name: 'Priya Raman', email: 'priya@halewhitcombe.law', role: 'editor', groups: ['Litigation'], active: 'Today', status: 'active', tone: 5 },
  { id: 'hm4', name: 'Tom Ashby', email: 'tom@halewhitcombe.law', role: 'editor', groups: ['Litigation'], active: 'Fri', status: 'active', tone: 2 },
  { id: 'hm5', name: me.name, email: 'nora@halewhitcombe.law', role: 'editor', groups: ['Litigation'], active: 'Now', status: 'active', tone: me.tone },
  { id: 'hm6', name: 'Grace Liu', email: 'grace@halewhitcombe.law', role: 'viewer', groups: [], active: 'Sep 20', status: 'active', tone: 4 },
  { id: 'hm7', name: 'Owen Price', email: 'owen.price@mail.com', role: 'viewer', groups: [], active: '', status: 'invited', tone: 6 }
];

const fieldMembers = [
  { id: 'fm1', name: 'Jun Park', email: 'jun@fieldnote.studio', role: 'owner', groups: ['Design'], active: 'Now', status: 'active', tone: 2 },
  { id: 'fm2', name: 'Lea Fontaine', email: 'lea@fieldnote.studio', role: 'editor', groups: ['Design'], active: 'Today', status: 'active', tone: 4 },
  { id: 'fm3', name: 'Sam Ortiz', email: 'sam@fieldnote.studio', role: 'editor', groups: ['Research'], active: 'Mon', status: 'active', tone: 1 },
  { id: 'fm4', name: me.name, email: 'nora@fieldnote.studio', role: 'editor', groups: ['Research'], active: 'Now', status: 'active', tone: me.tone },
  { id: 'fm5', name: 'Mika Sato', email: 'mika@fieldnote.studio', role: 'viewer', groups: [], active: 'Sep 17', status: 'active', tone: 5 }
];

function settings() {
  return { defaultRole: 'viewer', invites: true, sharing: false, region: 'us-central1', retention: 'forever' };
}

function vault(base) {
  return {
    settings: settings(),
    chats: [],
    conversations: {},
    library: [],
    folders: [],
    workflows: [],
    members: [{ id: `m-${base.id}`, name: me.name, email: me.email, role: 'owner', groups: [], active: 'Now', status: 'active', tone: me.tone }],
    groups: [],
    ...base,
    connectors: connectorsFor(base.id)
  };
}

function withConversations(list) {
  const out = {};
  for (const c of list) if (conversations[c.id]) out[c.id] = structuredClone(conversations[c.id]);
  return out;
}

function folderNames(list) {
  return [...new Set(list.map((f) => f.folder))];
}

export function seed() {
  const okaforIds = chats.map((c) => c.id).filter((id) => ![...haleChats, ...fieldChats, ...personalChats].includes(id));
  const okaforChats = pick(chats, okaforIds).map((c) => ({ ...c }));
  const okaforLibrary = library.map((f) => ({ ...f }));
  const hc = pick(chats, haleChats).map((c) => ({ ...c }));
  const fc = pick(chats, fieldChats).map((c) => ({ ...c }));
  const pc = pick(chats, personalChats).map((c) => ({ ...c }));

  return [
    vault({
      id: 'okafor',
      name: 'Okafor Family Archive',
      kind: 'Family',
      tone: 1,
      look: { type: 'family', icon: 'cottage', tint: 0 },
      chats: okaforChats,
      conversations: withConversations(okaforChats),
      library: okaforLibrary,
      folders: folderNames(okaforLibrary),
      workflows: structuredClone(workflowSeed.okafor),
      members: members.map((m) => ({ ...m, groups: [...m.groups] })),
      groups: groups.map((g) => ({ ...g }))
    }),
    vault({
      id: 'hale',
      name: 'Hale & Whitcombe LLP',
      kind: 'Law firm',
      tone: 3,
      look: { type: 'business', icon: 'capitol', tint: 3 },
      chats: hc,
      conversations: withConversations(hc),
      library: haleLibrary.map((f) => ({ ...f })),
      folders: folderNames(haleLibrary),
      workflows: structuredClone(workflowSeed.hale),
      members: haleMembers.map((m) => ({ ...m, groups: [...m.groups] })),
      groups: [
        { id: 'hg1', name: 'Partners' },
        { id: 'hg2', name: 'Litigation' }
      ]
    }),
    vault({
      id: 'fieldnote',
      name: 'Fieldnote Studio',
      kind: 'Studio',
      tone: 2,
      look: { type: 'business', icon: 'warehouse', tint: 2 },
      chats: fc,
      conversations: withConversations(fc),
      library: fieldLibrary.map((f) => ({ ...f })),
      folders: folderNames(fieldLibrary),
      workflows: structuredClone(workflowSeed.fieldnote),
      members: fieldMembers.map((m) => ({ ...m, groups: [...m.groups] })),
      groups: [
        { id: 'fg1', name: 'Design' },
        { id: 'fg2', name: 'Research' }
      ]
    }),
    vault({
      id: 'personal',
      name: 'Personal',
      kind: 'Just you',
      tone: 5,
      look: { type: 'personal', icon: 'chest', tint: 1 },
      chats: pc,
      conversations: withConversations(pc),
      library: personalLibrary.map((f) => ({ ...f })),
      folders: folderNames(personalLibrary)
    })
  ];
}

export function blankVault(id, name, look) {
  const kind = { family: 'Family', business: 'Business', personal: 'Just you' }[look.type];
  return vault({ id, name, kind, tone: (id.length % 6) + 1, look: { ...look } });
}
