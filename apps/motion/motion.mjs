#!/usr/bin/env node
// The Mac side of a session: rents and deletes the pod, syncs source, asks serve.mjs to render, downloads results.
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = dirname(fileURLToPath(import.meta.url));
const apps = dirname(dir);
const out = join(dir, 'out');
const state = join(dir, '.pod.json');
const REST = 'https://rest.runpod.io/v1';
const GPUS = ['NVIDIA GeForce RTX 5090', 'NVIDIA GeForce RTX 4090', 'NVIDIA RTX 6000 Ada Generation', 'NVIDIA GeForce RTX 4080 SUPER', 'NVIDIA GeForce RTX 4080', 'NVIDIA GeForce RTX 3090', 'NVIDIA GeForce RTX 3080', 'NVIDIA RTX PRO 6000 Blackwell Server Edition'];
const IMAGE = 'runpod/pytorch:2.1.0-py3.10-cuda11.8.0-devel-ubuntu22.04';
const IDLE = Number(process.env.MOTION_IDLE ?? 1800);

let secret;
const key = () => secret ??= execFileSync('gcloud', ['secrets', 'versions', 'access', 'latest', '--secret=RUNPOD_API_KEY', '--project=marzy-agent'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] }).trim();

async function api(method, path, body) {
  const r = await fetch(REST + path, { method, headers: { Authorization: `Bearer ${key()}`, 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) });
  const text = await r.text();
  if (!r.ok) throw Object.assign(new Error(`RunPod ${method} ${path}: ${r.status} ${text.slice(0, 300)}`), { status: r.status });
  return text ? JSON.parse(text) : null;
}

const pods = async () => (await api('GET', '/pods')) ?? [];
const load = () => (existsSync(state) ? JSON.parse(readFileSync(state, 'utf8')) : null);
const secs = ms => `${(ms / 1000).toFixed(2)}s`;
const stamp = () => { const t = Date.now(); return () => Date.now() - t; };

function session() {
  const s = load();
  if (!s?.host) throw new Error('No session. Run: motion up');
  return s;
}

function sshArgs(s) {
  return ['-p', String(s.port), '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=/dev/null', '-o', 'LogLevel=ERROR', '-o', 'ConnectTimeout=8', '-o', 'ControlMaster=auto', '-o', `ControlPath=/tmp/motion-${s.id}.sock`, '-o', 'ControlPersist=1800', `root@${s.host}`];
}

// Runs a script on the pod over stdin, so nothing needs quoting. Output streams through when `show` is set.
function remote(s, script, { show = false } = {}) {
  const r = spawnSync('ssh', [...sshArgs(s), 'bash -s'], { input: script, encoding: 'utf8', stdio: ['pipe', show ? 'inherit' : 'pipe', show ? 'inherit' : 'pipe'] });
  if (r.status !== 0) throw new Error(`Remote script failed (${r.status}): ${(r.stderr ?? '').trim().slice(-800)}`);
  return r.stdout ?? '';
}

const WATCHDOG = `
#!/bin/bash
touch /tmp/alive
while sleep 20; do
  now=$(date +%s); last=$(stat -c %Y /tmp/alive)
  if [ $((now - last)) -gt \${MOTION_IDLE:-1800} ]; then
    runpodctl remove pod "$RUNPOD_POD_ID"
  fi
done
`;

function rent(gpus, cloudType) {
  const b64 = Buffer.from(WATCHDOG).toString('base64');
  return api('POST', '/pods', {
    name: 'motion',
    imageName: IMAGE,
    gpuTypeIds: gpus,
    gpuTypePriority: 'custom',
    gpuCount: 1,
    cloudType,
    containerDiskInGb: 40,
    volumeInGb: 0,
    ports: ['22/tcp'],
    env: { PUBLIC_KEY: readFileSync(join(homedir(), '.ssh/id_ed25519.pub'), 'utf8').trim(), NVIDIA_DRIVER_CAPABILITIES: 'all', MOTION_IDLE: String(IDLE) },
    dockerStartCmd: ['bash', '-c', `echo ${b64} | base64 -d > /watchdog.sh; nohup bash /watchdog.sh >/dev/null 2>&1 & exec /start.sh`],
  });
}

async function ssh(id) {
  for (let i = 0; i < 90; i++) {
    const p = await api('GET', `/pods/${id}`);
    const port = p.portMappings?.['22'];
    if (p.publicIp && port) {
      const s = { id, host: p.publicIp, port };
      const r = spawnSync('ssh', [...sshArgs(s), 'true'], { stdio: 'ignore' });
      if (r.status === 0) return { ...s, gpu: p.machine?.gpuDisplayName ?? p.gpu?.displayName ?? '', price: p.costPerHr };
    }
    await new Promise(r => setTimeout(r, 2000));
  }
  throw new Error(`Pod ${id} had no working ssh after 3 minutes`);
}

function sync(s) {
  const args = ['-aR', '--delete', '--itemize-changes', '-e', `ssh ${sshArgs(s).slice(0, -1).join(' ')}`,
    '--exclude=out', '--exclude=reference', '--exclude=node_modules', '--exclude=.pod.json', '--exclude=.gitignore',
    './motion', './draw/sdf.glsl', './draw/palette.json', './kit/brands.js', './kit/Word.svelte', `root@${s.host}:/w/`];
  const r = spawnSync('rsync', args, { cwd: apps, encoding: 'utf8' });
  if (r.status !== 0) throw new Error(`rsync failed: ${r.stderr.trim()}`);
  return r.stdout.split('\n').filter(l => /^<f/.test(l)).map(l => l.split(' ').pop());
}

function fetchFile(s, from, to) {
  const r = spawnSync('rsync', ['-a', '-e', `ssh ${sshArgs(s).slice(0, -1).join(' ')}`, `root@${s.host}:${from}`, to], { encoding: 'utf8' });
  if (r.status !== 0) throw new Error(`Download failed: ${r.stderr.trim()}`);
}

const startServe = `
pkill -x node; pkill -x chrome; sleep 0.3
cd /w/motion
PATH=/opt/node/bin:/opt/ff:$PATH setsid nohup node serve.mjs >/tmp/serve.log 2>&1 </dev/null &
for i in $(seq 1 100); do curl -s -o /dev/null http://127.0.0.1:7000/ping && exit 0; sleep 0.1; done
tail -20 /tmp/serve.log; exit 1
`;

// Streams NDJSON from serve.mjs, printing progress; returns the final `done` message.
function ask(s, path, onLine) {
  return new Promise((res, rej) => {
    const p = spawn('ssh', [...sshArgs(s), `curl -sN -m 900 "http://127.0.0.1:7000${path}"`], { stdio: ['ignore', 'pipe', 'inherit'] });
    let buf = '', done = null;
    p.stdout.on('data', d => {
      buf += d;
      let i;
      while ((i = buf.indexOf('\n')) >= 0) {
        const line = buf.slice(0, i);
        buf = buf.slice(i + 1);
        if (!line) continue;
        const m = JSON.parse(line);
        if (m.done) done = m; else if (m.error) rej(new Error(m.error)); else onLine?.(m);
      }
    });
    p.on('close', code => (done ? res(done) : rej(new Error(`Lost the pod server (ssh ${code}). See: motion logs`))));
  });
}

async function up() {
  const total = stamp();
  const live = await pods();
  if (live.length) throw new Error(`Refusing to rent: pod exists already: ${live.map(p => `${p.id} ${p.name} $${p.costPerHr}/hr`).join(', ')}. Use it, or run: motion down`);
  let id = null;
  const kill = async () => {
    if (!id) return;
    const gone = id;
    id = null;
    rmSync(state, { force: true });
    await api('DELETE', `/pods/${gone}`);
    console.log(`deleted pod ${gone}`);
  };
  for (const sig of ['SIGINT', 'SIGTERM', 'SIGHUP']) process.on(sig, () => kill().finally(() => process.exit(1)));
  try {
    let t, s;
    const refused = [];
    const script = `${process.env.MOTION_CPU ? 'export MOTION_CPU=1\n' : ''}${readFileSync(join(dir, 'setup.sh'), 'utf8')}`;
    // Takes the first type with capacity. A host refusing NVENC is deleted and its GPU type dropped; capacity shifts, so retry for 5 minutes.
    let left = [...GPUS];
    const deadline = Date.now() + 300000;
    search: while (left.length && Date.now() < deadline) {
      for (const cloud of ['SECURE', 'COMMUNITY']) {
        t = stamp();
        let p;
        try {
          p = await rent(left, cloud);
        } catch (e) {
          if (e.status !== 500) throw e;
          console.log(`no capacity on ${cloud}`);
          continue;
        }
        id = p.id;
        writeFileSync(state, JSON.stringify({ id, created: Date.now() }));
        const type = (await api('GET', `/pods/${id}`)).machine?.gpuTypeId ?? p.machine?.gpuTypeId ?? p.gpu?.id ?? 'unknown';
        console.log(`rent ${secs(t())} ${type} ${cloud} ${id} $${p.costPerHr}/hr machine ${p.machineId ?? '?'}`);
        t = stamp();
        s = { ...(await ssh(id)), created: Date.now() };
        writeFileSync(state, JSON.stringify(s));
        console.log(`boot ${secs(t())} ${s.host}:${s.port}`);
        t = stamp();
        const r = spawnSync('ssh', [...sshArgs(s), 'bash -s'], { input: script, encoding: 'utf8', stdio: ['pipe', 'inherit', 'inherit'] });
        if (r.status === 0) { console.log(`install ${secs(t())}`); break search; }
        if (r.status !== 3) throw new Error(`setup.sh failed (${r.status})`);
        refused.push(`${type} ${cloud} machine ${p.machineId ?? '?'}`);
        await kill();
        s = null;
        left = left.filter(g => g !== type);
        continue search;
      }
      await new Promise(r => setTimeout(r, 15000));
    }
    if (!s) throw new Error(`No NVENC host found. Refused: ${refused.join('; ') || 'none'}; the rest had no capacity. To accept CPU encoding (libx264) for this session: MOTION_CPU=1 motion up`);
    t = stamp();
    const n = sync(s).length;
    console.log(`sync ${secs(t())} ${n} files`);
    t = stamp();
    remote(s, 'cd /w/motion; PATH=/opt/node/bin:$PATH npm ci --silent --no-audit --no-fund');
    console.log(`npm ${secs(t())}`);
    t = stamp();
    remote(s, startServe);
    const m = await ask(s, '/warm', l => console.log(l.msg));
    console.log(`warm ${secs(t())} ${m.workers} workers on ${m.renderer}, encoder ${m.encoder}`);
    writeFileSync(state, JSON.stringify({ ...s, renderer: m.renderer, workers: m.workers }));
    console.log(`ready in ${secs(total())}`);
    id = null;
  } catch (e) {
    await kill();
    throw e;
  }
}

async function down() {
  rmSync(state, { force: true });
  for (const p of await pods()) {
    await api('DELETE', `/pods/${p.id}`);
    console.log(`deleted ${p.id} ${p.name}`);
  }
  const left = await pods();
  console.log(`pods left: ${left.length}`);
  if (left.length) process.exitCode = 1;
}

async function status() {
  const s = load();
  const live = await pods();
  console.log(`pods: ${live.length}`);
  for (const p of live) {
    const started = p.id === s?.id ? s.created : Date.parse(p.lastStartedAt ?? p.createdAt);
    const h = (Date.now() - started) / 3600000;
    console.log(`${p.id} ${p.name} ${p.machine?.gpuDisplayName ?? p.gpu?.displayName ?? ''} ${p.desiredStatus} up ${(h * 60).toFixed(1)} min at $${p.costPerHr}/hr, about $${(h * p.costPerHr).toFixed(2)} so far`);
  }
  if (s && !live.some(p => p.id === s.id)) console.log(`session state names ${s.id}, which no longer exists (idle watchdog or manual delete); next: motion up`);
  if (s?.host && live.some(p => p.id === s.id)) {
    const m = await ask(s, '/status').catch(e => ({ error: e.message }));
    console.log(m.error ?? `${m.workers} workers on ${m.renderer}, idle ${m.idle}s, deleted after ${m.limit}s idle, ${m.chunks} chunks cached`);
  }
}

async function prep() {
  const s = session();
  const t = stamp();
  const changed = sync(s);
  if (changed.some(f => f.endsWith('serve.mjs'))) {
    remote(s, startServe);
    await ask(s, '/warm');
  }
  return { s, t: t() };
}

async function render(names) {
  const total = stamp();
  const { s, t: syncMs } = await prep();
  let t = stamp();
  const m = await ask(s, `/render?shots=${encodeURIComponent(names.join(','))}`, l => console.log(l.msg));
  const renderMs = t();
  mkdirSync(out, { recursive: true });
  t = stamp();
  const local = join(out, names.length ? `${names.join('-')}.mp4` : 'film.mp4');
  fetchFile(s, m.path, local);
  const parts = Object.entries(m.stages).map(([k, v]) => `${k} ${secs(v)}`).join('  ');
  console.log(`sync ${secs(syncMs)}  render ${secs(renderMs)} (${parts})  download ${secs(t())}  total ${secs(total())}`);
  console.log(`rendered ${m.rendered.length ? m.rendered.join(', ') : 'nothing'} of ${m.shots} shots, ${m.frames} frames, ${m.fps} fps`);
  if (m.detail) console.log(m.detail);
  console.log(`${local} ${m.seconds.toFixed(2)}s ${(m.bytes / 1e6).toFixed(2)} MB`);
}

async function still(path, name) {
  const total = stamp();
  const { s, t: syncMs } = await prep();
  let t = stamp();
  const m = await ask(s, path);
  const renderMs = t();
  mkdirSync(out, { recursive: true });
  t = stamp();
  const local = join(out, name);
  fetchFile(s, m.path, local);
  console.log(`sync ${secs(syncMs)}  render ${secs(renderMs)}  download ${secs(t())}  total ${secs(total())}`);
  console.log(local);
}

const [cmd, ...args] = process.argv.slice(2);
const commands = {
  up,
  down,
  status,
  render: () => render(args),
  frame: () => still(`/frame?t=${args[0]}`, 'frame.png'),
  sheet: () => still(`/sheet?a=${args[0]}&b=${args[1]}&n=${args[2] ?? 12}`, 'sheet.png'),
  logs: async () => process.stdout.write(remote(session(), 'tail -n 60 /tmp/serve.log')),
};
if (!commands[cmd]) {
  console.log('motion up | down | status | render [shot...] | frame <seconds> | sheet <from> <to> [count] | logs');
  process.exit(cmd ? 1 : 0);
}
try {
  await commands[cmd]();
} catch (e) {
  console.error(e.message);
  process.exitCode = 1;
}
process.exit();
