#pragma frame {"size":[1400,760],"center":[0.0,0.55,0.0],"zoom":250,"bounds":[[-3.0,-0.1,-3.0],[3.0,2.2,3.0]]}

vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  float s = 1.0 - t;
  return s * s * s * a + 3.0 * s * s * t * b + 3.0 * s * t * t * c + t * t * t * d;
}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float m = 1e9;
  vec3 q = a;
  for (int i = 1; i <= 16; i++) {
    vec3 n = bez(a, b, c, d, float(i) / 16.0);
    m = min(m, sdCapsule(p, q, n, r));
    q = n;
  }
  return m;
}

float sdTri2(vec2 p, vec2 a, vec2 b, vec2 c) {
  vec2 e0 = b - a, e1 = c - b, e2 = a - c;
  vec2 v0 = p - a, v1 = p - b, v2 = p - c;
  vec2 pq0 = v0 - e0 * clamp(dot(v0, e0) / dot(e0, e0), 0.0, 1.0);
  vec2 pq1 = v1 - e1 * clamp(dot(v1, e1) / dot(e1, e1), 0.0, 1.0);
  vec2 pq2 = v2 - e2 * clamp(dot(v2, e2) / dot(e2, e2), 0.0, 1.0);
  float s = sign(e0.x * e2.y - e0.y * e2.x);
  vec2 d = min(min(vec2(dot(pq0, pq0), s * (v0.x * e0.y - v0.y * e0.x)), vec2(dot(pq1, pq1), s * (v1.x * e1.y - v1.y * e1.x))), vec2(dot(pq2, pq2), s * (v2.x * e2.y - v2.y * e2.x)));
  return -sqrt(d.x) * sign(d.y);
}

float sdStar(vec2 p, float r, float m) {
  float an = PI / 4.0;
  float en = PI / m;
  vec2 acs = vec2(cos(an), sin(an));
  vec2 ecs = vec2(cos(en), sin(en));
  float bn = mod(atan(p.x, p.y), 2.0 * an) - an;
  p = length(p) * vec2(cos(bn), abs(sin(bn)));
  p -= r * acs;
  p += ecs * clamp(-dot(p, ecs), 0.0, r * acs.y / ecs.y);
  return length(p) * sign(p.x);
}

float glyph(vec2 g, float i) {
  if (i == 0.0) return length(g) - 0.06;
  if (i == 1.0) return sdStar(rot(PI * 0.25) * g, 0.085, 2.6);
  if (i == 2.0) return sdTri2(g, vec2(-0.07, -0.05), vec2(0.07, -0.05), vec2(0.0, 0.07));
  return abs(length(g) - 0.05) - 0.02;
}

vec2 card(vec3 p, float m) {
  vec2 c = M(sdRoundBox(p, vec3(0.1, 0.006, 0.07), 0.004), WHITE);
  vec3 q = faceY(p, 0.006);
  c = decal(c, q, sdRect(q.xz - vec2(-0.045, 0.0), vec2(0.03, 0.045), 0.008), m);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, -0.03), vec2(0.035, 0.008), 0.008), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.03, 0.0), vec2(0.025, 0.008), 0.008), STEEL);
  return decal(c, q, sdRect(q.xz - vec2(0.04, 0.03), vec2(0.035, 0.008), 0.008), STEEL);
}

vec2 assistant(vec3 q, float i, float m) {
  vec3 b = q - vec3(0.0, 0.07, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.27, 0.07, 0.2), 0.03), WHITE);
  r = seam(r, b.y + 0.025);
  vec3 f = vec3(b.x, b.z - 0.2, b.y);
  r = U(r, led(f - vec3(0.2, 0.0, 0.01), 0.016, i == 1.0 ? LIME : STEEL));
  r = vents(r, f - vec3(0.08, 0.0, 0.01), 3.0, 0.03, 0.05, 0.012, GRAPHITE);
  vec3 top = faceY(b, 0.07);
  r = U(r, screw(vec3(abs(top.x) - 0.22, top.y, top.z + 0.15), 0.014));
  r = U(r, M(sdRoundBox(top - vec3(-0.12, 0.015, 0.11), vec3(0.11, 0.015, 0.075), 0.01), STEEL));
  r = U(r, card(top - vec3(-0.12, 0.036, 0.11), i == 1.0 ? LIME : STEEL));
  vec3 s = q - vec3(0.06, 0.42, -0.06);
  vec2 sh = vec2(s.x, s.y);
  float d2 = sdRect(sh, vec2(0.23, 0.16), 0.08);
  d2 = min(d2, sdTri2(sh, vec2(-0.16, -0.08), vec2(-0.02, -0.12), vec2(-0.19, -0.24)));
  vec2 sign2 = M(extrude(d2, s.z, 0.035), m);
  sign2 = seam(sign2, abs(s.z) - 0.02);
  r = U(r, sign2);
  r = U(r, M(extrude(glyph(sh, i), s.z - 0.045, 0.012), WHITE + SOLID));
  r = U(r, M(sdBox(q - vec3(0.06, 0.18, -0.06), vec3(0.03, 0.06, 0.02)), GRAPHITE));
  r = U(r, M(sdRoundBox(q - vec3(0.06, 0.25, -0.06), vec3(0.07, 0.015, 0.035), 0.008), STEEL));
  return r;
}

vec2 padlock(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, -0.05, 0.0), vec3(0.045, 0.038, 0.018), 0.01), SUN);
  r = U(r, M(sdCylinder(vec3(q.x, q.z - 0.02, q.y + 0.055), 0.008, 0.004), GRAPHITE + SOLID));
  float sh = max(sdTorus(vec3(q.x, q.z, q.y + 0.01), 0.03, 0.008), -(q.y + 0.012));
  sh = min(sh, sdCapsule(q, vec3(-0.03, -0.01, 0.0), vec3(-0.03, -0.03, 0.0), 0.008));
  sh = min(sh, sdCapsule(q, vec3(0.03, -0.01, 0.0), vec3(0.03, -0.03, 0.0), 0.008));
  return U(r, M(sh, STEEL + SOLID));
}

float labelColor(float c, float r) {
  float k = mod(c * 2.0 + r * 3.0, 5.0);
  if (k < 0.5) return CORAL;
  if (k < 1.5) return STEEL;
  if (k < 2.5) return LILAC;
  if (k < 3.5) return STEEL;
  return SUN;
}

vec2 drawers(vec2 r, vec3 f, float side) {
  float cx = repC(f.x, 0.34, 3.0);
  float ci = repI(f.x, 0.34, 3.0);
  float cz = repC(f.z - 0.66, 0.24, 4.0);
  float ri = repI(f.z - 0.66, 0.24, 4.0);
  vec3 c = vec3(cx, f.y, cz);
  bool open = side < 0.5 && ci == 1.0 && ri == 2.0;
  if (open) return r;
  r = U(r, M(extrude(sdRect(c.xz, vec2(0.15, 0.1), 0.015), c.y - 0.012, 0.012), WHITE));
  vec3 h = c - vec3(0.0, 0.03, -0.03);
  r = U(r, M(sdRoundBox(h, vec3(0.05, 0.012, 0.012), 0.008), GRAPHITE));
  vec3 l = c - vec3(0.0, 0.024, 0.04);
  r = U(r, M(extrude(sdRect(l.xz, vec2(0.06, 0.03), 0.006), l.y, 0.004), labelColor(ci + side * 3.0, ri) + SOLID));
  return r;
}

vec2 drawerOut(vec3 p) {
  vec3 b = p - vec3(0.0, 0.0, -0.12);
  vec2 r = M(max(sdBox(b, vec3(0.14, 0.09, 0.2)), -sdBox(b - vec3(0.0, 0.03, 0.0), vec3(0.12, 0.09, 0.18))), WHITE);
  vec3 c = p - vec3(0.0, 0.0, 0.1);
  r = U(r, M(extrude(sdRect(c.xy, vec2(0.15, 0.1), 0.015), c.z, 0.012), WHITE));
  r = U(r, M(sdRoundBox(vec3(c.x, c.y + 0.03, c.z - 0.03), vec3(0.05, 0.012, 0.012), 0.008), GRAPHITE));
  r = U(r, M(extrude(sdRect(c.xy - vec2(0.0, 0.04), vec2(0.06, 0.03), 0.006), c.z - 0.016, 0.004), LIME + SOLID));
  for (int i = 0; i < 6; i++) {
    float fi = float(i);
    vec3 k = b - vec3(0.0, 0.03 + (fi == 3.0 ? 0.06 : 0.0), 0.15 - fi * 0.058);
    vec2 cd = M(sdBox(k, vec3(0.11, 0.07, 0.006)), WHITE);
    float tm = fi == 3.0 ? LIME : fi == 1.0 ? LILAC : fi == 5.0 ? SUN : STEEL;
    vec3 t = k - vec3(-0.06 + mod(fi, 3.0) * 0.06, 0.085, 0.0);
    cd = U(cd, M(sdRoundBox(t, vec3(0.035, 0.022, 0.006), 0.004), tm));
    r = U(r, cd);
  }
  return r;
}

vec2 plant(vec3 p) {
  vec3 q = p - vec3(0.0, 0.11, 0.0);
  vec2 r = M(sdCone(q, 0.11, 0.08, 0.11), CORAL + ROUND);
  r = seam(r, q.y - 0.07);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.215, 0.0), 0.095, 0.01), GRAPHITE + ROUND));
  for (int i = 0; i < 5; i++) {
    float a = float(i) * TAU / 5.0 + 0.4;
    vec3 d = vec3(cos(a), 0.0, sin(a));
    vec3 c = p - vec3(0.0, 0.3, 0.0) - d * 0.09 - vec3(0.0, 0.04 * float(i % 2), 0.0);
    vec3 l = rotY(c, -a);
    l = rotZ(l, 0.55);
    r = U(r, M(sdEllipsoid(l, vec3(0.13, 0.025, 0.06)), SUN + SOLID));
  }
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.36, 0.0), vec3(0.05, 0.13, 0.05)), SUN + SOLID));
  return r;
}

vec2 binders(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 3; i++) {
    float fi = float(i);
    vec3 q = rotY(p - vec3(0.0, 0.035 + fi * 0.07, 0.0), 0.15 * sin(fi * 2.1));
    vec2 b = M(sdRoundBox(q, vec3(0.2, 0.035, 0.15), 0.008), WHITE);
    float m = i == 0 ? CORAL : i == 1 ? SUN : LILAC;
    b = paint(b, q.x - 0.15, m);
    b = U(b, M(sdBox(vec3(q.x - 0.2, q.y, q.z), vec3(0.006, 0.012, 0.05)), GRAPHITE + SOLID));
    r = U(r, b);
  }
  return r;
}

vec2 catalog(vec3 p) {
  vec3 b = p - vec3(0.0, 0.65, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.56, 0.55, 0.56), 0.02), WHITE);
  vec3 pl = p - vec3(0.0, 0.05, 0.0);
  r = U(r, M(sdChamferBox(pl, vec3(0.62, 0.05, 0.62), 0.02), STEEL));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.5, p.y - 0.13, abs(p.z) - 0.5), vec3(0.04, 0.04, 0.04)), STEEL));
  vec3 tp = p - vec3(0.0, 1.23, 0.0);
  r = U(r, seam(M(sdChamferBox(tp, vec3(0.6, 0.03, 0.6), 0.015), WHITE), tp.y));
  r = U(r, screw(faceY(mirXZ(tp), 0.03) - vec3(0.52, 0.0, 0.52), 0.016));
  vec3 fz = faceZ(p, 0.56);
  vec3 fx = vec3(-p.z, p.x - 0.56, p.y);
  if (abs(fz.y) < 0.35) {
    vec3 hole = vec3(fz.x, fz.y, fz.z - 0.78);
    r = Sc(r, M(max(sdRect(hole.xz, vec2(0.13, 0.085), 0.01), -(hole.y + 0.3)), GRAPHITE));
    r = drawers(r, fz, 0.0);
  }
  if (abs(fx.y) < 0.12) r = drawers(r, fx, 1.0);
  return r;
}

vec2 port(vec3 f) {
  vec2 r = M(extrude(sdRect(f.xz, vec2(0.07, 0.045), 0.012), f.y - 0.012, 0.012), STEEL);
  return Sc(r, M(max(sdRect(f.xz, vec2(0.035, 0.02), 0.005), -(f.y + 0.03)), GRAPHITE));
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.65, 0.0), vec3(0.7, 0.7, 0.7), r.x)) r = U(r, catalog(p));
  if (near(p - vec3(0.0, 0.8, 0.72), vec3(0.2, 0.2, 0.3), r.x)) r = U(r, drawerOut(p - vec3(0.0, 0.78, 0.72)));
  if (near(p - vec3(0.63, 0.6, 0.0), vec3(0.1, 0.5, 0.5), r.x)) {
    r = U(r, padlock(vec3(-p.z + 0.34, p.y - 0.51, p.x - 0.63)));
    r = U(r, padlock(vec3(-p.z - 0.34, p.y - 0.99, p.x - 0.63)));
    r = U(r, padlock(vec3(-p.z, p.y - 0.27, p.x - 0.63)));
  }
  if (near(p - vec3(-0.3, 1.5, 0.3), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, plant(p - vec3(-0.3, 1.26, 0.3)));
  if (near(p - vec3(0.25, 1.36, -0.25), vec3(0.3, 0.14, 0.3), r.x)) r = U(r, binders(rotY(p - vec3(0.25, 1.26, -0.25), 0.2)));
  if (near(p - vec3(0.22, 1.3, 0.3), vec3(0.28, 0.08, 0.28), r.x)) {
    r = U(r, card((rotY(p - vec3(0.22, 1.272, 0.3), 0.3)) / 1.6, STEEL) * vec2(1.6, 1.0));
    r = U(r, card((rotY(p - vec3(0.21, 1.292, 0.28), -0.1)) / 1.6, STEEL) * vec2(1.6, 1.0));
    r = U(r, card((rotY(p - vec3(0.24, 1.312, 0.29), 0.15)) / 1.6, LILAC) * vec2(1.6, 1.0));
  }
  r = U(r, port(faceZ(p, 0.62) - vec3(-0.3, 0.0, 0.05)));
  r = U(r, port(faceZ(p, 0.62) - vec3(0.3, 0.0, 0.05)));
  r = U(r, port(vec3(-p.z - 0.3, p.x - 0.62, p.y - 0.05)));
  r = U(r, port(vec3(-p.z + 0.3, p.x - 0.62, p.y - 0.05)));

  float K = 1.25;
  vec3 a1 = (p - vec3(-1.55, 0.0, 0.95)) / K;
  if (near(a1 - vec3(0.0, 0.3, 0.0), vec3(0.4, 0.4, 0.3), r.x)) r = U(r, assistant(a1, 0.0, STEEL) * vec2(K, 1.0));
  vec3 a2 = (p - vec3(-0.45, 0.0, 1.95)) / K;
  if (near(a2 - vec3(0.0, 0.3, 0.0), vec3(0.4, 0.4, 0.3), r.x)) r = U(r, assistant(a2, 1.0, LIME) * vec2(K, 1.0));
  vec3 a3 = (p - vec3(1.95, 0.0, -0.45)) / K;
  vec3 q3 = vec3(-a3.z, a3.y, a3.x);
  if (near(q3 - vec3(0.0, 0.3, 0.0), vec3(0.4, 0.4, 0.3), r.x)) r = U(r, assistant(q3, 2.0, LILAC) * vec2(K, 1.0));
  vec3 a4 = (p - vec3(0.95, 0.0, -1.55)) / K;
  vec3 q4 = vec3(-a4.z, a4.y, a4.x);
  if (near(q4 - vec3(0.0, 0.3, 0.0), vec3(0.4, 0.4, 0.3), r.x)) r = U(r, assistant(q4, 3.0, SUN) * vec2(K, 1.0));

  float c = 1e9;
  c = min(c, cable(p, vec3(-0.3, 0.05, 0.66), vec3(-0.3, 0.03, 1.0), vec3(-1.2, 0.03, 0.6), vec3(-1.49, 0.03, 0.68), 0.026));
  c = min(c, cable(p, vec3(0.3, 0.05, 0.66), vec3(0.3, 0.03, 1.15), vec3(-0.35, 0.03, 1.3), vec3(-0.39, 0.03, 1.68), 0.026));
  c = min(c, cable(p, vec3(0.66, 0.05, -0.3), vec3(1.0, 0.03, -0.3), vec3(0.6, 0.03, -1.2), vec3(0.68, 0.03, -1.49), 0.026));
  c = min(c, cable(p, vec3(0.66, 0.05, 0.3), vec3(1.15, 0.03, 0.3), vec3(1.3, 0.03, -0.35), vec3(1.68, 0.03, -0.39), 0.026));
  r = U(r, M(c, CORAL + SOLID));
  return r;
}
