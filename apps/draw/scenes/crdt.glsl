#pragma frame {"size":[800,640],"center":[0.23,0.41,0.67],"zoom":190,"bounds":[[-1.4,-0.05,-0.8],[2.0,1.4,2.0]]}

vec2 minipage(vec2 o, vec3 q, vec2 h, float caret) {
  o = decal(o, q, sdRect(q.xz, h, 0.015), WHITE);
  float s = h.x / 0.2;
  vec2 c = q.xz / s;
  o = decal(o, q, sdRect(c - vec2(-0.06, h.y / s - 0.06), vec2(0.1, 0.022), 0.022) * s, GRAPHITE);
  o = decal(o, q, sdRect(c - vec2(0.0, h.y / s - 0.13), vec2(0.15, 0.012), 0.012) * s, STEEL);
  o = decal(o, q, sdRect(c - vec2(-0.03, h.y / s - 0.18), vec2(0.12, 0.012), 0.012) * s, STEEL);
  o = decal(o, q, sdRect(c - vec2(-0.13, h.y / s - 0.25), vec2(0.022, 0.022), 0.006) * s, LIME);
  o = decal(o, q, sdRect(c - vec2(0.02, h.y / s - 0.25), vec2(0.1, 0.012), 0.012) * s, STEEL);
  o = decal(o, q, sdRect(c - vec2(-0.13, h.y / s - 0.31), vec2(0.022, 0.022), 0.006) * s, STEEL);
  o = decal(o, q, sdRect(c - vec2(0.0, h.y / s - 0.31), vec2(0.08, 0.012), 0.012) * s, STEEL);
  o = decal(o, q, sdRect(c - vec2(0.13, h.y / s - 0.18), vec2(0.008, 0.03), 0.004) * s, caret);
  return o;
}

vec2 laptop(vec3 p) {
  vec3 b = p - vec3(0.0, 0.03, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.03, 0.34), 0.015), WHITE);
  r = seam(r, b.y);
  vec3 t = faceY(p, 0.06);
  r = inset(r, t - vec3(0.0, 0.0, -0.08), vec2(0.42, 0.15), 0.02, 0.01, STEEL);
  vec3 k = t - vec3(0.0, 0.0, -0.08);
  vec2 kc = rep2(k.xz, vec2(0.07, 0.07), vec2(12.0, 4.0));
  vec2 keys = M(max(sdBox(vec3(kc.x, k.y + 0.005, kc.y), vec3(0.026, 0.008, 0.026)), sdRect(k.xz, vec2(0.41, 0.14), 0.0)), WHITE);
  r = U(r, keys);
  r = inset(r, t - vec3(0.0, 0.0, 0.22), vec2(0.13, 0.07), 0.02, 0.006, PAPER);
  vec3 h = rotX(p - vec3(0.0, 0.06, -0.33), -0.18);
  vec3 l = h - vec3(0.0, 0.32, 0.0);
  vec2 lid = M(sdRoundBox(l, vec3(0.5, 0.32, 0.018), 0.015), WHITE);
  vec3 s = faceZ(l, 0.018);
  lid = inset(lid, s, vec2(0.44, 0.27), 0.015, 0.006, BLUE);
  lid = minipage(lid, s - vec3(0.0, -0.006, -0.01), vec2(0.2, 0.22), LILAC);
  lid = U(lid, M(sdCylinder(rotZ(h, PI * 0.5), 0.022, 0.4), STEEL + ROUND));
  r = U(r, lid);
  vec3 ft = vec3(abs(p.x) - 0.4, p.y + 0.004, abs(p.z) - 0.26);
  r = U(r, M(sdCylinder(ft, 0.03, 0.006), GRAPHITE + ROUND));
  return r;
}

vec2 tablet(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.025, -0.02), vec3(0.34, 0.025, 0.2), 0.02), STEEL);
  r = seam(r, p.y - 0.025);
  r = U(r, M(sdBox(p - vec3(0.0, 0.08, 0.09), vec3(0.3, 0.03, 0.03)), STEEL));
  vec3 h = rotX(p - vec3(0.0, 0.08, 0.04), -0.32);
  vec3 l = h - vec3(0.0, 0.4, 0.0);
  vec2 tab = M(sdRoundBox(l, vec3(0.3, 0.4, 0.022), 0.04), WHITE);
  vec3 s = faceZ(l, 0.022);
  tab = inset(tab, s, vec2(0.25, 0.35), 0.02, 0.006, BLUE);
  tab = minipage(tab, s - vec3(0.0, -0.006, -0.02), vec2(0.2, 0.22), LIME);
  r = U(r, tab);
  r = U(r, button(faceY(l, 0.4) - vec3(0.18, 0.0, 0.0), 0.025, 0.008, STEEL));
  r = U(r, M(sdBox(vec3(l.x, l.y + 0.36, l.z - 0.015), vec3(0.025, 0.008, 0.02)), GRAPHITE));
  return r;
}

vec2 server(vec3 p) {
  vec3 b = p - vec3(0.0, 0.47, 0.0);
  vec2 r = M(sdChamferBox(b, vec3(0.32, 0.43, 0.32), 0.03), WHITE);
  r = seam(r, abs(b.y) - 0.41);
  vec3 f = faceZ(p, 0.32);
  float row = repI(f.z - 0.47, 0.22, 3.0);
  vec3 rq = vec3(f.x, f.y, repC(f.z - 0.47, 0.22, 3.0));
  if (abs(f.z - 0.47) < 0.33) r = seam(r, abs(rq.z) - 0.1);
  r = vents(r, rq - vec3(-0.08, 0.0, 0.0), 5.0, 0.05, 0.1, 0.022, GRAPHITE);
  r = U(r, led(rq - vec3(0.18, 0.0, 0.025), 0.026, row == 2.0 ? LIME : row == 1.0 ? LILAC : STEEL));
  r = U(r, led(rq - vec3(0.18, 0.0, -0.045), 0.018, GRAPHITE));
  vec3 x = faceX(p, 0.32);
  r = vents(r, vec3(x.x, x.y, repC(x.z - 0.62, 0.1, 3.0)), 5.0, 0.07, 0.035, 0.035, GRAPHITE);
  r = inset(r, x - vec3(0.0, 0.0, 0.24), vec2(0.07, 0.05), 0.015, 0.02, GRAPHITE);
  r = inset(r, f - vec3(0.0, 0.0, 0.1), vec2(0.07, 0.05), 0.015, 0.02, GRAPHITE);
  vec3 t = faceY(p, 0.9);
  r = inset(r, t, vec2(0.22, 0.22), 0.03, 0.012, STEEL);
  r = U(r, screw(mirXZ(t) - vec3(0.27, 0.0, 0.27), 0.022));
  r = U(r, M(sdRoundBox(vec3(abs(p.x) - 0.24, p.y - 0.02, abs(p.z) - 0.24), vec3(0.05, 0.02, 0.05), 0.01), GRAPHITE));
  return r;
}

vec2 cable(vec3 p, vec3 a, vec3 b) {
  return M(pipe(p, a, b, 0.025), GRAPHITE + SOLID);
}

vec2 op(vec3 p, float m) {
  return M(sdRoundBox(p, vec3(0.06), 0.012), m);
}

vec2 map(vec3 p) {
  vec2 r = server(p);
  vec3 lp = p - vec3(-0.72, 0.0, 1.32);
  r = U(r, laptop(lp));
  vec3 tp = p - vec3(1.45, 0.0, 0.0);
  r = U(r, tablet(tp));
  vec2 c = cable(p, vec3(0.0, 0.1, 0.36), vec3(0.0, 0.03, 0.5));
  c = U(c, cable(p, vec3(0.0, 0.03, 0.5), vec3(0.0, 0.03, 1.2)));
  c = U(c, cable(p, vec3(0.0, 0.03, 1.2), vec3(-0.1, 0.03, 1.32)));
  c = U(c, cable(p, vec3(-0.1, 0.03, 1.32), vec3(-0.18, 0.03, 1.32)));
  c = U(c, M(sdRoundBox(p - vec3(0.0, 0.1, 0.34), vec3(0.055, 0.04, 0.04), 0.01), STEEL));
  c = U(c, M(sdRoundBox(p - vec3(-0.19, 0.035, 1.32), vec3(0.04, 0.03, 0.05), 0.01), STEEL));
  c = U(c, cable(p, vec3(0.36, 0.24, 0.0), vec3(0.5, 0.03, 0.0)));
  c = U(c, cable(p, vec3(0.5, 0.03, 0.0), vec3(1.07, 0.03, 0.0)));
  c = U(c, M(sdRoundBox(p - vec3(0.34, 0.24, 0.0), vec3(0.04, 0.04, 0.055), 0.01), STEEL));
  c = U(c, M(sdRoundBox(p - vec3(1.09, 0.035, 0.0), vec3(0.04, 0.03, 0.05), 0.01), STEEL));
  r = U(r, c);
  r = U(r, op(p - vec3(0.0, 0.115, 0.7), LILAC));
  r = U(r, op(rotY(p - vec3(0.0, 0.115, 0.95), 0.4), CORAL));
  r = U(r, op(p - vec3(0.66, 0.115, 0.0), LIME));
  r = U(r, op(rotY(p - vec3(0.86, 0.115, 0.0), -0.3), LILAC));
  return r;
}
