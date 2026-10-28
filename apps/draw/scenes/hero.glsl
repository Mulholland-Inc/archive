#pragma frame {"size":[1000,860],"center":[0.0,0.58,0.0],"zoom":188,"bounds":[[-1.9,-0.1,-1.9],[1.9,2.2,1.9]]}

float sdTri(vec2 p, float r) {
  const float k = 1.7320508;
  p.x = abs(p.x) - r;
  p.y = p.y + r / k;
  if (p.x + k * p.y > 0.0) p = vec2(p.x - k * p.y, -k * p.x - p.y) / 2.0;
  p.x -= clamp(p.x, -2.0 * r, 0.0);
  return -length(p) * sign(p.y);
}

float bar(vec2 p, float x0, float y, float len) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, 0.02), 0.02);
}

vec2 disk(vec3 p) {
  vec2 d = M(sdRoundCylinder(p, 0.5, 0.16, 0.05), WHITE + ROUND);
  vec3 q = faceX(rotY(p, -PI * 0.25), 0.5);
  d = vents(d, q - vec3(-0.1, 0.0, 0.0), 4.0, 0.055, 0.12, 0.02, GRAPHITE);
  d = U(d, led(q - vec3(0.17, 0.0, 0.03), 0.036, LIME));
  d = U(d, screw(q - vec3(0.17, 0.0, -0.075), 0.02));
  return d;
}

vec2 database(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.62, 0.04, 0.015), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.6, 0.0), 0.46, 0.5), GRAPHITE + ROUND));
  float y = p.y - 0.62;
  r = U(r, disk(vec3(p.x, repC(y, 0.38, 3.0), p.z)));
  vec3 t = p - vec3(0.0, 1.16, 0.0);
  r = U(r, M(sdRoundCylinder(t - vec3(0.0, 0.03, 0.0), 0.3, 0.03, 0.012), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(t - vec3(0.0, 0.08, 0.0), 0.13, 0.03, 0.012), WHITE + ROUND));
  r = U(r, screw(faceY(polarY(t, 8.0), 0.0) - vec3(0.4, 0.0, 0.0), 0.022));
  vec3 a = t - vec3(0.0, 0.11, 0.0);
  r = U(r, M(sdCylinder(a - vec3(0.0, 0.03, 0.0), 0.05, 0.03), GRAPHITE + ROUND));
  r = U(r, M(max(sdSphere(a - vec3(0.0, 0.06, 0.0), 0.06), -(a.y - 0.06)), LIME + SOLID));
  return r;
}

vec2 pageFace(vec2 r, vec3 f) {
  r = U(r, plate(f - vec3(-0.5, 0.0, 0.46), vec2(0.06), 0.016, 0.016, SUN));
  r = decal(r, f, sdRect(f.xz - vec2(-0.08, 0.46), vec2(0.3, 0.03), 0.03), GRAPHITE);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.35), vec2(0.6, 0.005), 0.005), STEEL);
  r = decal(r, f, bar(f.xz, -0.56, 0.25, 1.08), STEEL);
  r = decal(r, f, bar(f.xz, -0.56, 0.17, 0.96), STEEL);
  r = decal(r, f, bar(f.xz, -0.56, 0.09, 0.62), STEEL);
  vec3 im = f - vec3(0.0, 0.0, -0.12);
  float frame = sdRect(im.xz, vec2(0.56, 0.12), 0.03);
  r = inset(r, im, vec2(0.56, 0.12), 0.03, 0.014, SKY);
  vec3 fl = im + vec3(0.0, 0.014, 0.0);
  r = decal(r, fl, max(frame, min(sdCircle(fl.xz - vec2(0.22, -0.3), 0.26), sdCircle(fl.xz - vec2(-0.12, -0.36), 0.24))), MINT);
  r = decal(r, fl, sdCircle(fl.xz - vec2(-0.36, 0.04), 0.035), SUN);
  vec3 c1 = f - vec3(-0.5, 0.0, -0.36);
  r = U(r, plate(c1, vec2(0.045), 0.012, 0.016, LIME));
  vec3 ck = c1 - vec3(0.0, 0.016, 0.0);
  float tick = min(sdSeg(ck.xz, vec2(-0.024, 0.0), vec2(-0.006, -0.02)), sdSeg(ck.xz, vec2(-0.006, -0.02), vec2(0.026, 0.022))) - 0.008;
  r = decal(r, ck, tick, GRAPHITE);
  r = decal(r, f, bar(f.xz, -0.42, -0.36, 0.5), STEEL);
  r = U(r, plate(f - vec3(-0.5, 0.0, -0.5), vec2(0.045), 0.012, 0.016, WHITE));
  r = decal(r, f, bar(f.xz, -0.42, -0.5, 0.38), STEEL);
  return r;
}

vec2 pagePanel(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(0.2, 0.05, 0.66), 0.02), STEEL);
  r = seam(r, p.y - 0.06);
  vec3 b = p - vec3(0.0, 0.76, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.05, 0.66, 0.72), 0.035), WHITE);
  vec3 q = faceX(b, 0.05);
  body = pageFace(body, vec3(-q.x, q.y, q.z));
  r = U(r, body);
  vec3 s = faceY(p, 0.1);
  r = U(r, screw(vec3(s.x - 0.12, s.y, abs(s.z) - 0.56), 0.022));
  return r;
}

vec2 chatFace(vec2 r, vec3 f) {
  r = decal(r, f, sdCircle(vec2(repC(f.x + 0.5, 0.07, 3.0), f.z - 0.5), 0.022), STEEL);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.42), vec2(0.6, 0.005), 0.005), STEEL);
  vec3 b1 = f - vec3(-0.2, 0.0, 0.25);
  r = U(r, plate(b1, vec2(0.36, 0.1), 0.06, 0.016, WHITE));
  vec3 t1 = b1 - vec3(0.0, 0.016, 0.0);
  r = decal(r, t1, bar(t1.xz, -0.28, 0.035, 0.5), STEEL);
  r = decal(r, t1, bar(t1.xz, -0.28, -0.035, 0.32), STEEL);
  vec3 b2 = f - vec3(0.22, 0.0, 0.0);
  r = U(r, plate(b2, vec2(0.34, 0.1), 0.06, 0.016, GRAPHITE));
  vec3 t2 = b2 - vec3(0.0, 0.016, 0.0);
  r = decal(r, t2, bar(t2.xz, -0.26, 0.035, 0.52), WHITE);
  r = decal(r, t2, bar(t2.xz, -0.26, -0.035, 0.3), WHITE);
  vec3 b3 = f - vec3(-0.28, 0.0, -0.23);
  r = U(r, plate(b3, vec2(0.28, 0.07), 0.06, 0.016, WHITE));
  vec3 t3 = b3 - vec3(0.0, 0.016, 0.0);
  r = decal(r, t3, sdRect(t3.xz - vec2(-0.18, 0.0), vec2(0.045, 0.035), 0.012), LIME);
  r = decal(r, t3, bar(t3.xz, -0.1, 0.0, 0.3), STEEL);
  vec3 cp = f - vec3(-0.08, 0.0, -0.46);
  r = inset(r, cp, vec2(0.5, 0.08), 0.06, 0.012, PAPER);
  vec3 cf = cp + vec3(0.0, 0.012, 0.0);
  r = decal(r, cf, bar(cf.xz, -0.42, 0.0, 0.36), STEEL);
  vec3 rb = f - vec3(0.52, 0.0, -0.46);
  r = U(r, button(rb - vec3(0.0, 0.02, 0.0), 0.08, 0.035, LILAC));
  vec3 rt = rb - vec3(0.0, 0.055, 0.0);
  r = decal(r, rt, sdTri(vec2(rt.z, -rt.x - 0.008), 0.03) - 0.004, GRAPHITE);
  return r;
}

vec2 chatPanel(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(0.66, 0.05, 0.2), 0.02), STEEL);
  r = seam(r, p.y - 0.06);
  vec3 b = p - vec3(0.0, 0.76, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.72, 0.66, 0.05), 0.035), WHITE);
  body = chatFace(body, faceZ(b, 0.05));
  r = U(r, body);
  vec3 s = faceY(p, 0.1);
  r = U(r, screw(vec3(abs(s.x) - 0.56, s.y, s.z - 0.12), 0.022));
  return r;
}

vec2 folderCard(vec3 p, float m) {
  vec2 f = M(sdBox(p, vec3(0.007, 0.1, 0.19)), m);
  f = U(f, M(sdRoundBox(p - vec3(0.0, 0.11, -0.08), vec3(0.007, 0.03, 0.06), 0.006), m));
  return f;
}

vec2 cabinet(vec3 p) {
  vec3 c = p - vec3(0.0, 0.43, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.27, 0.43, 0.3), 0.025), WHITE);
  r = Sc(r, M(sdBox(c - vec3(0.27, 0.28, 0.0), vec3(0.22, 0.125, 0.25)), GRAPHITE));
  vec3 f = faceX(c, 0.27);
  for (int i = 0; i < 2; i++) {
    vec3 d = f - vec3(0.0, 0.0, -0.02 - 0.27 * float(i));
    r = U(r, plate(d, vec2(0.26, 0.115), 0.02, 0.012, WHITE));
    r = U(r, M(pipe(d, vec3(-0.08, 0.035, 0.05), vec3(0.08, 0.035, 0.05), 0.016), STEEL + SOLID));
    r = U(r, M(sdBox(d - vec3(0.0, 0.012, -0.03), vec3(0.06, 0.008, 0.03)), STEEL));
  }
  vec3 dr = c - vec3(0.38, 0.27, 0.0);
  float box = sdBox(dr, vec3(0.25, 0.11, 0.24));
  vec2 drawer = M(max(box, -sdBox(dr - vec3(0.0, 0.03, 0.0), vec3(0.235, 0.11, 0.225))), WHITE);
  vec3 fr = dr - vec3(0.262, 0.0, 0.0);
  drawer = U(drawer, M(sdRoundBox(fr, vec3(0.014, 0.125, 0.28), 0.012), WHITE));
  drawer = U(drawer, M(pipe(fr, vec3(0.03, 0.04, -0.08), vec3(0.03, 0.04, 0.08), 0.016), STEEL + SOLID));
  drawer = U(drawer, M(pipe(fr, vec3(0.015, 0.04, -0.08), vec3(0.03, 0.04, -0.08), 0.012), STEEL + SOLID));
  drawer = U(drawer, M(pipe(fr, vec3(0.015, 0.04, 0.08), vec3(0.03, 0.04, 0.08), 0.012), STEEL + SOLID));
  drawer = U(drawer, M(sdBox(fr - vec3(0.014, -0.035, 0.0), vec3(0.008, 0.03, 0.06)), STEEL));
  r = U(r, drawer);
  vec3 fq = dr - vec3(-0.17, 0.0, 0.0);
  float i = repLI(fq.x, 0.075, 5.0);
  float lift = i == 2.0 ? 0.13 : 0.0;
  float m = i == 2.0 ? SUN : i == 3.0 ? LILAC : WHITE;
  vec3 fc = vec3(repL(fq.x, 0.075, 5.0), fq.y + 0.03 - lift, fq.z + (i == 2.0 ? 0.0 : mod(i, 2.0) * 0.06 - 0.03));
  r = U(r, folderCard(fc, m));
  vec3 tp = faceY(c, 0.43);
  vec3 sh = rotY(tp - vec3(-0.02, 0.0, 0.02), 0.25);
  r = U(r, M(sdBox(sh - vec3(0.0, 0.008, 0.0), vec3(0.13, 0.008, 0.17)), PAPER));
  vec3 sh2 = rotY(tp - vec3(0.02, 0.0, 0.0), -0.1);
  vec2 s2 = M(sdBox(sh2 - vec3(0.0, 0.024, 0.0), vec3(0.13, 0.008, 0.17)), WHITE);
  vec3 st = sh2 - vec3(0.0, 0.032, 0.0);
  s2 = decal(s2, st, bar(st.xz, -0.09, -0.09, 0.16), STEEL);
  s2 = decal(s2, st, bar(st.xz, -0.09, -0.02, 0.12), STEEL);
  s2 = decal(s2, st, sdRect(st.xz - vec2(0.0, 0.08), vec2(0.09, 0.04), 0.01), STEEL);
  r = U(r, s2);
  return r;
}

vec2 cube(vec3 p, float s, float m) {
  return M(sdRoundBox(p - vec3(0.0, s, 0.0), vec3(s), s * 0.12), m);
}

vec2 cart(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.12, 0.0), vec3(0.3, 0.025, 0.22), 0.01), SUN);
  vec3 w = vec3(abs(p.x) - 0.2, p.y - 0.06, abs(p.z) - 0.2);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.06, 0.022), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.024, 0.028), STEEL + SOLID));
  vec3 h = p - vec3(-0.3, 0.0, 0.0);
  r = U(r, M(sdBox(vec3(h.x, h.y - 0.31, abs(h.z) - 0.16), vec3(0.014, 0.17, 0.014)), GRAPHITE));
  r = U(r, M(pipe(h, vec3(0.0, 0.48, -0.18), vec3(0.0, 0.48, 0.18), 0.02), GRAPHITE + SOLID));
  r = U(r, cube(p - vec3(-0.06, 0.145, -0.05), 0.11, WHITE));
  r = U(r, cube(rotY(p - vec3(0.17, 0.145, 0.08), 0.5), 0.08, LILAC));
  r = U(r, cube(rotY(p - vec3(-0.05, 0.365, -0.03), 0.3), 0.075, WHITE));
  return r;
}

vec2 pipes(vec3 p) {
  float y = 0.18;
  float rad = 0.045;
  vec2 r = M(pipe(p, vec3(-0.75, y, -0.6), vec3(-0.75, y, 0.5), rad), STEEL + SOLID);
  r = U(r, M(pipe(p, vec3(-0.75, y, 0.5), vec3(-1.14, y, 0.5), rad), STEEL + SOLID));
  r = U(r, M(pipe(p, vec3(-0.6, y, -0.75), vec3(0.5, y, -0.75), rad), STEEL + SOLID));
  r = U(r, M(pipe(p, vec3(0.5, y, -0.75), vec3(0.5, y, -1.14), rad), STEEL + SOLID));
  r = U(r, M(pipe(p, vec3(-0.75, y, 0.32), vec3(0.08, y, 0.32), rad), STEEL + SOLID));
  r = U(r, M(sdCylinder(rotX(p - vec3(-0.75, y, -0.2), PI * 0.5), 0.065, 0.018), STEEL + SOLID));
  r = U(r, M(sdCylinder(rotZ(vec3(p.x - 0.05, p.y - y, p.z + 0.75), PI * 0.5), 0.065, 0.018), STEEL + SOLID));
  r = U(r, M(sdCylinder(rotZ(vec3(p.x + 1.11, p.y - y, p.z - 0.5), PI * 0.5), 0.07, 0.022), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(vec3(p.x - 0.5, p.y - y, p.z + 1.11), PI * 0.5), 0.07, 0.022), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotZ(vec3(p.x - 0.06, p.y - y, p.z - 0.32), PI * 0.5), 0.07, 0.022), GRAPHITE + SOLID));
  vec3 v = p - vec3(-0.52, y, 0.32);
  r = U(r, M(sdCylinder(v - vec3(0.0, 0.08, 0.0), 0.016, 0.08), GRAPHITE + SOLID));
  r = U(r, M(sdTorus(v - vec3(0.0, 0.16, 0.0), 0.07, 0.014), SUN + SOLID));
  r = U(r, M(sdCylinder(v, 0.06, 0.06), STEEL + SOLID));
  return r;
}

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.15, 0.012, 0.1)), m);
  return seam(e, p.z + 0.1 - (0.15 - abs(p.x)) * 0.6);
}

vec2 tray(vec3 p) {
  vec3 t = p - vec3(0.0, 0.07, 0.0);
  vec2 r = M(max(sdRoundBox(t, vec3(0.24, 0.07, 0.2), 0.02), -sdBox(t - vec3(0.0, 0.05, 0.0), vec3(0.21, 0.07, 0.17))), STEEL);
  r = U(r, M(sdBox(rotY(t - vec3(0.0, -0.03, 0.0), 0.08), vec3(0.15, 0.012, 0.12)), PAPER));
  r = U(r, M(sdBox(rotY(t - vec3(0.01, -0.005, 0.01), -0.12), vec3(0.15, 0.008, 0.12)), SUN));
  r = U(r, envelope(rotZ(rotY(t - vec3(0.02, 0.05, -0.02), 0.25), 0.18), WHITE));
  vec3 s = faceX(t, 0.24);
  r = U(r, M(sdBox(s - vec3(0.0, 0.006, 0.0), vec3(0.06, 0.006, 0.022)), WHITE));
  return r;
}

vec2 plinth(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.1, 0.0), vec3(1.55, 0.1, 1.55), 0.06), WHITE);
  r = seam(r, p.y - 0.15);
  r = paint(r, p.y - 0.06, STEEL);
  r = vents(r, faceX(p, 1.55) - vec3(-0.8, 0.0, 0.105), 7.0, 0.07, 0.045, 0.03, GRAPHITE);
  r = vents(r, faceZ(p, 1.55) - vec3(0.8, 0.0, 0.105), 7.0, 0.07, 0.045, 0.03, GRAPHITE);
  vec3 lz = faceZ(p, 1.55) - vec3(-1.1, 0.0, 0.105);
  r = U(r, led(vec3(repC(lz.x, 0.1, 3.0), lz.y, lz.z), 0.025, LIME));
  vec3 c = mirXZ(p);
  r = U(r, screw(faceY(c, 0.2) - vec3(1.42, 0.0, 1.42), 0.03));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = plinth(p);
  vec3 g = p - vec3(0.0, 0.2, 0.0);
  if (near(g - vec3(-0.95, 0.8, -0.95), vec3(0.7, 0.9, 0.7), r.x)) r = U(r, database(g - vec3(-0.95, 0.0, -0.95)));
  if (near(g - vec3(-1.2, 0.75, 0.5), vec3(0.25, 0.8, 0.8), r.x)) r = U(r, pagePanel(g - vec3(-1.2, 0.0, 0.5)));
  if (near(g - vec3(0.5, 0.75, -1.2), vec3(0.8, 0.8, 0.25), r.x)) r = U(r, chatPanel(g - vec3(0.5, 0.0, -1.2)));
  if (near(g - vec3(0.5, 0.5, 0.4), vec3(0.6, 0.6, 0.4), r.x)) r = U(r, cabinet(g - vec3(0.35, 0.0, 0.4)));
  if (near(g - vec3(-0.3, 0.3, -0.3), vec3(0.9, 0.25, 0.9), r.x)) r = U(r, pipes(g));
  if (near(g - vec3(1.12, 0.3, -0.15), vec3(0.45, 0.35, 0.45), r.x)) r = U(r, cart(rotY(g - vec3(1.12, 0.0, -0.15), -1.85)));
  if (near(g - vec3(0.35, 0.2, 1.2), vec3(0.5, 0.3, 0.35), r.x)) {
    r = U(r, cube(g - vec3(0.15, 0.0, 1.12), 0.13, WHITE));
    r = U(r, cube(rotY(g - vec3(0.15, 0.26, 1.12), 0.35), 0.08, LILAC));
    r = U(r, cube(rotY(g - vec3(0.6, 0.0, 1.25), 0.45), 0.09, LIME));
  }
  if (near(g - vec3(1.12, 0.15, 0.85), vec3(0.3, 0.2, 0.3), r.x)) {
    r = U(r, tray(rotY(g - vec3(1.12, 0.0, 0.85), 0.0)));
  }
  return r;
}
