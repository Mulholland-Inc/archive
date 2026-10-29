#pragma frame {"size":[1200,960],"center":[0.0,0.69,0.0],"zoom":135,"trim":false,"bounds":[[-4.0,-0.1,-4.0],[4.0,3.2,4.0]]}
vec2 plinth(vec3 p, float h) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.1, 0.0), vec3(h, 0.1, h), 0.03), WHITE);
  r = seam(r, p.y - 0.15);
  return paint(r, p.y - 0.06, STEEL);
}

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.17, 0.014, 0.12)), m);
  return seam(e, p.z + 0.12 - (0.17 - abs(p.x)) * 0.6);
}

vec2 cabinet(vec3 p) {
  vec3 c = p - vec3(0.0, 0.65, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.4, 0.65, 0.32), 0.02), WHITE);
  vec3 f = faceZ(c, 0.32);
  vec3 d = vec3(f.x, f.y, repC(f.z, 0.42, 3.0));
  r = U(r, plate(d, vec2(0.34, 0.18), 0.02, 0.014, WHITE));
  r = U(r, M(sdBox(d - vec3(0.0, 0.03, 0.08), vec3(0.11, 0.016, 0.025)), STEEL));
  return r;
}

vec2 silo(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.56, 0.04, 0.015), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.79, 0.0), 0.44, 0.71), GRAPHITE + ROUND));
  r = U(r, M(sdRoundCylinder(vec3(p.x, repC(p.y - 0.79, 0.5, 3.0), p.z), 0.5, 0.21, 0.05), WHITE + ROUND));
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 1.54, 0.0), 0.22, 0.04, 0.015), STEEL + ROUND));
  r = U(r, led(p - vec3(0.0, 1.58, 0.0), 0.07, LIME));
  return r;
}

vec2 tower(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.6, 0.0), vec3(0.32, 0.6, 0.32), 0.03), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.22, 0.0), vec3(0.38, 0.04, 0.38), 0.02), WHITE));
  vec3 s = faceZ(p, 0.32) - vec3(0.0, 0.0, 0.88);
  r = Sc(r, M(max(sdRect(s.xz, vec2(0.21, 0.04), 0.02), -(s.y + 0.15)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(s.xz, vec2(0.26, 0.09), 0.04), -sdRect(s.xz, vec2(0.21, 0.04), 0.02)), s.y - 0.015, 0.015), STEEL));
  r = U(r, envelope(rotX(p - vec3(0.0, 0.88, 0.38), -0.12), WHITE));
  vec3 dr = faceZ(p, 0.32) - vec3(0.0, 0.0, 0.38);
  r = U(r, plate(dr, vec2(0.22, 0.26), 0.02, 0.012, WHITE));
  r = U(r, M(sdBox(dr - vec3(0.14, 0.03, 0.0), vec3(0.02, 0.016, 0.06)), STEEL));
  return r;
}

vec2 gridFace(vec2 r, vec3 f, vec2 n, vec2 cell) {
  vec2 id = vec2(repI(f.x, cell.x, n.x), repI(f.z, cell.y, n.y));
  vec3 q = vec3(repC(f.x, cell.x, n.x), f.y, repC(f.z, cell.y, n.y));
  float m = id.y == n.y - 1.0 ? SKY : WHITE;
  return U(r, plate(q, cell * 0.5 - 0.022, 0.012, 0.012, m));
}

vec2 board(vec3 p) {
  vec3 l = vec3(p.x, p.y, abs(p.z) - 0.46);
  vec2 r = M(sdBox(l - vec3(0.0, 0.03, 0.0), vec3(0.24, 0.03, 0.06)), STEEL);
  r = U(r, M(sdBox(l - vec3(0.0, 0.3, 0.0), vec3(0.035, 0.3, 0.035)), STEEL));
  vec3 b = p - vec3(0.0, 0.88, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.05, 0.5, 0.64), 0.02), WHITE);
  r = U(r, gridFace(body, faceX(b, 0.05), vec2(4.0, 4.0), vec2(0.29, 0.22)));
  return r;
}

const vec3 PC = vec3(-1.9, 0.2, 1.9);
const vec3 PS = vec3(-1.7, 0.2, -1.7);
const vec3 PB = vec3(1.9, 0.2, -1.9);
const vec3 PM = vec3(1.7, 0.2, 1.7);

vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  float s = 1.0 - t;
  vec3 q = s * s * s * a + 3.0 * s * s * t * b + 3.0 * s * t * t * c + t * t * t * d;
  return vec3(q.x, max(q.y, 0.035), q.z);
}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float w) {
  vec3 lo = min(min(a, b), min(c, d)), hi = max(max(a, b), max(c, d));
  float bb = sdBox(p - (lo + hi) * 0.5, (hi - lo) * 0.5 + 0.1);
  if (bb > 0.05) return bb;
  float k = 1e9;
  vec3 q0 = a;
  for (int i = 1; i <= 24; i++) {
    vec3 q1 = bez(a, b, c, d, float(i) / 24.0);
    k = min(k, sdCapsule(p, q0, q1, w));
    q0 = q1;
  }
  return k;
}

vec2 tape(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  return M(sdCapsule(p, bez(a, b, c, d, t - 0.025), bez(a, b, c, d, t + 0.025), 0.06), SUN + SOLID);
}

vec2 sheet(vec3 p, vec3 c, float a) {
  vec3 q = rotY(p - c, a);
  return M(sdBox(q - vec3(0.0, 0.008, 0.0), vec3(0.17, 0.008, 0.22)), WHITE);
}

vec2 map(vec3 p) {

  vec2 r = plinth(p - PC + vec3(0.0, 0.2, 0.0), 0.6);
  r = U(r, plinth(p - PS + vec3(0.0, 0.2, 0.0), 0.68));
  r = U(r, plinth(p - PB + vec3(0.0, 0.2, 0.0), 0.6));
  r = U(r, plinth(p - PM + vec3(0.0, 0.2, 0.0), 0.6));

  if (near(p - PC - vec3(0.0, 0.65, 0.0), vec3(0.45, 0.7, 0.4), r.x)) r = U(r, cabinet(p - PC));
  if (near(p - PS - vec3(0.0, 0.85, 0.0), vec3(0.6, 0.85, 0.6), r.x)) r = U(r, silo(p - PS));
  if (near(p - PB - vec3(0.0, 0.7, 0.0), vec3(0.3, 0.75, 0.7), r.x)) r = U(r, board(p - PB));
  if (near(p - PM - vec3(0.0, 0.65, 0.1), vec3(0.42, 0.7, 0.5), r.x)) r = U(r, tower(p - PM));

  if (p.y - 0.02 < r.x) {
    r = U(r, sheet(p, vec3(-0.35, 0.0, 0.25), 0.45));
    r = U(r, sheet(p, vec3(0.55, 0.0, -0.45), -0.7));
    r = U(r, sheet(p, vec3(0.15, 0.0, 1.05), 1.15));
    r = U(r, sheet(p, vec3(-0.95, 0.0, -0.35), 0.2));
    r = U(r, sheet(p, vec3(0.95, 0.0, 0.2), -0.25));
  }
  vec3 a1 = vec3(-1.8, 1.5, 1.7), b1 = vec3(-1.7, 0.5, 0.6), c1 = vec3(-2.0, 0.4, -0.5), d1 = vec3(-1.7, 1.15, -1.22);
  vec3 a2 = vec3(-1.55, 1.58, -1.55), b2 = vec3(-0.6, 0.7, -1.6), c2 = vec3(0.8, 0.5, -1.4), d2 = vec3(1.9, 1.3, -1.5);
  vec3 a3 = vec3(-1.5, 0.65, 2.05), b3 = vec3(1.6, -0.6, 2.9), c3 = vec3(-1.4, -0.6, 0.9), d3 = vec3(1.55, 0.5, 2.02);
  vec3 a4 = vec3(1.75, 1.26, 1.6), b4 = vec3(2.0, 0.5, 0.7), c4 = vec3(2.1, 0.3, -0.4), d4 = vec3(1.96, 0.95, -1.3);
  r = U(r, M(cable(p, a1, b1, c1, d1, 0.035), WHITE + SOLID));
  r = U(r, M(cable(p, a2, b2, c2, d2, 0.035), STEEL + SOLID));
  r = U(r, M(cable(p, a3, b3, c3, d3, 0.035), WHITE + SOLID));
  r = U(r, M(cable(p, a4, b4, c4, d4, 0.035), STEEL + SOLID));
  r = U(r, tape(p, a3, b3, c3, d3, 0.62));
  r = U(r, tape(p, a2, b2, c2, d2, 0.45));
  return r;
}
