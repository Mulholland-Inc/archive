#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec3 b = p - vec3(0.0, 0.4, 0.0);
  float bowl = max(max(sdSphere(b, 0.72), -sdSphere(b, 0.66)), b.y);
  bowl = max(bowl, -(b.y + 0.62));
  vec2 r = M(bowl, SKY + SOLID);
  r = U(r, M(sdCylinder(b, 0.66, 0.005) + 0.04, PAPER + ROUND));
  r = U(r, M(sdCapsule(p, vec3(-0.2, 0.3, 0.1), vec3(0.45, 1.05, -0.2), 0.05), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(-0.15, 0.36, 0.08), 0.14), GRAPHITE + SOLID));
  return r;
}
