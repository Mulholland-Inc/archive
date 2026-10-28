#pragma frame {"size":[1400,760],"center":[0.0,0.75,0.0],"zoom":190,"bounds":[[-2.6,-0.1,-1.4],[2.6,2.4,1.4]]}

const float B = 0.12;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}


vec2 vCard(vec3 q, float m) {
  vec2 c = plate(q, vec2(0.17, 0.062), 0.02, 0.014, m);
  vec3 t = q - vec3(0.0, 0.014, 0.0);
  float b = m == WHITE ? STEEL : WHITE;
  c = decal(c, t, bar(t.xz, -0.13, 0.02, 0.2, 0.012), b);
  c = decal(c, t, bar(t.xz, -0.13, -0.022, 0.12, 0.012), b);
  c = decal(c, t, sdCircle(t.xz - vec2(0.12, -0.02), 0.022), m == WHITE ? STEEL : WHITE);
  return c;
}

vec2 vBoard(vec3 q) {
  vec2 r = M(sdRoundBox(q, vec3(0.72, 0.46, 0.025), 0.025), WHITE);
  vec3 f = faceZ(q, 0.025);
  float lane = repI(f.x, 0.47, 3.0);
  vec3 l = vec3(repC(f.x, 0.47, 3.0), f.y, f.z);
  r = inset(r, l - vec3(0.0, 0.0, -0.05), vec2(0.205, 0.36), 0.03, 0.012, PAPER);
  r = decal(r, l, sdCircle(l.xz - vec2(-0.16, 0.39), 0.022), lane == 0.0 ? SUN : lane == 1.0 ? LILAC : LIME);
  r = decal(r, l, bar(l.xz, -0.12, 0.39, 0.16 + 0.05 * lane, 0.014), STEEL);
  float n = 3.0;
  float s = repLI(0.21 - l.z, 0.155, n);
  vec3 cq = vec3(l.x, l.y + 0.004, -repL(0.21 - l.z, 0.155, n));
  if (lane == 1.0 && s == 1.0) r = seam(r, -max(sdRect(cq.xz, vec2(0.17, 0.062), 0.02), abs(f.y + 0.012) - 0.01));
  else r = U(r, vCard(cq, WHITE));
  r = U(r, vCard(rotY(f - vec3(0.04, 0.07, 0.075), 0.16), LILAC));
  return r;
}

vec2 vCalendar(vec3 q) {
  vec2 r = M(sdRoundBox(q, vec3(0.72, 0.46, 0.025), 0.025), WHITE);
  vec3 f = faceZ(q, 0.025);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.37), vec2(0.66, 0.05), 0.03), SUN);
  r = decal(r, f, sdCircle(vec2(abs(f.x) - 0.5, f.z - 0.37), 0.02), WHITE);
  vec2 g = vec2(repC(f.x, 0.18, 7.0), repC(f.z + 0.08, 0.165, 4.0));
  float gi = repI(f.x, 0.18, 7.0) + 7.0 * repI(f.z + 0.08, 0.165, 4.0);
  float m = gi == 26.0 ? CORAL : gi == 19.0 ? LIME : STEEL;
  r = decal(r, f, sdRect(g, vec2(0.065, 0.06), 0.015), m);
  return r;
}

vec2 vSheetGrid(vec3 q) {
  vec2 r = M(sdRoundBox(q, vec3(0.72, 0.46, 0.025), 0.025), WHITE);
  vec3 f = faceZ(q, 0.025);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.36), vec2(0.66, 0.055), 0.025), LIME);
  float d = sdRect(f.xz - vec2(0.0, -0.06), vec2(0.66, 0.36), 0.0);
  float lines = min(abs(repC(f.z + 0.06, 0.12, 7.0) - 0.0) - 0.004, 1e9);
  float vl = abs(repC(f.x, 0.33, 5.0)) - 0.004;
  r = decal(r, f, max(d, min(lines, vl)), STEEL);
  r = decal(r, f, abs(d) - 0.005, STEEL);
  return r;
}

vec2 vTier(vec3 p, float h, float z, float hz, float sz) {
  vec3 b = p - vec3(0.0, h * 0.5, z);
  vec2 r = M(sdRoundBox(b, vec3(0.95, h * 0.5, hz), 0.03), WHITE);
  r = Sc(r, M(sdBox(p - vec3(0.0, h, sz), vec3(0.76, 0.04, 0.034)), GRAPHITE));
  return r;
}

vec2 vStand(vec3 p) {
  vec2 r = vTier(p, 0.16, 0.0, 0.36, 0.15);
  r = U(r, vTier(p, 0.34, -0.17, 0.19, -0.08));
  r = U(r, vTier(p, 0.54, -0.27, 0.09, -0.27));
  r = seam(r, p.y - 0.05);
  r = paint(r, p.y - 0.05, STEEL);
  vec3 fz = faceZ(p, 0.36);
  r = vents(r, fz - vec3(0.55, 0.0, 0.1), 5.0, 0.06, 0.06, 0.024, GRAPHITE);
  r = U(r, led(fz - vec3(-0.8, 0.0, 0.1), 0.022, LIME));
  r = U(r, led(fz - vec3(-0.72, 0.0, 0.1), 0.022, LIME));
  vec3 fx = faceX(p, 0.95);
  r = U(r, screw(vec3(fx.x + 0.27, fx.y, fx.z - 0.44), 0.02));
  r = U(r, screw(vec3(fx.x + 0.1, fx.y, fx.z - 0.25), 0.02));
  r = U(r, screw(vec3(fx.x - 0.2, fx.y, fx.z - 0.08), 0.02));
  vec3 t = faceY(p, 0.16);
  r = U(r, screw(vec3(abs(t.x) - 0.86, t.y, t.z - 0.27), 0.02));
  return r;
}

vec2 vMap(vec3 p) {
  vec2 r = vStand(p);
  if (near(p - vec3(-0.3, 0.97, -0.27), vec3(0.85, 0.55, 0.05), r.x)) r = U(r, vSheetGrid(rotZ(p - vec3(-0.3, 0.97, -0.27), 0.0)));
  if (near(p - vec3(0.3, 0.77, -0.08), vec3(0.85, 0.55, 0.05), r.x)) r = U(r, vCalendar(rotZ(p - vec3(0.3, 0.77, -0.08), 0.0)));
  if (near(p - vec3(0.0, 0.59, 0.15), vec3(0.8, 0.5, 0.12), r.x)) r = U(r, vBoard(p - vec3(0.0, 0.59, 0.15)));
  vec3 lt = faceY(p, 0.16);
  r = U(r, vCard(rotY(lt - vec3(-0.55, 0.0, 0.27), -0.1), WHITE));
  r = U(r, vCard(rotY(lt - vec3(0.64, 0.0, 0.265), 0.1), WHITE));
  return r;
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(0.25, B * 0.5, -0.05);
  vec2 r = M(sdRoundBox(b, vec3(2.05, B * 0.5, 0.9), 0.03), WHITE);
  r = seam(r, p.y - 0.04);
  r = paint(r, p.y - 0.04, STEEL);
  vec3 fz = faceZ(b, 0.9);
  r = vents(r, fz - vec3(1.3, 0.0, -0.015), 8.0, 0.07, 0.035, 0.026, GRAPHITE);
  vec3 cs = mirXZ(faceY(b, B * 0.5));
  r = U(r, screw(vec3(cs.x - 1.93, cs.y, cs.z - 0.78), 0.024));
  return r;
}

vec2 block(vec3 q, float m) {
  vec2 r = M(sdRoundBox(q, vec3(0.19, 0.14, 0.17), 0.025), m);
  r = seam(r, q.y - 0.08);
  vec3 t = faceY(q, 0.14);
  float b = m == WHITE ? STEEL : WHITE;
  r = decal(r, t, bar(t.xz, -0.12, -0.07, 0.2, 0.016), b);
  r = decal(r, t, bar(t.xz, -0.12, 0.0, 0.13, 0.016), b);
  r = decal(r, t, bar(t.xz, -0.12, 0.07, 0.17, 0.016), b);
  vec3 f = faceZ(q, 0.17);
  r = U(r, M(sdRoundBox(f - vec3(0.0, 0.012, -0.02), vec3(0.08, 0.012, 0.02), 0.01), m == LIME ? WHITE : GRAPHITE));
  return r;
}

vec2 grid(vec3 p) {
  vec3 c = p - vec3(0.0, 0.2, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(1.02, 0.2, 0.72), 0.03), WHITE);
  float ix = repI(c.x, 0.48, 4.0), iz = repI(c.z, 0.44, 3.0);
  vec3 cell = vec3(repC(c.x, 0.48, 4.0), c.y, repC(c.z, 0.44, 3.0));
  r = Sc(r, M(sdBox(cell - vec3(0.0, 0.2, 0.0), vec3(0.215, 0.3, 0.195)), STEEL));
  r = seam(r, c.y + 0.12);
  r = paint(r, c.y + 0.12, STEEL);
  vec3 fz = faceZ(c, 0.72);
  r = U(r, led(fz - vec3(0.8, 0.0, 0.0), 0.028, LIME));
  r = U(r, led(fz - vec3(0.9, 0.0, 0.0), 0.028, STEEL));
  r = vents(r, fz - vec3(-0.6, 0.0, 0.0), 6.0, 0.07, 0.12, 0.026, GRAPHITE);
  vec3 fx = faceX(c, 1.02);
  r = vents(r, fx - vec3(0.0, 0.0, 0.0), 5.0, 0.07, 0.12, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.6, fx.y, fx.z), 0.02));
  float k = ix + iz * 4.0;
  if (!(ix == 3.0 && iz == 0.0)) {
    float m = mod(k, 3.0) == 1.0 ? STEEL : WHITE;
    r = U(r, block(cell - vec3(0.0, 0.17, 0.0), m));
  } else {
    r = U(r, block(c - vec3(0.24, 0.17, -0.44), WHITE));
    r = U(r, block(c - vec3(0.72, 0.17, 0.0), STEEL));
  }
  return r;
}

vec2 crane(vec3 p) {
  vec3 o = p - vec3(0.12, 0.0, -0.98);
  vec2 r = M(sdRoundCylinder(o - vec3(0.0, 0.04, 0.0), 0.2, 0.04, 0.012), STEEL + ROUND);
  r = U(r, M(sdCylinder(o - vec3(0.0, 0.95, 0.0), 0.06, 0.9), WHITE + ROUND));
  r = U(r, M(sdCylinder(o - vec3(0.0, 0.25, 0.0), 0.085, 0.14), GRAPHITE + ROUND));
  vec3 j = o - vec3(0.0, 1.85, 0.32);
  vec2 jib = M(sdRoundBox(j, vec3(0.06, 0.06, 0.5), 0.015), SUN);
  jib = stripes(jib, j.z, 0.08);
  r = U(r, jib);
  r = U(r, M(sdRoundBox(o - vec3(0.0, 1.85, -0.28), vec3(0.11, 0.1, 0.12), 0.02), GRAPHITE));
  r = U(r, M(pipe(o, vec3(0.0, 1.8, 0.0), vec3(0.0, 1.45, 0.0), 0.02), GRAPHITE + SOLID));
  r = U(r, M(pipe(o, vec3(0.0, 1.45, 0.0), vec3(0.0, 1.79, 0.5), 0.015), GRAPHITE + SOLID));
  vec3 t = o - vec3(0.0, 1.74, 0.54);
  r = U(r, M(sdRoundBox(t, vec3(0.08, 0.045, 0.08), 0.015), WHITE));
  r = U(r, M(sdCylinder(t - vec3(0.0, -0.24, 0.0), 0.008, 0.22), GRAPHITE + SOLID));
  vec3 h = t - vec3(0.0, -0.5, 0.0);
  r = U(r, M(sdTorus(rotX(h, PI * 0.5), 0.04, 0.012), GRAPHITE + SOLID));
  vec3 s = o - vec3(0.0, 1.0, 0.54);
  r = U(r, M(pipe(s, vec3(0.0, 0.2, 0.0), vec3(0.15, 0.03, 0.13), 0.008), GRAPHITE + SOLID));
  r = U(r, M(pipe(s, vec3(0.0, 0.2, 0.0), vec3(-0.15, 0.03, -0.13), 0.008), GRAPHITE + SOLID));
  r = U(r, M(pipe(s, vec3(0.0, 0.2, 0.0), vec3(0.15, 0.03, -0.13), 0.008), GRAPHITE + SOLID));
  r = U(r, M(pipe(s, vec3(0.0, 0.2, 0.0), vec3(-0.15, 0.03, 0.13), 0.008), GRAPHITE + SOLID));
  r = U(r, block(s - vec3(0.0, -0.13, 0.0), LIME));
  return r;
}

vec2 viewsUnit(vec3 p) {
  float k = 0.72;
  vec2 r = vMap(p / k);
  return vec2(r.x * k, r.y);
}

vec2 cable(vec3 p) {
  float y = 0.025;
  float d = pipe(p, vec3(0.45, 0.2, 0.3), vec3(0.6, 0.2, 0.3), 0.025);
  d = min(d, pipe(p, vec3(0.6, 0.2, 0.3), vec3(0.6, y, 0.3), 0.025));
  d = min(d, pipe(p, vec3(0.6, y, 0.3), vec3(0.9, y, 0.3), 0.025));
  vec2 r = M(d, GRAPHITE + SOLID);
  r = U(r, M(sdRoundBox(p - vec3(0.44, 0.2, 0.3), vec3(0.03, 0.05, 0.05), 0.012), STEEL));
  return r;
}

vec2 toolbox(vec3 p) {
  vec3 b = p - vec3(0.0, 0.14, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.3, 0.14, 0.16), 0.02), CORAL);
  r = seam(r, b.y - 0.07);
  vec3 fz = faceZ(b, 0.16);
  r = U(r, M(sdRoundBox(fz - vec3(0.0, 0.012, 0.07), vec3(0.05, 0.012, 0.03), 0.008), STEEL));
  r = U(r, screw(vec3(abs(fz.x) - 0.22, fz.y, fz.z + 0.05), 0.016));
  vec3 h = p - vec3(0.0, 0.3, 0.0);
  float hd = max(sdTorus(rotX(h, PI * 0.5), 0.13, 0.022), -h.y);
  r = U(r, M(hd, GRAPHITE + SOLID));
  vec3 t = p - vec3(0.0, 0.0, 0.0);
  r = U(r, M(pipe(p, vec3(0.42, 0.02, 0.05), vec3(0.62, 0.02, 0.18), 0.02), GRAPHITE + SOLID));
  r = U(r, M(sdRoundBox(rotY(p - vec3(0.67, 0.025, 0.21), -0.57), vec3(0.06, 0.02, 0.035), 0.01), STEEL));
  r = U(r, M(pipe(p, vec3(0.42, 0.02, 0.05), vec3(0.42, 0.02, 0.05), 0.035), STEEL + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  vec3 g = p - vec3(0.0, B, 0.0);
  if (near(g - vec3(-0.6, 0.4, 0.0), vec3(1.05, 0.45, 0.75), r.x)) r = U(r, grid(g - vec3(-0.6, 0.0, 0.0)));
  if (near(g - vec3(0.12, 1.0, -0.6), vec3(0.4, 1.05, 0.55), r.x)) r = U(r, crane(g));
  if (near(g - vec3(1.6, 0.7, -0.3), vec3(0.75, 0.75, 0.45), r.x)) r = U(r, viewsUnit(g - vec3(1.6, 0.0, -0.3)));
  if (near(g - vec3(1.3, 0.25, 0.45), vec3(0.6, 0.3, 0.35), r.x)) r = U(r, toolbox(rotY(g - vec3(1.15, 0.0, 0.45), 0.15)));
  if (near(g - vec3(0.65, 0.15, -0.2), vec3(0.3, 0.2, 0.1), r.x)) r = U(r, cable(g - vec3(0.0, 0.0, -0.5)));
  return r;
}
