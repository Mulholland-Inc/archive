#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(0.52, 0.06, 0.7), 0.02), SKY + SOLID);
  r = U(r, M(sdBox(p - vec3(0.0, 0.13, 0.04), vec3(0.48, 0.015, 0.62)), WHITE + SOLID));
  for (int i = 0; i < 3; i++) {
    float z = -0.3 + float(i) * 0.24;
    r = U(r, M(sdBox(p - vec3(i == 2 ? -0.1 : 0.0, 0.15, z), vec3(i == 2 ? 0.25 : 0.35, 0.005, 0.03)), STEEL + SOLID));
  }
  for (int i = 0; i < 4; i++) {
    r = U(r, M(sdTorus(rotZ(p - vec3(-0.3 + float(i) * 0.2, 0.14, -0.66), 1.5708), 0.07, 0.022), GRAPHITE));
  }
  return r;
}
