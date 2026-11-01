#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.38, 0.0), vec3(0.7, 0.38, 0.5)), PAPER + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.82, 0.0), vec3(0.74, 0.08, 0.54)), WHITE + SOLID));
  vec3 f = p - vec3(0.0, 0.42, 0.5);
  r = U(r, M(sdBox(f, vec3(0.22, 0.12, 0.01)), WHITE + SOLID));
  r = Sc(r, M(sdBox(p - vec3(0.0, 0.82, 0.54), vec3(0.14, 0.03, 0.04)), GRAPHITE));
  return r;
}
