#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.05, 0.0), vec3(0.62, 0.05, 0.4)), GRAPHITE + SOLID);
  vec3 q = rotX(p - vec3(0.0, 0.55, 0.0), -0.2);
  r = U(r, M(sdBox(q, vec3(0.56, 0.5, 0.04)), WHITE + SOLID));
  r = U(r, M(sdBox(q - vec3(0.0, 0.38, 0.0), vec3(0.56, 0.12, 0.05)), BLUE + SOLID));
  vec3 m = mirX(q);
  r = U(r, M(sdCylinder(rotX(m - vec3(0.28, 0.52, 0.0), 1.5708), 0.04, 0.1), STEEL + ROUND));
  r = U(r, M(max(sdRect(q.xy - vec2(0.0, -0.08), vec2(0.18, 0.16), 0.03), abs(q.z) - 0.05), SKY + SOLID));
  return r;
}
