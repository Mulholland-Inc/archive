#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec3 q = p - vec3(-0.1, 0.03, 0.0);
  float zig = abs(fract((q.x + 0.5) * 5.0) - 0.5) * 0.12;
  float sheet = extrude(max(sdRect(q.xz, vec2(0.45, 0.68), 0.0), q.z - 0.6 - zig), q.y, 0.03);
  vec2 r = M(sheet, WHITE + SOLID);
  for (int i = 0; i < 3; i++) {
    float z = -0.4 + float(i) * 0.22;
    r = U(r, M(extrude(sdRect(q.xz - vec2(0.0, z), vec2(0.3, 0.035), 0.03), q.y - 0.04, 0.01), STEEL + SOLID));
  }
  vec3 c = p - vec3(0.45, 0.1, 0.45);
  r = U(r, M(sdChamferCylinder(c, 0.3, 0.07, 0.03), LILAC + ROUND));
  r = U(r, M(sdCylinder(c - vec3(0.0, 0.07, 0.0), 0.18, 0.01), BLUE + ROUND));
  return r;
}
