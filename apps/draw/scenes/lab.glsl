#pragma frame {"size":[1600,720],"center":[-0.1,0.87,0.1],"zoom":98,"bounds":[[-6.2,-0.2,-5.0],[6.5,4.2,7.5]]}

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

vec2 card(vec3 p) {
  vec2 c = M(sdBox(p, vec3(0.15, 0.018, 0.105)), WHITE);
  vec3 q = faceY(p, 0.018);
  c = decal(c, q, sdRect(q.xz - vec2(-0.085, 0.0), vec2(0.03, 0.08), 0.02), LIME);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, -0.035), vec2(0.07, 0.012), 0.012), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.03, 0.025), vec2(0.05, 0.012), 0.012), STEEL);
  return c;
}

vec2 chip(vec3 p) {
  vec2 c = M(sdRoundBox(p, vec3(0.14, 0.03, 0.14), 0.012), LILAC);
  vec3 q = faceY(p, 0.03);
  c = decal(c, q, sdRect(q.xz, vec2(0.065), 0.012), WHITE);
  c = decal(c, q, length(q.xz - vec2(0.09, 0.09)) - 0.018, WHITE);
  vec3 a = vec3(repC(p.x, 0.06, 4.0), p.y + 0.005, abs(p.z) - 0.16);
  c = U(c, M(sdBox(a, vec3(0.013, 0.012, 0.025)), STEEL));
  vec3 b = vec3(abs(p.x) - 0.16, p.y + 0.005, repC(p.z, 0.06, 4.0));
  c = U(c, M(sdBox(b, vec3(0.025, 0.012, 0.013)), STEEL));
  return c;
}

vec2 porthole(vec2 o, vec3 q, float r) {
  float d = length(q.xz);
  o = inset(o, q, vec2(r), r, 0.04, LILAC);
  vec3 g = q + vec3(0.0, 0.04, 0.0);
  o = decal(o, g, d - r * 0.55, WHITE);
  o = decal(o, g, d - r * 0.28, LILAC);
  o = decal(o, g, length(q.xz - vec2(-0.5, 0.5) * r) - r * 0.1, WHITE);
  o = U(o, M(extrude(abs(d - r - 0.035) - 0.04, q.y - 0.015, 0.02), STEEL + SOLID));
  vec3 s = polarY(q, 8.0);
  o = U(o, screw(s - vec3(r + 0.035, 0.035, 0.0), 0.014));
  return o;
}

vec2 gauge(vec3 q, float r, float a) {
  vec2 g = M(extrude(length(q.xz) - r, q.y - 0.02, 0.02), STEEL + SOLID);
  vec3 t = q - vec3(0.0, 0.04, 0.0);
  float d = length(t.xz);
  g = decal(g, t, d - r * 0.78, WHITE);
  g = decal(g, t, max(abs(d - r * 0.6) - 0.014, max(0.5 - atan(t.x, t.z), atan(t.x, t.z) - 2.2)), CORAL);
  g = decal(g, t, sdSeg(t.xz, vec2(0.0), vec2(sin(a), cos(a)) * r * 0.62) - 0.011, GRAPHITE);
  g = decal(g, t, d - 0.02, GRAPHITE);
  return g;
}

float railing(vec3 q, float len, float h) {
  float n = floor(len / 0.45) + 1.0;
  float sp = len / (n - 1.0);
  float d = sdBox(vec3(repL(q.x, sp, n), q.y - h * 0.5, q.z), vec3(0.016, h * 0.5, 0.016));
  d = min(d, sdBox(q - vec3(len * 0.5, h, 0.0), vec3(len * 0.5 + 0.016, 0.02, 0.02)));
  d = min(d, sdBox(q - vec3(len * 0.5, h * 0.5, 0.0), vec3(len * 0.5, 0.012, 0.012)));
  return d;
}

float warn(vec3 p, float w) { return abs(mod(p.x - p.z + p.y, 2.0 * w) - w) - w * 0.5; }

vec2 rig(vec3 p) {
  vec3 b = p - vec3(0.0, 0.08, 0.0);
  vec2 r = M(sdBox(b, vec3(1.75, 0.08, 1.95)), WHITE);
  r = paint(r, max(warn(p, 0.12), max(b.y - 0.06, -max(abs(b.x) - 1.73, abs(b.z) - 1.93))), CORAL);
  r = stripes(stripes(r, p.x + 0.05, 0.5), p.z + 0.05, 0.5);

  vec3 c = p - vec3(0.0, 1.16, 0.0);
  vec2 body = M(sdRoundBox(c, vec3(1.3, 1.0, 1.5), 0.2), WHITE);
  body = paint(body, p.y - 0.46, STEEL);
  body = seam(body, p.y - 1.92);

  vec3 fx = faceX(p, 1.3);
  body = porthole(body, fx - vec3(-0.55, 0.0, 1.32), 0.25);
  body = porthole(body, fx - vec3(0.35, 0.0, 1.32), 0.25);
  body = vents(body, vec3(fx.x - 1.0, fx.y, fx.z - 1.35), 4.0, 0.07, 0.34, 0.035, GRAPHITE);
  body = vents(body, vec3(fx.x + 0.6, fx.y, fx.z - 0.72), 5.0, 0.07, 0.18, 0.03, GRAPHITE);
  vec3 hx = fx - vec3(0.3, 0.0, 0.64);
  body = inset(body, hx, vec2(0.27, 0.2), 0.03, 0.08, GRAPHITE);
  body = U(body, M(extrude(max(sdRect(hx.xz, vec2(0.33, 0.26), 0.05), -sdRect(hx.xz, vec2(0.27, 0.2), 0.03)), hx.y - 0.02, 0.02), CORAL));
  vec3 cs = vec3(abs(fx.x) - 1.38, fx.y, abs(fx.z - 1.16) - 0.88);
  body = U(body, screw(cs, 0.022));

  vec3 fz = faceZ(p, 1.5);
  body = porthole(body, fz - vec3(0.45, 0.0, 1.32), 0.36);
  body = U(body, gauge(fz - vec3(-0.5, 0.0, 1.62), 0.12, -0.6));
  body = U(body, gauge(fz - vec3(-0.2, 0.0, 1.62), 0.12, 0.9));
  body = U(body, gauge(fz - vec3(-0.35, 0.0, 1.3), 0.09, 0.2));
  vec3 lr = vec3(repC(fz.x - 0.45, 0.12, 6.0), fz.y, fz.z - 1.83);
  float li = repI(fz.x - 0.45, 0.12, 6.0);
  body = U(body, led(lr, 0.028, li == 1.0 || li == 4.0 ? LILAC : LIME));
  vec3 hz = fz - vec3(-0.5, 0.0, 0.62);
  body = inset(body, hz, vec2(0.3, 0.2), 0.03, 0.08, GRAPHITE);
  body = U(body, M(extrude(max(sdRect(hz.xz, vec2(0.36, 0.26), 0.05), -sdRect(hz.xz, vec2(0.3, 0.2), 0.03)), hz.y - 0.02, 0.02), CORAL));
  vec3 zs = vec3(abs(fz.x) - 1.2, fz.y, abs(fz.z - 1.16) - 0.88);
  body = U(body, screw(zs, 0.022));
  r = U(r, body);

  vec3 f = p - vec3(-0.12, 2.36, -0.15);
  vec2 fins = M(sdBox(f - vec3(0.0, -0.17, 0.0), vec3(0.95, 0.04, 1.17)), STEEL);
  fins = U(fins, M(sdBox(vec3(repC(f.x, 0.16, 11.0), f.y, f.z), vec3(0.028, 0.2, 1.1)), WHITE));
  fins = U(fins, M(sdBox(f - vec3(0.0, 0.0, 1.13), vec3(0.86, 0.13, 0.03)), WHITE));
  r = U(r, fins);

  vec3 rq = vec3(p.x + 1.12, p.y - 2.16, p.z - 1.36);
  float rail = railing(rq, 2.24, 0.42);
  rail = min(rail, railing(vec3(1.36 - p.z, p.y - 2.16, p.x - 1.12), 2.72, 0.42));
  r = U(r, M(rail, GRAPHITE));
  vec3 lq = vec3(p.x + 0.98, p.y - 0.16, p.z - 1.6);
  r = U(r, M(ladder(lq, 2.42, 0.13, 0.15, 0.014), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(lq.x) - 0.13, lq.y - 1.6, lq.z + 0.06), vec3(0.014, 0.02, 0.06)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(lq.x) - 0.13, lq.y - 0.4, lq.z + 0.06), vec3(0.014, 0.02, 0.06)), GRAPHITE));

  vec3 bc = p - vec3(1.12, 2.16, 1.36);
  r = U(r, M(sdCylinder(bc - vec3(0.0, 0.5, 0.0), 0.05, 0.04), GRAPHITE + ROUND));
  r = U(r, M(max(sdSphere(bc - vec3(0.0, 0.54, 0.0), 0.07), -(bc.y - 0.54)), LIME + SOLID));
  return r;
}

vec2 crane(vec3 p) {
  vec3 c = p - vec3(1.55, 0.0, -1.25);
  vec2 r = M(sdBox(c - vec3(0.0, 1.86, 0.0), vec3(0.09, 1.7, 0.09)), STEEL);
  r = U(r, M(sdBox(c - vec3(0.0, 0.2, 0.0), vec3(0.16, 0.04, 0.16)), GRAPHITE));
  r = U(r, M(sdCylinder(c - vec3(0.0, 3.6, 0.0), 0.14, 0.06), GRAPHITE + ROUND));
  vec3 bm = c - vec3(-0.8, 3.74, 0.0);
  vec2 boom = M(sdBox(bm, vec3(1.5, 0.075, 0.075)), WHITE);
  boom = paint(boom, max(warn(p, 0.08), bm.x + 1.3), CORAL);
  boom = stripes(boom, bm.x * 1.0 + 0.0, 0.18);
  r = U(r, boom);
  r = U(r, M(sdBox(c - vec3(0.62, 3.67, 0.0), vec3(0.2, 0.16, 0.13)), CORAL));
  r = U(r, M(pipe(c, vec3(0.0, 3.0, 0.0), vec3(-0.8, 3.68, 0.0), 0.03), GRAPHITE + SOLID));
  vec3 t = c - vec3(-1.6, 3.6, 0.0);
  r = U(r, M(sdBox(t, vec3(0.1, 0.06, 0.1)), GRAPHITE));
  r = U(r, M(sdCylinder(t - vec3(0.0, -0.33, 0.0), 0.012, 0.3), GRAPHITE + SOLID));
  vec3 h = t - vec3(0.0, -0.68, 0.0);
  r = U(r, M(sdTorus(rotX(h, PI * 0.5), 0.05, 0.014), GRAPHITE + SOLID));
  vec3 ld = h - vec3(0.0, -0.3, 0.0);
  vec2 load = M(sdCylinder(ld, 0.17, 0.2), LILAC + ROUND);
  load = U(load, M(sdCylinder(ld - vec3(0.0, 0.2, 0.0), 0.19, 0.035), STEEL + ROUND));
  load = U(load, M(sdCylinder(ld + vec3(0.0, 0.2, 0.0), 0.19, 0.035), STEEL + ROUND));
  load = seam(load, abs(ld.y) - 0.07);
  load = U(load, M(pipe(ld, vec3(-0.1, 0.24, 0.0), vec3(0.0, 0.33, 0.0), 0.01), GRAPHITE + SOLID));
  load = U(load, M(pipe(ld, vec3(0.1, 0.24, 0.0), vec3(0.0, 0.33, 0.0), 0.01), GRAPHITE + SOLID));
  r = U(r, load);
  return r;
}

vec2 desk(vec3 p) {
  float a = 0.5;
  vec3 b = p - vec3(0.0, 0.4, 0.0);
  vec3 s = rotX(p - vec3(0.0, 0.66, 0.0), a);
  vec2 r = M(max(sdBox(b, vec3(0.62, 0.4, 0.27)), s.y), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  r = seam(r, -s.y - 0.04);
  vec3 fz = faceZ(p, 0.27);
  r = vents(r, fz - vec3(-0.35, 0.0, 0.3), 5.0, 0.07, 0.16, 0.03, GRAPHITE);
  r = inset(r, fz - vec3(0.32, 0.0, 0.3), vec2(0.18, 0.12), 0.02, 0.02, STEEL);
  r = inset(r, s - vec3(-0.3, 0.0, -0.02), vec2(0.17, 0.1), 0.02, 0.015, GRAPHITE);
  vec3 sc = s - vec3(-0.3, -0.015, -0.02);
  float bx = repC(sc.x, 0.06, 5.0);
  float bi = repI(sc.x, 0.06, 5.0);
  float bh = 0.025 + 0.012 * mod(bi * 3.0, 5.0);
  r = decal(r, sc, sdRect(vec2(bx, sc.z - 0.07 + bh), vec2(0.018, bh), 0.004), LIME);
  vec3 bt = vec3(repC(s.x - 0.2, 0.11, 3.0), s.y, s.z + 0.04);
  float bti = repI(s.x - 0.2, 0.11, 3.0);
  r = U(r, button(bt, 0.035, 0.018, bti == 0.0 ? CORAL : bti == 1.0 ? LIME : WHITE));
  r = U(r, button(vec3(repC(s.x - 0.2, 0.11, 3.0), s.y, s.z - 0.1), 0.025, 0.014, STEEL));
  r = U(r, led(s - vec3(0.5, 0.0, -0.08), 0.022, LIME));
  r = U(r, led(s - vec3(0.5, 0.0, 0.04), 0.022, LILAC));
  vec3 m = vec3(abs(p.x) - 0.3, p.y - 1.08, p.z + 0.17);
  r = U(r, M(sdBox(vec3(m.x, p.y - 0.85, m.z), vec3(0.025, 0.08, 0.025)), GRAPHITE));
  vec2 mon = M(sdRoundBox(m, vec3(0.27, 0.17, 0.03), 0.015), WHITE);
  vec3 mq = faceZ(vec3(p.x - 0.3 * sign(p.x), m.y, m.z), 0.03);
  mon = inset(mon, mq, vec2(0.23, 0.13), 0.015, 0.01, GRAPHITE);
  vec3 w = mq + vec3(0.0, 0.01, 0.0);
  if (p.x < 0.0) {
    mon = decal(mon, w, abs(w.z - 0.05 * sin(w.x * 26.0)) - 0.012, LILAC);
    mon = decal(mon, w, max(abs(w.z + 0.07) - 0.01, abs(w.x) - 0.18), STEEL);
  } else {
    float gx = repC(w.x, 0.07, 6.0);
    float gi = repI(w.x, 0.07, 6.0);
    float gh = 0.03 + 0.017 * mod(gi * 2.0 + 1.0, 5.0);
    mon = decal(mon, w, sdRect(vec2(gx, w.z + 0.1 - gh), vec2(0.02, gh), 0.004), LIME);
  }
  r = U(r, mon);
  return r;
}

vec2 drum(vec3 p, float lit) {
  vec2 d = M(sdRoundCylinder(p, 0.6, 0.3, 0.07), WHITE + ROUND);
  d = seam(d, abs(p.y) - 0.18);
  vec3 q = faceX(rotY(p, -PI * 0.25), 0.6);
  d = vents(d, q - vec3(-0.12, 0.0, 0.0), 4.0, 0.06, 0.15, 0.024, GRAPHITE);
  d = U(d, led(q - vec3(0.2, 0.0, 0.03), 0.04, lit > 0.5 ? LIME : STEEL));
  d = U(d, screw(q - vec3(0.2, 0.0, -0.09), 0.022));
  return d;
}

vec2 silo(vec3 p, float n, float k) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.74, 0.05), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.1 + n * 0.33, 0.0), 0.55, n * 0.33), GRAPHITE + ROUND));
  float y = p.y - 0.43;
  float i = repLI(y, 0.66, n);
  r = U(r, drum(vec3(p.x, repL(y, 0.66, n), p.z), mod(i + k, 3.0) == 0.0 ? 1.0 : 0.0));
  vec3 t = p - vec3(0.0, 0.1 + n * 0.66, 0.0);
  r = U(r, M(sdCone(t - vec3(0.0, 0.12, 0.0), 0.12, 0.6, 0.28), WHITE + SOLID));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.27, 0.0), 0.16, 0.04), STEEL + ROUND));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.33, 0.0), 0.05, 0.03), GRAPHITE + ROUND));
  return r;
}

vec2 piping(vec3 p) {
  float y = 1.77;
  vec2 r = M(pipe(p, vec3(-3.7, y, 5.0), vec3(-3.7, y, 0.9), 0.085), STEEL + SOLID);
  r = U(r, M(pipe(p, vec3(-3.7, y, 0.9), vec3(-1.25, y, 0.9), 0.085), STEEL + SOLID));
  vec3 st = vec3(p.x + 3.85, p.y - y, repC(p.z - 3.6, 1.4, 3.0));
  r = U(r, M(sdCapsule(st, vec3(-0.15, 0.0, 0.0), vec3(0.15, 0.0, 0.0), 0.06), STEEL + SOLID));
  r = U(r, M(sdCylinder(rotZ(st - vec3(0.08, 0.0, 0.0), PI * 0.5), 0.085, 0.02), STEEL + SOLID));
  vec3 fl = vec3(p.x + 3.7, p.y - y, p.z - 2.9);
  r = U(r, M(sdCylinder(rotX(fl, PI * 0.5), 0.12, 0.025), STEEL + SOLID));
  vec3 f2 = vec3(repC(p.x + 2.5, 1.2, 2.0), p.y - y, p.z - 0.9);
  r = U(r, M(sdCylinder(rotZ(f2, PI * 0.5), 0.12, 0.025), STEEL + SOLID));
  r = U(r, M(sdSphere(p - vec3(-3.7, y, 0.9), 0.12), STEEL + SOLID));
  vec3 v = p - vec3(-3.7, y + 0.1, 4.3);
  r = U(r, M(sdCylinder(v - vec3(0.0, 0.1, 0.0), 0.02, 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdTorus(v - vec3(0.0, 0.2, 0.0), 0.11, 0.022), CORAL + SOLID));
  r = U(r, M(sdBox(vec3(abs(v.x) - 0.0, v.y - 0.2, v.z), vec3(0.11, 0.012, 0.012)), CORAL + SOLID));
  r = U(r, M(sdCylinder(v - vec3(0.0, -0.02, 0.0), 0.11, 0.1), CORAL + ROUND));
  vec3 v2 = p - vec3(-2.2, y + 0.1, 0.9);
  r = U(r, M(sdCylinder(v2 - vec3(0.0, 0.1, 0.0), 0.02, 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdTorus(v2 - vec3(0.0, 0.2, 0.0), 0.1, 0.02), CORAL + SOLID));
  r = U(r, M(sdBox(v2 - vec3(0.0, 0.2, 0.0), vec3(0.1, 0.012, 0.012)), CORAL + SOLID));
  r = U(r, M(sdCylinder(v2 - vec3(0.0, -0.02, 0.0), 0.11, 0.1), CORAL + ROUND));
  vec3 s1 = vec3(p.x + 3.7, p.y, abs(p.z - 2.25) - 0.65);
  vec3 s2 = vec3(p.x + 2.75, p.y, p.z - 0.9);
  float ps = min(sdBox(s1 - vec3(0.0, 0.85, 0.0), vec3(0.03, 0.85, 0.03)), sdBox(s2 - vec3(0.0, 0.85, 0.0), vec3(0.03, 0.85, 0.03)));
  ps = min(ps, sdBox(s1 - vec3(0.0, y - 0.1, 0.0), vec3(0.14, 0.02, 0.05)));
  ps = min(ps, sdBox(s2 - vec3(0.0, y - 0.1, 0.0), vec3(0.05, 0.02, 0.14)));
  ps = min(ps, sdBox(s1 - vec3(0.0, 0.02, 0.0), vec3(0.12, 0.02, 0.12)));
  ps = min(ps, sdBox(s2 - vec3(0.0, 0.02, 0.0), vec3(0.12, 0.02, 0.12)));
  r = U(r, M(ps, GRAPHITE));
  return r;
}

vec2 pallet(vec3 p, float n) {
  vec2 r = M(sdBox(vec3(p.x, p.y - 0.135, repC(p.z, 0.19, 5.0)), vec3(0.5, 0.018, 0.075)), PAPER);
  vec3 bq = vec3(repC(p.x, 0.44, 3.0), p.y - 0.075, repC(p.z, 0.32, 3.0));
  r = U(r, M(sdBox(bq, vec3(0.06, 0.042, 0.06)), PAPER));
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.016, repC(p.z, 0.32, 3.0)), vec3(0.5, 0.016, 0.065)), PAPER));
  float h = n * 0.035;
  vec3 s = p - vec3(0.0, 0.153 + h, 0.0);
  vec2 st = M(sdBox(s, vec3(0.44, h, 0.34)), WHITE);
  st = stripes(st, s.y + h, 0.035);
  float li = floor((s.y + h) / 0.07);
  vec3 tq = vec3(s.x - (-0.28 + 0.14 * mod(li, 4.0)), s.y - (li + 0.5) * 0.07 + h, s.z - 0.34);
  vec2 tab = M(sdBox(tq, vec3(0.06, 0.025, 0.02)), mod(li, 3.0) == 0.0 ? LIME : mod(li, 3.0) == 1.0 ? LILAC : SKY);
  if (s.y + h > 0.0 && s.y < h) st = U(st, tab);
  st = paint(st, max(abs(abs(s.x) - 0.24) - 0.03, -s.y - h + 0.001), CORAL);
  r = U(r, st);
  vec3 cq = faceY(s, h);
  r = U(r, card(vec3(cq.x - 0.1, cq.y - 0.018, cq.z + 0.05)));
  r = U(r, card(rotY(vec3(cq.x + 0.17, cq.y - 0.054, cq.z - 0.1), 0.3)));
  return r;
}

vec2 arm(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.22, 0.05), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.22, 0.0), 0.14, 0.13), WHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.37, 0.0), 0.17, 0.03), CORAL + ROUND));
  vec3 a = vec3(0.0, 0.48, 0.0), b = vec3(-0.15, 1.35, 0.15), c = vec3(0.7, 1.0, -0.55);
  r = U(r, M(sdCapsule(p, a, b, 0.065), WHITE + SOLID));
  r = U(r, M(sdCapsule(p, b, c, 0.055), WHITE + SOLID));
  r = U(r, M(sdSphere(p - a, 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - b, 0.085), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - c, 0.07), LIME + SOLID));
  vec3 g = p - c;
  r = U(r, M(sdBox(g - vec3(0.0, -0.1, 0.0), vec3(0.08, 0.025, 0.05)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(g.x) - 0.07, g.y - 0.17 * -1.0, g.z), vec3(0.012, 0.07, 0.04)), GRAPHITE));
  r = U(r, card(rotZ(g - vec3(0.0, -0.25, 0.0), 0.08)));
  return r;
}

vec2 press(vec3 p) {
  vec3 c = vec3(p.x, p.y - 0.95, abs(p.z) - 0.5);
  vec2 r = M(sdBox(c, vec3(0.19, 0.95, 0.1)), WHITE);
  r = paint(r, p.y - 0.3, STEEL);
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.04, abs(p.z) - 0.5), vec3(0.28, 0.04, 0.17)), GRAPHITE));
  vec3 cq = faceZ(p, 0.6);
  r = vents(r, cq - vec3(0.0, 0.0, 1.45), 3.0, 0.07, 0.2, 0.03, GRAPHITE);
  vec3 bx = p - vec3(0.0, 1.0, 0.68);
  vec2 box = M(sdBox(bx, vec3(0.15, 0.17, 0.08)), WHITE);
  vec3 bf = faceZ(bx, 0.08);
  r = U(r, box);
  r = U(r, button(bf - vec3(-0.05, 0.0, 0.07), 0.04, 0.02, CORAL));
  r = U(r, button(bf - vec3(0.06, 0.0, 0.07), 0.035, 0.02, LIME));
  r = U(r, led(bf - vec3(0.0, 0.0, -0.08), 0.025, LIME));
  vec3 h = p - vec3(0.0, 2.0, 0.0);
  vec2 head = M(sdRoundBox(h, vec3(0.3, 0.24, 0.68), 0.03), WHITE);
  head = paint(head, max(warn(p, 0.09), h.y + 0.08), CORAL);
  head = seam(head, h.y + 0.08);
  r = U(r, head);
  vec3 hf = faceX(h, 0.3);
  r = U(r, gauge(hf - vec3(0.4, 0.0, 0.07), 0.08, 1.2));
  r = U(r, screw(vec3(abs(hf.x) - 0.6, hf.y, hf.z - 0.12), 0.02));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.5, 0.0), 0.15, 0.27), STEEL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.22, 0.0), 0.18, 0.03), STEEL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.0, 0.0), 0.06, 0.22), WHITE + ROUND));
  r = U(r, M(sdBox(p - vec3(0.0, 0.73, 0.0), vec3(0.2, 0.06, 0.2)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.64, 0.0), vec3(0.16, 0.03, 0.16)), STEEL));
  r = U(r, M(sdCylinder(vec3(p.x, p.y - 1.25, abs(p.z) - 0.3), 0.025, 0.55), STEEL + ROUND));
  vec3 bc = p - vec3(0.15, 2.24, 0.5);
  r = U(r, M(sdCylinder(bc - vec3(0.0, 0.03, 0.0), 0.05, 0.03), GRAPHITE + ROUND));
  r = U(r, M(max(sdSphere(bc - vec3(0.0, 0.06, 0.0), 0.07), -(bc.y - 0.06)), CORAL + SOLID));
  return r;
}

vec2 turntable(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.2, 0.0), 0.36, 0.2), WHITE + ROUND);
  r = paint(r, p.y - 0.08, STEEL);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.425, 0.0), 0.33, 0.025), STEEL + ROUND));
  r = seam(r, -(length(p.xz) - 0.22));
  return r;
}

vec2 cabinet(vec3 p, float k) {
  vec3 c = p - vec3(0.0, 1.06, 0.0);
  vec2 r = M(sdBox(c, vec3(0.38, 1.06, 0.45)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.04, 0.0), vec3(0.4, 0.04, 0.47)), GRAPHITE));
  vec3 f = faceZ(p, 0.45);
  float u = repC(f.z - 1.2, 0.25, 7.0);
  float ui = repI(f.z - 1.2, 0.25, 7.0);
  bool slot = k == 1.0 && ui == 0.0;
  vec3 uq = vec3(f.x, f.y, u);
  if (!slot) {
    r = U(r, M(extrude(sdRect(uq.xz, vec2(0.33, 0.105), 0.015), uq.y - 0.012, 0.012), WHITE));
    vec3 vq = uq - vec3(-0.1, 0.024, 0.0);
    r = vents(r, vq, 5.0, 0.05, 0.12, 0.022, GRAPHITE);
    float on = mod(ui * 2.0 + k * 3.0, 5.0);
    r = U(r, led(uq - vec3(0.2, 0.024, 0.03), 0.024, on < 3.0 ? LIME : STEEL));
    r = U(r, led(uq - vec3(0.27, 0.024, 0.03), 0.024, on < 1.5 ? LIME : STEEL));
    r = U(r, screw(vec3(abs(uq.x) - 0.3, uq.y - 0.024, uq.z + 0.06), 0.014));
  } else {
    r = inset(r, uq - vec3(0.0, 0.0, -0.05), vec2(0.28, 0.1), 0.02, 0.1, GRAPHITE);
  }
  vec3 t = faceY(p, 2.12);
  r = vents(r, t - vec3(0.0, 0.0, 0.08), 4.0, 0.13, 0.32, 0.035, GRAPHITE);
  vec3 sx = faceX(p, 0.38);
  r = vents(r, vec3(sx.x, sx.y, sx.z - 1.8), 6.0, 0.12, 0.16, 0.04, GRAPHITE);
  return r;
}

vec2 rack(vec3 p) {
  vec2 r = cabinet(vec3(repC(p.x, 0.8, 3.0), p.y, p.z), repI(p.x, 0.8, 3.0));
  vec3 t = p - vec3(0.0, 2.2, -0.15);
  r = U(r, M(max(sdBox(t, vec3(1.2, 0.08, 0.14)), -sdBox(t - vec3(0.0, 0.05, 0.0), vec3(1.17, 0.08, 0.11))), STEEL));
  vec3 cb = vec3(t.x, t.y - 0.02, repC(t.z, 0.07, 3.0));
  r = U(r, M(sdCylinder(rotZ(cb, PI * 0.5), 0.028, 1.17), LILAC + SOLID));
  vec3 cd = vec3(repC(p.x, 0.8, 3.0), p.y - 2.18, p.z + 0.15);
  r = U(r, M(pipe(cd, vec3(0.15, -0.06, 0.0), vec3(0.15, 0.0, 0.0), 0.028), GRAPHITE + SOLID));
  return r;
}

vec2 gate(vec3 p) {
  vec3 g = vec3(abs(p.x) - 0.4, p.y, p.z);
  vec2 r = M(sdBox(g - vec3(0.0, 0.6, 0.0), vec3(0.06, 0.6, 0.09)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  r = U(r, M(sdBox(g - vec3(0.0, 0.02, 0.0), vec3(0.1, 0.02, 0.13)), GRAPHITE));
  vec3 h = p - vec3(0.0, 1.27, 0.0);
  vec2 head = M(sdRoundBox(h, vec3(0.52, 0.11, 0.14), 0.03), WHITE);
  vec3 hf = faceZ(h, 0.14);
  head = U(head, led(hf - vec3(-0.3, 0.0, 0.0), 0.03, LIME));
  head = U(head, led(hf - vec3(-0.2, 0.0, 0.0), 0.03, LIME));
  head = vents(head, hf - vec3(0.2, 0.0, 0.0), 4.0, 0.06, 0.1, 0.025, GRAPHITE);
  r = U(r, head);
  vec3 e = p - vec3(0.0, 1.13, 0.0);
  r = U(r, M(sdBox(e, vec3(0.3, 0.03, 0.05)), GRAPHITE));
  r = U(r, M(max(sdBox(e - vec3(0.0, -0.32, 0.0), vec3(0.24, 0.29, 0.004)), abs(mod(p.x + 0.03, 0.08) - 0.04) - 0.012), LIME + SOLID));
  return r;
}

vec2 cone(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.015, 0.0), vec3(0.13, 0.015, 0.13)), GRAPHITE);
  vec2 c = M(sdCone(p - vec3(0.0, 0.2, 0.0), 0.18, 0.1, 0.025), CORAL + SOLID);
  c = paint(c, abs(p.y - 0.22) - 0.035, WHITE + SOLID);
  return U(r, c);
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 1.45, 0.0), vec3(1.8, 1.45, 2.0), r.x)) r = U(r, rig(p));
  if (near(p - vec3(0.95, 2.0, -1.25), vec3(1.4, 2.0, 0.25), r.x)) r = U(r, crane(p));
  if (near(p - vec3(0.7, 0.6, 2.55), vec3(0.7, 0.6, 0.35), r.x)) r = U(r, desk(p - vec3(0.7, 0.0, 2.55)));
  if (near(p - vec3(-4.6, 1.5, 3.6), vec3(0.8, 1.5, 2.2), r.x)) {
    r = U(r, silo(p - vec3(-4.6, 0.0, 2.2), 4.0, 0.0));
    r = U(r, silo(p - vec3(-4.6, 0.0, 3.6), 3.0, 1.0));
    r = U(r, silo(p - vec3(-4.6, 0.0, 5.0), 3.0, 2.0));
  }
  if (near(p - vec3(-2.6, 1.0, 3.0), vec3(1.4, 1.1, 2.2), r.x)) r = U(r, piping(p));
  vec3 c0 = vec3(5.6 - p.z, p.y, p.x + 0.5);
  if (near(c0 - vec3(2.05, 0.3, 0.0), vec3(2.1, 0.3, 0.3), r.x)) {
    r = U(r, conveyor(c0, 4.1, 0.24, 0.45));
    r = U(r, card(vec3(repL(c0.x - 0.45, 0.6, 6.0), c0.y - 0.458, c0.z)));
  }
  if (near(p - vec3(-2.3, 0.4, 6.2), vec3(0.55, 0.45, 0.45), r.x)) r = U(r, pallet(p - vec3(-2.3, 0.0, 6.2), 7.0));
  if (near(p - vec3(-2.65, 0.4, 4.4), vec3(0.55, 0.45, 0.45), r.x)) r = U(r, pallet(p - vec3(-2.65, 0.0, 4.4), 5.0));
  if (near(p - vec3(-0.9, 0.75, 5.5), vec3(0.65, 0.8, 0.65), r.x)) r = U(r, arm(p - vec3(-1.2, 0.0, 5.75)));
  vec3 c1 = p - vec3(1.3, 0.0, 0.3);
  if (near(c1 - vec3(1.45, 0.3, 0.0), vec3(1.5, 0.3, 0.3), r.x)) {
    r = U(r, conveyor(c1, 2.9, 0.24, 0.45));
    r = U(r, M(sdRoundBox(c1 - vec3(0.5, 0.55, 0.0), vec3(0.17, 0.1, 0.15), 0.08), LILAC + SOLID));
    r = U(r, chip(vec3(repL(c1.x - 1.45, 0.62, 3.0), c1.y - 0.47, c1.z)));
  }
  if (near(p - vec3(2.75, 1.15, 0.3), vec3(0.4, 1.15, 0.8), r.x)) r = U(r, press(p - vec3(2.75, 0.0, 0.3)));
  vec3 tt = p - vec3(4.55, 0.0, 0.3);
  if (near(tt - vec3(0.0, 0.3, 0.0), vec3(0.4, 0.3, 0.4), r.x)) {
    r = U(r, turntable(tt));
    r = U(r, chip(rotY(tt - vec3(0.0, 0.48, 0.0), 0.5)));
  }
  vec3 c2 = vec3(-0.06 - p.z, p.y, p.x - 4.55);
  if (near(c2 - vec3(1.75, 0.3, 0.0), vec3(1.8, 0.3, 0.3), r.x)) {
    r = U(r, conveyor(c2, 3.49, 0.24, 0.45));
    r = U(r, chip(vec3(repL(c2.x - 0.3, 0.72, 4.0), c2.y - 0.47, c2.z)));
  }
  if (near(p - vec3(4.55, 0.7, -1.8), vec3(0.6, 0.75, 0.25), r.x)) r = U(r, gate(p - vec3(4.55, 0.0, -1.8)));
  if (near(p - vec3(4.55, 1.15, -4.0), vec3(1.3, 1.2, 0.5), r.x)) r = U(r, rack(p - vec3(4.55, 0.0, -4.0)));
  if (near(p - vec3(0.1, 0.2, 6.0), vec3(0.6, 0.25, 0.5), r.x)) {
    r = U(r, cone(p - vec3(-0.15, 0.0, 6.2)));
    r = U(r, cone(p - vec3(0.35, 0.0, 5.85)));
  }
  return r;
}
