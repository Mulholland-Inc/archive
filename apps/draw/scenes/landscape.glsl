#pragma frame {"size":[2000,580],"center":[-0.45,0.19,-0.45],"zoom":118,"bounds":[[-12.0,-0.4,-12.0],[12.0,3.2,12.0]]}

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

vec2 stack(vec3 p, vec2 h, float n, float m) {
  vec2 s = M(sdBox(p - vec3(0.0, n * 0.5, 0.0), vec3(h.x, n * 0.5, h.y)), WHITE);
  s = stripes(s, p.y, 0.018);
  return decal(s, faceY(p, n), sdRect(p.xz - vec2(h.x * 0.3, 0.0), vec2(h.x * 0.45, h.y * 0.12), 0.005), m);
}

vec2 tube(vec3 p, vec3 a, vec3 b, float r, float off) {
  vec3 ab = b - a;
  float t = clamp(dot(p - a, ab) / dot(ab, ab), 0.0, 1.0);
  float l = t * length(ab);
  vec2 g = M(length(p - a - ab * t) - r, SKY + SOLID);
  g = paint(g, abs(mod(l + off, 0.9) - 0.45) - 0.12, LIME + SOLID);
  if (g.y == LIME + SOLID) g = seam(g, abs(mod(l + off, 0.9) - 0.45) - 0.06);
  float c = abs(mod(l + 0.2, 0.45) - 0.225) - 0.025;
  g = U(g, M(max(length(p - a - ab * t) - r - 0.025, max(c, max(-l + 0.01, l - length(ab) + 0.01))), STEEL + SOLID));
  return g;
}

vec2 drawers(vec3 f, vec2 c0, vec2 pitch, vec2 n, vec2 h, vec2 skip, float k) {
  vec2 g = (f.xz - c0) / pitch + (n - 1.0) * 0.5;
  vec2 i0 = clamp(floor(g + 0.5), vec2(0.0), n - 1.0);
  vec2 s = vec2(g.x >= i0.x ? 1.0 : -1.0, g.y >= i0.y ? 1.0 : -1.0);
  vec2 r = M(1e9, 0.0);
  for (int j = 0; j < 4; j++) {
    vec2 idx = clamp(i0 + vec2(mod(float(j), 2.0), floor(float(j) / 2.0)) * s, vec2(0.0), n - 1.0);
    if (idx == skip) continue;
    vec2 cc = c0 + (idx - (n - 1.0) * 0.5) * pitch;
    vec3 d = vec3(f.x - cc.x, f.y, f.z - cc.y);
    vec2 pl = M(extrude(sdRect(d.xz, h, 0.015), d.y - 0.012, 0.012), WHITE);
    vec3 dq = d - vec3(0.0, 0.024, 0.0);
    float lit = hash(k + idx.x * 7.0 + idx.y * 13.0);
    pl = decal(pl, dq, sdRect(dq.xz - vec2(0.0, h.y * 0.42), vec2(h.x * 0.28, h.y * 0.2), 0.006), lit > 0.72 ? LIME : PAPER);
    pl = U(pl, M(sdBox(dq - vec3(0.0, 0.012, -h.y * 0.18), vec3(h.x * 0.4, 0.012, 0.022)), STEEL));
    r = U(r, pl);
  }
  return r;
}

vec2 pulled(vec3 d, vec2 h, float ext, float k) {
  vec2 r = M(sdBox(d - vec3(0.0, ext * 0.5, -h.y * 0.25), vec3(h.x * 0.94, ext * 0.5, h.y * 0.68)), WHITE);
  vec3 e = d - vec3(0.0, ext, 0.0);
  r = U(r, M(extrude(sdRect(e.xz, h, 0.015), e.y - 0.012, 0.012), WHITE));
  r = U(r, M(sdBox(e - vec3(0.0, 0.036, -h.y * 0.18), vec3(h.x * 0.4, 0.012, 0.022)), STEEL));
  float top = h.y * 0.43;
  float n = 4.0;
  float fi = repI(d.y - ext * 0.5, ext / (n + 0.6), n);
  vec3 fo = vec3(d.x, repC(d.y - ext * 0.5, ext / (n + 0.6), n), d.z - top);
  float fm = fi == 0.0 ? LIME : fi == 1.0 ? SKY : fi == 2.0 ? LILAC : WHITE;
  r = U(r, M(sdBox(fo, vec3(h.x * 0.85, 0.016, h.y * 0.32)), fm));
  r = U(r, M(sdBox(fo - vec3(-h.x * 0.5 + h.x * 0.33 * fi, 0.0, h.y * 0.38), vec3(h.x * 0.2, 0.016, h.y * 0.1)), fm));
  return r;
}

vec2 mailroom(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.47, 0.0), vec3(0.85, 0.47, 0.7)), WHITE);
  r = paint(r, p.y - 0.08, STEEL);
  r = seam(r, p.y - 0.84);
  vec3 t = p - vec3(0.0, 0.97, 0.0);
  r = U(r, M(max(sdBox(t, vec3(0.87, 0.04, 0.72)), -sdBox(t - vec3(0.0, 0.04, 0.0), vec3(0.81, 0.06, 0.66))), WHITE));
  vec3 fz = faceZ(p, 0.7);
  vec3 s = fz - vec3(-0.2, 0.0, 0.55);
  r = Sc(r, M(max(sdRect(s.xz, vec2(0.4, 0.07), 0.0), -(s.y + 0.2)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(s.xz, vec2(0.48, 0.14), 0.04), -sdRect(s.xz, vec2(0.4, 0.07), 0.0)), s.y - 0.035, 0.035), LIME));
  r = U(r, envelope(rotX(p - vec3(-0.42, 0.55, 0.78), 0.12), WHITE));
  r = U(r, envelope(rotX(rotY(p - vec3(-0.05, 0.56, 0.76), 0.15), 0.1), SKY));
  r = inset(r, fz - vec3(0.55, 0.0, 0.3), vec2(0.13, 0.27), 0.02, 0.03, GRAPHITE);
  r = inset(r, fz - vec3(0.55, 0.0, 0.73), vec2(0.13, 0.05), 0.02, 0.02, SKY);
  vec3 fx = faceX(p, 0.85);
  vec3 g = fx - vec3(0.0, 0.0, 0.48);
  r = U(r, M(extrude(sdRect(g.xz, vec2(0.52, 0.3), 0.02), g.y - 0.03, 0.03), WHITE));
  vec2 ci = vec2(repI(g.x, 0.2, 5.0), repI(g.z, 0.19, 3.0));
  vec3 cq = vec3(repC(g.x, 0.2, 5.0), g.y, repC(g.z, 0.19, 3.0));
  r = Sc(r, M(max(sdRect(cq.xz, vec2(0.08, 0.075), 0.0), -(cq.y + 0.04)), GRAPHITE));
  float h = hash(ci.x * 3.0 + ci.y * 11.0);
  if (h > 0.3) {
    float em = h > 0.8 ? LIME : h > 0.6 ? SKY : WHITE;
    r = U(r, M(max(sdBox(cq - vec3(0.0, 0.0, -0.01 - 0.02 * h), vec3(0.06, 0.06, 0.04 + 0.02 * h)), -cq.y - 0.04), em));
  }
  vec3 hq = p - vec3(-0.35, 1.01, 0.2);
  r = U(r, M(sdCylinder(hq - vec3(0.0, 0.08, 0.0), 0.13, 0.08), STEEL + ROUND));
  vec3 cq2 = hq - vec3(0.0, 0.42, 0.0);
  float outer = sdCone(cq2, 0.26, 0.14, 0.5);
  float inner = sdCone(cq2 - vec3(0.0, 0.035, 0.0), 0.26, 0.105, 0.465);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq2 - vec3(0.0, 0.26, 0.0), 0.5, 0.028), LIME + SOLID));
  r = U(r, envelope(rotZ(rotY(cq2 - vec3(0.06, 0.12, 0.03), 0.6), 0.4), WHITE));
  r = U(r, sheet(rotX(rotY(cq2 - vec3(-0.12, 0.14, -0.08), -0.3), -0.45), SKY));
  vec3 th = p - vec3(0.45, 1.01, -0.35);
  r = U(r, M(sdBox(th - vec3(0.0, 0.14, 0.0), vec3(0.17, 0.14, 0.17)), WHITE));
  r = U(r, M(sdBox(th - vec3(0.0, 0.3, 0.0), vec3(0.2, 0.025, 0.2)), WHITE));
  r = U(r, led(faceZ(th, 0.17) - vec3(0.08, 0.0, 0.16), 0.025, LIME));
  return r;
}

vec2 trays(vec3 p) {
  vec3 l = mirXZ(p) - vec3(0.27, 0.8, 0.2);
  vec2 r = M(sdBox(l, vec3(0.02, 0.8, 0.02)), GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.32, 0.02, 0.25)), GRAPHITE));
  float i = repI(p.y - 0.95, 0.3, 5.0);
  vec3 q = vec3(p.x, repC(p.y - 0.95, 0.3, 5.0), p.z);
  vec2 tr = M(max(sdBox(q - vec3(0.0, -0.08, 0.0), vec3(0.29, 0.06, 0.22)), -sdBox(q - vec3(0.0, -0.04, 0.04), vec3(0.26, 0.06, 0.22))), i == 2.0 ? LIME : WHITE);
  r = U(r, tr);
  float hh = 0.03 + 0.05 * hash(i + 4.0);
  r = U(r, stack(q - vec3(0.0, -0.12, 0.0), vec2(0.2, 0.15), hh, i == 1.0 ? LILAC : SKY));
  r = U(r, envelope(rotY(q - vec3(0.02, -0.12 + hh + 0.014, 0.02), 0.3 * sin(i * 3.1)), mod(i, 2.0) < 0.5 ? WHITE : SKY));
  r = U(r, M(sdBox(p - vec3(0.0, 1.66, 0.0), vec3(0.31, 0.025, 0.24)), WHITE));
  r = U(r, led(p - vec3(0.15, 1.7, 0.1), 0.03, LIME));
  return r;
}

vec2 mfp(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.37, 0.0), vec3(1.0, 0.35, 0.6)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.97, 0.02, 0.57)), GRAPHITE));
  vec3 fz = faceZ(p, 0.6);
  vec3 dq = vec3(repC(fz.x, 0.98, 2.0), fz.y, repC(fz.z - 0.38, 0.28, 2.0));
  vec2 dr = M(extrude(sdRect(dq.xz, vec2(0.45, 0.115), 0.02), dq.y - 0.012, 0.012), WHITE);
  vec3 dd = dq - vec3(0.0, 0.024, 0.0);
  dr = Sc(dr, M(max(sdRect(dd.xz - vec2(0.0, 0.05), vec2(0.1, 0.022), 0.02), -(dd.y + 0.015)), GRAPHITE));
  dr = decal(dr, dd, sdRect(dd.xz - vec2(0.33, -0.04), vec2(0.07, 0.015), 0.01), LIME);
  r = U(r, dr);
  vec3 fx = faceX(p, 1.0);
  r = vents(r, fx - vec3(-0.25, 0.0, 0.4), 6.0, 0.07, 0.24, 0.028, GRAPHITE);
  vec2 eng = M(sdBox(p - vec3(0.0, 0.88, 0.0), vec3(0.97, 0.16, 0.58)), WHITE);
  eng = inset(eng, faceZ(p, 0.58) - vec3(-0.2, 0.0, 0.95), vec2(0.6, 0.018), 0.018, 0.012, LIME);
  vec3 ex = faceX(p, 0.97);
  eng = Sc(eng, M(max(sdRect(ex.xz - vec2(0.05, 0.88), vec2(0.42, 0.09), 0.02), -(ex.y + 0.3)), GRAPHITE));
  r = U(r, eng);
  vec3 ot = p - vec3(1.12, 0.84, 0.05);
  r = U(r, M(sdBox(rotZ(ot, 0.12), vec3(0.2, 0.014, 0.36)), STEEL));
  r = U(r, stack(rotZ(ot - vec3(0.02, 0.014, 0.0), 0.12), vec2(0.13, 0.17), 0.07, LIME));
  vec3 sb = p - vec3(-0.08, 1.09, -0.02);
  vec2 bed = M(sdBox(sb, vec3(0.86, 0.05, 0.53)), WHITE);
  bed = decal(bed, faceY(sb, 0.05), sdRect(sb.xz - vec2(0.0, 0.03), vec2(0.72, 0.4), 0.02), SKY);
  r = U(r, bed);
  r = U(r, M(sdBox(sb - vec3(-0.1, 0.07, 0.03), vec3(0.03, 0.025, 0.41)), LIME + SOLID));
  vec3 lq = rotX(sb - vec3(0.0, 0.05, -0.53), 0.42);
  vec2 lid = M(sdBox(lq - vec3(0.0, 0.035, 0.52), vec3(0.86, 0.035, 0.52)), WHITE);
  lid = seam(lid, lq.y - 0.035);
  r = U(r, lid);
  vec3 af = lq - vec3(0.15, 0.07, 0.5);
  r = U(r, M(sdBox(af - vec3(0.0, 0.03, 0.0), vec3(0.5, 0.03, 0.34)), WHITE));
  r = U(r, stack(af - vec3(0.0, 0.06, 0.0), vec2(0.3, 0.24), 0.05, SKY));
  vec3 cp = rotX(p - vec3(0.62, 1.08, 0.7), 0.55);
  vec2 pan = M(sdBox(cp, vec3(0.28, 0.03, 0.13)), WHITE);
  vec3 ct = faceY(cp, 0.03);
  pan = decal(pan, ct, sdRect(ct.xz - vec2(-0.08, 0.0), vec2(0.13, 0.085), 0.015), SKY);
  pan = decal(pan, ct, sdRect(ct.xz - vec2(-0.12, -0.02), vec2(0.06, 0.02), 0.01), LIME);
  r = U(r, pan);
  r = U(r, led(ct - vec3(0.13, 0.0, -0.04), 0.022, LIME));
  r = U(r, led(ct - vec3(0.2, 0.0, -0.04), 0.022, STEEL));
  r = U(r, button(ct - vec3(0.165, 0.0, 0.05), 0.03, 0.012, WHITE));
  return r;
}

vec2 sorter(vec3 p) {
  vec3 l = mirXZ(p) - vec3(0.24, 0.32, 0.24);
  vec2 r = M(sdBox(l, vec3(0.025, 0.32, 0.025)), GRAPHITE);
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.2, abs(p.z) - 0.24), vec3(0.24, 0.018, 0.018)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.24, p.y - 0.2, p.z), vec3(0.018, 0.018, 0.24)), GRAPHITE));
  vec3 b = p - vec3(0.0, 0.84, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.32, 0.2, 0.32), 0.025), WHITE);
  body = seam(body, b.y + 0.08);
  body = seam(body, b.y - 0.15);
  vec3 fx = faceX(p, 0.32);
  body = vents(body, fx - vec3(-0.14, 0.0, 0.9), 4.0, 0.055, 0.1, 0.022, GRAPHITE);
  vec3 fz = faceZ(p, 0.32);
  body = inset(body, fz - vec3(-0.06, 0.0, 0.9), vec2(0.16, 0.05), 0.02, 0.02, GRAPHITE);
  r = U(r, body);
  r = U(r, led(fx - vec3(0.07, 0.0, 0.9), 0.026, LIME));
  r = U(r, led(fx - vec3(0.15, 0.0, 0.9), 0.026, LILAC));
  r = U(r, led(fx - vec3(0.23, 0.0, 0.9), 0.026, SKY));
  r = U(r, M(sdBox(fz - vec3(-0.06, 0.0, 0.9), vec3(0.14, 0.004, 0.006)), LIME + SOLID));
  vec3 h = p - vec3(0.0, 1.06, 0.0);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.04, 0.0), 0.12, 0.05), STEEL + ROUND));
  vec3 cq = h - vec3(0.0, 0.32, 0.0);
  float outer = sdCone(cq, 0.22, 0.12, 0.42);
  float inner = sdCone(cq - vec3(0.0, 0.03, 0.0), 0.22, 0.09, 0.39);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq - vec3(0.0, 0.22, 0.0), 0.42, 0.025), BLUE + SOLID));
  r = U(r, sheet(rotZ(rotY(cq - vec3(0.06, 0.04, 0.02), 0.6), 0.45), SKY));
  r = U(r, sheet(rotX(rotY(cq - vec3(-0.1, 0.07, -0.06), -0.3), -0.5), WHITE));
  return r;
}

vec2 chute(vec3 p, vec3 a, vec3 b, float m) {
  vec3 d = b - a;
  vec3 q = rotY(p - a, -atan(d.z, d.x));
  q = rotZ(q, atan(d.y, length(d.xz)));
  float l = length(d);
  vec3 c = q - vec3(l * 0.5, 0.0, 0.0);
  vec2 r = M(max(sdBox(c, vec3(l * 0.5, 0.035, 0.13)), -sdBox(c - vec3(0.0, 0.04, 0.0), vec3(l * 0.5 + 0.05, 0.04, 0.1))), STEEL);
  return U(r, sheet(rotY(c - vec3(0.05, 0.0, 0.0), PI * 0.5 + 0.1), m));
}

vec2 bin(vec3 p, float m) {
  vec3 q = p - vec3(0.0, 0.2, 0.0);
  vec2 c = M(max(sdRoundBox(q, vec3(0.27, 0.2, 0.27), 0.015), -sdBox(q - vec3(0.0, 0.05, 0.0), vec3(0.24, 0.2, 0.24))), WHITE);
  c = seam(c, q.y + 0.12);
  c = paint(c, 0.14 - q.y, m);
  c = U(c, stack(q - vec3(0.0, -0.15, 0.0), vec2(0.15, 0.18), 0.3, m == LIME ? SKY : LIME));
  return c;
}

vec2 sortUnit(vec3 p) {
  vec2 r = sorter(p);
  r = U(r, bin(p - vec3(0.0, 0.0, 1.3), SKY));
  r = U(r, bin(p - vec3(0.95, 0.0, 0.95), LILAC));
  r = U(r, bin(p - vec3(1.3, 0.0, 0.0), LIME));
  if (near(p - vec3(0.6, 0.6, 0.6), vec3(0.9, 0.2, 0.9), r.x)) {
    r = U(r, chute(p, vec3(0.0, 0.7, 0.3), vec3(0.0, 0.5, 1.2), SKY));
    r = U(r, chute(p, vec3(0.22, 0.7, 0.22), vec3(0.88, 0.5, 0.88), LILAC));
    r = U(r, chute(p, vec3(0.3, 0.7, 0.0), vec3(1.2, 0.5, 0.0), LIME));
  }
  return r;
}

vec2 binder(vec3 p) {
  vec3 c = vec3(p.x, p.y - 0.62, abs(p.z) - 0.42);
  vec2 r = M(sdBox(c, vec3(0.16, 0.62, 0.09)), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.03, abs(p.z) - 0.42), vec3(0.22, 0.03, 0.14)), GRAPHITE));
  vec3 h = p - vec3(0.0, 1.32, 0.0);
  vec2 hd = M(sdRoundBox(h, vec3(0.24, 0.13, 0.55), 0.03), WHITE);
  hd = seam(hd, h.y + 0.04);
  hd = inset(hd, faceX(h, 0.24) - vec3(0.0, 0.0, 0.03), vec2(0.36, 0.035), 0.02, 0.015, LIME);
  r = U(r, hd);
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.04, 0.0), 0.1, 0.16), STEEL + ROUND));
  r = U(r, M(sdBox(p - vec3(0.0, 0.84, 0.0), vec3(0.2, 0.04, 0.2)), GRAPHITE));
  r = U(r, led(faceY(h, 0.13) - vec3(0.1, 0.0, 0.35), 0.03, LIME));
  return r;
}

vec2 belt(vec3 p, float len, float h) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(q.x, q.y - h + 0.04, abs(q.z) - 0.17), vec3(len * 0.5, 0.04, 0.02)), WHITE);
  vec3 ro = vec3(repC(q.x, 0.11, floor(len / 0.11)), q.y - h + 0.035, q.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.028, 0.16), STEEL + SOLID));
  float n = floor(len / 0.7) + 1.0;
  float lx = repC(q.x, len / (n - 1.0) * 0.96, n);
  r = U(r, M(sdBox(vec3(lx, q.y - (h - 0.08) * 0.5, abs(q.z) - 0.17), vec3(0.02, (h - 0.08) * 0.5, 0.02)), GRAPHITE));
  return r;
}

vec2 cabinetHall(vec3 p) {
  vec3 b = p - vec3(0.0, 0.98, 0.0);
  vec2 r = M(sdBox(b, vec3(0.78, 0.94, 0.7)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.75, 0.02, 0.67)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 1.95, 0.0), vec3(0.82, 0.04, 0.74)), WHITE));
  vec2 h = vec2(0.32, 0.145);
  vec3 fz = faceZ(p, 0.7);
  vec2 cz = vec2(0.0, 0.98);
  vec2 sz = vec2(1.0, 1.0);
  vec3 dz = vec3(fz.x - (sz.x - 0.5) * 0.74, fz.y, fz.z - (cz.y + (sz.y - 2.0) * 0.36));
  r = Sc(r, M(max(sdRect(dz.xz, h, 0.0), -(dz.y + 0.3)), GRAPHITE));
  r = U(r, drawers(fz, cz, vec2(0.74, 0.36), vec2(2.0, 5.0), h, sz, 1.0));
  r = U(r, pulled(dz, h, 0.55, 1.0));
  vec3 fx = faceX(p, 0.78);
  vec2 cx = vec2(0.0, 0.98);
  vec2 sx = vec2(0.0, 3.0);
  vec3 dx = vec3(fx.x - (sx.x - 0.5) * 0.66, fx.y, fx.z - (cx.y + (sx.y - 2.0) * 0.36));
  r = Sc(r, M(max(sdRect(dx.xz, vec2(0.28, 0.145), 0.0), -(dx.y + 0.3)), GRAPHITE));
  r = U(r, drawers(fx, cx, vec2(0.66, 0.36), vec2(2.0, 5.0), vec2(0.28, 0.145), sx, 9.0));
  r = U(r, pulled(dx, vec2(0.28, 0.145), 0.4, 2.0));
  vec3 m = p - vec3(-0.35, 1.99, -0.3);
  r = U(r, M(sdBox(m - vec3(0.0, 0.03, 0.0), vec3(0.14, 0.03, 0.14)), WHITE));
  r = U(r, M(sdCylinder(m - vec3(0.0, 0.3, 0.0), 0.018, 0.28), STEEL + SOLID));
  r = U(r, M(sdTorus(m - vec3(0.0, 0.36, 0.0), 0.09, 0.014), STEEL + SOLID));
  r = U(r, M(sdTorus(m - vec3(0.0, 0.48, 0.0), 0.06, 0.014), STEEL + SOLID));
  r = U(r, led(m - vec3(0.0, 0.6, 0.0), 0.04, LIME));
  return r;
}

vec2 catalog(vec3 p) {
  vec3 b = p - vec3(0.0, 0.62, 0.0);
  vec2 r = M(sdBox(b, vec3(0.5, 0.3, 0.3)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.94, 0.0), vec3(0.53, 0.025, 0.33)), WHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.3, 0.0), vec3(0.52, 0.025, 0.32)), WHITE));
  vec3 l = mirXZ(p) - vec3(0.44, 0.15, 0.25);
  r = U(r, M(sdBox(l, vec3(0.03, 0.15, 0.03)), GRAPHITE));
  r = U(r, drawers(faceZ(p, 0.3), vec2(0.0, 0.62), vec2(0.165, 0.145), vec2(6.0, 4.0), vec2(0.07, 0.06), vec2(-1.0), 30.0));
  r = U(r, drawers(faceX(p, 0.5), vec2(0.0, 0.62), vec2(0.15, 0.145), vec2(4.0, 4.0), vec2(0.065, 0.06), vec2(-1.0), 50.0));
  return r;
}

vec2 vault(vec3 p) {
  vec2 r = M(sdChamferBox(p - vec3(0.0, 0.46, 0.0), vec3(0.34, 0.44, 0.3), 0.03), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.32, 0.02, 0.28)), GRAPHITE));
  vec3 d = faceZ(p, 0.3) - vec3(0.0, 0.0, 0.47);
  r = U(r, M(extrude(sdRect(d.xz, vec2(0.27, 0.36), 0.03), d.y - 0.014, 0.014), WHITE));
  vec3 c = d - vec3(-0.03, 0.028, 0.04);
  r = U(r, M(extrude(length(c.xz) - 0.16, c.y - 0.012, 0.012), LIME + SOLID));
  r = U(r, M(extrude(length(c.xz) - 0.1, c.y - 0.03, 0.012), WHITE + SOLID));
  float sp = min(min(sdSeg(c.xz, vec2(0.0), vec2(0.12, -0.07)), sdSeg(c.xz, vec2(0.0), vec2(-0.12, -0.07))), sdSeg(c.xz, vec2(0.0), vec2(0.0, 0.14)));
  r = U(r, M(extrude(sp - 0.014, c.y - 0.045, 0.01), GRAPHITE));
  r = U(r, M(extrude(length(c.xz) - 0.035, c.y - 0.055, 0.014), GRAPHITE + SOLID));
  vec3 k = d - vec3(0.19, 0.03, -0.18);
  r = U(r, M(extrude(sdRect(k.xz, vec2(0.03, 0.06), 0.01), k.y - 0.01, 0.01), GRAPHITE));
  r = U(r, led(k - vec3(0.0, 0.02, 0.0), 0.018, LIME));
  r = U(r, M(sdBox(vec3(p.x + 0.3, abs(p.y - 0.46) - 0.2, p.z - 0.32), vec3(0.025, 0.06, 0.025)), STEEL));
  return r;
}

vec2 abox(vec3 p, float m) {
  vec2 b = M(sdBox(p - vec3(0.0, 0.09, 0.0), vec3(0.13, 0.09, 0.1)), WHITE);
  b = seam(b, p.y - 0.135);
  return inset(b, faceX(p, 0.13) - vec3(0.0, 0.0, 0.08), vec2(0.05, 0.022), 0.012, 0.01, m);
}

vec2 shelving(vec3 p) {
  vec3 l = vec3(abs(p.x) - 0.2, p.y - 0.6, abs(p.z) - 0.55);
  vec2 r = M(sdBox(l, vec3(0.025, 0.6, 0.025)), GRAPHITE);
  float i = repI(p.y - 0.62, 0.38, 4.0);
  vec3 s = vec3(p.x, repC(p.y - 0.05 - 0.57, 0.38, 4.0), p.z);
  r = U(r, M(sdBox(s, vec3(0.22, 0.015, 0.58)), WHITE));
  float sh = clamp(floor((p.y - 0.05) / 0.38), 0.0, 2.0);
  float j = repI(p.z, 0.27, 4.0);
  vec3 bq = vec3(p.x, p.y - 0.065 - sh * 0.38, repC(p.z, 0.27, 4.0));
  float hb = hash(sh * 5.0 + j);
  r = U(r, abox(bq, hb > 0.7 ? LIME : hb > 0.45 ? SKY : PAPER));
  return r;
}

vec2 rack(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.62, 0.0), vec3(0.55, 0.6, 0.32)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.52, 0.02, 0.3)), GRAPHITE));
  vec3 f = faceZ(p, 0.32);
  vec3 u = vec3(repC(f.x, 0.36, 3.0), f.y, repC(f.z - 0.62, 0.17, 6.0));
  r = U(r, M(extrude(sdRect(u.xz, vec2(0.155, 0.065), 0.01), u.y - 0.01, 0.01), WHITE));
  r = vents(r, u - vec3(-0.04, 0.02, 0.0), 4.0, 0.04, 0.08, 0.016, GRAPHITE);
  float on = hash(repI(f.x, 0.36, 3.0) * 7.0 + repI(f.z - 0.62, 0.17, 6.0));
  r = U(r, led(u - vec3(0.11, 0.02, 0.0), 0.016, on > 0.45 ? LIME : STEEL));
  r = vents(r, faceX(p, 0.55) - vec3(0.0, 0.0, 0.62), 4.0, 0.12, 0.8, 0.03, GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 1.24, 0.0), vec3(0.57, 0.02, 0.34)), WHITE));
  return r;
}

vec2 planter(vec3 p, float n) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.1, 0.0), vec3(0.18 * n, 0.1, 0.18)), WHITE);
  r = seam(r, p.y - 0.17);
  vec3 s = vec3(repC(p.x, 0.36, n), p.y - 0.24, p.z);
  r = U(r, M(sdEllipsoid(s, vec3(0.16, 0.13, 0.15)), MINT + SOLID));
  return r;
}

vec2 tree(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.25 * s, 0.0), 0.03 * s, 0.25 * s), GRAPHITE + SOLID);
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.6 * s, 0.0), vec3(0.24, 0.3, 0.24) * s), MINT + SOLID));
  return r;
}

float paths(vec3 p, vec2 q) {
  float d = abs(q.y - 1.55) - 0.24;
  vec3 a = W(-5.3, -0.6), b = W(0.0, -0.7), c = W(5.3, -0.6);
  d = min(d, sdSeg(p.xz, a.xz, a.xz + vec2(0.0, 3.0)) - 0.2);
  d = min(d, sdSeg(p.xz, c.xz, c.xz + vec2(0.0, 3.0)) - 0.2);
  return d;
}

vec2 ground(vec3 p, vec2 q) {
  vec2 r = M(max(max(p.y, -p.y - 0.3), -(q.y + 2.9)), MINT);
  vec3 top = faceY(p, 0.0);
  float pd = paths(p, q);
  r = decal(r, top, pd, WHITE);
  if (pd < 0.0 && p.y > -0.01) r = stripes(r, q.x, 0.3);
  return r;
}

vec2 map(vec3 p) {
  vec2 q = toUV(p);
  vec2 r = p.y < 0.2 ? ground(p, q) : M(p.y - 0.1, MINT);
  vec3 c;
  c = W(-5.3, -0.6);
  if (near(p - c - vec3(0.0, 0.8, 0.0), vec3(0.95, 0.85, 0.8), r.x)) r = U(r, mailroom(p - c));
  vec3 tq = c + vec3(-1.2, 0.0, 0.45);
  if (near(p - tq - vec3(0.0, 0.85, 0.0), vec3(0.35, 0.9, 0.3), r.x)) r = U(r, trays(p - tq));
  vec3 a1 = W(-5.3, -0.6) + vec3(0.45, 1.33, -0.35);
  c = W(0.0, -0.7);
  vec3 so = c + vec3(-1.9, 0.0, 1.0);
  vec3 a2 = so + vec3(0.0, 1.75, 0.0);
  float tuv = toUV(a2).y;
  vec3 a1b = W(toUV(a1).x, tuv) + vec3(0.0, 2.0, 0.0);
  vec3 a1t = a1 + vec3(0.0, 0.42, 0.0);
  if (near(p - vec3(0.0, 1.8, 0.0) - (a1 + a2) * vec3(0.5, 0.0, 0.5), vec3(abs(a1.x - a2.x) * 0.5 + 0.4, 0.4, abs(a1.z - a2.z) * 0.5 + 0.4), r.x)) {
    r = U(r, tube(p, a1, a1t, 0.09, 0.0));
    r = U(r, tube(p, a1t, vec3(a2.x, a1t.y, a2.z), 0.09, 0.3));
    r = U(r, tube(p, vec3(a2.x, a1t.y, a2.z), a2 - vec3(0.0, 0.05, 0.0), 0.09, 0.0));
    r = U(r, M(sdSphere(p - a1t, 0.12), STEEL + SOLID));
    r = U(r, M(sdSphere(p - vec3(a2.x, a1t.y, a2.z), 0.12), STEEL + SOLID));
  }
  if (near(p - c - vec3(0.0, 0.8, 0.0), vec3(1.4, 0.85, 1.0), r.x)) r = U(r, mfp(p - c));
  if (near(p - so - vec3(0.65, 0.85, 0.65), vec3(1.0, 0.9, 1.0), r.x)) r = U(r, sortUnit(p - so));
  vec3 bd = c + vec3(2.1, 0.0, 0.05);
  if (near(p - bd - vec3(0.0, 0.75, 0.0), vec3(0.35, 0.8, 0.7), r.x)) r = U(r, binder(p - bd));
  vec3 bs = c + vec3(1.3, 0.0, 0.05);
  if (near(p - bs - vec3(0.75, 0.35, 0.0), vec3(0.8, 0.4, 0.25), r.x)) {
    r = U(r, belt(p - bs, 1.62, 0.42));
    float ix = repI(p.x - bs.x - 0.25, 0.42, 4.0);
    vec3 iq = vec3(repC(p.x - bs.x - 0.25 - 0.55, 0.42, 4.0), p.y - 0.47, p.z - bs.z);
    if (p.x - bs.x < 0.55) r = U(r, stack(iq, vec2(0.12, 0.15), 0.05, SKY));
    else r = U(r, folder(iq - vec3(0.0, 0.022, 0.0), ix == 2.0 ? LILAC : LIME));
  }
  vec3 b1 = bd + vec3(1.05, 0.0, 0.0);
  vec3 b1t = b1 + vec3(0.0, 1.75, 0.0);
  c = W(5.3, -0.6);
  vec3 b2 = c + vec3(-0.78, 1.55, 0.2);
  vec3 b2t = vec3(b2.x - 0.35, b1t.y, b2.z);
  if (near(p - vec3(0.0, 1.2, 0.0) - (b1 + b2) * vec3(0.5, 0.0, 0.5), vec3(abs(b1.x - b2.x) * 0.5 + 0.4, 0.9, abs(b1.z - b2.z) * 0.5 + 0.4), r.x)) {
    r = U(r, M(sdBox(p - b1 - vec3(0.0, 0.25, 0.0), vec3(0.16, 0.25, 0.2)), WHITE));
    r = U(r, tube(p, b1 + vec3(0.0, 0.5, 0.0), b1t, 0.09, 0.0));
    r = U(r, tube(p, b1t, b2t, 0.09, 0.5));
    r = U(r, tube(p, b2t, b2, 0.09, 0.2));
    r = U(r, M(sdSphere(p - b1t, 0.12), STEEL + SOLID));
    r = U(r, M(sdSphere(p - b2t, 0.12), STEEL + SOLID));
  }
  if (near(p - c - vec3(0.0, 1.2, 0.0), vec3(1.0, 1.3, 1.0), r.x)) r = U(r, cabinetHall(p - c));
  vec3 cc = c + vec3(-1.35, 0.0, 0.6);
  if (near(p - cc - vec3(0.0, 0.5, 0.0), vec3(0.6, 0.55, 0.45), r.x)) r = U(r, catalog(p - cc));
  vec3 vq = c + vec3(-0.35, 0.0, 1.45);
  if (near(p - vq - vec3(0.0, 0.48, 0.0), vec3(0.4, 0.5, 0.4), r.x)) r = U(r, vault(p - vq));
  vec3 sq = c + vec3(1.45, 0.0, -0.3);
  if (near(p - sq - vec3(0.0, 0.62, 0.0), vec3(0.3, 0.65, 0.65), r.x)) r = U(r, shelving(p - sq));
  vec3 rk = W(2.5, -2.0);
  if (near(p - rk - vec3(0.0, 0.65, 0.0), vec3(0.6, 0.66, 0.36), r.x)) r = U(r, rack(p - rk));
  for (int i = 0; i < 4; i++) {
    vec3 pl = i == 0 ? W(-2.9, 1.1) : i == 1 ? W(2.8, 1.05) : i == 2 ? W(-7.6, 1.1) : W(7.7, 1.0);
    if (near(p - pl - vec3(0.0, 0.2, 0.0), vec3(0.6, 0.25, 0.25), r.x)) r = U(r, planter(p - pl, 3.0));
  }
  for (int i = 0; i < 4; i++) {
    vec4 t = i == 0 ? vec4(-7.7, -1.2, 1.1, 0.0) : i == 1 ? vec4(-3.4, 2.6, 0.9, 0.0) : i == 2 ? vec4(-0.6, 2.4, 0.9, 0.0) : vec4(7.9, -1.3, 1.1, 0.0);
    vec3 l = p - W(t.x, t.y);
    if (near(l - vec3(0.0, 0.5 * t.z, 0.0), vec3(0.28, 0.55, 0.28) * t.z, r.x)) r = U(r, tree(l, t.z));
  }
  return r;
}
