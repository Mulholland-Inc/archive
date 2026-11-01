#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.62, 0.02), PAPER + ROUND);
  vec3 q = p - vec3(0.0, 0.85, 0.0);
  float pin = smin(sdSphere(q, 0.36), sdCone(p - vec3(0.0, 0.42, 0.0), 0.38, 0.02, 0.3), 0.12);
  r = U(r, M(pin, BLUE + SOLID));
  r = U(r, M(sdCylinder(rotX(q - vec3(0.0, 0.0, 0.3), 1.5708), 0.12, 0.08), WHITE + ROUND));
  return r;
}
