#pragma frame {"set":["personal"],"label":"Hut","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, can(p, V(0,.34,0), .56, .34, PALE));
  r = U(r, cone(p, V(0,.92,0), .3, .74, .12, MID));
  r = U(r, cone(p, V(0,1.3,0), .1, .14, .01, TONE));
  r = U(r, box(p, V(0,.2,.55), V(.11,.2,.03), .04, DARK));
  return r;
}
