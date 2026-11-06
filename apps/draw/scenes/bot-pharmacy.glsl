#pragma frame {"set":["business"],"label":"Pharmacy","group":"health"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .7, .58, .42));
  r = U(r, awning(p, .46, .7, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec3 c = rotZ(p - V(0,1.18,0), .85);
  r = U(r, stick(c, V(0,-.3,0), V(0,0,0), .24, PALE));
  r = U(r, stick(c, V(0,0,0), V(0,.3,0), .24, TONE));
  return r;
}
