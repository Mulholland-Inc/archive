#pragma frame {"size":[1200,720],"center":[0.0,0.65,0.0],"zoom":200,"bounds":[[-2.2,-0.1,-1.2],[2.2,2.0,1.2]]}

const float B = 0.1;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

float ringRect(vec2 p, vec2 b, float r, float w) {
  return abs(sdRect(p, b, r)) - w;
}

float entity(vec2 p, vec2 b, float rows) {
  float d = ringRect(p, b, 0.02, 0.008);
  d = min(d, sdRect(p - vec2(0.0, b.y - 0.045), vec2(b.x, 0.045), 0.02));
  for (int i = 0; i < 4; i++) {
    if (float(i) >= rows) break;
    float y = b.y - 0.14 - 0.085 * float(i);
    d = min(d, sdRect(p - vec2(-b.x + 0.06, y), vec2(0.022), 0.006));
    d = min(d, bar(p, -b.x + 0.11, y, b.x * 2.0 - 0.2 - 0.06 * mod(float(i), 2.0), 0.012));
  }
  return d;
}

vec2 blueprint(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.04, 0.0), vec3(0.2, 0.04, 0.52), 0.015), STEEL);
  vec3 b = p - vec3(0.0, 0.58, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.04, 0.5, 0.48), 0.03), LILAC);
  vec3 q = faceX(b, 0.04);
  vec3 f = vec3(-q.x, q.y, q.z);
  float d = ringRect(f.xz, vec2(0.42, 0.44), 0.02, 0.006);
  d = min(d, entity(f.xz - vec2(-0.15, 0.2), vec2(0.2, 0.17), 2.0));
  d = min(d, entity(f.xz - vec2(0.17, -0.22), vec2(0.17, 0.14), 1.0));
  d = min(d, sdSeg(f.xz, vec2(0.05, 0.12), vec2(0.17, 0.12)) - 0.008);
  d = min(d, sdSeg(f.xz, vec2(0.17, 0.12), vec2(0.17, -0.06)) - 0.008);
  d = min(d, sdCircle(f.xz - vec2(0.17, -0.06), 0.022));
  d = min(d, bar(f.xz, -0.33, -0.27, 0.16, 0.012));
  d = min(d, bar(f.xz, -0.33, -0.34, 0.1, 0.012));
  body = decal(body, f, d, WHITE);
  r = U(r, body);
  r = U(r, screw(faceY(p, 0.08) - vec3(0.12, 0.0, 0.0), 0.022));
  vec3 cl = vec3(p.x, p.y - 1.08, abs(p.z) - 0.3);
  r = U(r, M(sdRoundBox(cl, vec3(0.07, 0.035, 0.05), 0.012), GRAPHITE));
  return r;
}


vec2 base(vec3 p) {
  vec3 b = p - vec3(0.0, B * 0.5, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(1.75, B * 0.5, 0.75), 0.03), WHITE);
  r = seam(r, p.y - 0.035);
  r = paint(r, p.y - 0.035, STEEL);
  vec3 fz = faceZ(b, 0.75);
  r = vents(r, fz - vec3(1.1, 0.0, -0.01), 6.0, 0.07, 0.03, 0.024, GRAPHITE);
  vec3 cs = mirXZ(faceY(b, B * 0.5));
  r = U(r, screw(vec3(cs.x - 1.65, cs.y, cs.z - 0.65), 0.022));
  return r;
}

vec2 console(vec3 p) {
  vec3 c = p - vec3(0.0, 0.26, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.5, 0.26, 0.42), 0.03), WHITE);
  r = seam(r, c.y + 0.16);
  r = paint(r, c.y + 0.16, STEEL);
  vec3 fx = faceX(c, 0.5);
  r = vents(r, fx - vec3(0.0, 0.0, 0.0), 6.0, 0.07, 0.14, 0.026, GRAPHITE);
  vec3 fz = faceZ(c, 0.42);
  r = U(r, screw(vec3(abs(fz.x) - 0.4, fz.y, fz.z - 0.12), 0.02));
  r = vents(r, fz - vec3(0.0, 0.0, 0.02), 8.0, 0.07, 0.1, 0.026, GRAPHITE);
  vec3 t = p - vec3(0.0, 0.57, 0.0);
  vec2 top = M(sdRoundBox(t, vec3(0.52, 0.05, 0.44), 0.025), WHITE);
  vec3 tt = faceY(t, 0.05);
  top = inset(top, tt - vec3(-0.22, 0.0, -0.12), vec2(0.2, 0.16), 0.03, 0.015, GRAPHITE);
  vec3 sc = tt - vec3(-0.22, -0.015, -0.12);
  top = decal(top, sc, bar(sc.xz, -0.15, -0.09, 0.22, 0.014), LILAC);
  top = decal(top, sc, bar(sc.xz, -0.15, -0.03, 0.14, 0.014), STEEL);
  top = decal(top, sc, bar(sc.xz, -0.15, 0.03, 0.26, 0.014), STEEL);
  top = decal(top, sc, bar(sc.xz, -0.15, 0.09, 0.18, 0.014), LILAC);
  top = U(top, led(tt - vec3(-0.36, 0.0, 0.2), 0.026, LIME));
  top = U(top, led(tt - vec3(-0.27, 0.0, 0.2), 0.026, STEEL));
  top = U(top, led(tt - vec3(-0.18, 0.0, 0.2), 0.026, STEEL));
  vec3 gb = tt - vec3(0.22, 0.0, 0.16);
  vec2 g = M(extrude(max(sdRect(gb.xz, vec2(0.13, 0.13), 0.03), -sdRect(gb.xz, vec2(0.09, 0.09), 0.02)), gb.y - 0.02, 0.02), SUN);
  g = stripes(g, gb.x + gb.z, 0.05);
  top = U(top, g);
  top = U(top, button(gb - vec3(0.0, 0.02, 0.0), 0.065, 0.03, CORAL));
  vec3 lv = tt - vec3(0.22, 0.0, -0.15);
  top = U(top, M(extrude(sdRect(lv.xz, vec2(0.03, 0.1), 0.03), lv.y, 0.012), GRAPHITE));
  vec3 la = rotX(lv, -0.5);
  top = U(top, M(pipe(la, vec3(0.0, 0.0, 0.0), vec3(0.0, 0.26, 0.0), 0.02), STEEL + SOLID));
  top = U(top, M(sdSphere(la - vec3(0.0, 0.29, 0.0), 0.06), LIME + SOLID));
  return U(r, top);
}

vec2 record(vec3 q) {
  vec2 r = M(sdRoundBox(q, vec3(0.19, 0.12, 0.17), 0.025), WHITE);
  r = seam(r, q.y - 0.06);
  vec3 t = faceY(q, 0.12);
  r = decal(r, t, bar(t.xz, -0.12, -0.07, 0.2, 0.016), STEEL);
  r = decal(r, t, bar(t.xz, -0.12, 0.0, 0.13, 0.016), STEEL);
  r = decal(r, t, bar(t.xz, -0.12, 0.07, 0.17, 0.016), STEEL);
  return r;
}

vec2 inspect(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.36, 0.03, 0.32), 0.015), STEEL);
  r = U(r, record(p - vec3(0.0, 0.18, 0.0)));
  vec3 tg = p - vec3(0.2, 0.2, 0.18);
  r = U(r, M(pipe(p, vec3(0.13, 0.24, 0.12), vec3(0.24, 0.16, 0.26), 0.008), GRAPHITE + SOLID));
  vec3 tq = rotY(rotZ(p - vec3(0.3, 0.1, 0.32), 0.2), -0.6);
  vec2 tag = M(sdRoundBox(tq, vec3(0.012, 0.06, 0.1), 0.012), LIME);
  vec3 tf = faceX(tq, 0.012);
  tag = Sc(tag, M(max(length(tf.xz - vec2(-0.06, 0.0)) - 0.014, -(tf.y + 0.03)), GRAPHITE));
  r = U(r, tag);
  vec3 bq = p - vec3(0.42, 0.0, -0.3);
  r = U(r, M(sdRoundCylinder(bq - vec3(0.0, 0.03, 0.0), 0.1, 0.03, 0.01), GRAPHITE + ROUND));
  vec3 j0 = vec3(0.42, 0.06, -0.3), j1 = vec3(0.42, 0.85, -0.3), j2 = vec3(0.17, 0.62, -0.11);
  r = U(r, M(pipe(p, j0, j1, 0.022), STEEL + SOLID));
  r = U(r, M(pipe(p, j1, j2, 0.022), STEEL + SOLID));
  r = U(r, M(sdSphere(p - j1, 0.04), GRAPHITE + SOLID));
  vec3 lq = p - vec3(0.0, 0.62, 0.0);
  r = U(r, M(sdTorus(lq, 0.2, 0.03), WHITE + SOLID));
  r = U(r, M(extrude(length(lq.xz) - 0.18, lq.y, 0.008), LILAC + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  vec3 g = p - vec3(0.0, B, 0.0);
  if (near(g - vec3(-1.1, 0.6, -0.1), vec3(0.3, 0.7, 0.6), r.x)) r = U(r, blueprint(g - vec3(-1.1, 0.0, -0.1)));
  if (near(g - vec3(-0.1, 0.5, 0.0), vec3(0.6, 0.6, 0.55), r.x)) r = U(r, console(g - vec3(-0.1, 0.0, 0.0)));
  if (near(g - vec3(1.05, 0.45, 0.0), vec3(0.55, 0.6, 0.55), r.x)) r = U(r, inspect(g - vec3(1.1, 0.0, 0.05)));
  return r;
}
