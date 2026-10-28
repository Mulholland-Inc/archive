#pragma frame {"size":[1200,720],"center":[0.0,0.7,0.0],"zoom":165,"bounds":[[-2.2,-0.1,-1.4],[2.0,2.4,1.4]]}

const float G = 0.1;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 ground(vec3 p) {
  vec3 b = p - vec3(-0.2, G * 0.5, -0.2);
  vec2 r = M(sdRoundBox(b, vec3(1.75, G * 0.5, 0.95), 0.03), WHITE);
  r = seam(r, p.y - 0.035);
  r = paint(r, p.y - 0.035, STEEL);
  vec3 cs = mirXZ(faceY(b, G * 0.5));
  r = U(r, screw(vec3(cs.x - 1.65, cs.y, cs.z - 0.85), 0.024));
  vec3 mat = p - vec3(-1.1, G + 0.012, 0.0);
  vec2 m = M(sdRoundBox(mat, vec3(0.3, 0.012, 0.42), 0.012), STEEL);
  m = stripes(m, mat.z, 0.07);
  r = U(r, m);
  return r;
}

vec2 wall(vec3 p) {
  vec3 w = p - vec3(-1.62, G + 0.85, -0.2);
  vec2 r = M(sdBox(w, vec3(0.1, 0.85, 0.92)), WHITE);
  r = stripes(r, w.z + 0.15, 0.3);
  r = seam(r, -(abs(w.z - 0.2) - 0.6));
  vec3 fx = faceX(w, 0.1);
  float op = sdRect(fx.xz - vec2(0.2, -0.2), vec2(0.42, 0.76), 0.0);
  r = Sc(r, M(max(op, -(fx.y + 0.16)), GRAPHITE));
  vec2 fr = M(extrude(max(sdRect(fx.xz - vec2(0.2, -0.17), vec2(0.5, 0.82), 0.02), -op), fx.y, 0.03), SUN);
  fr = M(max(fr.x, fx.z + 0.95), fr.y);
  r = U(r, fr);
  vec3 lp = fx - vec3(0.2, 0.0, 0.7);
  r = U(r, M(sdRoundBox(lp - vec3(0.0, 0.04, 0.0), vec3(0.12, 0.04, 0.04), 0.02), WHITE));
  r = U(r, led(lp - vec3(0.0, 0.06, -0.02), 0.03, SUN));
  vec3 d = p - vec3(-1.52 + 0.4, G + 0.75, 0.43);
  vec2 door = M(sdRoundBox(d, vec3(0.4, 0.74, 0.025), 0.012), BLUE);
  vec3 df = faceZ(d, 0.025);
  door = inset(door, df - vec3(0.0, 0.0, 0.3), vec2(0.28, 0.3), 0.03, 0.012, BLUE);
  door = inset(door, df - vec3(0.0, 0.0, -0.36), vec2(0.28, 0.26), 0.03, 0.012, BLUE);
  door = U(door, M(sdSphere(df - vec3(0.3, 0.04, 0.0), 0.04), STEEL + SOLID));
  r = U(r, door);
  return r;
}

vec2 crate(vec3 p, float m, float k) {
  p /= k;
  vec2 r = M(sdRoundBox(p, vec3(0.28, 0.2, 0.24), 0.015), m);
  r = seam(r, -abs(p.x) + 0.0);
  vec3 t = faceY(p, 0.2);
  r = U(r, M(extrude(sdRect(t.xz, vec2(0.06, 0.24), 0.0), t.y, 0.006), PAPER));
  vec3 fz = faceZ(p, 0.24);
  r = U(r, M(extrude(sdRect(fz.xz - vec2(0.0, 0.14), vec2(0.06, 0.06), 0.0), fz.y, 0.006), PAPER));
  vec3 fx = faceX(p, 0.28);
  r = decal(r, fx, bar(fx.xz, -0.14, 0.04, 0.18, 0.018), WHITE);
  r = decal(r, fx, bar(fx.xz, -0.14, -0.04, 0.11, 0.018), WHITE);
  return vec2(r.x * k, r.y);
}

vec2 load(vec3 q) {
  vec3 c = q - vec3(0.36, 0.42, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.3, 0.36, 0.3), 0.025), WHITE);
  vec3 fx = faceX(c, 0.3);
  vec3 dr = vec3(fx.x, fx.y, repC(fx.z, 0.24, 3.0));
  r = seam(r, -(abs(dr.z) - 0.105));
  r = U(r, M(sdRoundBox(dr - vec3(0.0, 0.02, 0.04), vec3(0.09, 0.02, 0.022), 0.01), SUN));
  vec3 fz = faceZ(c, 0.3);
  r = vents(r, fz - vec3(0.0, 0.0, 0.2), 5.0, 0.07, 0.12, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fz.x) - 0.2, fz.y, fz.z + 0.22), 0.018));
  vec3 d = q - vec3(0.36, 0.78, 0.0);
  r = U(r, M(sdRoundCylinder(d - vec3(0.0, 0.03, 0.0), 0.27, 0.03, 0.01), STEEL + ROUND));
  for (int i = 0; i < 2; i++) {
    vec3 k = d - vec3(0.0, 0.17 + 0.25 * float(i), 0.0);
    vec2 disc = M(sdRoundCylinder(k, 0.27, 0.105, 0.03), WHITE + ROUND);
    vec3 f = faceX(rotY(k, -PI * 0.25), 0.27);
    disc = vents(disc, f - vec3(-0.07, 0.0, 0.0), 3.0, 0.05, 0.09, 0.022, GRAPHITE);
    disc = U(disc, led(f - vec3(0.09, 0.0, 0.0), 0.024, i == 1 ? LIME : STEEL));
    r = U(r, disc);
  }
  r = U(r, M(sdCylinder(d - vec3(0.0, 0.3, 0.0), 0.2, 0.25), GRAPHITE + ROUND));
  r = U(r, M(sdRoundCylinder(d - vec3(0.0, 0.57, 0.0), 0.18, 0.025, 0.01), LIME + ROUND));
  vec3 s = q - vec3(0.36, 0.5, 0.0);
  float strap = max(abs(sdBox(s, vec3(0.315, 0.5, 0.315))) - 0.012, abs(s.y - 0.0) - 0.04);
  r = U(r, M(strap, CORAL + SOLID));
  r = U(r, M(sdRoundBox(faceX(s, 0.33) - vec3(0.0, 0.0, 0.0), vec3(0.06, 0.012, 0.06), 0.01), GRAPHITE));
  return r;
}

vec2 dolly(vec3 p) {
  vec3 pv = vec3(-0.12, 0.17, 0.0);
  vec3 q = rotZ(p - pv, 0.32) + pv;
  vec2 r = M(1e9, 0.0);
  vec3 rl = vec3(q.x + 0.06, q.y - 0.95, abs(q.z) - 0.3);
  r = U(r, M(sdCylinder(rl, 0.03, 0.9), GRAPHITE + ROUND));
  r = U(r, M(pipe(q, vec3(-0.06, 1.85, -0.3), vec3(-0.06, 1.85, 0.3), 0.03), GRAPHITE + SOLID));
  r = U(r, M(pipe(q, vec3(-0.06, 1.85, -0.3), vec3(-0.06, 1.85, 0.3), 0.045) , GRAPHITE + SOLID));
  vec3 gp = vec3(q.x + 0.06, q.y - 1.85, q.z);
  r = U(r, M(sdCylinder(rotX(gp, PI * 0.5), 0.05, 0.18), SUN + ROUND));
  vec3 cb = vec3(q.x + 0.06, repC(q.y - 0.95, 0.42, 4.0), q.z);
  r = U(r, M(sdCylinder(rotX(cb, PI * 0.5), 0.02, 0.3), GRAPHITE + ROUND));
  r = U(r, M(sdRoundBox(q - vec3(0.24, 0.035, 0.0), vec3(0.3, 0.02, 0.33), 0.01), STEEL));
  vec3 w = vec3(q.x + 0.12 - 0.0, q.y - 0.17, abs(q.z) - 0.42);
  vec3 wq = rotX(w, PI * 0.5);
  vec2 wheel = M(sdRoundCylinder(wq, 0.17, 0.05, 0.02), GRAPHITE + ROUND);
  wheel = U(wheel, M(sdCylinder(wq, 0.1, 0.058), WHITE + ROUND));
  wheel = U(wheel, M(sdCylinder(wq, 0.035, 0.07), STEEL + ROUND));
  r = U(r, wheel);
  r = U(r, M(sdCylinder(rotX(vec3(q.x + 0.12, q.y - 0.17, q.z), PI * 0.5), 0.018, 0.42), STEEL + ROUND));
  r = U(r, load(q - vec3(0.0, 0.055, 0.0)));
  return r;
}

vec2 plant(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.13, 0.0), 0.13, 0.13), WHITE + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.25, 0.0), 0.145, 0.03), WHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.27, 0.0), 0.12, 0.01), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.42, 0.0), 0.016, 0.16), GRAPHITE + SOLID));
  vec2 l = M(1e9, 0.0);
  for (int i = 0; i < 6; i++) {
    float a = float(i) * 1.05 + 0.3;
    vec3 q = rotY(p - vec3(0.0, 0.42 + 0.05 * float(i), 0.0), a);
    q = rotZ(q - vec3(0.1, 0.06, 0.0), -0.8 + 0.08 * float(i));
    l = U(l, M(sdEllipsoid(q - vec3(0.0, 0.08, 0.0), vec3(0.05, 0.14, 0.022)), MINT + SOLID));
  }
  return U(r, l);
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  vec3 g = p - vec3(0.0, G, 0.0);
  if (near(p - vec3(-1.4, 1.05, 0.0), vec3(0.4, 1.0, 1.2), r.x)) r = U(r, wall(p));
  if (near(g - vec3(0.3, 1.0, 0.1), vec3(0.85, 1.05, 0.6), r.x)) r = U(r, dolly(g - vec3(0.25, 0.0, 0.1)));
  if (near(g - vec3(1.2, 0.4, -0.6), vec3(0.4, 0.45, 0.4), r.x)) {
    r = U(r, crate(rotY(g - vec3(1.2, 0.2, -0.6), 0.1), SUN, 1.0));
    r = U(r, crate(rotY(g - vec3(1.2, 0.56, -0.62), -0.15), CORAL, 0.8));
  }
  return r;
}
