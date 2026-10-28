#pragma frame {"size":[1200,720],"center":[0.0,0.4,0.0],"zoom":260,"bounds":[[-2.6,-0.1,-2.6],[2.6,2.0,2.6]]}

vec2 strip(vec3 p, float len) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  float wave = 0.025 * sin(q.x * 9.0);
  vec3 w = vec3(q.x, q.y - wave, q.z);
  vec2 r = M(sdBox(w, vec3(len * 0.5, 0.005, 0.2)) * 0.8, WHITE);
  vec3 f = faceY(w, 0.005);
  float lx = repC(f.x, 0.07, floor(len / 0.07));
  float li = repI(f.x, 0.07, floor(len / 0.07));
  float ln = 0.1 + 0.06 * sin(li * 2.7);
  r = decal(r, f, sdRect(vec2(lx, f.z + 0.14 - ln), vec2(0.012, ln), 0.012), STEEL);
  r = decal(r, f, sdRect(vec2(lx, f.z - 0.07), vec2(0.012, 0.06 + 0.04 * cos(li * 1.9)), 0.012), STEEL);
  return r;
}

vec2 roll(vec3 p) {
  vec3 a = vec3(p.x, p.z, p.y);
  vec2 r = M(sdCylinder(a, 0.2, 0.21), WHITE + ROUND);
  r = U(r, M(sdCylinder(a, 0.05, 0.25), STEEL + ROUND));
  r = U(r, M(sdCylinder(a, 0.02, 0.27), GRAPHITE + ROUND));
  vec3 st = vec3(p.x, p.y + 0.12, abs(p.z) - 0.29);
  r = U(r, M(sdRoundBox(st, vec3(0.04, 0.14, 0.02), 0.01), STEEL));
  r = U(r, M(sdRoundBox(vec3(p.x, p.y + 0.24, abs(p.z) - 0.29), vec3(0.12, 0.02, 0.05), 0.01), STEEL));
  return r;
}

vec2 filer(vec3 p) {
  vec3 b = p - vec3(0.0, 0.33, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.38, 0.33, 0.34), 0.03), WHITE);
  r = seam(r, b.y + 0.2);
  r = seam(r, b.y - 0.22);
  vec3 fz = faceZ(p, 0.34);
  r = vents(r, fz - vec3(-0.12, 0.0, 0.4), 5.0, 0.055, 0.12, 0.022, GRAPHITE);
  r = U(r, led(fz - vec3(0.2, 0.0, 0.44), 0.026, LIME));
  r = U(r, led(fz - vec3(0.28, 0.0, 0.44), 0.026, STEEL));
  r = U(r, screw(vec3(abs(fz.x) - 0.3, fz.y, fz.z - 0.06), 0.016));
  vec3 fx = faceX(p, 0.38);
  r = Sc(r, M(max(sdRect(fx.xz - vec2(0.0, 0.2), vec2(0.26, 0.03), 0.02), -(fx.y + 0.3)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(fx.xz - vec2(0.0, 0.2), vec2(0.3, 0.07), 0.04), -sdRect(fx.xz - vec2(0.0, 0.2), vec2(0.26, 0.03), 0.02)), fx.y - 0.012, 0.012), STEEL));
  r = U(r, screw(vec3(abs(fx.x) - 0.26, fx.y, fx.z - 0.5), 0.016));
  vec3 t = faceY(p, 0.66);
  r = U(r, button(t - vec3(0.12, 0.03, 0.12), 0.06, 0.03, LIME));
  r = U(r, button(t - vec3(-0.06, 0.03, 0.15), 0.045, 0.025, STEEL));
  r = U(r, M(sdRoundBox(t - vec3(-0.1, 0.03, -0.12), vec3(0.16, 0.03, 0.08), 0.015), STEEL));
  r = vents(r, t - vec3(-0.1, 0.06, -0.12), 5.0, 0.05, 0.08, 0.018, GRAPHITE);
  vec3 ip = faceNX(p, 0.38);
  r = Sc(r, M(max(sdRect(ip.xz - vec2(0.0, 0.08), vec2(0.23, 0.02), 0.01), -(ip.y + 0.3)), GRAPHITE));
  return r;
}

vec2 record(vec3 p, float full) {
  vec2 r = M(sdRoundBox(p, vec3(0.26, 0.012, 0.2), 0.01), WHITE);
  vec3 q = faceY(p, 0.012);
  for (int i = 0; i < 4; i++) {
    float fi = float(i);
    float z = -0.13 + fi * 0.087;
    r = decal(r, q, sdRect(q.xz - vec2(-0.17, z), vec2(0.05, 0.012), 0.012), GRAPHITE);
    r = decal(r, q, abs(sdRect(q.xz - vec2(0.07, z), vec2(0.15, 0.03), 0.01)) - 0.004, STEEL);
    float w = 0.06 + 0.05 * fract(fi * 0.37 + 0.2);
    if (full > 0.5) r = decal(r, q, sdRect(q.xz - vec2(-0.05 + w, z), vec2(w, 0.014), 0.014), i == 0 ? LIME : LILAC);
  }
  return r;
}

vec2 tray(vec3 p) {
  vec3 q = p - vec3(0.0, 0.05, 0.0);
  vec2 r = M(max(sdRoundBox(q, vec3(0.32, 0.05, 0.26), 0.012), -sdBox(q - vec3(0.0, 0.03, 0.0), vec3(0.3, 0.05, 0.24))), STEEL);
  for (int i = 0; i < 4; i++) {
    float fi = float(i);
    r = U(r, record(rotY(p - vec3(0.0, 0.04 + fi * 0.026, 0.0), 0.05 * sin(fi * 2.2)), 1.0));
  }
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 rp = p - vec3(-1.85, 0.25, 0.0);
  if (near(rp, vec3(0.3, 0.42, 0.35), r.x)) r = U(r, roll(rp));
  if (near(p - vec3(-1.05, 0.05, 0.0), vec3(0.75, 0.08, 0.25), r.x)) r = U(r, strip(p - vec3(-1.78, 0.03, 0.0), 1.42));
  vec3 fp = p - vec3(0.0, 0.0, 0.0);
  if (near(fp - vec3(0.0, 0.4, 0.0), vec3(0.5, 0.45, 0.45), r.x)) r = U(r, filer(fp));
  if (near(p - vec3(0.6, 0.2, 0.0), vec3(0.25, 0.05, 0.25), r.x)) r = U(r, record(rotY(p - vec3(0.6, 0.2, 0.0), 0.0), 1.0));
  vec3 tp = p - vec3(0.7, 0.0, 0.95);
  if (near(tp - vec3(0.0, 0.1, 0.0), vec3(0.4, 0.15, 0.32), r.x)) r = U(r, tray(rotY(tp, 0.15)));
  return r;
}
