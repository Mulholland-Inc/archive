#pragma frame {"size":[800,640],"center":[0.0,0.14,0.0],"zoom":200,"bounds":[[-1.3,-0.05,-1.5],[1.3,1.1,1.5]]}

vec2 bar(vec3 q, vec2 c, vec2 h, float t, float m) {
  vec3 b = q - vec3(c.x, t, c.y);
  return M(sdRoundBox(b, vec3(h.x, t, h.y), min(t, h.y) * 0.9), m);
}

vec2 checkbox(vec3 q, float on) {
  vec3 b = q - vec3(0.0, 0.03, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.1, 0.03, 0.1), 0.015), on > 0.5 ? LIME : WHITE);
  vec3 t = faceY(q, 0.06);
  if (on > 0.5) {
    float tick = min(sdSeg(t.xz, vec2(-0.05, 0.07), vec2(0.0, 0.075)), sdSeg(t.xz, vec2(0.0, 0.075), vec2(-0.05, -0.085))) - 0.017;
    r = U(r, M(extrude(tick, t.y - 0.01, 0.01), GRAPHITE));
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
  r = U(r, flag);
  return r;
}

vec2 page(vec3 p) {
  vec3 q = p;
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.02, 0.0), vec3(0.72, 0.02, 0.95), 0.01), WHITE);
  vec3 t = faceY(q, 0.04);
  r = U(r, bar(t, vec2(-0.2, -0.72), vec2(0.4, 0.075), 0.035, GRAPHITE));
  r = U(r, bar(t, vec2(0.0, -0.48), vec2(0.6, 0.028), 0.012, STEEL));
  r = U(r, bar(t, vec2(-0.1, -0.37), vec2(0.5, 0.028), 0.012, STEEL));
  r = U(r, bar(t, vec2(-0.25, -0.26), vec2(0.35, 0.028), 0.012, STEEL));
  r = U(r, checkbox(t - vec3(-0.5, 0.0, -0.03), 1.0));
  r = U(r, bar(t, vec2(0.0, -0.03), vec2(0.36, 0.028), 0.012, STEEL));
  r = U(r, checkbox(t - vec3(-0.5, 0.0, 0.2), 0.0));
  r = U(r, bar(t, vec2(0.08, 0.2), vec2(0.44, 0.028), 0.012, STEEL));
  r = U(r, bar(t, vec2(-0.56, 0.58), vec2(0.03, 0.2), 0.03, CORAL));
  r = U(r, bar(t, vec2(0.04, 0.47), vec2(0.5, 0.028), 0.012, STEEL));
  r = U(r, bar(t, vec2(-0.06, 0.58), vec2(0.4, 0.028), 0.012, STEEL));
  r = U(r, bar(t, vec2(-0.16, 0.69), vec2(0.3, 0.028), 0.012, STEEL));
  r = U(r, caret(t - vec3(0.42, 0.0, -0.37), LILAC));
  r = U(r, caret(t - vec3(0.54, 0.0, 0.2), LIME));
  return r;
}

vec2 stand(vec3 p) {
  vec3 b = p - vec3(0.0, 0.11, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(1.0, 0.11, 1.24), 0.04), WHITE);
  r = seam(r, b.y + 0.04);
  r = paint(r, b.y + 0.04, STEEL);
  vec3 fx = faceX(p, 1.0);
  r = vents(r, fx - vec3(0.55, 0.0, 0.14), 6.0, 0.07, 0.05, 0.03, GRAPHITE);
  vec3 fz = faceZ(p, 1.24);
  r = vents(r, fz - vec3(-0.6, 0.0, 0.14), 4.0, 0.07, 0.05, 0.03, GRAPHITE);
  r = U(r, led(fz - vec3(0.62, 0.0, 0.14), 0.035, LIME));
  r = U(r, button(fz - vec3(0.78, 0.0, 0.14), 0.035, 0.012, GRAPHITE));
  vec3 top = faceY(p, 0.22);
  r = inset(r, top, vec2(0.84, 1.08), 0.04, 0.025, PAPER);
  vec3 c = mirXZ(top);
  r = U(r, screw(c - vec3(0.92, 0.0, 1.16), 0.03));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = stand(p);
  r = U(r, M(sdBox(p - vec3(0.1, 0.209, 0.08), vec3(0.72, 0.014, 0.95)), WHITE));
  r = U(r, page(p - vec3(0.0, 0.223, 0.0)));
  return r;
}
