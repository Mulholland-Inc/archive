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

vec2 map(vec3 p) {

  vec2 r = plinth(p - PC + vec3(0.0, 0.2, 0.0), 0.6);
  r = U(r, plinth(p - PS + vec3(0.0, 0.2, 0.0), 0.68));
  r = U(r, plinth(p - PB + vec3(0.0, 0.2, 0.0), 0.6));
  r = U(r, plinth(p - PM + vec3(0.0, 0.2, 0.0), 0.6));

  if (near(p - PC - vec3(0.0, 0.65, 0.0), vec3(0.45, 0.7, 0.4), r.x)) r = U(r, cabinet(p - PC));
  if (near(p - PS - vec3(0.0, 0.85, 0.0), vec3(0.6, 0.85, 0.6), r.x)) r = U(r, silo(p - PS));
  if (near(p - PB - vec3(0.0, 0.7, 0.0), vec3(0.3, 0.75, 0.7), r.x)) r = U(r, board(p - PB));
  if (near(p - PM - vec3(0.0, 0.65, 0.1), vec3(0.42, 0.7, 0.5), r.x)) r = U(r, tower(p - PM));

  return r;
}
