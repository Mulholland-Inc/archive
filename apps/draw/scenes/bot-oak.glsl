#pragma frame {"size":[320,320],"center":[0.0,0.92,0.0],"zoom":118,"pitch":18,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,2.3,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.34, 0.0), 0.2, 0.34, 0.06), WHITE + SOLID);
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.03, 0.0), 0.34, 0.03, 0.03), WHITE + SOLID));
  float c = sdSphere(p - vec3(0.0, 1.32, 0.0), 0.62);
  c = smin(c, sdSphere(m - vec3(0.5, 1.1, 0.1), 0.42), 0.12);
  c = smin(c, sdSphere(p - vec3(0.0, 1.08, 0.42), 0.42), 0.12);
  c = smin(c, sdSphere(p - vec3(0.0, 1.1, -0.42), 0.42), 0.12);
  c = smin(c, sdSphere(m - vec3(0.32, 1.62, 0.0), 0.34), 0.12);
  r = U(r, M(c, WHITE));
  r = U(r, M(sdRoundBox(m - vec3(0.085, 0.42, 0.19), vec3(0.05, 0.09, 0.03), 0.03), GLOW + SOLID));
  return r;
}
