#pragma frame {"size":[320,320],"center":[0.0,1.02,0.0],"zoom":100,"pitch":14,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.6,1.3]]}
#define GLOW GRAPHITE
vec2 tier(vec3 p, float y, float h, float w) {
  return M(sdBox(p - vec3(0.0, y + h, 0.0), vec3(w, h, w)), WHITE + SOLID);
}
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // Art Deco setbacks, each tier a step narrower.
  vec2 r = tier(p, 0.0, 0.42, 0.52);
  r = U(r, tier(p, 0.84, 0.32, 0.42));
  r = U(r, tier(p, 1.48, 0.2, 0.31));
  r = U(r, tier(p, 1.88, 0.09, 0.2));
  r = U(r, M(sdCone(p - vec3(0.0, 2.06, 0.0), 0.28, 0.12, 0.04), WHITE + SOLID));
  r = U(r, M(sdCapsule(p, vec3(0.0, 2.3, 0.0), vec3(0.0, 2.54, 0.0), 0.016), GRAPHITE));
  // Eyes, a door, and a band of windows up each tier.
  r = U(r, M(sdRoundBox(m - vec3(0.16, 0.58, 0.52), vec3(0.07, 0.11, 0.025), 0.05), GLOW + SOLID));
  r = U(r, M(sdBox(p - vec3(0.0, 0.11, 0.52), vec3(0.09, 0.11, 0.02)), GRAPHITE));
  for (int i = 0; i < 3; i++) {
    float x = -0.2 + float(i) * 0.2;
    r = U(r, M(sdBox(p - vec3(x, 1.16, 0.42), vec3(0.04, 0.24, 0.012)), GRAPHITE));
  }
  r = U(r, M(sdBox(m - vec3(0.09, 1.68, 0.31), vec3(0.03, 0.12, 0.012)), GRAPHITE));
  return r;
}
