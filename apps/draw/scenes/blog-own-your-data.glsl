#pragma frame {"size":[1400,760],"center":[0.25,0.7,-0.15],"zoom":185,"bounds":[[-3.0,-0.1,-1.8],[3.0,2.6,2.2]]}

const float TOP = 0.34;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 slab(vec3 p) {
  vec3 b = p - vec3(0.0, TOP * 0.5, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(2.1, TOP * 0.5, 1.15), 0.04), WHITE);
  r = seam(r, p.y - 0.09);
  r = paint(r, p.y - 0.09, STEEL);
  r = seam(r, p.y - TOP + 0.05);
  vec3 fz = faceZ(p, 1.15);
  r = vents(r, fz - vec3(0.95, 0.0, 0.2), 9.0, 0.07, 0.1, 0.026, GRAPHITE);
  r = U(r, led(fz - vec3(1.75, 0.0, 0.21), 0.028, LIME));
  r = U(r, led(fz - vec3(1.85, 0.0, 0.21), 0.028, LIME));
  vec3 fx = faceX(p, 2.1);
  r = vents(r, fx - vec3(-0.45, 0.0, 0.21), 6.0, 0.07, 0.1, 0.026, GRAPHITE);
  vec3 h = fx - vec3(0.45, 0.0, 0.21);
  vec2 handle = M(sdTorus(vec3(h.x, h.z, h.y), 0.22, 0.03), GRAPHITE + SOLID);
  handle = M(max(handle.x, -h.y + 0.0), handle.y);
  r = U(r, handle);
  r = U(r, M(sdRoundBox(vec3(abs(h.x) - 0.22, h.y - 0.02, h.z), vec3(0.05, 0.02, 0.05), 0.01), STEEL));
  vec3 cs = mirXZ(faceY(p, TOP));
  r = U(r, screw(vec3(cs.x - 1.98, cs.y, cs.z - 1.03), 0.026));
  return r;
}

vec2 lock(vec3 p) {
  vec3 fz = faceZ(p, 1.15) - vec3(-1.25, 0.0, 0.19);
  vec2 r = plate(fz, vec2(0.17, 0.11), 0.03, 0.022, CORAL);
  vec3 t = fz - vec3(0.0, 0.022, 0.0);
  r = Sc(r, M(max(min(length(t.xz - vec2(0.0, 0.025)) - 0.03, sdRect(t.xz - vec2(0.0, -0.02), vec2(0.012, 0.045), 0.006)), -(t.y + 0.03)), GRAPHITE));
  r = U(r, screw(vec3(abs(t.x) - 0.13, t.y, t.z), 0.018));
  vec3 k = p - vec3(-1.25, TOP * 0.5 + 0.025, 1.17);
  vec2 key = M(sdCylinder(rotX(k - vec3(0.0, 0.0, 0.12), PI * 0.5), 0.03, 0.12), LIME + ROUND);
  key = U(key, M(sdRoundBox(k - vec3(0.0, 0.0, 0.27), vec3(0.03, 0.06, 0.04), 0.015), LIME));
  vec3 bw = rotZ(k - vec3(0.0, 0.0, 0.47), PI * 0.5);
  key = U(key, M(sdTorus(vec3(bw.x, bw.y, bw.z), 0.15, 0.045), LIME + SOLID));
  vec3 ring = k - vec3(0.0, -0.17, 0.62);
  key = U(key, M(sdTorus(rotX(ring, 0.5), 0.07, 0.012), GRAPHITE + SOLID));
  vec3 tag = rotX(k - vec3(0.0, -0.28, 0.67), 0.15);
  key = U(key, M(sdRoundBox(tag, vec3(0.012, 0.07, 0.05), 0.01), WHITE));
  return U(r, key);
}

vec2 drum(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.03, 0.0), 0.5, 0.03, 0.01), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.6, 0.0), 0.36, 0.5), GRAPHITE + ROUND));
  for (int i = 0; i < 3; i++) {
    vec3 q = p - vec3(0.0, 0.22 + 0.37 * float(i), 0.0);
    vec2 d = M(sdRoundCylinder(q, 0.46, 0.15, 0.04), WHITE + ROUND);
    vec3 f = faceX(rotY(q, -PI * 0.25), 0.46);
    d = vents(d, f - vec3(-0.12, 0.0, 0.0), 4.0, 0.06, 0.13, 0.026, GRAPHITE);
    d = U(d, led(f - vec3(0.13, 0.0, 0.0), 0.03, i == 2 ? LIME : STEEL));
    r = U(r, d);
  }
  vec3 t = p - vec3(0.0, 1.15, 0.0);
  r = U(r, M(sdRoundCylinder(t, 0.3, 0.04, 0.015), STEEL + ROUND));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.1, 0.0), 0.08, 0.06), STEEL + ROUND));
  return r;
}

vec2 folderTab(vec3 q, float m) {
  vec2 f = M(sdBox(q, vec3(0.012, 0.11, 0.17)), m);
  f = U(f, M(sdRoundBox(q - vec3(0.0, 0.12, -0.06), vec3(0.012, 0.03, 0.05), 0.008), m));
  return f;
}

vec2 cabinet(vec3 p) {
  vec3 c = p - vec3(0.0, 0.6, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.42, 0.6, 0.36), 0.03), WHITE);
  vec3 fz = faceZ(c, 0.36);
  float i = repI(fz.z, 0.38, 3.0);
  vec3 d = vec3(fz.x, fz.y, repC(fz.z, 0.38, 3.0));
  r = seam(r, -(abs(d.z) - 0.17));
  r = seam(r, -sdRect(d.xz, vec2(0.36, 0.17), 0.02));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.02, 0.06), vec3(0.11, 0.02, 0.025), 0.012), i == 1.0 ? STEEL : SUN));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.004, -0.06), vec3(0.08, 0.004, 0.03), 0.004), STEEL));
  vec3 dw = c - vec3(0.0, 0.0, 0.36 + 0.24);
  vec3 dwq = dw;
  vec2 drawer = M(max(sdBox(dwq, vec3(0.36, 0.17, 0.25)), -sdBox(dwq - vec3(0.0, 0.05, 0.0), vec3(0.33, 0.17, 0.22))), WHITE);
  vec3 df = faceZ(dw, 0.25);
  drawer = U(drawer, M(sdRoundBox(df - vec3(0.0, 0.02, 0.06), vec3(0.11, 0.02, 0.025), 0.012), SUN));
  r = U(r, drawer);
  vec3 ft = vec3(dw.z, dw.y - 0.04, dw.x);
  ft = vec3(repC(dw.z, 0.07, 6.0), dw.y - 0.02, dw.x);
  float fi = repI(dw.z, 0.07, 6.0);
  float m = fi == 1.0 ? LIME : fi == 3.0 ? LILAC : fi == 4.0 ? MINT : SUN;
  vec2 tabs = folderTab(vec3(ft.x, ft.y, ft.z - 0.0), m);
  tabs = M(max(tabs.x, abs(dw.z) - 0.21), tabs.y);
  r = U(r, tabs);
  vec3 fx = faceX(c, 0.42);
  r = vents(r, fx - vec3(0.0, 0.0, 0.45), 5.0, 0.07, 0.12, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.28, fx.y, fx.z + 0.48), 0.02));
  vec3 t = faceY(c, 0.6);
  r = U(r, M(sdRoundBox(t - vec3(0.0, 0.03, 0.0), vec3(0.2, 0.03, 0.14), 0.015), STEEL));
  return r;
}

vec2 dock(vec3 p) {
  vec3 c = p - vec3(0.0, 0.32, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.34, 0.32, 0.32), 0.03), WHITE);
  r = seam(r, c.y + 0.2);
  r = paint(r, c.y + 0.2, STEEL);
  vec3 fx = faceX(c, 0.34);
  float si = repI(fx.x, 0.18, 3.0);
  vec3 s = vec3(repC(fx.x, 0.18, 3.0), fx.y, fx.z - 0.05);
  r = U(r, plate(s, vec2(0.06, 0.08), 0.02, 0.012, GRAPHITE));
  r = U(r, led(fx - vec3(0.0, 0.0, 0.2), 0.028, LILAC));
  r = U(r, led(fx - vec3(0.12, 0.0, 0.2), 0.028, LIME));
  r = U(r, led(fx - vec3(-0.12, 0.0, 0.2), 0.028, STEEL));
  vec3 fz = faceZ(c, 0.32);
  r = vents(r, fz - vec3(0.0, 0.0, 0.08), 6.0, 0.07, 0.16, 0.026, GRAPHITE);
  vec3 m = p - vec3(0.0, 0.64, 0.0);
  r = U(r, M(sdRoundCylinder(m - vec3(0.0, 0.03, 0.0), 0.16, 0.03, 0.01), STEEL + ROUND));
  r = U(r, M(sdCylinder(m - vec3(0.0, 0.32, 0.0), 0.035, 0.3), GRAPHITE + ROUND));
  vec3 dq = rotZ(rotY(m - vec3(0.0, 0.62, 0.0), -PI * 0.25), -0.6);
  float dish = max(sdSphere(dq - vec3(0.0, 0.28, 0.0), 0.36), -sdSphere(dq - vec3(0.0, 0.31, 0.0), 0.35));
  dish = max(dish, dq.y - 0.12);
  r = U(r, M(dish, LILAC + SOLID));
  r = U(r, M(pipe(dq, vec3(0.0, -0.02, 0.0), vec3(0.0, 0.24, 0.0), 0.016), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(dq - vec3(0.0, 0.26, 0.0), 0.04), SUN + SOLID));
  r = U(r, M(sdTorus(m - vec3(0.0, 0.45, 0.0), 0.06, 0.014), LILAC + SOLID));
  return r;
}

float wire(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d) {
  return min(min(pipe(p, a, b, 0.02), pipe(p, b, c, 0.02)), pipe(p, c, d, 0.02));
}

vec2 plugs(vec3 p) {
  const float Y = 0.71, X = 1.59, L = TOP + 0.022;
  vec2 r = M(1e9, 0.0);
  r = U(r, M(sdRoundBox(p - vec3(X + 0.05, Y, -0.48), vec3(0.05, 0.045, 0.045), 0.012), LILAC));
  r = U(r, M(sdRoundBox(p - vec3(X + 0.05, Y, -0.3), vec3(0.05, 0.045, 0.045), 0.012), MINT));
  r = U(r, M(sdRoundBox(p - vec3(X + 0.05, Y, -0.12), vec3(0.05, 0.045, 0.045), 0.012), BLUE));
  float w = wire(p, vec3(X + 0.1, Y, -0.48), vec3(1.96, Y, -0.48), vec3(1.96, L, -0.48), vec3(1.96, L, -0.62));
  w = min(w, wire(p, vec3(X + 0.1, Y, -0.3), vec3(1.84, Y, -0.3), vec3(1.84, L, -0.3), vec3(1.84, L, 0.6)));
  w = min(w, wire(p, vec3(X + 0.1, Y, -0.12), vec3(1.72, Y, -0.12), vec3(1.72, L, -0.12), vec3(1.72, L, 0.2)));
  w = min(w, wire(p, vec3(1.72, L, 0.2), vec3(1.2, L, 0.2), vec3(1.2, L, 0.66), vec3(1.2, L, 0.66)));
  r = U(r, M(w, GRAPHITE + SOLID));
  return r;
}

vec2 pod(vec3 p, float m) {
  vec3 c = p - vec3(0.0, 0.13, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.17, 0.13, 0.15), 0.04), m);
  r = seam(r, c.y + 0.07);
  vec3 f = faceX(c, 0.17);
  r = inset(r, f - vec3(0.0, 0.0, 0.03), vec2(0.1, 0.05), 0.03, 0.015, GRAPHITE);
  r = U(r, led(f - vec3(-0.04, 0.0, 0.03), 0.018, WHITE));
  r = U(r, led(f - vec3(0.04, 0.0, 0.03), 0.018, WHITE));
  r = U(r, M(sdCylinder(c - vec3(0.0, 0.18, 0.0), 0.012, 0.06), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(c - vec3(0.0, 0.25, 0.0), 0.03), WHITE + SOLID));
  return r;
}

vec2 stack(vec3 p) {
  vec2 r = M(1e9, 0.0);
  r = U(r, M(sdRoundBox(rotY(p - vec3(0.0, 0.02, 0.0), 0.1), vec3(0.2, 0.02, 0.15), 0.008), WHITE));
  r = U(r, M(sdRoundBox(rotY(p - vec3(0.01, 0.055, 0.0), -0.08), vec3(0.2, 0.015, 0.15), 0.008), SUN));
  r = U(r, M(sdRoundBox(rotY(p - vec3(-0.01, 0.085, 0.01), 0.18), vec3(0.19, 0.015, 0.14), 0.008), WHITE));
  vec3 t = rotY(p - vec3(-0.01, 0.1, 0.01), 0.18);
  r = decal(r, t, bar(t.xz, -0.14, -0.06, 0.2, 0.014), STEEL);
  r = decal(r, t, bar(t.xz, -0.14, 0.0, 0.14, 0.014), STEEL);
  r = decal(r, t, bar(t.xz, -0.14, 0.06, 0.18, 0.014), STEEL);
  return r;
}

vec2 plant(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.16, 0.0), 0.15, 0.16), CORAL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.3, 0.0), 0.17, 0.035), CORAL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.33, 0.0), 0.14, 0.01), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.5, 0.0), 0.018, 0.18), GRAPHITE + SOLID));
  vec2 leaves = M(sdEllipsoid(rotZ(p - vec3(0.1, 0.62, 0.0), -0.7), vec3(0.06, 0.16, 0.035)), MINT + SOLID);
  leaves = U(leaves, M(sdEllipsoid(rotZ(p - vec3(-0.1, 0.72, 0.02), 0.7), vec3(0.06, 0.17, 0.035)), MINT + SOLID));
  leaves = U(leaves, M(sdEllipsoid(rotX(p - vec3(0.0, 0.8, 0.08), 0.6), vec3(0.035, 0.17, 0.06)), MINT + SOLID));
  leaves = U(leaves, M(sdEllipsoid(rotX(p - vec3(0.0, 0.58, -0.1), -0.7), vec3(0.035, 0.15, 0.06)), MINT + SOLID));
  return U(r, leaves);
}

vec2 laptop(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.02, 0.0), vec3(0.26, 0.02, 0.18), 0.012), STEEL);
  vec3 k = faceY(p, 0.04);
  r = grille(r, k - vec3(0.0, 0.0, -0.04), vec2(6.0, 3.0), 0.07, 0.022, WHITE);
  r = decal(r, k, sdRect(k.xz - vec2(0.0, 0.11), vec2(0.07, 0.035), 0.01), WHITE);
  vec3 s = rotX(p - vec3(0.0, 0.03, -0.18), -0.28) - vec3(0.0, 0.18, 0.0);
  r = U(r, M(sdRoundBox(s, vec3(0.26, 0.18, 0.012), 0.012), STEEL));
  vec3 sf = faceZ(s, 0.012);
  r = inset(r, sf, vec2(0.23, 0.15), 0.01, 0.006, WHITE);
  r = decal(r, sf, bar(sf.xz, -0.18, 0.08, 0.22, 0.016), LIME);
  r = decal(r, sf, bar(sf.xz, -0.18, 0.02, 0.3, 0.016), STEEL);
  r = decal(r, sf, bar(sf.xz, -0.18, -0.04, 0.16, 0.016), STEEL);
  r = decal(r, sf, bar(sf.xz, -0.18, -0.1, 0.26, 0.016), STEEL);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = slab(p);
  vec3 g = p - vec3(0.0, TOP, 0.0);
  if (near(p - vec3(-1.25, TOP * 0.5, 1.5), vec3(0.25, 0.3, 0.45), r.x)) r = U(r, lock(p));
  if (near(g - vec3(-1.25, 0.65, -0.3), vec3(0.55, 0.7, 0.55), r.x)) r = U(r, drum(g - vec3(-1.25, 0.0, -0.3)));
  if (near(g - vec3(0.05, 0.65, 0.0), vec3(0.47, 0.7, 0.9), r.x)) r = U(r, cabinet(g - vec3(0.05, 0.0, -0.35)));
  if (near(g - vec3(1.25, 0.9, -0.3), vec3(0.6, 1.0, 0.5), r.x)) r = U(r, dock(g - vec3(1.25, 0.0, -0.3)));
  if (near(p - vec3(1.6, 0.45, 0.0), vec3(0.7, 0.5, 0.8), r.x)) r = U(r, plugs(p));
  if (near(g - vec3(1.84, 0.2, 0.72), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, pod(rotY(g - vec3(1.84, 0.0, 0.72), 0.0), MINT));
  if (near(g - vec3(1.2, 0.2, 0.78), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, pod(rotY(g - vec3(1.2, 0.0, 0.78), 0.0), BLUE));
  if (near(g - vec3(1.84, 0.2, -0.8), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, pod(g - vec3(1.84, 0.0, -0.82), LILAC));
  if (near(g - vec3(-1.8, 0.5, 0.78), vec3(0.25, 0.55, 0.25), r.x)) r = U(r, plant(g - vec3(-1.8, 0.0, 0.78)));
  if (near(g - vec3(-0.25, 0.2, 0.75), vec3(0.3, 0.25, 0.3), r.x)) r = U(r, laptop(rotY(g - vec3(-0.25, 0.0, 0.78), 0.25)));
  if (near(g - vec3(-1.0, 0.1, 0.8), vec3(0.3, 0.15, 0.3), r.x)) r = U(r, stack(g - vec3(-1.0, 0.0, 0.8)));
  return r;
}
