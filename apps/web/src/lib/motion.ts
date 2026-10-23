function token(name: string) {
  if (typeof document === 'undefined') return '';
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

export function ms(name: 'instant' | 'fast' | 'normal' | 'slow' | 'slower' | 'hover-delay') {
  const value = token(`--duration-${name}`);
  const n = parseFloat(value);
  if (Number.isNaN(n)) return 0;
  return value.endsWith('ms') ? n : n * 1000;
}

export function stagger(index: number) {
  return index * ms('instant') * 0.35;
}

function solve(x1: number, y1: number, x2: number, y2: number) {
  const a = (a1: number, a2: number) => 1 - 3 * a2 + 3 * a1;
  const b = (a1: number, a2: number) => 3 * a2 - 6 * a1;
  const c = (a1: number) => 3 * a1;
  const at = (t: number, a1: number, a2: number) => ((a(a1, a2) * t + b(a1, a2)) * t + c(a1)) * t;
  const slope = (t: number, a1: number, a2: number) => 3 * a(a1, a2) * t * t + 2 * b(a1, a2) * t + c(a1);
  return (x: number) => {
    if (x <= 0) return 0;
    if (x >= 1) return 1;
    let t = x;
    for (let i = 0; i < 8; i++) {
      const s = slope(t, x1, x2);
      if (Math.abs(s) < 1e-6) break;
      t -= (at(t, x1, x2) - x) / s;
    }
    return at(t, y1, y2);
  };
}

const cache = new Map<string, (t: number) => number>();

export function ease(name: 'out' | 'in-out' | 'in' | 'spring') {
  const value = token(`--ease-${name}`);
  if (cache.has(value)) return cache.get(value)!;
  const m = value.match(/cubic-bezier\(([^)]+)\)/);
  const fn = m ? solve(...(m[1].split(',').map(Number) as [number, number, number, number])) : (t: number) => t;
  cache.set(value, fn);
  return fn;
}

export function pop(_node: Element, { y = -0.25, scale = 0.96 }: { y?: number; scale?: number } = {}) {
  return {
    duration: ms('normal'),
    easing: ease('out'),
    css: (t: number) =>
      `opacity:${t};transform:translateY(${(1 - t) * y}rem) scale(${scale + (1 - scale) * t});filter:blur(${(1 - t) * 0.125}rem)`
  };
}

export function rise(_node: Element, { delay = 0, distance = 0.5 }: { delay?: number; distance?: number } = {}) {
  return {
    delay,
    duration: ms('slow'),
    easing: ease('out'),
    css: (t: number) => `opacity:${t};transform:translateY(${(1 - t) * distance}rem)`
  };
}

export function fade(_node: Element, { delay = 0 }: { delay?: number } = {}) {
  return { delay, duration: ms('fast'), easing: ease('out'), css: (t: number) => `opacity:${t}` };
}

export function collapse(node: Element) {
  const h = (node as HTMLElement).offsetHeight;
  return {
    duration: ms('normal'),
    easing: ease('out'),
    css: (t: number) => `opacity:${t};height:${t * h}px;overflow:hidden`
  };
}
