import type { MenuItem } from './lib';

export type Person = { name: string; color: number };
export type Tag = { name: string; color: number };
export type Status = 'review' | 'progress' | 'todo' | 'backlog' | 'done' | 'unscoped' | 'canceled' | 'duplicate' | 'triage';
export type Confidence = 'urgent' | 'high' | 'medium' | 'low' | 'none';

export type Record = {
  id: string;
  title: string;
  status: Status;
  confidence: Confidence;
  person?: Person;
  labels: Tag[];
  parent?: string;
  pages?: number;
  conflicts?: string[];
  supersedes?: string[];
  source?: string;
};

export const people: Person[] = [
  { name: 'Mila Ahn', color: 1 },
  { name: 'Hana Mori', color: 2 },
  { name: 'Marco Rossi', color: 3 },
  { name: 'Arne Olsen', color: 4 }
];

const [MA, HM, MR, AO] = people;

export const tags = {
  invoice: { name: 'Invoice', color: 1 },
  week: { name: 'This week', color: 2 },
  flagged: { name: 'Flagged', color: 3 },
  gmail: { name: 'Gmail', color: 4 },
  finance: { name: 'Finance', color: 5 },
  receipt: { name: 'Receipt', color: 1 },
  drive: { name: 'Drive', color: 4 },
  contract: { name: 'Contract', color: 2 },
  ai: { name: 'Agent', color: 3 }
} satisfies { [k: string]: Tag };

export const statuses: { id: Status; label: string; glyph: string }[] = [
  { id: 'backlog', label: 'Inbox', glyph: 'status-backlog' },
  { id: 'todo', label: 'Queued', glyph: 'status-todo' },
  { id: 'progress', label: 'Extracting', glyph: 'status-progress' },
  { id: 'review', label: 'In review', glyph: 'status-review' },
  { id: 'done', label: 'Accepted', glyph: 'status-done' },
  { id: 'unscoped', label: 'Unmapped', glyph: 'status-unscoped' },
  { id: 'triage', label: 'Triage', glyph: 'status-triage' },
  { id: 'duplicate', label: 'Duplicate', glyph: 'status-duplicate' },
  { id: 'canceled', label: 'Dropped', glyph: 'status-canceled' }
];

export const confidences: { id: Confidence; label: string; glyph: string }[] = [
  { id: 'none', label: 'No score', glyph: 'priority-none' },
  { id: 'urgent', label: 'Conflict', glyph: 'priority-urgent' },
  { id: 'high', label: 'High', glyph: 'priority-high' },
  { id: 'medium', label: 'Medium', glyph: 'priority-medium' },
  { id: 'low', label: 'Low', glyph: 'priority-low' }
];

export const statusItems: MenuItem[] = statuses.map((s) => ({ id: s.id, label: s.label, glyph: s.glyph }));

export const filterProperties: MenuItem[] = [
  { id: 'status', label: 'Status', glyph: 'status-unscoped' },
  { id: 'person', label: 'Reviewer', icon: 'user-square' },
  { id: 'label', label: 'Label', icon: 'tag' },
  { id: 'confidence', label: 'Confidence', glyph: 'priority-medium' },
  { id: 'source', label: 'Source', glyph: 'status-parent', separator: true },
  { id: 'pages', label: 'Pages', glyph: 'status-subissue' },
  { id: 'conflicts', label: 'Conflicting records', glyph: 'status-blocked' },
  { id: 'supersedes', label: 'Superseding records', glyph: 'status-blocks' }
];

export const glyphOf = (s: Status) => statuses.find((x) => x.id === s)!.glyph;
export const confidenceOf = (c: Confidence) => confidences.find((x) => x.id === c)!;

export const records: Record[] = [
  { id: 'REC-196', title: 'Hetzner invoice for October, €48.90 billed to Mulholland Inc', status: 'review', confidence: 'high', person: MA, labels: [tags.invoice, tags.week, tags.gmail, tags.finance] },
  { id: 'REC-197', title: 'KPN mobile bill with roaming charges from the Lisbon trip', status: 'review', confidence: 'medium', person: AO, labels: [tags.invoice, tags.week], pages: 2 },
  { id: 'REC-198', title: 'Roaming detail page, 14 calls and 2.1 GB of data', status: 'progress', confidence: 'medium', labels: [tags.invoice], parent: 'REC-197' },
  { id: 'REC-199', title: 'Lisbon hotel folio matched to the business card', status: 'todo', confidence: 'low', labels: [tags.receipt], parent: 'REC-197' },
  { id: 'REC-201', title: 'Payslip October 2026 from Mulholland Inc', status: 'review', confidence: 'high', person: HM, labels: [tags.week, tags.finance], conflicts: ['REC-213'] },
  { id: 'REC-202', title: 'Albert Heijn receipt, 23 items split by category', status: 'progress', confidence: 'urgent', person: HM, labels: [tags.receipt, tags.week, tags.flagged, tags.drive, tags.finance] },
  { id: 'REC-203', title: 'Lease agreement for Prinsengracht 412 with renewal terms', status: 'backlog', confidence: 'medium', labels: [tags.contract, tags.finance] },
  { id: 'REC-204', title: 'Belastingdienst provisional assessment for 2027', status: 'backlog', confidence: 'high', person: AO, labels: [tags.week], conflicts: ['REC-213'] },
  { id: 'REC-205', title: 'Coolblue order for a Dyson V15, warranty until 2028', status: 'done', confidence: 'high', person: MR, labels: [tags.receipt, tags.gmail] },
  { id: 'REC-206', title: 'Centraal Beheer car insurance renewal and green card', status: 'todo', confidence: 'medium', person: MA, labels: [tags.contract, tags.week, tags.flagged, tags.drive, tags.finance] },
  { id: 'REC-207', title: 'Dentist invoice from Tandartspraktijk Noord', status: 'todo', confidence: 'high', person: HM, labels: [tags.week], pages: 2, conflicts: ['REC-213'] },
  { id: 'REC-208', title: 'Vattenfall energy contract with the new variable rate', status: 'progress', confidence: 'high', person: MA, labels: [tags.ai, tags.finance] },
  { id: 'REC-209', title: 'Passport scan for Mila, expires March 2031', status: 'progress', confidence: 'high', person: HM, labels: [tags.week, tags.ai, tags.drive] },
  { id: 'REC-210', title: 'Bol.com return label and refund confirmation', status: 'todo', confidence: 'urgent', person: MR, labels: [tags.week, tags.gmail], source: 'REC-205' },
  { id: 'REC-211', title: 'Ziggo internet bill matched to the bank transfer', status: 'review', confidence: 'low', person: AO, labels: [tags.finance], supersedes: ['REC-213'] },
  { id: 'REC-212', title: 'Gym membership cancellation letter from Basic-Fit', status: 'backlog', confidence: 'high', person: MR, labels: [tags.contract, tags.finance] },
  { id: 'REC-213', title: 'Annual income statement 2026 from the employer', status: 'review', confidence: 'medium', person: HM, labels: [tags.week, tags.finance], pages: 2, conflicts: ['REC-201'] },
  { id: 'REC-214', title: 'Schiphol parking receipt, 3 days at P3', status: 'done', confidence: 'high', person: AO, labels: [tags.receipt] },
  { id: 'REC-215', title: 'Gemeente Amsterdam parking permit renewal', status: 'backlog', confidence: 'none', labels: [tags.gmail, tags.finance] },
  { id: 'REC-216', title: 'Apple receipt for iCloud+ 2 TB, monthly', status: 'done', confidence: 'medium', person: MA, labels: [tags.receipt, tags.gmail] }
];
