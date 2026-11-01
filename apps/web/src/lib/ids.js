let n = 0;
export function uid(prefix = 'a') {
  n += 1;
  return `${prefix}-${n.toString(36)}`;
}
