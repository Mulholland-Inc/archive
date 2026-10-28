const base = new URL('./', import.meta.url);

async function text(path) {
  const r = await fetch(new URL(path, base), { cache: 'no-store' });
  if (!r.ok) throw new Error(`${path}: ${r.status}`);
  return r.text();
}

export async function listScenes() {
  const html = await text('scenes/');
  return [...new Set([...html.matchAll(/href="([\w-]+)\.glsl"/g)].map(m => m[1]))].sort();
}

export async function loadPalette() {
  return JSON.parse(await text('palette.json'));
}

export async function loadSources(name) {
  const [lib, scene, palette] = await Promise.all([text('sdf.glsl'), text(`scenes/${name}.glsl`), text('palette.json')]);
  return { lib, scene, palette: JSON.parse(palette) };
}

function parseFrame(scene) {
  const m = scene.match(/^#pragma frame (.*)$/m);
  if (!m) throw new Error('scene has no "#pragma frame {...}" line');
  const f = JSON.parse(m[1]);
  return {
    size: f.size,
    center: f.center ?? [0, 0, 0],
    zoom: f.zoom ?? 200,
    yaw: f.yaw ?? 45,
    pitch: f.pitch ?? 35.2644,
    bounds: f.bounds ?? [[-20, -20, -20], [20, 20, 20]],
    shade: f.shade ?? 0,
    body: scene.replace(m[0], ''),
  };
}

function hex(h) {
  const n = parseInt(h.slice(1), 16);
  return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
}

const vsrc = `#version 300 es
in vec2 a;
void main() { gl_Position = vec4(a, 0.0, 1.0); }`;

const gbufMain = `
uniform vec3 uCenter, uRight, uUp, uFwd, uBmin, uBmax;
uniform vec2 uOrigin, uHalf;
uniform float uPx, uAO;
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
  vec3 n = normalAt(p, max(uPx * 0.5, 2e-5));
  float ao = 1.0;
  if (uAO > 0.0) {
    float occ = 0.0;
    for (int i = 1; i <= 5; i++) {
      float s = 0.03 * float(i);
      occ += (s - map(p + n * s).x) / s;
    }
    ao = clamp(1.0 - occ * 0.2, 0.0, 1.0);
  }
  o0 = vec4(n, t);
  o1 = vec4(h.y, ao, 1.0, 0.0);
}`;

const compSrc = `#version 300 es
precision highp float;
precision highp int;
uniform highp sampler2D g0;
uniform highp sampler2D g1;
uniform ivec2 uOff;
uniform float uR, uPx, uCos, uTol, uAO, uSolid;
uniform vec3 uRight, uUp, uFwd, uLeftN, uRightN;
uniform vec3 uPal[16];
uniform vec3 uShade1, uShade2;
uniform float uShade;
uniform vec3 uBg;
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
  if (ah) {
    vec3 c = colour(a1.x, at);
    if (uAO > 0.0) c *= mix(1.0, a1.y, uAO);
    o = vec4(mix(c, inkc, ink), 1.0);
  } else if (uSolid > 0.5) {
    o = vec4(mix(uBg, inkc, ink), 1.0);
  } else {
    o = vec4(inkc * ink, ink);
  }
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

export class GLSLError extends Error {}

function annotate(log, parts) {
  return log.replace(/ERROR: 0:(\d+):/g, (all, n) => {
    let line = +n;
    for (const p of parts) {
      if (line <= p.lines) return `${p.name}:${line}:`;
      line -= p.lines;
    }
    return all;
  });
}

export class Renderer {
  constructor() {
    this.canvas = document.createElement('canvas');
    const gl = this.canvas.getContext('webgl2', { antialias: false, premultipliedAlpha: false, preserveDrawingBuffer: true });
    if (!gl) throw new Error('WebGL2 is not available');
    if (!gl.getExtension('EXT_color_buffer_float')) throw new Error('EXT_color_buffer_float is not available');
    this.gl = gl;
    gl.bindVertexArray(gl.createVertexArray());
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    this.comp = this.program([{ name: 'composite', src: compSrc }]);
    this.down = this.program([{ name: 'downsample', src: downSrc }]);
    this.cache = new Map();
    this.cap = 0;
  }

  program(parts) {
    const gl = this.gl;
    const src = parts.map(p => p.src).join('\n');
    const sh = (type, s) => {
      const o = gl.createShader(type);
      gl.shaderSource(o, s);
      gl.compileShader(o);
      if (!gl.getShaderParameter(o, gl.COMPILE_STATUS)) throw new GLSLError(annotate(gl.getShaderInfoLog(o), parts.map(p => ({ name: p.name, lines: p.src.split('\n').length }))));
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
      const key = info.name.replace(/\[0\]$/, '');
      u[key] = gl.getUniformLocation(p, info.name);
    }
    return { p, u };
  }

  sceneProgram(src) {
    const key = src.lib + '\0' + src.scene;
    if (this.cache.has(key)) return this.cache.get(key);
    const names = Object.keys(src.palette);
    const head = '#version 300 es\nprecision highp float;\nprecision highp int;\n' + names.map((k, i) => `#define ${k.toUpperCase()} ${i}.0`).join('\n');
    const frame = parseFrame(src.scene);
    const prog = this.program([
      { name: 'header', src: head },
      { name: 'sdf.glsl', src: src.lib },
      { name: 'scene', src: frame.body },
      { name: 'gbuffer', src: gbufMain },
    ]);
    const out = { prog, frame };
    this.cache.clear();
    this.cache.set(key, out);
    return out;
  }

  target(size) {
    const gl = this.gl;
    if (this.cap >= size) return;
    for (const t of this.textures ?? []) gl.deleteTexture(t);
    for (const f of this.fbos ?? []) gl.deleteFramebuffer(f);
    const tex = (fmt, w, h) => {
      const t = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texStorage2D(gl.TEXTURE_2D, 1, fmt, w, h);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      return t;
    };
    const g0 = tex(gl.RGBA32F, size, size);
    const g1 = tex(gl.RGBA16F, size, size);
    const c = tex(gl.RGBA8, size, size);
    const d = tex(gl.RGBA8, size, size);
    const fb = (...ts) => {
      const f = gl.createFramebuffer();
      gl.bindFramebuffer(gl.FRAMEBUFFER, f);
      ts.forEach((t, i) => gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0 + i, gl.TEXTURE_2D, t, 0));
      gl.drawBuffers(ts.map((_, i) => gl.COLOR_ATTACHMENT0 + i));
      return f;
    };
    this.textures = [g0, g1, c, d];
    this.fbos = [fb(g0, g1), fb(c), fb(d)];
    this.tex = { g0, g1, c, d };
    this.cap = size;
  }

  draw(w, h) {
    const gl = this.gl;
    gl.viewport(0, 0, w, h);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }

  async render(src, opts = {}) {
    const o = { scale: 1, ss: 2, line: 1, bg: null, ao: 0, tile: 640, crease: 30, ...opts };
    const gl = this.gl;
    const { prog, frame } = this.sceneProgram(src);
    const S = o.ss;
    const W = Math.round(frame.size[0] * o.scale), H = Math.round(frame.size[1] * o.scale);
    const R = o.line * o.scale * S / 2;
    const Mg = Math.ceil(R) + 2;
    const px = 1 / (frame.zoom * o.scale * S);
    const rad = Math.PI / 180;
    const yaw = frame.yaw * rad, pitch = frame.pitch * rad;
    const toCam = [Math.cos(pitch) * Math.sin(yaw), Math.sin(pitch), Math.cos(pitch) * Math.cos(yaw)];
    const fwd = toCam.map(v => -v);
    const right = [Math.cos(yaw), 0, -Math.sin(yaw)];
    const up = [right[1] * fwd[2] - right[2] * fwd[1], right[2] * fwd[0] - right[0] * fwd[2], right[0] * fwd[1] - right[1] * fwd[0]];
    const hc = [Math.sin(yaw), 0, Math.cos(yaw)];
    const norm = v => { const l = Math.hypot(...v); return v.map(x => x / l); };
    const leftN = norm([hc[0] - right[0], 0, hc[2] - right[2]]);
    const rightN = norm([hc[0] + right[0], 0, hc[2] + right[2]]);
    const pal = new Float32Array(48);
    Object.values(src.palette).forEach((h, i) => pal.set(hex(h), i * 3));
    const tile = Math.min(o.tile, Math.floor((gl.getParameter(gl.MAX_TEXTURE_SIZE) - 2 * Mg) / S));
    this.target(tile * S + 2 * Mg);
    const out = new ImageData(W, H);
    const row = new Uint8Array(tile * tile * 4);
    gl.disable(gl.BLEND);
    for (let y0 = 0; y0 < H; y0 += tile) {
      for (let x0 = 0; x0 < W; x0 += tile) {
        const w = Math.min(tile, W - x0), h = Math.min(tile, H - y0);
        gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbos[0]);
        gl.useProgram(prog.p);
        const u = prog.u;
        gl.uniform3fv(u.uCenter, frame.center);
        gl.uniform3fv(u.uRight, right);
        gl.uniform3fv(u.uUp, up);
        gl.uniform3fv(u.uFwd, fwd);
        gl.uniform3fv(u.uBmin, frame.bounds[0]);
        gl.uniform3fv(u.uBmax, frame.bounds[1]);
        gl.uniform2f(u.uOrigin, x0 * S - Mg, y0 * S - Mg);
        gl.uniform2f(u.uHalf, W * S / 2, H * S / 2);
        gl.uniform1f(u.uPx, px);
        gl.uniform1f(u.uAO, o.ao);
        this.draw(w * S + 2 * Mg, h * S + 2 * Mg);

        gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbos[1]);
        gl.useProgram(this.comp.p);
        const c = this.comp.u;
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, this.tex.g0);
        gl.activeTexture(gl.TEXTURE1);
        gl.bindTexture(gl.TEXTURE_2D, this.tex.g1);
        gl.uniform1i(c.g0, 0);
        gl.uniform1i(c.g1, 1);
        gl.uniform2i(c.uOff, Mg, Mg);
        gl.uniform1f(c.uR, R);
        gl.uniform1f(c.uPx, px);
        gl.uniform1f(c.uCos, Math.cos(o.crease * rad));
        gl.uniform1f(c.uTol, 0.5 / (frame.zoom));
        gl.uniform1f(c.uAO, o.ao);
        gl.uniform1f(c.uSolid, o.bg ? 1 : 0);
        gl.uniform3fv(c.uBg, o.bg ? hex(o.bg) : [0, 0, 0]);
        gl.uniform3fv(c.uRight, right);
        gl.uniform3fv(c.uUp, up);
        gl.uniform3fv(c.uFwd, fwd);
        gl.uniform3fv(c.uLeftN, leftN);
        gl.uniform3fv(c.uRightN, rightN);
        gl.uniform3fv(c.uPal, pal);
        gl.uniform3fv(c.uShade1, [0.9, 0.92, 0.97]);
        gl.uniform3fv(c.uShade2, [0.78, 0.81, 0.9]);
        gl.uniform1f(c.uShade, o.shade ?? frame.shade);
        this.draw(w * S, h * S);

        gl.bindFramebuffer(gl.FRAMEBUFFER, this.fbos[2]);
        gl.useProgram(this.down.p);
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, this.tex.c);
        gl.uniform1i(this.down.u.c, 0);
        gl.uniform1i(this.down.u.uS, S);
        this.draw(w, h);
        gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, row);
        for (let j = 0; j < h; j++) {
          const dst = ((H - 1 - (y0 + j)) * W + x0) * 4;
          out.data.set(row.subarray(j * w * 4, (j + 1) * w * 4), dst);
        }
        if (o.onTile) await o.onTile(out);
      }
    }
    return out;
  }
}

export async function bake(name, opts = {}) {
  const r = new Renderer();
  const img = await r.render(await loadSources(name), opts);
  const [x, y, w, h] = opts.bg || opts.trim === false ? [0, 0, img.width, img.height] : trim(img);
  const cv = document.createElement('canvas');
  cv.width = w;
  cv.height = h;
  cv.getContext('2d').putImageData(img, -x, -y);
  return cv.toDataURL('image/png');
}

function trim(img) {
  const { width: W, height: H, data } = img;
  let x0 = W, y0 = H, x1 = -1, y1 = -1;
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      if (data[(y * W + x) * 4 + 3] > 8) {
        if (x < x0) x0 = x;
        if (x > x1) x1 = x;
        if (y < y0) y0 = y;
        if (y > y1) y1 = y;
      }
    }
  }
  if (x1 < 0) return [0, 0, W, H];
  const pad = Math.round(Math.max(x1 - x0, y1 - y0) * 0.02);
  x0 = Math.max(0, x0 - pad);
  y0 = Math.max(0, y0 - pad);
  x1 = Math.min(W - 1, x1 + pad);
  y1 = Math.min(H - 1, y1 + pad);
  return [x0, y0, x1 - x0 + 1, y1 - y0 + 1];
}
