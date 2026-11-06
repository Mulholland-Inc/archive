#pragma frame {"set":["business"],"label":"Water tower","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  vec3 c = mirXZ(p);
  r = U(r, stick(c, V(.36,0,.36), V(.3,.96,.3), .03, TONE));
  r = U(r, can(p, V(0,1.16,0), .54, .34, PALE));
  r = U(r, cone(p, V(0,1.62,0), .12, .58, .04, TONE));
  r = U(r, box(p, V(0,.5,.32), V(.32,.012,.012), .006, TONE));
  r = U(r, box(p, V(.32,.5,0), V(.012,.012,.32), .006, TONE));
  return r;
}
