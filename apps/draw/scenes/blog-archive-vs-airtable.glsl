#pragma frame {"size":[1400,760],"center":[0.0,0.7,0.0],"zoom":175,"bounds":[[-3.0,-0.1,-3.0],[3.0,3.2,3.0]]}

float bar(vec2 p, float x0, float y, float len) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, 0.02), 0.02);
}

vec2 record(vec3 p, float m) {
  vec2 c = M(sdBox(p, vec3(0.007, 0.09, 0.17)), m);
  c = U(c, M(sdRoundBox(p - vec3(0.0, 0.1, -0.07), vec3(0.007, 0.028, 0.055), 0.006), m));
  return c;
}

float grid(vec2 c, vec2 h, vec2 n) {
  vec2 cell = h * 2.0 / n;
  vec2 q = c + h;
  vec2 g = abs(mod(q + cell * 0.5, cell) - cell * 0.5);
  return min(g.x, g.y) - 0.006;
}

vec2 sheetCell(vec2 o, vec3 t, vec2 h, vec2 n, vec2 ij, float m) {
  vec2 cell = h * 2.0 / n;
  vec2 c = -h + cell * (ij + 0.5);
  return decal(o, t, sdRect(t.xz - c, cell * 0.5 - 0.022, 0.012), m);
}

vec2 slab(vec3 p) {
  vec3 b = p - vec3(0.0, 0.4, 0.0);
  vec2 h = vec2(0.95, 0.7);
  vec2 r = M(sdRoundBox(b, vec3(h.x, 0.035, h.y), 0.015), WHITE);
  r = seam(r, b.y + 0.01);
  r = paint(r, b.y + 0.01, STEEL);
  vec3 t = faceY(b, 0.035);
  vec2 n = vec2(6.0, 8.0);
  r = decal(r, t, max(grid(t.xz, h, n), sdRect(t.xz, h - 0.01, 0.0)), STEEL);
  for (int i = 0; i < 6; i++) r = sheetCell(r, t, h, n, vec2(float(i), 0.0), GRAPHITE);
  for (int j = 1; j < 8; j++) r = sheetCell(r, t, h, n, vec2(0.0, float(j)), PAPER);
  r = sheetCell(r, t, h, n, vec2(2.0, 2.0), LILAC);
  r = sheetCell(r, t, h, n, vec2(3.0, 4.0), SUN);
  r = sheetCell(r, t, h, n, vec2(4.0, 2.0), BLUE);
  r = sheetCell(r, t, h, n, vec2(1.0, 5.0), CORAL);
  r = sheetCell(r, t, h, n, vec2(5.0, 6.0), LILAC);
  r = sheetCell(r, t, h, n, vec2(2.0, 6.0), SUN);
  vec3 l = vec3(abs(p.x) - 0.82, p.y - 0.18, abs(p.z) - 0.58);
  r = U(r, M(sdCylinder(l, 0.035, 0.18), STEEL + ROUND));
  r = U(r, M(sdCylinder(l + vec3(0.0, 0.17, 0.0), 0.055, 0.015), GRAPHITE + ROUND));
  return r;
}

vec2 tile(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.15, 0.03, 0.085), 0.012), m);
  vec3 t = faceY(p, 0.03);
  return decal(r, t, sdRect(t.xz - vec2(-0.03, 0.0), vec2(0.08, 0.016), 0.016), m == WHITE ? STEEL : WHITE);
}

vec2 drawerRow(vec3 f, float i) {
  vec2 r = plate(f, vec2(0.44, 0.17), 0.02, 0.012, WHITE);
  vec3 t = f - vec3(0.0, 0.012, 0.0);
  r = U(r, M(pipe(t, vec3(-0.12, 0.025, -0.03), vec3(0.12, 0.025, -0.03), 0.018), STEEL + SOLID));
  r = U(r, M(pipe(t, vec3(-0.12, 0.0, -0.03), vec3(-0.12, 0.025, -0.03), 0.014), STEEL + SOLID));
  r = U(r, M(pipe(t, vec3(0.12, 0.0, -0.03), vec3(0.12, 0.025, -0.03), 0.014), STEEL + SOLID));
  r = U(r, M(extrude(max(sdRect(t.xz - vec2(0.0, 0.08), vec2(0.09, 0.04), 0.008), -sdRect(t.xz - vec2(0.0, 0.08), vec2(0.07, 0.024), 0.004)), t.y - 0.006, 0.006), STEEL));
  r = U(r, led(t - vec3(0.34, 0.0, 0.08), 0.026, i == 1.0 ? LIME : STEEL));
  return r;
}

vec2 rack(vec3 p, float open) {
  vec3 b = p - vec3(0.0, 0.95, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.85, 0.5), 0.03), WHITE);
  r = seam(r, -(abs(b.y) - 0.8));
  r = U(r, M(sdBox(p - vec3(0.0, 0.05, 0.0), vec3(0.44, 0.05, 0.44)), GRAPHITE));
  vec3 fz = faceZ(b, 0.5);
  for (int k = 0; k < 4; k++) {
    float i = float(k);
    vec3 d = fz - vec3(0.0, 0.0, 0.57 - i * 0.38);
    if (k == 1 && open > 0.5) {
      r = Sc(r, M(max(sdRect(d.xz, vec2(0.44, 0.17), 0.012), -(d.y + 0.5)), GRAPHITE));
    } else {
      r = U(r, drawerRow(d, i));
    }
  }
  vec3 fx = faceX(b, 0.5);
  r = inset(r, fx - vec3(0.0, 0.0, 0.25), vec2(0.36, 0.5), 0.03, 0.012, WHITE);
  r = vents(r, vec3(fx.x, fx.y, repC(fx.z - 0.45, 0.07, 4.0)), 5.0, 0.07, 0.04, 0.03, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.3, fx.y, abs(fx.z - 0.25) - 0.44), 0.022));
  vec3 lz = fx - vec3(0.0, 0.0, -0.55);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.024, LIME));
  r = vents(r, fx - vec3(0.0, 0.0, -0.66), 8.0, 0.07, 0.04, 0.03, GRAPHITE);
  if (open < 0.5) return r;
  vec3 dr = b - vec3(0.0, 0.19, 0.42);
  vec2 tray = M(max(sdBox(dr, vec3(0.42, 0.15, 0.36)), -sdBox(dr - vec3(0.0, 0.03, 0.0), vec3(0.4, 0.15, 0.34))), WHITE);
  r = U(r, tray);
  r = U(r, drawerRow(faceZ(dr, 0.36), 1.0));
  vec3 fq = dr - vec3(0.0, 0.0, 0.0);
  float j = repC(fq.z, 0.075, 8.0);
  float ji = repI(fq.z, 0.075, 8.0);
  float lift = ji == 5.0 ? 0.12 : 0.0;
  float m = ji == 5.0 ? LIME : ji == 2.0 ? LILAC : WHITE;
  vec3 rc = vec3(fq.x + (mod(ji, 2.0) * 0.08 - 0.04), fq.y + 0.02 - lift, j);
  r = U(r, record(vec3(rc.z, rc.y, rc.x) * vec3(1.0, 1.0, 0.7), m));
  return r;
}

vec2 plinth(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.09, 0.0), vec3(1.2, 0.09, 1.2), 0.05), WHITE);
  r = seam(r, p.y - 0.13);
  r = paint(r, p.y - 0.05, STEEL);
  r = vents(r, faceX(p, 1.2) - vec3(-0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  r = vents(r, faceZ(p, 1.2) - vec3(0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  vec3 lz = faceZ(p, 1.2) - vec3(-0.8, 0.0, 0.095);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.022, LIME));
  r = U(r, screw(faceY(mirXZ(p), 0.18) - vec3(1.09, 0.0, 1.09), 0.026));
  return r;
}

vec2 cube(vec3 p, float s, float m) {
  return M(sdRoundBox(p - vec3(0.0, s, 0.0), vec3(s), s * 0.12), m);
}

vec2 cart(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.12, 0.0), vec3(0.3, 0.025, 0.22), 0.01), SUN);
  vec3 w = vec3(abs(p.x) - 0.2, p.y - 0.06, abs(p.z) - 0.2);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.06, 0.022), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.024, 0.028), STEEL + SOLID));
  vec3 h = p - vec3(-0.3, 0.0, 0.0);
  r = U(r, M(sdBox(vec3(h.x, h.y - 0.31, abs(h.z) - 0.16), vec3(0.014, 0.17, 0.014)), GRAPHITE));
  r = U(r, M(pipe(h, vec3(0.0, 0.48, -0.18), vec3(0.0, 0.48, 0.18), 0.02), GRAPHITE + SOLID));
  r = U(r, cube(p - vec3(-0.06, 0.145, -0.05), 0.11, WHITE));
  r = U(r, cube(rotY(p - vec3(0.17, 0.145, 0.08), 0.5), 0.08, LILAC));
  r = U(r, cube(rotY(p - vec3(-0.05, 0.365, -0.03), 0.3), 0.075, WHITE));
  return r;
}

vec2 sheet(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.22, 0.012, 0.28), 0.006), WHITE);
  vec3 t = faceY(p, 0.012);
  r = decal(r, t, sdRect(t.xz - vec2(-0.05, -0.19), vec2(0.12, 0.025), 0.025), GRAPHITE);
  r = decal(r, t, bar(t.xz, -0.15, -0.1, 0.28), STEEL);
  r = decal(r, t, bar(t.xz, -0.15, -0.03, 0.22), STEEL);
  r = decal(r, t, bar(t.xz, -0.15, 0.04, 0.26), STEEL);
  r = decal(r, t, bar(t.xz, -0.15, 0.11, 0.16), STEEL);
  return r;
}

vec2 folder(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.2, 0.014, 0.15), 0.006), SUN);
  r = U(r, M(sdRoundBox(p - vec3(-0.1, 0.0, -0.17), vec3(0.07, 0.014, 0.03), 0.006), SUN));
  r = U(r, M(sdBox(p - vec3(0.02, 0.022, 0.01), vec3(0.16, 0.006, 0.12)), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.04, 0.02), vec3(0.2, 0.012, 0.14), 0.006), SUN));
  return r;
}

vec2 arm(vec3 p, out vec3 grip) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.16, 0.04, 0.012), GRAPHITE + ROUND);
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.12, 0.0), 0.11, 0.05, 0.015), WHITE + ROUND));
  vec3 a = vec3(0.0, 0.2, 0.0);
  vec3 e = vec3(0.08, 0.64, -0.08);
  vec3 w = vec3(0.36, 0.5, -0.34);
  r = U(r, M(sdSphere(p - a, 0.085), STEEL + SOLID));
  r = U(r, M(pipe(p, a, e, 0.06), WHITE + SOLID));
  r = U(r, M(sdSphere(p - e, 0.07), LIME + SOLID));
  r = U(r, M(pipe(p, e, w, 0.045), WHITE + SOLID));
  r = U(r, M(sdSphere(p - w, 0.05), STEEL + SOLID));
  vec3 g = p - w - vec3(0.0, -0.07, 0.0);
  r = U(r, M(sdRoundBox(g, vec3(0.06, 0.025, 0.06), 0.01), GRAPHITE));
  vec3 fn = vec3(g.x, g.y + 0.07, abs(g.z) - 0.05);
  r = U(r, M(sdBox(fn, vec3(0.03, 0.06, 0.008)), GRAPHITE));
  grip = w - vec3(0.0, 0.16, 0.0);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 a = p - vec3(-1.05, 0.0, 1.05);
  r = U(r, slab(a));
  r = U(r, tile(rotY(a - vec3(1.15, 0.03, 0.55), 0.3), LILAC));
  r = U(r, tile(rotY(a - vec3(1.2, 0.09, 0.5), -0.15), WHITE));
  r = U(r, tile(rotZ(rotY(a - vec3(1.05, 0.1, 0.15), 0.5), 0.35), SUN));
  vec3 b = p - vec3(1.0, 0.0, -1.0);
  if (near(b - vec3(0.0, 1.3, 0.0), vec3(1.25, 1.5, 1.25), r.x)) {
    r = U(r, plinth(b));
    vec3 g = b - vec3(0.0, 0.18, 0.0);
    r = U(r, rack(g - vec3(-0.35, 0.0, 0.32), 1.0));
    r = U(r, rack(g - vec3(-0.35, 0.0, -0.82), 0.0));
    r = U(r, cart(rotY(g - vec3(0.78, 0.0, 0.82), 0.35)));
    vec3 top = g - vec3(-0.35, 1.8, 0.32);
    r = U(r, sheet(rotY(top - vec3(0.2, 0.012, 0.24), 0.1)));
    r = U(r, folder(rotY(top - vec3(0.26, 0.0, -0.26), -0.1)));
    vec3 grip;
    float k = 1.25;
    vec3 ab = (top - vec3(-0.28, 0.0, 0.26)) / k;
    vec2 ar = arm(ab, grip);
    r = U(r, vec2(ar.x * k, ar.y));
    vec3 c = ab - grip;
    r = U(r, M(sdBox(c - vec3(0.0, 0.03, 0.0), vec3(0.008, 0.09, 0.13)) * k, LIME));
  }
  return r;
}
