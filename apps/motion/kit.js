import gsap from './node_modules/gsap/index.js';
import { BRANDS } from '../kit/brands.js';
import { W, H, hooks, register, TR } from './engine.js';

export { W, H, art } from './engine.js';

export const C = { bg: '#f7f8fa', ink: '#0b0d12', blue: '#1a40a1', lime: '#0066c5', lilac: '#66aef5', sky: '#dcebfc', white: '#f4f5f7', steel: '#c3c7ce', paper: '#e6e8ea', graphite: '#2a2f38', mint: '#dde1e6' };
export const SERIF = '"EB Garamond"', SANS = '"Inter Tight"';

gsap.ticker.sleep();

const spring = (k, d) => x => (x >= 1 ? 1 : 1 - Math.exp(-d * x) * Math.cos(k * x));
export const EASE = {
  snap: 'expo.out', quart: 'power3.out', soft: x => x * x * x * (x * (6 * x - 15) + 10), glide: 'sine.inOut', settle: 'expo.inOut', pop: 'back.out(1.7)', in: 'power3.in', linear: 'none',
  spring: spring(14, 6), bounce: spring(22, 5),
};
const ease = name => EASE[name] ?? name;

export const clamp = x => Math.min(1, Math.max(0, x));
export const mix = (a, b, x) => a + (b - a) * x;
export const hash = n => { const s = Math.sin(n * 12.9898) * 43758.5453; return s - Math.floor(s); };
const rad = d => d * Math.PI / 180;

const marks = new Map();
let word = [];
await Promise.all([`500 100px ${SERIF}`, `500 100px ${SANS}`].map(f => document.fonts.load(f)));
if (document.fonts.check(`500 100px ${SERIF}`) === false) throw new Error('EB Garamond did not load');
word = [...(await (await fetch('../kit/Word.svelte')).text()).matchAll(/"(M [^"]+)"/g)].map(m => new Path2D(m[1]));

async function mark(name) {
  if (marks.has(name)) return marks.get(name);
  if (!BRANDS[name]) throw new Error(`No logo named ${name} in apps/kit/brands.js`);
  const [box, body] = BRANDS[name];
  const [, , bw, bh] = box.split(' ').map(Number);
  const k = 1024 / Math.max(bw, bh);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="${box}" width="${bw * k}" height="${bh * k}" color="${C.ink}">${body}</svg>`;
  const img = new Image();
  img.src = `data:image/svg+xml,${encodeURIComponent(svg)}`;
  await img.decode();
  marks.set(name, img);
  return img;
}

export const APPS = ['Salesforce', 'Notion', 'QuickBooks', 'Shopify', 'Slack', 'HubSpot', 'Stripe', 'Airtable', 'Asana', 'Jira', 'Gmail', 'Zendesk', 'Xero', 'Trello', 'Dropbox', 'Linear', 'Figma', 'GitHub', 'Intercom', 'Mailchimp', 'monday.com', 'Zoom', 'Snowflake', 'Google Drive', 'Microsoft Teams', 'Google Calendar', 'Google Sheets', 'ClickUp', 'Brex', 'Zapier', 'Databricks', 'Outlook', 'Okta'];
await Promise.all(APPS.map(mark));

export function logo(g, name, cx, cy, size, alpha = 1) {
  const img = marks.get(name);
  if (!img) throw new Error(`Logo ${name} is not in APPS in kit.js`);
  const k = size / Math.max(img.width, img.height);
  g.globalAlpha = alpha;
  g.drawImage(img, cx - img.width * k / 2, cy - img.height * k / 2, img.width * k, img.height * k);
  g.globalAlpha = 1;
}

export function wordmark(g, x, y, w, color) {
  const k = w / 1662;
  g.save();
  g.translate(x - 417 * k, y - 135 * k);
  g.scale(k, k);
  g.fillStyle = color;
  for (const p of word) g.fill(p);
  g.restore();
}

export function round(g, x, y, w, h, r) {
  g.beginPath();
  g.roundRect(x, y, w, h, r);
}

export function tile(g, name, cx, cy, s, flip = 1) {
  if (flip <= 0.01 || s <= 1) return;
  g.save();
  g.translate(cx, cy);
  g.scale(flip, 1);
  round(g, -s / 2, -s / 2, s, s, s * 0.14);
  g.fillStyle = '#ffffff';
  g.fill();
  g.lineWidth = Math.max(2.4, s * 0.012);
  g.strokeStyle = C.ink;
  g.stroke();
  logo(g, name, 0, 0, s * 0.6);
  g.restore();
}

export function font(g, size, family = SERIF) {
  g.font = `500 ${size}px ${family}`;
  g.letterSpacing = `${(family === SERIF ? -0.018 : -0.01) * size}px`;
  g.textBaseline = 'alphabetic';
  g.textAlign = 'left';
}

export function fill(g, color, x = 0, y = 0, w = W, h = H) {
  g.fillStyle = color;
  g.fillRect(x, y, w, h);
}

export function zoomed(g, s, cx, cy, draw) {
  g.save();
  g.translate(cx, cy);
  g.scale(s, s);
  g.translate(-cx, -cy);
  draw();
  g.restore();
}

export function curve(g, pts, p) {
  const n = (pts.length - 1) * clamp(p), whole = Math.floor(n);
  if (n <= 0) return;
  g.beginPath();
  g.moveTo(...pts[0]);
  for (let i = 1; i <= whole; i++) g.lineTo(...pts[i]);
  if (whole < pts.length - 1) g.lineTo(mix(pts[whole][0], pts[whole + 1][0], n - whole), mix(pts[whole][1], pts[whole + 1][1], n - whole));
  g.stroke();
}

export function basis(cam) {
  const yaw = rad(cam.yaw ?? 45), pitch = rad(cam.pitch ?? 35.2644);
  const right = [Math.cos(yaw), 0, -Math.sin(yaw)];
  const fwd = [-Math.cos(pitch) * Math.sin(yaw), -Math.sin(pitch), -Math.cos(pitch) * Math.cos(yaw)];
  const up = [right[1] * fwd[2] - right[2] * fwd[1], right[2] * fwd[0] - right[0] * fwd[2], right[0] * fwd[1] - right[1] * fwd[0]];
  return { right, up };
}

export function shifted(cam, dx, dy) {
  const { right, up } = basis(cam);
  const k = 1 / cam.zoom;
  return { ...cam, center: cam.center.map((v, i) => v - right[i] * dx * k + up[i] * dy * k) };
}

export function project(p, cam) {
  const { right, up } = basis(cam);
  const d = p.map((v, i) => v - cam.center[i]);
  const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  return [(cam.w ?? W) / 2 + dot(d, right) * cam.zoom + (cam.x ?? 0), (cam.h ?? H) / 2 - dot(d, up) * cam.zoom + (cam.y ?? 0)];
}

class Shot {
  constructor(dur) {
    this.dur = dur;
    this.tl = gsap.timeline({ paused: true, defaults: { lazy: false } });
    this.tl.set({}, {}, dur);
    this.motion = 0;
  }

  v(at, d, name = 'snap', from = 0, to = 1) {
    const o = { v: from };
    this.tl.to(o, { v: to, duration: d, ease: ease(name), lazy: false }, at);
    this.motion++;
    return o;
  }

  keys(from, list, at = 0) {
    const o = { v: from };
    let t = at;
    for (const [to, d, name = 'soft', wait = 0] of list) {
      t += wait;
      this.tl.to(o, { v: to, duration: d, ease: ease(name), lazy: false }, t);
      t += d;
    }
    this.motion++;
    return o;
  }

  cam(a, b, at = 0, d = this.dur, name = 'soft') {
    const o = { x: 0 };
    this.tl.to(o, { x: 1, duration: d, ease: ease(name), lazy: false }, at);
    this.motion++;
    const blend = (p, q) => (Array.isArray(p) ? p.map((v, i) => mix(v, q[i], o.x)) : mix(p, q, o.x));
    return extra => {
      const c = {};
      for (const k of Object.keys(a)) c[k] = k in b ? blend(a[k], b[k]) : a[k];
      return { ...c, ...extra };
    };
  }

  type(lines, { x = 96, y = 200, size = 120, lead = size * 1.04, at = 0, gap = 0.04, d = 0.28, ease: e = 'snap', center = false, color = C.ink, family = SERIF, hot = [], hotColor = C.lime, leave } = {}) {
    const words = [];
    let n = 0;
    lines.forEach((text, i) => {
      for (const w of text.split(' ')) {
        words.push({ w, line: i, a: this.v(at + n * gap, d, e), b: leave === undefined ? null : this.v(leave + n * gap * 0.5, 0.2, 'in'), color: hot.includes(w) ? hotColor : color });
        n++;
      }
    });
    return g => {
      font(g, size, family);
      const space = g.measureText(' ').width;
      lines.forEach((text, i) => {
        let cx = center ? x - g.measureText(text).width / 2 : x;
        for (const o of words.filter(q => q.line === i)) {
          const width = g.measureText(o.w).width;
          const ly = y + i * lead;
          if (o.a.v > 0) {
            g.save();
            g.beginPath();
            g.rect(cx - size * 0.1, ly - size * 0.92, width + size * 0.2, size * 1.24);
            g.clip();
            g.fillStyle = o.color;
            g.fillText(o.w, cx, ly + size * 1.2 * (1 - o.a.v) - size * 1.2 * (o.b?.v ?? 0));
            g.restore();
          }
          cx += width + space;
        }
      });
    };
  }

  swap(entries, { x, y, size, color = C.lime, family = SERIF } = {}) {
    const list = entries.map(([word, at], i) => ({ word, a: this.v(at, 0.26, 'snap'), b: i + 1 < entries.length ? this.v(entries[i + 1][1], 0.2, 'quart') : null }));
    return g => {
      font(g, size, family);
      for (const o of list) {
        if (o.a.v <= 0 || (o.b?.v ?? 0) >= 1) continue;
        const width = g.measureText(o.word).width;
        g.save();
        g.beginPath();
        g.rect(x - size * 0.1, y - size * 0.92, width + size * 0.2, size * 1.24);
        g.clip();
        g.fillStyle = color;
        g.fillText(o.word, x, y + size * 1.2 * (1 - o.a.v) - size * 1.2 * (o.b?.v ?? 0));
        g.restore();
      }
    };
  }

  strike(rows, { x, y, size, lead = size * 1.1, at = 0, gap = 0.3, color = C.ink, line = C.lime, family = SERIF } = {}) {
    const list = rows.map((row, i) => ({ row, a: this.v(at + i * gap, 0.16), cut: this.v(at + i * gap + 0.2, 0.14) }));
    return g => {
      font(g, size, family);
      list.forEach(({ row: [head, word, tail], a, cut }, i) => {
        if (a.v <= 0) return;
        const ly = y + i * lead, text = head + word + tail;
        g.save();
        g.beginPath();
        g.rect(x - size * 0.1, ly - size * 0.92, W, size * 1.24);
        g.clip();
        g.fillStyle = color;
        g.fillText(text, x, ly + size * 1.2 * (1 - a.v));
        g.restore();
        if (word && cut.v > 0) fill(g, line, x + g.measureText(head).width - 10, ly - size * 0.3, (g.measureText(word).width + 20) * cut.v, Math.max(8, size * 0.07));
      });
    };
  }

  bar(x, y, w, h, color, at, d = 0.2, name = 'snap') {
    const a = this.v(at, d, name);
    return g => fill(g, color, x, y, w * a.v, h);
  }
}

export function shot(name, dur, a, b) {
  const enter = typeof a === 'string' ? a : null;
  register(name, dur, enter, b ?? a);
}

hooks.build = sh => {
  const s = new Shot(sh.dur);
  const draw = sh.build(s);
  if (!s.motion) throw new Error(`shot '${sh.name}' has no motion: add a tween, a type reveal or a camera move`);
  return { tl: s.tl, draw };
};

export { TR };
