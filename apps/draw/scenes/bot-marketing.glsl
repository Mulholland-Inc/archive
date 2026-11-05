#pragma frame {"set":["business"],"label":"Marketing agency","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec3 c = rotZ(p - V(-.08,1.14,0), -1.8);
  r = U(r, cone(c, V(0,.12,0), .42, .13, .46, PALE));
  r = U(r, M(sdCylinder(c - V(0,.55,0), .44, .012), DARK));
  r = U(r, can(c, V(0,-.34,0), .12, .08, TONE));
  r = U(r, stick(p, V(-.1,.98,0), V(-.2,.66,0), .06, TONE));
  return r;
}
