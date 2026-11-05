#pragma frame {"set":["business"],"label":"Stock exchange","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.64,0), V(.66,.64,.46), .01, PALE));
  for (int i = 0; i < 5; i++) { float h = .16 + float(i) * .11 - mod(float(i), 2.) * .1; r = U(r, box(p, V(-.42 + float(i) * .21,.74 + h,.46), V(.07,h,.012), .01, TONE)); }
  r = U(r, box(p, V(0,.14,.46), V(.12,.14,.012), .01, DARK));
  r = U(r, box(p, V(0,1.3,0), V(.7,.03,.5), .01, MID));
  return r;
}
