#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.4, 0.0), vec3(0.74, 0.4, 0.26), 0.06), GRAPHITE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.62, 0.0), vec3(0.75, 0.02, 0.27)), BLUE + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.62, 0.27), vec3(0.08, 0.06, 0.02), 0.02), STEEL));
  float h = max(sdTorus(rotX(p - vec3(0.0, 0.82, 0.0), 1.5708), 0.18, 0.045), 0.82 - p.y);
  r = U(r, M(h, GRAPHITE));
  return r;
}
