#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec2 r = M(max(sdEllipsoid(p - vec3(0.0, 0.0, 0.0), vec3(0.62, 0.62, 0.42)), -p.y), SKY + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.62, 0.0), 0.13, 0.1), WHITE + ROUND));
  r = U(r, M(sdSphere(p - vec3(0.0, 0.92, 0.0), 0.3), WHITE + SOLID));
  return r;
}
