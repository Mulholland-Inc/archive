const INK = [11, 13, 18];
const C = Math.sqrt(3) / 2;
const RX = Math.sqrt(1.5);
const RY = Math.SQRT1_2;

const MATERIALS = {
  white: '#F4F5F7',
  sky: '#DCEBFC',
  lilac: '#66AEF5',
  lime: '#0066C5',
  blue: '#1A40A1',
  steel: '#C3C7CE',
  paper: '#E6E8EA'
};

const PAIRS = [
  ['white', 'lime'],
  ['sky', 'blue'],
  ['lilac', 'white'],
  ['lime', 'sky'],
  ['blue', 'lilac'],
  ['steel', 'lime'],
  ['white', 'blue'],
  ['paper', 'lilac'],
  ['sky', 'lime'],
  ['lilac', 'blue']
];

export function hash(str) {
  let h1 = 0xdeadbeef;
  let h2 = 0x41c6ce57;
  for (let i = 0; i < str.length; i++) {
    const c = str.charCodeAt(i);
    h1 = Math.imul(h1 ^ c, 2654435761);
    h2 = Math.imul(h2 ^ c, 1597334677);
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507) ^ Math.imul(h2 ^ (h2 >>> 13), 3266489909);
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507) ^ Math.imul(h1 ^ (h1 >>> 13), 3266489909);
  return 4294967296 * (2097151 & h2) + (h1 >>> 0);
}

function prng(seed) {
  let a = seed >>> 0 || 1;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function shade(hex, k) {
  const n = parseInt(hex.slice(1), 16);
  const rgb = [n >> 16, (n >> 8) & 255, n & 255].map((v, i) => Math.round(v + (INK[i] - v) * k));
  return `#${rgb.map((v) => v.toString(16).padStart(2, '0')).join('')}`;
}

function material(name) {
  const base = MATERIALS[name];
  const dark = name === 'blue' || name === 'lime';
  return { top: base, left: shade(base, dark ? 0.12 : 0.07), right: shade(base, dark ? 0.26 : 0.17) };
}

const P = (x, y, z) => [(x - y) * C, (x + y) / 2 - z];

function solid() {
  return { faces: [], edges: [], outline: [], box: [Infinity, Infinity, -Infinity, -Infinity] };
}

function grow(s, x, y) {
  s.box[0] = Math.min(s.box[0], x);
  s.box[1] = Math.min(s.box[1], y);
  s.box[2] = Math.max(s.box[2], x);
  s.box[3] = Math.max(s.box[3], y);
}

function hull(pts) {
  const p = [...pts].sort((a, b) => a[0] - b[0] || a[1] - b[1]);
  const cross = (o, a, b) => (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0]);
  const lo = [];
  const hi = [];
  for (const q of p) {
    while (lo.length > 1 && cross(lo[lo.length - 2], lo[lo.length - 1], q) <= 0) lo.pop();
    lo.push(q);
  }
  for (const q of p.reverse()) {
    while (hi.length > 1 && cross(hi[hi.length - 2], hi[hi.length - 1], q) <= 0) hi.pop();
    hi.push(q);
  }
  return lo.slice(0, -1).concat(hi.slice(0, -1));
}

const poly = (pts) => [['M', ...pts[0]], ...pts.slice(1).map((q) => ['L', ...q]), ['Z']];

function sideShade(m, n) {
  const len = Math.hypot(n[0], n[1], n[2]) || 1;
  const u = (n[0] - n[1]) / len / 2 + 0.5;
  return u < 0.5 ? m.left : m.right;
}

function faceted(s, m, base, apex, top) {
  const pts = [];
  const all = [...base, ...(top ?? [apex])];
  for (const q of all) pts.push(P(...q));
  for (let i = 0; i < base.length; i++) {
    const a = base[i];
    const b = base[(i + 1) % base.length];
    const quad = top ? [a, b, top[(i + 1) % base.length], top[i]] : [a, b, apex];
    const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]];
    const v = [quad[2][0] - a[0], quad[2][1] - a[1], quad[2][2] - a[2]];
    const n = [u[1] * v[2] - u[2] * v[1], u[2] * v[0] - u[0] * v[2], u[0] * v[1] - u[1] * v[0]];
    if (n[0] + n[1] + n[2] <= 1e-6) continue;
    const d = poly(quad.map((q) => P(...q)));
    s.faces.push({ d, fill: sideShade(m, n) });
    s.edges.push(d);
  }
  if (top) {
    const d = poly(top.map((q) => P(...q)));
    s.faces.push({ d, fill: m.top });
    s.edges.push(d);
  }
  const h = hull(pts);
  for (const q of h) grow(s, ...q);
  s.outline.push(poly(h));
}

function ring4(x, y, z, w, d, yaw) {
  const c = Math.cos(yaw);
  const n = Math.sin(yaw);
  return [[-w, -d], [w, -d], [w, d], [-w, d]].map(([a, b]) => [x + (a * c - b * n) / 2, y + (a * n + b * c) / 2, z]);
}

function box(m, x, y, z, w, d, h, yaw = 0) {
  const s = solid();
  const base = ring4(x, y, z, w, d, yaw);
  faceted(s, m, base, null, base.map(([a, b]) => [a, b, z + h]));
  return s;
}

function pyramid(m, x, y, z, w, h, yaw = 0) {
  const s = solid();
  faceted(s, m, ring4(x, y, z, w, w, yaw), [x, y, z + h]);
  return s;
}

function ellipse(cx, cy, rx, ry) {
  return [['M', cx - rx, cy], ['A', rx, ry, 0, 1, cx + rx, cy], ['A', rx, ry, 0, 1, cx - rx, cy], ['Z']];
}

function band(T, B, rx, ry) {
  const a = rx * 0.32;
  const q = ry * Math.sqrt(1 - 0.32 * 0.32);
  return [['M', T[0] + a, T[1] + q], ['L', B[0] + a, B[1] + q], ['A', rx, ry, 0, 0, B[0] + rx, B[1]], ['L', T[0] + rx, T[1]], ['A', rx, ry, 0, 1, T[0] + a, T[1] + q], ['Z']];
}

function cylinder(m, x, y, z, r, h, { cap = true } = {}) {
  const s = solid();
  const T = P(x, y, z + h);
  const B = P(x, y, z);
  const rx = r * RX;
  const ry = r * RY;
  const top = cap ? [['A', rx, ry, 0, 1, T[0] + rx, T[1]]] : [['A', rx, ry, 0, 0, T[0] + rx, T[1]]];
  const body = [['M', T[0] - rx, T[1]], ...(cap ? [['L', B[0] - rx, B[1]], ['A', rx, ry, 0, 0, B[0] + rx, B[1]], ['L', T[0] + rx, T[1]], ['Z']] : [...top, ['L', B[0] + rx, B[1]], ['A', rx, ry, 0, 1, B[0] - rx, B[1]], ['Z']])];
  s.faces.push({ d: body, fill: m.left });
  s.faces.push({ d: band(T, B, rx, ry), fill: m.right });
  if (cap) {
    const d = ellipse(T[0], T[1], rx, ry);
    s.faces.push({ d, fill: m.top });
    s.edges.push(d);
  }
  s.outline.push([['M', T[0] - rx, T[1]], ...top, ['L', B[0] + rx, B[1]], ['A', rx, ry, 0, 1, B[0] - rx, B[1]], ['Z']]);
  grow(s, T[0] - rx, T[1] - ry);
  grow(s, B[0] + rx, B[1] + ry);
  return s;
}

function cone(m, x, y, z, r, h) {
  const s = solid();
  const A = P(x, y, z + h);
  const B = P(x, y, z);
  const rx = r * RX;
  const ry = r * RY;
  const H = B[1] - A[1];
  const ty = -(ry * ry) / H;
  const tx = rx * Math.sqrt(Math.max(0, 1 - (ry * ry) / (H * H)));
  const a = rx * 0.3;
  const q = ry * Math.sqrt(1 - 0.09);
  s.faces.push({ d: [['M', ...A], ['L', B[0] - tx, B[1] + ty], ['A3', rx, ry, B[0] + tx, B[1] + ty], ['Z']], fill: m.left });
  s.faces.push({ d: [['M', ...A], ['L', B[0] + a, B[1] + q], ['A', rx, ry, 0, 0, B[0] + tx, B[1] + ty], ['Z']], fill: m.right });
  s.outline.push(s.faces[0].d);
  grow(s, B[0] - rx, A[1]);
  grow(s, B[0] + rx, B[1] + ry);
  return s;
}

function sphere(m, x, y, z, r) {
  const s = solid();
  const [cx, cy] = P(x, y, z);
  const R = r * RX;
  const at = (deg, rad = R) => [cx + rad * Math.cos((deg * Math.PI) / 180), cy + rad * Math.sin((deg * Math.PI) / 180)];
  const circle = ellipse(cx, cy, R, R);
  s.faces.push({ d: circle, fill: m.top });
  s.faces.push({ d: [['M', ...at(-30)], ['A', R, R, 0, 1, ...at(150)], ['A', R * 1.25, R * 1.25, 0, 0, ...at(-30)], ['Z']], fill: m.left });
  s.outline.push(circle);
  grow(s, cx - R, cy - R);
  grow(s, cx + R, cy + R);
  return s;
}

function dome(m, x, y, z, r) {
  const s = solid();
  const [cx, cy] = P(x, y, z);
  const rx = r * RX;
  const ry = r * RY;
  const d = [['M', cx - rx, cy], ['A', rx, rx, 0, 1, cx + rx, cy], ['A', rx, ry, 0, 1, cx - rx, cy], ['Z']];
  s.faces.push({ d, fill: m.top });
  s.faces.push({ d: [['M', cx + rx * 0.32, cy + ry * 0.947], ['A', rx, ry, 0, 0, cx + rx, cy], ['A', rx, rx, 0, 0, cx + rx * 0.5, cy - rx * 0.866], ['A', rx * 1.3, rx * 1.3, 0, 0, cx + rx * 0.32, cy + ry * 0.947], ['Z']], fill: m.left });
  s.outline.push(d);
  grow(s, cx - rx, cy - rx);
  grow(s, cx + rx, cy + ry);
  return s;
}

function washer(m, x, y, z, r, ri, t) {
  const s = cylinder(m, x, y, z, r, t, { cap: false });
  const T = P(x, y, z + t);
  const rx = r * RX;
  const ry = r * RY;
  const ix = ri * RX;
  const iy = ri * RY;
  const outer = ellipse(T[0], T[1], rx, ry);
  const inner = ellipse(T[0], T[1], ix, iy);
  s.faces.push({ d: [...outer, ...inner], fill: m.top, evenodd: true });
  s.faces.push({ d: inner, fill: m.right });
  const half = Math.min(t, iy * 1.9) / 2;
  const xi = ix * Math.sqrt(1 - (half / iy) ** 2);
  const lens = [['M', T[0] - xi, T[1] + half], ['A', ix, iy, 0, 0, T[0] + xi, T[1] + half], ['A', ix, iy, 0, 0, T[0] - xi, T[1] + half], ['Z']];
  s.faces.push({ d: lens, hole: true });
  s.edges.push(lens);
  s.outline.push(outer, inner);
  return s;
}

const between = (rand, a, b) => a + (b - a) * rand();
const pick = (rand, list) => list[Math.floor(rand() * list.length)];

const BUILDERS = [
  (r, a, b) => [box(a, 0, 0, 0, between(r, 0.9, 1.1), between(r, 0.9, 1.1), between(r, 0.85, 1.1), between(r, -0.35, 0.35))],
  (r, a, b) => {
    const w = between(r, 1, 1.25);
    return [box(a, 0, 0, 0, w, between(r, 0.5, 0.8), between(r, 1.2, 1.6), between(r, -0.3, 0.3))];
  },
  (r, a, b) => [cylinder(a, 0, 0, 0, between(r, 0.45, 0.6), between(r, 0.8, 1.4))],
  (r, a, b) => [cone(a, 0, 0, 0, between(r, 0.55, 0.7), between(r, 1.2, 1.6))],
  (r, a, b) => [sphere(a, 0, 0, 0, 0.6)],
  (r, a, b) => [pyramid(a, 0, 0, 0, between(r, 1.1, 1.3), between(r, 0.9, 1.3), between(r, -0.4, 0.4))],
  (r, a, b) => [washer(a, 0, 0, 0, 0.7, between(r, 0.32, 0.4), between(r, 0.22, 0.3))],
  (r, a, b) => {
    const h1 = between(r, 0.4, 0.55);
    const h2 = between(r, 0.35, 0.5);
    return [box(a, 0, 0, 0, 1.2, 1.2, h1, between(r, -0.2, 0.2)), box(b, 0, 0, h1, 0.8, 0.8, h2, between(r, -0.5, 0.5)), ...(r() < 0.5 ? [box(a, 0, 0, h1 + h2, 0.45, 0.45, 0.3, between(r, -0.5, 0.5))] : [])];
  },
  (r, a, b) => {
    const rad = between(r, 0.38, 0.48);
    const h = between(r, 0.6, 1);
    return [cylinder(a, 0, 0, 0, rad, h, { cap: false }), dome(a, 0, 0, h, rad)];
  },
  (r, a, b) => {
    const w = between(r, 0.9, 1.1);
    const rad = between(r, 0.3, 0.4);
    return [box(a, 0, 0, 0, w, w, between(r, 0.7, 0.9), between(r, -0.3, 0.3)), sphere(b, 0, 0, 0.8 + rad, rad)];
  },
  (r, a, b) => {
    const w = between(r, 0.9, 1.05);
    const h = between(r, 0.7, 0.95);
    const yaw = between(r, -0.3, 0.3);
    return [box(a, 0, 0, 0, w, w, h, yaw), pyramid(b, 0, 0, h, w * 1.08, between(r, 0.5, 0.75), yaw)];
  },
  (r, a, b) => {
    const rad = between(r, 0.4, 0.5);
    const h = between(r, 0.7, 1.1);
    return [cylinder(a, 0, 0, 0, rad, h), cone(b, 0, 0, h, rad * between(r, 1, 1.2), between(r, 0.6, 0.9))];
  }
];

function fmt(n) {
  return Math.round(n * 10) / 10;
}

function serialize(cmds, k, tx, ty) {
  let out = '';
  for (const c of cmds) {
    const [op] = c;
    if (op === 'Z') out += 'Z';
    else if (op === 'M' || op === 'L') out += `${op}${fmt(c[1] * k + tx)} ${fmt(c[2] * k + ty)}`;
    else if (op === 'A') out += `A${fmt(c[1] * k)} ${fmt(c[2] * k)} 0 0 ${c[c.length - 3]} ${fmt(c[c.length - 2] * k + tx)} ${fmt(c[c.length - 1] * k + ty)}`;
    else if (op === 'A3') out += `A${fmt(c[1] * k)} ${fmt(c[2] * k)} 0 1 0 ${fmt(c[3] * k + tx)} ${fmt(c[4] * k + ty)}`;
  }
  return out;
}

const cache = new Map();

export function avatarShape(seed, { small = false } = {}) {
  const key = `${small ? 's' : 'l'}:${seed}`;
  const hit = cache.get(key);
  if (hit) return hit;
  const rand = prng(hash(String(seed)));
  const tile = 1 + Math.floor(rand() * 6);
  const [ma, mb] = pick(rand, PAIRS);
  const flip = rand() < 0.5;
  const solids = pick(rand, BUILDERS)(rand, material(flip ? mb : ma), material(flip ? ma : mb));
  const box4 = solids.reduce((b, s) => [Math.min(b[0], s.box[0]), Math.min(b[1], s.box[1]), Math.max(b[2], s.box[2]), Math.max(b[3], s.box[3])], [Infinity, Infinity, -Infinity, -Infinity]);
  const room = small ? 50 : 44;
  const k = room / Math.max(box4[2] - box4[0], box4[3] - box4[1]);
  const tx = 32 - ((box4[0] + box4[2]) / 2) * k;
  const ty = 32 - ((box4[1] + box4[3]) / 2) * k;
  const out = {
    tile,
    solids: solids.map((s) => ({
      faces: s.faces.map((f) => ({ d: serialize(f.d, k, tx, ty), fill: f.fill, hole: !!f.hole, evenodd: !!f.evenodd })),
      edges: small ? [] : s.edges.map((d) => serialize(d, k, tx, ty)),
      outline: s.outline.map((d) => serialize(d, k, tx, ty))
    }))
  };
  cache.set(key, out);
  return out;
}
