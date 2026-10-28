#pragma frame {"size":[800,640],"center":[0.05,0.45,0.2],"zoom":312,"bounds":[[-1.0,-0.1,-1.0],[1.0,1.2,1.2]]}

vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  float s = 1.0 - t;
  return s * s * s * a + 3.0 * s * s * t * b + 3.0 * s * t * t * c + t * t * t * d;
}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float m = 1e9;
  vec3 q = a;
  for (int i = 1; i <= 14; i++) {
    vec3 n = bez(a, b, c, d, float(i) / 14.0);
    m = min(m, sdCapsule(p, q, n, r));
    q = n;
  }
  return m;
}

vec2 monitor(vec3 p) {
  vec3 b = p - vec3(0.0, 0.66, -0.12);
  vec2 r = M(sdRoundBox(b, vec3(0.42, 0.34, 0.12), 0.05), WHITE);
  r = seam(r, b.z + 0.06);
  vec3 bk = b - vec3(0.0, 0.02, -0.12);
  float k = mix(1.0, 0.62, clamp(-bk.z / 0.36, 0.0, 1.0));
  float back = sdRoundBox(vec3(bk.x / k, bk.y / k, bk.z + 0.18), vec3(0.36, 0.28, 0.18), 0.05) * k * 0.8;
  vec2 bkv = M(back, WHITE);
  bkv = seam(bkv, bk.z + 0.12);
  r = U(r, bkv);
  vec3 f = faceZ(p, 0.0) - vec3(0.0, 0.0, 0.7);
  r = inset(r, f, vec2(0.33, 0.23), 0.06, 0.035, GRAPHITE);
  vec3 s = vec3(f.x, f.y + 0.035, f.z);
  r = U(r, M(extrude(sdRect(s.xz - vec2(-0.1, 0.17), vec2(0.15, 0.018), 0.01), s.y - 0.004, 0.004), STEEL));
  r = U(r, M(extrude(sdRect(s.xz - vec2(-0.16, 0.105), vec2(0.09, 0.018), 0.01), s.y - 0.004, 0.004), STEEL));
  r = U(r, M(extrude(sdRect(s.xz - vec2(-0.12, 0.04), vec2(0.13, 0.018), 0.01), s.y - 0.004, 0.004), STEEL));
  r = U(r, M(extrude(sdRect(s.xz - vec2(-0.215, -0.07), vec2(0.035, 0.018), 0.01), s.y - 0.006, 0.006), WHITE));
  r = U(r, M(extrude(sdRect(s.xz - vec2(-0.135, -0.07), vec2(0.026, 0.035), 0.006), s.y - 0.012, 0.012), LIME));
  vec3 g = f - vec3(0.0, 0.0, -0.29);
  r = U(r, button(vec3(g.x - 0.3, g.y - 0.012, g.z), 0.03, 0.012, STEEL));
  r = U(r, button(vec3(g.x - 0.21, g.y - 0.012, g.z), 0.03, 0.012, STEEL));
  r = U(r, led(g - vec3(-0.3, 0.0, 0.0), 0.016, LIME));
  r = vents(r, g - vec3(-0.08, 0.0, 0.0), 5.0, 0.04, 0.02, 0.012, GRAPHITE);
  vec3 fx = faceX(p, 0.42);
  r = vents(r, vec3(fx.x + 0.12, fx.y, fx.z - 0.66), 4.0, 0.05, 0.3, 0.02, GRAPHITE);
  vec3 sc = vec3(abs(f.x) - 0.37, f.y, abs(f.z) - 0.29);
  r = U(r, screw(sc, 0.016));
  vec3 n = p - vec3(0.0, 0.0, -0.2);
  r = U(r, M(sdCylinder(n - vec3(0.0, 0.2, 0.0), 0.07, 0.16), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(n - vec3(0.0, 0.33, 0.0), 0.11, 0.025, 0.01), GRAPHITE + ROUND));
  vec2 base = M(sdRoundBox(n - vec3(0.0, 0.035, 0.0), vec3(0.3, 0.035, 0.24), 0.02), WHITE);
  base = seam(base, n.y - 0.035);
  r = U(r, base);
  vec3 bt = mirXZ(faceY(n, 0.07)) - vec3(0.25, 0.0, 0.19);
  r = U(r, screw(bt, 0.016));
  return r;
}

vec2 keyboard(vec3 p) {
  vec3 b = p - vec3(0.0, 0.05, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.045, 0.2), 0.02), WHITE);
  r = seam(r, b.y + 0.01);
  vec3 t = faceY(b, 0.045);
  r = inset(r, t - vec3(0.0, 0.0, 0.01), vec2(0.46, 0.165), 0.02, 0.015, STEEL);
  vec2 kq = rep2(t.xz - vec2(-0.03, -0.075), vec2(0.084, 0.075), vec2(10.0, 3.0));
  vec2 ki = vec2(repI(t.x + 0.03, 0.084, 10.0), repI(t.z + 0.075, 0.075, 3.0));
  float km = ki == vec2(9.0, 2.0) ? LIME : ki == vec2(0.0, 0.0) ? LILAC : WHITE;
  r = U(r, M(sdRoundBox(vec3(kq.x, t.y, kq.y), vec3(0.032, 0.022, 0.028), 0.01), km));
  vec3 sp = vec3(t.x + 0.06, t.y, t.z - 0.12);
  r = U(r, M(sdRoundBox(sp, vec3(0.2, 0.022, 0.028), 0.01), WHITE));
  vec3 ar = vec3(repC(t.x - 0.33, 0.07, 3.0), t.y, t.z - 0.12);
  r = U(r, M(sdRoundBox(ar, vec3(0.026, 0.022, 0.028), 0.01), WHITE));
  r = U(r, M(sdRoundBox(vec3(t.x + 0.38, t.y, t.z - 0.12), vec3(0.05, 0.022, 0.028), 0.01), WHITE));
  return r;
}

vec2 mug(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.09, 0.0), 0.075, 0.09, 0.012), LILAC + ROUND);
  r = Sc(r, M(sdCylinder(p - vec3(0.0, 0.17, 0.0), 0.058, 0.03), GRAPHITE + ROUND));
  r = U(r, M(sdTorus(rotX(p - vec3(0.09, 0.09, 0.0), PI * 0.5), 0.045, 0.014), LILAC + SOLID));
  r = seam(r, p.y - 0.035);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.5, -0.3), vec3(0.5, 0.55, 0.5), r.x)) r = U(r, monitor(p));
  vec3 kq = p - vec3(-0.05, 0.0, 0.48);
  if (near(kq - vec3(0.0, 0.07, 0.0), vec3(0.55, 0.12, 0.25), r.x)) r = U(r, keyboard(kq));
  float c = cable(p, vec3(0.25, 0.04, 0.28), vec3(0.45, 0.025, 0.25), vec3(0.42, 0.025, 0.02), vec3(0.3, 0.025, -0.08), 0.018);
  r = U(r, M(c, GRAPHITE + SOLID));
  if (near(p - vec3(0.66, 0.1, 0.16), vec3(0.2, 0.12, 0.12), r.x)) r = U(r, mug(rotY(p - vec3(0.66, 0.0, 0.16), 0.5)));
  return r;
}
