#pragma frame {"size":[320,320],"center":[0.0,0.92,0.0],"zoom":118,"pitch":18,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,2.3,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  float d = sdEllipsoid(p - vec3(0.0, 0.9, 0.0), vec3(0.5, 0.8, 0.5));
  // Rows of overlapping scales, each row turned half a step from the last.
  for (int i = 0; i < 8; i++) {
    float y = 0.24 + float(i) * 0.2;
    float k = sqrt(max(0.0, 1.0 - pow((y - 0.9) / 0.82, 2.0)));
    vec3 q = polarY(rotY(p, float(i) * 0.26), 9.0);
    d = smin(d, sdEllipsoid(q - vec3(0.5 * k + 0.02, y, 0.0), vec3(0.16, 0.09, 0.15)), 0.015);
  }
  vec2 r = M(d, WHITE);
  r = U(r, M(sdCapsule(p, vec3(0.0, 1.66, 0.0), vec3(0.0, 1.84, 0.0), 0.06), WHITE + SOLID));
  r = U(r, M(sdRoundBox(m - vec3(0.17, 0.9, 0.68), vec3(0.075, 0.12, 0.08), 0.05), GLOW + SOLID));
  return r;
}
