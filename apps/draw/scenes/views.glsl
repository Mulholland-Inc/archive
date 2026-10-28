#pragma frame {"size":[800,640],"center":[0.0,0.78,0.0],"zoom":228,"bounds":[[-1.1,-0.1,-0.5],[1.1,1.8,0.5]]}

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 card(vec3 q, float m) {
  vec2 c = plate(q, vec2(0.17, 0.062), 0.02, 0.014, m);
  vec3 t = q - vec3(0.0, 0.014, 0.0);
  float b = m == WHITE ? STEEL : WHITE;
  c = decal(c, t, bar(t.xz, -0.13, 0.02, 0.2, 0.012), b);
  c = decal(c, t, bar(t.xz, -0.13, -0.022, 0.12, 0.012), b);
  c = decal(c, t, sdCircle(t.xz - vec2(0.12, -0.02), 0.022), m == WHITE ? STEEL : WHITE);
  return c;
}

vec2 board(vec3 q) {
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
  else r = U(r, card(cq, WHITE));
  r = U(r, card(rotY(f - vec3(0.04, 0.07, 0.075), 0.16), LILAC));
  return r;
}

vec2 calendar(vec3 q) {
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

vec2 sheetGrid(vec3 q) {
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

vec2 tier(vec3 p, float h, float z, float hz, float sz) {
  vec3 b = p - vec3(0.0, h * 0.5, z);
  vec2 r = M(sdRoundBox(b, vec3(0.95, h * 0.5, hz), 0.03), WHITE);
  r = Sc(r, M(sdBox(p - vec3(0.0, h, sz), vec3(0.76, 0.04, 0.034)), GRAPHITE));
  return r;
}

vec2 stand(vec3 p) {
  vec2 r = tier(p, 0.16, 0.0, 0.36, 0.15);
  r = U(r, tier(p, 0.34, -0.17, 0.19, -0.08));
  r = U(r, tier(p, 0.54, -0.27, 0.09, -0.27));
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

vec2 map(vec3 p) {
  vec2 r = stand(p);
  if (near(p - vec3(-0.3, 0.97, -0.27), vec3(0.85, 0.55, 0.05), r.x)) r = U(r, sheetGrid(rotZ(p - vec3(-0.3, 0.97, -0.27), 0.0)));
  if (near(p - vec3(0.3, 0.77, -0.08), vec3(0.85, 0.55, 0.05), r.x)) r = U(r, calendar(rotZ(p - vec3(0.3, 0.77, -0.08), 0.0)));
  if (near(p - vec3(0.0, 0.59, 0.15), vec3(0.8, 0.5, 0.12), r.x)) r = U(r, board(p - vec3(0.0, 0.59, 0.15)));
  vec3 lt = faceY(p, 0.16);
  r = U(r, card(rotY(lt - vec3(-0.55, 0.0, 0.27), -0.1), WHITE));
  r = U(r, card(rotY(lt - vec3(0.64, 0.0, 0.265), 0.1), WHITE));
  return r;
}
