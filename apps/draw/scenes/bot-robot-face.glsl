#pragma frame {"size":[320,320],"center":[0.0,1.24,0.0],"zoom":212,"pitch":14,"trim":false,"bounds":[[-1.0,0.25,-1.0],[1.0,2.0,1.0]]}
#define GLOW WHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 1.12, 0.0), vec3(0.46, 0.38, 0.32), 0.11), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.13, 0.31), vec3(0.37, 0.23, 0.03), 0.09), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.16, 1.14, 0.33), 0.1), GLOW + SOLID));
  r = U(r, M(sdRoundBox(m - vec3(0.48, 1.12, 0.0), vec3(0.04, 0.11, 0.09), 0.03), STEEL));
  r = U(r, M(sdCapsule(p, vec3(0.0, 1.5, 0.0), vec3(0.0, 1.66, 0.0), 0.022), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 1.71, 0.0), 0.075), GLOW + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.7, 0.0), 0.11, 0.06), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.42, 0.0), vec3(0.42, 0.24, 0.3), 0.06), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.44, 0.29), vec3(0.32, 0.18, 0.03), 0.02), SKY));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.33), vec3(0.11, 0.025, 0.025), 0.02), GRAPHITE));
  return r;
}
