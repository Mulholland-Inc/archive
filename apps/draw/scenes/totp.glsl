#pragma frame {"size":[900,720],"center":[0.06,0.12,0.0],"zoom":300,"bounds":[[-0.9,-0.1,-0.7],[1.5,0.6,0.7]]}

vec2 caps(vec2 r, vec3 t, float accent) {
  float x = abs(t.x) - 0.26;
  float i = repI(x, 0.15, 3.0);
  vec3 q = vec3(repC(x, 0.15, 3.0), t.y + 0.002, t.z - 0.04);
  float m = t.x > 0.0 && i == 2.0 ? accent : WHITE;
  vec2 c = M(sdRoundBox(q, vec3(0.058, 0.032, 0.085), 0.012), m);
  return U(r, seam(c, q.y - 0.012));
}

vec2 fob(vec3 p, float body) {
  vec3 b = p - vec3(0.0, 0.12, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.62, 0.12, 0.36), 0.04), body);
  r = seam(r, b.y - 0.08);
  r = seam(r, -(b.y + 0.08));
  vec3 t = faceY(p, 0.24);
  r = inset(r, t - vec3(0.0, 0.0, 0.04), vec2(0.5, 0.135), 0.03, 0.03, GRAPHITE);
  r = caps(r, t, LIME);
  r = U(r, screw(vec3(abs(t.x) - 0.565, t.y, abs(t.z) - 0.3), 0.018));
  r = U(r, led(t - vec3(0.46, 0.0, -0.21), 0.022, LIME));
  r = U(r, M(extrude(sdRect(t.xz - vec2(-0.26, -0.21), vec2(0.2, 0.018), 0.018), t.y + 0.006, 0.01), STEEL));
  vec3 fz = faceZ(p, 0.36);
  r = vents(r, fz - vec3(-0.22, 0.0, 0.12), 7.0, 0.06, 0.08, 0.024, GRAPHITE);
  vec3 fx = faceX(p, 0.62);
  r = inset(r, fx - vec3(0.2, 0.0, 0.12), vec2(0.06, 0.03), 0.02, 0.01, STEEL);
  return r;
}

vec2 loop(vec3 p) {
  vec3 e = p - vec3(0.68, 0.12, -0.12);
  vec2 r = M(sdTorus(e.xzy, 0.065, 0.018), STEEL + SOLID);
  vec3 g = rotZ(e, -0.42);
  float c = sdCircle(g.xz - vec2(0.17, 0.0), 0.17);
  return U(r, M(extrude(abs(c) - 0.022, g.y, 0.014), LIME));
}

vec2 map(vec3 p) {
  return U(fob(p, WHITE), loop(p));
}
