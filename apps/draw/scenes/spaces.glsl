#pragma frame {"size":[800,640],"center":[0.2,0.8,0.0],"zoom":222,"bounds":[[-1.0,-0.2,-1.1],[1.1,2.2,1.0]]}

const float GAP = 0.72;
const vec3 MID = vec3(0.2, GAP, -0.2);

vec2 cube(vec3 q, vec3 h, float m) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, h.y, 0.0), h, 0.012), m);
  return seam(r, q.y - h.y * 1.4);
}

vec2 drum(vec3 q, float rad, float h, float m) {
  vec2 r = M(sdRoundCylinder(q - vec3(0.0, h, 0.0), rad, h, 0.01), m + ROUND);
  return seam(r, q.y - h * 1.5);
}

vec2 plate(vec3 q, float m) {
  vec2 r = M(sdRoundBox(q, vec3(0.62, 0.04, 0.62), 0.03), m);
  r = seam(r, q.y + 0.01);
  vec3 t = faceY(q, 0.04);
  r = inset(r, t, vec2(0.52, 0.52), 0.04, 0.012, m);
  return r;
}

vec2 posts(vec3 p) {
  vec2 pc = vec2(p.x - 0.09, p.z + 0.09);
  vec2 c = vec2(abs(pc.x) - 0.43, abs(pc.y) - 0.43);
  vec3 q = vec3(c.x, p.y - GAP, c.y);
  vec2 r = M(sdCylinder(q, 0.018, GAP), GRAPHITE + SOLID);
  vec3 k = vec3(c.x, abs(p.y - GAP) - GAP, c.y);
  float col = sdRoundCylinder(vec3(k.x, abs(k.y) - 0.065, k.z), 0.035, 0.022, 0.008);
  col = min(col, sdRoundCylinder(vec3(c.x, p.y - GAP, c.y), 0.035, 0.065, 0.008));
  r = U(r, M(col, WHITE + ROUND));
  return r;
}

vec2 bottomStuff(vec3 q) {
  vec2 r = cube(q - vec3(-0.04, 0.0, 0.36), vec3(0.1, 0.1, 0.1), WHITE);
  r = U(r, cube(q - vec3(-0.04, 0.2, 0.36), vec3(0.07, 0.06, 0.07), WHITE));
  r = U(r, led(faceY(q - vec3(-0.04, 0.0, 0.36), 0.32), 0.022, LIME));
  r = U(r, drum(q - vec3(0.24, 0.0, 0.4), 0.08, 0.06, WHITE));
  r = U(r, cube(q - vec3(-0.42, 0.0, 0.0), vec3(0.06, 0.05, 0.13), STEEL));
  return r;
}

vec2 midStuff(vec3 q) {
  vec2 r = cube(q - vec3(0.3, 0.0, 0.32), vec3(0.12, 0.08, 0.09), WHITE);
  r = U(r, drum(q - vec3(0.4, 0.0, -0.1), 0.07, 0.1, WHITE));
  r = U(r, cube(q - vec3(-0.08, 0.0, 0.42), vec3(0.08, 0.05, 0.08), WHITE));
  r = U(r, cube(q - vec3(-0.08, 0.1, 0.42), vec3(0.06, 0.05, 0.06), STEEL));
  return r;
}

vec2 topStuff(vec3 q) {
  vec2 r = cube(q - vec3(-0.18, 0.0, -0.15), vec3(0.14, 0.14, 0.14), WHITE);
  vec3 f = faceX(q - vec3(-0.18, 0.0, -0.15), 0.14);
  r = vents(r, f - vec3(0.0, 0.0, 0.11), 3.0, 0.05, 0.1, 0.02, GRAPHITE);
  r = U(r, cube(q - vec3(0.25, 0.0, -0.25), vec3(0.09, 0.09, 0.09), WHITE));
  r = U(r, drum(q - vec3(0.22, 0.0, 0.22), 0.1, 0.07, WHITE));
  r = U(r, cube(q - vec3(-0.25, 0.0, 0.3), vec3(0.11, 0.04, 0.08), STEEL));
  r = U(r, cube(q - vec3(-0.25, 0.08, 0.3), vec3(0.08, 0.04, 0.06), WHITE));
  r = U(r, led(faceY(q - vec3(0.22, 0.0, 0.22), 0.14), 0.024, LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 b = p - vec3(0.0, 0.0, 0.0);
  vec3 m = p - MID;
  vec3 t = p - vec3(0.0, 2.0 * GAP, 0.0);
  if (near(b - vec3(0.0, 0.1, 0.0), vec3(0.66, 0.16, 0.66), r.x)) r = U(r, U(plate(b, SUN), bottomStuff(faceY(b, 0.028))));
  if (near(m - vec3(0.0, 0.1, 0.0), vec3(0.66, 0.16, 0.66), r.x)) r = U(r, U(plate(m, LIME), midStuff(faceY(m, 0.028))));
  if (near(t - vec3(0.0, 0.12, 0.0), vec3(0.66, 0.2, 0.66), r.x)) r = U(r, U(plate(t, LILAC), topStuff(faceY(t, 0.028))));
  if (near(p - vec3(0.09, GAP, -0.09), vec3(0.5, GAP + 0.05, 0.5), r.x)) r = U(r, posts(p));
  return r;
}
