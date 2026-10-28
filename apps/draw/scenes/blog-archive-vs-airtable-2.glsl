#pragma frame {"size":[1200,720],"center":[0.0,0.85,0.0],"zoom":185,"bounds":[[-2.6,-0.1,-2.6],[2.6,2.6,2.6]]}

float bar(vec2 p, float x0, float y, float len) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, 0.02), 0.02);
}

vec2 record(vec3 p, float m) {
  vec2 c = M(sdBox(p, vec3(0.007, 0.09, 0.17)), m);
  c = U(c, M(sdRoundBox(p - vec3(0.0, 0.1, -0.07), vec3(0.007, 0.028, 0.055), 0.006), m));
  return c;
}

vec2 drawerRow(vec3 f, float i) {
  vec2 r = plate(f, vec2(0.44, 0.17), 0.02, 0.012, WHITE);
  vec3 t = f - vec3(0.0, 0.012, 0.0);
  r = U(r, M(pipe(t, vec3(-0.12, 0.025, -0.03), vec3(0.12, 0.025, -0.03), 0.018), STEEL + SOLID));
  r = U(r, M(pipe(t, vec3(-0.12, 0.0, -0.03), vec3(-0.12, 0.025, -0.03), 0.014), STEEL + SOLID));
  r = U(r, M(pipe(t, vec3(0.12, 0.0, -0.03), vec3(0.12, 0.025, -0.03), 0.014), STEEL + SOLID));
  r = U(r, M(extrude(max(sdRect(t.xz - vec2(0.0, 0.08), vec2(0.09, 0.04), 0.008), -sdRect(t.xz - vec2(0.0, 0.08), vec2(0.07, 0.024), 0.004)), t.y - 0.006, 0.006), STEEL));
  r = U(r, led(t - vec3(0.34, 0.0, 0.08), 0.026, i == 1.0 ? LIME : STEEL));
  return r;
}

vec2 rack(vec3 p, float open) {
  vec3 b = p - vec3(0.0, 0.95, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.85, 0.5), 0.03), WHITE);
  r = seam(r, -(abs(b.y) - 0.8));
  r = U(r, M(sdBox(p - vec3(0.0, 0.05, 0.0), vec3(0.44, 0.05, 0.44)), GRAPHITE));
  vec3 fz = faceZ(b, 0.5);
  for (int k = 0; k < 4; k++) {
    float i = float(k);
    vec3 d = fz - vec3(0.0, 0.0, 0.57 - i * 0.38);
    if (k == 1 && open > 0.5) {
      r = Sc(r, M(max(sdRect(d.xz, vec2(0.44, 0.17), 0.012), -(d.y + 0.5)), GRAPHITE));
    } else {
      r = U(r, drawerRow(d, i));
    }
  }
  vec3 fx = faceX(b, 0.5);
  r = inset(r, fx - vec3(0.0, 0.0, 0.25), vec2(0.36, 0.5), 0.03, 0.012, WHITE);
  r = vents(r, vec3(fx.x, fx.y, repC(fx.z - 0.45, 0.07, 4.0)), 5.0, 0.07, 0.04, 0.03, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.3, fx.y, abs(fx.z - 0.25) - 0.44), 0.022));
  vec3 lz = fx - vec3(0.0, 0.0, -0.55);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.024, LIME));
  r = vents(r, fx - vec3(0.0, 0.0, -0.66), 8.0, 0.07, 0.04, 0.03, GRAPHITE);
  if (open < 0.5) return r;
  vec3 dr = b - vec3(0.0, 0.19, 0.42);
  vec2 tray = M(max(sdBox(dr, vec3(0.42, 0.15, 0.36)), -sdBox(dr - vec3(0.0, 0.03, 0.0), vec3(0.4, 0.15, 0.34))), WHITE);
  r = U(r, tray);
  r = U(r, drawerRow(faceZ(dr, 0.36), 1.0));
  vec3 fq = dr - vec3(0.0, 0.0, 0.0);
  float j = repC(fq.z, 0.075, 8.0);
  float ji = repI(fq.z, 0.075, 8.0);
  float lift = ji == 5.0 ? 0.12 : 0.0;
  float m = ji == 5.0 ? LIME : ji == 2.0 ? LILAC : WHITE;
  vec3 rc = vec3(fq.x + (mod(ji, 2.0) * 0.08 - 0.04), fq.y + 0.02 - lift, j);
  r = U(r, record(vec3(rc.z, rc.y, rc.x) * vec3(1.0, 1.0, 0.7), m));
  return r;
}

float sdTri(vec2 p, float r) {
  const float k = 1.7320508;
  p.x = abs(p.x) - r;
  p.y = p.y + r / k;
  if (p.x + k * p.y > 0.0) p = vec2(p.x - k * p.y, -k * p.x - p.y) / 2.0;
  p.x -= clamp(p.x, -2.0 * r, 0.0);
  return -length(p) * sign(p.y);
}

vec2 chatFace(vec2 r, vec3 f) {
  r = decal(r, f, sdCircle(vec2(repC(f.x + 0.4, 0.06, 3.0), f.z - 0.4), 0.02), STEEL);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.33), vec2(0.48, 0.005), 0.005), STEEL);
  vec3 b1 = f - vec3(-0.15, 0.0, 0.18);
  r = U(r, plate(b1, vec2(0.3, 0.085), 0.05, 0.016, WHITE));
  vec3 t1 = b1 - vec3(0.0, 0.016, 0.0);
  r = decal(r, t1, bar(t1.xz, -0.22, 0.03, 0.42), STEEL);
  r = decal(r, t1, bar(t1.xz, -0.22, -0.03, 0.26), STEEL);
  vec3 b2 = f - vec3(0.17, 0.0, -0.04);
  r = U(r, plate(b2, vec2(0.28, 0.085), 0.05, 0.016, GRAPHITE));
  vec3 t2 = b2 - vec3(0.0, 0.016, 0.0);
  r = decal(r, t2, bar(t2.xz, -0.21, 0.03, 0.42), WHITE);
  r = decal(r, t2, bar(t2.xz, -0.21, -0.03, 0.24), WHITE);
  vec3 cp = f - vec3(-0.06, 0.0, -0.37);
  r = inset(r, cp, vec2(0.4, 0.07), 0.05, 0.012, PAPER);
  vec3 cf = cp + vec3(0.0, 0.012, 0.0);
  r = decal(r, cf, bar(cf.xz, -0.34, 0.0, 0.3), STEEL);
  vec3 rb = f - vec3(0.42, 0.0, -0.37);
  r = U(r, button(rb - vec3(0.0, 0.02, 0.0), 0.07, 0.03, LILAC));
  vec3 rt = rb - vec3(0.0, 0.05, 0.0);
  r = decal(r, rt, sdTri(vec2(rt.z, -rt.x - 0.006), 0.026) - 0.004, GRAPHITE);
  return r;
}

vec2 chatPanel(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(0.54, 0.05, 0.17), 0.02), STEEL);
  r = seam(r, p.y - 0.06);
  vec3 b = p - vec3(0.0, 0.64, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.58, 0.54, 0.045), 0.03), WHITE);
  body = chatFace(body, faceZ(b, 0.045));
  r = U(r, body);
  vec3 s = faceY(p, 0.1);
  r = U(r, screw(vec3(abs(s.x) - 0.44, s.y, s.z - 0.1), 0.02));
  return r;
}

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

vec2 gate(vec3 p) {
  vec3 b = p - vec3(0.0, 0.16, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.32, 0.16, 0.32), 0.03), WHITE);
  r = seam(r, b.y + 0.06);
  r = paint(r, b.y + 0.06, STEEL);
  vec3 t = faceY(b, 0.16);
  r = inset(r, t, vec2(0.24, 0.24), 0.03, 0.01, PAPER);
  r = U(r, screw(mirXZ(t) - vec3(0.27, 0.0, 0.27), 0.02));
  vec3 a = p - vec3(0.0, 0.32, 0.0);
  float arch = max(sdBox(a - vec3(0.0, 0.3, 0.0), vec3(0.24, 0.3, 0.09)), -sdBox(a - vec3(0.0, 0.22, 0.0), vec3(0.16, 0.22, 0.2)));
  vec2 ar = M(arch, WHITE);
  ar = seam(ar, -(abs(a.x) - 0.2));
  r = U(r, ar);
  vec3 lp = a - vec3(0.0, 0.6, 0.0);
  r = U(r, M(sdCylinder(lp - vec3(0.0, 0.03, 0.0), 0.06, 0.03), GRAPHITE + ROUND));
  r = U(r, M(max(sdSphere(lp - vec3(0.0, 0.06, 0.0), 0.07), -(lp.y - 0.06)), LIME + SOLID));
  vec3 fz = faceZ(a - vec3(0.0, 0.3, 0.0), 0.09);
  r = U(r, led(vec3(abs(fz.x) - 0.2, fz.y, fz.z - 0.2), 0.02, LIME));
  vec3 fx = faceX(b, 0.32);
  r = vents(r, fx - vec3(0.0, 0.0, 0.03), 5.0, 0.07, 0.07, 0.028, GRAPHITE);
  vec3 fz2 = faceZ(b, 0.32);
  r = vents(r, fz2 - vec3(0.0, 0.0, 0.03), 5.0, 0.07, 0.07, 0.028, GRAPHITE);
  return r;
}

vec2 wire(vec3 p, vec3 a, vec3 b, vec3 c, float m) {
  vec2 r = M(min(pipe(p, a, b, 0.03), pipe(p, b, c, 0.03)), m + SOLID);
  r = U(r, M(sdRoundBox(p - a, vec3(0.05, 0.045, 0.05), 0.012), STEEL));
  r = U(r, M(sdRoundBox(p - c, vec3(0.05, 0.045, 0.05), 0.012), STEEL));
  return r;
}

vec2 op(vec3 p, float m) {
  return M(sdRoundBox(p, vec3(0.06), 0.012), m);
}

vec2 map(vec3 p) {
  vec2 r = rack(p - vec3(-0.75, 0.0, -0.75), 0.0);
  r = U(r, gate(p - vec3(0.15, 0.0, 0.15)));
  r = U(r, wire(p, vec3(-0.25, 0.045, -0.22), vec3(-0.08, 0.045, -0.22), vec3(-0.08, 0.045, -0.15), STEEL));
  r = U(r, laptop(p - vec3(-1.25, 0.0, 1.2)));
  r = U(r, tablet(p - vec3(1.6, 0.0, 0.6)));
  r = U(r, chatPanel(p - vec3(1.1, 0.0, -1.4)));
  r = U(r, wire(p, vec3(-0.74, 0.045, 1.35), vec3(0.0, 0.045, 1.35), vec3(0.0, 0.045, 0.49), STEEL));
  r = U(r, wire(p, vec3(1.6, 0.045, 0.34), vec3(1.6, 0.045, 0.22), vec3(0.49, 0.045, 0.22), STEEL));
  r = U(r, wire(p, vec3(1.1, 0.045, -1.22), vec3(1.1, 0.045, -0.06), vec3(0.49, 0.045, -0.06), STEEL));
  r = U(r, op(p - vec3(0.0, 0.1, 0.9), WHITE));
  r = U(r, op(rotY(p - vec3(0.8, 0.1, 0.22), 0.3), LILAC));
  r = U(r, op(rotY(p - vec3(1.1, 0.1, -0.6), -0.2), WHITE));
  r = U(r, op(rotY(p - vec3(0.15, 0.52, 0.15), 0.4), LIME));
  return r;
}
