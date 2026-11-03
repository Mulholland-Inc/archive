// How a layout writes what it shows: a column by its name as a heading, money and numbers with thousands
// separators, in full or compact (1.2M) where room is short, and a date column's value as a day.
export const label = (c) => c.name.replaceAll('_', ' ').replace(/^./, (x) => x.toUpperCase());

export const money = (amount, currency, compact = false) =>
  new Intl.NumberFormat('en-US', { style: 'currency', currency: currency || 'USD', maximumFractionDigits: compact ? 2 : 0, notation: compact ? 'compact' : 'standard' }).format(amount);

export const number = (n, compact = false) => new Intl.NumberFormat('en-US', { maximumFractionDigits: compact ? 1 : 2, notation: compact ? 'compact' : 'standard' }).format(n);

// figure is a cell as a layout shows it: money and numbers formatted, anything else as text has it.
export function figure(c, r, text, compact = false) {
  const v = r[c.name];
  if (c.type === 'money' && v?.amount != null) return money(v.amount, v.currency, compact);
  if (c.type === 'number' && typeof v === 'number') return number(v, compact);
  return text(c, r);
}

// day is a date or timestamp as the local day it falls on, or null.
export function day(v) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(typeof v === 'string' ? v : '');
  return m ? new Date(+m[1], m[2] - 1, +m[3]) : null;
}
