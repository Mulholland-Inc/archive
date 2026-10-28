#pragma frame {"size":[800,640],"center":[-0.22,0.5,0.0],"zoom":200,"bounds":[[-2.5,-0.1,-0.8],[2.0,1.3,0.8]]}

const float TY = 0.405;

vec2 segment(vec3 q, float i) {
  float m = i == 7.0 ? LIME : i == 6.0 ? SUN : i == 5.0 ? LILAC : WHITE;
  vec2 r = M(sdRoundBox(q, vec3(0.175, 0.016, 0.28), 0.008), m);
  vec3 t = faceY(q, 0.016);
  vec3 h = vec3(repC(t.x, 0.07, 5.0), t.y, abs(t.z) - 0.235);
  r = Sc(r, M(max(sdRect(h.xz, vec2(0.014, 0.02), 0.006), -(h.y + 0.02)), GRAPHITE));
  float bar = m == WHITE ? STEEL : WHITE;
  r = decal(r, t, sdRect(t.xz - vec2(-0.05, -0.08), vec2(0.09, 0.022), 0.022), bar);
  r = decal(r, t, sdRect(t.xz - vec2(-0.08, 0.0), vec2(0.06, 0.022), 0.022), bar);
  r = decal(r, t, sdCircle(t.xz - vec2(0.08, 0.08), 0.04), bar);
  return r;
}

vec2 bed(vec3 p) {
  vec3 rq = vec3(p.x, p.y - 0.33, abs(p.z) - 0.36);
  vec2 r = M(sdRoundBox(rq, vec3(1.65, 0.06, 0.04), 0.015), WHITE);
  r = seam(r, rq.y + 0.02);
  vec3 fs = faceZ(vec3(p.x, p.y, abs(p.z)), 0.4);
  r = U(r, screw(vec3(repC(fs.x, 0.5, 7.0), fs.y, fs.z - 0.33), 0.014));
  vec3 lq = vec3(repC(p.x, 1.4, 3.0), p.y - 0.14, abs(p.z) - 0.36);
  r = U(r, M(sdBox(lq, vec3(0.03, 0.14, 0.03)), GRAPHITE));
  r = U(r, M(sdRoundBox(lq - vec3(0.0, -0.13, 0.0), vec3(0.07, 0.012, 0.07), 0.006), STEEL));
  vec3 cb = vec3(repC(p.x, 1.4, 3.0), p.y - 0.1, p.z);
  r = U(r, M(sdBox(cb, vec3(0.02, 0.02, 0.36)), GRAPHITE));
  vec3 ro = vec3(repC(p.x, 0.25, 13.0), p.y - 0.34, p.z);
  r = U(r, M(sdCylinder(rotX(ro, PI * 0.5), 0.05, 0.32), STEEL + ROUND));
  return r;
}

vec2 reel(vec3 p) {
  vec3 q = p - vec3(-2.04, 0.6, 0.0);
  vec3 a = rotX(q, PI * 0.5);
  vec2 r = M(sdCylinder(a, 0.21, 0.28), WHITE + ROUND);
  r = U(r, M(sdCylinder(a - vec3(0.0, 0.3, 0.0), 0.3, 0.018), STEEL + ROUND));
  r = U(r, M(sdCylinder(a + vec3(0.0, 0.3, 0.0), 0.3, 0.018), STEEL + ROUND));
  r = U(r, M(sdCylinder(a, 0.05, 0.4), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(a, 0.025, 0.47), STEEL + ROUND));
  vec3 s = vec3(q.x, p.y - 0.3, abs(q.z) - 0.43);
  r = U(r, M(sdBox(s, vec3(0.05, 0.3, 0.03)), WHITE));
  r = U(r, M(sdRoundBox(s - vec3(0.0, -0.29, 0.0), vec3(0.14, 0.012, 0.07), 0.006), STEEL));
  r = U(r, M(sdBox(q - vec3(0.2, -0.198, 0.0), vec3(0.2, 0.01, 0.27)), WHITE));
  return r;
}

vec2 stops(vec3 p) {
  vec3 q = vec3(p.x - 1.72, p.y - 0.36, abs(p.z) - 0.36);
  vec2 r = M(sdRoundBox(q, vec3(0.06, 0.09, 0.06), 0.015), GRAPHITE);
  r = U(r, M(sdRoundBox(vec3(p.x - 1.72, p.y - 0.4, p.z), vec3(0.035, 0.03, 0.34), 0.01), SUN));
  r = stripes(r, p.z + 0.04, 0.08);
  return r;
}

vec2 flag(vec3 p) {
  vec3 q = p - vec3(0.4, 0.39, 0.36);
  vec2 r = M(sdRoundCylinder(q - vec3(0.0, 0.02, 0.0), 0.05, 0.02, 0.008), GRAPHITE + ROUND);
  r = U(r, M(sdCylinder(q - vec3(0.0, 0.3, 0.0), 0.012, 0.3), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(q - vec3(0.0, 0.62, 0.0), 0.03), LIME + SOLID));
  vec3 f = q - vec3(0.0, 0.5, 0.0);
  vec3 fv = vec3(dot(f.xz, vec2(0.70710678, -0.70710678)), f.y, dot(f.xz, vec2(0.70710678, 0.70710678)));
  float tri = max(max(-fv.x, abs(fv.y) - 0.09 + fv.x * 0.45), fv.x - 0.2);
  r = U(r, M(extrude(tri, fv.z, 0.006), LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.2, 0.0), vec3(1.72, 0.22, 0.42), r.x)) r = U(r, bed(p));
  if (near(p - vec3(-0.2, TY, 0.0), vec3(1.42, 0.03, 0.3), r.x)) {
    float i = repLI(p.x + 1.4, 0.4, 7.0);
    r = U(r, segment(vec3(repL(p.x + 1.4, 0.4, 7.0), p.y - TY, p.z), i));
  }
  vec3 n = rotX(rotZ(p - vec3(1.47, TY + 0.2, 0.04), -0.12), 0.06);
  if (near(n, vec3(0.2, 0.04, 0.3), r.x)) r = U(r, segment(n, 7.0));
  if (near(p - vec3(1.72, 0.4, 0.0), vec3(0.1, 0.1, 0.45), r.x)) r = U(r, stops(p));
  if (near(p - vec3(-1.98, 0.5, 0.0), vec3(0.4, 0.5, 0.5), r.x)) r = U(r, reel(p));
  if (near(p - vec3(0.45, 0.7, 0.36), vec3(0.3, 0.35, 0.3), r.x)) r = U(r, flag(p));
  return r;
}
