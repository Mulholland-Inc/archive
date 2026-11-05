// Live SDF avatars: JSON primitives interpreted by one fixed shader in one shared WebGL2 context, inked like apps/draw.

const MAX = 24;
const KINDS = { ball: 0, box: 1, cone: 2, can: 3, ring: 4, eye: 5 };
const TONES = { pale: 0, mid: 1, tone: 2, dark: 3, white: 4, ink: 5 };
const PITCH = 8 * Math.PI / 180;
const REST = -16 * Math.PI / 180;
// Supersampled pixels raymarched per frame, and how far a pose must move on screen, in device pixels, to be redrawn.
const BUDGET = 200_000;
const STILL = 0.6;
const samples = (S) => (S < 48 ? 3 : 2);
const rate = (css) => (css >= 120 ? 20 : css >= 48 ? 15 : 12);

const vs = `#version 300 es
in vec2 a;
void main() { gl_Position = vec4(a, 0.0, 1.0); }`;

const gbuffer = `#version 300 es
precision highp float;
precision highp int;
uniform vec4 uP[${MAX * 5}];
uniform int uN;
uniform vec3 uRight, uUp, uFwd;
uniform float uHalf, uBound, uPx;
uniform vec2 uRes;
layout(location = 0) out vec4 o0;
layout(location = 1) out vec4 o1;

vec3 rot(vec3 v, vec4 q) { vec3 u = -q.xyz; return v + 2.0 * cross(u, cross(u, v) + q.w * v); }
float ball(vec3 q, vec3 r) { float a = length(q / r), b = length(q / (r * r)); return b < 1e-5 ? -min(r.x, min(r.y, r.z)) : a * (a - 1.0) / b; }
float box(vec3 q, vec3 b, float r) { vec3 d = abs(q) - b + r; return length(max(d, 0.0)) + min(max(d.x, max(d.y, d.z)), 0.0) - r; }
float cone(vec3 q, float r1, float h, float r2) {
  vec2 p = vec2(length(q.xz), q.y + h * 0.5);
  float b = (r1 - r2) / h, a = sqrt(max(1.0 - b * b, 0.0)), k = dot(p, vec2(-b, a));
  if (k < 0.0) return length(p) - r1;
  if (k > a * h) return length(p - vec2(0.0, h)) - r2;
  return dot(p, vec2(a, b)) - r1;
}
float can(vec3 q, float r, float h, float e) { vec2 d = vec2(length(q.xz) - r + e, abs(q.y) - h + e); return min(max(d.x, d.y), 0.0) + length(max(d, 0.0)) - e; }
float ring(vec3 q, float R, float r) { return length(vec2(length(q.xy) - R, q.z)) - r; }

// x: distance, y: which surface, so that a change of it is a line, z: tone.
vec3 map(vec3 p) {
  vec3 o = vec3(1e3, -1.0, 0.0);
  for (int i = 0; i < uN; i++) {
    vec4 a = uP[i * 5], e = uP[i * 5 + 3];
    // a.w holds the kind and, in its fraction, the part's reach. Past that it cannot change the result, so skip it.
    int kind = int(a.w);
    if (kind == 5) continue;
    float near = length(p - a.xyz) - fract(a.w) * 8.0;
    if (e.w > 0.5 ? near > e.x - o.x : near > o.x + e.x) continue;
    vec4 b = uP[i * 5 + 1], c = uP[i * 5 + 2], f = uP[i * 5 + 4];
    vec3 q = rot(p - a.xyz, c);
    float id = e.y, tone = e.z, d;
    if (kind == 0) d = ball(q, b.xyz);
    else if (kind == 1) d = box(q, b.xyz, b.w);
    else if (kind == 2) d = cone(q, b.x, b.y, b.z);
    else if (kind == 3) d = can(q, b.x, b.y, b.z);
    else d = ring(q, b.x, b.y);
    float k = e.x;
    if (e.w > 0.5) {
      float h = k > 0.0 ? max(k - abs(o.x + d), 0.0) / k : 0.0;
      float cut = max(o.x, -d) + h * h * k * 0.25;
      if (-d > o.x - h * h * k * 0.25) { o.y = id; o.z = tone; }
      o.x = cut;
    } else {
      float h = k > 0.0 ? max(k - abs(o.x - d), 0.0) / k : 0.0;
      if (d < o.x) { o.y = id; o.z = tone; }
      o.x = min(o.x, d) - h * h * k * 0.25;
    }
  }
  return o;
}

void main() {
  vec2 xy = (gl_FragCoord.xy - uRes * 0.5) * uPx;
  o0 = vec4(0.0);
  o1 = vec4(-1.0, 0.0, 0.0, 0.0);
  float r2 = dot(xy, xy), b2 = uBound * uBound;
  if (r2 >= b2) return;
  float half_ = sqrt(b2 - r2);
  vec3 ro = uRight * xy.x + uUp * xy.y - uFwd * uBound;
  float t = uBound - half_, end = uBound + half_;
  float eps = uPx * 0.4;
  vec3 h = vec3(1.0);
  bool hit = false;
  for (int i = 0; i < 36; i++) {
    h = map(ro + uFwd * t);
    if (h.x < eps) { hit = true; break; }
    t += h.x * 0.9;
    if (t > end) break;
  }
  if (!hit) return;
  vec3 p = ro + uFwd * t;
  const vec2 k = vec2(1.0, -1.0);
  float e = max(uPx * 0.6, uHalf * 2e-3);
  vec3 n = normalize(k.xyy * map(p + k.xyy * e).x + k.yyx * map(p + k.yyx * e).x + k.yxy * map(p + k.yxy * e).x + k.xxx * map(p + k.xxx * e).x);
  // Eyes are flat, painted on whatever surface lies under them. Their lids only clip the disc.
  for (int i = 0; i < uN; i++) {
    vec4 a = uP[i * 5];
    if (int(a.w) != 5) continue;
    vec4 b = uP[i * 5 + 1], c = uP[i * 5 + 2], f = uP[i * 5 + 4];
    if (rot(n, c).z < 0.05) continue;
    vec2 d = rot(p - a.xyz, c).xy / b.x;
    if (dot(d, d) > 1.0 || d.y < f.w || dot(d, vec2(sin(b.z), cos(b.z))) > f.z) continue;
    bool pupil = length(d - f.xy * 0.7) < b.y;
    h.y = uP[i * 5 + 3].y + (pupil ? 0.75 : 0.25);
    h.z = pupil ? ${TONES.ink}.0 : ${TONES.white}.0;
  }
  o0 = vec4(n, t);
  o1 = vec4(h.y, h.z, 0.0, 1.0);
}`;

const composite = `#version 300 es
precision highp float;
precision highp int;
uniform highp sampler2D g0;
uniform highp sampler2D g1;
uniform ivec2 uMax;
uniform float uR, uRi, uPx, uHalf, uTol, uCos, uStep;
uniform vec3 uRight, uUp, uFwd, uInk;
uniform vec3 uTone[6];
out vec4 o;
vec4 f0(ivec2 c) { return texelFetch(g0, clamp(c, ivec2(0), uMax), 0); }
vec4 f1(ivec2 c) { return texelFetch(g1, clamp(c, ivec2(0), uMax), 0); }
void main() {
  ivec2 pc = ivec2(gl_FragCoord.xy);
  vec4 a0 = f0(pc), a1 = f1(pc);
  bool ah = a1.w > 0.5;
  float ink = 0.0;
  int R = int(ceil(max(uR, uRi) + 0.5));
  for (int y = -R; y <= R; y++) for (int x = -R; x <= R; x++) {
    if (x == 0 && y == 0) continue;
    float dd = length(vec2(x, y));
    ivec2 qc = pc + ivec2(x, y);
    vec4 b1 = f1(qc);
    bool bh = b1.w > 0.5;
    if (ah != bh) { ink = max(ink, clamp(uR - dd + 0.5, 0.0, 1.0)); continue; }
    if (!ah || dd > uRi + 0.5) continue;
    vec4 b0 = f0(qc);
    bool e = abs(a1.x - b1.x) > 0.1 || dot(a0.xyz, b0.xyz) < uCos;
    if (!e) {
      vec3 dP = (uRight * float(x) + uUp * float(y)) * uPx + uFwd * (b0.w - a0.w);
      e = min(abs(dot(dP, a0.xyz)), abs(dot(dP, b0.xyz))) > uTol;
    }
    if (e) ink = max(ink, clamp(uRi - dd + 0.5, 0.0, 1.0));
  }
  if (!ah) { o = vec4(uInk * ink, ink); return; }
  // A valley drawn as a line: where normals converge the surface is concave, and the line tapers out with it.
  int st = int(uStep);
  ivec2 dx = ivec2(st, 0), dy = ivec2(0, st);
  vec4 l = f0(pc - dx), r = f0(pc + dx), d = f0(pc - dy), u = f0(pc + dy);
  // Well away from a part's edge only, or the valley beside a seam fattens the seam's line.
  ivec2 fx = dx * 3, fy = dy * 3;
  bool same = abs(f1(pc - fx).x - a1.x) < 0.1 && abs(f1(pc + fx).x - a1.x) < 0.1 && abs(f1(pc - fy).x - a1.x) < 0.1 && abs(f1(pc + fy).x - a1.x) < 0.1;
  if (same) {
    float span = uStep * uPx;
    float kappa = (dot(r.xyz - l.xyz, uRight) + dot(u.xyz - d.xyz, uUp)) / (2.0 * span);
    if (abs(l.w - r.w) + abs(u.w - d.w) < span * 6.0) ink = max(ink, smoothstep(5.0, 10.0, -kappa * uHalf));
  }
  o = vec4(mix(uTone[int(a1.y + 0.5)], uInk, ink), 1.0);
}`;

const down = `#version 300 es
precision highp float;
uniform sampler2D c;
uniform int uS;
out vec4 o;
void main() {
  ivec2 b = ivec2(gl_FragCoord.xy) * uS;
  vec4 s = vec4(0.0);
  for (int y = 0; y < uS; y++) for (int x = 0; x < uS; x++) s += texelFetch(c, b + ivec2(x, y), 0);
  o = s / float(uS * uS);
}`;

const quat = (x, y, z) => {
  const [cx, sx, cy, sy, cz, sz] = [Math.cos(x / 2), Math.sin(x / 2), Math.cos(y / 2), Math.sin(y / 2), Math.cos(z / 2), Math.sin(z / 2)];
  return [sx * cy * cz - cx * sy * sz, cx * sy * cz + sx * cy * sz, cx * cy * sz - sx * sy * cz, cx * cy * cz + sx * sy * sz];
};
const mul = (a, b) => [
  a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
  a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
  a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
  a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2],
];
const apply = (q, v) => {
  const [x, y, z, w] = q;
  const c = [y * v[2] - z * v[1] + w * v[0], z * v[0] - x * v[2] + w * v[1], x * v[1] - y * v[0] + w * v[2]];
  return [v[0] + 2 * (y * c[2] - z * c[1]), v[1] + 2 * (z * c[0] - x * c[2]), v[2] + 2 * (x * c[1] - y * c[0])];
};
const hex = (h) => { const n = parseInt(h.slice(1), 16); return [n >> 16 & 255, n >> 8 & 255, n & 255]; };
const mix = (a, b, k) => a.map((v, i) => (v + (b[i] - v) * k) / 255);

// A definition comes from an agent, so it is untrusted: every refusal names the field to fix, and what passes holds
// only known fields. A size's zeros mark what may be 0.
const SIZES = { ball: [1, 1, 1], box: [1, 1, 1, 0], cone: [1, 1, 1], can: [1, 1, 0], ring: [1, 1], eye: [1] };
export function check(def) {
  const errors = [];
  const num = (v, min, max) => typeof v === 'number' && Number.isFinite(v) && v >= min && v <= max;
  if (typeof def?.id !== 'string' || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(def.id) || def.id.length > 40) errors.push('id: expected a lowercase slug of at most 40 characters');
  if (typeof def?.name !== 'string' || !def.name || def.name.length > 40) errors.push('name: expected 1 to 40 characters');
  if (!Array.isArray(def?.parts) || !def.parts.length) return { ok: false, errors: [...errors, 'parts: expected a non-empty array'] };
  let count = 0, solid = 0, eyes = 0;
  const parts = def.parts.map((p, i) => {
    const at = `parts[${i}]`;
    if (!Object.hasOwn(SIZES, p?.kind)) errors.push(`${at}.kind: expected one of ${Object.keys(SIZES).join(', ')}`);
    else if (!Array.isArray(p.size) || p.size.length !== SIZES[p.kind].length || !p.size.every((v) => num(v, 0, 3))) errors.push(`${at}.size: a ${p.kind} takes ${SIZES[p.kind].length} numbers within 0..3`);
    if (!Array.isArray(p?.at) || p.at.length !== 3 || !p.at.every((v) => num(v, -3, 3))) errors.push(`${at}.at: expected [x, y, z] within -3..3`);
    if (p?.turn !== undefined && (!Array.isArray(p.turn) || p.turn.length !== 3 || !p.turn.every((v) => num(v, -360, 360)))) errors.push(`${at}.turn: expected [x, y, z] degrees`);
    if (p?.blend !== undefined && !num(p.blend, 0, 1)) errors.push(`${at}.blend: expected 0..1`);
    if (p?.tone !== undefined && !Object.hasOwn(TONES, p.tone)) errors.push(`${at}.tone: expected one of ${Object.keys(TONES).join(', ')}`);
    if (p?.part !== undefined && (typeof p.part !== 'string' || p.part.length > 40)) errors.push(`${at}.part: expected a short name`);
    count += p?.mirror ? 2 : 1;
    if (p?.kind === 'eye') eyes++;
    else if (!p?.cut) solid++;
    return { kind: p?.kind, at: p?.at, size: p?.size, turn: p?.turn, blend: p?.blend, tone: p?.tone, part: p?.part, mirror: !!p?.mirror, cut: !!p?.cut };
  });
  if (count > MAX) errors.push(`parts: ${count} after mirroring, at most ${MAX}`);
  if (!solid) errors.push('parts: nothing solid to draw');
  if (!eyes) errors.push('parts: needs an eye');
  return errors.length ? { ok: false, errors } : { ok: true, value: { id: def.id, name: def.name, parts } };
}

// Mirrors the parts out, numbers the surfaces and frames the figure from its own extent.
function prepare(def) {
  const ids = new Map();
  const parts = [];
  for (const p of def.parts) {
    const key = p.part ?? `#${parts.length}`;
    if (!ids.has(key)) ids.set(key, ids.size + 1);
    const t = (p.turn ?? [0, 0, 0]).map((v) => v * Math.PI / 180);
    // Nothing is thinner than the shader can divide by, and rounding stays within what it rounds.
    const size = [...p.size, 0, 0, 0, 0].slice(0, 4).map((v, i) => (i < SIZES[p.kind].length && SIZES[p.kind][i] > 0 ? Math.max(v, 0.005) : v));
    if (p.kind === 'can') size[2] = Math.min(size[2], size[0], size[1]);
    if (p.kind === 'box') size[3] = Math.min(size[3], size[0], size[1], size[2]);
    const one = (side) => ({
      kind: KINDS[p.kind], eye: p.kind === 'eye', side, at: [p.at[0] * side, p.at[1], p.at[2]], size,
      q: side > 0 ? quat(t[0], t[1], t[2]) : quat(t[0], -t[1], -t[2]),
      blend: p.blend ?? 0, id: ids.get(key), tone: TONES[p.tone ?? 'mid'], cut: p.cut ? 1 : 0,
    });
    parts.push(one(1));
    if (p.mirror) parts.push(one(-1));
  }
  const reach = (p) => (p.kind === 0 ? Math.max(p.size[0], p.size[1], p.size[2]) : p.kind === 1 ? Math.hypot(p.size[0], p.size[1], p.size[2]) : p.kind === 2 ? Math.max(p.size[0], p.size[2]) + p.size[1] / 2 : p.kind === 3 ? Math.hypot(p.size[0], p.size[1]) : p.kind === 4 ? p.size[0] + p.size[1] : p.size[0] * 1.13);
  const lo = [1e3, 1e3, 1e3], hi = [-1e3, -1e3, -1e3];
  for (const p of parts) {
    if (p.cut || p.eye) continue;
    const r = reach(p);
    p.at.forEach((v, i) => { lo[i] = Math.min(lo[i], v - r); hi[i] = Math.max(hi[i], v + r); });
  }
  for (const p of parts) p.reach = Math.min(reach(p) * 1.02, 7.9);
  const center = [(lo[0] + hi[0]) / 2, (lo[1] + hi[1]) / 2, (lo[2] + hi[2]) / 2];
  const fit = Math.max(...parts.filter((p) => !p.cut && !p.eye).map((p) => Math.hypot(p.at[0] - center[0], p.at[1] - center[1], p.at[2] - center[2]) + reach(p)));
  // Fit the box the viewer actually sees, leaving room to turn and for the outline.
  const half = Math.max(hi[0] - lo[0], hi[1] - lo[1]) / 2 * 1.08;
  return { parts, center, half, bound: fit * 1.05 };
}

class Stage {
  constructor() {
    this.canvas = document.createElement('canvas');
    const gl = this.canvas.getContext('webgl2', { antialias: false, alpha: true, premultipliedAlpha: true });
    if (!gl) throw new Error('WebGL2 is not available');
    if (!gl.getExtension('EXT_color_buffer_float')) throw new Error('EXT_color_buffer_float is not available');
    this.gl = gl;
    this.setup();
    this.live = new Set();
    this.pointer = null;
    this.tick = this.tick.bind(this);
    addEventListener('pointermove', (e) => { this.pointer = [e.clientX, e.clientY]; }, { passive: true });
    document.documentElement.addEventListener('pointerleave', () => { this.pointer = null; });
    this.seen = new IntersectionObserver((list) => { for (const e of list) e.target.__avatar.visible = e.isIntersecting; });
    this.canvas.addEventListener('webglcontextlost', (e) => e.preventDefault());
    this.canvas.addEventListener('webglcontextrestored', () => {
      this.setup();
      for (const av of this.live) av.dirty = true;
    });
  }

  setup() {
    const gl = this.gl;
    gl.getExtension('EXT_color_buffer_float');
    gl.bindVertexArray(gl.createVertexArray());
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    gl.disable(gl.BLEND);
    this.g = this.program(gbuffer);
    this.c = this.program(composite);
    this.d = this.program(down);
    this.cap = 0;
  }

  program(src) {
    const gl = this.gl;
    const sh = (type, s) => {
      const o = gl.createShader(type);
      gl.shaderSource(o, s);
      gl.compileShader(o);
      if (!gl.getShaderParameter(o, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(o));
      return o;
    };
    const p = gl.createProgram();
    gl.attachShader(p, sh(gl.VERTEX_SHADER, vs));
    gl.attachShader(p, sh(gl.FRAGMENT_SHADER, src));
    gl.bindAttribLocation(p, 0, 'a');
    gl.linkProgram(p);
    if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(p));
    const u = {};
    const n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS);
    for (let i = 0; i < n; i++) {
      const info = gl.getActiveUniform(p, i);
      u[info.name.replace(/\[0\]$/, '')] = gl.getUniformLocation(p, info.name);
    }
    return { p, u };
  }

  grow(size) {
    if (this.cap >= size) return;
    const gl = this.gl;
    for (const t of this.textures ?? []) gl.deleteTexture(t);
    for (const f of this.fbos ?? []) gl.deleteFramebuffer(f);
    const tex = (fmt) => {
      const t = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texStorage2D(gl.TEXTURE_2D, 1, fmt, size, size);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      return t;
    };
    const [g0, g1, c] = [tex(gl.RGBA32F), tex(gl.RGBA16F), tex(gl.RGBA8)];
    const fb = (...ts) => {
      const f = gl.createFramebuffer();
      gl.bindFramebuffer(gl.FRAMEBUFFER, f);
      ts.forEach((t, i) => gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0 + i, gl.TEXTURE_2D, t, 0));
      gl.drawBuffers(ts.map((_, i) => gl.COLOR_ATTACHMENT0 + i));
      return f;
    };
    this.textures = [g0, g1, c];
    this.fbos = [fb(g0, g1), fb(c)];
    this.tex = { g0, g1, c };
    this.cap = size;
  }

  draw(av, S) {
    const gl = this.gl;
    const ss = samples(S);
    const G = S * ss;
    this.grow(G);
    if (this.canvas.width < S) { this.canvas.width = S; this.canvas.height = S; }
    const fig = av.fig;
    const px = (fig.half * 2) / G;
    const right = [1, 0, 0];
    const up = [0, Math.cos(PITCH), -Math.sin(PITCH)];
    const fwd = [0, -Math.sin(PITCH), -Math.cos(PITCH)];

    gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbos[0]);
    gl.viewport(0, 0, G, G);
    gl.useProgram(this.g.p);
    let u = this.g.u;
    gl.uniform4fv(u.uP, av.data);
    gl.uniform1i(u.uN, fig.parts.length);
    gl.uniform3fv(u.uRight, right);
    gl.uniform3fv(u.uUp, up);
    gl.uniform3fv(u.uFwd, fwd);
    gl.uniform1f(u.uHalf, fig.half);
    gl.uniform1f(u.uBound, fig.bound);
    gl.uniform1f(u.uPx, px);
    gl.uniform2f(u.uRes, G, G);
    gl.drawArrays(gl.TRIANGLES, 0, 3);

    const css = S / av.dpr;
    const line = (0.55 + css * 0.008) * av.dpr * ss;
    gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbos[1]);
    gl.useProgram(this.c.p);
    u = this.c.u;
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.tex.g0);
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, this.tex.g1);
    gl.uniform1i(u.g0, 0);
    gl.uniform1i(u.g1, 1);
    gl.uniform2i(u.uMax, G - 1, G - 1);
    gl.uniform1f(u.uR, line / 2);
    gl.uniform1f(u.uRi, line * 0.38);
    gl.uniform1f(u.uPx, px);
    gl.uniform1f(u.uHalf, fig.half);
    gl.uniform1f(u.uTol, px * 2.5);
    gl.uniform1f(u.uCos, Math.cos(38 * Math.PI / 180));
    gl.uniform1f(u.uStep, Math.max(2, Math.round(line / 2)));
    gl.uniform3fv(u.uRight, right);
    gl.uniform3fv(u.uUp, up);
    gl.uniform3fv(u.uFwd, fwd);
    gl.uniform3fv(u.uInk, av.ink);
    gl.uniform3fv(u.uTone, av.tones);
    gl.drawArrays(gl.TRIANGLES, 0, 3);

    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, S, S);
    gl.useProgram(this.d.p);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.tex.c);
    gl.uniform1i(this.d.u.c, 0);
    gl.uniform1i(this.d.u.uS, ss);
    gl.drawArrays(gl.TRIANGLES, 0, 3);

    av.ctx.clearRect(0, 0, S, S);
    av.ctx.drawImage(this.canvas, 0, this.canvas.height - S, S, S, 0, 0, S, S);
  }

  tick(now) {
    if (!this.live.size) { this.running = false; return; }
    requestAnimationFrame(this.tick);
    const dt = Math.min((now - (this.last ?? now)) / 1000, 0.1);
    this.last = now;
    if (document.hidden || this.gl.isContextLost()) return;
    const due = [];
    for (const av of this.live) {
      if (!av.visible) continue;
      av.pose(now / 1000, dt, this.pointer);
      if (av.due(now, av.resize())) due.push(av);
    }
    // The longest waiting go first, so a frame over budget only delays the rest to the next one.
    due.sort((a, b) => a.drawn - b.drawn);
    let left = BUDGET;
    for (const av of due) {
      if (left <= 0) break;
      const S = av.canvas.width, G = S * samples(S);
      this.draw(av, S);
      av.mark(now);
      left -= G * G;
    }
    if (due.length) this.gl.flush();
  }
}

let stage;

// Where the lids clip the eye, -1 to 1 up its height, with past 1 fully open; pupil size and lid tilt; and how the figure carries itself.
const MOODS = {
  idle: { upper: 1.2, lower: -1.2, pupil: 0.56, tilt: 0, bob: 0.014, rate: 1.5 },
  thinking: { upper: 0.5, lower: -1.2, pupil: 0.5, tilt: 0.12, bob: 0.008, rate: 0.9, gaze: [0.4, 0.42] },
};

class Live {
  constructor(host, opts) {
    this.host = host;
    this.canvas = document.createElement('canvas');
    this.canvas.style.cssText = 'display:block;width:100%;height:100%';
    host.append(this.canvas);
    host.__avatar = this;
    this.ctx = this.canvas.getContext('2d');
    this.dpr = 1;
    this.visible = true;
    this.drawn = 0;
    this.seed = Math.random() * 100;
    this.look = [0, 0];
    this.gaze = [0, 0];
    this.next = 1 + Math.random() * 3;
    this.blink = 0;
    this.now = { ...MOODS.idle };
    this.update(opts);
  }

  update({ def, tint, ink, mood = 'idle' }) {
    if (def !== this.def) {
      this.def = def;
      this.fig = prepare(def);
      this.data = new Float32Array(this.fig.parts.length * 20);
      this.was = new Float32Array(this.data.length);
    }
    this.dirty = true;
    this.mood = MOODS[mood] ?? MOODS.idle;
    const t = hex(tint), k = hex(ink);
    this.ink = k.map((v) => v / 255);
    this.tones = new Float32Array([...mix(t, [255, 255, 255], 0.6), ...mix(t, t, 0), ...mix(t, k, 0.28), ...mix(t, k, 0.62), 1, 1, 1, ...this.ink]);
  }

  resize() {
    const r = this.host.getBoundingClientRect();
    this.rect = r;
    this.dpr = devicePixelRatio || 1;
    const S = Math.max(8, Math.round(r.width * this.dpr));
    if (this.canvas.width !== S) { this.canvas.width = S; this.canvas.height = S; this.dirty = true; }
    return S;
  }

  // Worth drawing again: its rate allows it and the pose moved by a visible amount since the last draw.
  due(now, S) {
    if (this.dirty) return true;
    if (now - this.drawn < 1000 / rate(S / this.dpr)) return false;
    const unit = S / (2 * this.fig.half);
    const { data, was } = this;
    for (let i = 0; i < data.length; i++) {
      const weight = i % 20 < 3 ? unit : unit * this.fig.half * 0.5;
      if (Math.abs(data[i] - was[i]) * weight > STILL) return true;
    }
    return false;
  }

  mark(now) {
    this.drawn = now;
    this.dirty = false;
    this.was.set(this.data);
  }

  // The rig only moves parts: everything per frame is data for the fixed shader.
  pose(t, dt, pointer) {
    const data = this.data, m = this.mood, now = this.now, ease = 1 - Math.exp(-dt * 9);
    for (const k of ['upper', 'lower', 'pupil', 'tilt', 'bob', 'rate']) now[k] += (m[k] - now[k]) * ease;
    const calm = matchMedia('(prefers-reduced-motion: reduce)').matches;
    let lx = 0, ly = 0;
    const r = this.rect;
    if (pointer && r && !calm) {
      const dx = pointer[0] - (r.left + r.width / 2), dy = pointer[1] - (r.top + r.height / 2);
      const reach = Math.max(r.width, 240);
      lx = Math.tanh(dx / reach);
      ly = Math.tanh(dy / reach);
    }
    const k = 1 - Math.exp(-dt * 5);
    this.look[0] += (lx - this.look[0]) * k;
    this.look[1] += (ly - this.look[1]) * k;
    const g = m.gaze ?? [lx, -ly];
    const gk = 1 - Math.exp(-dt * 16);
    this.gaze[0] += (g[0] * 0.55 - this.gaze[0]) * gk;
    this.gaze[1] += (g[1] * 0.55 - this.gaze[1]) * gk;
    this.next -= dt;
    if (this.next < 0 && !calm) { this.blink = 1; this.next = 1.8 + Math.random() * 4.5; }
    this.blink = Math.max(0, this.blink - dt / 0.15);
    const lid = Math.sin(this.blink * Math.PI);
    const s = this.seed;
    const sway = calm ? 0 : 1;
    const yaw = REST + this.look[0] * 0.5 + sway * Math.sin(t * 0.43 + s) * 0.05;
    const pitch = this.look[1] * 0.28 + sway * Math.sin(t * 0.37 + s * 2) * 0.025;
    const roll = sway * Math.sin(t * 0.31 + s * 3) * 0.022;
    const root = mul(quat(pitch, 0, 0), mul(quat(0, yaw, 0), quat(0, 0, roll)));
    const fig = this.fig;
    const bob = sway * Math.abs(Math.sin(t * now.rate + s)) * now.bob * fig.half;
    fig.parts.forEach((p, i) => {
      const at = apply(root, p.at.map((v, j) => v - fig.center[j]));
      const q = mul(root, p.q);
      const o = i * 20;
      data[o] = at[0]; data[o + 1] = at[1] + bob; data[o + 2] = at[2]; data[o + 3] = p.kind + p.reach / 8;
      if (p.eye) {
        data[o + 4] = p.size[0];
        data[o + 5] = now.pupil * (1 - lid);
        data[o + 6] = now.tilt * -p.side;
        data[o + 7] = 0;
        data[o + 16] = this.gaze[0];
        data[o + 17] = this.gaze[1];
        data[o + 18] = now.upper + (0.07 - now.upper) * lid;
        data[o + 19] = now.lower + (-0.07 - now.lower) * lid;
      } else {
        data.set(p.size, o + 4);
      }
      data.set(q, o + 8);
      data[o + 12] = p.blend; data[o + 13] = p.id; data[o + 14] = p.tone; data[o + 15] = p.cut;
    });
  }
}

export function mount(host, opts) {
  try {
    stage ??= new Stage();
  } catch {
    return null; // No WebGL2: the avatar stays empty, and the page around it works.
  }
  const st = stage;
  const av = new Live(host, opts);
  st.live.add(av);
  st.seen.observe(host);
  if (!st.running) { st.running = true; requestAnimationFrame(st.tick); }
  return {
    update: (next) => av.update(next),
    destroy: () => { st.live.delete(av); st.seen.unobserve(host); av.canvas.remove(); },
  };
}
