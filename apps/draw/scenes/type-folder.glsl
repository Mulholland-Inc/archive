#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.07, 0.0), vec3(0.8, 0.07, 0.6), 0.02), BLUE + SOLID);
  r = U(r, M(sdRoundBox(p - vec3(-0.45, 0.07, -0.66), vec3(0.3, 0.07, 0.08), 0.02), BLUE + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.18, 0.04), vec3(0.78, 0.04, 0.56), 0.02), LILAC + SOLID));
  return r;
}
