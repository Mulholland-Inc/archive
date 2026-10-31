#pragma frame {"size":[320,320],"center":[0.0,0.98,0.0],"zoom":108,"pitch":14,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.5,1.3]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // A round tower, flaring out at the top to carry its battlements.
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.7, 0.0), 0.5, 0.7), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.06, 0.0), 0.56, 0.06), WHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.46, 0.0), 0.62, 0.1), WHITE + SOLID));
  vec3 c = polarY(p, 10.0);
  r = U(r, M(sdBox(c - vec3(0.56, 1.66, 0.0), vec3(0.08, 0.1, 0.11)), WHITE + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.5, 0.0), 0.48, 0.1), GRAPHITE));
  // The flag on its pole.
  r = U(r, M(sdCapsule(p, vec3(0.0, 1.56, 0.0), vec3(0.0, 2.3, 0.0), 0.018), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.19, 2.16, 0.0), vec3(0.18, 0.11, 0.012)), WHITE + SOLID));
  // Eyes, an arrow slit and an arched door.
  r = U(r, M(sdRoundBox(m - vec3(0.15, 0.82, 0.48), vec3(0.07, 0.11, 0.035), 0.05), GLOW + SOLID));
  r = U(r, M(sdBox(p - vec3(0.0, 1.16, 0.49), vec3(0.02, 0.1, 0.02)), GRAPHITE));
  float door = min(sdBox(p - vec3(0.0, 0.17, 0.5), vec3(0.11, 0.17, 0.03)), sdCylinder((p - vec3(0.0, 0.34, 0.5)).xzy, 0.11, 0.03));
  r = U(r, M(door, GRAPHITE));
  return r;
}
