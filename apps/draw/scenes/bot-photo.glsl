#pragma frame {"set":["business"],"label":"Photo studio","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(p, V(0,1.08,0), V(.6,.36,.2), .05, PALE));
  r = U(r, box(p, V(-.28,1.5,0), V(.14,.07,.14), .03, TONE));
  r = U(r, canZ(p, V(0,1.06,.2), .27, .06, TONE));
  r = U(r, canZ(p, V(0,1.06,.27), .17, .03, DARK));
  r = U(r, box(mirX(p), V(.46,1.36,.2), V(.07,.04,.012), .01, DARK));
  return r;
}
