#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdChamferCylinder(p - vec3(0.0, 0.08, 0.0), 0.55, 0.08, 0.03), GRAPHITE + ROUND);
  vec3 h = rotZ(p - vec3(-0.05, 0.55, 0.0), 0.0);
  r = U(r, M(sdChamferCylinder(rotZ(h, 1.5708), 0.17, 0.4, 0.04), SKY + ROUND));
  r = U(r, M(sdCylinder(rotZ(h, 1.5708), 0.2, 0.04), BLUE + ROUND));
  r = U(r, M(sdCapsule(p, vec3(-0.05, 0.55, 0.0), vec3(-0.05, 0.55, 0.95), 0.05), GRAPHITE));
  return r;
}
