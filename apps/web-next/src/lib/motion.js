import { cubicOut } from 'svelte/easing';

const cache = new Map();

function css(name) {
  if (typeof document === 'undefined') return '';
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

export function reduced() {
  return typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function duration(name = 'normal') {
  if (reduced()) return 0;
  const value = css(`--duration-${name}`);
  if (value.endsWith('ms')) return parseFloat(value);
  if (value.endsWith('s')) return parseFloat(value) * 1000;
  return 200;
}

export function length(name) {
  const value = css(name);
  const root = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
  if (value.endsWith('rem')) return parseFloat(value) * root;
  return parseFloat(value) || 0;
}

export function number(name) {
  return parseFloat(css(name)) || 0;
}

function bezier(x1, y1, x2, y2) {
  const cx = 3 * x1, bx = 3 * (x2 - x1) - cx, ax = 1 - cx - bx;
  const cy = 3 * y1, by = 3 * (y2 - y1) - cy, ay = 1 - cy - by;
  const sx = (t) => ((ax * t + bx) * t + cx) * t;
  const sy = (t) => ((ay * t + by) * t + cy) * t;
  const dx = (t) => (3 * ax * t + 2 * bx) * t + cx;
  return (x) => {
    if (x <= 0) return 0;
    if (x >= 1) return 1;
    let t = x;
    for (let i = 0; i < 6; i++) {
      const d = dx(t);
      if (Math.abs(d) < 1e-6) break;
      t -= (sx(t) - x) / d;
    }
    return sy(Math.min(1, Math.max(0, t)));
  };
}

export function ease(name = 'out') {
  const key = `--ease-${name}`;
  if (cache.has(key)) return cache.get(key);
  const match = css(key).match(/cubic-bezier\(([^)]+)\)/);
  if (!match) return cubicOut;
  const fn = bezier(...match[1].split(',').map(Number));
  cache.set(key, fn);
  return fn;
}

export function rise(node, { shift = '--shift-md', speed = 'normal', delay = 0 } = {}) {
  const y = length(shift);
  return {
    delay,
    duration: duration(speed),
    easing: ease('out'),
    css: (t, u) => `opacity:${t};transform:translateY(${u * y}px)`
  };
}

export function fade(node, { speed = 'fast', delay = 0 } = {}) {
  return { delay, duration: duration(speed), easing: ease('out'), css: (t) => `opacity:${t}` };
}

export function pop(node, { speed = 'normal' } = {}) {
  const s = number('--scale-enter') || 0.97;
  const y = length('--shift-sm');
  return {
    duration: duration(speed),
    easing: ease('out'),
    css: (t, u) => `opacity:${t};transform:translateY(${u * y}px) scale(${s + (1 - s) * t})`
  };
}

export function slide(node, { speed = 'normal', axis = 'y' } = {}) {
  const style = getComputedStyle(node);
  const prop = axis === 'y' ? 'height' : 'width';
  const size = parseFloat(style[prop]);
  return {
    duration: duration(speed),
    easing: ease('in-out'),
    css: (t) => `overflow:hidden;opacity:${Math.min(1, t * 2)};${prop}:${t * size}px`
  };
}

export function toastIn(node, { speed = 'slow' } = {}) {
  const y = length('--shift-lg');
  return {
    duration: duration(speed),
    easing: ease('emphasized'),
    css: (t, u) => `opacity:${t};transform:translateY(${u * y}px) scale(${0.96 + 0.04 * t})`
  };
}
