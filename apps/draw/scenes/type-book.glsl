#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.14, 0.0), vec3(0.58, 0.14, 0.76), 0.03), BLUE + SOLID);
  r = U(r, M(sdBox(p - vec3(0.04, 0.14, 0.0), vec3(0.56, 0.11, 0.73)), WHITE));
  r = U(r, M(sdBox(p - vec3(-0.1, 0.285, -0.2), vec3(0.3, 0.005, 0.12)), WHITE + SOLID));
  return r;
}
