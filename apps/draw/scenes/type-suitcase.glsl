#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.45, 0.0), vec3(0.7, 0.45, 0.28), 0.1), SKY + SOLID);
  vec3 m = mirX(p);
  r = U(r, M(sdBox(m - vec3(0.36, 0.45, 0.0), vec3(0.06, 0.455, 0.29)), BLUE + SOLID));
  float h = max(sdTorus(rotX(p - vec3(0.0, 0.92, 0.0), 1.5708), 0.18, 0.045), 0.92 - p.y);
  r = U(r, M(h, GRAPHITE));
  return r;
}
