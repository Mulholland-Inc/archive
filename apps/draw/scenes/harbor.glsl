#pragma frame {"size":[1600,720],"center":[0.25,1.13,-0.25],"zoom":92,"bounds":[[-6.0,-0.3,-7.2],[6.4,4.0,5.0]]}

float hue(float i) {
  i = mod(i, 6.0);
  return i < 1.0 ? CORAL : i < 2.0 ? SUN : i < 3.0 ? MINT : i < 4.0 ? WHITE : i < 5.0 ? LILAC : SKY;
}

vec2 container(vec3 p, float m) {
  vec2 c = M(sdBox(p, vec3(0.5, 0.21, 0.21)), m);
  if (c.x > 0.06) return c;
  c = stripes(c, p.x + 0.5, 0.045);
  c = paint(c, 0.44 - abs(p.x), m);
  if (abs(p.x) > 0.44) c = seam(c, abs(p.z) - 0.21 + 0.0);
  vec3 e = faceX(p, 0.5);
  if (p.x > 0.44) c = seam(c, p.z);
  c = U(c, M(sdBox(vec3(abs(e.x) - 0.08, e.y - 0.006, e.z), vec3(0.01, 0.008, 0.18)), GRAPHITE));
  return c;
}

vec2 column(vec3 p, float n, float s) {
  float y = p.y - 0.21;
  float i = repLI(y, 0.43, n);
  return container(vec3(p.x, repL(y, 0.43, n), p.z), hue(s + i * 3.0));
}

vec2 barge(vec3 p, float seed, float miss) {
  vec3 h = p - vec3(0.0, 0.1, 0.0);
  float hull = max(sdBox(h, vec3(1.75, 0.18, 0.56)), (abs(h.x) - h.y - 1.62) * 0.707);
  vec2 r = M(hull, WHITE);
  r = paint(r, p.y - 0.13, GRAPHITE);
  r = seam(r, p.y - 0.22);
  r = U(r, M(max(sdBox(p - vec3(0.0, 0.31, 0.0), vec3(1.75, 0.03, 0.56)), -sdBox(p - vec3(0.0, 0.34, 0.0), vec3(1.69, 0.06, 0.5))), STEEL));
  vec3 bq = vec3(abs(p.x) - 1.62, p.y - 0.32, abs(p.z) - 0.4);
  r = U(r, M(sdCylinder(bq, 0.035, 0.06), GRAPHITE + ROUND));
  for (int i = 0; i < 3; i++) {
    for (int k = 0; k < 2; k++) {
      float fi = float(i), fk = float(k);
      vec3 cq = p - vec3((fi - 1.0) * 1.06, 0.28, (fk - 0.5) * 0.45);
      if (sdBox(cq - vec3(0.0, 0.5, 0.0), vec3(0.55, 0.55, 0.25)) > r.x) continue;
      float n = mod(fi + fk * 2.0 + seed, 3.0) < 1.0 ? 1.0 : 2.0;
      if (fi * 2.0 + fk == miss) n -= 1.0;
      if (n > 0.5) r = U(r, column(cq, n, fi * 2.0 + fk * 3.0 + seed));
    }
  }
  return r;
}

vec2 water(vec3 p) {
  vec3 w = p - vec3(-2.0, -0.07, 2.5);
  vec2 r = M(sdBox(w, vec3(3.4, 0.07, 1.9)), BLUE);
  float d = 1e9;
  d = min(d, sdSeg(p.xz, vec2(-1.1, 1.25), vec2(-0.6, 1.25)));
  d = min(d, sdSeg(p.xz, vec2(-0.85, 1.55), vec2(-0.45, 1.55)));
  d = min(d, sdSeg(p.xz, vec2(0.85, 3.2), vec2(1.25, 3.2)));
  d = min(d, sdSeg(p.xz, vec2(0.95, 3.6), vec2(1.3, 3.6)));
  d = min(d, sdSeg(p.xz, vec2(0.8, 3.4), vec2(1.15, 3.4)));
  d = min(d, sdSeg(p.xz, vec2(-4.9, 2.2), vec2(-4.4, 2.2)));
  d = min(d, sdSeg(p.xz, vec2(-4.7, 2.45), vec2(-4.35, 2.45)));
  d = min(d, sdSeg(p.xz, vec2(-1.2, 4.15), vec2(-0.6, 4.15)));
  d = min(d, sdSeg(p.xz, vec2(-3.4, 4.2), vec2(-2.9, 4.2)));
  d = min(d, sdSeg(p.xz, vec2(0.3, 1.9), vec2(0.8, 1.9)));
  d = min(d, sdSeg(p.xz, vec2(-4.1, 4.25), vec2(-3.7, 4.25)));
  r = seam(r, -(d - 0.012) + 0.0 * w.y);
  if (w.y < 0.065) r = M(r.x, BLUE);
  return r;
}

vec2 land(vec3 p) {
  float a = sdBox(p - vec3(-0.2, 0.08, -0.6), vec3(5.2, 0.22, 1.2));
  float b = sdBox(p - vec3(3.0, 0.08, -4.25), vec3(1.4, 0.22, 2.45));
  vec2 r = M(min(a, b), PAPER);
  r = paint(r, p.y - 0.27, STEEL);
  if (p.y < 0.27) r = stripes(r, p.y + 0.14, 0.1);
  if (p.y < 0.27) r = seam(r, abs(mod(p.x - p.z + (mod(floor((p.y + 0.14) / 0.1), 2.0)) * 0.3, 0.6) - 0.3) - 0.29);
  if (p.y > 0.27) r = stripes(stripes(r, p.x + 0.2, 1.1), p.z - 0.6 + 1.1, 1.1);
  vec3 e = p - vec3(-2.0, 0.31, 0.52);
  r = U(r, M(sdBox(e, vec3(3.4, 0.015, 0.06)), WHITE));
  r = decal(r, faceY(p, 0.3), max(abs(p.z - 0.3) - 0.025, abs(p.x + 2.0) - 3.35), SUN);
  vec3 cr = vec3(p.x + 2.6, p.y - 0.31, abs(p.z + 0.5) - 0.8);
  r = U(r, M(sdBox(cr, vec3(2.75, 0.012, 0.025)), GRAPHITE));
  vec3 fz = faceZ(p, 0.6);
  vec3 fq = vec3(repC(fz.x + 2.4, 0.75, 9.0), fz.y + 0.01, fz.z - 0.12);
  r = U(r, M(sdTorus(rotX(fq, PI * 0.5), 0.07, 0.03), GRAPHITE + SOLID));
  return r;
}

vec2 bollard(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.08, 0.0), 0.06, 0.08), GRAPHITE + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.17, 0.0), 0.085, 0.022), GRAPHITE + ROUND));
  r = U(r, M(sdBox(p - vec3(0.0, 0.012, 0.0), vec3(0.11, 0.012, 0.11)), GRAPHITE));
  return r;
}

vec2 track(vec3 q, float len) {
  vec3 c = q - vec3(len * 0.5, 0.0, 0.0);
  vec2 r = M(sdBox(vec3(c.x, c.y - 0.06, abs(c.z) - 0.2), vec3(len * 0.5, 0.02, 0.018)), GRAPHITE);
  float n = floor(len / 0.2);
  r = U(r, M(sdBox(vec3(repC(c.x, len / n, n), c.y - 0.02, c.z), vec3(0.04, 0.02, 0.3)), STEEL));
  return r;
}

vec2 arc(vec3 q, float R) {
  float l = length(q.xz) - R;
  float clip = max(-q.x, -q.z);
  vec2 r = M(max(sdBox(vec3(abs(l) - 0.2, q.y - 0.06, 0.0), vec3(0.018, 0.02, 1.0)), clip), GRAPHITE);
  vec3 s = polarY(q, 32.0) - vec3(R, 0.02, 0.0);
  r = U(r, M(max(sdBox(s, vec3(0.3, 0.02, 0.04)), clip), STEEL));
  return r;
}

vec2 wagon(vec3 p, float m) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.25, 0.0), vec3(0.55, 0.035, 0.25)), WHITE);
  r = seam(r, abs(p.z) - 0.2);
  r = U(r, M(sdBox(p - vec3(0.0, 0.19, 0.0), vec3(0.48, 0.03, 0.15)), GRAPHITE));
  vec3 w = vec3(abs(p.x) - 0.33, p.y - 0.16, abs(p.z) - 0.2);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.075, 0.022), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(rotX(w - vec3(0.0, 0.0, 0.012), PI * 0.5), 0.03, 0.022), STEEL + SOLID));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.6, p.y - 0.2, p.z), vec3(0.05, 0.02, 0.025)), GRAPHITE));
  vec3 k = vec3(abs(p.x) - 0.5, p.y - 0.31, abs(p.z) - 0.22);
  r = U(r, M(sdBox(k, vec3(0.02, 0.025, 0.02)), SUN));
  if (m > -0.5) r = U(r, container(p - vec3(0.0, 0.495, 0.0), m));
  return r;
}

vec2 crane(vec3 p, float tz, float drop, float m) {
  vec3 l = vec3(abs(p.x) - 0.5, p.y - 0.75, abs(p.z) - 0.8);
  vec2 r = M(sdBox(l, vec3(0.065, 0.75, 0.065)), WHITE);
  r = paint(r, p.y - 0.35, STEEL);
  vec3 sb = vec3(p.x, p.y - 0.07, abs(p.z) - 0.8);
  r = U(r, M(sdBox(sb, vec3(0.62, 0.07, 0.09)), GRAPHITE));
  vec3 wh = vec3(abs(p.x) - 0.42, p.y - 0.05, abs(p.z) - 0.8);
  r = U(r, M(sdCylinder(rotZ(wh, PI * 0.5), 0.05, 0.1), STEEL + SOLID));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.5, p.y - 1.48, p.z), vec3(0.07, 0.07, 0.87)), WHITE));
  r = U(r, M(sdBox(vec3(p.x, p.y - 1.48, abs(p.z) - 0.8), vec3(0.57, 0.07, 0.07)), WHITE));
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.75, abs(p.z) - 0.8), vec3(0.5, 0.035, 0.035)), STEEL));
  vec3 bm = vec3(abs(p.x) - 0.17, p.y - 1.62, p.z - 0.65);
  vec2 boom = M(sdBox(bm, vec3(0.05, 0.07, 2.3)), WHITE);
  boom = stripes(boom, p.z, 0.15);
  r = U(r, boom);
  vec3 ap = vec3(abs(p.x) - 0.17, p.y, p.z);
  float st = sdCapsule(ap, vec3(0.0, 1.55, -0.8), vec3(0.0, 2.2, -0.35), 0.03);
  st = min(st, sdCapsule(ap, vec3(0.0, 1.55, 0.8), vec3(0.0, 2.2, -0.35), 0.03));
  st = min(st, sdCapsule(ap, vec3(0.0, 2.2, -0.35), vec3(0.0, 1.68, 2.9), 0.012));
  st = min(st, sdCapsule(ap, vec3(0.0, 2.2, -0.35), vec3(0.0, 1.68, -1.55), 0.012));
  r = U(r, M(st, GRAPHITE + SOLID));
  r = U(r, M(sdBox(p - vec3(0.0, 2.22, -0.35), vec3(0.22, 0.04, 0.05)), GRAPHITE));
  vec3 mh = p - vec3(0.0, 1.87, -1.15);
  vec2 house = M(sdBox(mh, vec3(0.32, 0.18, 0.32)), WHITE);
  house = vents(house, faceX(mh, 0.32) - vec3(0.0, 0.0, 0.02), 4.0, 0.07, 0.18, 0.03, GRAPHITE);
  house = inset(house, faceZ(mh, 0.32) - vec3(0.1, 0.0, 0.04), vec2(0.1, 0.07), 0.015, 0.015, SKY);
  r = U(r, house);
  r = U(r, M(sdBox(p - vec3(0.0, 1.7, -1.15), vec3(0.34, 0.03, 0.34)), GRAPHITE));
  vec3 cb = p - vec3(0.32, 1.42, 0.95);
  vec2 cab = M(sdBox(cb, vec3(0.12, 0.11, 0.14)), WHITE);
  cab = inset(cab, faceZ(cb, 0.14), vec2(0.09, 0.07), 0.015, 0.015, SKY);
  cab = inset(cab, faceX(cb, 0.12), vec2(0.1, 0.07), 0.015, 0.015, SKY);
  r = U(r, cab);
  vec3 t = p - vec3(0.0, 1.52, tz);
  r = U(r, M(sdBox(t, vec3(0.24, 0.05, 0.17)), GRAPHITE));
  vec3 cq = vec3(abs(t.x) - 0.14, t.y, abs(t.z) - 0.1);
  r = U(r, M(sdBox(cq - vec3(0.0, -drop * 0.5, 0.0), vec3(0.006, drop * 0.5, 0.006)), GRAPHITE));
  vec3 sp = t - vec3(0.0, -drop - 0.04, 0.0);
  r = U(r, M(sdBox(sp, vec3(0.52, 0.04, 0.2)), SUN));
  r = U(r, M(sdBox(sp - vec3(0.0, 0.07, 0.0), vec3(0.16, 0.035, 0.12)), SUN));
  if (m > -0.5) r = U(r, container(sp - vec3(0.0, -0.25, 0.0), m));
  return r;
}

vec2 lighthouse(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.55, 0.12), STEEL + ROUND);
  r = seam(r, p.y - 0.04);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.2, 0.0), 0.4, 0.05), WHITE + ROUND));
  vec2 tw = M(sdCone(p - vec3(0.0, 1.05, 0.0), 0.8, 0.31, 0.21), WHITE + ROUND);
  tw = paint(tw, abs(mod(p.y - 0.25, 0.54) - 0.405) - 0.135, CORAL + ROUND);
  r = U(r, tw);
  vec3 f = faceX(rotY(p, -PI * 0.25), 0.0);
  vec3 dq = vec3(f.x, f.y - 0.3, f.z - 0.42);
  r = Sc(r, M(max(sdRect(dq.xz, vec2(0.075, 0.12), 0.06), -(dq.y + 0.04)), GRAPHITE + SOLID));
  vec3 wq = vec3(f.x, f.y - 0.25, f.z - 1.25);
  r = Sc(r, M(max(sdRect(wq.xz, vec2(0.04, 0.07), 0.04), -(wq.y + 0.04)), SKY + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.88, 0.0), 0.33, 0.03), WHITE + ROUND));
  vec3 rp = polarY(p, 12.0) - vec3(0.3, 2.0, 0.0);
  r = U(r, M(sdBox(rp, vec3(0.01, 0.1, 0.01)), GRAPHITE));
  r = U(r, M(sdTorus(p - vec3(0.0, 2.1, 0.0), 0.3, 0.012), GRAPHITE + SOLID));
  vec2 lan = M(sdCylinder(p - vec3(0.0, 2.08, 0.0), 0.17, 0.17), SUN + ROUND);
  vec3 mp = polarY(p, 6.0) - vec3(0.17, 2.08, 0.0);
  lan = U(lan, M(sdBox(mp, vec3(0.018, 0.17, 0.018)), GRAPHITE));
  r = U(r, lan);
  r = U(r, M(sdCylinder(p - vec3(0.0, 2.26, 0.0), 0.21, 0.02), GRAPHITE + ROUND));
  r = U(r, M(sdCone(p - vec3(0.0, 2.38, 0.0), 0.1, 0.21, 0.05), CORAL + SOLID));
  r = U(r, M(sdSphere(p - vec3(0.0, 2.52, 0.0), 0.045), GRAPHITE + SOLID));
  return r;
}

vec2 tug(vec3 p) {
  vec3 h = p - vec3(0.0, 0.1, 0.0);
  float hull = sdRoundBox(h, vec3(0.45, 0.13, 0.24), 0.06);
  hull = max(hull, (abs(h.z) + (-h.x) * 0.6 - 0.36) * 0.857);
  vec2 r = M(hull, CORAL + SOLID);
  r = paint(r, -(p.y - 0.17), CORAL + SOLID);
  r = paint(r, p.y - 0.17, WHITE + SOLID);
  r = Sc(r, M(max(max(sdBox(h, vec3(0.4, 0.2, 0.19)), (abs(h.z) + (-h.x) * 0.6 - 0.3) * 0.857), -(h.y - 0.09)), WHITE + SOLID));
  r = U(r, M(sdTorus(rotZ(h - vec3(-0.44, 0.02, 0.0), PI * 0.5), 0.07, 0.03), GRAPHITE + SOLID));
  vec3 fq = vec3(repC(h.x, 0.22, 3.0), h.y - 0.0, abs(h.z) - 0.25);
  r = U(r, M(sdTorus(rotX(fq, PI * 0.5), 0.05, 0.022), GRAPHITE + SOLID));
  vec3 c = p - vec3(0.05, 0.36, 0.0);
  vec2 cab = M(sdBox(c, vec3(0.17, 0.12, 0.15)), WHITE);
  cab = inset(cab, faceZ(c, 0.15) - vec3(-0.06, 0.0, 0.02), vec2(0.06, 0.05), 0.015, 0.012, SKY);
  cab = inset(cab, faceZ(c, 0.15) - vec3(0.08, 0.0, 0.02), vec2(0.04, 0.05), 0.015, 0.012, SKY);
  r = U(r, cab);
  vec3 w = p - vec3(-0.02, 0.56, 0.0);
  vec2 wh = M(sdBox(w, vec3(0.12, 0.08, 0.13)), WHITE);
  wh = inset(wh, faceZ(w, 0.13), vec2(0.09, 0.04), 0.012, 0.012, SKY);
  wh = inset(wh, faceX(w, 0.12), vec2(0.1, 0.04), 0.012, 0.012, SKY);
  r = U(r, wh);
  r = U(r, M(sdBox(w - vec3(0.0, 0.1, 0.0), vec3(0.15, 0.02, 0.16)), GRAPHITE));
  r = U(r, M(sdCylinder(w - vec3(0.0, 0.22, 0.0), 0.012, 0.12), GRAPHITE + SOLID));
  r = U(r, led(w - vec3(0.0, 0.35, 0.0), 0.03, LIME));
  vec3 fn = p - vec3(0.3, 0.5, 0.0);
  vec2 fun = M(sdCylinder(fn, 0.07, 0.16), WHITE + ROUND);
  fun = paint(fun, abs(fn.y - 0.06) - 0.04, CORAL + ROUND);
  r = U(r, fun);
  r = U(r, M(sdCylinder(fn - vec3(0.0, 0.17, 0.0), 0.075, 0.015), GRAPHITE + ROUND));
  return r;
}

vec2 drum(vec3 p, float lit) {
  vec2 d = M(sdRoundCylinder(p, 0.62, 0.3, 0.07), WHITE + ROUND);
  d = seam(d, abs(p.y) - 0.18);
  vec3 q = faceX(rotY(p, -PI * 0.25), 0.62);
  d = vents(d, q - vec3(-0.12, 0.0, 0.0), 4.0, 0.06, 0.15, 0.024, GRAPHITE);
  d = U(d, led(q - vec3(0.2, 0.0, 0.03), 0.04, lit > 0.5 ? LIME : STEEL));
  d = U(d, screw(q - vec3(0.2, 0.0, -0.09), 0.022));
  return d;
}

vec2 silo(vec3 p, float n) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.76, 0.05), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.1 + n * 0.33, 0.0), 0.56, n * 0.33), GRAPHITE + ROUND));
  float y = p.y - 0.43;
  float i = repLI(y, 0.66, n);
  r = U(r, drum(vec3(p.x, repL(y, 0.66, n), p.z), mod(i, 2.0) == 0.0 ? 1.0 : 0.0));
  vec3 t = p - vec3(0.0, 0.1 + n * 0.66, 0.0);
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.03, 0.0), 0.64, 0.03), WHITE + ROUND));
  r = U(r, M(sdCone(t - vec3(0.0, 0.18, 0.0), 0.12, 0.6, 0.26), LIME + SOLID));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.33, 0.0), 0.14, 0.04), WHITE + ROUND));
  r = U(r, M(sdCylinder(t - vec3(0.0, 0.4, 0.0), 0.05, 0.03), GRAPHITE + ROUND));
  vec3 po = polarY(t, 14.0) - vec3(0.66, 0.16, 0.0);
  r = U(r, M(sdBox(po, vec3(0.01, 0.1, 0.01)), GRAPHITE));
  r = U(r, M(sdTorus(t - vec3(0.0, 0.26, 0.0), 0.66, 0.012), GRAPHITE + SOLID));
  return r;
}

vec2 shutter(vec2 o, vec3 q, vec2 s, float open) {
  vec2 frame = M(extrude(max(sdRect(q.xz, s + 0.05, 0.0), -sdRect(q.xz, s, 0.0)), q.y - 0.02, 0.02), STEEL);
  if (open > 0.5) {
    o = Sc(o, M(max(sdRect(q.xz, s, 0.0), -(q.y + 0.7)), GRAPHITE));
  } else {
    o = inset(o, q, s, 0.0, 0.015, WHITE);
    if (sdRect(q.xz, s, 0.0) < 0.0) o = stripes(o, q.z, 0.035);
  }
  o = U(o, frame);
  vec3 hq = q - vec3(0.0, 0.05, s.y + 0.1);
  o = U(o, M(sdBox(hq, vec3(s.x + 0.05, 0.05, 0.06)), STEEL));
  return o;
}

vec2 warehouse(vec3 p) {
  vec3 b = p - vec3(0.0, 1.05, 0.0);
  vec2 r = M(sdBox(b, vec3(1.0, 0.75, 1.2)), WHITE);
  r = paint(r, p.y - 0.45, STEEL);
  if (p.y > 0.45) r = stripes(r, p.x + p.z, 0.2);
  vec3 fz = faceZ(p, 1.2);
  r = shutter(r, fz - vec3(-0.3, 0.0, 0.85), vec2(0.36, 0.5), 1.0);
  r = shutter(r, fz - vec3(0.52, 0.0, 0.75), vec2(0.27, 0.4), 0.0);
  vec3 fx = faceX(p, 1.0);
  r = shutter(r, fx - vec3(0.55, 0.0, 0.75), vec2(0.3, 0.4), 0.0);
  r = shutter(r, fx - vec3(-0.3, 0.0, 0.75), vec2(0.3, 0.4), 0.0);
  vec3 rf = vec3(p.z, p.y - 1.8, p.x);
  vec2 roof = M(sdPrism(rf, 1.32, 0.45, 1.06), WHITE);
  roof = stripes(roof, p.x, 0.12);
  float sk = max(abs(abs(p.x) - 0.55) - 0.16, abs(p.z - 0.62) - 0.22);
  roof = paint(roof, max(sk, -p.z), SKY);
  r = U(r, roof);
  r = U(r, M(sdBox(p - vec3(0.0, 1.81, 0.0), vec3(1.06, 0.02, 1.32)), STEEL));
  vec3 vq = p - vec3(-0.45, 2.3, 0.0);
  r = U(r, M(sdCylinder(vq, 0.09, 0.08), STEEL + ROUND));
  r = U(r, M(sdCylinder(vq - vec3(0.0, 0.1, 0.0), 0.13, 0.02), GRAPHITE + ROUND));
  vec3 lp = p - vec3(0.95, 1.55, 1.25);
  r = U(r, M(sdBox(lp, vec3(0.08, 0.02, 0.06)), GRAPHITE));
  r = U(r, M(max(sdSphere(lp - vec3(0.0, -0.02, 0.0), 0.05), lp.y + 0.02), SUN + SOLID));
  return r;
}

vec2 office(vec3 p) {
  vec3 b = p - vec3(0.0, 0.42, 0.0);
  vec2 r = M(sdBox(b, vec3(0.55, 0.42, 0.42)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  vec3 fz = faceZ(p, 0.42);
  vec3 wq = vec3(repC(fz.x - 0.12, 0.3, 2.0), fz.y, fz.z - 0.5);
  r = inset(r, wq, vec2(0.11, 0.13), 0.02, 0.025, SKY);
  if (sdRect(wq.xz, vec2(0.11, 0.13), 0.02) < 0.0) r = seam(r, wq.x);
  vec3 dq = fz - vec3(-0.33, 0.0, 0.33);
  r = inset(r, dq, vec2(0.11, 0.2), 0.02, 0.03, CORAL);
  r = U(r, M(sdSphere(dq - vec3(0.07, 0.0, 0.0), 0.02), GRAPHITE + SOLID));
  vec3 fx = faceX(p, 0.55);
  vec3 xq = vec3(repC(fx.x, 0.3, 2.0), fx.y, fx.z - 0.5);
  r = inset(r, xq, vec2(0.11, 0.13), 0.02, 0.025, SKY);
  if (sdRect(xq.xz, vec2(0.11, 0.13), 0.02) < 0.0) r = seam(r, xq.x);
  r = U(r, M(sdBox(p - vec3(0.0, 0.86, 0.0), vec3(0.6, 0.03, 0.47)), GRAPHITE));
  vec3 ac = p - vec3(-0.2, 0.99, -0.1);
  vec2 unit = M(sdBox(ac, vec3(0.16, 0.1, 0.13)), WHITE);
  vec3 af = faceZ(ac, 0.13);
  unit = Sc(unit, M(max(length(af.xz) - 0.075, -(af.y + 0.02)), GRAPHITE + SOLID));
  r = U(r, unit);
  vec3 an = p - vec3(0.35, 0.89, -0.2);
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.3, 0.0), 0.012, 0.3), GRAPHITE + SOLID));
  r = U(r, M(sdBox(an - vec3(0.12, 0.52, 0.0), vec3(0.11, 0.07, 0.006)), LIME));
  vec3 st = p - vec3(-0.33, 0.0, 0.52);
  r = U(r, M(sdBox(st - vec3(0.0, 0.03, 0.0), vec3(0.16, 0.03, 0.1)), STEEL));
  return r;
}

vec2 crate(vec3 p, float s) {
  vec3 q = p - vec3(0.0, s, 0.0);
  vec2 c = M(sdBox(q, vec3(s)), PAPER);
  c = stripes(c, q.y + s, s * 0.34);
  vec3 a = abs(q);
  c = paint(c, s - 0.035 - max(min(a.x, a.z), a.y), PAPER);
  return c;
}

vec2 lamp(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.08, 0.02), GRAPHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.5, 0.0), 0.02, 0.5), GRAPHITE + SOLID));
  r = U(r, M(pipe(p, vec3(0.0, 0.98, 0.0), vec3(-0.16, 0.98, -0.16), 0.018), GRAPHITE + SOLID));
  vec3 h = p - vec3(-0.2, 0.95, -0.2);
  r = U(r, M(sdCone(h, 0.04, 0.11, 0.04), STEEL + SOLID));
  r = U(r, M(max(sdSphere(h + vec3(0.0, 0.03, 0.0), 0.07), h.y + 0.03), SUN + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(-2.0, -0.07, 2.5), vec3(3.4, 0.1, 1.9), r.x)) r = U(r, water(p));
  if (near(p - vec3(-0.2, 0.1, -3.1), vec3(5.25, 0.25, 3.75), r.x)) r = U(r, land(p));
  vec3 bq = vec3(repC(p.x + 2.4, 2.2, 3.0), p.y - 0.3, p.z - 0.47);
  if (near(p - vec3(-2.4, 0.4, 0.47), vec3(2.4, 0.1, 0.15), r.x)) r = U(r, bollard(bq));
  if (near(p - vec3(-2.4, 0.9, 1.5), vec3(1.8, 0.9, 0.6), r.x)) r = U(r, barge(p - vec3(-2.4, 0.0, 1.5), 0.0, 5.0));
  if (near(p - vec3(-1.9, 0.9, 3.4), vec3(1.8, 0.9, 0.6), r.x)) r = U(r, barge(p - vec3(-1.9, 0.0, 3.4), 1.0, -1.0));
  if (near(p - vec3(0.3, 0.5, 3.4), vec3(0.55, 0.5, 0.35), r.x)) r = U(r, tug(p - vec3(0.3, 0.0, 3.4)));
  if (near(p - vec3(-4.6, 1.3, 3.7), vec3(0.6, 1.35, 0.6), r.x)) r = U(r, lighthouse(p - vec3(-4.6, 0.0, 3.7)));
  float rope = min(sdCapsule(p, vec3(-4.6, 0.46, 0.47), vec3(-4.05, 0.38, 1.1), 0.012), sdCapsule(p, vec3(-0.2, 0.46, 0.47), vec3(-0.75, 0.38, 1.1), 0.012));
  r = U(r, M(rope, GRAPHITE + SOLID));
  if (near(p - vec3(-1.75, 0.4, -0.5), vec3(3.6, 0.1, 0.35), r.x)) r = U(r, track(p - vec3(-5.35, 0.3, -0.5), 7.15));
  vec3 aq = p - vec3(1.8, 0.3, -1.5);
  if (near(aq - vec3(0.6, 0.05, 0.6), vec3(0.65, 0.1, 0.65), r.x)) r = U(r, arc(aq, 1.0));
  if (near(p - vec3(2.8, 0.4, -2.15), vec3(0.35, 0.1, 0.7), r.x)) r = U(r, track(vec3(-1.5 - p.z, p.y - 0.3, p.x - 2.8), 1.2));
  vec3 wq = p - vec3(0.0, 0.3, -0.5);
  if (near(wq - vec3(-2.5, 0.4, 0.0), vec3(3.2, 0.45, 0.3), r.x)) {
    for (int i = 0; i < 5; i++) {
      float fi = float(i);
      vec3 q = wq - vec3(-4.9 + 1.2 * fi, 0.0, 0.0);
      if (sdBox(q - vec3(0.0, 0.4, 0.0), vec3(0.66, 0.45, 0.3)) > r.x) continue;
      r = U(r, wagon(q, fi == 1.0 ? WHITE : fi == 0.0 ? MINT : fi == 4.0 ? SUN : -1.0));
    }
  }
  vec3 dw = p - vec3(2.507, 0.3, -0.793);
  if (near(dw - vec3(0.0, 0.4, 0.0), vec3(0.6, 0.45, 0.6), r.x)) r = U(r, wagon(vec3(dot(dw.xz, vec2(0.70710678, -0.70710678)), dw.y, dot(dw.xz, vec2(0.70710678))), CORAL));
  vec3 iw = p - vec3(2.8, 0.3, -2.4);
  if (near(iw - vec3(0.0, 0.4, 0.0), vec3(0.35, 0.45, 0.7), r.x)) r = U(r, wagon(vec3(iw.z, iw.y, iw.x), LILAC));
  if (near(p - vec3(-3.46, 1.4, -0.3), vec3(0.7, 1.2, 3.0), r.x)) r = U(r, crane(p - vec3(-3.46, 0.3, -0.5), 1.775, 0.35, -1.0));
  if (near(p - vec3(-1.34, 1.4, -0.3), vec3(0.7, 1.2, 3.0), r.x)) r = U(r, crane(p - vec3(-1.34, 0.3, -0.5), 1.1, 0.6, SKY));
  if (near(p - vec3(3.1, 1.3, -3.8), vec3(1.15, 1.2, 1.4), r.x)) r = U(r, warehouse(p - vec3(3.1, 0.3, -3.8)));
  if (near(p - vec3(3.2, 1.8, -5.9), vec3(0.8, 1.8, 0.8), r.x)) r = U(r, silo(p - vec3(3.2, 0.3, -5.9), 4.0));
  if (near(p - vec3(4.3, 0.9, -0.9), vec3(0.65, 0.9, 0.6), r.x)) r = U(r, office(p - vec3(4.3, 0.3, -0.9)));
  if (near(p - vec3(3.6, 0.6, 0.15), vec3(0.65, 0.35, 0.4), r.x)) {
    r = U(r, crate(p - vec3(3.3, 0.3, 0.3), 0.17));
    r = U(r, crate(p - vec3(3.68, 0.3, 0.27), 0.17));
    r = U(r, crate(p - vec3(3.48, 0.64, 0.29), 0.17));
    r = U(r, crate(rotY(p - vec3(4.0, 0.3, -0.05), 0.3), 0.14));
  }
  if (near(p - vec3(1.95, 0.9, 0.3), vec3(0.3, 0.7, 0.3), r.x)) r = U(r, lamp(p - vec3(1.95, 0.3, 0.3)));
  if (near(p - vec3(4.75, 0.9, -2.0), vec3(0.3, 0.7, 0.3), r.x)) r = U(r, lamp(p - vec3(4.75, 0.3, -2.0)));
  return r;
}
