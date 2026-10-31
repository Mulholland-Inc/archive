#pragma frame {"size":[320,320],"center":[0.0,0.7,0.0],"zoom":136,"pitch":16,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.8,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  // One clean pyramid, split by a hairline like the Archive mark.
  vec3 a = abs(p);
  float d = max(max(a.x, a.z) * 1.1 + (p.y - 1.5) * 0.6, -p.y);
  vec2 r = M(d, WHITE + SOLID);
  r = U(r, M(max(d - 0.006, abs(p.x - 0.26 * (1.5 - p.y) / 1.5) - 0.007), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(-0.24, 0.42, 0.58), vec3(0.06, 0.1, 0.06), 0.04), GLOW + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(-0.03, 0.42, 0.58), vec3(0.06, 0.1, 0.06), 0.04), GLOW + SOLID));
  return r;
}
