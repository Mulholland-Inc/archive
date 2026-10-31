#pragma frame {"size":[320,320],"center":[0.0,0.78,0.0],"zoom":118,"pitch":18,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.0,1.3]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // A thick bound book standing up, cover over a block of pages.
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.78, 0.0), vec3(0.56, 0.78, 0.3), 0.04), WHITE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.04, 0.78, 0.0), vec3(0.54, 0.73, 0.235)), STEEL));
  r = U(r, M(sdRoundBox(p - vec3(-0.58, 0.78, 0.0), vec3(0.05, 0.78, 0.3), 0.05), WHITE + SOLID));
  // Raised bands across the spine, and a label on the cover.
  for (int i = 0; i < 4; i++) {
    float y = 0.24 + float(i) * 0.36;
    r = U(r, M(sdRoundBox(p - vec3(-0.6, y, 0.0), vec3(0.05, 0.03, 0.31), 0.02), WHITE + SOLID));
  }
  r = U(r, M(sdBox(p - vec3(0.02, 1.2, 0.3), vec3(0.3, 0.14, 0.01)), SKY));
  r = U(r, M(sdBox(p - vec3(0.02, 1.2, 0.31), vec3(0.2, 0.018, 0.008)), GRAPHITE));
  // Eyes, and a brass clasp across the fore edge.
  r = U(r, M(sdRoundBox(m - vec3(0.17, 0.72, 0.31), vec3(0.07, 0.11, 0.025), 0.05), GLOW + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(0.5, 0.78, 0.31), vec3(0.09, 0.05, 0.02), 0.02), GRAPHITE));
  return r;
}
