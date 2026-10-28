#pragma frame {"size":[1200,720],"center":[0.0,0.45,0.0],"zoom":250,"bounds":[[-2.6,-0.1,-2.6],[2.6,2.0,2.6]]}

vec2 conveyor(vec3 p, float len, float w, float h) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(q.x, q.y - h + 0.045, abs(q.z) - w), vec3(len * 0.5, 0.045, 0.022)), WHITE);
  vec3 ro = vec3(repC(q.x, 0.11, floor(len / 0.11)), q.y - h + 0.04, q.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.03, w), STEEL + SOLID));
  float n = floor(len / 0.8) + 1.0;
  float lx = repC(q.x, len / (n - 1.0) * 0.96, n);
  vec3 lq = vec3(lx, q.y - (h - 0.09) * 0.5, abs(q.z) - w);
  r = U(r, M(sdBox(lq, vec3(0.022, (h - 0.09) * 0.5, 0.022)), GRAPHITE));
  r = U(r, M(sdBox(vec3(lx, q.y - 0.14, q.z), vec3(0.016, 0.016, w)), GRAPHITE));
  return r;
}

vec2 card(vec3 p, float m) {
  vec2 c = M(sdRoundBox(p, vec3(0.12, 0.008, 0.085), 0.005), WHITE);
  vec3 q = faceY(p, 0.008);
  c = decal(c, q, sdRect(q.xz - vec2(-0.055, 0.0), vec2(0.035, 0.055), 0.01), m);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, -0.035), vec2(0.04, 0.009), 0.009), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, 0.0), vec2(0.03, 0.009), 0.009), STEEL);
  return decal(c, q, sdRect(q.xz - vec2(0.05, 0.035), vec2(0.04, 0.009), 0.009), STEEL);
}

vec2 bundle(vec3 p, float m) {
  vec3 t = p - vec3(0.0, 0.025, 0.0);
  vec2 r = M(sdRoundBox(t, vec3(0.2, 0.025, 0.16), 0.01), STEEL);
  r = seam(r, t.y);
  for (int i = 0; i < 4; i++) {
    float fi = float(i);
    vec3 q = rotY(p - vec3(0.0, 0.065 + fi * 0.022, 0.0), 0.06 * sin(fi * 2.3));
    r = U(r, card(q, i == 3 ? m : STEEL));
  }
  float band = max(abs(p.x) - 0.03, sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(0.2, 0.065, 0.14), 0.01));
  band = max(band, -sdBox(p - vec3(0.0, 0.08, 0.0), vec3(0.2, 0.055, 0.125)));
  r = U(r, M(band, m + SOLID));
  return r;
}

vec2 gate(vec3 p) {
  vec3 c = vec3(p.x, p.y - 0.5, abs(p.z) - 0.36);
  vec2 r = M(sdRoundBox(c, vec3(0.1, 0.5, 0.07), 0.015), WHITE);
  r = seam(r, c.y + 0.32);
  vec3 h = p - vec3(0.0, 1.05, 0.0);
  vec2 top = M(sdRoundBox(h, vec3(0.12, 0.1, 0.45), 0.02), WHITE);
  top = seam(top, h.y + 0.04);
  vec3 fx = faceX(h, 0.12);
  top = vents(top, fx - vec3(0.2, 0.0, 0.02), 4.0, 0.045, 0.08, 0.018, GRAPHITE);
  r = U(r, top);
  r = U(r, led(fx - vec3(-0.15, 0.0, 0.02), 0.028, LIME));
  r = U(r, led(fx - vec3(-0.25, 0.0, 0.02), 0.028, STEEL));
  r = U(r, screw(fx - vec3(-0.36, 0.0, -0.05), 0.016));
  r = U(r, screw(fx - vec3(0.36, 0.0, -0.05), 0.016));
  vec3 s = p - vec3(0.0, 0.92, 0.0);
  r = U(r, M(sdBox(s, vec3(0.06, 0.03, 0.29)), GRAPHITE));
  vec3 fz = faceZ(p, 0.43);
  r = U(r, screw(fz - vec3(0.0, 0.0, 0.1), 0.016));
  return r;
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.0, 0.45, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.4, 0.45, 0.42), 0.02), WHITE);
  vec3 fx = faceX(p, 0.4);
  vec3 d = vec3(repC(fx.x, 0.4, 2.0), fx.y, repC(fx.z - 0.45, 0.28, 3.0));
  r = U(r, M(extrude(sdRect(d.xz, vec2(0.17, 0.1), 0.015), d.y - 0.012, 0.012), WHITE));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.03, 0.03), vec3(0.06, 0.012, 0.012), 0.008), GRAPHITE));
  vec3 fz = faceZ(p, 0.42);
  r = vents(r, fz - vec3(0.0, 0.0, 0.7), 5.0, 0.06, 0.14, 0.022, GRAPHITE);
  r = U(r, screw(vec3(abs(fz.x) - 0.32, fz.y, fz.z - 0.82), 0.016));
  r = U(r, screw(vec3(abs(fz.x) - 0.32, fz.y, fz.z - 0.08), 0.016));
  vec3 tp = p - vec3(0.0, 0.92, 0.0);
  r = U(r, M(sdChamferBox(tp, vec3(0.43, 0.025, 0.45), 0.01), STEEL));
  r = U(r, bundle((p - vec3(-0.05, 0.945, 0.05)) / 1.25, LILAC) * vec2(1.25, 1.0));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 cv = p - vec3(-1.6, 0.0, 0.0);
  if (near(p - vec3(-0.4, 0.25, 0.0), vec3(1.3, 0.3, 0.3), r.x)) r = U(r, conveyor(cv, 2.4, 0.22, 0.42));
  if (near(p - vec3(-0.25, 0.6, 0.0), vec3(0.2, 0.62, 0.5), r.x)) r = U(r, gate(p - vec3(-0.25, 0.0, 0.0)));
  if (near(p - vec3(-1.25, 0.5, 0.0), vec3(0.3, 0.2, 0.3), r.x)) r = U(r, bundle((p - vec3(-1.25, 0.42, 0.0)) / 1.25, LIME) * vec2(1.25, 1.0));
  if (near(p - vec3(0.4, 0.5, 0.0), vec3(0.3, 0.2, 0.3), r.x)) r = U(r, bundle((p - vec3(0.35, 0.42, 0.0)) / 1.25, LIME) * vec2(1.25, 1.0));
  vec3 cp = p - vec3(1.35, 0.0, 0.0);
  if (near(cp - vec3(0.0, 0.47, 0.0), vec3(0.5, 0.5, 0.5), r.x)) r = U(r, cabinet(cp));
  return r;
}
