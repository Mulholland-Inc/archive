import { spawnSync } from 'node:child_process';
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { withPage } from './bake.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '../..');
const bots = join(repo, 'apps/web/public/bots');
const WORK = ['personal', 'family', 'business'];
const DEFAULTS = { personal: 'logcabin', family: 'cottage', business: 'office' };
const GROUPS = ['home', 'office', 'health', 'food', 'shop', 'care', 'culture', 'civic', 'auto', 'trade', 'transport', 'energy'];

const agents = process.argv.includes('--agents');
const [what] = process.argv.slice(2).filter(a => a !== '--agents');
const at = process.argv.indexOf('--out');
const out = at < 0 ? join(tmpdir(), 'icons.png') : resolve(process.argv[at + 1]);
if (!what) {
  console.error('usage: icons.mjs [--agents] <a,b|all> [--out file.png]');
  process.exit(1);
}

const prefix = agents ? 'agent-' : 'bot-';
const lib = readdirSync(join(here, 'scenes')).filter(f => f.startsWith(prefix) && f.endsWith('.glsl')).map(f => {
  const m = readFileSync(join(here, 'scenes', f), 'utf8').match(/^#pragma frame (.*)$/m);
  return { name: f.slice(prefix.length, -5), ...JSON.parse(m[1]) };
}).sort((a, b) => GROUPS.indexOf(a.group) - GROUPS.indexOf(b.group) || a.label.localeCompare(b.label));
const names = what === 'all' ? lib.map(i => i.name) : what.split(',');

function edit(file, re, to) {
  const path = join(repo, file);
  const before = readFileSync(path, 'utf8');
  if (!re.test(before)) throw new Error(`register: nothing matches ${re}`);
  const after = before.replace(re, to);
  if (after !== before) writeFileSync(path, after);
}

function register() {
  const of = set => lib.filter(i => i.set.includes(set)).map(i => i.name);
  const offered = t => { const rest = of(t).filter(n => n !== DEFAULTS[t]); return [rest[0], DEFAULTS[t], ...rest.slice(1)]; };
  const work = [...new Set(WORK.flatMap(of))];
  edit('apps/web/src/lib/looks.js', /export const TYPES = \{[\s\S]*?\n\};/, `export const TYPES = {\n${WORK.map(t => `  ${t}: { label: '${t[0].toUpperCase()}${t.slice(1)}', icons: [${offered(t).map(n => `'${n}'`).join(', ')}] }`).join(',\n')}\n};`);
  edit('apps/web/src/lib/looks.js', /export const ICON_LABELS = \{[\s\S]*?\n\};/, `export const ICON_LABELS = {\n${lib.map(i => `  ${i.name}: '${i.label.replace(/'/g, "\\'")}'`).join(',\n')}\n};`);
  edit('src/workspace.go', /var workspaceIcons = \[\]string\{.*\}/, `var workspaceIcons = []string{${work.map(n => `"${n}"`).join(', ')}}`);
  console.log(`registered ${work.length} workspace icons`);
}

function registerAgents() {
  const saved = readFileSync(join(repo, 'apps/web/src/lib/looks.js'), 'utf8').match(/export const AVATARS = \[(.*)\];/)[1].match(/\w+/g) ?? [];
  const order = [...saved.filter(n => lib.some(i => i.name === n)), ...lib.map(i => i.name).filter(n => !saved.includes(n))];
  const label = n => lib.find(i => i.name === n).label.replace(/'/g, "\\'");
  edit('apps/web/src/lib/looks.js', /export const AVATARS = \[.*\];/, `export const AVATARS = [${order.map(n => `'${n}'`).join(', ')}];`);
  edit('apps/web/src/lib/looks.js', /export const AVATAR_LABELS = \{.*\};/, `export const AVATAR_LABELS = { ${order.map(n => `${n}: '${label(n)}'`).join(', ')} };`);
  edit('src/archive.sql', /CHECK \(avatar BETWEEN 0 AND \d+\)/, `CHECK (avatar BETWEEN 0 AND ${order.length - 1})`);
  edit('src/archive.sql', /(COMMENT ON COLUMN archive\.agent\.avatar IS 'Its picture, one figure by number: ).*';/, `$1${order.map((n, i) => `${i} ${label(n).toLowerCase()}`).join(', ')}';`);
  console.log(`registered ${order.length} agent figures`);
  return order;
}

async function bake() {
  let failed = 0;
  await withPage(async send => {
    for (const name of names) {
      const t0 = Date.now();
      const { result, exceptionDetails } = await send('Runtime.evaluate', { expression: `new Promise(r => { const f = () => window.icon ? r() : setTimeout(f, 30); f(); }).then(() => window.icon(${JSON.stringify(`${prefix}${name}`)}))`, awaitPromise: true, returnByValue: true });
      if (exceptionDetails || result.value.error) {
        console.error(`${name}: ${result.value?.error ?? exceptionDetails.exception?.description}`);
        failed++;
        continue;
      }
      const made = spawnSync('cwebp', ['-quiet', '-lossless', '-z', '9', '-m', '6', '-o', join(bots, `${agents ? 'agent-' : ''}${name}.webp`), '--', '-'], { input: Buffer.from(result.value.png.split(',')[1], 'base64') });
      if (made.status !== 0) throw new Error(`cwebp: ${made.stderr}`);
      console.log(`${name} ${((Date.now() - t0) / 1000).toFixed(1)}s`);
    }
  });
  if (failed) process.exit(1);
}

async function sheet() {
  const tokens = readFileSync(join(repo, 'apps/web/src/styles/tokens.css'), 'utf8');
  const styles = readFileSync(join(repo, 'apps/web/src/components/ArchiveIcon.svelte'), 'utf8').match(/<style>([\s\S]*)<\/style>/)[1];
  const tints = tokens.match(/--tint-\d+:/g).length;
  const bot = (name, tint, size) => `<span class="bot ${size} i-${name}" style="--tint: var(--tint-${tint})"><span class="art"><span class="layer" style="--cell: 0"><i class="sheen"></i></span></span></span>`;
  const cell = (name, t) => `<div class="cell"><div class="row">${bot(name, t, 'xxl')}</div><div class="row">${bot(name, t, 'lg')}${bot(name, (t + 5) % tints, 'rail')}</div><span>${name}</span></div>`;
  const rows = [];
  for (let i = 0; i < names.length; i += 10) rows.push(names.slice(i, i + 10));
  const html = `<!doctype html><meta charset="utf-8"><style>${tokens}${styles}
    ${names.map(n => `.i-${n} { --src: url(data:image/webp;base64,${readFileSync(join(bots, `${n}.webp`)).toString('base64')}); }`).join('\n')}
    body { margin: 0; font: 11px/14px -apple-system, sans-serif; width: max-content; }
    .band { display: grid; grid-template-columns: repeat(10, 104px); gap: 4px; padding: 10px; }
    .light { background: #f7f7f8; color: #333; }
    .dark { background: #17181b; color: #ccc; }
    .cell { display: grid; justify-items: center; gap: 2px; width: 104px; }
    .row { display: flex; align-items: center; gap: 6px; }
  </style>${rows.map((row, r) => `<div class="band ${r % 2 ? 'dark' : 'light'}">${row.map((n, i) => cell(n, ((r * 10 + i) * 3) % tints)).join('')}</div>`).join('')}`;
  await shoot(html);
}

async function agentSheet(order) {
  const shown = order.filter(n => names.includes(n));
  const tokens = readFileSync(join(repo, 'apps/web/src/styles/tokens.css'), 'utf8');
  const styles = readFileSync(join(repo, 'apps/web/src/components/AgentAvatar.svelte'), 'utf8').match(/<style>([\s\S]*)<\/style>/)[1];
  const tints = [17, 10, 28, 12, 21, 8, 24, 2, 16, 26, 19];
  const bot = (name, tint, size, cell = 0, state = '') => `<span class="bot ${size} ${state} i-${name}" style="--tint: var(--tint-${tint})"><span class="art"><span class="layer" style="--cell: ${cell}"><i class="sheen"></i><i class="eyes"></i></span></span></span>`;
  const cell = (name, i) => `<div class="cell"><div class="row">${bot(name, tints[i % tints.length], 'xxl')}</div><div class="row">${bot(name, tints[(i + 3) % tints.length], 'lg')}${bot(name, tints[(i + 5) % tints.length], 'lg', 3, 'open')}${bot(name, tints[(i + 7) % tints.length], 'xs')}</div><span>${name}</span></div>`;
  const band = kind => `<div class="band ${kind}">${shown.map(cell).join('')}</div>`;
  await shoot(`<!doctype html><meta charset="utf-8"><style>${tokens}${styles}
    ${names.map(n => `.i-${n} { --src: url(data:image/webp;base64,${readFileSync(join(bots, `agent-${n}.webp`)).toString('base64')}); }`).join('\n')}
    body { margin: 0; font: 11px/14px -apple-system, sans-serif; width: max-content; }
    .band { display: grid; grid-template-columns: repeat(11, 124px); gap: 10px 4px; padding: 12px; }
    .light { background: #f7f7f8; color: #333; }
    .dark { background: #17181b; color: #ccc; }
    .cell { display: grid; justify-items: center; gap: 4px; width: 124px; }
    .row { display: flex; align-items: center; gap: 2px; }
  </style>${band('light')}${band('dark')}`);
}

async function shoot(html) {
  await withPage(async send => {
    await send('Emulation.setDeviceMetricsOverride', { width: 1400, height: 1000, deviceScaleFactor: 2, mobile: false });
    await send('Runtime.evaluate', { expression: `document.open(); document.write(${JSON.stringify(html)}); document.close(); new Promise(r => setTimeout(r, 600))`, awaitPromise: true });
    const { result } = await send('Runtime.evaluate', { expression: 'JSON.stringify([document.body.scrollWidth, document.body.scrollHeight])', returnByValue: true });
    const [width, height] = JSON.parse(result.value);
    const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true, clip: { x: 0, y: 0, width, height, scale: 1 } });
    writeFileSync(out, Buffer.from(shot.data, 'base64'));
    console.log(out);
  });
}

await bake();
if (agents) {
  await agentSheet(registerAgents());
} else {
  register();
  await sheet();
}
