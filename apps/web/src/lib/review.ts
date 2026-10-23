import { kinds } from './sources';
import { humanize, titleField } from './spec';
import type { Ontology, Proposal } from './types';

export function proposalTitle(o: Ontology | null, p: Proposal) {
  const tf = titleField(o, p.type);
  const v = p.fields.find((f) => f.path === tf)?.value;
  if (typeof v === 'string' && v.trim()) return v;
  return p.label || humanize(p.type);
}

export function proposalSource(p: Proposal): { label: string; icon: string } {
  const kind = p.source.split(':')[0];
  if (kind === 'file') return { label: 'Upload', icon: 'upload' };
  const k = kinds.find((x) => x.id === kind);
  return k ? { label: k.label, icon: k.icon } : { label: humanize(kind || 'unknown'), icon: 'plug' };
}

export function tone(confidence: number): 'high' | 'mid' | 'low' {
  return confidence >= 0.9 ? 'high' : confidence >= 0.5 ? 'mid' : 'low';
}

export function pct(confidence: number) {
  return `${Math.round(confidence * 100)}%`;
}
