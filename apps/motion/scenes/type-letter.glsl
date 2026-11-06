#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(0.8, 0.06, 0.52), 0.02), WHITE + SOLID);
  vec3 q = p - vec3(0.0, 0.125, 0.0);
  float flap = extrude(max(-(q.z + 0.5), (abs(q.x) * 0.75 + q.z - 0.08) * 0.8), q.y, 0.008);
  r = U(r, M(flap, PAPER + SOLID));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.14, 0.0), 0.09, 0.02), BLUE + ROUND));
  return r;
}
