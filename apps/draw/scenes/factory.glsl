#pragma frame {"size":[1600,720],"center":[1.8,1.0,0.4],"zoom":96,"bounds":[[-4.6,-0.2,-3.2],[8.2,3.4,5.0]]}

vec2 conveyor(vec3 p, float len, float w, float h) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(q.x, q.y - h + 0.045, abs(q.z) - w), vec3(len * 0.5, 0.045, 0.022)), WHITE);
  vec3 ro = vec3(repC(q.x, 0.11, floor(len / 0.11)), q.y - h + 0.04, q.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.03, w), STEEL + SOLID));
  float n = floor(len / 0.8) + 1.0;
  float lx = repC(q.x, len / (n - 1.0) * 0.96, n);
  vec3 lq = vec3(lx, q.y - (h - 0.09) * 0.5, abs(q.z) - w);
  r = U(r, M(sdBox(lq, vec3(0.022, (h - 0.09) * 0.5, 0.022)), GRAPHITE));
  r = U(r, M(sdBox(vec3(lx, q.y - 0.14, q.z), vec3(0.016, 0.016, w)), GRAPHITE));
  return r;
}

vec2 card(vec3 p) {
  vec2 c = M(sdBox(p, vec3(0.15, 0.018, 0.105)), WHITE);
  vec3 q = faceY(p, 0.018);
  c = decal(c, q, sdRect(q.xz - vec2(-0.085, 0.0), vec2(0.03, 0.08), 0.02), LIME);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, -0.035), vec2(0.07, 0.012), 0.012), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.03, 0.025), vec2(0.05, 0.012), 0.012), STEEL);
  return c;
}

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

vec2 bubble(vec3 p, float m) {
  float d2 = min(sdRect(p.xz, vec2(0.14, 0.09), 0.07), sdSeg(p.xz, vec2(-0.05, 0.06), vec2(-0.11, 0.15)) - 0.025);
  vec2 b = M(extrude(d2, p.y, 0.018), m);
  vec3 q = faceY(p, 0.018);
  return decal(b, q, length(vec2(repC(q.x, 0.07, 3.0), q.z)) - 0.017, m == WHITE ? GRAPHITE : WHITE);
}

vec2 messy(vec3 p) {
  vec2 r = M(1e9, 0.0);
  float y = 0.43;
  r = U(r, envelope(rotY(p - vec3(-0.66, y + 0.014, 1.5), 0.35), CORAL));
  r = U(r, sheet(rotZ(rotY(p - vec3(-0.72, y + 0.07, 1.92), -0.5), 0.3), WHITE));
  r = U(r, bubble(rotY(p - vec3(-0.6, y + 0.02, 2.05), -0.9), LILAC));
  r = U(r, folder(rotY(p - vec3(-0.68, y + 0.022, 2.32), 0.7), SUN));
  r = U(r, envelope(rotY(p - vec3(-0.7, y + 0.014, 2.76), -0.3), WHITE));
  r = U(r, sheet(rotX(rotY(p - vec3(-0.64, y + 0.05, 2.82), 0.9), 0.12), PINK));
  r = U(r, bubble(rotY(p - vec3(-0.74, y + 0.02, 3.2), 0.45), SKY));
  r = U(r, sheet(rotY(p - vec3(-0.68, y + 0.02, 3.27), -0.2), WHITE));
  r = U(r, folder(rotY(p - vec3(-0.7, y + 0.022, 3.68), -0.55), BLUE));
  r = U(r, envelope(rotX(rotY(p - vec3(-0.7, y + 0.1, 4.08), 0.2), -0.35), LILAC));
  return r;
}

vec2 hopper(vec3 p) {
  vec3 q = p - vec3(-0.7, 0.8, 4.15);
  float outer = sdCone(q, 0.2, 0.16, 0.42);
  float inner = sdCone(q - vec3(0.0, 0.03, 0.0), 0.2, 0.12, 0.38);
  vec2 r = M(max(outer, -inner), CORAL + SOLID);
  r = U(r, M(sdTorus(q - vec3(0.0, 0.2, 0.0), 0.42, 0.025), CORAL + SOLID));
  vec3 l = vec3(abs(q.x) - 0.32, q.y + 0.4, abs(q.z) - 0.32);
  r = U(r, M(sdBox(l, vec3(0.022, 0.4, 0.022)), WHITE));
  vec3 fr = vec3(q.x, q.y + 0.62, q.z);
  r = U(r, M(max(sdBox(fr, vec3(0.34, 0.018, 0.34)), -sdBox(fr, vec3(0.3, 0.05, 0.3))), WHITE));
  r = U(r, envelope(rotZ(rotY(q - vec3(0.06, 0.16, 0.02), 0.6), 0.55), WHITE));
  r = U(r, sheet(rotX(rotY(q - vec3(-0.12, 0.2, -0.05), -0.3), -0.5), SUN));
  r = U(r, folder(rotZ(rotY(q - vec3(0.12, 0.18, -0.16), 1.2), -0.4), MINT));
  r = U(r, bubble(rotX(rotY(q - vec3(-0.1, 0.17, 0.16), 2.2), 0.4), BLUE));
  return r;
}

vec2 door(vec2 r, vec3 q, vec2 c) {
  vec3 d = q - vec3(c.x, 0.0, c.y);
  r = Sc(r, M(max(sdRect(d.xz, vec2(0.3, 0.32), 0.0), -(d.y + 0.4)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(d.xz, vec2(0.36, 0.36), 0.03), -sdRect(d.xz, vec2(0.29, 0.31), 0.0)), d.y, 0.03), SUN));
  vec3 s = d - vec3(0.0, -0.06, 0.16);
  vec2 strip = M(extrude(sdRect(s.xz, vec2(0.3, 0.16), 0.0), s.y, 0.008), STEEL);
  return U(r, stripes(strip, s.x, 0.075));
}

vec2 window(vec2 r, vec3 q) {
  float d = sdRect(q.xz, vec2(0.13, 0.19), 0.025);
  r = inset(r, q, vec2(0.13, 0.19), 0.025, 0.035, SKY);
  if (d < 0.0) r = seam(seam(r, q.x), q.z - 0.05);
  return r;
}

vec2 hall(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(2.05, 0.06, 1.6), 0.03), WHITE);
  vec3 w = p - vec3(0.0, 0.72, 0.0);
  vec2 walls = M(sdBox(w, vec3(1.6, 0.6, 1.2)), WHITE);
  walls = stripes(walls, p.x + p.z + 0.15, 0.4);
  walls = paint(walls, p.y - 0.3, STEEL);
  vec3 fz = faceZ(p, 1.2);
  walls = window(walls, vec3(fz.x - 0.02, fz.y, fz.z - 0.86));
  walls = window(walls, vec3(abs(fz.x - 0.01) - 1.36, fz.y, fz.z - 0.86));
  vec3 fx = faceX(p, 1.6);
  walls = window(walls, vec3(repC(fx.x + 0.85, 0.5, 2.0), fx.y, fx.z - 0.86));
  walls = vents(walls, fx - vec3(0.95, 0.0, 0.92), 5.0, 0.07, 0.22, 0.03, GRAPHITE);
  walls = door(walls, fz, vec2(-0.7, 0.44));
  walls = door(walls, fz, vec2(0.75, 0.66));
  walls = door(walls, fx, vec2(0.3, 0.44));
  r = U(r, walls);
  vec2 roof = M(1e9, 0.0);
  for (int i = 0; i < 4; i++) {
    float tx = p.x + 1.2 - 0.8 * float(i);
    vec3 tq = vec3(-tx, p.y - 1.32 - 0.24, p.z);
    vec2 t = M(sdWedge(tq, vec3(0.4, 0.24, 1.27)), WHITE);
    t = stripes(t, p.z, 0.32);
    float glass = max(0.385 - tx, sdRect(vec2(p.z, tq.y + 0.01), vec2(1.12, 0.15), 0.0));
    t = paint(t, glass, SKY);
    if (glass < 0.0) t = stripes(t, p.z + 0.14, 0.28);
    roof = U(roof, t);
  }
  r = U(r, roof);
  r = U(r, M(sdBox(p - vec3(0.0, 1.34, 0.0), vec3(1.63, 0.025, 1.23)), WHITE));
  vec3 c1 = p - vec3(-1.15, 0.0, -0.75);
  vec2 ch = M(sdCylinder(c1 - vec3(0.0, 1.85, 0.0), 0.15, 0.55), WHITE + SOLID);
  ch = paint(ch, abs(c1.y - 2.12) - 0.07, CORAL + SOLID);
  ch = U(ch, M(sdCylinder(c1 - vec3(0.0, 2.4, 0.0), 0.18, 0.035), WHITE + SOLID));
  ch = Sc(ch, M(sdCylinder(c1 - vec3(0.0, 2.45, 0.0), 0.12, 0.08), GRAPHITE + SOLID));
  vec3 c2 = p - vec3(-0.35, 0.0, -0.75);
  vec2 ch2 = M(sdCylinder(c2 - vec3(0.0, 1.7, 0.0), 0.11, 0.4), WHITE + SOLID);
  ch2 = paint(ch2, abs(c2.y - 1.92) - 0.05, CORAL + SOLID);
  ch2 = U(ch2, M(sdCylinder(c2 - vec3(0.0, 2.1, 0.0), 0.135, 0.03), WHITE + SOLID));
  ch2 = Sc(ch2, M(sdCylinder(c2 - vec3(0.0, 2.14, 0.0), 0.085, 0.06), GRAPHITE + SOLID));
  r = U(r, U(ch, ch2));
  vec3 s = c1 - vec3(0.0, 2.62, 0.0);
  float puff = smin(smin(sdSphere(s, 0.13), sdSphere(s - vec3(0.12, 0.14, -0.08), 0.16), 0.06), sdSphere(s - vec3(0.32, 0.26, -0.2), 0.12), 0.06);
  vec3 s2 = c2 - vec3(0.0, 2.3, 0.0);
  puff = min(puff, smin(sdSphere(s2, 0.09), sdSphere(s2 - vec3(0.1, 0.1, -0.06), 0.11), 0.05));
  r = U(r, M(puff, WHITE + SOLID));
  vec3 pp = p - vec3(1.72, 0.0, -0.85);
  vec2 pipes = M(pipe(pp, vec3(0.0, 0.12, 0.0), vec3(0.0, 1.05, 0.0), 0.07), STEEL + SOLID);
  pipes = U(pipes, M(pipe(pp, vec3(0.0, 1.05, 0.0), vec3(2.5, 1.05, 0.0), 0.07), STEEL + SOLID));
  vec3 fl = vec3(repC(pp.x - 0.75, 0.75, 3.0), pp.y - 1.05, pp.z);
  pipes = U(pipes, M(sdCylinder(rotZ(fl, PI * 0.5), 0.1, 0.022), STEEL + SOLID));
  pipes = U(pipes, M(sdCylinder(vec3(pp.x, repC(pp.y - 0.62, 0.45, 2.0), pp.z), 0.1, 0.022), STEEL + SOLID));
  vec3 vq = rotZ(pp - vec3(0.13, 0.62, 0.0), PI * 0.5);
  pipes = U(pipes, M(sdTorus(vq, 0.075, 0.014), SUN + SOLID));
  pipes = U(pipes, M(pipe(pp, vec3(0.0, 0.62, 0.0), vec3(0.13, 0.62, 0.0), 0.016), GRAPHITE + SOLID));
  r = U(r, pipes);
  vec3 a = pp - vec3(1.2, 0.0, 0.0);
  vec3 al = vec3(a.x, a.y - 0.5, abs(a.z) - 0.18 + a.y * 0.12);
  r = U(r, M(sdBox(al, vec3(0.025, 0.5, 0.025)), GRAPHITE));
  r = U(r, M(sdBox(a - vec3(0.0, 0.97, 0.0), vec3(0.03, 0.025, 0.14)), GRAPHITE));
  vec3 u = p - vec3(1.85, 0.0, -1.32);
  vec2 unit = M(sdBox(u - vec3(0.0, 0.3, 0.0), vec3(0.15, 0.18, 0.2)), WHITE);
  vec3 uf = faceX(u, 0.15);
  unit = inset(unit, uf - vec3(0.0, 0.0, 0.3), vec2(0.13, 0.13), 0.13, 0.02, GRAPHITE);
  r = U(r, unit);
  vec3 fan = rotZ(uf - vec3(0.0, -0.02, 0.3), 0.0);
  float an = atan(fan.z, fan.x);
  float blade = max(length(fan.xz) - 0.11, abs(sin(an * 2.0 + length(fan.xz) * 8.0)) - 0.5);
  r = U(r, M(max(extrude(blade, fan.y, 0.008), -length(fan.xz) + 0.025), STEEL + SOLID));
  r = U(r, M(extrude(length(fan.xz) - 0.03, fan.y, 0.012), GRAPHITE + SOLID));
  return r;
}

vec2 silo(vec3 p) {
  vec3 q = p - vec3(4.6, 0.0, -1.2);
  vec2 r = M(sdCylinder(q - vec3(0.0, 0.04, 0.0), 0.72, 0.04), WHITE + SOLID);
  vec2 body = M(sdCylinder(q - vec3(0.0, 1.2, 0.0), 0.58, 1.12), WHITE + SOLID);
  body = stripes(body, q.y - 0.08, 0.37);
  vec3 f = faceX(rotY(q, -PI * 0.25), 0.58);
  float row = repI(f.z - 1.19, 0.37, 6.0);
  vec3 lq = vec3(f.x, f.y, repC(f.z - 1.19, 0.37, 6.0));
  body = vents(body, lq - vec3(-0.12, 0.0, 0.0), 3.0, 0.055, 0.12, 0.022, GRAPHITE);
  r = U(r, body);
  r = U(r, led(lq - vec3(0.12, 0.0, 0.0), 0.035, row == 5.0 ? LIME : STEEL));
  r = U(r, M(sdCone(q - vec3(0.0, 2.48, 0.0), 0.16, 0.6, 0.22), WHITE + SOLID));
  r = U(r, M(sdCylinder(q - vec3(0.0, 2.68, 0.0), 0.12, 0.05), STEEL + SOLID));
  r = U(r, M(sdCylinder(q - vec3(0.0, 2.32, 0.0), 0.61, 0.02), WHITE + SOLID));
  r = U(r, M(sdTorus(q - vec3(0.0, 2.55, 0.0), 0.66, 0.012), GRAPHITE + SOLID));
  vec3 po = polarY(q, 12.0) - vec3(0.66, 2.44, 0.0);
  r = U(r, M(sdBox(po, vec3(0.01, 0.11, 0.01)), GRAPHITE));
  vec3 lad = vec3(q.z, q.y - 0.08, q.x - 0.64);
  r = U(r, M(ladder(lad, 2.38, 0.085, 0.11, 0.012), GRAPHITE));
  return r;
}

vec2 crate(vec3 p, float full) {
  vec3 q = p - vec3(0.0, 0.17, 0.0);
  vec2 c = M(max(sdBox(q, vec3(0.25, 0.17, 0.25)), -sdBox(q - vec3(0.0, 0.1, 0.0), vec3(0.22, 0.17, 0.22))), WHITE);
  c = stripes(c, q.y + 0.17, 0.085);
  vec3 k = mirXZ(q) - vec3(0.25, 0.0, 0.25);
  c = U(c, M(sdBox(k, vec3(0.035, 0.175, 0.035)), WHITE));
  if (full > 0.5) {
    float i = repI(q.x, 0.075, 6.0);
    vec3 s = vec3(repC(q.x, 0.075, 6.0), q.y - 0.07 - (i == 2.0 ? 0.05 : 0.0), q.z);
    vec2 cards = M(sdBox(s, vec3(0.02, 0.14, 0.19)), i == 2.0 ? LIME : WHITE);
    c = U(c, cards);
  }
  return c;
}

vec2 truck(vec3 p, float cm, float full) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.16, 0.0), vec3(0.62, 0.04, 0.26)), GRAPHITE);
  vec3 w = vec3(repC(p.x + 0.05, 0.62, 2.0), p.y - 0.11, abs(p.z) - 0.24);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.11, 0.04), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(w - vec3(0.0, 0.0, 0.01), PI * 0.5), 0.045, 0.04), STEEL + SOLID));
  vec3 b = p - vec3(0.15, 0.55, 0.0);
  vec2 box = M(sdBox(b, vec3(0.45, 0.35, 0.28)), WHITE);
  box = stripes(box, b.x + 0.45, 0.15);
  box = Sc(box, M(sdBox(b - vec3(0.5, 0.0, 0.0), vec3(0.08, 0.31, 0.24)), GRAPHITE));
  if (full < 0.5) box = paint(box, max(abs(b.y + 0.12) - 0.05, -abs(b.z) + 0.27), LIME);
  r = U(r, box);
  if (full > 0.5) {
    r = U(r, envelope(rotZ(rotY(b - vec3(0.36, -0.29, 0.06), 0.4), 0.1), CORAL));
    r = U(r, folder(rotZ(rotY(b - vec3(0.38, -0.24, -0.1), -0.3), 0.15), WHITE));
    r = U(r, bubble(rotZ(rotY(b - vec3(0.33, -0.12, 0.02), 1.2), 1.1), LILAC));
  }
  vec3 c = p - vec3(-0.5, 0.42, 0.0);
  vec2 cab = M(sdRoundBox(c, vec3(0.16, 0.22, 0.26), 0.04), cm);
  vec3 cw = faceZ(c, 0.26);
  cab = inset(cab, cw - vec3(-0.01, 0.0, 0.09), vec2(0.1, 0.08), 0.02, 0.02, SKY);
  cab = seam(cab, c.y + 0.03);
  r = U(r, cab);
  r = U(r, M(sdBox(c - vec3(-0.17, -0.16, 0.0), vec3(0.03, 0.04, 0.27)), GRAPHITE));
  return r;
}

vec2 lamp(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.08, 0.02), GRAPHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.5, 0.0), 0.02, 0.5), GRAPHITE + SOLID));
  r = U(r, M(pipe(p, vec3(0.0, 0.98, 0.0), vec3(0.16, 0.98, 0.16), 0.018), GRAPHITE + SOLID));
  vec3 h = p - vec3(0.2, 0.95, 0.2);
  r = U(r, M(sdCone(h, 0.04, 0.11, 0.04), STEEL + SOLID));
  r = U(r, M(max(sdSphere(h + vec3(0.0, 0.03, 0.0), 0.07), h.y + 0.03), SUN + SOLID));
  return r;
}

vec2 tree(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.16 * s, 0.03), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.3 * s, 0.0), 0.035 * s, 0.3 * s), GRAPHITE + SOLID));
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.62 * s, 0.0), vec3(0.27, 0.33, 0.27) * s), MINT + SOLID));
  return r;
}

vec2 pine(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.14 * s, 0.03), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.15 * s, 0.0), 0.035 * s, 0.15 * s), GRAPHITE + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.42 * s, 0.0), 0.16 * s, 0.26 * s, 0.1 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.66 * s, 0.0), 0.14 * s, 0.2 * s, 0.05 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.86 * s, 0.0), 0.12 * s, 0.14 * s, 0.0), MINT + SOLID));
  return r;
}

vec2 pallet(vec3 p, vec2 h) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.105, 0.0), vec3(h.x, 0.015, h.y)), PAPER);
  r = stripes(r, p.x + h.x, 0.1);
  vec3 b = vec3(repC(p.x, h.x - 0.04, 3.0), p.y - 0.045, repC(p.z, h.y - 0.04, 3.0));
  r = U(r, M(sdBox(b, vec3(0.04, 0.045, 0.04)), PAPER));
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.006, repC(p.z, h.y - 0.04, 3.0)), vec3(h.x, 0.006, 0.04)), PAPER));
  return r;
}

vec2 forklift(vec3 p) {
  vec3 b = p - vec3(0.05, 0.2, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.26, 0.09, 0.17), 0.03), SUN);
  r = seam(r, b.y + 0.03);
  r = U(r, M(sdRoundBox(p - vec3(0.31, 0.26, 0.0), vec3(0.06, 0.13, 0.175), 0.03), GRAPHITE));
  vec3 w = vec3(repC(p.x - 0.025, 0.35, 2.0), p.y - 0.09, abs(p.z) - 0.16);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.09, 0.035), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(w - vec3(0.0, 0.0, 0.01), PI * 0.5), 0.04, 0.035), STEEL + SOLID));
  r = U(r, M(sdBox(p - vec3(0.12, 0.33, 0.0), vec3(0.07, 0.04, 0.09)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.19, 0.43, 0.0), vec3(0.02, 0.1, 0.09)), GRAPHITE));
  r = U(r, M(sdTorus(rotZ(p - vec3(-0.08, 0.42, 0.0), 0.7), 0.05, 0.012), GRAPHITE + SOLID));
  r = U(r, M(pipe(p, vec3(-0.12, 0.29, 0.0), vec3(-0.08, 0.42, 0.0), 0.012), GRAPHITE + SOLID));
  vec3 g = vec3(repC(p.x - 0.07, 0.36, 2.0), p.y - 0.51, abs(p.z) - 0.15);
  r = U(r, M(sdBox(g, vec3(0.015, 0.22, 0.015)), GRAPHITE));
  vec3 rf = p - vec3(0.07, 0.74, 0.0);
  vec2 roof = M(sdBox(rf, vec3(0.21, 0.018, 0.17)), WHITE);
  r = U(r, stripes(roof, rf.x, 0.05));
  r = U(r, led(faceY(rf, 0.018) - vec3(0.13, 0.0, 0.0), 0.035, CORAL));
  vec3 m = vec3(p.x + 0.25, p.y - 0.5, abs(p.z) - 0.11);
  r = U(r, M(sdBox(m, vec3(0.02, 0.44, 0.022)), STEEL));
  r = U(r, M(sdBox(p - vec3(-0.25, 0.93, 0.0), vec3(0.025, 0.02, 0.13)), STEEL));
  r = U(r, M(sdBox(p - vec3(-0.29, 0.4, 0.0), vec3(0.015, 0.1, 0.14)), GRAPHITE));
  r = U(r, M(sdBox(vec3(p.x + 0.52, p.y - 0.29, abs(p.z) - 0.09), vec3(0.23, 0.012, 0.025)), STEEL));
  return r;
}

vec3 jframe(vec3 q, vec3 d, vec3 n) { return vec3(dot(q, d), dot(q, n), q.y); }

vec2 robot(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.05, 0.0), 0.24, 0.05, 0.015), WHITE + ROUND);
  r = U(r, screw(faceY(polarY(p, 6.0), 0.1) - vec3(0.19, 0.0, 0.0), 0.02));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.2, 0.0), 0.13, 0.1), STEEL + SOLID));
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.32, 0.0), 0.17, 0.035, 0.01), GRAPHITE + ROUND));
  vec3 d = vec3(-0.331, 0.0, 0.944);
  vec3 n = vec3(0.944, 0.0, 0.331);
  vec3 S = vec3(0.0, 0.5, 0.0);
  vec3 E = vec3(-0.116, 1.0, 0.33);
  vec3 W = vec3(-0.27, 0.56, 0.77);
  vec3 t = jframe(p - vec3(0.0, 0.41, 0.0), d, n);
  r = U(r, M(sdRoundBox(t.xzy, vec3(0.12, 0.07, 0.1), 0.03), WHITE));
  r = U(r, M(pipe(p, S, E, 0.065), WHITE + SOLID));
  r = U(r, M(pipe(p, E, W, 0.05), WHITE + SOLID));
  vec3 js = jframe(p - S, d, n);
  r = U(r, M(sdCylinder(js.xzy * vec3(1.0, 1.0, 1.0), 0.1, 0.12), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(js.xzy, 0.05, 0.135), STEEL + SOLID));
  vec3 je = jframe(p - E, d, n);
  r = U(r, M(sdCylinder(je.xzy, 0.08, 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(je.xzy, 0.04, 0.115), LIME + SOLID));
  vec3 wq = p - W;
  r = U(r, M(sdCylinder(wq, 0.055, 0.05), GRAPHITE + SOLID));
  vec3 g = jframe(wq - vec3(0.0, -0.07, 0.0), d, n);
  r = U(r, M(sdBox(g.xzy, vec3(0.04, 0.02, 0.14)), GRAPHITE));
  r = U(r, M(sdBox(vec3(g.x, g.z + 0.06, abs(g.y) - 0.13), vec3(0.03, 0.06, 0.012)), GRAPHITE));
  vec3 c = jframe(wq - vec3(0.0, -0.15, 0.0), d, n);
  r = U(r, card(c.xzy));
  r = U(r, led(faceY(p, 0.1) - vec3(0.0, 0.0, 0.19), 0.03, LIME));
  return r;
}

vec2 dock(vec3 p) {
  vec3 d = p - vec3(0.75, 0.15, 1.66);
  vec2 r = M(sdBox(d, vec3(0.6, 0.15, 0.44)), WHITE);
  r = seam(r, d.y + 0.11);
  r = paint(r, max(d.y - 0.11, -(d.z - 0.36)), SUN);
  vec3 bq = vec3(abs(d.x) - 0.4, d.y + 0.01, d.z - 0.46);
  r = U(r, M(sdRoundBox(bq, vec3(0.05, 0.08, 0.025), 0.015), GRAPHITE));
  vec3 st = p - vec3(1.45, 0.0, 1.85);
  r = U(r, M(sdBox(st - vec3(0.0, 0.05, 0.0), vec3(0.1, 0.05, 0.2)), WHITE));
  r = U(r, M(sdBox(st - vec3(-0.04, 0.15, 0.0), vec3(0.06, 0.05, 0.2)), WHITE));
  vec3 cp = p - vec3(0.75, 1.1, 1.36);
  r = U(r, M(sdBox(cp, vec3(0.44, 0.022, 0.16)), STEEL));
  vec3 cs = vec3(abs(cp.x) - 0.36, cp.y, cp.z);
  r = U(r, M(pipe(cs, vec3(0.0, 0.0, 0.14), vec3(0.0, 0.18, -0.16), 0.012), GRAPHITE + SOLID));
  return r;
}

vec2 win(vec2 r, vec3 q, vec2 s) {
  float d = sdRect(q.xz, s, 0.02);
  r = inset(r, q, s, 0.02, 0.03, SKY);
  if (d < 0.0) r = seam(r, q.x);
  return r;
}

vec2 annex(vec3 p) {
  vec3 a = p - vec3(-2.82, 0.0, 1.1);
  vec2 r = M(sdBox(a - vec3(0.0, 0.55, 0.0), vec3(0.48, 0.55, 0.8)), WHITE);
  r = paint(r, a.y - 0.12, STEEL);
  r = seam(r, a.y - 0.58);
  vec3 fz = faceZ(a, 0.8);
  r = win(r, vec3(repC(fz.x, 0.3, 3.0), fz.y, fz.z - 0.84), vec2(0.09, 0.14));
  r = win(r, vec3(fz.x - 0.22, fz.y, fz.z - 0.34), vec2(0.13, 0.13));
  r = inset(r, fz - vec3(-0.18, 0.0, 0.25), vec2(0.11, 0.25), 0.02, 0.04, GRAPHITE);
  vec3 fx = faceX(a, 0.48);
  r = win(r, vec3(repC(fx.x, 0.34, 4.0), fx.y, fx.z - 0.84), vec2(0.1, 0.14));
  r = win(r, vec3(repC(fx.x, 0.34, 4.0), fx.y, fx.z - 0.34), vec2(0.1, 0.14));
  vec3 cn = a - vec3(-0.18, 0.56, 0.9);
  r = U(r, M(sdBox(cn, vec3(0.17, 0.02, 0.1)), SUN));
  vec3 rt = a - vec3(0.0, 1.14, 0.0);
  r = U(r, M(max(sdBox(rt, vec3(0.5, 0.04, 0.82)), -sdBox(rt - vec3(0.0, 0.03, 0.0), vec3(0.45, 0.06, 0.77))), WHITE));
  vec3 ac = a - vec3(-0.12, 1.22, -0.3);
  vec2 unit = M(sdBox(ac, vec3(0.2, 0.12, 0.17)), WHITE);
  unit = grille(unit, faceX(ac, 0.2) - vec3(0.0, 0.0, 0.0), vec2(5.0, 3.0), 0.06, 0.018, GRAPHITE);
  r = U(r, unit);
  vec3 fan = faceY(ac, 0.12);
  r = U(r, M(extrude(length(fan.xz) - 0.1, fan.y, 0.012), GRAPHITE + SOLID));
  r = U(r, M(extrude(length(fan.xz) - 0.03, fan.y - 0.01, 0.01), STEEL + SOLID));
  vec3 an = a - vec3(0.25, 1.1, 0.45);
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.3, 0.0), 0.015, 0.3), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.03, 0.0), 0.06, 0.03), STEEL + SOLID));
  r = U(r, led(an - vec3(0.0, 0.62, 0.0), 0.035, CORAL));
  return r;
}

vec2 watertower(vec3 p) {
  vec3 m = mirXZ(p);
  vec2 r = M(sdBox(m - vec3(0.42, 0.04, 0.42), vec3(0.07, 0.04, 0.07)), WHITE);
  r = U(r, M(pipe(m, vec3(0.42, 0.0, 0.42), vec3(0.3, 1.58, 0.3), 0.028), GRAPHITE + SOLID));
  for (int i = 0; i < 2; i++) {
    float h0 = 0.12 + 0.72 * float(i), h1 = h0 + 0.72;
    float x0 = 0.42 - 0.12 * h0 / 1.58, x1 = 0.42 - 0.12 * h1 / 1.58;
    r = U(r, M(pipe(vec3(abs(p.x), p.y, p.z), vec3(x0, h0, -x0), vec3(x1, h1, x1), 0.012), GRAPHITE + SOLID));
    r = U(r, M(pipe(vec3(abs(p.z), p.y, p.x), vec3(x0, h0, -x0), vec3(x1, h1, x1), 0.012), GRAPHITE + SOLID));
    r = U(r, M(pipe(m, vec3(x1, h1, -x1), vec3(x1, h1, x1), 0.014), GRAPHITE + SOLID));
    r = U(r, M(pipe(vec3(m.z, m.y, m.x), vec3(x1, h1, -x1), vec3(x1, h1, x1), 0.014), GRAPHITE + SOLID));
  }
  r = U(r, M(pipe(p, vec3(0.0, 0.0, 0.0), vec3(0.0, 1.6, 0.0), 0.06), STEEL + SOLID));
  vec2 deck = M(sdCylinder(p - vec3(0.0, 1.6, 0.0), 0.64, 0.025), WHITE + SOLID);
  r = U(r, deck);
  r = U(r, M(sdBox(polarY(p, 16.0) - vec3(0.61, 1.72, 0.0), vec3(0.01, 0.1, 0.01)), GRAPHITE));
  r = U(r, M(sdTorus(p - vec3(0.0, 1.82, 0.0), 0.61, 0.012), GRAPHITE + SOLID));
  vec2 tk = M(sdCylinder(p - vec3(0.0, 1.95, 0.0), 0.47, 0.32), WHITE + SOLID);
  tk = stripes(tk, atan(p.z, p.x) * 0.47, 0.1);
  r = U(r, tk);
  r = U(r, M(sdTorus(vec3(p.x, repC(p.y - 1.95, 0.36, 2.0), p.z), 0.475, 0.014), GRAPHITE + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 2.43, 0.0), 0.16, 0.53, 0.05), BLUE + SOLID));
  r = U(r, M(sdSphere(p - vec3(0.0, 2.62, 0.0), 0.05), GRAPHITE + SOLID));
  r = U(r, M(ladder(vec3(p.x, p.y, p.z - 0.62), 1.6, 0.08, 0.1, 0.012), GRAPHITE));
  return r;
}

vec2 tcone(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.01, 0.0), vec3(0.08, 0.01, 0.08)), GRAPHITE);
  vec2 c = M(sdCone(p - vec3(0.0, 0.11, 0.0), 0.1, 0.06, 0.012), CORAL + SOLID);
  c = paint(c, abs(p.y - 0.11) - 0.025, WHITE + SOLID);
  return U(r, c);
}

vec2 drum(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.15, 0.0), 0.1, 0.15), STEEL + SOLID);
  r = U(r, M(sdTorus(vec3(p.x, repC(p.y - 0.15, 0.1, 2.0), p.z), 0.102, 0.008), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.04, 0.31, 0.03), 0.02, 0.012), GRAPHITE + SOLID));
  return r;
}

vec2 bench(vec3 p) {
  vec3 s = p - vec3(0.0, 0.14, 0.0);
  vec2 r = M(sdBox(s, vec3(0.24, 0.012, 0.07)), WHITE);
  r = stripes(r, s.z + 0.07, 0.035);
  r = U(r, M(sdBox(p - vec3(0.0, 0.24, -0.08), vec3(0.24, 0.04, 0.01)), WHITE));
  vec3 l = vec3(abs(p.x) - 0.2, p.y - 0.12, p.z);
  r = U(r, M(sdBox(l, vec3(0.012, 0.12, 0.06)), GRAPHITE));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.95, 1.5, -0.2), vec3(3.1, 1.5, 1.8), r.x)) r = U(r, hall(p));
  vec3 ic = vec3(p.z - 1.0, p.y, p.x + 0.7);
  if (near(ic - vec3(1.65, 0.3, 0.0), vec3(1.7, 0.3, 0.3), r.x)) r = U(r, conveyor(ic, 3.3, 0.24, 0.43));
  if (near(p - vec3(-0.7, 0.6, 2.9), vec3(0.5, 0.65, 1.8), r.x)) r = U(r, messy(p));
  if (near(p - vec3(-0.7, 0.6, 4.15), vec3(0.5, 0.6, 0.5), r.x)) r = U(r, hopper(p));
  vec3 oc = p - vec3(1.4, 0.0, 0.3);
  if (near(oc - vec3(1.45, 0.3, 0.0), vec3(1.5, 0.3, 0.3), r.x)) {
    r = U(r, conveyor(oc, 2.9, 0.22, 0.42));
    r = U(r, card(vec3(repL(p.x - 1.95, 0.44, 5.0), p.y - 0.438, p.z - 0.3)));
  }
  if (near(p - vec3(4.6, 1.4, -1.2), vec3(0.8, 1.45, 0.8), r.x)) r = U(r, silo(p));
  vec3 rb = p - vec3(4.78, 0.0, 0.3);
  if (near(rb - vec3(-0.2, 0.75, 0.5), vec3(0.55, 0.8, 0.75), r.x)) { vec2 a = robot(rb / 1.3); r = U(r, M(a.x * 1.3, a.y)); }
  if (near(p - vec3(4.43, 0.2, 1.3), vec3(0.3, 0.25, 0.3), r.x)) r = U(r, crate(p - vec3(4.43, 0.0, 1.3), 1.0));
  if (near(p - vec3(6.0, 0.45, -0.05), vec3(0.6, 0.5, 0.35), r.x)) {
    r = U(r, pallet(p - vec3(6.0, 0.0, -0.05), vec2(0.56, 0.3)));
    r = U(r, crate(p - vec3(5.72, 0.12, -0.05), 1.0));
    r = U(r, crate(p - vec3(6.28, 0.12, -0.05), 1.0));
    r = U(r, crate(p - vec3(5.72, 0.46, -0.05), 1.0));
  }
  vec3 fk = p - vec3(3.45, 0.0, 2.1);
  if (near(fk - vec3(-0.4, 0.75, 0.0), vec3(0.95, 0.8, 0.4), r.x)) {
    vec2 f = forklift(fk / 1.5);
    r = U(r, M(f.x * 1.5, f.y));
    r = U(r, crate(fk - vec3(-0.72, 0.453, 0.0), 1.0));
  }
  if (near(p - vec3(0.8, 0.6, 1.6), vec3(0.8, 0.6, 0.6), r.x)) r = U(r, dock(p));
  vec3 tq = p - vec3(0.75, 0.0, 2.72);
  if (near(tq - vec3(0.0, 0.5, 0.0), vec3(0.35, 0.55, 0.75), r.x)) r = U(r, truck(vec3(-tq.z, tq.y, tq.x), WHITE, 0.0));
  if (near(p - vec3(1.7, 0.25, 2.8), vec3(0.4, 0.3, 0.4), r.x)) {
    r = U(r, pallet(p - vec3(1.7, 0.0, 2.8), vec2(0.3, 0.3)));
    r = U(r, pallet(p - vec3(1.7, 0.12, 2.8), vec2(0.3, 0.3)));
    r = U(r, pallet(p - vec3(1.72, 0.24, 2.78), vec2(0.3, 0.3)));
  }
  if (near(p - vec3(-2.82, 0.8, 1.1), vec3(0.6, 0.9, 0.95), r.x)) r = U(r, annex(p));
  if (near(p - vec3(6.9, 1.3, -2.2), vec3(0.7, 1.35, 0.7), r.x)) r = U(r, watertower(p - vec3(6.9, 0.0, -2.2)));
  if (near(p - vec3(-2.0, 0.5, 4.15), vec3(0.7, 0.55, 0.35), r.x)) r = U(r, truck(p - vec3(-2.0, 0.0, 4.15), SUN, 1.0));
  if (near(p - vec3(-1.5, 0.5, 2.9), vec3(0.3, 0.55, 0.3), r.x)) r = U(r, lamp(p - vec3(-1.5, 0.0, 2.9)));
  if (near(p - vec3(2.5, 0.5, 1.1), vec3(0.3, 0.55, 0.3), r.x)) r = U(r, lamp(p - vec3(2.5, 0.0, 1.1)));
  if (near(p - vec3(-3.7, 0.5, 2.6), vec3(0.35, 0.55, 0.35), r.x)) r = U(r, tree(p - vec3(-3.7, 0.0, 2.6), 1.0));
  if (near(p - vec3(-4.1, 0.6, 1.5), vec3(0.35, 0.65, 0.35), r.x)) r = U(r, pine(p - vec3(-4.1, 0.0, 1.5), 1.2));
  if (near(p - vec3(0.6, 0.5, 4.3), vec3(0.3, 0.55, 0.3), r.x)) r = U(r, pine(p - vec3(0.6, 0.0, 4.3), 0.85));
  if (near(p - vec3(3.0, 0.5, 3.5), vec3(0.35, 0.55, 0.35), r.x)) r = U(r, tree(p - vec3(3.0, 0.0, 3.5), 0.9));
  if (near(p - vec3(6.4, 0.5, 1.1), vec3(0.35, 0.55, 0.35), r.x)) r = U(r, tree(p - vec3(6.4, 0.0, 1.1), 1.0));
  if (near(p - vec3(7.6, 0.6, -0.8), vec3(0.35, 0.65, 0.35), r.x)) r = U(r, pine(p - vec3(7.6, 0.0, -0.8), 1.2));
  if (near(p - vec3(2.25, 0.1, 2.2), vec3(0.25, 0.15, 0.45), r.x)) { r = U(r, tcone(p - vec3(2.15, 0.0, 1.85))); r = U(r, tcone(p - vec3(2.35, 0.0, 2.55))); }
  if (near(p - vec3(-2.0, 0.2, 2.1), vec3(0.25, 0.25, 0.2), r.x)) { r = U(r, drum(p - vec3(-2.08, 0.0, 2.05))); r = U(r, drum(p - vec3(-1.86, 0.0, 2.15))); }
  if (near(p - vec3(-2.5, 0.15, 2.2), vec3(0.3, 0.2, 0.15), r.x)) r = U(r, bench(p - vec3(-2.5, 0.0, 2.2)));
  return r;
}
