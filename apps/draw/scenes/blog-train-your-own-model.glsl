#pragma frame {"size":[1400,760],"center":[0.0,0.6,0.0],"zoom":215,"bounds":[[-3.0,-0.1,-3.0],[3.0,2.6,3.0]]}

vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  float s = 1.0 - t;
  return s * s * s * a + 3.0 * s * s * t * b + 3.0 * s * t * t * c + t * t * t * d;
}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float m = 1e9;
  vec3 q = a;
  for (int i = 1; i <= 16; i++) {
    vec3 n = bez(a, b, c, d, float(i) / 16.0);
    m = min(m, sdCapsule(p, q, n, r));
    q = n;
  }
  return m;
}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.12, 0.007, 0.16)), m);
  vec3 q = faceY(p, 0.007);
  float bar = m == WHITE ? STEEL : WHITE;
  s = decal(s, q, sdRect(q.xz - vec2(0.0, -0.08), vec2(0.07, 0.01), 0.01), bar);
  s = decal(s, q, sdRect(q.xz - vec2(-0.02, -0.03), vec2(0.05, 0.01), 0.01), bar);
  return decal(s, q, sdRect(q.xz - vec2(0.0, 0.07), vec2(0.07, 0.04), 0.01), bar);
}

vec2 chute(vec3 p, vec3 a, vec3 b, float m) {
  vec3 d = b - a;
  vec3 q = rotY(p - a, -atan(d.z, d.x));
  float L = length(d.xz);
  q = rotZ(q, atan(d.y, L));
  float l = length(d);
  vec3 c = q - vec3(l * 0.5, 0.0, 0.0);
  float body = sdBox(c, vec3(l * 0.5, 0.035, 0.13));
  float cut = sdBox(c - vec3(0.0, 0.04, 0.0), vec3(l * 0.5 + 0.05, 0.04, 0.1));
  vec2 r = M(max(body, -cut), STEEL);
  r = U(r, sheet(rotY(c - vec3(0.05, 0.0, 0.0), PI * 0.5 + 0.1), m));
  return r;
}

vec2 bin(vec3 p, float m) {
  vec3 q = p - vec3(0.0, 0.2, 0.0);
  vec2 c = M(max(sdRoundBox(q, vec3(0.27, 0.2, 0.27), 0.015), -sdBox(q - vec3(0.0, 0.05, 0.0), vec3(0.24, 0.2, 0.24))), WHITE);
  c = seam(c, q.y + 0.12);
  c = paint(c, 0.14 - q.y, m);
  vec3 fx = faceX(q, 0.27);
  c = U(c, screw(fx - vec3(0.17, 0.0, -0.03), 0.018));
  c = U(c, screw(fx - vec3(-0.17, 0.0, -0.03), 0.018));
  vec3 fz = faceZ(q, 0.27);
  c = U(c, screw(fz - vec3(0.17, 0.0, -0.03), 0.018));
  c = U(c, screw(fz - vec3(-0.17, 0.0, -0.03), 0.018));
  for (int i = 0; i < 6; i++) {
    float fi = float(i);
    c = U(c, sheet(rotY(q - vec3(0.02 * sin(fi * 1.7), -0.02 + 0.025 * fi, 0.02 * cos(fi * 2.3)), 0.35 * sin(fi * 2.9)), m));
  }
  c = U(c, sheet(rotX(rotY(q - vec3(-0.02, 0.17, -0.02), 0.5), 0.12), m));
  return c;
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
  r = U(r, led(fx - vec3(0.07, 0.0, 0.9), 0.026, SUN));
  r = U(r, led(fx - vec3(0.15, 0.0, 0.9), 0.026, LILAC));
  r = U(r, led(fx - vec3(0.23, 0.0, 0.9), 0.026, CORAL));
  r = U(r, screw(fz - vec3(0.2, 0.0, 0.9), 0.018));
  r = U(r, M(sdBox(fz - vec3(-0.06, 0.0, 0.9), vec3(0.14, 0.004, 0.006)), LIME + SOLID));
  vec3 h = p - vec3(0.0, 1.06, 0.0);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.04, 0.0), 0.12, 0.05), STEEL + ROUND));
  vec3 cq = h - vec3(0.0, 0.32, 0.0);
  float outer = sdCone(cq, 0.22, 0.12, 0.42);
  float inner = sdCone(cq - vec3(0.0, 0.03, 0.0), 0.22, 0.09, 0.39);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq - vec3(0.0, 0.22, 0.0), 0.42, 0.025), STEEL + SOLID));
  r = U(r, sheet(rotZ(rotY(cq - vec3(0.06, 0.04, 0.02), 0.6), 0.45), CORAL));
  r = U(r, sheet(rotX(rotY(cq - vec3(-0.1, 0.07, -0.06), -0.3), -0.5), SUN));
  return r;
}

vec2 model(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.1, 0.0), 0.24, 0.1, 0.02), WHITE + ROUND);
  r = seam(r, p.y - 0.15);
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.21, 0.0), 0.19, 0.015, 0.008), STEEL + ROUND));
  vec3 c = p - vec3(0.0, 0.28, 0.0);
  r = U(r, M(sdChamferBox(c, vec3(0.13, 0.045, 0.13), 0.015), LIME));
  vec3 cq = faceY(c, 0.045);
  r = U(r, M(sdChamferBox(cq - vec3(0.0, 0.012, 0.0), vec3(0.07, 0.012, 0.07), 0.006), LIME));
  vec3 px = vec3(abs(c.x) - 0.15, c.y + 0.01, repC(c.z, 0.06, 4.0));
  r = U(r, M(sdBox(px, vec3(0.022, 0.008, 0.012)), STEEL));
  vec3 pz = vec3(repC(c.x, 0.06, 4.0), c.y + 0.01, abs(c.z) - 0.15);
  r = U(r, M(sdBox(pz, vec3(0.012, 0.008, 0.022)), STEEL));
  vec3 f = faceX(rotY(p, -PI * 0.25), 0.24);
  r = U(r, led(f - vec3(0.0, 0.0, 0.1), 0.026, LIME));
  return r;
}

vec2 stack(vec3 p, float n, float m) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 9; i++) {
    float fi = float(i);
    if (fi >= n) break;
    vec3 q = rotY(p - vec3(0.0, 0.016 + fi * 0.034, 0.0), 0.07 * sin(fi * 2.9 + m));
    vec2 k = M(sdRoundBox(q, vec3(0.15, 0.014, 0.2), 0.008), WHITE);
    k = paint(k, -(q.x - 0.11), i == int(n) - 1 ? m : STEEL);
    r = U(r, k);
  }
  return r;
}

vec2 lessons(vec3 p) {
  vec3 q = p - vec3(0.0, 0.05, 0.0);
  vec2 r = M(sdRoundBox(q, vec3(0.62, 0.05, 0.28), 0.015), STEEL);
  r = U(r, M(sdBox(vec3(repC(q.x, 0.4, 4.0), q.y + 0.07, q.z), vec3(0.05, 0.03, 0.26)), STEEL));
  vec3 s = p - vec3(0.0, 0.1, 0.0);
  r = U(r, stack(s - vec3(-0.4, 0.0, 0.0), 7.0, SUN));
  r = U(r, stack(s - vec3(0.0, 0.0, 0.02), 9.0, STEEL));
  r = U(r, stack(s - vec3(0.4, 0.0, -0.01), 5.0, LILAC));
  return r;
}

vec2 trainer(vec3 p) {
  vec3 b = p - vec3(0.0, 0.4, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.36, 0.4, 0.36), 0.025), WHITE);
  r = seam(r, b.y + 0.28);
  r = seam(r, b.y - 0.3);
  vec3 fz = faceZ(p, 0.36);
  vec3 w = fz - vec3(-0.05, 0.0, 0.48);
  r = inset(r, w, vec2(0.2, 0.14), 0.04, 0.02, GRAPHITE);
  r = U(r, M(extrude(max(sdRect(w.xz, vec2(0.24, 0.18), 0.06), -sdRect(w.xz, vec2(0.2, 0.14), 0.04)), w.y - 0.012, 0.012), STEEL));
  vec3 g = vec3(w.x, w.y + 0.012, w.z);
  r = U(r, M(extrude(abs(length(g.xz - vec2(-0.06, 0.0)) - 0.07) - 0.018, g.y, 0.008), STEEL + SOLID));
  r = U(r, M(extrude(abs(length(g.xz - vec2(0.09, 0.03)) - 0.045) - 0.014, g.y, 0.008), SUN + SOLID));
  r = U(r, led(fz - vec3(0.27, 0.0, 0.62), 0.024, STEEL));
  r = U(r, led(fz - vec3(0.27, 0.0, 0.52), 0.024, STEEL));
  r = vents(r, fz - vec3(0.0, 0.0, 0.17), 6.0, 0.07, 0.1, 0.024, GRAPHITE);
  vec3 fx = faceX(p, 0.36);
  r = U(r, M(extrude(sdRect(fx.xz - vec2(0.0, 0.45), vec2(0.12, 0.08), 0.02), fx.y - 0.015, 0.015), STEEL));
  r = U(r, button(vec3(fx.x, fx.z - 0.45, fx.y - 0.03), 0.045, 0.02, GRAPHITE));
  r = U(r, screw(vec3(abs(fx.x) - 0.26, fx.y, fx.z - 0.15), 0.016));
  r = U(r, screw(vec3(abs(fx.x) - 0.26, fx.y, fx.z - 0.68), 0.016));
  vec3 h = p - vec3(0.0, 0.8, 0.0);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.04, 0.0), 0.12, 0.04), STEEL + ROUND));
  vec3 cq = h - vec3(0.0, 0.22, 0.0);
  float outer = sdCone(cq, 0.14, 0.12, 0.3);
  float inner = sdCone(cq - vec3(0.0, 0.03, 0.0), 0.14, 0.09, 0.27);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq - vec3(0.0, 0.14, 0.0), 0.3, 0.02), STEEL + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 lp = p - vec3(-1.45, 0.0, 0.95);
  vec3 lq = rotY(lp, PI * 0.5);
  if (near(lq - vec3(0.0, 0.25, 0.0), vec3(0.7, 0.35, 0.35), r.x)) r = U(r, lessons(lq));
  vec3 tp = p - vec3(-0.85, 0.0, -0.1);
  if (near(tp - vec3(0.0, 0.6, 0.0), vec3(0.45, 0.65, 0.45), r.x)) r = U(r, trainer(tp));
  if (near(p - vec3(-0.85, 1.35, -0.1), vec3(0.35, 0.3, 0.35), r.x)) {
    r = U(r, sheet(rotZ(rotX(rotY(p - vec3(-0.8, 1.25, -0.05), 0.4), 0.35), -0.25), STEEL));
    r = U(r, sheet(rotX(rotZ(rotY(p - vec3(-0.95, 1.42, -0.2), -0.7), 0.25), 0.2), LILAC));
    r = U(r, sheet(rotZ(rotX(rotY(p - vec3(-0.78, 1.55, -0.18), 1.2), -0.3), 0.2), SUN));
  }
  vec3 mp = (p - vec3(0.1, 0.0, 0.95)) / 1.4;
  if (near(mp - vec3(0.0, 0.3, 0.0), vec3(0.3, 0.32, 0.3), r.x)) r = U(r, model(mp) * vec2(1.4, 1.0));
  vec3 sp = p - vec3(0.85, 0.0, -0.95);
  if (near(sp - vec3(0.0, 0.85, 0.0), vec3(0.5, 0.85, 0.5), r.x)) r = U(r, sorter(sp));
  vec3 b1 = sp - vec3(0.0, 0.0, 1.3);
  vec3 b2 = sp - vec3(0.95, 0.0, 0.95);
  vec3 b3 = sp - vec3(1.3, 0.0, 0.0);
  if (near(b1 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b1, SUN));
  if (near(b2 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b2, STEEL));
  if (near(b3 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b3, LILAC));
  if (near(sp - vec3(0.6, 0.6, 0.6), vec3(0.9, 0.2, 0.9), r.x)) {
    r = U(r, chute(sp, vec3(0.0, 0.7, 0.3), vec3(0.0, 0.5, 1.2), SUN));
    r = U(r, chute(sp, vec3(0.22, 0.7, 0.22), vec3(0.88, 0.5, 0.88), STEEL));
    r = U(r, chute(sp, vec3(0.3, 0.7, 0.0), vec3(1.2, 0.5, 0.0), LILAC));
  }
  if (near(sp - vec3(0.0, 1.9, 0.0), vec3(0.45, 0.4, 0.45), r.x)) {
    r = U(r, sheet(rotZ(rotX(rotY(sp - vec3(0.1, 1.7, 0.12), 0.4), 0.35), -0.25), LILAC));
    r = U(r, sheet(rotX(rotZ(rotY(sp - vec3(-0.18, 1.92, -0.14), -0.7), 0.25), 0.2), WHITE));
    r = U(r, sheet(rotZ(rotX(rotY(sp - vec3(0.14, 2.08, -0.22), 1.2), -0.3), 0.2), SUN));
  }
  float c = 1e9;
  c = min(c, cable(p, vec3(-0.49, 0.05, -0.1), vec3(-0.3, 0.03, -0.1), vec3(-0.35, 0.03, 0.95), vec3(-0.22, 0.05, 0.95), 0.026));
  c = min(c, cable(p, vec3(0.3, 0.05, 0.75), vec3(0.42, 0.03, 0.5), vec3(0.42, 0.03, -0.75), vec3(0.6, 0.05, -0.75), 0.026));
  r = U(r, M(c, CORAL + SOLID));
  return r;
}
