#pragma frame {"set":["business"],"label":"Florist","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, awning(p, .44, .66, .42));
  r = U(r, box(mirX(p), V(0.38,0.22,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .28, .42));
  vec3 c = polarY((p - V(0,1.3,0)).xzy, 7.);
  r = U(r, M(sdEllipsoid(c - V(.36,0,0), V(.21,.07,.15)), PALE));
  r = U(r, canZ(p, V(0,1.3,0), .2, .09, TONE));
  r = U(r, stick(p, V(0,.7,0), V(0,1.2,0), .04, TONE));
  return r;
}
