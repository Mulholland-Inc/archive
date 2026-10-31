#pragma frame {"size":[320,320],"center":[0.0,0.92,0.0],"zoom":118,"pitch":18,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,2.3,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  vec3 z = mirZ(m);
  // The great swollen trunk, wider at the waist than at the top.
  float t = sdEllipsoid(p - vec3(0.0, 0.62, 0.0), vec3(0.52, 0.72, 0.52));
  t = max(t, -p.y + 0.0);
  t = smin(t, sdCapsule(z, vec3(0.1, 1.15, 0.1), vec3(0.62, 1.5, 0.26), 0.07), 0.12);
  t = smin(t, sdCapsule(p, vec3(0.0, 1.2, 0.0), vec3(0.0, 1.66, 0.0), 0.08), 0.12);
  t = smin(t, sdCapsule(z, vec3(0.1, 1.2, 0.1), vec3(0.2, 1.62, 0.5), 0.06), 0.1);
  vec2 r = M(t, WHITE + SOLID);
  // Small flat crowns at the end of each stubby branch.
  float c = sdEllipsoid(z - vec3(0.66, 1.56, 0.28), vec3(0.34, 0.12, 0.3));
  c = min(c, sdEllipsoid(p - vec3(0.0, 1.74, 0.0), vec3(0.42, 0.14, 0.42)));
  c = min(c, sdEllipsoid(z - vec3(0.22, 1.68, 0.56), vec3(0.3, 0.1, 0.26)));
  r = U(r, M(c, WHITE));
  r = U(r, M(sdRoundBox(m - vec3(0.17, 0.72, 0.5), vec3(0.06, 0.095, 0.03), 0.04), GLOW + SOLID));
  return r;
}
