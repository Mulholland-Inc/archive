#pragma frame {"set":["business","family"],"label":"Daycare","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(p, V(-.3,.86,0), V(.22,.22,.22), .04, PALE));
  r = U(r, box(p, V(.3,.86,0), V(.22,.22,.22), .04, MID));
  r = U(r, box(p, V(0,1.3,0), V(.22,.22,.22), .04, PALE));
  r = U(r, ball(p, V(-.3,.86,.2), .08, DARK));
  r = U(r, canZ(p, V(.3,.86,.2), .1, .03, DARK));
  r = U(r, M(sdPrism(p - V(0,1.22,.22), .12, .2, .02), DARK));
  return r;
}
