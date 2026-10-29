import { createServer } from 'node:http';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync, statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, extname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, '../site/static/art');
const chrome = process.env.CHROME ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

const args = process.argv.slice(2);
const flag = (k, d) => { const i = args.indexOf(`--${k}`); return i < 0 ? d : args[i + 1]; };
const target = args.find((a, i) => !a.startsWith('--') && !(i > 0 && args[i - 1].startsWith('--'))) ?? 'all';
const opts = {
  scale: +flag('scale', 2),
  ss: +flag('ss', 3),
  line: +flag('line', 1),
  shade: flag('shade', null) === null ? undefined : +flag('shade'),
  bg: flag('bg', null),
  ao: +flag('ao', 0),
};
const out = resolve(flag('out', outDir));
const scenes = target === 'all'
  ? readdirSync(join(here, 'scenes')).filter(f => f.endsWith('.glsl')).map(f => f.slice(0, -5)).sort()
  : target.split(',');

const types = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.json': 'application/json', '.glsl': 'text/plain' };
const server = createServer((req, res) => {
  const path = join(here, decodeURIComponent(new URL(req.url, 'http://x').pathname));
  if (!path.startsWith(here)) { res.writeHead(403).end(); return; }
  try {
    if (statSync(path).isDirectory()) {
      res.writeHead(200, { 'content-type': 'text/html' }).end(readdirSync(path).map(f => `<a href="${f}">${f}</a>`).join('\n'));
      return;
    }
    res.writeHead(200, { 'content-type': types[extname(path)] ?? 'application/octet-stream', 'cache-control': 'no-store' }).end(readFileSync(path));
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise(r => server.listen(0, '127.0.0.1', r));
const port = server.address().port;

const profile = mkdtempSync(join(tmpdir(), 'draw-chrome-'));
const proc = spawn(chrome, [
  '--headless=new', '--remote-debugging-port=0', `--user-data-dir=${profile}`, '--no-first-run',
  '--no-default-browser-check', '--hide-scrollbars', '--enable-gpu', '--ignore-gpu-blocklist', 'about:blank',
], { stdio: ['ignore', 'ignore', 'pipe'] });
const wsUrl = await new Promise((ok, fail) => {
  let buf = '';
  proc.stderr.on('data', d => {
    buf += d;
    const m = buf.match(/DevTools listening on (ws:\S+)/);
    if (m) ok(m[1]);
  });
  proc.on('exit', c => fail(new Error(`chrome exited ${c}: ${buf}`)));
});

const ws = new WebSocket(wsUrl);
await new Promise((ok, fail) => { ws.onopen = ok; ws.onerror = fail; });
let seq = 0;
const pending = new Map();
ws.onmessage = e => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) {
    const { ok, fail } = pending.get(m.id);
    pending.delete(m.id);
    m.error ? fail(new Error(m.error.message)) : ok(m.result);
  }
};
const send = (method, params = {}, sessionId) => new Promise((ok, fail) => {
  const id = ++seq;
  pending.set(id, { ok, fail });
  ws.send(JSON.stringify({ id, method, params, sessionId }));
});

let failed = 0;
try {
  const { targetId } = await send('Target.createTarget', { url: `http://127.0.0.1:${port}/render.html` });
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
  mkdirSync(out, { recursive: true });
  const cwebp = spawnSync('which', ['cwebp']).status === 0;
  for (const name of scenes) {
    const t0 = Date.now();
    const expr = `new Promise(r => { const f = () => window.bake ? r() : setTimeout(f, 30); f(); }).then(() => window.bake(${JSON.stringify(name)}, ${JSON.stringify(opts)}))`;
    const { result, exceptionDetails } = await send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true }, sessionId);
    if (exceptionDetails) throw new Error(exceptionDetails.exception?.description ?? exceptionDetails.text);
    if (result.value.error) {
      console.error(`${name}: ${result.value.error}`);
      failed++;
      continue;
    }
    const file = join(out, `${name}.png`);
    writeFileSync(file, Buffer.from(result.value.png.split(',')[1], 'base64'));
    const webp = file.replace(/\.png$/, '.webp');
    if (cwebp && spawnSync('cwebp', ['-quiet', '-lossless', '-z', '6', file, '-o', webp]).status === 0 && !args.includes('--png')) rmSync(file);
    console.log(`${cwebp ? webp : file} ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  }
} finally {
  ws.close();
  const gone = new Promise(r => proc.once('exit', r));
  proc.kill();
  await gone;
  server.close();
  rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}
process.exit(failed ? 1 : 0);
