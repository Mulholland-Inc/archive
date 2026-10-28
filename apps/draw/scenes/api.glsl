#pragma frame {"size":[800,640],"center":[0.1,0.12,0.5],"zoom":272,"bounds":[[-1.6,-0.1,-1.0],[1.6,1.0,1.8]]}

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

vec2 plug(vec3 p, vec3 at, vec3 dir, float m) {
  float a = atan(dir.x, dir.z);
  vec3 q = rotY(p - at, a);
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.0, 0.07), vec3(0.042, 0.032, 0.06), 0.012), STEEL);
  r = seam(r, q.z - 0.1);
  r = U(r, M(sdRoundCylinder(rotX(q, PI * 0.5), 0.034, 0.025, 0.01), m + ROUND));
  r = U(r, M(sdBox(vec3(abs(q.x) - 0.018, q.y, q.z - 0.15), vec3(0.007, 0.012, 0.025)), WHITE));
  return r;
}

vec2 switchbox(vec3 p) {
  vec3 b = p - vec3(0.0, 0.2, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.82, 0.15, 0.36), 0.03), WHITE);
  r = seam(r, b.y - 0.1);
  vec3 f = faceZ(p, 0.36);
  r = inset(r, f - vec3(-0.12, 0.0, 0.17), vec2(0.6, 0.075), 0.02, 0.012, STEEL);
  vec3 pq = vec3(repC(f.x + 0.12, 0.15, 8.0), f.y, f.z - 0.155);
  r = inset(r, pq, vec2(0.045, 0.035), 0.008, 0.06, GRAPHITE);
  r = inset(r, f - vec3(0.64, 0.0, 0.17), vec2(0.11, 0.075), 0.02, 0.012, STEEL);
  r = inset(r, vec3(repC(f.x - 0.64, 0.11, 2.0), f.y, f.z - 0.17), vec2(0.035, 0.03), 0.006, 0.06, GRAPHITE);
  vec3 top = faceY(p, 0.35);
  r = vents(r, top - vec3(0.35, 0.0, -0.05), 9.0, 0.05, 0.3, 0.02, GRAPHITE);
  vec3 sc = mirXZ(top) - vec3(0.74, 0.0, 0.28);
  r = U(r, screw(sc, 0.02));
  r = U(r, plate(top - vec3(-0.45, 0.0, 0.0), vec2(0.2, 0.15), 0.025, 0.008, STEEL));
  r = U(r, screw(vec3(abs(top.x + 0.45) - 0.15, top.y - 0.008, top.z), 0.016));
  vec3 lq = vec3(repC(f.x + 0.12, 0.15, 8.0), f.y, f.z - 0.27);
  float li = repI(f.x + 0.12, 0.15, 8.0);
  float lm = li == 4.0 ? LIME : (li == 0.0 || li == 2.0 || li == 6.0) ? MINT : STEEL;
  r = U(r, led(lq, 0.017, lm));
  vec3 ears = vec3(abs(p.x) - 0.87, p.y - 0.2, p.z - 0.3);
  vec2 ear = M(sdRoundBox(ears, vec3(0.05, 0.15, 0.025), 0.01), STEEL);
  ear = Sc(ear, M(sdCylinder(rotX(vec3(ears.x, abs(ears.y) - 0.08, ears.z), PI * 0.5), 0.016, 0.05), GRAPHITE + ROUND));
  r = U(r, ear);
  vec3 ft = vec3(abs(p.x) - 0.68, p.y - 0.025, abs(p.z) - 0.26);
  r = U(r, M(sdRoundCylinder(ft, 0.05, 0.025, 0.01), GRAPHITE + ROUND));
  vec3 fx = faceX(p, 0.82);
  r = U(r, M(extrude(sdRect(fx.xz - vec2(-0.1, 0.2), vec2(0.07, 0.05), 0.015), fx.y - 0.008, 0.008), GRAPHITE));
  r = U(r, led(fx - vec3(0.12, 0.0, 0.2), 0.02, LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.2, 0.0), vec3(0.95, 0.2, 0.42), r.x)) r = U(r, switchbox(p));
  float g = 1e9;
  float l = 1e9;
  vec3 e0 = vec3(-0.8, 0.032, 0.98);
  vec3 e1 = vec3(-0.4, 0.032, 1.28);
  vec3 e2 = vec3(0.3, 0.032, 1.25);
  vec3 e3 = vec3(1.2, 0.032, 0.9);
  vec3 d0 = normalize(vec3(-0.6, 0.0, 1.0));
  vec3 d1 = normalize(vec3(-0.2, 0.0, 1.0));
  vec3 d2 = normalize(vec3(0.35, 0.0, 1.0));
  vec3 d3 = normalize(vec3(1.0, 0.0, 0.45));
  float px0 = -0.12 - 0.525, px1 = -0.12 - 0.225, px2 = -0.12 + 0.075, px3 = -0.12 + 0.375;
  float y = 0.155;
  g = min(g, cable(p, vec3(px0, y, 0.5), vec3(px0, y, 0.75), e0 - d0 * 0.3, e0, 0.024));
  g = min(g, cable(p, vec3(px1, y, 0.5), vec3(px1, y, 0.8), e1 - d1 * 0.35, e1, 0.024));
  l = min(l, cable(p, vec3(px2, y, 0.5), vec3(px2, y, 0.8), e2 - d2 * 0.35, e2, 0.024));
  g = min(g, cable(p, vec3(px3, y, 0.5), vec3(px3, y, 0.75), e3 - d3 * 0.35, e3, 0.024));
  r = U(r, M(g, GRAPHITE + SOLID));
  r = U(r, M(l, LIME + SOLID));
  vec3 pp = vec3(repC(p.x + 0.195, 0.3, 4.0), p.y - y, p.z - 0.42);
  r = U(r, M(sdRoundBox(pp, vec3(0.04, 0.03, 0.06), 0.01), STEEL));
  r = U(r, plug(p, e0, d0, GRAPHITE));
  r = U(r, plug(p, e1, d1, GRAPHITE));
  r = U(r, plug(p, e2, d2, LIME));
  r = U(r, plug(p, e3, d3, GRAPHITE));
  return r;
}
