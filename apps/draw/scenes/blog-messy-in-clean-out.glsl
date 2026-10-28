#pragma frame {"size":[1400,760],"center":[0.0,0.6,0.0],"zoom":170,"bounds":[[-3.4,-0.1,-1.6],[3.4,2.2,1.6]]}

const float B = 0.12;
const float H = 0.5;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 conveyor(vec3 p, float len, float w, float h) {
  vec3 q = p - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(q.x, q.y - h + 0.045, abs(q.z) - w), vec3(len * 0.5, 0.045, 0.022)), WHITE);
  vec3 ro = vec3(repC(q.x, 0.11, floor(len / 0.11)), q.y - h + 0.04, q.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.03, w), STEEL + SOLID));
  vec3 lq = vec3(repC(q.x, len - 0.12, 2.0), q.y - (h - 0.09) * 0.5, abs(q.z) - w);
  r = U(r, M(sdBox(lq, vec3(0.022, (h - 0.09) * 0.5, 0.022)), GRAPHITE));
  r = U(r, M(sdBox(vec3(repC(q.x, len - 0.12, 2.0), q.y - 0.14, q.z), vec3(0.016, 0.016, w)), GRAPHITE));
  return r;
}

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.16, 0.014, 0.11)), m);
  return seam(e, p.z + 0.11 - (0.16 - abs(p.x)) * 0.62);
}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.12, 0.006, 0.16)), m);
  vec3 q = faceY(p, 0.006);
  float b = m == WHITE ? STEEL : WHITE;
  s = decal(s, q, sdRect(q.xz - vec2(0.0, -0.08), vec2(0.07, 0.01), 0.01), b);
  s = decal(s, q, sdRect(q.xz - vec2(-0.02, -0.03), vec2(0.05, 0.01), 0.01), b);
  return decal(s, q, sdRect(q.xz - vec2(0.0, 0.07), vec2(0.07, 0.04), 0.01), b);
}

vec2 folder(vec3 p, float m) {
  vec2 f = M(sdBox(p, vec3(0.17, 0.022, 0.12)), m);
  f = U(f, M(sdBox(p - vec3(-0.08, 0.0, -0.13), vec3(0.06, 0.012, 0.02)), m));
  return seam(f, p.y - 0.006);
}

vec2 bubble(vec3 p, float m) {
  float d2 = min(sdRect(p.xz, vec2(0.14, 0.09), 0.07), sdSeg(p.xz, vec2(-0.05, 0.06), vec2(-0.11, 0.15)) - 0.025);
  vec2 b = M(extrude(d2, p.y, 0.018), m);
  vec3 q = faceY(p, 0.018);
  return decal(b, q, length(vec2(repC(q.x, 0.07, 3.0), q.z)) - 0.017, m == WHITE ? GRAPHITE : WHITE);
}

vec2 card(vec3 p) {
  vec2 c = M(sdRoundBox(p, vec3(0.13, 0.02, 0.17), 0.01), WHITE);
  vec3 q = faceY(p, 0.02);
  c = decal(c, q, sdRect(q.xz - vec2(-0.07, 0.0), vec2(0.025, 0.13), 0.02), LIME);
  c = decal(c, q, bar(q.xz, -0.02, -0.08, 0.12, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, -0.02, 0.08, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, 0.04, 0.12, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, 0.1, 0.06, 0.014), STEEL);
  return c;
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(0.0, B * 0.5, -0.08);
  vec2 r = M(sdRoundBox(b, vec3(3.0, B * 0.5, 0.75), 0.03), WHITE);
  r = seam(r, p.y - 0.04);
  r = paint(r, p.y - 0.04, STEEL);
  vec3 fz = faceZ(b, 0.75);
  r = vents(r, fz - vec3(-1.8, 0.0, -0.015), 8.0, 0.07, 0.035, 0.026, GRAPHITE);
  vec3 cs = mirXZ(faceY(b, B * 0.5));
  r = U(r, screw(vec3(cs.x - 2.88, cs.y, cs.z - 0.63), 0.024));
  return r;
}

vec2 mailbox(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.35, 0.0), vec3(0.03, 0.35, 0.03)), GRAPHITE);
  vec3 b = p - vec3(0.0, 0.84, 0.0);
  float body = min(sdBox(b - vec3(0.0, -0.05, 0.0), vec3(0.3, 0.1, 0.16)), sdCylinder(rotZ(b - vec3(0.0, 0.05, 0.0), PI * 0.5), 0.16, 0.3));
  r = U(r, M(body, BLUE));
  vec3 fx = faceX(b, 0.3);
  r = U(r, M(sdBox(b - vec3(0.3, 0.0, 0.0), vec3(0.012, 0.13, 0.14)), BLUE));
  r = U(r, M(sdSphere(b - vec3(0.32, 0.0, 0.0), 0.025), STEEL + SOLID));
  vec3 f = b - vec3(0.0, 0.2, 0.17);
  r = U(r, M(sdBox(f - vec3(0.0, 0.0, 0.0), vec3(0.015, 0.2, 0.012)), GRAPHITE));
  r = U(r, M(sdBox(f - vec3(0.08, 0.14, 0.0), vec3(0.08, 0.06, 0.008)), CORAL));
  r = U(r, envelope(rotZ(rotY(b - vec3(0.36, 0.06, 0.0), 0.2), 0.9), WHITE));
  return r;
}

vec2 bin(vec3 p) {
  vec3 q = p - vec3(0.0, 0.2, 0.0);
  vec2 r = M(max(sdRoundBox(q, vec3(0.42, 0.2, 0.42), 0.02), -sdBox(q - vec3(0.0, 0.08, 0.0), vec3(0.38, 0.2, 0.38))), STEEL);
  r = stripes(r, q.x + q.z, 0.12);
  float y = 0.42;
  r = U(r, envelope(rotX(rotY(p - vec3(-0.15, y, -0.12), 0.4), 0.15), CORAL));
  r = U(r, sheet(rotZ(rotY(p - vec3(0.12, y + 0.02, -0.15), -0.5), 0.2), WHITE));
  r = U(r, bubble(rotX(rotY(p - vec3(-0.12, y + 0.04, 0.16), -0.9), -0.2), LILAC));
  r = U(r, folder(rotZ(rotY(p - vec3(0.15, y + 0.03, 0.14), 0.7), -0.15), SUN));
  r = U(r, sheet(rotX(rotY(p - vec3(0.0, y + 0.09, 0.0), 1.2), 0.25), PINK));
  r = U(r, envelope(rotZ(rotY(p - vec3(0.02, y + 0.13, 0.05), -0.3), 0.3), WHITE));
  return r;
}

vec2 messy(vec3 p) {
  vec2 r = M(1e9, 0.0);
  float y = H + 0.02;
  r = U(r, envelope(rotY(p - vec3(-1.95, y, 0.0), 0.5), WHITE));
  r = U(r, bubble(rotY(p - vec3(-1.55, y, 0.05), -0.4), LILAC));
  r = U(r, sheet(rotY(p - vec3(-1.18, y, -0.04), 0.9), WHITE));
  r = U(r, folder(rotY(p - vec3(-0.82, y + 0.01, 0.03), -0.3), SUN));
  return r;
}

vec2 machine(vec3 p) {
  vec3 c = p - vec3(0.0, 0.48, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.45, 0.48, 0.42), 0.03), WHITE);
  r = seam(r, c.y + 0.36);
  r = paint(r, c.y + 0.36, STEEL);
  vec3 fz = faceZ(c, 0.42);
  r = inset(r, fz - vec3(-0.08, 0.0, 0.14), vec2(0.26, 0.16), 0.03, 0.02, GRAPHITE);
  vec3 sc = fz - vec3(-0.08, -0.02, 0.14);
  r = decal(r, sc, bar(sc.xz, -0.2, 0.08, 0.14, 0.016), STEEL);
  r = decal(r, sc, bar(sc.xz, -0.2, 0.02, 0.26, 0.016), STEEL);
  r = decal(r, sc, bar(sc.xz, -0.2, -0.04, 0.2, 0.016), STEEL);
  r = decal(r, sc, bar(sc.xz, -0.2, -0.1, 0.3, 0.016), LIME);
  r = U(r, led(fz - vec3(0.32, 0.0, 0.24), 0.03, LIME));
  r = U(r, led(fz - vec3(0.32, 0.0, 0.14), 0.03, STEEL));
  r = U(r, button(fz - vec3(-0.25, 0.015, -0.17), 0.05, 0.02, STEEL));
  r = U(r, button(fz - vec3(-0.1, 0.015, -0.17), 0.05, 0.02, STEEL));
  r = vents(r, fz - vec3(0.2, 0.0, -0.17), 4.0, 0.06, 0.1, 0.024, GRAPHITE);
  vec3 fx = faceX(c, 0.45);
  vec3 sl = fx - vec3(0.0, 0.0, H - 0.48 + 0.04);
  r = Sc(r, M(max(sdRect(sl.xz, vec2(0.22, 0.05), 0.02), -(sl.y + 0.2)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(sl.xz, vec2(0.27, 0.09), 0.03), -sdRect(sl.xz, vec2(0.22, 0.05), 0.02)), sl.y, 0.02), STEEL));
  r = vents(r, fx - vec3(0.0, 0.0, 0.28), 6.0, 0.07, 0.14, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.33, fx.y, fx.z + 0.38), 0.02));
  vec3 t = p - vec3(0.0, 0.96, 0.0);
  r = U(r, M(sdRoundBox(t - vec3(0.0, 0.03, 0.0), vec3(0.3, 0.03, 0.28), 0.015), STEEL));
  r = U(r, M(sdCylinder(t - vec3(-0.12, 0.2, 0.0), 0.05, 0.2), GRAPHITE + ROUND));
  vec3 h = t - vec3(0.15, 0.06, -0.05);
  r = U(r, M(sdRoundCylinder(h - vec3(0.0, 0.1, 0.0), 0.12, 0.1, 0.03), WHITE + ROUND));
  r = U(r, M(max(sdSphere(h - vec3(0.0, 0.2, 0.0), 0.12), -(h.y - 0.2)), LILAC + SOLID));
  r = U(r, led(faceX(rotY(h - vec3(0.0, 0.1, 0.0), -PI * 0.25), 0.12) - vec3(0.0, 0.0, 0.0), 0.025, LIME));
  vec3 ch = t - vec3(-0.12, 0.42, 0.0);
  r = U(r, M(sdTorus(ch, 0.09, 0.02), STEEL + SOLID));
  r = U(r, M(sdSphere(ch - vec3(0.0, 0.08, 0.0), 0.05), WHITE + SOLID));
  return r;
}

vec2 clean(vec3 p) {
  float i = repLI(p.x - 0.7, 0.36, 4.0);
  vec3 q = vec3(repL(p.x - 0.7, 0.36, 4.0), p.y - H - 0.025, p.z);
  return card(q);
}

vec2 cabinet(vec3 p) {
  vec3 c = p - vec3(0.0, 0.55, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.4, 0.55, 0.45), 0.03), WHITE);
  vec3 fx = faceX(c, 0.4);
  vec3 d = vec3(fx.x, fx.y, repC(fx.z, 0.35, 3.0));
  float di = repI(fx.z, 0.35, 3.0);
  r = seam(r, -sdRect(d.xz, vec2(0.38, 0.16), 0.02));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.02, 0.06), vec3(0.12, 0.02, 0.025), 0.012), di == 1.0 ? LIME : STEEL));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.004, -0.06), vec3(0.08, 0.004, 0.03), 0.004), STEEL));
  vec3 fz = faceZ(c, 0.45);
  r = vents(r, fz - vec3(0.0, 0.0, 0.4), 5.0, 0.07, 0.12, 0.026, GRAPHITE);
  r = U(r, screw(vec3(abs(fz.x) - 0.28, fz.y, fz.z + 0.42), 0.02));
  vec3 tr = p - vec3(0.0, 1.1, 0.0);
  vec2 tray = M(max(sdRoundBox(tr - vec3(0.0, 0.1, 0.0), vec3(0.3, 0.1, 0.3), 0.02), -sdBox(tr - vec3(0.0, 0.16, 0.0), vec3(0.27, 0.1, 0.27))), STEEL);
  r = U(r, tray);
  for (int k = 0; k < 4; k++) {
    r = U(r, card(rotY(tr - vec3(0.0, 0.05 + 0.042 * float(k), 0.0), PI * 0.5)));
  }
  return r;
}

vec2 arm(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.03, 0.0), 0.16, 0.03, 0.01), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.12, 0.0), 0.1, 0.08), WHITE + ROUND));
  vec3 a = p - vec3(0.0, 0.2, 0.0);
  vec3 j1 = vec3(0.0, 0.55, 0.0);
  vec3 j2 = vec3(0.0, 0.85, 0.42);
  r = U(r, M(pipe(a, vec3(0.0), j1, 0.055), WHITE + SOLID));
  r = U(r, M(pipe(a, j1, j2, 0.045), WHITE + SOLID));
  r = U(r, M(sdSphere(a - j1, 0.075), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(a - j2, 0.06), GRAPHITE + SOLID));
  vec3 g = a - j2 - vec3(0.0, -0.12, 0.0);
  r = U(r, M(sdBox(g, vec3(0.025, 0.08, 0.025)), GRAPHITE));
  r = U(r, M(sdBox(vec3(g.x, g.y + 0.1, abs(g.z) - 0.06), vec3(0.02, 0.05, 0.012)), GRAPHITE));
  r = U(r, card(rotX(g - vec3(0.0, -0.16, 0.0), 0.0)));
  return r;
}

vec2 chatSign(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.02, 0.0), 0.12, 0.02, 0.008), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.42, 0.0), 0.018, 0.42), GRAPHITE + ROUND));
  vec3 b = p - vec3(0.0, 0.95, 0.0);
  float d2 = min(sdRect(b.xy, vec2(0.26, 0.16), 0.14), sdSeg(b.xy, vec2(-0.1, -0.1), vec2(-0.2, -0.24)) - 0.04);
  vec2 s = M(extrude(d2, b.z, 0.035), LILAC);
  vec3 f = faceZ(b, 0.035);
  s = decal(s, f, length(vec2(repC(f.x, 0.13, 3.0), f.z)) - 0.035, WHITE);
  return U(r, s);
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  vec3 g = p - vec3(0.0, B, 0.0);
  if (near(g - vec3(-2.55, 0.7, -0.5), vec3(0.45, 0.8, 0.3), r.x)) r = U(r, mailbox(g - vec3(-2.55, 0.0, -0.5)));
  if (near(g - vec3(-2.45, 0.3, 0.18), vec3(0.5, 0.4, 0.5), r.x)) r = U(r, bin(g - vec3(-2.45, 0.0, 0.18)));
  if (near(g - vec3(-1.4, 0.3, 0.0), vec3(0.75, 0.35, 0.3), r.x)) {
    r = U(r, conveyor(g - vec3(-2.05, 0.0, 0.0), 1.6, 0.24, H));
    r = U(r, messy(g));
  }
  if (near(g - vec3(-1.2, 0.5, -0.6), vec3(0.35, 0.5, 0.15), r.x)) r = U(r, chatSign(rotY(g - vec3(-1.2, 0.0, -0.6), 0.25)));
  if (near(g - vec3(0.0, 0.8, 0.0), vec3(0.5, 0.85, 0.47), r.x)) r = U(r, machine(g));
  if (near(g - vec3(1.1, 0.3, 0.0), vec3(0.65, 0.35, 0.3), r.x)) {
    r = U(r, conveyor(g - vec3(0.45, 0.0, 0.0), 1.3, 0.24, H));
    r = U(r, clean(g));
  }
  if (near(g - vec3(2.3, 0.7, 0.0), vec3(0.45, 0.75, 0.5), r.x)) r = U(r, cabinet(g - vec3(2.3, 0.0, 0.0)));
  return r;
}
