#pragma frame {"size":[320,320],"center":[0.0,0.92,0.0],"zoom":118,"pitch":18,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,2.3,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.22, 0.0), 0.16, 0.22, 0.03), WHITE + SOLID);
  r = U(r, M(sdCone(p - vec3(0.0, 1.0, 0.0), 0.56, 0.74, 0.34), WHITE));
  r = U(r, M(sdCone(p - vec3(0.0, 1.5, 0.0), 0.44, 0.56, 0.22), WHITE));
  r = U(r, M(sdCone(p - vec3(0.0, 1.92, 0.0), 0.34, 0.36, 0.02), WHITE));
  r = U(r, M(sdRoundBox(m - vec3(0.2, 0.74, 0.66), vec3(0.07, 0.11, 0.05), 0.04), GLOW + SOLID));
  return r;
}
