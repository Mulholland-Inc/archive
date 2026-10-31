#pragma frame {"size":[320,320],"center":[0.0,0.9,0.0],"zoom":116,"pitch":14,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.1,1.3]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // A keep that tapers as it rises.
  float wall = max(length(p.xz) - (0.62 - 0.1 * p.y), abs(p.y - 0.72) - 0.72);
  vec2 r = M(wall, WHITE + SOLID);
  // Courses of stone: a joint around the wall, and staggered joints between the blocks.
  for (int i = 1; i < 7; i++) {
    float y = float(i) * 0.2;
    float w = 0.62 - 0.1 * y;
    r = U(r, M(sdTorus(p - vec3(0.0, y, 0.0), w, 0.004), GRAPHITE));
    vec3 q = polarY(rotY(p, mod(float(i), 2.0) * 0.17), 9.0);
    r = U(r, M(sdBox(q - vec3(w, y + 0.1, 0.0), vec3(0.006, 0.1, 0.004)), GRAPHITE));
  }
  // Stone corbels carrying a wider parapet, with heavy merlons on top.
  vec3 c = polarY(p, 12.0);
  r = U(r, M(sdBox(c - vec3(0.52, 1.36, 0.0), vec3(0.09, 0.05, 0.06)), WHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.5, 0.0), 0.64, 0.09), WHITE + SOLID));
  vec3 n = polarY(p, 8.0);
  r = U(r, M(sdBox(n - vec3(0.56, 1.73, 0.0), vec3(0.09, 0.14, 0.15)), WHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.55, 0.0), 0.48, 0.09), GRAPHITE));
  // Narrow eyes, a cross-shaped arrow loop and an iron-bound door.
  r = U(r, M(sdBox(m - vec3(0.15, 0.86, 0.5), vec3(0.075, 0.055, 0.04)), GLOW + SOLID));
  r = U(r, M(min(sdBox(p - vec3(0.0, 1.16, 0.47), vec3(0.016, 0.13, 0.04)), sdBox(p - vec3(0.0, 1.2, 0.47), vec3(0.06, 0.016, 0.04))), GRAPHITE));
  float door = min(sdBox(p - vec3(0.0, 0.17, 0.62), vec3(0.12, 0.17, 0.05)), sdCylinder((p - vec3(0.0, 0.34, 0.62)).xzy, 0.12, 0.05));
  r = U(r, M(door, GRAPHITE));
  return r;
}
