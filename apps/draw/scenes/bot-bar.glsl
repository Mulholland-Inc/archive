#pragma frame {"set":["business"],"label":"Bar","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .62, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .34, .42));
  r = U(r, cone(p, V(0,1.46,0), .3, .03, .56, PALE));
  r = U(r, stick(p, V(0,.7,0), V(0,1.2,0), .035, PALE));
  r = U(r, can(p, V(0,.72,0), .26, .025, PALE));
  r = U(r, ball(p, V(.2,1.72,.1), .08, DARK));
  r = U(r, stick(p, V(.1,1.5,.06), V(.44,1.92,.06), .018, DARK));
  return r;
}
