#pragma frame {"size":[800,640],"center":[0.0,0.55,0.0],"zoom":210,"bounds":[[-1.0,-0.2,-1.7],[1.0,1.8,1.8]]}

vec2 plinth(vec3 p, float h, float n, float m) {
  vec3 b = p - vec3(0.0, h * 0.5 + 0.06, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.32, h * 0.5, 0.32), 0.02), m);
  r = seam(r, -(p.y - 0.14));
  vec3 c = p - vec3(0.0, h + 0.06, 0.0);
  vec2 cap = M(sdRoundBox(c, vec3(0.35, 0.035, 0.35), 0.015), m);
  cap = seam(cap, c.y);
  vec3 ct = faceY(c, 0.035);
  cap = inset(cap, ct, vec2(0.27, 0.27), 0.04, 0.01, m);
  r = U(r, cap);
  r = U(r, screw(vec3(abs(ct.x) - 0.31, ct.y, abs(ct.z) - 0.31), 0.014));
  vec3 f = faceX(p, 0.32);
  float top = h + 0.06 - 0.14;
  r = inset(r, f - vec3(0.0, 0.0, top), vec2(0.22, 0.06), 0.03, 0.012, m == WHITE ? STEEL : WHITE);
  vec3 d = f - vec3(0.0, 0.0, top);
  d.x = repC(d.x, 0.09, n);
  r = U(r, M(max(length(d.xz) - 0.025, abs(d.y + 0.006) - 0.008), GRAPHITE + ROUND));
  vec3 fz = faceZ(p, 0.32);
  r = vents(r, fz - vec3(0.0, 0.0, top), 4.0, 0.05, 0.08, 0.02, GRAPHITE);
  vec3 s = vec3(abs(f.x) - 0.25, f.y, f.z - 0.15);
  r = U(r, screw(s, 0.016));
  return r;
}

vec2 stair(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 4; i++) {
    float k = float(i);
    float y = 0.12 + k * 0.12;
    float hh = (y - 0.06) * 0.5;
    vec3 q = p - vec3(0.0, 0.06 + hh, 0.36 + (3.0 - k) * 0.08);
    vec2 st = M(sdBox(q, vec3(0.16, hh, 0.04)), WHITE);
    st = seam(st, q.y - hh + 0.015);
    r = U(r, st);
  }
  return r;
}

vec2 prize(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.13, 0.0), vec3(0.13, 0.13, 0.13), 0.02), WHITE);
  r = seam(r, p.y - 0.2);
  vec3 f = faceX(p, 0.13);
  r = U(r, led(f - vec3(0.0, 0.0, 0.1), 0.025, LIME));
  vec3 t = faceY(p, 0.26);
  r = inset(r, t, vec2(0.08, 0.08), 0.02, 0.01, STEEL);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec2 base = M(sdRoundBox(p - vec3(0.0, 0.03, 0.1), vec3(0.48, 0.03, 1.42), 0.02), WHITE);
  r = U(r, base);
  vec3 bs = vec3(abs(p.x) - 0.4, p.y - 0.06, abs(p.z - 0.1) - 1.34);
  r = U(r, screw(bs, 0.018));
  vec3 q1 = p - vec3(0.0, 0.0, 0.8);
  vec3 q2 = p - vec3(0.0, 0.0, 0.0);
  vec3 q3 = p - vec3(0.0, 0.0, -0.8);
  if (near(q1 - vec3(0.0, 0.3, 0.0), vec3(0.36, 0.3, 0.36), r.x)) r = U(r, plinth(q1, 0.42, 1.0, WHITE));
  if (near(q2 - vec3(0.0, 0.5, 0.0), vec3(0.36, 0.5, 0.36), r.x)) r = U(r, plinth(q2, 0.78, 2.0, WHITE));
  if (near(q3 - vec3(0.0, 0.7, 0.0), vec3(0.36, 0.7, 0.36), r.x)) r = U(r, plinth(q3, 1.18, 3.0, LIME));
  if (near(q3 - vec3(0.05, 1.42, 0.05), vec3(0.16, 0.16, 0.16), r.x)) r = U(r, prize(rotY(q3 - vec3(0.04, 1.275, 0.06), 0.3)));
  if (near(q1 - vec3(0.0, 0.3, 0.55), vec3(0.2, 0.3, 0.2), r.x)) r = U(r, stair(q1));
  vec3 c = rotY(q1 - vec3(0.3, 0.06, 0.62), -0.3);
  if (near(c, vec3(0.08), r.x)) {
    vec2 k = M(sdRoundBox(c - vec3(0.0, 0.065, 0.0), vec3(0.065), 0.012), SUN);
    r = U(r, seam(k, c.y - 0.09));
  }
  return r;
}
