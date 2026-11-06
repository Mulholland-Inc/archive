#pragma frame {"set":["family"],"label":"Duplex","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(m, V(.38,.48,0), V(.36,.48,.42), .01, PALE));
  r = U(r, M(sdPrism(m - V(.38,.96,0), .44, .36, .46), TONE));
  r = U(r, box(m, V(.38,.16,.42), V(.08,.16,.02), .01, DARK));
  r = U(r, box(p, V(0,.48,.42), V(.012,.46,.012), .006, DARK));
  return r;
}
