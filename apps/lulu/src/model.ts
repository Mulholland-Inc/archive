import { createContext, useContext } from 'react';
import type { Row } from './api';
import type { IconName, Tone } from './ui';

export type Op = 'is' | 'not' | 'contains' | 'above' | 'below' | 'filled';
export type Measure = 'longest' | 'shortest' | 'girth';
export type Test = { field: string; op: Op; values?: string[]; measure?: Measure };
export type When = Test[];
export type Check = { when?: When; op: Op; values?: string[]; message?: string };
export type Fill = { when?: When; value: string };
export type FieldType = 'text' | 'number' | 'select' | 'yesno' | 'country' | 'dimensions';
export type TField = {
  key: string; label: string; type: FieldType; section: string; owner: 'vendor' | 'lg'; required: boolean; requiredWhen?: When[]; must?: Check[]; fills?: Fill[];
  options?: string[]; parts?: string[]; help?: string; note?: string; pctBreakdown?: boolean; multi?: boolean;
};
export type Rule = { role: string; min: number; max?: number; when?: When };
export type Photo = { minWidth?: number; minHeight?: number; formats?: string[]; transparentPng?: boolean; fill?: number };
export type Spec = { fields: TField[]; images: Rule[]; photo: Photo | null };
export type Issue = { field: string; level: string; message?: string };

export const ANGLES: { key: string; label: string; hint: string; many?: boolean }[] = [
  { key: 'full', label: 'Full', hint: 'the whole product, straight on' },
  { key: 'three_quarter', label: '¾ angle', hint: 'the whole product turned three-quarters' },
  { key: 'side', label: 'Side', hint: 'the whole product from the side' },
  { key: 'back', label: 'Back', hint: 'the whole product from behind, or its second side' },
  { key: 'open', label: 'Open', hint: 'the product with its doors or drawers open' },
  { key: 'corner', label: 'Corner', hint: 'a close view of one corner' },
  { key: 'detail', label: 'Detail', hint: 'a close-up of part of the product', many: true },
  { key: 'swatch', label: 'Swatch', hint: 'the material or finish filling the frame' },
  { key: 'set', label: 'Set', hint: 'every piece of the set together, or stacked' },
  { key: 'placesetting', label: 'Place setting', hint: 'one place set with the pieces' },
  { key: 'scale', label: 'Scale', hint: 'the product styled with a prop that shows its size' },
  { key: 'lifestyle', label: 'Lifestyle', hint: 'the product in a room or a styled scene', many: true },
  { key: 'repeat', label: 'Repeat', hint: "the pattern's digital repeat file, or the full panel" },
  { key: 'video', label: 'Video', hint: 'a GIF or video of the product moving' }
];
export const angleRank = (role: string) => ANGLES.findIndex(a => a.key === role) + 1 || 99;

const BY_KEY: Record<string, IconName> = {
  base: 'home', accessories: 'gift', bed_bath: 'bed', curtains: 'curtain', furniture: 'sofa', lighting: 'lamp', pillows: 'pillow', rugs: 'rug',
  wall_art: 'image', wall_decor: 'star', wallpaper: 'brush', mirrors: 'mirror', made_to_order_frame: 'frame', made_to_order_fabric: 'swatch', ceiling_fans: 'fan'
};
const BY_NAME: Record<string, IconName> = {
  product: 'box', home: 'home', gift: 'gift', moon: 'moon', sun: 'sun', curtain: 'curtain', package: 'box', bulb: 'lamp', pillow: 'pillow', rug: 'rug', image: 'image',
  star: 'star', brush: 'brush', colour: 'swatch', flower: 'fan', heart: 'star', globe: 'globe', camera: 'camera', clock: 'clock', ruler: 'layout', theme: 'swatch', wrench: 'settings'
};
export const ICON_NAMES = Object.keys(BY_NAME);
export const categoryIcon = (key?: unknown, icon?: unknown): IconName => BY_NAME[String(icon ?? '')] ?? BY_KEY[String(key ?? '')] ?? 'box';

export const sentence = (s: string) => s.replace(/_/g, ' ').replace(/^./, c => c.toUpperCase());
export const plural = (n: number, one: string) => `${n} ${n === 1 ? one : `${one}s`}`;
const day = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', ...(d.getFullYear() === new Date().getFullYear() ? {} : { year: 'numeric' }) });
export function show(v: unknown): string {
  if (v == null || v === '') return '';
  if (typeof v === 'boolean') return v ? 'Yes' : 'No';
  if (Array.isArray(v)) return v.map(show).filter(Boolean).join(', ');
  if (typeof v === 'object') return '';
  const s = String(v);
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return day(new Date(`${s}T12:00:00`));
  if (/^\d{4}-\d{2}-\d{2}T/.test(s)) {
    const d = new Date(s);
    return new Date().toDateString() === d.toDateString() ? d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' }) : day(d);
  }
  return s;
}
const SAYS = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });
const STEPS: [Intl.RelativeTimeFormatUnit, number][] = [['minute', 60], ['hour', 24], ['day', 7], ['week', 4.35], ['month', 12], ['year', Infinity]];
export function ago(v: unknown): string {
  if (!v) return '';
  let n = (Date.now() - new Date(String(v)).getTime()) / 60000;
  if (Number.isNaN(n)) return '';
  if (n < 1) return 'just now';
  for (const [unit, size] of STEPS) {
    if (Math.abs(n) < size) return SAYS.format(-Math.round(n), unit);
    n /= size;
  }
  return '';
}

const TONES: Record<string, Tone> = { red: 'danger', amber: 'warning', green: 'success', blue: 'info', neutral: 'neutral' };
export const tone = (v: unknown): Tone => TONES[String(v ?? '')] ?? 'neutral';
export const STATUS: [string, string][] = [['review', 'Review'], ['overdue', 'Overdue'], ['in_progress', 'In progress'], ['sent', 'Sent'], ['draft', 'Draft'], ['complete', 'Approved'], ['cancelled', 'Cancelled']];
export const statusName = (r: Row) => STATUS.find(s => s[0] === r.status)?.[1] ?? sentence(String(r.status ?? ''));
export const statusRank = (r: Row) => STATUS.findIndex(x => x[0] === r.status);
export const isOpen = (r: Row) => r.stage !== 'complete' && r.stage !== 'cancelled';
export const needsYou = (r: Row) => r.stage === 'review' || r.stage === 'draft' || r.status === 'overdue';
export const idle = (r: Row) => r.stage === 'draft' || r.stage === 'cancelled';
export const late = (b: Row) => (b.stage === 'sent' || b.stage === 'in_progress') && !!b.due_at && new Date(b.due_at) < new Date();
export const stageTone = (b: Row): Tone => b.stage === 'complete' ? 'success' : b.stage === 'review' ? 'warning' : b.stage === 'cancelled' ? 'neutral' : late(b) ? 'danger' : b.stage === 'in_progress' ? 'info' : 'neutral';
export const standingOf = (b: Row) => b.stage === 'complete' ? 'Approved' : late(b) ? (b.opened_at ? 'Overdue' : 'Overdue, never opened') : sentence(String(b.stage));
export const whenOf = (b: Row) => b.stage === 'review' ? `Submitted ${show(b.submitted_at)}` : b.stage === 'complete' ? `Approved ${show(b.approved_at)}` : `${late(b) ? 'Overdue' : 'Due'} ${show(b.due_at)}`;
export const byPosition = (a: Row, b: Row) => Number(a.position) - Number(b.position);

export const SKU_KEY = 'manufacturer_sku';
export const NAME_KEY = 'vendor_product_name';
export const LISTED_BY = [SKU_KEY, NAME_KEY];
export const issuesOf = (r: Row) => (r.issues ?? []) as Issue[];
export const isPhotoIssue = (i: Issue) => i.field.startsWith('image:') || i.field.startsWith('photo:');
export const vendorFields = (spec: Spec) => spec.fields.filter(f => f.owner !== 'lg');
export const nameKeyOf = (spec: Spec) => vendorFields(spec).find(f => /name|title/i.test(f.key))?.key;
export function fixing(rows: Row[]) {
  let values = 0, products = 0;
  for (const r of rows) {
    const n = issuesOf(r).filter(i => i.level === 'error' && !isPhotoIssue(i)).length;
    values += n;
    if (n) products++;
  }
  return { values, words: values ? `${plural(values, 'value')} ${values === 1 ? 'needs' : 'need'} fixing across ${plural(products, 'product')}` : '' };
}
export const listedKey = (p: { sku: string; name: string }) => (p.sku.trim() ? `s:${p.sku.trim().toLowerCase()}` : p.name.trim() ? `n:${p.name.trim().toLowerCase()}` : '');
export const MAX_FILE = 25 << 20;
export const tooBig = (f: File) => (f.size > MAX_FILE ? `${f.name} is larger than 25 MB, the most one file can be. Save it without images, or split it, and drop it again.` : '');

export type Nav = { go: (path: string) => void; swap: (path: string) => void; brand: string };
export const NavCtx = createContext<Nav>({ go: () => {}, swap: () => {}, brand: '' });
export const useNav = () => useContext(NavCtx);
