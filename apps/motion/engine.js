export const W = 1920, H = 1080, SS = 2, FPS = 60, TR = 0.16;

const lines = s => s.split('\n').length;
const hex = h => { const n = parseInt(h.slice(1), 16); return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255]; };
const text = async p => { const r = await fetch(p, { cache: 'no-store' }); if (!r.ok) throw new Error(`${p}: ${r.status}`); return r.text(); };

const vsrc = `#version 300 es
in vec2 a;
void main() { gl_Position = vec4(a, 0.0, 1.0); }`;

const gbuf = `
uniform vec3 uCenter, uRight, uUp, uFwd, uBmin, uBmax;
uniform vec2 uOrigin, uHalf;
uniform float uPx;
layout(location = 0) out vec4 o0;
layout(location = 1) out vec4 o1;
vec3 normalAt(vec3 p, float e) {
  const vec2 k = vec2(1.0, -1.0);
  return normalize(k.xyy * map(p + k.xyy * e).x + k.yyx * map(p + k.yyx * e).x + k.yxy * map(p + k.yxy * e).x + k.xxx * map(p + k.xxx * e).x);
}
void main() {
  vec2 pix = gl_FragCoord.xy + uOrigin - uHalf;
  vec3 ro = uCenter + (uRight * pix.x + uUp * pix.y) * uPx;
  vec3 rd = uFwd;
  vec3 inv = 1.0 / (rd + vec3(1e-7));
  vec3 t0 = (uBmin - ro) * inv, t1 = (uBmax - ro) * inv;
  vec3 tn = min(t0, t1), tf = max(t0, t1);
  float tmin = max(max(tn.x, tn.y), tn.z), tmax = min(min(tf.x, tf.y), tf.z);
  o0 = vec4(0.0);
  o1 = vec4(-1.0, 1.0, 0.0, 0.0);
  if (tmax < tmin) return;
  float t = tmin;
  float eps = uPx * 0.15;
  vec2 h = vec2(1.0);
  bool hit = false;
  for (int i = 0; i < 600; i++) {
    h = map(ro + rd * t);
    if (h.x < eps) { hit = true; break; }
    t += h.x * 0.85;
    if (t > tmax) break;
  }
  if (!hit) return;
  vec3 p = ro + rd * t;
  o0 = vec4(normalAt(p, max(uPx * 0.5, 2e-5)), t);
  o1 = vec4(h.y, 1.0, 1.0, 0.0);
}`;

const compSrc = `#version 300 es
precision highp float;
precision highp int;
uniform highp sampler2D g0;
uniform highp sampler2D g1;
uniform ivec2 uOff;
uniform float uR, uPx, uCos, uTol;
uniform vec3 uRight, uUp, uFwd, uLeftN, uRightN;
uniform vec3 uPal[16];
uniform vec3 uShade1, uShade2;
uniform float uShade;
out vec4 o;
int tone(vec3 n, float m) {
  float v = floor(mod(m, 64.0) / 16.0);
  if (v > 1.5 || n.y > 0.5) return 0;
  if (v > 0.5) return 1;
  return dot(n, uLeftN) > dot(n, uRightN) - 0.1 ? 1 : 2;
}
vec3 colour(float m, int t) {
  int i = int(mod(m, 16.0) + 0.5);
  vec3 c = uPal[i];
  if (i == 0 || t == 0) return c;
  vec3 k = t == 1 ? uShade1 : uShade2;
  return mix(c, c * mix(k, c, 0.3), uShade);
}
void main() {
  ivec2 pc = ivec2(gl_FragCoord.xy) + uOff;
  vec4 a0 = texelFetch(g0, pc, 0);
  vec4 a1 = texelFetch(g1, pc, 0);
  bool ah = a1.z > 0.5;
  int at = ah ? tone(a0.xyz, a1.x) : -1;
  int R = int(ceil(uR + 0.5));
  float dmin = 1e9;
  for (int y = -R; y <= R; y++) {
    for (int x = -R; x <= R; x++) {
      float dd = length(vec2(x, y));
      if (dd > uR + 0.5 || dd >= dmin || (x == 0 && y == 0)) continue;
      ivec2 qc = pc + ivec2(x, y);
      vec4 b1 = texelFetch(g1, qc, 0);
      bool bh = b1.z > 0.5;
      bool e = false;
      if (ah != bh) e = true;
      else if (ah) {
        vec4 b0 = texelFetch(g0, qc, 0);
        if (abs(a1.x - b1.x) > 0.5) e = true;
        else if (at != tone(b0.xyz, b1.x)) e = true;
        else if (dot(a0.xyz, b0.xyz) < uCos) e = true;
        else {
          vec3 dP = (uRight * float(x) + uUp * float(y)) * uPx + uFwd * (b0.w - a0.w);
          if (min(abs(dot(dP, a0.xyz)), abs(dot(dP, b0.xyz))) > uTol) e = true;
        }
      }
      if (e) dmin = dd;
    }
  }
  float ink = clamp(uR - dmin + 0.5, 0.0, 1.0);
  vec3 inkc = uPal[0];
  o = ah ? vec4(mix(colour(a1.x, at), inkc, ink), 1.0) : vec4(inkc * ink, ink);
}`;

const downSrc = `#version 300 es
precision highp float;
uniform sampler2D c;
uniform int uS;
out vec4 o;
void main() {
  ivec2 b = ivec2(gl_FragCoord.xy) * uS;
  vec4 s = vec4(0.0);
  for (int y = 0; y < uS; y++) for (int x = 0; x < uS; x++) s += texelFetch(c, b + ivec2(x, y), 0);
  s /= float(uS * uS);
  o = s.a > 0.0 ? vec4(s.rgb / s.a, s.a) : vec4(0.0);
}`;

const yuvSrc = `#version 300 es
precision highp float;
precision highp int;
uniform sampler2D c;
out vec4 o;
const vec3 Y = vec3(0.2126, 0.7152, 0.0722), U = vec3(-0.1146, -0.3854, 0.5), V = vec3(0.5, -0.4542, -0.0458);
vec3 chroma(ivec2 q) {
  ivec2 b = q * 2;
  return (texelFetch(c, b, 0).rgb + texelFetch(c, b + ivec2(1, 0), 0).rgb + texelFetch(c, b + ivec2(0, 1), 0).rgb + texelFetch(c, b + ivec2(1, 1), 0).rgb) * 0.25;
}
void main() {
  ivec2 p = ivec2(gl_FragCoord.xy);
  float v;
  if (p.y < ${H}) {
    v = (16.0 + 219.0 * dot(texelFetch(c, p, 0).rgb, Y)) / 255.0;
  } else {
    int r = p.y - ${H}, plane = r >= ${H / 4} ? 1 : 0;
    int n = (r - plane * ${H / 4}) * ${W} + p.x;
    vec3 k = chroma(ivec2(n % ${W / 2}, n / ${W / 2}));
    v = (128.0 + 224.0 * dot(k, plane == 0 ? U : V)) / 255.0;
  }
  o = vec4(v, 0.0, 0.0, 1.0);
}`;

export class GLSLError extends Error {}

function annotate(log, parts) {
  return log.split('\n').filter(Boolean).map(l => {
    const m = l.match(/^ERROR: 0:(\d+): (?:'([^']*)')?(.*)$/);
    if (!m) return l;
    let n = +m[1];
    for (const p of parts) {
      const c = lines(p.src);
      if (n <= c) {
        const col = m[2] ? p.src.split('\n')[n - 1].indexOf(m[2]) + 1 : 0;
        return `${p.name}:${n}:${Math.max(col, 1)}: ${m[2] ? `'${m[2]}'` : ''}${m[3]}`;
      }
      n -= c;
    }
    return l;
  }).join('\n');
}

const glc = Object.assign(document.createElement('canvas'), { width: W, height: H });
const gl = glc.getContext('webgl2', { antialias: false, premultipliedAlpha: false, preserveDrawingBuffer: true, powerPreference: 'high-performance' });

function program(parts) {
  const src = parts.map(p => p.src).join('\n');
  const sh = (type, s) => {
    const o = gl.createShader(type);
    gl.shaderSource(o, s);
    gl.compileShader(o);
    if (!gl.getShaderParameter(o, gl.COMPILE_STATUS)) throw new GLSLError(annotate(gl.getShaderInfoLog(o) ?? 'no compile log', parts));
    return o;
  };
  const p = gl.createProgram();
  gl.attachShader(p, sh(gl.VERTEX_SHADER, vsrc));
  gl.attachShader(p, sh(gl.FRAGMENT_SHADER, src));
  gl.bindAttribLocation(p, 0, 'a');
  gl.linkProgram(p);
  if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new GLSLError(gl.getProgramInfoLog(p));
  const u = {};
  const n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS);
  for (let i = 0; i < n; i++) {
    const info = gl.getActiveUniform(p, i);
    u[info.name.replace(/\[0\]$/, '')] = gl.getUniformLocation(p, info.name);
  }
  return { p, u };
}

let comp, down, yuv, fbo, tex, pal, palette, frameTex;
const programs = new Map();
const compiled = new Map();

function tex2(fmt, w, h) {
  const t = gl.createTexture();
  gl.bindTexture(gl.TEXTURE_2D, t);
  gl.texStorage2D(gl.TEXTURE_2D, 1, fmt, w, h);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  return t;
}

function fb(...ts) {
  const f = gl.createFramebuffer();
  gl.bindFramebuffer(gl.FRAMEBUFFER, f);
  ts.forEach((t, i) => gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0 + i, gl.TEXTURE_2D, t, 0));
  gl.drawBuffers(ts.map((_, i) => gl.COLOR_ATTACHMENT0 + i));
  return f;
}

async function init() {
  if (!gl) throw new Error('WebGL2 is not available');
  if (!gl.getExtension('EXT_color_buffer_float')) throw new Error('EXT_color_buffer_float is not available');
  gl.bindVertexArray(gl.createVertexArray());
  gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
  gl.enableVertexAttribArray(0);
  gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
  gl.disable(gl.BLEND);
  comp = program([{ name: 'composite', src: compSrc }]);
  down = program([{ name: 'downsample', src: downSrc }]);
  yuv = program([{ name: 'yuv', src: yuvSrc }]);
  const m = 16, w = W * SS + 2 * m, h = H * SS + 2 * m;
  const g0 = tex2(gl.RGBA32F, w, h), g1 = tex2(gl.RGBA16F, w, h), c = tex2(gl.RGBA8, W * SS, H * SS);
  tex = { g0, g1, c };
  fbo = { g: fb(g0, g1), c: fb(c), y: fb(tex2(gl.R8, W, H * 3 / 2)) };
  frameTex = tex2(gl.RGBA8, W, H);
  palette = JSON.parse(await text('../draw/palette.json'));
  pal = new Float32Array(48);
  Object.values(palette).forEach((h, i) => pal.set(hex(h), i * 3));
}

let libs;
async function scene(name) {
  if (programs.has(name)) return programs.get(name);
  libs ??= await Promise.all([text('../draw/sdf.glsl'), text('film.glsl')]);
  const src = await text(`scenes/${name}.glsl`);
  const m = src.match(/^#pragma frame (.*)$/m);
  if (!m) throw new GLSLError(`motion/scenes/${name}.glsl:1:1: a scene starts with a "#pragma frame {...}" line`);
  const f = JSON.parse(m[1]);
  const head = `#version 300 es\nprecision highp float;\nprecision highp int;\nuniform float T;\n${Object.keys(palette).map((k, i) => `#define ${k.toUpperCase()} ${i}.0`).join('\n')}`;
  const key = libs.join('\0') + src;
  if (!compiled.has(key)) {
    compiled.set(key, program([
      { name: 'header', src: head },
      { name: 'draw/sdf.glsl', src: libs[0] },
      { name: 'motion/film.glsl', src: libs[1] },
      { name: `motion/scenes/${name}.glsl`, src: src.replace(m[0], '') },
      { name: 'gbuffer', src: gbuf },
    ]));
  }
  const prog = compiled.get(key);
  const out = { prog, frame: { center: f.center ?? [0, 0, 0], zoom: f.zoom ?? 200, yaw: f.yaw ?? 45, pitch: f.pitch ?? 35.2644, bounds: f.bounds ?? [[-20, -20, -20], [20, 20, 20]] } };
  programs.set(name, out);
  return out;
}

function draw3(w, h) {
  gl.viewport(0, 0, w, h);
  gl.drawArrays(gl.TRIANGLES, 0, 3);
}

export function art(g, name, t, cam = {}) {
  if (!allowed.has(name)) throw new Error(`scene '${name}' must appear as a quoted literal in this shot's source, so the cache key can see it`);
  const sc = programs.get(name);
  if (!sc) throw new Error(`scene '${name}' is not compiled`);
  const fr = { ...sc.frame, ...Object.fromEntries(Object.entries(cam).filter(([, v]) => v !== undefined)) };
  const w = cam.w ?? W, h = cam.h ?? H, line = cam.line ?? 2.4;
  const R = line * SS / 2, Mg = Math.ceil(R) + 2, px = 1 / (fr.zoom * SS), rad = Math.PI / 180;
  const yaw = fr.yaw * rad, pitch = fr.pitch * rad;
  const fwd = [-Math.cos(pitch) * Math.sin(yaw), -Math.sin(pitch), -Math.cos(pitch) * Math.cos(yaw)];
  const right = [Math.cos(yaw), 0, -Math.sin(yaw)];
  const up = [right[1] * fwd[2] - right[2] * fwd[1], right[2] * fwd[0] - right[0] * fwd[2], right[0] * fwd[1] - right[1] * fwd[0]];
  const hc = [Math.sin(yaw), 0, Math.cos(yaw)];
  const norm = v => { const l = Math.hypot(...v); return v.map(x => x / l); };
  const leftN = norm([hc[0] - right[0], 0, hc[2] - right[2]]), rightN = norm([hc[0] + right[0], 0, hc[2] + right[2]]);

  gl.bindFramebuffer(gl.FRAMEBUFFER, fbo.g);
  gl.useProgram(sc.prog.p);
  const u = sc.prog.u;
  gl.uniform3fv(u.uCenter, fr.center);
  gl.uniform3fv(u.uRight, right);
  gl.uniform3fv(u.uUp, up);
  gl.uniform3fv(u.uFwd, fwd);
  gl.uniform3fv(u.uBmin, fr.bounds[0]);
  gl.uniform3fv(u.uBmax, fr.bounds[1]);
  gl.uniform2f(u.uOrigin, -Mg, -Mg);
  gl.uniform2f(u.uHalf, w * SS / 2, h * SS / 2);
  gl.uniform1f(u.uPx, px);
  gl.uniform1f(u.T, t);
  draw3(w * SS + 2 * Mg, h * SS + 2 * Mg);

  gl.bindFramebuffer(gl.FRAMEBUFFER, fbo.c);
  gl.useProgram(comp.p);
  const c = comp.u;
  gl.activeTexture(gl.TEXTURE0);
  gl.bindTexture(gl.TEXTURE_2D, tex.g0);
  gl.activeTexture(gl.TEXTURE1);
  gl.bindTexture(gl.TEXTURE_2D, tex.g1);
  gl.uniform1i(c.g0, 0);
  gl.uniform1i(c.g1, 1);
  gl.uniform2i(c.uOff, Mg, Mg);
  gl.uniform1f(c.uR, R);
  gl.uniform1f(c.uPx, px);
  gl.uniform1f(c.uCos, Math.cos(30 * rad));
  gl.uniform1f(c.uTol, 0.5 / fr.zoom);
  gl.uniform3fv(c.uRight, right);
  gl.uniform3fv(c.uUp, up);
  gl.uniform3fv(c.uFwd, fwd);
  gl.uniform3fv(c.uLeftN, leftN);
  gl.uniform3fv(c.uRightN, rightN);
  gl.uniform3fv(c.uPal, pal);
  gl.uniform3fv(c.uShade1, [0.9, 0.92, 0.97]);
  gl.uniform3fv(c.uShade2, [0.78, 0.81, 0.9]);
  gl.uniform1f(c.uShade, 0);
  draw3(w * SS, h * SS);

  gl.bindFramebuffer(gl.FRAMEBUFFER, null);
  gl.useProgram(down.p);
  gl.bindTexture(gl.TEXTURE_2D, tex.c);
  gl.activeTexture(gl.TEXTURE0);
  gl.bindTexture(gl.TEXTURE_2D, tex.c);
  gl.uniform1i(down.u.c, 0);
  gl.uniform1i(down.u.uS, SS);
  gl.clearColor(0, 0, 0, 0);
  gl.clear(gl.COLOR_BUFFER_BIT);
  draw3(w, h);
  g.drawImage(glc, 0, H - h, w, h, cam.x ?? 0, cam.y ?? 0, w, h);
}

export const shots = new Map();
let allowed = new Set();
let filmV = null;
let loads = 0;
export function register(name, dur, enter, build) {
  shots.set(name, { name, dur, enter, build, tl: null, drawFn: null });
}

const out = Object.assign(document.createElement('canvas'), { width: W, height: H });
const octx = out.getContext('2d', { alpha: false });
const layer = Object.assign(document.createElement('canvas'), { width: W, height: H });
const lctx = layer.getContext('2d', { alpha: false });
const old = Object.assign(document.createElement('canvas'), { width: W, height: H });
const octx2 = old.getContext('2d', { alpha: false });

async function film(v) {
  if (filmV === v) return;
  shots.clear();
  programs.clear();
  libs = null;
  await import(`./film.js?v=${v}&n=${++loads}`);
  filmV = v;
}

export const hooks = { build: null };

function ready(s) {
  if (s.drawFn) return;
  const r = hooks.build(s);
  s.tl = r.tl;
  s.drawFn = r.draw;
  if (!s.tl.getChildren().length) throw new Error(`shot '${s.name}' has no motion: add a tween, a type reveal or a camera move`);
}

function play(s, g, t) {
  s.tl.seek(t, false);
  s.drawFn(g, t);
}

function enter(kind, u, g) {
  if (kind === 'left') g.translate(W * (1 - u), 0);
  if (kind === 'right') g.translate(-W * (1 - u), 0);
  if (kind === 'up') g.translate(0, H * (1 - u));
  if (kind === 'down') g.translate(0, -H * (1 - u));
  if (kind === 'iris') {
    g.beginPath();
    g.arc(W / 2, H / 2, Math.hypot(W, H) / 2 * u, 0, Math.PI * 2);
    g.clip();
  }
}

function compose(job) {
  try {
    layers(job);
  } catch (e) {
    if (!(e instanceof GLSLError)) e.message = `shot ${job.shot} at +${job.t.toFixed(3)}s: ${e.message}`;
    throw e;
  }
}

function layers(job) {
  const s = shots.get(job.shot);
  if (!s) throw new Error(`film.js has no shot named ${job.shot}`);
  allowed = new Set(job.scenes);
  ready(s);
  play(s, lctx, job.t);
  if (job.enter && job.prev && job.t < TR) {
    const p = shots.get(job.prev);
    allowed = new Set(job.prevScenes);
    ready(p);
    play(p, octx2, job.prevDur);
    octx.drawImage(old, 0, 0);
    octx.save();
    const x = 1 - (1 - job.t / TR) ** 4;
    enter(job.enter, x, octx);
    octx.drawImage(layer, 0, 0);
    octx.restore();
  } else {
    octx.drawImage(layer, 0, 0);
  }
}

const yuvBuf = new Uint8Array(W * H * 3 / 2);

function readback() {
  gl.bindFramebuffer(gl.FRAMEBUFFER, null);
  gl.activeTexture(gl.TEXTURE0);
  gl.bindTexture(gl.TEXTURE_2D, frameTex);
  gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, gl.RGBA, gl.UNSIGNED_BYTE, out);
  gl.bindFramebuffer(gl.FRAMEBUFFER, fbo.y);
  gl.useProgram(yuv.p);
  gl.uniform1i(yuv.u.c, 0);
  draw3(W, H * 3 / 2);
  gl.readPixels(0, 0, W, H * 3 / 2, gl.RED, gl.UNSIGNED_BYTE, yuvBuf);
  return yuvBuf;
}

const blob = (c, type = 'image/png') => new Promise(r => c.toBlob(r, type));

async function run(job) {
  if (job.kind === 'compile') {
    await film(job.v);
    for (const n of job.scenes) await scene(n);
    await post('/done', '{}');
    return;
  }
  await film(job.v);
  for (const n of [...(job.scenes ?? []), ...(job.prevScenes ?? [])]) await scene(n);
  if (job.kind === 'frame') {
    const t0 = performance.now();
    compose(job);
    const t1 = performance.now();
    const b = readback();
    const t2 = performance.now();
    await fetch(`/push?w=${id}&shot=${job.shot}&i=${job.i}`, { method: 'POST', body: b, headers: { 'x-draw': `${t1 - t0}`, 'x-read': `${t2 - t1}`, 'x-post': `${last}` } });
    last = performance.now() - t2;
    return;
  }
  if (job.kind === 'png') {
    compose(job);
    await fetch(`/image?name=${job.name}`, { method: 'POST', body: await blob(out) });
    return;
  }
  if (job.kind === 'sheet') {
    const cols = Math.ceil(Math.sqrt(job.items.length * 1.2)), tw = 640, th = 360;
    const rows = Math.ceil(job.items.length / cols);
    const sheet = Object.assign(document.createElement('canvas'), { width: cols * tw, height: rows * th });
    const sg = sheet.getContext('2d', { alpha: false, willReadFrequently: true });
    for (const [k, it] of job.items.entries()) {
      for (const n of [...it.scenes, ...it.prevScenes]) await scene(n);
      compose(it);
      const x = (k % cols) * tw, y = Math.floor(k / cols) * th;
      sg.drawImage(out, x, y, tw, th);
      sg.fillStyle = '#0b0d12';
      sg.fillRect(x, y + th - 26, tw, 26);
      sg.fillStyle = '#ffffff';
      sg.font = '500 18px "Inter Tight"';
      sg.fillText(`${it.at.toFixed(2)}s  ${it.shot}  +${it.t.toFixed(2)}s`, x + 8, y + th - 7);
    }
    await fetch(`/image?name=${job.name}`, { method: 'POST', body: await blob(sheet) });
  }
}

const id = new URLSearchParams(location.search).get('w');
const version = new URLSearchParams(location.search).get('e');
let last = 0;
const post = (p, body) => fetch(`${p}${p.includes('?') ? '&' : '?'}w=${id}`, { method: 'POST', body });

async function main() {
  try {
    await init();
    await import('./kit.js');
    const dbg = gl.getExtension('WEBGL_debug_renderer_info');
    const renderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : 'unknown';
    await post('/hello', JSON.stringify({ renderer }));
  } catch (e) {
    await post('/hello', JSON.stringify({ error: e.message }));
    return;
  }
  for (;;) {
    const job = await (await fetch(`/next?w=${id}`)).json();
    if (job.engine && job.engine !== version) {
      location.replace(`index.html?w=${id}&e=${job.engine}`);
      return;
    }
    try {
      await run(job);
    } catch (e) {
      const glsl = e instanceof GLSLError;
      const stack = (e.stack ?? '').split('\n').filter(l => /^\s*at .*film\.js/.test(l)).slice(0, 2).map(l => l.trim().replace(/https?:\/\/[^/]+\/motion\//, 'motion/').replace(/\?[^:]+/, '')).join('\n');
      await post('/fail', JSON.stringify({ message: glsl ? `GLSL error\n${e.message}` : `${e.message}\n${stack}`.trim(), film: e instanceof SyntaxError || /import|dynamically/i.test(e.message) }));
    }
  }
}

main();
