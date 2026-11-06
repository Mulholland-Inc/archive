#pragma frame {"set":["business"],"label":"Bookshop","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .56, .42));
  r = U(r, awning(p, .44, .68, .42));
  r = U(r, box(mirX(p), V(0.38,0.22,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .28, .42));
  vec3 l = rotZ(mirX(p) - V(.34,1.12,0), -.26);
  r = U(r, box(l, V(0,0,0), V(.36,.36,.05), .02, PALE));
  r = U(r, box(l, V(0,.18,.05), V(.22,.018,.012), .01, TONE));
  r = U(r, box(l, V(0,.06,.05), V(.22,.018,.012), .01, TONE));
  r = U(r, box(l, V(0,-.06,.05), V(.22,.018,.012), .01, TONE));
  return r;
}
