#pragma frame {"size":[1200,720],"center":[0.0,0.45,0.0],"zoom":215,"bounds":[[-2.6,-0.1,-2.6],[2.6,2.4,2.6]]}

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

vec2 caret(vec3 q, float m) {
  vec2 r = M(sdRoundCylinder(q - vec3(0.0, 0.015, 0.0), 0.06, 0.015, 0.006), GRAPHITE + ROUND);
  r = U(r, M(sdCylinder(q - vec3(0.0, 0.26, 0.0), 0.016, 0.24), GRAPHITE + ROUND));
  r = U(r, M(sdSphere(q - vec3(0.0, 0.54, 0.0), 0.075), m + SOLID));
  vec3 f = q - vec3(0.12, 0.38, 0.0);
  vec2 flag = M(sdRoundBox(f, vec3(0.11, 0.055, 0.012), 0.008), m);
  vec3 fq = faceZ(f, 0.012);
  flag = decal(flag, fq, sdRect(fq.xz - vec2(0.01, 0.0), vec2(0.06, 0.012), 0.012), WHITE);
  return U(r, flag);
}

vec2 livePage(vec3 q) {
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
  r = U(r, caret(t - vec3(0.42, 0.0, -0.37), LILAC));
  r = U(r, caret(t - vec3(0.54, 0.0, 0.2), LIME));
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
  r = U(r, livePage(p - vec3(0.0, 0.243, 0.0)));
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

vec2 wire(vec3 p, vec3 a, vec3 b, vec3 c, float m) {
  vec2 r = M(min(pipe(p, a, b, 0.03), pipe(p, b, c, 0.03)), m + SOLID);
  r = U(r, M(sdRoundBox(p - a, vec3(0.055, 0.045, 0.055), 0.012), STEEL));
  r = U(r, M(sdRoundBox(p - c, vec3(0.055, 0.045, 0.055), 0.012), STEEL));
  return r;
}

vec2 op(vec3 p, float m) {
  return M(sdRoundBox(p, vec3(0.065), 0.014), m);
}

vec2 map(vec3 p) {
  float s = 0.85;
  vec2 st = stand(p / s);
  vec2 r = vec2(st.x * s, st.y);
  r = U(r, laptop(p - vec3(-1.6, 0.0, 0.95)));
  r = U(r, chatPanel(p - vec3(1.12, 0.0, -1.3)));
  r = U(r, wire(p, vec3(-1.06, 0.045, 1.12), vec3(-0.3, 0.045, 1.12), vec3(-0.3, 0.045, 0.99), LILAC));
  r = U(r, wire(p, vec3(1.12, 0.045, -1.08), vec3(1.12, 0.045, -0.25), vec3(0.78, 0.045, -0.25), LIME));
  r = U(r, op(p - vec3(-0.78, 0.11, 1.12), LILAC));
  r = U(r, op(rotY(p - vec3(-0.56, 0.11, 1.12), 0.35), WHITE));
  r = U(r, op(p - vec3(1.12, 0.11, -0.78), LIME));
  r = U(r, op(rotY(p - vec3(1.12, 0.11, -0.55), -0.3), WHITE));
  return r;
}
