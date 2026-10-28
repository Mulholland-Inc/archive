#pragma frame {"size":[800,640],"center":[-0.266,0.035,0.056],"zoom":225,"bounds":[[-1.6,-0.1,-1.6],[1.6,1.4,1.6]]}

float dot2(vec3 v) { return dot(v, v); }

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

float sdTri(vec2 p, float r) {
  const float k = 1.7320508;
  p.x = abs(p.x) - r;
  p.y = p.y + r / k;
  if (p.x + k * p.y > 0.0) p = vec2(p.x - k * p.y, -k * p.x - p.y) / 2.0;
  p.x -= clamp(p.x, -2.0 * r, 0.0);
  return -length(p) * sign(p.y);
}

vec2 tile(vec3 p, float i) {
  float body = i == 1.0 ? LILAC : WHITE;
  vec3 b = p - vec3(0.0, 0.11, 0.0);
  vec2 t = M(sdRoundBox(b, vec3(0.22, 0.11, 0.22), 0.05), body);
  t = seam(t, b.y + 0.05);
  vec3 f = faceZ(p, 0.22);
  t = inset(t, f - vec3(0.0, 0.0, 0.055), vec2(0.13, 0.025), 0.025, 0.012, i == 1.0 ? WHITE : STEEL);
  t = U(t, led(f - vec3(0.09, 0.0, 0.055), 0.016, LIME));
  t = vents(t, f - vec3(-0.06, 0.0, 0.055), 3.0, 0.035, 0.03, 0.012, GRAPHITE);
  vec3 top = faceY(p, 0.22);
  float g;
  float gm;
  if (i == 0.0) { g = length(top.xz) - 0.1; gm = LILAC; }
  else if (i == 1.0) { g = sdStar(rot(PI * 0.25) * top.xz, 0.13, 2.6); gm = WHITE; }
  else if (i == 2.0) { g = sdTri(vec2(top.x, -top.z) , 0.11); gm = CORAL; }
  else { g = abs(length(top.xz) - 0.085) - 0.032; gm = GRAPHITE; }
  t = U(t, M(extrude(g, top.y - 0.02, 0.02), gm + SOLID));
  vec3 c = mirXZ(top) - vec3(0.155, 0.0, 0.155);
  t = U(t, screw(c, 0.016));
  vec3 bk = p - vec3(0.0, 0.07, -0.25);
  t = U(t, M(sdRoundBox(bk, vec3(0.045, 0.035, 0.04), 0.01), STEEL));
  return t;
}

vec2 disk(vec3 p, float i) {
  vec2 d = M(sdRoundCylinder(p, 0.36, 0.11, 0.04), (i > 1.5 ? LIME : WHITE) + ROUND);
  vec3 q = faceX(rotY(p, -PI * 0.25), 0.36);
  d = vents(d, q - vec3(-0.07, 0.0, 0.0), 4.0, 0.045, 0.1, 0.018, GRAPHITE);
  d = U(d, led(q - vec3(0.12, 0.0, 0.02), 0.026, i > 1.5 ? WHITE : i > 0.5 ? LIME : STEEL));
  d = U(d, screw(q - vec3(0.12, 0.0, -0.05), 0.016));
  return d;
}

vec2 database(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.38, 0.0), 0.33, 0.36), GRAPHITE + ROUND);
  float y = p.y - 0.13;
  r = U(r, disk(vec3(p.x, repL(y, 0.27, 3.0), p.z), repLI(y, 0.27, 3.0)));
  vec3 t = p - vec3(0.0, 0.78, 0.0);
  r = U(r, M(sdRoundCylinder(t, 0.25, 0.025, 0.012), WHITE + ROUND));
  r = U(r, M(sdRoundCylinder(t - vec3(0.0, 0.035, 0.0), 0.1, 0.022, 0.01), LIME + ROUND));
  r = U(r, screw(faceY(polarY(p - vec3(0.0, 0.75, 0.0), 6.0), 0.0) - vec3(0.305, 0.0, 0.0), 0.016));
  return r;
}

vec2 socket(vec3 p) {
  vec3 b = p - vec3(0.0, 0.2, 0.0);
  vec2 s = M(sdChamferBox(b, vec3(0.42, 0.2, 0.24), 0.03), WHITE);
  s = seam(s, b.y - 0.11);
  vec3 f = faceZ(p, 0.24);
  s = inset(s, f - vec3(0.0, 0.0, 0.11), vec2(0.37, 0.07), 0.03, 0.015, STEEL);
  vec3 pq = vec3(repC(f.x, 0.18, 4.0), f.y, f.z - 0.11);
  s = inset(s, pq, vec2(0.045, 0.035), 0.01, 0.05, GRAPHITE);
  vec3 top = faceY(p, 0.4);
  s = vents(s, top - vec3(-0.12, 0.0, -0.05), 8.0, 0.05, 0.22, 0.02, GRAPHITE);
  vec2 r = s;
  vec3 lq = vec3(repC(f.x, 0.18, 4.0), f.y, f.z - 0.27);
  float li = repI(f.x, 0.18, 4.0);
  r = U(r, led(lq, 0.018, li == 3.0 ? STEEL : LIME));
  vec3 sc = vec3(abs(f.x) - 0.36, f.y, f.z - 0.27);
  r = U(r, screw(sc, 0.018));
  vec3 fx = faceX(p, 0.42);
  r = U(r, M(extrude(sdRect(fx.xz - vec2(0.0, 0.15), vec2(0.09, 0.07), 0.02), fx.y - 0.01, 0.01), STEEL));
  r = U(r, M(sdRoundBox(p - vec3(0.47, 0.15, 0.0), vec3(0.05, 0.045, 0.045), 0.012), GRAPHITE));
  r = U(r, screw(fx - vec3(0.0, 0.0, 0.3), 0.018));
  vec3 pl = vec3(repC(p.x, 0.18, 4.0), p.y - 0.11, p.z - 0.3);
  r = U(r, M(sdRoundBox(pl, vec3(0.04, 0.032, 0.06), 0.012), STEEL));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.25, 0.05), vec3(0.6, 0.3, 0.45), r.x)) r = U(r, socket(p));
  vec3 dc = p - vec3(0.2, 0.0, -1.05);
  if (near(dc - vec3(0.0, 0.45, 0.0), vec3(0.45, 0.5, 0.45), r.x)) r = U(r, database(dc));
  float tx = p.x + 0.84;
  float ti = repLI(tx, 0.56, 4.0);
  vec3 tq = vec3(repL(tx, 0.56, 4.0), p.y, p.z - 1.15);
  if (near(p - vec3(0.0, 0.15, 1.1), vec3(1.15, 0.2, 0.35), r.x)) r = U(r, tile(tq, ti));
  float c = 1e9;
  for (int i = 0; i < 4; i++) {
    float xi = -0.84 + 0.56 * float(i);
    float xp = -0.27 + 0.18 * float(i);
    c = min(c, cable(p, vec3(xi, 0.07, 0.86), vec3(xi, 0.035, 0.6), vec3(xp, 0.11, 0.66), vec3(xp, 0.11, 0.36), 0.026));
  }
  c = min(c, cable(p, vec3(0.52, 0.15, 0.0), vec3(1.0, 0.035, 0.05), vec3(1.05, 0.035, -1.05), vec3(0.64, 0.12, -1.05), 0.035));
  r = U(r, M(c, GRAPHITE + SOLID));
  vec2 col = M(sdRoundBox(p - vec3(0.6, 0.12, -1.05), vec3(0.06, 0.045, 0.045), 0.012), STEEL);
  r = U(r, seam(col, p.x - 0.6));
  return r;
}
