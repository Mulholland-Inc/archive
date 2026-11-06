#pragma frame {"set":["business"],"label":"Courthouse","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.06,.08), V(.82,.06,.56), .01, PALE));
  r = U(r, box(p, V(0,.16,.08), V(.72,.05,.46), .01, MID));
  r = U(r, box(p, V(0,.6,-.08), V(.62,.4,.34), .01, PALE));
  for (int i = 0; i < 6; i++) { r = U(r, can(p, V(-.55 + float(i) * .22,.56,.38), .05, .36, PALE)); }
  r = U(r, box(p, V(0,.98,.08), V(.68,.05,.42), .01, MID));
  r = U(r, M(sdPrism(p - V(0,1.03,.08), .7, .3, .44), PALE));
  return r;
}
