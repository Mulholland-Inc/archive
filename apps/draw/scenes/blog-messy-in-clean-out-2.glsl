#pragma frame {"size":[1200,720],"center":[0.0,0.6,0.0],"zoom":200,"bounds":[[-2.0,-0.1,-1.0],[2.0,1.9,1.0]]}

const float B = 0.1;

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 card(vec3 p) {
  vec2 c = M(sdRoundBox(p, vec3(0.13, 0.016, 0.17), 0.008), WHITE);
  vec3 q = faceY(p, 0.016);
  c = decal(c, q, sdRect(q.xz - vec2(-0.07, 0.0), vec2(0.025, 0.13), 0.02), LIME);
  c = decal(c, q, bar(q.xz, -0.02, -0.08, 0.12, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, -0.02, 0.08, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, 0.04, 0.12, 0.014), STEEL);
  c = decal(c, q, bar(q.xz, -0.02, 0.1, 0.06, 0.014), STEEL);
  return c;
}

vec2 slip(vec3 p, vec2 size, float m, float k) {
  vec2 c = M(sdRoundBox(p, vec3(size.x, 0.008, size.y), 0.004), m);
  vec3 q = faceY(p, 0.008);
  float b = m == WHITE ? STEEL : WHITE;
  float s = 0.6 + 0.4 * fract(k * 0.37);
  c = decal(c, q, bar(q.xz, -size.x + 0.04, -size.y * 0.5, size.x * 1.4 * s, 0.012), b);
  c = decal(c, q, bar(q.xz, -size.x + 0.04, 0.0, size.x * 0.9, 0.012), b);
  c = decal(c, q, bar(q.xz, -size.x + 0.04, size.y * 0.5, size.x * 1.2 * (1.4 - s), 0.012), b);
  return c;
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(-0.15, B * 0.5, -0.05);
  vec2 r = M(sdRoundBox(b, vec3(1.5, B * 0.5, 0.7), 0.03), WHITE);
  r = seam(r, p.y - 0.035);
  r = paint(r, p.y - 0.035, STEEL);
  vec3 cs = mirXZ(faceY(b, B * 0.5));
  r = U(r, screw(vec3(cs.x - 1.4, cs.y, cs.z - 0.6), 0.022));
  return r;
}

vec2 table(vec3 p) {
  vec3 t = p - vec3(0.0, 0.32, 0.0);
  vec2 r = M(sdRoundBox(t, vec3(0.5, 0.03, 0.42), 0.015), STEEL);
  r = seam(r, t.y - 0.012);
  vec3 l = vec3(abs(p.x) - 0.42, p.y - 0.15, abs(p.z) - 0.34);
  r = U(r, M(sdBox(l, vec3(0.025, 0.15, 0.025)), GRAPHITE));
  vec3 s = p - vec3(0.0, 0.358, 0.0);
  r = U(r, slip(rotY(s - vec3(-0.22, 0.0, -0.18), 0.5), vec2(0.15, 0.1), SUN, 1.0));
  r = U(r, slip(rotY(s - vec3(0.18, 0.0, -0.16), -0.3), vec2(0.1, 0.17), WHITE, 2.0));
  r = U(r, slip(rotY(s - vec3(-0.2, 0.0, 0.2), -0.8), vec2(0.12, 0.12), PINK, 3.0));
  r = U(r, slip(rotY(s - vec3(0.2, 0.0, 0.2), 0.9), vec2(0.17, 0.08), CORAL, 4.0));
  r = U(r, slip(rotX(rotY(s - vec3(0.02, 0.04, 0.02), 0.25), 0.1), vec2(0.13, 0.15), BLUE, 5.0));
  r = U(r, slip(rotZ(rotY(s - vec3(-0.05, 0.075, -0.02), -0.6), -0.08), vec2(0.09, 0.13), WHITE, 6.0));
  return r;
}

vec2 press(vec3 p) {
  vec3 a = p - vec3(0.0, 0.17, 0.0);
  vec2 r = M(sdRoundBox(a, vec3(0.36, 0.17, 0.36), 0.025), WHITE);
  r = seam(r, a.y - 0.1);
  vec3 fz = faceZ(a, 0.36);
  r = vents(r, fz - vec3(0.0, 0.0, -0.04), 6.0, 0.07, 0.1, 0.026, GRAPHITE);
  vec3 fx = faceX(a, 0.36);
  r = U(r, screw(vec3(abs(fx.x) - 0.26, fx.y, fx.z + 0.06), 0.02));
  vec3 pl = p - vec3(0.0, 0.355, 0.0);
  r = U(r, M(sdRoundBox(pl, vec3(0.2, 0.015, 0.24), 0.01), STEEL));
  r = U(r, card(pl - vec3(0.0, 0.031, 0.0)));
  vec3 c = p - vec3(0.0, 0.78, -0.5);
  vec2 col = M(sdRoundBox(c, vec3(0.24, 0.78, 0.16), 0.03), WHITE);
  vec3 cx = faceX(c, 0.24);
  vec3 dc = cx - vec3(0.0, 0.0, -0.05);
  col = inset(col, dc, vec2(0.1, 0.1), 0.1, 0.015, STEEL);
  vec3 dl = dc - vec3(0.0, -0.016, 0.0);
  col = decal(col, dl, sdSeg(dl.xz, vec2(0.0), vec2(0.05, 0.05)) - 0.01, GRAPHITE);
  col = decal(col, dl, length(dl.xz) - 0.02, GRAPHITE);
  col = U(col, led(cx - vec3(-0.06, 0.0, 0.14), 0.024, LIME));
  col = U(col, led(cx - vec3(0.06, 0.0, 0.14), 0.024, STEEL));
  col = U(col, button(cx - vec3(0.0, 0.015, -0.24), 0.05, 0.022, STEEL));
  col = vents(col, cx - vec3(0.0, 0.0, -0.45), 4.0, 0.06, 0.14, 0.024, GRAPHITE);
  r = U(r, col);
  vec3 h = p - vec3(0.0, 1.5, -0.2);
  vec2 head = M(sdRoundBox(h, vec3(0.28, 0.17, 0.46), 0.03), WHITE);
  head = seam(head, h.y + 0.08);
  head = paint(head, h.y + 0.08, STEEL);
  vec3 hx = faceX(h, 0.28);
  head = vents(head, hx - vec3(0.15, 0.0, 0.03), 5.0, 0.06, 0.12, 0.024, GRAPHITE);
  head = U(head, screw(vec3(hx.x + 0.32, hx.y, hx.z - 0.06), 0.02));
  r = U(r, head);
  vec3 rm = p - vec3(0.0, 0.0, 0.0);
  r = U(r, M(sdCylinder(rm - vec3(0.0, 1.15, 0.0), 0.075, 0.2), STEEL + ROUND));
  r = U(r, M(sdCylinder(rm - vec3(0.0, 1.25, 0.0), 0.1, 0.03), GRAPHITE + ROUND));
  vec3 st = rm - vec3(0.0, 0.62, 0.0);
  vec2 stamp = M(sdRoundBox(st - vec3(0.0, 0.24, 0.0), vec3(0.17, 0.05, 0.21), 0.015), LIME);
  stamp = seam(stamp, st.y - 0.22);
  r = U(r, stamp);
  r = U(r, M(sdCylinder(st - vec3(0.0, 0.4, 0.0), 0.05, 0.12), GRAPHITE + ROUND));
  vec3 gd = vec3(abs(p.x) - 0.2, p.y - 0.75, p.z + 0.02);
  r = U(r, M(sdCylinder(gd, 0.022, 0.6), STEEL + ROUND));
  return r;
}

vec2 stacks(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int s = 0; s < 2; s++) {
    vec3 q = p - vec3(0.0, 0.0, -0.22 + 0.44 * float(s));
    int n = s == 0 ? 6 : 4;
    for (int i = 0; i < 6; i++) {
      if (i >= n) break;
      r = U(r, card(q - vec3(0.0, 0.017 + 0.034 * float(i), 0.0)));
    }
  }
  vec3 t = p - vec3(0.0, 0.0, 0.0);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  vec3 g = p - vec3(0.0, B, 0.0);
  if (near(g - vec3(-1.05, 0.25, 0.0), vec3(0.55, 0.3, 0.48), r.x)) r = U(r, table(g - vec3(-1.05, 0.0, 0.0)));
  if (near(g - vec3(0.0, 0.85, -0.2), vec3(0.4, 0.9, 0.6), r.x)) r = U(r, press(g));
  if (near(g - vec3(0.9, 0.12, 0.0), vec3(0.2, 0.22, 0.5), r.x)) r = U(r, stacks(g - vec3(0.9, 0.0, 0.0)));
  return r;
}
