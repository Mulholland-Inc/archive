#pragma frame {"size":[512,512],"center":[0.0,0.9,0.0],"zoom":190,"pitch":30,"bounds":[[-0.8,-0.1,-0.8],[0.8,1.9,0.8]]}
vec2 map(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.9, 0.0), vec3(0.5, 0.9, 0.45)), STEEL + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 1.83, 0.0), vec3(0.52, 0.04, 0.47)), PAPER + SOLID));
  float y = repC(p.y - 0.84, 0.26, 6.0);
  vec3 q = vec3(p.x, y, p.z - 0.45);
  r = U(r, M(sdBox(q, vec3(0.44, 0.1, 0.025)), WHITE + SOLID));
  r = U(r, M(sdBox(vec3(repC(q.x + 0.14, 0.07, 4.0), q.y, q.z - 0.025), vec3(0.018, 0.055, 0.01)), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(q - vec3(0.32, 0.0, 0.025), 0.03), LIME + SOLID));
  return r;
}
