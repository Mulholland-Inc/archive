#pragma frame {"size":[2000,580],"center":[0.0735,0.0,0.0735],"zoom":115,"bounds":[[-12.0,-0.4,-12.0],[12.0,3.0,12.0]]}

#define R2 0.70710678

vec2 toUV(vec3 p) { return vec2(p.x - p.z, p.x + p.z) * R2; }
vec3 W(float u, float v) { return vec3((u + v) * R2, 0.0, (v - u) * R2); }
float hash(float n) { return fract(sin(n * 127.1) * 43758.5453); }
vec3 along(vec3 p, vec2 d) { d = normalize(d); return vec3(d.x * p.x + d.y * p.z, p.y, d.x * p.z - d.y * p.x); }
vec2 scaled(vec2 d, float s) { return M(d.x * s, d.y); }

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.16, 0.014, 0.11)), m);
  return seam(e, p.z + 0.11 - (0.16 - abs(p.x)) * 0.62);
}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.12, 0.006, 0.16)), m);
  vec3 q = faceY(p, 0.006);
  float bar = m == WHITE ? STEEL : WHITE;
  s = decal(s, q, sdRect(q.xz - vec2(0.0, -0.08), vec2(0.07, 0.01), 0.01), bar);
  s = decal(s, q, sdRect(q.xz - vec2(-0.02, -0.03), vec2(0.05, 0.01), 0.01), bar);
  return decal(s, q, sdRect(q.xz - vec2(0.0, 0.07), vec2(0.07, 0.04), 0.01), bar);
}

vec2 folder(vec3 p, float m) {
  vec2 f = M(sdBox(p, vec3(0.17, 0.022, 0.12)), m);
  f = U(f, M(sdBox(p - vec3(-0.08, 0.0, -0.13), vec3(0.06, 0.012, 0.02)), m));
  return seam(f, p.y - 0.006);
}

vec2 card(vec3 p) {
  vec2 c = M(sdBox(p, vec3(0.15, 0.018, 0.105)), WHITE);
  vec3 q = faceY(p, 0.018);
  c = decal(c, q, sdRect(q.xz - vec2(-0.085, 0.0), vec2(0.03, 0.08), 0.02), LIME);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, -0.035), vec2(0.07, 0.012), 0.012), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.03, 0.025), vec2(0.05, 0.012), 0.012), STEEL);
  return c;
}

vec2 head(vec3 h, float hair) {
  vec2 r = M(sdSphere(h, 0.078), WHITE + SOLID);
  r = U(r, M(max(sdSphere(h - vec3(-0.006, 0.006, 0.0), 0.084), -(h.y - 0.55 * h.x + 0.005)), hair + SOLID));
  return r;
}

vec2 person(vec3 p, float shirt, float hair) {
  vec3 m = vec3(p.x, p.y, abs(p.z));
  vec2 r = M(sdCapsule(m, vec3(0.0, 0.05, 0.055), vec3(0.0, 0.3, 0.055), 0.045), GRAPHITE + SOLID);
  r = U(r, M(sdBox(m - vec3(0.025, 0.02, 0.055), vec3(0.06, 0.02, 0.04)), GRAPHITE));
  r = U(r, M(sdCapsule(p, vec3(0.0, 0.38, 0.0), vec3(0.0, 0.52, 0.0), 0.11), shirt + SOLID));
  r = U(r, M(sdCapsule(m, vec3(0.0, 0.56, 0.13), vec3(0.05, 0.34, 0.15), 0.036), shirt + SOLID));
  r = U(r, head(p - vec3(0.0, 0.74, 0.0), hair));
  return r;
}

vec2 carrier(vec3 p, float shirt, float hair, float m) {
  vec3 q = vec3(p.x, p.y, abs(p.z));
  vec2 r = M(sdCapsule(q, vec3(0.0, 0.05, 0.055), vec3(0.0, 0.3, 0.055), 0.045), GRAPHITE + SOLID);
  r = U(r, M(sdBox(q - vec3(0.025, 0.02, 0.055), vec3(0.06, 0.02, 0.04)), GRAPHITE));
  r = U(r, M(sdCapsule(p, vec3(0.0, 0.38, 0.0), vec3(0.0, 0.52, 0.0), 0.11), shirt + SOLID));
  r = U(r, M(sdCapsule(q, vec3(0.0, 0.56, 0.13), vec3(0.17, 0.44, 0.1), 0.036), shirt + SOLID));
  r = U(r, head(p - vec3(0.0, 0.74, 0.0), hair));
  vec3 b = p - vec3(0.2, 0.47, 0.0);
  vec2 box = M(sdBox(b, vec3(0.09, 0.07, 0.11)), WHITE);
  box = seam(box, b.y - 0.035);
  r = U(r, inset(box, faceX(b, 0.09) - vec3(0.0, 0.0, 0.0), vec2(0.06, 0.018), 0.01, 0.01, m));
  return r;
}

vec2 seated(vec3 p, float shirt, float hair) {
  vec3 m = vec3(p.x, p.y, abs(p.z));
  vec2 r = M(sdCapsule(m, vec3(-0.02, 0.33, 0.055), vec3(0.17, 0.33, 0.055), 0.045), GRAPHITE + SOLID);
  r = U(r, M(sdCapsule(m, vec3(0.17, 0.33, 0.055), vec3(0.17, 0.05, 0.055), 0.042), GRAPHITE + SOLID));
  r = U(r, M(sdBox(m - vec3(0.2, 0.02, 0.055), vec3(0.06, 0.02, 0.04)), GRAPHITE));
  r = U(r, M(sdCapsule(p, vec3(-0.03, 0.44, 0.0), vec3(-0.03, 0.56, 0.0), 0.11), shirt + SOLID));
  r = U(r, M(sdCapsule(m, vec3(-0.03, 0.6, 0.13), vec3(0.2, 0.46, 0.09), 0.036), shirt + SOLID));
  r = U(r, head(p - vec3(-0.03, 0.78, 0.0), hair));
  return r;
}

vec2 chair(vec3 p) {
  vec2 r = M(sdBox(p - vec3(-0.02, 0.27, 0.0), vec3(0.13, 0.025, 0.13)), STEEL);
  r = U(r, M(sdBox(p - vec3(-0.16, 0.44, 0.0), vec3(0.025, 0.14, 0.12)), STEEL));
  r = U(r, M(sdCylinder(p - vec3(-0.02, 0.14, 0.0), 0.018, 0.12), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(-0.02, 0.02, 0.0), 0.12, 0.02), GRAPHITE + SOLID));
  return r;
}

vec2 screen(vec3 p, float k) {
  vec3 s = p - vec3(0.0, 0.62, 0.0);
  vec2 r = M(sdBox(s, vec3(0.018, 0.12, 0.19)), GRAPHITE);
  vec3 f = faceNX(s, -0.018);
  r = U(r, M(extrude(sdRect(f.xz, vec2(0.165, 0.095), 0.01), f.y - 0.004, 0.004), SKY));
  vec3 g = faceNX(s, -0.026);
  r = decal(r, g, sdRect(g.xz - vec2(-0.07, 0.04), vec2(0.06, 0.035), 0.01), k > 0.5 ? LIME : WHITE);
  r = decal(r, g, sdRect(g.xz - vec2(0.06, 0.05), vec2(0.06, 0.012), 0.01), WHITE);
  r = decal(r, g, sdRect(g.xz - vec2(0.06, 0.01), vec2(0.06, 0.012), 0.01), WHITE);
  r = decal(r, g, sdRect(g.xz - vec2(0.0, -0.05), vec2(0.13, 0.012), 0.01), k > 0.5 ? WHITE : LIME);
  r = U(r, M(sdBox(p - vec3(0.03, 0.47, 0.0), vec3(0.012, 0.04, 0.025)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.02, 0.44, 0.0), vec3(0.05, 0.008, 0.07)), GRAPHITE));
  return r;
}

vec2 desk(vec3 p, float k) {
  vec3 t = p - vec3(0.0, 0.42, 0.0);
  vec2 r = M(sdBox(t, vec3(0.17, 0.018, 0.3)), WHITE);
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.21, abs(p.z) - 0.28), vec3(0.15, 0.2, 0.015)), WHITE));
  r = U(r, screen(p - vec3(0.08, 0.0, 0.08), k));
  r = U(r, M(sdBox(p - vec3(-0.07, 0.445, 0.04), vec3(0.04, 0.007, 0.12)), STEEL));
  vec3 ps = p - vec3(-0.02, 0.44, -0.2);
  vec2 pile = M(sdBox(ps - vec3(0.0, 0.035, 0.0), vec3(0.08, 0.035, 0.065)), WHITE);
  r = U(r, stripes(pile, ps.y, 0.014));
  r = U(r, M(sdCylinder(p - vec3(-0.08, 0.475, -0.05), 0.022, 0.035), (k > 0.5 ? LILAC : WHITE) + ROUND));
  return r;
}

vec2 station(vec3 p, float shirt, float hair, float k) {
  vec2 r = chair(p);
  r = U(r, seated(p, shirt, hair));
  r = U(r, desk(p - vec3(0.4, 0.0, 0.0), k));
  return r;
}

vec2 plant(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.1 * s, 0.0), 0.08 * s, 0.1 * s), WHITE + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.205 * s, 0.0), 0.09 * s, 0.012 * s), STEEL + ROUND));
  vec3 l = polarY(rotY(p, 0.3), 5.0) - vec3(0.0, 0.22 * s, 0.0);
  float f = sdEllipsoid(rotZ(l - vec3(0.1, 0.1, 0.0) * s, -0.75), vec3(0.11, 0.025, 0.045) * s);
  vec3 l2 = polarY(rotY(p, 0.9), 3.0) - vec3(0.0, 0.22 * s, 0.0);
  f = min(f, sdEllipsoid(rotZ(l2 - vec3(0.04, 0.17, 0.0) * s, -1.2), vec3(0.13, 0.025, 0.045) * s));
  r = U(r, M(f, MINT + SOLID));
  return r;
}

vec2 bubble(vec3 p, float m, float check) {
  float d2 = min(sdRect(p.xy, vec2(0.24, 0.15), 0.1), sdSeg(p.xy, vec2(-0.08, -0.08), vec2(-0.16, -0.24)) - 0.035);
  vec2 b = M(extrude(d2, p.z, 0.025), m);
  vec3 q = vec3(p.x, p.z - 0.025, p.y);
  float mark = m == WHITE ? LIME : WHITE;
  if (check > 0.5) {
    float c = min(sdSeg(q.xz, vec2(-0.09, 0.0), vec2(-0.025, -0.065)), sdSeg(q.xz, vec2(-0.025, -0.065), vec2(0.1, 0.07))) - 0.028;
    return decal(b, q, c, mark);
  }
  return decal(b, q, length(vec2(repC(q.x, 0.11, 3.0), q.z)) - 0.03, mark);
}

vec2 table(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.42, 0.0), 0.32, 0.02), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.21, 0.0), 0.03, 0.2), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.015, 0.0), 0.15, 0.015), GRAPHITE + SOLID));
  r = U(r, sheet(rotY(p - vec3(-0.08, 0.446, 0.06), 0.4), WHITE));
  r = U(r, sheet(rotY(p - vec3(0.1, 0.446, -0.08), -0.3), SKY));
  r = U(r, M(sdBox(p - vec3(0.08, 0.452, 0.12), vec3(0.09, 0.006, 0.06)), GRAPHITE));
  return r;
}

vec2 inbox(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.42, 0.0), vec3(0.22, 0.018, 0.22)), WHITE);
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.19, p.y - 0.21, abs(p.z) - 0.19), vec3(0.018, 0.2, 0.018)), WHITE));
  r = U(r, envelope(rotY(p - vec3(-0.05, 0.452, 0.04), 0.3), WHITE));
  r = U(r, envelope(rotY(p - vec3(0.06, 0.48, -0.03), -0.5), LILAC));
  r = U(r, sheet(rotY(p - vec3(0.0, 0.505, 0.02), 1.1), WHITE));
  r = U(r, envelope(rotZ(rotY(p - vec3(-0.02, 0.535, -0.02), 0.9), 0.15), SKY));
  r = U(r, envelope(rotY(p - vec3(0.12, 0.02, 0.3), 0.7), WHITE));
  r = U(r, sheet(rotY(p - vec3(-0.18, 0.012, 0.32), -0.4), WHITE));
  return r;
}

vec2 board(vec3 f) {
  vec2 r = M(extrude(sdRect(f.xz, vec2(0.36, 0.2), 0.02), f.y - 0.01, 0.01), WHITE);
  vec3 g = f - vec3(0.0, 0.02, 0.0);
  r = decal(r, g, max(sdRect(vec2(repC(g.x + 0.12, 0.07, 4.0), g.z + 0.1 - (0.05 + 0.04 * repI(g.x + 0.12, 0.07, 4.0)) * 0.5), vec2(0.022, (0.05 + 0.04 * repI(g.x + 0.12, 0.07, 4.0)) * 0.5), 0.0), -1.0), LIME);
  r = decal(r, g, length(g.xz - vec2(0.2, 0.0)) - 0.11, SKY);
  r = decal(r, g, max(length(g.xz - vec2(0.2, 0.0)) - 0.11, min(g.x - 0.2, g.z)), LILAC);
  return r;
}

vec2 room(vec3 p, float h, float H) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.06, 0.0), vec3(h, 0.06, h)), WHITE);
  r = seam(r, p.y - 0.02);
  vec3 t = faceY(p, 0.12);
  if (abs(p.y - 0.12) < 0.01) r = stripes(stripes(r, p.x + h, 0.5), p.z + h, 0.5);
  vec3 wx = p - vec3(-h + 0.04, 0.12 + H * 0.5, 0.0);
  vec2 w = M(sdBox(wx, vec3(0.04, H * 0.5, h)), WHITE);
  vec3 wz = p - vec3(0.0, 0.12 + H * 0.5, -h + 0.04);
  w = U(w, M(sdBox(wz, vec3(h, H * 0.5, 0.04)), WHITE));
  w = seam(w, p.y - 0.12 - H + 0.06);
  vec3 fx = faceX(p, -h + 0.08);
  vec3 fz = faceZ(p, -h + 0.08);
  float wy = 0.12 + H * 0.58;
  w = inset(w, vec3(repC(fx.x - 0.35, 0.62, 3.0), fx.y, fx.z - wy), vec2(0.22, H * 0.25), 0.02, 0.02, SKY);
  w = inset(w, vec3(repC(fz.x - 0.35, 0.62, 3.0), fz.y, fz.z - wy), vec2(0.22, H * 0.25), 0.02, 0.02, SKY);
  return U(r, w);
}

vec2 cabinet(vec3 p, float k) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.5, 0.0), vec3(0.16, 0.48, 0.2)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.015, 0.0), vec3(0.15, 0.015, 0.19)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.985, 0.0), vec3(0.165, 0.015, 0.205)), WHITE));
  vec3 f = faceZ(p, 0.2);
  float row = repI(f.z - 0.53, 0.225, 4.0);
  float op = hash(k * 7.1 + 1.3) > 0.5 ? floor(hash(k * 3.3 + 0.7) * 4.0) : -1.0;
  float zc = f.z - 0.53 - (row - 1.5) * 0.225;
  for (int j = 0; j < 2; j++) {
    float rw = j == 0 ? row : clamp(row + (zc > 0.0 ? 1.0 : -1.0), 0.0, 3.0);
    if (rw == op || (j == 1 && rw == row)) continue;
    vec3 d = vec3(f.x, f.y, f.z - 0.53 - (rw - 1.5) * 0.225);
    vec2 fr = M(extrude(sdRect(d.xz, vec2(0.135, 0.098), 0.012), d.y - 0.008, 0.008), WHITE);
    vec3 fq = d - vec3(0.0, 0.016, 0.0);
    fr = U(fr, M(sdBox(fq - vec3(0.0, 0.01, -0.03), vec3(0.055, 0.01, 0.012)), STEEL));
    fr = decal(fr, fq, sdRect(fq.xz - vec2(0.0, 0.035), vec2(0.04, 0.02), 0.004), hash(k + rw * 5.0) > 0.7 ? LIME : PAPER);
    r = U(r, fr);
  }
  if (op >= 0.0) {
    vec3 e0 = vec3(f.x, f.y, f.z - 0.53 - (op - 1.5) * 0.225);
    r = Sc(r, M(max(sdRect(e0.xz, vec2(0.135, 0.098), 0.0), -(e0.y + 0.3)), GRAPHITE));
    vec3 o = e0 - vec3(0.0, 0.1, 0.0);
    vec2 tray = M(sdBox(o - vec3(0.0, 0.0, -0.02), vec3(0.13, 0.12, 0.07)), WHITE);
    vec3 tt = faceY(o, 0.05);
    tray = decal(tray, tt, sdRect(tt.xz, vec2(0.115, 0.105), 0.005), GRAPHITE);
    r = U(r, tray);
    vec3 e = e0 - vec3(0.0, 0.228, 0.0);
    vec2 fr = M(extrude(sdRect(e.xz, vec2(0.135, 0.098), 0.012), e.y, 0.008), WHITE);
    fr = U(fr, M(sdBox(e - vec3(0.0, 0.018, -0.03), vec3(0.055, 0.01, 0.012)), STEEL));
    r = U(r, fr);
    float fi = repI(o.y + 0.01, 0.07, 3.0);
    vec3 fo = vec3(o.x, repC(o.y + 0.01, 0.07, 3.0), o.z);
    float fm = fi == 0.0 ? LIME : fi == 1.0 ? SKY : LILAC;
    r = U(r, M(sdBox(fo - vec3(0.0, 0.0, 0.07), vec3(0.1, 0.014, 0.04)), fm));
    r = U(r, M(sdBox(fo - vec3(-0.05 + 0.05 * fi, 0.0, 0.12), vec3(0.03, 0.014, 0.014)), fm));
  }
  return r;
}

vec2 vault(vec3 p) {
  vec2 r = M(sdChamferBox(p - vec3(0.0, 0.44, 0.0), vec3(0.3, 0.42, 0.27), 0.03), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.28, 0.02, 0.25)), GRAPHITE));
  vec3 f = faceZ(p, 0.27);
  vec3 d = f - vec3(0.0, 0.0, 0.46);
  r = U(r, M(extrude(sdRect(d.xz, vec2(0.24, 0.34), 0.03), d.y - 0.012, 0.012), WHITE));
  vec3 c = d - vec3(-0.02, 0.024, 0.05);
  r = U(r, M(extrude(length(c.xz) - 0.13, c.y - 0.012, 0.012), LIME + SOLID));
  r = U(r, M(extrude(length(c.xz) - 0.08, c.y - 0.03, 0.012), WHITE + SOLID));
  r = U(r, M(extrude(max(abs(sdSeg(c.xz, vec2(0.0), vec2(0.0, 0.11)) ) - 0.012, -1.0), c.y - 0.03, 0.01), GRAPHITE));
  float sp = min(sdSeg(c.xz, vec2(0.0), vec2(0.095, -0.055)), sdSeg(c.xz, vec2(0.0), vec2(-0.095, -0.055)));
  r = U(r, M(extrude(min(sp, sdSeg(c.xz, vec2(0.0), vec2(0.0, 0.11))) - 0.012, c.y - 0.04, 0.008), GRAPHITE));
  r = U(r, M(extrude(length(c.xz) - 0.03, c.y - 0.05, 0.012), GRAPHITE + SOLID));
  vec3 h = d - vec3(0.17, 0.03, -0.15);
  r = U(r, M(extrude(sdRect(h.xz, vec2(0.025, 0.05), 0.01), h.y - 0.01, 0.01), GRAPHITE));
  r = U(r, led(h - vec3(0.0, 0.02, 0.0), 0.016, LIME));
  vec3 hn = vec3(f.x, f.y, f.z) - vec3(0.0, 0.0, 0.0);
  r = U(r, M(sdBox(vec3(abs(p.x - 0.0) - 0.0, p.y - 0.62, p.z - 0.29) - vec3(-0.25, 0.0, 0.0), vec3(0.02, 0.06, 0.02)), STEEL));
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.26, p.z - 0.29) - vec3(-0.25, 0.0, 0.0), vec3(0.02, 0.06, 0.02)), STEEL));
  return r;
}

vec2 helper(vec3 p, float m, float carry) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.07, 0.0), 0.15, 0.05, 0.02), GRAPHITE + ROUND);
  vec3 b = p - vec3(0.0, 0.29, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.12, 0.15, 0.12), 0.06), WHITE);
  body = paint(body, abs(b.y + 0.06) - 0.025, m);
  r = U(r, body);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.47, 0.0), 0.04, 0.03), STEEL + ROUND));
  vec3 h = p - vec3(0.0, 0.58, 0.0);
  r = U(r, M(sdRoundBox(h, vec3(0.115, 0.085, 0.14), 0.05), WHITE));
  vec3 f = faceX(h, 0.115);
  r = Sc(r, M(max(sdRect(f.xz, vec2(0.1, 0.045), 0.04), -(f.y + 0.015)), GRAPHITE));
  r = U(r, led(vec3(abs(f.x) - 0.045, f.y + 0.012, f.z - 0.005), 0.02, LIME));
  r = U(r, M(sdCylinder(p - vec3(-0.03, 0.73, 0.0), 0.008, 0.06), GRAPHITE + SOLID));
  r = U(r, led(p - vec3(-0.03, 0.8, 0.0), 0.025, m));
  vec3 q = vec3(p.x, p.y, abs(p.z));
  vec3 hand = carry > 0.5 ? vec3(0.2, 0.36, 0.1) : vec3(0.05, 0.16, 0.17);
  r = U(r, M(sdCapsule(q, vec3(0.0, 0.36, 0.14), hand, 0.032), WHITE + SOLID));
  r = U(r, M(sdSphere(q - hand, 0.04), GRAPHITE + SOLID));
  if (carry > 0.5) r = U(r, folder(p - vec3(0.22, 0.4, 0.0), m));
  return r;
}

vec2 conveyor(vec3 p, float len, float w, float h) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(q.x, q.y - h + 0.045, abs(q.z) - w), vec3(len * 0.5, 0.045, 0.022)), WHITE);
  vec3 ro = vec3(repC(q.x, 0.11, floor(len / 0.11)), q.y - h + 0.04, q.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.03, w), STEEL + SOLID));
  float n = floor(len / 0.7) + 1.0;
  float lx = repC(q.x, len / (n - 1.0) * 0.96, n);
  vec3 lq = vec3(lx, q.y - (h - 0.09) * 0.5, abs(q.z) - w);
  r = U(r, M(sdBox(lq, vec3(0.022, (h - 0.09) * 0.5, 0.022)), GRAPHITE));
  r = U(r, M(sdBox(vec3(lx, q.y - 0.14, q.z), vec3(0.016, 0.016, w)), GRAPHITE));
  float ci = repI(q.x + 0.05, 0.4, floor(len / 0.4));
  vec3 cq = vec3(repC(q.x + 0.05, 0.4, floor(len / 0.4)), q.y - h - 0.09, q.z);
  if (q.x < 0.0) r = U(r, envelope(rotY(cq - vec3(0.0, -0.005, 0.0), 0.5 * sin(ci * 2.7)), mod(ci, 2.0) < 0.5 ? WHITE : SKY));
  else r = U(r, card(cq));
  return r;
}

vec2 arm(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.22, 0.05), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.22, 0.0), 0.14, 0.13), WHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.37, 0.0), 0.17, 0.03), LIME + ROUND));
  vec3 a = vec3(0.0, 0.48, 0.0), b = vec3(-0.15, 1.35, 0.15), c = vec3(0.7, 1.0, -0.55);
  r = U(r, M(sdCapsule(p, a, b, 0.065), WHITE + SOLID));
  r = U(r, M(sdCapsule(p, b, c, 0.055), WHITE + SOLID));
  r = U(r, M(sdSphere(p - a, 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - b, 0.085), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - c, 0.07), LIME + SOLID));
  vec3 g = p - c;
  r = U(r, M(sdBox(g - vec3(0.0, -0.1, 0.0), vec3(0.08, 0.025, 0.05)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(g.x) - 0.07, g.y + 0.17, g.z), vec3(0.012, 0.07, 0.04)), GRAPHITE));
  r = U(r, card(rotZ(g - vec3(0.0, -0.25, 0.0), 0.08)));
  return r;
}

vec2 gate(vec3 p) {
  vec3 g = vec3(abs(p.x) - 0.4, p.y, p.z);
  vec2 r = M(sdBox(g - vec3(0.0, 0.6, 0.0), vec3(0.06, 0.6, 0.09)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  r = U(r, M(sdBox(g - vec3(0.0, 0.02, 0.0), vec3(0.1, 0.02, 0.13)), GRAPHITE));
  vec3 h = p - vec3(0.0, 1.27, 0.0);
  vec2 hd = M(sdRoundBox(h, vec3(0.52, 0.11, 0.14), 0.03), WHITE);
  vec3 hf = faceZ(h, 0.14);
  hd = U(hd, led(hf - vec3(-0.3, 0.0, 0.0), 0.03, LIME));
  hd = U(hd, led(hf - vec3(-0.2, 0.0, 0.0), 0.03, LIME));
  hd = vents(hd, hf - vec3(0.2, 0.0, 0.0), 4.0, 0.06, 0.1, 0.025, GRAPHITE);
  r = U(r, hd);
  vec3 e = p - vec3(0.0, 1.13, 0.0);
  r = U(r, M(sdBox(e, vec3(0.3, 0.03, 0.05)), GRAPHITE));
  r = U(r, M(max(sdBox(e - vec3(0.0, -0.32, 0.0), vec3(0.24, 0.29, 0.004)), abs(mod(p.x + 0.03, 0.08) - 0.04) - 0.012), LIME + SOLID));
  return r;
}

vec2 bin(vec3 p, float m) {
  vec2 r = M(max(sdBox(p - vec3(0.0, 0.12, 0.0), vec3(0.15, 0.12, 0.12)), -sdBox(p - vec3(0.0, 0.2, 0.0), vec3(0.13, 0.12, 0.1))), WHITE);
  r = paint(r, max(abs(p.y - 0.18) - 0.03, -1.0), m);
  vec3 fo = vec3(repC(p.x, 0.05, 4.0), p.y - 0.2, p.z);
  r = U(r, M(sdBox(fo, vec3(0.006, 0.08, 0.09)), m == LIME ? SKY : WHITE));
  return r;
}

vec2 drone(vec3 p, float k) {
  vec2 r = M(sdRoundBox(p, vec3(0.11, 0.04, 0.11), 0.03), WHITE);
  vec3 m = mirXZ(p);
  r = U(r, M(pipe(m, vec3(0.0), vec3(0.17, 0.02, 0.17), 0.016), GRAPHITE + SOLID));
  vec3 ro = m - vec3(0.17, 0.05, 0.17);
  r = U(r, M(sdCylinder(ro, 0.02, 0.03), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(ro - vec3(0.0, 0.03, 0.0), 0.12, 0.006), STEEL + SOLID));
  r = U(r, led(faceY(p, 0.04) - vec3(0.0, 0.0, 0.06), 0.025, LIME));
  r = U(r, M(pipe(m, vec3(0.06, -0.04, 0.0), vec3(0.06, -0.12, 0.0), 0.008), GRAPHITE + SOLID));
  if (k < 0.5) r = U(r, folder(p - vec3(0.0, -0.15, 0.0), LIME));
  else r = U(r, envelope(p - vec3(0.0, -0.14, 0.0), WHITE));
  return r;
}

vec2 tree(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.25 * s, 0.0), 0.03 * s, 0.25 * s), GRAPHITE + SOLID);
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.6 * s, 0.0), vec3(0.24, 0.3, 0.24) * s), MINT + SOLID));
  return r;
}

vec2 pine(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.12 * s, 0.0), 0.028 * s, 0.12 * s), GRAPHITE + SOLID);
  r = U(r, M(sdCone(p - vec3(0.0, 0.34 * s, 0.0), 0.13 * s, 0.21 * s, 0.08 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.53 * s, 0.0), 0.11 * s, 0.16 * s, 0.04 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.69 * s, 0.0), 0.1 * s, 0.11 * s, 0.0), MINT + SOLID));
  return r;
}

vec2 bench(vec3 p) {
  vec3 s = p - vec3(0.0, 0.16, 0.0);
  vec2 r = M(sdBox(s, vec3(0.26, 0.015, 0.08)), WHITE);
  r = stripes(r, s.z + 0.08, 0.04);
  r = U(r, M(sdBox(p - vec3(0.0, 0.28, -0.085), vec3(0.26, 0.05, 0.012)), WHITE));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.2, p.y - 0.08, p.z), vec3(0.015, 0.08, 0.07)), GRAPHITE));
  return r;
}

float paths(vec3 p, vec2 q) {
  float d = abs(q.y - 0.0) - 0.26;
  vec3 a = W(-5.6, 2.19), b = W(5.6, 2.19), c = W(0.0, 1.7);
  d = min(d, sdSeg(p.xz, a.xz, a.xz + vec2(0.0, 6.0)) - 0.2);
  d = min(d, sdSeg(p.xz, b.xz, b.xz + vec2(6.0, 0.0)) - 0.2);
  d = min(d, sdSeg(p.xz, c.xz, c.xz + vec2(0.0, 6.0)) - 0.2);
  return d;
}

vec2 ground(vec3 p, vec2 q) {
  vec2 r = M(max(max(p.y, -p.y - 0.3), -(q.y + 2.6)), MINT);
  vec3 top = faceY(p, 0.0);
  float pd = paths(p, q);
  r = decal(r, top, pd, WHITE);
  if (pd < 0.0 && p.y > -0.01) r = stripes(r, q.x + q.y * 0.0, 0.3);
  return r;
}

vec2 office(vec3 p) {
  vec2 r = room(p, 1.55, 0.85);
  vec3 o = p - vec3(0.0, 0.12, 0.0);
  r = U(r, board(faceX(o, -1.47) - vec3(-0.85, 0.0, 0.55)));
  if (max(o.z, o.x - 1.0) < r.x) {
    float ix = repI(o.x + 0.1, 0.7, 3.0);
    vec3 so = vec3(repC(o.x + 0.1, 0.7, 3.0), o.y, o.z + 0.4);
    float sh = ix == 0.0 ? LIME : ix == 1.0 ? WHITE : LILAC;
    float hr = ix == 0.0 ? GRAPHITE : ix == 1.0 ? CORAL : GRAPHITE;
    r = U(r, station(along(so, vec2(0.0, -1.0)), sh, hr, mod(ix, 2.0)));
  }
  if (max(o.x - 0.2, -0.1 - o.z) < r.x) {
    float iz = repI(o.z - 0.7, 0.7, 2.0);
    vec3 so = vec3(o.x + 0.5, o.y, repC(o.z - 0.7, 0.7, 2.0));
    r = U(r, station(along(so, vec2(-1.0, 0.0)), iz == 0.0 ? SKY : STEEL, iz == 0.0 ? CORAL : GRAPHITE, iz));
  }
  vec3 t = o - vec3(0.75, 0.0, 0.7);
  if (near(t - vec3(0.0, 0.45, 0.0), vec3(0.75, 0.5, 0.75), r.x)) {
    r = U(r, table(t));
    r = U(r, person(along(t - vec3(-0.5, 0.0, 0.0), vec2(1.0, 0.0)), LIME, CORAL));
    r = U(r, person(along(t - vec3(0.0, 0.0, -0.5), vec2(0.0, 1.0)), WHITE, GRAPHITE));
    r = U(r, person(along(t - vec3(0.4, 0.0, 0.35), vec2(-1.0, -0.85)), LILAC, STEEL));
  }
  r = U(r, inbox(o - vec3(1.1, 0.0, -1.05)));
  r = U(r, plant(o - vec3(-1.22, 0.0, -1.22), 1.15));
  r = U(r, plant(o - vec3(1.3, 0.0, -0.2), 0.85));
  return r;
}

vec2 hub(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.04, 0.0), vec3(1.2, 0.04, 1.2)), WHITE);
  r = seam(r, p.y - 0.02);
  if (abs(p.y - 0.08) < 0.01) r = stripes(stripes(r, p.x, 0.3), p.z, 0.3);
  vec3 o = p - vec3(0.0, 0.08, 0.0);
  r = U(r, conveyor(o - vec3(-0.95, 0.0, 0.0), 1.9, 0.17, 0.36));
  vec2 g = gate(o / 0.6); r = U(r, M(g.x * 0.6, g.y));
  r = U(r, scaled(arm((o - vec3(-0.9, 0.0, 0.33)) / 0.6), 0.6));
  r = U(r, scaled(arm(rotY(o - vec3(0.95, 0.0, -0.33), PI) / 0.6), 0.6));
  r = U(r, bin(o - vec3(0.85, 0.0, 0.45), LIME));
  r = U(r, bin(o - vec3(0.5, 0.0, 0.62), LILAC));
  r = U(r, envelope(rotY(o - vec3(-0.85, 0.014, -0.45), 0.6), WHITE));
  r = U(r, envelope(rotY(o - vec3(-0.75, 0.042, -0.5), -0.2), SKY));
  r = U(r, sheet(rotY(o - vec3(-0.6, 0.008, -0.62), 1.0), WHITE));
  return r;
}

vec2 cabRow(vec3 o, float kind, float n, float base) {
  float a = kind == 1.0 ? o.z - 0.2 : kind == 2.0 ? o.x - 0.25 : o.x;
  float c = (n - 1.0) * 0.5;
  float i0 = clamp(round(a / 0.34 + c), 0.0, n - 1.0);
  float i1 = clamp(i0 + (a / 0.34 + c > i0 ? 1.0 : -1.0), 0.0, n - 1.0);
  vec2 r = M(1e9, 0.0);
  for (int j = 0; j < 2; j++) {
    float i = j == 0 ? i0 : i1;
    if (j == 1 && i1 == i0) break;
    float l = a - (i - c) * 0.34;
    vec3 q = kind == 1.0 ? along(vec3(o.x + 1.25, o.y, l), vec2(0.0, -1.0)) : kind == 2.0 ? vec3(l, o.y, o.z - 0.25) : vec3(l, o.y, o.z + 1.25);
    r = U(r, cabinet(q, i + base));
  }
  return r;
}

vec2 archive(vec3 p) {
  vec2 r = room(p, 1.55, 1.0);
  vec3 o = p - vec3(0.0, 0.12, 0.0);
  if (o.z + 0.75 < r.x) r = U(r, cabRow(o, 0.0, 7.0, 0.0));
  if (o.x + 0.75 < r.x) r = U(r, cabRow(o, 1.0, 5.0, 20.0));
  if (max(abs(o.z - 0.25) - 0.5, max(-0.5 - o.x, o.x - 1.0)) < r.x) r = U(r, cabRow(o, 2.0, 4.0, 41.0));
  r = U(r, vault(o - vec3(1.1, 0.0, -0.55)));
  r = U(r, helper(along(o - vec3(0.42, 0.0, 0.88), vec2(0.0, -1.0)), LIME, 1.0));
  r = U(r, carrier(along(o - vec3(-0.55, 0.0, 0.95), vec2(1.0, 0.0)), SKY, GRAPHITE, LIME));
  vec3 bs = o - vec3(1.1, 0.0, 1.0);
  vec3 bq = vec3(repC(bs.x, 0.24, 2.0), bs.y, repC(bs.z, 0.18, 2.0));
  vec2 bx = M(sdBox(bq - vec3(0.0, 0.08, 0.0), vec3(0.11, 0.08, 0.08)), WHITE);
  bx = seam(bx, bq.y - 0.12);
  bx = inset(bx, faceZ(bq, 0.08) - vec3(0.0, 0.0, 0.07), vec2(0.05, 0.018), 0.012, 0.01, LIME);
  r = U(r, bx);
  vec3 bt = bs - vec3(0.0, 0.16, 0.0);
  vec2 bx2 = M(sdBox(bt - vec3(0.0, 0.08, 0.0), vec3(0.11, 0.08, 0.08)), WHITE);
  r = U(r, inset(seam(bx2, bt.y - 0.12), faceZ(bt, 0.08) - vec3(0.0, 0.0, 0.07), vec2(0.05, 0.018), 0.012, 0.01, LILAC));
  return r;
}

vec2 map(vec3 p) {
  vec2 q = toUV(p);
  vec2 r = p.y < 0.2 ? ground(p, q) : M(p.y - 0.1, MINT);
  vec3 c;
  c = W(-5.6, 0.0);
  if (near(p - c - vec3(0.0, 0.55, 0.0), vec3(1.6, 0.6, 1.6), r.x)) r = U(r, office(p - c));
  c = W(0.0, 0.0);
  if (near(p - c - vec3(0.0, 0.45, 0.0), vec3(1.25, 0.5, 1.25), r.x)) r = U(r, hub(p - c));
  vec3 bq = c + vec3(0.0, 1.3, 0.0);
  if (near(p - bq, vec3(0.3, 0.3, 0.1), r.x)) r = U(r, bubble(p - bq, WHITE, 1.0));
  c = W(5.6, 0.0);
  if (near(p - c - vec3(0.0, 0.6, 0.0), vec3(1.6, 0.65, 1.6), r.x)) r = U(r, archive(p - c));
  vec3 mb = W(-5.6, 0.0) + vec3(0.75, 1.32, 0.7);
  if (near(p - mb, vec3(0.3, 0.3, 0.1), r.x)) r = U(r, bubble(p - mb, LILAC, 0.0));
  for (int i = 0; i < 5; i++) {
    vec3 fp = i == 0 ? W(-3.6, -0.15) + vec3(0.0, 0.85, 0.0) : i == 1 ? W(-3.1, 0.15) + vec3(0.0, 1.1, 0.0) : i == 2 ? W(-2.6, -0.2) + vec3(0.0, 0.95, 0.0) : i == 3 ? W(-2.1, 0.1) + vec3(0.0, 0.78, 0.0) : W(-1.6, -0.25) + vec3(0.0, 0.62, 0.0);
    if (!near(p - fp, vec3(0.22, 0.2, 0.22), r.x)) continue;
    vec3 l = rotX(rotZ(rotY(p - fp, float(i) * 1.7), 0.35 * sin(float(i) * 2.1)), 0.3 * cos(float(i) * 1.3));
    r = U(r, mod(float(i), 2.0) < 0.5 ? envelope(l, i == 2 ? SKY : WHITE) : sheet(l, WHITE));
  }
  for (int i = 0; i < 3; i++) {
    vec3 dq = i == 0 ? W(2.5, -0.6) + vec3(0.0, 1.35, 0.0) : i == 1 ? W(3.4, 0.35) + vec3(0.0, 1.05, 0.0) : W(-8.2, -0.5) + vec3(0.0, 1.25, 0.0);
    if (near(p - dq - vec3(0.0, -0.08, 0.0), vec3(0.3, 0.2, 0.3), r.x)) r = U(r, drone(p - dq, i == 2 ? 1.0 : 0.0));
  }
  vec3 h1 = W(2.4, 0.08);
  if (near(p - h1 - vec3(0.0, 0.42, 0.0), vec3(0.32, 0.45, 0.32), r.x)) r = U(r, helper(along(p - h1, vec2(1.0, -1.0)), LILAC, 1.0));
  vec3 h2 = W(-2.7, 0.08);
  if (near(p - h2 - vec3(0.0, 0.42, 0.0), vec3(0.32, 0.45, 0.32), r.x)) r = U(r, helper(along(p - h2, vec2(1.0, -1.0)), LIME, 0.0));
  vec3 pw = W(-7.9, 0.08);
  if (near(p - pw - vec3(0.0, 0.42, 0.0), vec3(0.35, 0.45, 0.35), r.x)) r = U(r, carrier(along(p - pw, vec2(1.0, -1.0)), LILAC, GRAPHITE, LIME));
  vec3 pw2 = W(-5.6, 2.19) + vec3(-0.0, 0.0, 0.6);
  if (near(p - pw2 - vec3(0.0, 0.42, 0.0), vec3(0.35, 0.45, 0.35), r.x)) r = U(r, carrier(along(p - pw2, vec2(0.0, -1.0)), WHITE, CORAL, SKY));
  vec3 pw3 = W(5.6, 2.19) + vec3(0.75, 0.0, 0.0);
  if (near(p - pw3 - vec3(0.0, 0.42, 0.0), vec3(0.35, 0.45, 0.35), r.x)) r = U(r, carrier(along(p - pw3, vec2(-1.0, 0.0)), LIME, STEEL, WHITE));
  for (int i = 0; i < 6; i++) {
    vec4 t = i == 0 ? vec4(-8.6, 2.0, 1.1, 0.0) : i == 1 ? vec4(-2.6, 2.3, 1.0, 1.0) : i == 2 ? vec4(2.7, 2.2, 1.0, 0.0) : i == 3 ? vec4(8.5, 1.8, 1.1, 1.0) : i == 4 ? vec4(-2.7, -2.0, 1.0, 0.0) : vec4(2.8, -2.0, 1.0, 1.0);
    vec3 l = p - W(t.x, t.y);
    if (near(l - vec3(0.0, 0.45 * t.z, 0.0), vec3(0.28, 0.5, 0.28) * t.z, r.x)) r = U(r, t.w > 0.5 ? pine(l, t.z) : tree(l, t.z));
  }
  vec3 bb = W(-1.5, 2.4);
  if (near(p - bb - vec3(0.0, 0.18, 0.0), vec3(0.3, 0.2, 0.3), r.x)) r = U(r, bench(p - bb));
  return r;
}
