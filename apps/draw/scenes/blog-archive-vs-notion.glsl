#pragma frame {"size":[1400,760],"center":[0.0,0.55,0.0],"zoom":175,"bounds":[[-3.0,-0.1,-3.0],[3.0,2.6,3.0]]}

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

float tick(vec2 p, float s) {
  return min(sdSeg(p, vec2(-0.024, 0.0) * s, vec2(-0.006, -0.02) * s), sdSeg(p, vec2(-0.006, -0.02) * s, vec2(0.026, 0.022) * s)) - 0.008 * s;
}

vec2 checkbox(vec3 q, float on);

vec2 box(vec3 q, float on) {
  float k = 0.45;
  vec2 r = checkbox(q / k, on);
  return vec2(r.x * k, r.y);
}

vec2 leaf(vec3 p, float side) {
  vec2 r = M(sdRoundBox(p, vec3(0.5, 0.018, 0.66), 0.008), WHITE);
  vec3 t = faceY(p, 0.018);
  vec2 s = t.xz;
  if (side < 0.5) {
    r = decal(r, t, sdRect(s - vec2(-0.08, -0.5), vec2(0.3, 0.035), 0.035), GRAPHITE);
    r = decal(r, t, bar(s, -0.38, -0.36, 0.72), STEEL);
    r = decal(r, t, bar(s, -0.38, -0.27, 0.62), STEEL);
    r = decal(r, t, bar(s, -0.38, -0.18, 0.68), STEEL);
    vec3 im = t - vec3(0.0, 0.0, 0.08);
    float fr = sdRect(im.xz, vec2(0.38, 0.13), 0.025);
    r = inset(r, im, vec2(0.38, 0.13), 0.025, 0.01, SKY);
    vec3 fl = im + vec3(0.0, 0.01, 0.0);
    r = decal(r, fl, max(fr, min(sdCircle(fl.xz - vec2(0.16, 0.34), 0.3), sdCircle(fl.xz - vec2(-0.2, 0.4), 0.32))), MINT);
    r = decal(r, fl, sdCircle(fl.xz - vec2(-0.24, -0.04), 0.035), SUN);
    r = decal(r, t, bar(s, -0.38, 0.32, 0.7), STEEL);
    r = decal(r, t, bar(s, -0.38, 0.41, 0.5), STEEL);
    r = decal(r, t, bar(s, -0.38, 0.5, 0.6), STEEL);
  } else {
    r = decal(r, t, sdRect(s - vec2(-0.14, -0.5), vec2(0.24, 0.035), 0.035), GRAPHITE);
    r = U(r, box(t - vec3(-0.34, 0.0, -0.32), 2.0));
    r = decal(r, t, bar(s, -0.24, -0.32, 0.5), STEEL);
    r = U(r, box(t - vec3(-0.34, 0.0, -0.18), 2.0));
    r = decal(r, t, bar(s, -0.24, -0.18, 0.4), STEEL);
    r = U(r, box(t - vec3(-0.34, 0.0, -0.04), 0.0));
    r = decal(r, t, bar(s, -0.24, -0.04, 0.56), STEEL);
    r = U(r, box(t - vec3(-0.34, 0.0, 0.1), 0.0));
    r = decal(r, t, bar(s, -0.24, 0.1, 0.32), STEEL);
    r = decal(r, t, sdRect(s - vec2(-0.37, 0.36), vec2(0.018, 0.14), 0.018), CORAL);
    r = decal(r, t, bar(s, -0.3, 0.27, 0.62), STEEL);
    r = decal(r, t, bar(s, -0.3, 0.36, 0.5), STEEL);
    r = decal(r, t, bar(s, -0.3, 0.45, 0.56), STEEL);
  }
  return r;
}

vec2 notebook(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(1.1, 0.03, 0.72), 0.02), CORAL);
  r = U(r, M(sdRoundBox(p - vec3(-0.55, 0.075, 0.0), vec3(0.53, 0.018, 0.69), 0.006), PAPER));
  r = U(r, M(sdRoundBox(p - vec3(0.55, 0.075, 0.0), vec3(0.53, 0.018, 0.69), 0.006), PAPER));
  r = U(r, leaf(rotZ(p - vec3(-0.56, 0.115, 0.0), -0.03), 0.0));
  r = U(r, leaf(rotZ(p - vec3(0.56, 0.115, 0.0), 0.03), 1.0));
  vec3 s = vec3(p.x, p.y - 0.12, repC(p.z, 0.11, 12.0));
  r = U(r, M(sdTorus(s.xzy, 0.065, 0.012), STEEL + SOLID));
  vec3 rb = p - vec3(0.3, 0.135, 0.68);
  vec2 rib = M(sdBox(rb - vec3(0.0, 0.0, 0.0), vec3(0.04, 0.004, 0.16)), LILAC);
  rib = S(rib, M(sdPrism(rotX(rb - vec3(0.0, 0.0, 0.17), PI * 0.5), 0.04, 0.05, 0.02), LILAC));
  r = U(r, rib);
  return r;
}

vec2 pencil(vec3 p) {
  vec3 q = rotZ(p, PI * 0.5);
  vec2 r = M(sdHexPrism(q, 0.035, 0.42), SUN);
  r = U(r, M(sdCylinder(q - vec3(0.0, 0.46, 0.0), 0.034, 0.045), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(q - vec3(0.0, 0.53, 0.0), 0.033, 0.035, 0.015), PINK + ROUND));
  vec3 c = q + vec3(0.0, 0.5, 0.0);
  r = U(r, M(sdCone(c, 0.08, 0.0, 0.035), PAPER + ROUND));
  r = U(r, M(sdCone(c + vec3(0.0, 0.055, 0.0), 0.025, 0.0, 0.012), GRAPHITE + ROUND));
  return r;
}

vec2 mug(vec3 p) {
  vec3 b = p - vec3(0.0, 0.17, 0.0);
  vec2 r = M(max(sdRoundCylinder(b, 0.14, 0.17, 0.02), -sdCylinder(b - vec3(0.0, 0.05, 0.0), 0.115, 0.17)), WHITE + ROUND);
  r = U(r, M(sdCylinder(b - vec3(0.0, 0.1, 0.0), 0.116, 0.01), GRAPHITE + ROUND));
  r = seam(r, b.y + 0.06);
  r = paint(r, -(b.y + 0.06), STEEL);
  vec3 h = b - vec3(0.15, 0.0, 0.0);
  r = U(r, M(max(sdTorus(h.xzy, 0.075, 0.022), -h.x), WHITE + SOLID));
  return r;
}

vec2 mat(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.04, 0.0), vec3(1.45, 0.04, 1.05), 0.04), WHITE);
  r = seam(r, p.y - 0.045);
  r = paint(r, p.y - 0.03, STEEL);
  vec3 c = mirXZ(faceY(p, 0.08));
  r = U(r, screw(c - vec3(1.34, 0.0, 0.94), 0.026));
  return r;
}

vec2 disk(vec3 p) {
  vec2 d = M(sdRoundCylinder(p, 0.42, 0.14, 0.045), WHITE + ROUND);
  vec3 q = faceX(rotY(p, -PI * 0.25), 0.42);
  d = vents(d, q - vec3(-0.08, 0.0, 0.0), 4.0, 0.05, 0.1, 0.018, GRAPHITE);
  d = U(d, led(q - vec3(0.15, 0.0, 0.03), 0.03, LIME));
  d = U(d, screw(q - vec3(0.15, 0.0, -0.06), 0.018));
  return d;
}

vec2 database(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.52, 0.04, 0.015), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.52, 0.0), 0.38, 0.44), GRAPHITE + ROUND));
  r = U(r, disk(vec3(p.x, repC(p.y - 0.54, 0.33, 3.0), p.z)));
  vec3 t = p - vec3(0.0, 1.0, 0.0);
  r = U(r, M(sdRoundCylinder(t - vec3(0.0, 0.03, 0.0), 0.26, 0.03, 0.012), STEEL + ROUND));
  r = U(r, screw(faceY(polarY(t, 8.0), 0.0) - vec3(0.34, 0.0, 0.0), 0.02));
  vec3 a = t - vec3(0.0, 0.06, 0.0);
  r = U(r, M(sdCylinder(a - vec3(0.0, 0.03, 0.0), 0.05, 0.03), GRAPHITE + ROUND));
  r = U(r, M(max(sdSphere(a - vec3(0.0, 0.06, 0.0), 0.06), -(a.y - 0.06)), LIME + SOLID));
  return r;
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

vec2 record(vec3 p, float m) {
  vec2 c = M(sdBox(p, vec3(0.007, 0.09, 0.17)), m);
  c = U(c, M(sdRoundBox(p - vec3(0.0, 0.1, -0.07), vec3(0.007, 0.028, 0.055), 0.006), m));
  return c;
}

vec2 cabinet(vec3 p) {
  vec3 c = p - vec3(0.0, 0.38, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.25, 0.38, 0.28), 0.022), WHITE);
  r = Sc(r, M(sdBox(c - vec3(0.25, 0.24, 0.0), vec3(0.2, 0.11, 0.23)), GRAPHITE));
  vec3 f = faceX(c, 0.25);
  for (int i = 0; i < 2; i++) {
    vec3 d = f - vec3(0.0, 0.0, -0.03 - 0.24 * float(i));
    r = U(r, plate(d, vec2(0.24, 0.1), 0.018, 0.012, WHITE));
    r = U(r, M(pipe(d, vec3(-0.07, 0.035, 0.045), vec3(0.07, 0.035, 0.045), 0.015), STEEL + SOLID));
    r = U(r, M(sdBox(d - vec3(0.0, 0.012, -0.03), vec3(0.055, 0.008, 0.026)), STEEL));
  }
  vec3 dr = c - vec3(0.35, 0.23, 0.0);
  vec2 drawer = M(max(sdBox(dr, vec3(0.23, 0.1, 0.22)), -sdBox(dr - vec3(0.0, 0.03, 0.0), vec3(0.215, 0.1, 0.205))), WHITE);
  vec3 fr = dr - vec3(0.242, 0.0, 0.0);
  drawer = U(drawer, M(sdRoundBox(fr, vec3(0.013, 0.115, 0.255), 0.011), WHITE));
  drawer = U(drawer, M(pipe(fr, vec3(0.03, 0.035, -0.07), vec3(0.03, 0.035, 0.07), 0.015), STEEL + SOLID));
  drawer = U(drawer, M(pipe(fr, vec3(0.013, 0.035, -0.07), vec3(0.03, 0.035, -0.07), 0.011), STEEL + SOLID));
  drawer = U(drawer, M(pipe(fr, vec3(0.013, 0.035, 0.07), vec3(0.03, 0.035, 0.07), 0.011), STEEL + SOLID));
  drawer = U(drawer, M(sdBox(fr - vec3(0.013, -0.035, 0.0), vec3(0.008, 0.028, 0.055)), STEEL));
  r = U(r, drawer);
  vec3 fq = dr - vec3(-0.16, 0.0, 0.0);
  float i = repLI(fq.x, 0.068, 5.0);
  float lift = i == 2.0 ? 0.12 : 0.0;
  float m = i == 2.0 ? LIME : i == 3.0 ? LILAC : WHITE;
  vec3 fc = vec3(repL(fq.x, 0.068, 5.0), fq.y + 0.03 - lift, fq.z + (i == 2.0 ? 0.0 : mod(i, 2.0) * 0.05 - 0.025));
  r = U(r, record(fc, m));
  return r;
}

vec2 cable(vec3 p, vec3 a, vec3 b, float m) {
  vec3 c = vec3(a.x, a.y, b.z);
  float d = min(pipe(p, a, c, 0.035), pipe(p, c, b, 0.035));
  vec2 r = M(d, m + SOLID);
  r = U(r, M(sdRoundBox(p - a, vec3(0.06, 0.06, 0.06), 0.015), STEEL));
  r = U(r, M(sdRoundBox(p - b, vec3(0.06, 0.06, 0.06), 0.015), STEEL));
  return r;
}

vec2 plinth(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.09, 0.0), vec3(1.2, 0.09, 1.2), 0.05), WHITE);
  r = seam(r, p.y - 0.13);
  r = paint(r, p.y - 0.05, STEEL);
  r = vents(r, faceX(p, 1.2) - vec3(-0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  r = vents(r, faceZ(p, 1.2) - vec3(0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  vec3 lz = faceZ(p, 1.2) - vec3(-0.8, 0.0, 0.095);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.022, LIME));
  r = U(r, screw(faceY(mirXZ(p), 0.18) - vec3(1.09, 0.0, 1.09), 0.026));
  return r;
}

vec2 pbar(vec3 q, vec2 c, vec2 h, float t, float m) {
  vec3 b = q - vec3(c.x, t, c.y);
  return M(sdRoundBox(b, vec3(h.x, t, h.y), min(t, h.y) * 0.9), m);
}

vec2 checkbox(vec3 q, float on) {
  vec3 b = q - vec3(0.0, 0.03, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.1, 0.03, 0.1), 0.015), on > 1.5 ? STEEL : on > 0.5 ? LIME : WHITE);
  vec3 t = faceY(q, 0.06);
  if (on > 0.5) {
    float tk = min(sdSeg(t.xz, vec2(-0.05, 0.07), vec2(0.0, 0.075)), sdSeg(t.xz, vec2(0.0, 0.075), vec2(-0.05, -0.085))) - 0.017;
    r = U(r, M(extrude(tk, t.y - 0.01, 0.01), GRAPHITE));
  } else {
    r = inset(r, t, vec2(0.068, 0.068), 0.012, 0.02, PAPER);
  }
  return r;
}

vec2 doc(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.02, 0.0), vec3(0.72, 0.02, 0.95), 0.01), WHITE);
  vec3 t = faceY(q, 0.04);
  r = U(r, pbar(t, vec2(-0.2, -0.72), vec2(0.4, 0.075), 0.035, GRAPHITE));
  r = U(r, pbar(t, vec2(0.0, -0.48), vec2(0.6, 0.028), 0.012, STEEL));
  r = U(r, pbar(t, vec2(-0.1, -0.37), vec2(0.5, 0.028), 0.012, STEEL));
  r = U(r, pbar(t, vec2(-0.25, -0.26), vec2(0.35, 0.028), 0.012, STEEL));
  r = U(r, checkbox(t - vec3(-0.5, 0.0, -0.03), 1.0));
  r = U(r, pbar(t, vec2(0.0, -0.03), vec2(0.36, 0.028), 0.012, STEEL));
  r = U(r, checkbox(t - vec3(-0.5, 0.0, 0.2), 0.0));
  r = U(r, pbar(t, vec2(0.08, 0.2), vec2(0.44, 0.028), 0.012, STEEL));
  r = U(r, pbar(t, vec2(-0.56, 0.58), vec2(0.03, 0.2), 0.03, CORAL));
  r = U(r, pbar(t, vec2(0.04, 0.47), vec2(0.5, 0.028), 0.012, STEEL));
  r = U(r, pbar(t, vec2(-0.06, 0.58), vec2(0.4, 0.028), 0.012, STEEL));
  r = U(r, pbar(t, vec2(-0.16, 0.69), vec2(0.3, 0.028), 0.012, STEEL));
  return r;
}

vec2 stand(vec3 p) {
  vec3 b = p - vec3(0.0, 0.11, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.86, 0.11, 1.1), 0.04), WHITE);
  r = seam(r, b.y + 0.04);
  r = paint(r, b.y + 0.04, STEEL);
  vec3 fx = faceX(p, 0.86);
  r = vents(r, fx - vec3(0.5, 0.0, 0.14), 5.0, 0.07, 0.05, 0.03, GRAPHITE);
  vec3 fz = faceZ(p, 1.1);
  r = U(r, led(fz - vec3(0.5, 0.0, 0.14), 0.035, LIME));
  r = U(r, button(fz - vec3(0.66, 0.0, 0.14), 0.035, 0.012, GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.06, 0.229, 0.06), vec3(0.72, 0.014, 0.95)), PAPER));
  r = U(r, doc(p - vec3(0.0, 0.243, 0.0)));
  return r;
}

vec2 machine(vec3 p) {
  vec2 r = plinth(p);
  vec3 g = p - vec3(0.0, 0.18, 0.0);
  r = U(r, database(g - vec3(-0.6, 0.0, -0.6)));
  r = U(r, chatPanel(g - vec3(0.5, 0.0, -0.86)));
  r = U(r, cabinet(g - vec3(-0.84, 0.0, 0.55)));
  float s = 0.65;
  vec2 st = stand((g - vec3(0.4, 0.0, 0.44)) / s);
  r = U(r, vec2(st.x * s, st.y));
  r = U(r, cable(g, vec3(-0.6, 0.06, -0.02), vec3(-0.84, 0.06, 0.2), GRAPHITE));
  r = U(r, cable(g, vec3(-0.02, 0.06, -0.6), vec3(0.2, 0.06, -0.62), GRAPHITE));
  r = U(r, cable(g, vec3(-0.24, 0.06, -0.24), vec3(0.02, 0.06, -0.02), GRAPHITE));
  r = U(r, cable(g, vec3(0.62, 0.06, -0.62), vec3(0.62, 0.06, -0.32), LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 a = p - vec3(-1.3, 0.0, 1.3);
  r = U(r, mat(a));
  vec3 n = a - vec3(-0.15, 0.08, -0.05);
  r = U(r, notebook(n));
  r = U(r, pencil(rotY(a - vec3(0.55, 0.115, 0.92), 0.12)));
  r = U(r, mug(a - vec3(1.2, 0.08, -0.7)));
  vec3 b = p - vec3(1.25, 0.0, -1.25);
  if (near(b - vec3(0.0, 1.0, 0.0), vec3(1.3, 1.1, 1.3), r.x)) r = U(r, machine(b));
  return r;
}
