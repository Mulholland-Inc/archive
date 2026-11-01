#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(0.82, 0.08, 0.52), 0.04), GRAPHITE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.165, -0.05), vec3(0.62, 0.01, 0.3)), WHITE + SOLID));
  vec3 m = mirX(p);
  r = U(r, M(sdCylinder(m - vec3(0.3, 0.18, -0.05), 0.13, 0.02), STEEL + ROUND));
  r = U(r, M(sdCylinder(m - vec3(0.3, 0.2, -0.05), 0.05, 0.02), GRAPHITE + ROUND));
  return r;
}
