#pragma frame {"size":[800,640],"center":[0.0,0.84,0.05],"zoom":245,"bounds":[[-1.0,-0.05,-0.9],[1.0,1.75,1.1]]}

#define TILT 0.5

vec2 bar(vec2 o, vec3 q, vec2 c, vec2 h, float m) {
  return decal(o, q, sdRect(q.xz - c, h, min(h.x, h.y)), m);
}

vec2 leftPage(vec3 q) {
  vec2 r = M(sdBox(q, vec3(0.32, 0.03, 0.43)), WHITE);
  if (abs(q.x) > 0.315) r = stripes(r, q.y + 0.03, 0.012);
  vec3 t = faceY(q, 0.03);
  r = bar(r, t, vec2(-0.08, -0.33), vec2(0.18, 0.03), GRAPHITE);
  r = bar(r, t, vec2(0.0, -0.22), vec2(0.26, 0.014), STEEL);
  r = bar(r, t, vec2(-0.04, -0.16), vec2(0.22, 0.014), STEEL);
  r = bar(r, t, vec2(-0.08, -0.1), vec2(0.18, 0.014), STEEL);
  r = decal(r, t, sdRect(t.xz - vec2(0.0, 0.08), vec2(0.26, 0.12), 0.02), GRAPHITE);
  r = bar(r, t, vec2(-0.1, 0.02), vec2(0.12, 0.012), LIME);
  r = bar(r, t, vec2(-0.04, 0.07), vec2(0.17, 0.012), WHITE);
  r = bar(r, t, vec2(-0.06, 0.12), vec2(0.14, 0.012), LILAC);
  r = bar(r, t, vec2(-0.13, 0.17), vec2(0.07, 0.012), WHITE);
  r = bar(r, t, vec2(0.0, 0.29), vec2(0.26, 0.014), STEEL);
  r = bar(r, t, vec2(-0.06, 0.35), vec2(0.2, 0.014), STEEL);
  return r;
}

vec2 rightPage(vec3 q) {
  vec2 r = M(sdBox(q, vec3(0.32, 0.03, 0.43)), WHITE);
  if (abs(q.x) > 0.315) r = stripes(r, q.y + 0.03, 0.012);
  vec3 t = faceY(q, 0.03);
  r = bar(r, t, vec2(-0.06, -0.33), vec2(0.2, 0.014), STEEL);
  r = bar(r, t, vec2(0.0, -0.27), vec2(0.26, 0.014), STEEL);
  r = bar(r, t, vec2(-0.08, -0.21), vec2(0.18, 0.014), STEEL);
  vec3 d = t - vec3(0.04, 0.0, -0.02);
  r = inset(r, d, vec2(0.24, 0.11), 0.02, 0.01, PAPER);
  r = U(r, M(sdRoundBox(d - vec3(-0.15, 0.02, 0.0), vec3(0.05, 0.03, 0.05), 0.01), WHITE));
  r = U(r, M(sdRoundBox(d - vec3(0.15, 0.02, 0.0), vec3(0.05, 0.03, 0.05), 0.01), LIME));
  r = U(r, M(sdRoundBox(d - vec3(0.0, 0.003, 0.0), vec3(0.1, 0.008, 0.008), 0.004), STEEL));
  r = bar(r, t, vec2(0.0, 0.17), vec2(0.26, 0.014), STEEL);
  r = bar(r, t, vec2(-0.04, 0.23), vec2(0.22, 0.014), STEEL);
  r = bar(r, t, vec2(-0.1, 0.29), vec2(0.16, 0.014), STEEL);
  r = bar(r, t, vec2(0.0, 0.35), vec2(0.26, 0.014), STEEL);
  return r;
}

vec2 manual(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.01, 0.0), vec3(0.72, 0.012, 0.46), 0.01), STEEL);
  r = U(r, leftPage(q - vec3(-0.37, 0.052, 0.0)));
  r = U(r, rightPage(q - vec3(0.37, 0.052, 0.0)));
  vec3 rq = vec3(q.x, q.y - 0.06, repC(q.z, 0.075, 11.0));
  r = U(r, M(sdTorus(rq.xzy, 0.065, 0.011), GRAPHITE + SOLID));
  vec3 tq = q - vec3(0.7, 0.05, 0.0);
  r = U(r, M(sdRoundBox(tq - vec3(0.0, 0.0, -0.26), vec3(0.05, 0.008, 0.045), 0.006), LILAC));
  r = U(r, M(sdRoundBox(tq - vec3(0.0, 0.0, -0.12), vec3(0.05, 0.008, 0.045), 0.006), LIME));
  r = U(r, M(sdRoundBox(tq - vec3(0.0, 0.0, 0.02), vec3(0.05, 0.008, 0.045), 0.006), LILAC));
  vec3 bq = q - vec3(0.105, 0.087, 0.0);
  r = U(r, M(sdBox(bq - vec3(0.0, 0.0, 0.14), vec3(0.025, 0.004, 0.46)), CORAL));
  return r;
}

vec2 lectern(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.46, 0.05), WHITE + ROUND);
  r = seam(r, p.y - 0.035);
  r = paint(r, p.y - 0.035, STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.12, 0.0), 0.2, 0.02), STEEL + ROUND));
  r = U(r, screw(faceY(polarY(p, 6.0), 0.1) - vec3(0.38, 0.0, 0.0), 0.024));
  vec3 c = p - vec3(0.0, 0.58, 0.0);
  r = U(r, M(sdBox(c, vec3(0.07, 0.46, 0.07)), WHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.38, 0.0), vec3(0.085, 0.015, 0.085)), STEEL));
  vec3 cf = faceZ(p, 0.07);
  r = vents(r, vec3(cf.x, cf.y, repC(cf.z - 0.72, 0.06, 3.0)), 3.0, 0.04, 0.03, 0.022, GRAPHITE);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = lectern(p);
  vec3 q = rotX(p - vec3(0.0, 1.13, 0.0), TILT);
  vec2 board = M(sdRoundBox(q, vec3(0.8, 0.035, 0.52), 0.015), WHITE);
  board = seam(board, q.y);
  board = U(board, M(sdRoundBox(q - vec3(0.0, 0.05, 0.53), vec3(0.8, 0.06, 0.025), 0.01), WHITE));
  board = U(board, M(sdBox(q - vec3(0.0, -0.08, 0.0), vec3(0.12, 0.05, 0.3)), STEEL));
  r = U(r, board);
  r = U(r, manual(q - vec3(0.0, 0.035, -0.02)));
  vec3 rb = p - vec3(0.105, 0.77, 0.575);
  float rib = max(sdBox(rb, vec3(0.025, 0.2, 0.004)), -sdBox(rotZ(rb - vec3(0.0, -0.2, 0.0), PI * 0.25), vec3(0.018, 0.018, 0.01)));
  r = U(r, M(rib, CORAL));
  return r;
}
