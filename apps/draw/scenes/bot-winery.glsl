#pragma frame {"set":["business"],"label":"Winery","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.28,0), V(.72,.28,.46), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,.56,0), .86, .2, .54), TONE));
  r = U(r, canZ(p, V(0,.22,.46), .2, .06, MID));
  r = U(r, canZ(p, V(0,.22,.52), .13, .015, DARK));
  r = U(r, box(mirX(p), V(.5,.24,.46), V(.12,.13,.012), .01, MID));
  r = U(r, can(p, V(0,1.08,0), .3, .4, PALE));
  r = U(r, can(p, V(0,1.06,.0), .305, .17, TONE));
  r = U(r, cone(p, V(0,1.56,0), .1, .3, .11, PALE));
  r = U(r, can(p, V(0,1.76,0), .11, .16, PALE));
  r = U(r, can(p, V(0,1.94,0), .12, .05, DARK));
  return r;
}
