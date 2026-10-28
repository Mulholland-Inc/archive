#pragma frame {"size":[800,640],"center":[0.0,0.55,0.15],"zoom":250,"bounds":[[-1.4,-0.1,-1.0],[1.4,1.6,1.4]]}

float sdTri(vec2 p, float r) {
  const float k = 1.7320508;
  p.x = abs(p.x) - r;
  p.y = p.y + r / k;
  if (p.x + k * p.y > 0.0) p = vec2(p.x - k * p.y, -k * p.x - p.y) / 2.0;
  p.x -= clamp(p.x, -2.0 * r, 0.0);
  return -length(p) * sign(p.y);
}

vec2 bar(vec3 q, vec2 a, float len, float m) {
  return M(extrude(sdRect(q.xz - vec2(a.x + len, a.y), vec2(len, 0.017), 0.017), q.y - 0.008, 0.008), m);
}

vec2 editor(vec3 p) {
  vec3 b = p - vec3(0.0, 0.82, -0.2);
  vec2 w = M(sdRoundBox(b, vec3(0.72, 0.5, 0.07), 0.035), WHITE);
  vec3 f = faceZ(p, -0.13) - vec3(0.0, 0.0, 0.82);
  w = paint(w, -(f.z - 0.36), STEEL);
  w = inset(w, f - vec3(0.03, 0.0, -0.08), vec2(0.62, 0.38), 0.025, 0.018, PAPER);
  vec2 r = w;
  vec3 d = vec3(repC(f.x + 0.53, 0.075, 3.0), f.y, f.z - 0.43);
  float di = repI(f.x + 0.53, 0.075, 3.0);
  r = U(r, M(extrude(length(d.xz) - 0.026, d.y - 0.008, 0.008), (di == 0.0 ? CORAL : di == 1.0 ? SUN : LIME) + SOLID));
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.12, 0.43), vec2(0.3, 0.022), 0.022), f.y - 0.006, 0.006), WHITE));
  r = U(r, screw(vec3(f.x - 0.64, f.y, f.z - 0.43), 0.016));
  vec3 c = f - vec3(0.0, -0.018, 0.0);
  float row = floor((0.24 - c.z) / 0.085 + 0.5);
  float rz = c.z - (0.24 - row * 0.085);
  vec3 lq = vec3(c.x, c.y, rz);
  if (row >= 0.0 && row <= 6.0) {
    float ind = row == 1.0 || row == 2.0 || row == 4.0 ? 0.08 : row == 3.0 ? 0.16 : 0.0;
    float len = row == 0.0 ? 0.24 : row == 1.0 ? 0.32 : row == 2.0 ? 0.14 : row == 3.0 ? 0.26 : row == 4.0 ? 0.2 : row == 5.0 ? 0.1 : 0.3;
    float m = row == 3.0 ? LILAC : row == 0.0 || row == 5.0 ? GRAPHITE : STEEL;
    r = U(r, bar(lq, vec2(-0.48 + ind, 0.0), len, m));
    if (row != 5.0 && row != 2.0) r = U(r, bar(lq, vec2(-0.48 + ind + 2.0 * len + 0.06, 0.0), row == 1.0 ? 0.06 : 0.1, STEEL));
    r = U(r, M(extrude(length(lq.xz - vec2(-0.535, 0.0)) - 0.014, lq.y - 0.006, 0.006), STEEL + SOLID));
  }
  r = U(r, M(extrude(sdRect(c.xz - vec2(0.6, -0.08), vec2(0.016, 0.34), 0.016), c.y - 0.004, 0.004), STEEL));
  r = U(r, M(extrude(sdRect(c.xz - vec2(0.6, 0.12), vec2(0.016, 0.1), 0.016), c.y - 0.012, 0.008), WHITE));
  vec3 s = p - vec3(0.0, 0.0, -0.2);
  vec3 lg = vec3(abs(s.x) - 0.45, s.y - 0.17, s.z);
  r = U(r, M(sdBox(lg, vec3(0.04, 0.17, 0.04)), STEEL));
  r = U(r, M(sdRoundBox(vec3(abs(s.x) - 0.45, s.y - 0.025, s.z), vec3(0.11, 0.025, 0.16), 0.012), WHITE));
  return r;
}

vec2 runner(vec3 p) {
  vec3 b = p - vec3(0.0, 0.06, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.3, 0.06, 0.3), 0.02), WHITE);
  r = seam(r, b.y + 0.02);
  vec3 cs = mirXZ(faceY(p, 0.12)) - vec3(0.25, 0.0, 0.25);
  r = U(r, screw(cs, 0.018));
  vec3 k = p - vec3(0.0, 0.2, 0.0);
  vec2 key = M(sdRoundBox(k, vec3(0.21, 0.09, 0.21), 0.045), LIME);
  key = seam(key, k.y + 0.04);
  r = U(r, key);
  vec3 t = faceY(p, 0.29);
  r = U(r, M(extrude(sdTri(rot(PI * 0.25) * t.xz * vec2(1.0, -1.0), 0.085) - 0.012, t.y - 0.015, 0.015), WHITE + SOLID));
  vec3 fz = faceZ(p, 0.3);
  r = U(r, led(fz - vec3(0.18, 0.0, 0.06), 0.02, LIME));
  r = vents(r, fz - vec3(-0.08, 0.0, 0.06), 5.0, 0.04, 0.03, 0.014, GRAPHITE);
  return r;
}

vec2 card(vec3 p, float tag) {
  vec2 c = M(sdRoundBox(p, vec3(0.17, 0.014, 0.12), 0.008), WHITE);
  vec3 q = faceY(p, 0.014);
  c = decal(c, q, sdRect(q.xz - vec2(-0.09, -0.06), vec2(0.05, 0.016), 0.016), tag);
  c = decal(c, q, sdRect(q.xz - vec2(0.0, 0.0), vec2(0.14, 0.011), 0.011), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(-0.04, 0.05), vec2(0.1, 0.011), 0.011), STEEL);
  return c;
}

vec2 tray(vec3 p) {
  vec3 b = p - vec3(0.0, 0.07, 0.0);
  float outer = sdRoundBox(b, vec3(0.24, 0.07, 0.19), 0.02);
  float inner = sdRoundBox(b - vec3(0.0, 0.07, 0.0), vec3(0.205, 0.1, 0.155), 0.012);
  vec2 r = M(max(outer, -inner), STEEL);
  r = seam(r, b.y + 0.035);
  vec3 fz = faceZ(p, 0.19);
  r = vents(r, fz - vec3(0.0, 0.0, 0.07), 6.0, 0.045, 0.02, 0.012, GRAPHITE);
  r = U(r, card(p - vec3(0.0, 0.05, 0.0), LILAC));
  r = U(r, card(rotZ(rotY(p - vec3(-0.02, 0.085, 0.01), 0.12), 0.0), STEEL));
  r = U(r, card(rotX(rotZ(rotY(p - vec3(0.04, 0.34, -0.06), -0.25), -0.35), 0.18), LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.7, -0.2), vec3(0.8, 0.7, 0.2), r.x)) r = U(r, editor(p));
  vec3 rq = p - vec3(0.38, 0.0, 0.5);
  if (near(rq - vec3(0.0, 0.18, 0.0), vec3(0.35, 0.2, 0.35), r.x)) r = U(r, runner(rq));
  vec3 tq = p - vec3(-0.5, 0.0, 0.55);
  if (near(tq - vec3(0.0, 0.25, 0.0), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, tray(tq));
  return r;
}
