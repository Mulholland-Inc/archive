#pragma frame {"size":[320,320],"center":[0.0,0.92,0.0],"zoom":104,"pitch":14,"trim":false,"bounds":[[-1.5,-0.1,-1.5],[1.5,2.4,1.5]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // Steps, and the long wings either side of the centre.
  vec2 r = M(sdBox(p - vec3(0.0, 0.05, 0.0), vec3(1.12, 0.05, 0.56)), WHITE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.4, 0.0), vec3(1.0, 0.3, 0.42)), WHITE + SOLID));
  r = U(r, M(sdBox(p - vec3(0.0, 0.74, 0.0), vec3(1.04, 0.04, 0.46)), WHITE + SOLID));
  // The drum, ringed with columns, under the dome.
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.92, 0.0), 0.42, 0.15), WHITE + SOLID));
  vec3 c = polarY(p, 16.0);
  r = U(r, M(sdCylinder(c - vec3(0.4, 1.2, 0.0), 0.02, 0.15), WHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.2, 0.0), 0.33, 0.15), GRAPHITE));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.38, 0.0), 0.44, 0.03), WHITE + SOLID));
  float dome = max(sdEllipsoid(p - vec3(0.0, 1.41, 0.0), vec3(0.36, 0.5, 0.36)), 1.41 - p.y);
  r = U(r, M(dome, WHITE));
  // The lantern and its point.
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.98, 0.0), 0.07, 0.08), WHITE + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 2.06, 0.0), 0.2, 0.08, 0.01), WHITE + SOLID));
  // Eyes in the portico, with a row of tall windows along each wing.
  r = U(r, M(sdRoundBox(m - vec3(0.16, 0.42, 0.42), vec3(0.07, 0.11, 0.025), 0.05), GLOW + SOLID));
  for (int i = 0; i < 3; i++) {
    float x = 0.46 + float(i) * 0.19;
    r = U(r, M(sdBox(m - vec3(x, 0.4, 0.42), vec3(0.035, 0.13, 0.012)), GRAPHITE));
  }
  return r;
}
