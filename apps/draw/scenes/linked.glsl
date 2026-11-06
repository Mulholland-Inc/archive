#pragma frame {"size":[900,720],"center":[0.0,0.35,0.0],"zoom":240,"bounds":[[-1.3,-0.1,-1.1],[1.3,1.1,1.1]]}

const float TOP = 0.46;

vec2 caps(vec2 r, vec3 t) {
  float x = abs(t.x) - 0.26;
  vec3 q = vec3(repC(x, 0.15, 3.0), t.y + 0.002, t.z - 0.04);
  vec2 c = M(sdRoundBox(q, vec3(0.058, 0.032, 0.085), 0.012), WHITE);
  return U(r, seam(c, q.y - 0.012));
}

vec2 fob(vec3 p) {
  vec3 b = p - vec3(0.0, 0.12, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.62, 0.12, 0.36), 0.04), LIME);
  r = seam(r, b.y - 0.08);
  vec3 t = faceY(p, 0.24);
  r = inset(r, t - vec3(0.0, 0.0, 0.04), vec2(0.5, 0.135), 0.03, 0.03, GRAPHITE);
  r = caps(r, t);
  r = U(r, screw(vec3(abs(t.x) - 0.565, t.y, abs(t.z) - 0.3), 0.018));
  r = U(r, led(t - vec3(0.46, 0.0, -0.21), 0.022, LILAC));
  r = U(r, M(extrude(sdRect(t.xz - vec2(-0.26, -0.21), vec2(0.2, 0.018), 0.018), t.y + 0.006, 0.01), LILAC));
  return r;
}

vec2 dock(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(0.98, 0.05, 0.72), 0.02), WHITE);
  r = seam(r, p.y - 0.06);
  vec3 b = p - vec3(0.0, 0.1 + (TOP - 0.1) * 0.5, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.84, (TOP - 0.1) * 0.5, 0.58), 0.04), WHITE);
  body = seam(body, b.y - (TOP - 0.1) * 0.5 + 0.05);
  vec3 t = faceY(p, TOP);
  body = inset(body, t, vec2(0.68, 0.42), 0.06, 0.1, GRAPHITE);
  vec3 fz = faceZ(p, 0.58);
  body = vents(body, fz - vec3(-0.4, 0.0, 0.26), 8.0, 0.06, 0.13, 0.026, GRAPHITE);
  body = inset(body, fz - vec3(0.38, 0.0, 0.26), vec2(0.3, 0.09), 0.025, 0.012, WHITE);
  r = U(r, body);
  r = U(r, M(extrude(sdRect(fz.xz - vec2(0.38, 0.29), vec2(0.22, 0.018), 0.018), fz.y + 0.006, 0.01), STEEL));
  r = U(r, M(extrude(sdRect(fz.xz - vec2(0.3, 0.23), vec2(0.14, 0.018), 0.018), fz.y + 0.006, 0.01), STEEL));
  r = U(r, screw(vec3(abs(t.x) - 0.77, t.y, abs(t.z) - 0.5), 0.02));
  vec3 fx = faceX(p, 0.84);
  r = U(r, button(fx - vec3(-0.3, 0.012, 0.26), 0.07, 0.014, STEEL));
  r = U(r, button(fx - vec3(-0.08, 0.012, 0.26), 0.07, 0.014, STEEL));
  r = vents(r, fx - vec3(0.32, 0.0, 0.26), 4.0, 0.06, 0.13, 0.026, GRAPHITE);
  r = U(r, led(t - vec3(-0.5, 0.0, 0.5), 0.035, LIME));
  return r;
}

vec2 map(vec3 p) {
  return U(dock(p), fob(p - vec3(0.0, TOP - 0.1, 0.0)));
}
