#pragma frame {"size":[800,640],"center":[-0.1,0.36,-0.5],"zoom":270,"bounds":[[-0.8,-0.05,-1.6],[0.8,1.5,0.6]]}

#define SH 0.14
#define RUN 0.27

vec2 stair(vec3 p, float i) {
  float hx = 0.6 - 0.04 * i, hz = 0.33 - 0.02 * i;
  vec3 c = vec3(0.0, SH * 0.5 + SH * i, -RUN * i);
  vec3 q = p - c;
  float m = i > 3.5 ? LIME : WHITE;
  vec2 r = M(sdRoundBox(q, vec3(hx, SH * 0.5, hz), 0.018), m);
  r = seam(r, q.y + SH * 0.2);
  if (m == WHITE) r = paint(r, q.y + SH * 0.2, STEEL);
  vec3 fx = faceX(q, hx);
  vec3 fz = faceZ(q, hz);
  float ink = m == WHITE ? STEEL : WHITE;
  r = decal(r, fz, sdRect(fz.xz - vec2(-hx + 0.16, 0.025), vec2(0.09, 0.018), 0.018), ink);
  r = decal(r, fz, sdRect(fz.xz - vec2(-hx + 0.36, 0.025), vec2(0.05, 0.018), 0.018), ink);
  r = U(r, led(fz - vec3(hx - 0.09, 0.0, 0.025), 0.024, i == 4.0 ? GRAPHITE : i == 3.0 ? LIME : STEEL));
  r = vents(r, fx - vec3(0.0, 0.0, 0.025), 4.0, 0.06, 0.035, 0.026, GRAPHITE);
  vec3 t = mirX(faceY(q, SH * 0.5));
  r = U(r, screw(t - vec3(hx - 0.05, 0.0, hz - 0.05), 0.02));
  return r;
}

vec2 chip(vec3 p, float m) {
  return M(sdRoundBox(p - vec3(0.0, 0.025, 0.0), vec3(0.05, 0.025, 0.05), 0.01), m);
}

vec2 flag(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.015, 0.0), 0.07, 0.015, 0.006), GRAPHITE + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.27, 0.0), 0.016, 0.25), GRAPHITE + ROUND));
  r = U(r, M(sdSphere(p - vec3(0.0, 0.54, 0.0), 0.05), GRAPHITE + SOLID));
  vec3 f = p - vec3(0.16, 0.42, 0.0);
  float notch = (f.x - 0.08 - 0.9 * abs(f.y)) * 0.74;
  vec2 fl = M(extrude(max(sdRect(f.xy, vec2(0.15, 0.075), 0.0), notch), f.z, 0.01), CORAL);
  return U(r, fl);
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 5; i++) r = U(r, stair(p, float(i)));
  r = U(r, chip(p - vec3(-0.3, SH, 0.18), WHITE));
  r = U(r, chip(rotY(p - vec3(-0.12, SH, 0.2), 0.4), MINT));
  r = U(r, chip(p - vec3(0.2, SH * 2.0, -0.1), SKY));
  r = U(r, chip(rotY(p - vec3(-0.2, SH * 3.0, -0.4), -0.3), CORAL));
  r = U(r, chip(p - vec3(0.25, SH * 4.0, -0.66), WHITE));
  r = U(r, chip(rotY(p - vec3(0.25, SH * 4.0 + 0.05, -0.66), 0.35), LILAC));
  r = U(r, flag(p - vec3(-0.1, SH * 5.0, -RUN * 4.0)));
  return r;
}
