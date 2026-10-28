#pragma frame {"size":[800,640],"center":[0.02,0.0,-0.22],"zoom":222,"bounds":[[-1.8,-0.1,-0.9],[2.2,1.2,1.2]]}

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

vec2 card(vec3 p, float tag) {
  vec2 c = M(sdRoundBox(p, vec3(0.17, 0.02, 0.12), 0.012), WHITE);
  vec3 q = faceY(p, 0.02);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, -0.06), vec2(0.09, 0.012), 0.012), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.02, 0.0), vec2(0.07, 0.012), 0.012), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, 0.06), vec2(0.08, 0.012), 0.012), STEEL);
  if (tag > 0.5) c = U(c, M(sdRoundBox(q - vec3(-0.11, 0.028, 0.0), vec3(0.035, 0.028, 0.085), 0.012), tag));
  return c;
}

vec2 gantry(vec3 p) {
  vec3 g = p - vec3(-0.2, 0.0, 0.0);
  vec3 po = g - vec3(0.0, 0.0, -0.46);
  vec2 r = M(sdBox(po - vec3(0.0, 0.5, 0.0), vec3(0.04, 0.5, 0.04)), GRAPHITE);
  r = U(r, M(sdRoundBox(po - vec3(0.0, 0.025, 0.0), vec3(0.11, 0.025, 0.11), 0.01), WHITE));
  r = U(r, screw(faceY(mirXZ(po), 0.05) - vec3(0.075, 0.0, 0.075), 0.016));
  r = U(r, M(sdBox(po - vec3(0.0, 0.88, 0.14), vec3(0.03, 0.035, 0.14)), GRAPHITE));
  vec3 h = g - vec3(0.0, 0.84, 0.0);
  vec2 head = M(sdRoundBox(h, vec3(0.17, 0.13, 0.2), 0.02), WHITE);
  head = seam(head, h.y + 0.06);
  vec3 hx = faceX(h, 0.17);
  head = vents(head, hx - vec3(-0.06, 0.0, 0.02), 4.0, 0.06, 0.1, 0.022, GRAPHITE);
  vec3 hz = faceZ(h, 0.2);
  head = vents(head, hz - vec3(-0.06, 0.0, 0.02), 3.0, 0.06, 0.1, 0.022, GRAPHITE);
  r = U(r, head);
  r = U(r, led(hx - vec3(0.12, 0.0, 0.04), 0.028, LIME));
  r = U(r, screw(hz - vec3(0.1, 0.0, 0.04), 0.02));
  vec3 cq = h - vec3(0.0, 0.17, 0.0);
  float cz = repC(cq.z, 0.12, 3.0);
  float ci = repI(cq.z, 0.12, 3.0);
  float cm = ci == 0.0 ? CORAL : ci == 1.0 ? LILAC : LIME;
  vec3 cc = vec3(cq.x, cq.y, cz);
  r = U(r, M(sdRoundCylinder(cc, 0.045, 0.05, 0.012), cm + ROUND));
  r = U(r, M(sdCylinder(cc - vec3(0.0, 0.055, 0.0), 0.02, 0.012), WHITE + ROUND));
  vec3 s = g - vec3(0.0, 0.66, 0.0);
  r = U(r, M(sdCylinder(s, 0.03, 0.07), STEEL + ROUND));
  r = U(r, M(sdRoundBox(s - vec3(0.0, -0.11, 0.0), vec3(0.09, 0.02, 0.11), 0.01), STEEL));
  return r;
}

vec2 crate(vec3 p) {
  vec3 q = p - vec3(0.0, 0.2, 0.0);
  vec2 c = M(max(sdBox(q, vec3(0.34, 0.2, 0.3)), -sdBox(q - vec3(0.0, 0.08, 0.0), vec3(0.31, 0.2, 0.27))), WHITE);
  c = stripes(c, q.y + 0.2, 0.067);
  vec3 k = mirXZ(q) - vec3(0.34, 0.0, 0.3);
  c = U(c, M(sdBox(k, vec3(0.035, 0.205, 0.035)), WHITE));
  return c;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(-0.25, 0.25, 0.0), vec3(1.25, 0.28, 0.3), r.x)) r = U(r, conveyor(p - vec3(-1.45, 0.0, 0.0), 2.4, 0.2, 0.42));
  if (near(p - vec3(-0.2, 0.52, 0.0), vec3(0.3, 0.53, 0.6), r.x)) r = U(r, gantry(p));
  if (near(p - vec3(-0.2, 0.47, 0.0), vec3(1.2, 0.08, 0.2), r.x)) {
    r = U(r, card(p - vec3(-1.16, 0.43, 0.0), 0.0));
    r = U(r, card(p - vec3(-0.68, 0.43, 0.0), 0.0));
    r = U(r, card(p - vec3(-0.2, 0.43, 0.0), LILAC));
    r = U(r, card(p - vec3(0.28, 0.43, 0.0), CORAL));
    r = U(r, card(p - vec3(0.76, 0.43, 0.0), LIME));
  }
  vec3 cp = rotY(p - vec3(1.7, 0.0, 0.02), -0.22);
  if (near(cp - vec3(0.0, 0.25, 0.0), vec3(0.4, 0.28, 0.36), r.x)) {
    r = U(r, crate(cp));
    r = U(r, card(rotY(cp - vec3(-0.1, 0.11, -0.08), 0.15), CORAL));
    r = U(r, card(rotY(cp - vec3(0.12, 0.11, 0.1), -0.2), LIME));
    r = U(r, card(rotX(rotY(cp - vec3(0.1, 0.18, -0.1), 0.35), 0.1), LILAC));
    r = U(r, card(rotZ(rotY(cp - vec3(-0.08, 0.2, 0.08), -0.1), 0.12), LIME));
    r = U(r, card(rotX(rotZ(rotY(cp - vec3(0.04, 0.27, 0.0), 0.5), -0.08), 0.06), CORAL));
    r = U(r, card(rotZ(rotY(cp - vec3(0.1, 0.33, 0.06), -0.35), 0.06), LILAC));
    r = U(r, card(rotX(rotY(cp - vec3(-0.12, 0.34, -0.06), 0.1), -0.08), LIME));
  }
  if (near(p - vec3(1.24, 0.6, 0.0), vec3(0.25, 0.2, 0.2), r.x)) r = U(r, card(rotX(rotZ(p - vec3(1.24, 0.6, 0.0), -0.75), 0.1), LILAC));
  vec3 sp = p - vec3(-0.85, 0.0, 0.62);
  if (near(sp - vec3(0.0, 0.15, 0.0), vec3(0.3, 0.2, 0.25), r.x)) {
    vec2 b = M(sdRoundBox(sp - vec3(0.0, 0.03, 0.0), vec3(0.24, 0.03, 0.18), 0.01), WHITE);
    b = U(b, screw(faceY(mirXZ(sp), 0.06) - vec3(0.2, 0.0, 0.14), 0.016));
    r = U(r, b);
    for (int i = 0; i < 4; i++) {
      float fi = float(i);
      r = U(r, card(rotY(sp - vec3(0.0, 0.08 + 0.045 * fi, 0.0), 0.05 * sin(fi * 2.1)), 0.0));
    }
  }
  vec3 tp = p - vec3(0.5, 0.0, 0.66);
  if (near(tp - vec3(0.0, 0.1, 0.0), vec3(0.3, 0.15, 0.24), r.x)) {
    vec3 tq = tp - vec3(0.0, 0.07, 0.0);
    vec2 t = M(max(sdRoundBox(tq, vec3(0.24, 0.07, 0.18), 0.015), -sdBox(tq - vec3(0.0, 0.05, 0.0), vec3(0.21, 0.07, 0.15))), WHITE);
    t = seam(t, tq.y + 0.03);
    r = U(r, t);
    r = U(r, M(sdRoundBox(rotY(tq - vec3(-0.1, 0.0, -0.06), 0.4), vec3(0.035, 0.028, 0.085), 0.012), LIME));
    r = U(r, M(sdRoundBox(rotY(tq - vec3(0.04, 0.0, -0.05), -0.3), vec3(0.035, 0.028, 0.085), 0.012), CORAL));
    r = U(r, M(sdRoundBox(rotY(tq - vec3(0.13, 0.0, 0.03), 0.2), vec3(0.035, 0.028, 0.085), 0.012), LILAC));
    r = U(r, M(sdRoundBox(rotY(tq - vec3(-0.06, 0.0, 0.08), 1.3), vec3(0.035, 0.028, 0.085), 0.012), LILAC));
    r = U(r, M(sdRoundBox(rotZ(rotY(tq - vec3(-0.01, 0.045, 0.02), -1.1), 0.15), vec3(0.035, 0.028, 0.085), 0.012), LIME));
  }
  return r;
}
