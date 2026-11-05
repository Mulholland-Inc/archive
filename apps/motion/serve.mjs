import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createReadStream, existsSync, readFileSync, readdirSync, renameSync, rmSync, statSync, utimesSync, writeFileSync } from 'node:fs';
import http from 'node:http';
import { dirname, extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = dirname(here);
const CACHE = '/cache/chunks';
const WORKERS = Number(process.env.WORKERS ?? 6);
const ENCODERS = 6;
const CHROME = '/opt/chrome/chrome-linux64/chrome';
const FPS = 60, W = 1920, H = 1080, FRAME = W * H * 3 / 2;
const IDLE_LIMIT = Number(readFileSync('/proc/1/environ', 'utf8').match(/MOTION_IDLE=(\d+)/)?.[1] ?? 1800);

const NVENC = '-c:v h264_nvenc -preset p7 -tune hq -profile:v high -level 4.2 -rc vbr -cq 33 -b:v 0 -maxrate 14M -bufsize 28M -bf 3 -b_ref_mode middle -rc-lookahead 32 -g 240 -forced-idr 1 -spatial-aq 1 -temporal-aq 1';
const X264 = '-c:v libx264 -preset veryfast -crf 20 -profile:v high -level 4.2 -bf 0 -g 240 -threads 8';
const hasNvenc = spawnSync('ffmpeg', '-hide_banner -loglevel error -y -f lavfi -i testsrc2=s=1920x1080:r=60 -frames:v 2 -pix_fmt yuv420p -c:v h264_nvenc /tmp/nvenc.mp4'.split(' ')).status === 0;
const ENCODE = hasNvenc ? NVENC : X264;
const encoderName = hasNvenc ? 'h264_nvenc (GPU)' : 'libx264 (CPU)';
const COLOR = '-colorspace bt709 -color_primaries bt709 -color_trc bt709 -color_range tv';
const INPUT = `-f rawvideo -pix_fmt yuv420p -s ${W}x${H} -r ${FPS} ${COLOR} -thread_queue_size 64`;

const sha = (...parts) => { const h = createHash('sha256'); for (const p of parts) h.update(p + '\0'); return h.digest('hex').slice(0, 16); };
const read = p => readFileSync(join(root, p), 'utf8');
const sleep = ms => new Promise(r => setTimeout(r, ms));

const workers = new Map();
let renderer = '', gpuError = '';

function launch(i) {
  const flags = ['--headless=new', '--no-sandbox', '--use-gl=angle', '--use-angle=gl-egl', '--ignore-gpu-blocklist', '--enable-gpu-rasterization', '--disable-software-rasterizer', '--disable-gpu-vsync', '--disable-frame-rate-limit', '--disable-background-timer-throttling', '--disable-renderer-backgrounding', '--disable-backgrounding-occluded-windows', '--no-first-run', '--mute-audio', '--disable-extensions', `--user-data-dir=/tmp/chrome-${i}`, `http://127.0.0.1:7000/motion/index.html?w=${i}&e=${engineKey()}`];
  const p = spawn(CHROME, flags, { env: { ...process.env, EGL_PLATFORM: 'surfaceless' }, stdio: 'ignore' });
  const w = { id: i, p, ready: false, res: null, job: null };
  workers.set(i, w);
  p.on('exit', () => {
    const job = w.job;
    if (workers.get(i) === w) { workers.delete(i); console.log(`worker ${i} exited`); }
    if (job && current) fail(`Chrome worker ${i} died during ${job.shot ?? ''} at ${job.t ?? '?'}s`);
    setTimeout(() => { if (!workers.has(i)) launch(i); }, 500);
  });
}

function engineKey() {
  return sha(read('motion/engine.js'), read('motion/kit.js'), read('motion/index.html'));
}

const queue = [];
let current = null;
let warmKey = '';

function pump() {
  for (const w of workers.values()) {
    if (!queue.length) return;
    if (!w.res) continue;
    const at = queue.findIndex(j => j.to === undefined || j.to === w.id);
    if (at < 0) continue;
    const [job] = queue.splice(at, 1);
    w.job = job;
    const res = w.res;
    w.res = null;
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({ ...job, engine: engineKey() }));
  }
}

function send(res, obj) { res.write(JSON.stringify(obj) + '\n'); }

function fail(message) {
  const c = current;
  if (!c) return;
  current = null;
  queue.length = 0;
  for (const e of c.encoders.values()) { e.proc.kill('SIGKILL'); rmSync(e.tmp, { force: true }); }
  c.reject(new Error(message));
}

async function warm(res) {
  const t = Date.now();
  while (workers.size < WORKERS || [...workers.values()].some(w => !w.ready)) {
    if (gpuError) throw new Error(gpuError);
    if (Date.now() - t > 60000) throw new Error('Workers did not report in 60s. See: motion logs');
    await sleep(50);
  }
  const v = manifest();
  const key = engineKey() + v.film;
  if (key !== warmKey) {
    const a = Date.now();
    await broadcast({ kind: 'compile', v: v.film, scenes: v.scenes });
    warmKey = key;
    send(res, { msg: `  loaded ${v.scenes.length} scenes in ${Date.now() - a}ms` });
  }
  return v;
}

function broadcast(job) {
  return new Promise((res, rej) => {
    const prev = current;
    let left = workers.size;
    current = { encoders: new Map(), reject: rej, done() { if (--left === 0) { current = prev; res(); } } };
    for (const w of workers.values()) queue.push({ ...job, to: w.id });
    pump();
  });
}

function manifest() {
  const src = read('motion/film.js');
  const starts = [...src.matchAll(/^shot\(/gm)].map(m => m.index);
  const preamble = src.slice(0, starts[0] ?? src.length);
  const names = new Set(readdirSync(join(here, 'scenes')).filter(f => f.endsWith('.glsl')).map(f => f.slice(0, -5)));
  const engine = engineKey();
  const lib = sha(read('draw/sdf.glsl'), read('motion/film.glsl'), read('draw/palette.json'));
  const seen = new Set();
  const shots = starts.map((at, i) => {
    const block = src.slice(at, starts[i + 1] ?? src.length).trimEnd();
    const line = src.slice(0, at).split('\n').length;
    const m = block.match(/^shot\('([\w-]+)', ([\d.]+)(?:, '(\w+)')?/);
    if (!m) throw new Error(`film.js:${line}:1: a shot starts with shot('name', seconds, ...`);
    if (seen.has(m[1])) throw new Error(`film.js:${line}:7: shot name '${m[1]}' is used twice`);
    seen.add(m[1]);
    const scenes = [...new Set([...block.matchAll(/'([\w-]+)'/g)].map(x => x[1]).filter(x => names.has(x)))];
    return { name: m[1], line, frames: Math.round(+m[2] * FPS), enter: m[3] ?? null, scenes, block, own: sha(block, ...scenes.map(n => read(`motion/scenes/${n}.glsl`)), scenes.length ? lib : '') };
  });
  shots.forEach((s, i) => { s.key = sha(INPUT, ENCODE, COLOR, engine, preamble, s.own, s.frames, s.enter && i ? shots[i - 1].own + shots[i - 1].frames : ''); });
  const scenes = [...new Set(shots.flatMap(s => s.scenes))];
  return { shots, film: sha(src, lib, ...scenes.map(n => read(`motion/scenes/${n}.glsl`))), scenes };
}

function encoder(c, shot, i) {
  const tmp = join(CACHE, `${shot.key}.${process.pid}.tmp.mp4`);
  const args = `-hide_banner -loglevel error -y ${INPUT} -i pipe:0 ${ENCODE} ${COLOR} -an -video_track_timescale 15360 ${tmp}`.split(' ');
  const proc = spawn('ffmpeg', args, { stdio: ['pipe', 'ignore', 'pipe'] });
  let err = '';
  proc.stderr.on('data', d => { err += d; });
  const e = { shot, proc, tmp, next: 0, hold: new Map(), written: 0, done: null };
  e.done = new Promise((res, rej) => {
    proc.on('close', code => {
      if (code === 0) { renameSync(tmp, join(CACHE, `${shot.key}.mp4`)); res(); } else rej(new Error(`ffmpeg failed on shot ${shot.name}: ${err.trim().slice(-400)}`));
    });
  });
  proc.stdin.on('error', () => {});
  e.done.then(() => c.finished(e), err => fail(err.message));
  return e;
}

function receive(c, e, i, buf) {
  e.hold.set(i, buf);
  let wrote = false;
  while (e.hold.has(e.next)) {
    const b = e.hold.get(e.next);
    e.hold.delete(e.next);
    e.next++;
    e.proc.stdin.write(b);
    wrote = true;
  }
  if (wrote && e.next === e.shot.frames) e.proc.stdin.end();
}

function render(res, v, want) {
  return new Promise((resolve, reject) => {
    const prev = name => v.shots[v.shots.findIndex(s => s.name === name) - 1];
    const todo = want.filter(s => !existsSync(join(CACHE, `${s.key}.mp4`)));
    const c = { encoders: new Map(), reject, stats: { draw: 0, read: 0, post: 0, n: 0 }, waiting: [...todo], started: Date.now() };
    const next = () => {
      while (c.waiting.length && c.encoders.size < ENCODERS) {
        const s = c.waiting.shift();
        const e = encoder(c, s);
        c.encoders.set(s.name, e);
        const p = s.enter ? prev(s.name) : null;
        for (let i = 0; i < s.frames; i++) queue.push({ kind: 'frame', v: v.film, shot: s.name, scenes: s.scenes, i, t: i / FPS, prev: p?.name ?? null, prevScenes: p?.scenes ?? [], prevDur: p ? p.frames / FPS : 0, enter: s.enter });
      }
      pump();
    };
    c.finished = e => {
      c.encoders.delete(e.shot.name);
      send(res, { msg: `  ${e.shot.name} ${e.shot.frames} frames in ${((Date.now() - c.started) / 1000).toFixed(2)}s` });
      next();
      if (!c.encoders.size && !c.waiting.length) { current = null; resolve({ v, todo, c }); }
    };
    if (!todo.length) { resolve({ v, todo, c }); return; }
    current = c;
    next();
  });
}

function join_(shots, path) {
  const list = `${path}.txt`;
  writeFileSync(list, shots.map(s => `file '${CACHE}/${s.key}.mp4'`).join('\n') + '\n');
  const r = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-f', 'concat', '-safe', '0', '-i', list, '-c', 'copy', '-movflags', '+faststart', path], { encoding: 'utf8' });
  if (r.status !== 0) throw new Error(`join failed: ${r.stderr}`);
}

function select(v, names) {
  const want = names.length ? names.map(n => v.shots.find(s => s.name === n) ?? (() => { throw new Error(`No shot named ${n}. Shots: ${v.shots.map(s => s.name).join(', ')}`); })()) : v.shots;
  return want;
}

function locate(v, t) {
  let at = 0;
  for (let i = 0; i < v.shots.length; i++) {
    const s = v.shots[i];
    if (t < at + s.frames / FPS || i === v.shots.length - 1) return { s, p: v.shots[i - 1], local: Math.min(t - at, s.frames / FPS) };
    at += s.frames / FPS;
  }
  throw new Error('The film has no shots');
}

function stillJob(job) {
  return new Promise((resolve, reject) => {
    current = { encoders: new Map(), reject, resolve };
    queue.push(job);
    pump();
  });
}

function shotJob(v, t) {
  const { s, p, local } = locate(v, t);
  const i = Math.round(local * FPS);
  return { shot: s.name, scenes: s.scenes, i, t: i / FPS, prev: s.enter && p ? p.name : null, prevScenes: s.enter && p ? p.scenes : [], prevDur: p ? p.frames / FPS : 0, enter: s.enter };
}

const types = { '.js': 'text/javascript', '.mjs': 'text/javascript', '.html': 'text/html', '.json': 'application/json', '.woff2': 'font/woff2', '.glsl': 'text/plain', '.svelte': 'text/plain' };

function body(req) {
  return new Promise(res => {
    const parts = [];
    let n = 0;
    req.on('data', d => { parts.push(d); n += d.length; });
    req.on('end', () => res(parts.length === 1 ? parts[0] : Buffer.concat(parts, n)));
  });
}

function touch() {
  const now = new Date();
  try { utimesSync('/tmp/alive', now, now); } catch { writeFileSync('/tmp/alive', ''); }
}

async function answer(res, fn) {
  res.setHeader('Content-Type', 'application/x-ndjson');
  try {
    send(res, await fn());
  } catch (e) {
    send(res, { error: e.message });
  }
  res.end();
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://x');
  const p = url.pathname, q = url.searchParams;
  const w = workers.get(Number(q.get('w')));

  if (p === '/next') { if (w) { w.job = null; w.res = res; pump(); } else res.end('{}'); return; }

  if (p === '/hello') {
    const m = JSON.parse(await body(req));
    if (m.error || /swiftshader|llvmpipe|software/i.test(m.renderer ?? '')) gpuError = `GPU check failed on worker ${q.get('w')}: ${m.error ?? m.renderer}. Software fallback runs near 1 fps, so this session refuses to render.`;
    renderer = m.renderer ?? renderer;
    if (w) w.ready = true;
    console.log(`worker ${q.get('w')}: ${m.renderer ?? m.error}`);
    res.end('{}');
    if (w?.job) { queue.unshift(w.job); w.job = null; }
    return;
  }

  if (p === '/push') {
    const c = current;
    const b = await body(req);
    res.end('{}');
    const e = c?.encoders?.get(q.get('shot'));
    if (!e) return;
    if (b.length !== FRAME) { fail(`Worker sent ${b.length} bytes for a frame, expected ${FRAME}`); return; }
    c.stats.draw += +req.headers['x-draw'];
    c.stats.read += +req.headers['x-read'];
    c.stats.post += +req.headers['x-post'];
    c.stats.n++;
    receive(c, e, +q.get('i'), b);
    return;
  }

  if (p === '/done') {
    await body(req);
    res.end('{}');
    current?.done?.();
    return;
  }

  if (p === '/image') {
    const b = await body(req);
    res.end('{}');
    const path = `/cache/${q.get('name')}`;
    writeFileSync(path, b);
    current?.resolve?.({ done: true, path });
    current = null;
    return;
  }

  if (p === '/fail') {
    const m = JSON.parse(await body(req));
    res.end('{}');
    console.log('fail', m);
    const check = m.film ? spawnSync('node', ['--check', join(here, 'film.js')], { encoding: 'utf8' }).stderr : '';
    fail(check ? check.split('\n').filter(l => !/^\s*at |^Node\.js/.test(l)).join('\n').trim() : m.message);
    return;
  }

  if (p === '/ping') { res.end('ok'); return; }

  if (p === '/warm') {
    await answer(res, async () => {
      await warm(res);
      return { done: true, workers: workers.size, renderer, encoder: encoderName };
    });
    return;
  }

  if (p === '/status') {
    await answer(res, async () => ({ done: true, workers: workers.size, renderer, idle: Math.round((Date.now() - statSync('/tmp/alive').mtimeMs) / 1000), limit: IDLE_LIMIT, chunks: readdirSync(CACHE).length }));
    return;
  }

  if (p === '/render') {
    touch();
    await answer(res, async () => {
      if (current) throw new Error('A render is running already');
      const t0 = Date.now();
      const v0 = await warm(res);
      const want = select(v0, (q.get('shots') ?? '').split(',').filter(Boolean));
      const r = await render(res, v0, want);
      const t1 = Date.now();
      const path = '/cache/film.mp4';
      join_(want, path);
      const frames = r.todo.reduce((n, s) => n + s.frames, 0);
      const { draw, read: rd, post, n } = r.c.stats;
      touch();
      return {
        done: true, path, shots: v0.shots.length, frames,
        rendered: r.todo.map(s => s.name),
        fps: frames ? Math.round(frames / ((t1 - t0) / 1000)) : 0,
        stages: { frames: t1 - t0, join: Date.now() - t1 },
        detail: n ? `per frame in a worker: draw ${(draw / n).toFixed(1)}ms, gpu+readback ${(rd / n).toFixed(1)}ms, post ${(post / n).toFixed(1)}ms` : '',
        seconds: want.reduce((n, s) => n + s.frames, 0) / FPS,
        bytes: statSync(path).size,
      };
    });
    return;
  }

  if (p === '/frame' || p === '/sheet') {
    touch();
    await answer(res, async () => {
      if (current) throw new Error('A render is running already');
      const v = await warm(res);
      if (p === '/frame') {
        return await stillJob({ kind: 'png', v: v.film, name: 'frame.png', ...shotJob(v, +q.get('t')) });
      }
      const a = +q.get('a'), b = +q.get('b'), n = Math.max(1, +q.get('n'));
      const items = Array.from({ length: n }, (_, i) => { const at = n > 1 ? a + (b - a) * i / (n - 1) : a; return { at, ...shotJob(v, at) }; });
      return await stillJob({ kind: 'sheet', v: v.film, name: 'sheet.png', items });
    });
    return;
  }

  const file = normalize(join(root, p));
  if (!file.startsWith(root) || !existsSync(file) || statSync(file).isDirectory()) { res.statusCode = 404; res.end(); return; }
  res.setHeader('Content-Type', types[extname(file)] ?? 'application/octet-stream');
  res.setHeader('Cache-Control', 'no-store');
  createReadStream(file).pipe(res);
});

server.keepAliveTimeout = 0;
server.requestTimeout = 0;
server.listen(7000, '127.0.0.1', () => {
  touch();
  for (let i = 0; i < WORKERS; i++) launch(i);
});
