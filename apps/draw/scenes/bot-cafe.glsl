#pragma frame {"set":["business"],"label":"Cafe","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .58, .42));
  r = U(r, awning(p, .46, .68, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, can(p, V(-.06,1.08,0), .4, .34, PALE));
  r = U(r, can(p, V(-.06,.72,0), .5, .03, TONE));
  r = U(r, ringZ(p, V(.4,1.08,0), .17, .05, PALE));
  r = U(r, stick(p, V(-.2,1.5,0), V(-.12,1.74,0), .035, MID));
  r = U(r, stick(p, V(.08,1.5,0), V(.14,1.78,0), .035, MID));
  return r;
}
