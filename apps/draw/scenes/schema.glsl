#pragma frame {"size":[800,640],"center":[0.05,0.5,0.0],"zoom":185,"bounds":[[-2.0,-0.1,-1.0],[2.0,1.6,1.0]]}

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

float ringRect(vec2 p, vec2 b, float r, float w) {
  return abs(sdRect(p, b, r)) - w;
}

float entity(vec2 p, vec2 b, float rows) {
  float d = ringRect(p, b, 0.02, 0.008);
  d = min(d, sdRect(p - vec2(0.0, b.y - 0.045), vec2(b.x, 0.045), 0.02));
  for (int i = 0; i < 4; i++) {
    if (float(i) >= rows) break;
    float y = b.y - 0.14 - 0.085 * float(i);
    d = min(d, sdRect(p - vec2(-b.x + 0.06, y), vec2(0.022), 0.006));
    d = min(d, bar(p, -b.x + 0.11, y, b.x * 2.0 - 0.2 - 0.06 * mod(float(i), 2.0), 0.012));
  }
  return d;
}

vec2 blueprint(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.04, 0.0), vec3(0.2, 0.04, 0.52), 0.015), STEEL);
  vec3 b = p - vec3(0.0, 0.58, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.04, 0.5, 0.48), 0.03), BLUE);
  vec3 q = faceX(b, 0.04);
  vec3 f = vec3(-q.x, q.y, q.z);
  float d = ringRect(f.xz, vec2(0.42, 0.44), 0.02, 0.006);
  d = min(d, entity(f.xz - vec2(-0.15, 0.2), vec2(0.2, 0.17), 2.0));
  d = min(d, entity(f.xz - vec2(0.17, -0.22), vec2(0.17, 0.14), 1.0));
  d = min(d, sdSeg(f.xz, vec2(0.05, 0.12), vec2(0.17, 0.12)) - 0.008);
  d = min(d, sdSeg(f.xz, vec2(0.17, 0.12), vec2(0.17, -0.06)) - 0.008);
  d = min(d, sdCircle(f.xz - vec2(0.17, -0.06), 0.022));
  d = min(d, bar(f.xz, -0.33, -0.27, 0.16, 0.012));
  d = min(d, bar(f.xz, -0.33, -0.34, 0.1, 0.012));
  body = decal(body, f, d, WHITE);
  r = U(r, body);
  r = U(r, screw(faceY(p, 0.08) - vec3(0.12, 0.0, 0.0), 0.022));
  vec3 cl = vec3(p.x, p.y - 1.08, abs(p.z) - 0.3);
  r = U(r, M(sdRoundBox(cl, vec3(0.07, 0.035, 0.05), 0.012), GRAPHITE));
  return r;
}

vec2 machine(vec3 p) {
  vec3 c = p - vec3(0.0, 0.36, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.3, 0.36, 0.32), 0.03), WHITE);
  r = seam(r, c.y + 0.26);
  r = paint(r, c.y + 0.26, STEEL);
  vec3 fz = faceZ(c, 0.32);
  r = inset(r, fz - vec3(-0.06, 0.0, 0.14), vec2(0.17, 0.1), 0.02, 0.02, GRAPHITE);
  vec3 sc = fz - vec3(-0.06, -0.02, 0.14);
  r = decal(r, sc, bar(sc.xz, -0.12, 0.04, 0.2, 0.014), LIME);
  r = decal(r, sc, bar(sc.xz, -0.12, -0.01, 0.13, 0.014), LIME);
  r = decal(r, sc, bar(sc.xz, -0.12, -0.06, 0.17, 0.014), STEEL);
  r = U(r, led(fz - vec3(0.2, 0.0, 0.2), 0.03, LIME));
  r = U(r, led(fz - vec3(0.2, 0.0, 0.1), 0.03, STEEL));
  r = U(r, button(fz - vec3(-0.15, 0.015, -0.1), 0.045, 0.02, SUN));
  r = U(r, button(fz - vec3(-0.02, 0.015, -0.1), 0.045, 0.02, STEEL));
  r = U(r, button(fz - vec3(0.11, 0.015, -0.1), 0.045, 0.02, STEEL));
  vec3 fx = faceX(c, 0.3);
  r = vents(r, fx - vec3(0.0, 0.0, 0.12), 5.0, 0.07, 0.16, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.22, fx.y, fx.z + 0.18), 0.02));
  vec3 t = p - vec3(0.0, 0.72, 0.0);
  r = U(r, M(sdRoundCylinder(t - vec3(0.0, 0.03, 0.0), 0.16, 0.03, 0.01), STEEL + ROUND));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.25, 0.0), 0.075, 0.24), WHITE + ROUND));
  r = U(r, M(sdTorus(t - vec3(0.0, 0.2, 0.0), 0.075, 0.012), GRAPHITE + SOLID));
  vec3 a = p - vec3(0.45, 1.06, 0.0);
  vec2 arm = M(sdRoundBox(a, vec3(0.56, 0.055, 0.075), 0.02), WHITE);
  arm = vents(arm, faceZ(a, 0.075) - vec3(0.05, 0.0, 0.0), 8.0, 0.09, 0.05, 0.024, GRAPHITE);
  r = U(r, arm);
  r = U(r, M(sdRoundBox(p - vec3(-0.06, 1.06, 0.0), vec3(0.1, 0.09, 0.1), 0.02), STEEL));
  r = U(r, screw(faceZ(p - vec3(-0.06, 1.06, 0.0), 0.1), 0.02));
  vec3 tr = p - vec3(0.905, 0.96, 0.0);
  r = U(r, M(sdRoundBox(tr, vec3(0.1, 0.06, 0.11), 0.02), GRAPHITE));
  r = U(r, led(faceZ(tr, 0.11) - vec3(0.04, 0.0, 0.0), 0.022, LIME));
  vec3 w = vec3(p.x - 0.905, p.y - 0.83, abs(p.z) - 0.07);
  r = U(r, M(sdCylinder(w, 0.012, 0.1), GRAPHITE + SOLID));
  return r;
}

vec2 newColumn(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.025, 0.0), vec3(0.15, 0.025, 0.47), 0.012), LIME);
  r = seam(r, p.y - 0.035);
  float i = repI(p.z, 0.19, 5.0);
  vec3 t = vec3(p.x, p.y - 0.09, repC(p.z, 0.19, 5.0));
  vec2 tl = M(sdRoundBox(t, vec3(0.13, 0.04, 0.075), 0.015), LIME);
  vec3 tq = faceY(t, 0.04);
  tl = decal(tl, tq, bar(tq.xz, -0.08, 0.0, i == 0.0 ? 0.16 : 0.08 + 0.03 * mod(i, 3.0), 0.016), i == 0.0 ? GRAPHITE : WHITE);
  r = U(r, tl);
  vec3 cb = p - vec3(0.0, 0.2, 0.0);
  r = U(r, M(sdRoundBox(cb, vec3(0.035, 0.022, 0.53), 0.01), STEEL));
  vec3 j = vec3(p.x, p.y - 0.12, abs(p.z) - 0.515);
  r = U(r, M(sdRoundBox(j, vec3(0.035, 0.1, 0.014), 0.008), STEEL));
  r = U(r, M(sdRoundBox(vec3(j.x, j.y + 0.085, j.z + 0.03), vec3(0.035, 0.015, 0.03), 0.008), STEEL));
  r = U(r, screw(faceY(cb, 0.03) - vec3(0.0, 0.0, 0.0), 0.018));
  return r;
}

vec2 table(vec3 p) {
  vec3 c = p - vec3(0.0, 0.12, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.7, 0.12, 0.5), 0.03), WHITE);
  r = seam(r, c.y + 0.06);
  r = paint(r, c.y + 0.06, STEEL);
  r = vents(r, faceX(c, 0.7) - vec3(0.0, 0.0, 0.0), 5.0, 0.07, 0.08, 0.026, GRAPHITE);
  vec3 top = faceY(c, 0.12);
  vec3 slot = top - vec3(-0.495, 0.0, 0.0);
  r = inset(r, slot, vec2(0.15, 0.47), 0.02, 0.02, STEEL);
  vec3 sf = slot + vec3(0.0, 0.02, 0.0);
  if (sdRect(slot.xz, vec2(0.15, 0.47), 0.02) < 0.0) r = seam(r, sdRect(vec2(sf.x, repC(sf.z, 0.19, 5.0)), vec2(0.13, 0.075), 0.015));
  float tx = top.x - 0.165;
  float col = repI(tx, 0.33, 3.0);
  float row = repI(top.z, 0.19, 5.0);
  vec3 t = vec3(repC(tx, 0.33, 3.0), top.y - 0.04, repC(top.z, 0.19, 5.0));
  vec2 tl = M(sdRoundBox(t, vec3(0.14, 0.04, 0.075), 0.015), row == 0.0 ? GRAPHITE : WHITE);
  vec3 tq = faceY(t, 0.04);
  float k = col * 2.0 + row;
  tl = decal(tl, tq, bar(tq.xz, -0.09, 0.0, row == 0.0 ? 0.16 : 0.07 + 0.04 * mod(k, 3.0), 0.016), row == 0.0 ? WHITE : STEEL);
  r = U(r, tl);
  return r;
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(0.0, 0.06, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(1.78, 0.06, 0.66), 0.03), WHITE);
  r = seam(r, b.y + 0.01);
  vec3 fz = faceZ(b, 0.66);
  r = vents(r, fz - vec3(-0.7, 0.0, -0.025), 6.0, 0.07, 0.035, 0.026, GRAPHITE);
  vec3 cs = mirXZ(faceY(p, 0.12));
  r = U(r, screw(vec3(cs.x - 1.68, cs.y, cs.z - 0.56), 0.024));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  vec3 g = p - vec3(0.0, 0.12, 0.0);
  if (near(g - vec3(-1.3, 0.6, 0.0), vec3(0.25, 0.65, 0.55), r.x)) r = U(r, blueprint(g - vec3(-1.3, 0.0, 0.0)));
  if (near(g - vec3(0.0, 0.6, 0.0), vec3(1.0, 0.65, 0.4), r.x)) r = U(r, machine(g - vec3(-0.35, 0.0, 0.0)));
  if (near(g - vec3(1.05, 0.2, 0.0), vec3(0.75, 0.25, 0.55), r.x)) r = U(r, table(g - vec3(1.05, 0.0, 0.0)));
  if (near(g - vec3(0.555, 0.65, 0.0), vec3(0.2, 0.25, 0.6), r.x)) r = U(r, newColumn(g - vec3(0.555, 0.52, 0.0)));
  vec2 cable = M(pipe(g, vec3(-1.12, 0.03, 0.3), vec3(-0.85, 0.03, 0.3), 0.025), GRAPHITE + SOLID);
  cable = U(cable, M(pipe(g, vec3(-0.85, 0.03, 0.3), vec3(-0.85, 0.12, 0.3), 0.025), GRAPHITE + SOLID));
  cable = U(cable, M(pipe(g, vec3(-0.85, 0.12, 0.3), vec3(-0.64, 0.12, 0.3), 0.025), GRAPHITE + SOLID));
  r = U(r, cable);
  return r;
}
