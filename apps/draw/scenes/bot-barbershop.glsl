#pragma frame {"set":["business"],"label":"Barbershop","group":"care"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .62, .58, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, -.16, .1, .3, .42));
  vec3 c = p - V(.68,.9,.2);
  r = U(r, can(c, V(0,0,0), .2, .9, PALE));
  r = U(r, M(max(sdCylinder(c, .205, .9), abs(mod(c.y + c.x * 1.4 + 3., .36) - .18) - .07), TONE));
  r = U(r, ball(c, V(0,.96,0), .2, MID));
  r = U(r, can(c, V(0,-.88,0), .24, .03, TONE));
  return r;
}
