#pragma frame {"size":[1200,720],"center":[0.0,0.45,0.0],"zoom":260,"bounds":[[-2.4,-0.1,-2.4],[2.4,2.0,2.4]]}

vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  float s = 1.0 - t;
  return s * s * s * a + 3.0 * s * s * t * b + 3.0 * s * t * t * c + t * t * t * d;
}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float m = 1e9;
  vec3 q = a;
  for (int i = 1; i <= 28; i++) {
    vec3 n = bez(a, b, c, d, float(i) / 28.0);
    m = min(m, sdCapsule(p, q, n, r));
    q = n;
  }
  return m;
}

vec2 card(vec3 p, float m) {
  vec2 c = M(sdRoundBox(p, vec3(0.1, 0.006, 0.07), 0.004), WHITE);
  vec3 q = faceY(p, 0.006);
  c = decal(c, q, sdRect(q.xz - vec2(-0.045, 0.0), vec2(0.03, 0.045), 0.008), m);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, -0.03), vec2(0.035, 0.008), 0.008), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.03, 0.0), vec2(0.025, 0.008), 0.008), STEEL);
  return decal(c, q, sdRect(q.xz - vec2(0.04, 0.03), vec2(0.035, 0.008), 0.008), STEEL);
}

vec2 laptop(vec3 p) {
  vec3 b = p - vec3(0.0, 0.025, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.025, 0.34), 0.012), WHITE);
  vec3 t = faceY(b, 0.025);
  r = grille(r, t - vec3(0.0, 0.0, -0.12), vec2(11.0, 4.0), 0.07, 0.022, STEEL);
  r = inset(r, t - vec3(0.0, 0.0, 0.18), vec2(0.15, 0.08), 0.02, 0.006, STEEL);
  r = U(r, led(faceZ(b, 0.34) - vec3(0.38, 0.0, 0.0), 0.012, STEEL));
  vec3 s = rotX(p - vec3(0.0, 0.05, -0.33), 0.22);
  s -= vec3(0.0, 0.34, 0.0);
  vec2 sc = M(sdRoundBox(s, vec3(0.5, 0.34, 0.018), 0.012), WHITE);
  sc = seam(sc, -s.z);
  vec3 f = vec3(s.x, s.z - 0.018, s.y);
  sc = inset(sc, f, vec2(0.45, 0.29), 0.02, 0.006, GRAPHITE);
  r = U(r, sc);
  vec3 q = vec3(f.x, f.y + 0.006, f.z);
  r = U(r, M(extrude(sdRect(q.xz - vec2(0.17, 0.17), vec2(0.21, 0.065), 0.05), q.y - 0.006, 0.006), STEEL + SOLID));
  r = U(r, M(extrude(sdRect(q.xz - vec2(-0.12, -0.08), vec2(0.28, 0.15), 0.05), q.y - 0.006, 0.006), WHITE + SOLID));
  vec3 w = vec3(q.x, q.y - 0.012, q.z);
  r = U(r, M(extrude(sdRect(w.xz - vec2(-0.14, -0.01), vec2(0.2, 0.016), 0.016), w.y - 0.003, 0.003), STEEL + SOLID));
  r = U(r, M(extrude(sdRect(w.xz - vec2(-0.18, -0.07), vec2(0.16, 0.016), 0.016), w.y - 0.003, 0.003), STEEL + SOLID));
  r = U(r, M(extrude(sdRect(w.xz - vec2(-0.2, -0.13), vec2(0.14, 0.016), 0.016), w.y - 0.003, 0.003), STEEL + SOLID));
  r = U(r, M(extrude(sdRect(w.xz - vec2(0.09, -0.13), vec2(0.05, 0.03), 0.03), w.y - 0.006, 0.006), LIME + SOLID));
  r = U(r, M(extrude(sdRect(w.xz - vec2(0.17, 0.17), vec2(0.16, 0.016), 0.016), w.y - 0.003, 0.003), WHITE + SOLID));
  return r;
}

vec2 folder(vec3 q, float m) {
  vec2 r = M(sdBox(q, vec3(0.006, 0.13, 0.22)), m);
  vec3 t = q - vec3(0.0, 0.15, -0.08);
  r = U(r, M(sdRoundBox(t, vec3(0.006, 0.03, 0.06), 0.004), m));
  vec3 sh = q - vec3(0.018, 0.04, 0.02);
  r = U(r, M(sdBox(sh, vec3(0.004, 0.13, 0.19)), WHITE));
  return r;
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.0, 0.56, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.34, 0.56, 0.38), 0.02), WHITE);
  vec3 tp = faceY(p, 1.12);
  r = U(r, M(sdChamferBox(tp - vec3(0.0, 0.02, 0.0), vec3(0.37, 0.02, 0.41), 0.01), STEEL));
  vec3 fx = vec3(-p.z, p.x - 0.34, p.y);
  r = Sc(r, M(max(sdRect(fx.xz - vec2(0.0, 0.82), vec2(0.32, 0.22), 0.01), -(fx.y + 0.5)), GRAPHITE));
  vec3 d = vec3(fx.x, fx.y, fx.z - 0.3);
  r = U(r, M(extrude(sdRect(d.xz, vec2(0.33, 0.23), 0.02), d.y - 0.012, 0.012), WHITE));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.03, 0.08), vec3(0.1, 0.012, 0.014), 0.008), GRAPHITE));
  r = U(r, M(extrude(sdRect(d.xz - vec2(0.0, -0.06), vec2(0.08, 0.04), 0.006), d.y - 0.026, 0.004), STEEL + SOLID));
  r = U(r, screw(faceZ(p, 0.38) - vec3(0.24, 0.0, 1.0), 0.016));
  r = U(r, screw(faceZ(p, 0.38) - vec3(0.24, 0.0, 0.12), 0.016));
  r = U(r, screw(faceZ(p, 0.38) - vec3(-0.24, 0.0, 1.0), 0.016));
  r = U(r, screw(faceZ(p, 0.38) - vec3(-0.24, 0.0, 0.12), 0.016));
  return r;
}

vec2 drawer(vec3 p) {
  vec3 b = p - vec3(-0.2, 0.0, 0.0);
  vec2 r = M(max(sdBox(b, vec3(0.3, 0.2, 0.31)), -sdBox(b - vec3(0.0, 0.05, 0.0), vec3(0.28, 0.2, 0.29))), WHITE);
  vec3 f = vec3(-p.z, p.x - 0.1, p.y);
  r = U(r, M(extrude(sdRect(f.xz, vec2(0.33, 0.23), 0.02), f.y - 0.012, 0.012), WHITE));
  r = U(r, M(sdRoundBox(f - vec3(0.0, 0.03, 0.08), vec3(0.1, 0.012, 0.014), 0.008), GRAPHITE));
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, -0.06), vec2(0.08, 0.04), 0.006), f.y - 0.026, 0.004), STEEL + SOLID));
  for (int i = 0; i < 7; i++) {
    float fi = float(i);
    if (i == 3) continue;
    float m = i == 1 ? LILAC : i == 5 ? SUN : i == 6 ? CORAL : STEEL;
    vec3 q = p - vec3(0.02 - fi * 0.07, -0.02, 0.0);
    vec2 fo = M(sdBox(q, vec3(0.006, 0.17, 0.27)), WHITE);
    vec3 t = q - vec3(0.0, 0.2, -0.16 + mod(fi * 0.13, 0.32));
    fo = U(fo, M(sdRoundBox(t, vec3(0.006, 0.03, 0.05), 0.004), m));
    r = U(r, fo);
  }
  return r;
}

vec2 mug(vec3 p) {
  vec3 q = p - vec3(0.0, 0.1, 0.0);
  vec2 r = M(max(sdCylinder(q, 0.08, 0.1), -sdCylinder(q - vec3(0.0, 0.1, 0.0), 0.065, 0.1)), WHITE + ROUND);
  r = seam(r, q.y - 0.05);
  r = U(r, M(sdCylinder(q - vec3(0.0, 0.06, 0.0), 0.066, 0.01), GRAPHITE + ROUND));
  vec3 hq = q - vec3(0.085, 0.0, 0.0);
  float h = max(sdTorus(vec3(hq.x, hq.z, hq.y), 0.045, 0.014), -hq.x);
  return U(r, M(h, WHITE + SOLID));
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 lp = p - vec3(-0.75, 0.0, 0.55);
  if (near(lp - vec3(0.0, 0.35, -0.1), vec3(0.6, 0.45, 0.5), r.x)) r = U(r, laptop(lp));
  if (near(p - vec3(0.05, 0.12, 0.95), vec3(0.2, 0.15, 0.2), r.x)) r = U(r, mug(rotY(p - vec3(0.05, 0.0, 0.95), 0.6)));
  vec3 cp = p - vec3(0.75, 0.0, -0.55);
  if (near(cp - vec3(0.0, 0.6, 0.0), vec3(0.45, 0.65, 0.45), r.x)) r = U(r, cabinet(cp));
  vec3 dp = cp - vec3(0.62, 0.82, 0.0);
  if (near(dp - vec3(-0.15, 0.05, 0.0), vec3(0.4, 0.3, 0.36), r.x)) r = U(r, drawer(dp));
  vec3 lf = rotZ(rotY(dp - vec3(-0.12, 0.66, 0.05), -0.12), -0.1) / 1.3;
  if (near(lf, vec3(0.1, 0.25, 0.3), r.x)) r = U(r, folder(lf, LIME) * vec2(1.3, 1.0));
  vec3 cs = p - vec3(0.35, 0.0, 0.55);
  if (near(cs - vec3(0.0, 0.05, 0.0), vec3(0.25, 0.08, 0.25), r.x)) {
    r = U(r, card(rotY(cs - vec3(0.0, 0.012, 0.0), 0.2) / 1.5, STEEL) * vec2(1.5, 1.0));
    r = U(r, card(rotY(cs - vec3(0.01, 0.032, 0.01), -0.15) / 1.5, STEEL) * vec2(1.5, 1.0));
    r = U(r, card(rotY(cs - vec3(-0.01, 0.052, 0.0), 0.05) / 1.5, LIME) * vec2(1.5, 1.0));
  }
  vec3 a = vec3(-0.66, 0.246, 0.305);
  vec3 z = vec3(1.25, 1.74, -0.6);
  float th = cable(p, a, a + vec3(0.25, 0.75, 0.25), z + vec3(-0.45, 0.45, -0.1), z, 0.014);
  r = U(r, M(th, LIME + SOLID));
  r = U(r, M(sdSphere(z, 0.03), LIME + SOLID));
  r = U(r, M(sdSphere(p - a, 0.026), GRAPHITE + SOLID));
  return r;
}
