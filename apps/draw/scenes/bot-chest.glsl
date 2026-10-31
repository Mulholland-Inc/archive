#pragma frame {"size":[320,320],"center":[0.0,0.62,0.0],"zoom":128,"pitch":20,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.6,1.4]]}
#define GLOW GRAPHITE
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // The body, and a lid rounded front to back.
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.36, 0.0), vec3(0.8, 0.36, 0.46), 0.03), WHITE + SOLID);
  float lid = max(length(vec2(p.z, (p.y - 0.74) * 1.5)) - 0.5, max(abs(p.x) - 0.8, 0.74 - p.y));
  r = U(r, M(lid, WHITE + SOLID));
  // A band where the lid meets the body, and leather straps over the top.
  r = U(r, M(sdBox(p - vec3(0.0, 0.74, 0.0), vec3(0.815, 0.03, 0.475)), WHITE + SOLID));
  float strap = max(abs(m.x - 0.5) - 0.07, max(length(vec2(p.z, (p.y - 0.74) * 1.5)) - 0.525, 0.74 - p.y));
  strap = min(strap, sdBox(m - vec3(0.5, 0.36, 0.0), vec3(0.07, 0.38, 0.485)));
  r = U(r, M(strap, GRAPHITE));
  // Corner guards, a brass lock, and two eyes.
  vec3 z = mirZ(m);
  r = U(r, M(sdBox(z - vec3(0.78, 0.1, 0.44), vec3(0.04, 0.1, 0.04)), STEEL));
  r = U(r, M(sdBox(z - vec3(0.78, 0.62, 0.44), vec3(0.04, 0.1, 0.04)), STEEL));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.7, 0.47), vec3(0.09, 0.1, 0.03), 0.02), STEEL));
  r = U(r, M(sdRoundBox(m - vec3(0.17, 0.4, 0.46), vec3(0.07, 0.11, 0.025), 0.05), GLOW + SOLID));
  return r;
}
