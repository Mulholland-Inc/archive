#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 4; i++) {
    vec3 c = p - vec3(float(i) * 0.03 - 0.05, 0.08 + float(i) * 0.15, float(i) * -0.02);
    r = U(r, M(sdChamferCylinder(c, 0.5, 0.07, 0.03), LILAC + ROUND));
  }
  r = U(r, M(sdCylinder(p - vec3(0.04, 0.6, -0.06), 0.3, 0.012), BLUE + ROUND));
  return r;
}
