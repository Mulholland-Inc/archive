#pragma frame {"set":["family"],"label":"Row houses","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(-.54,.54,0), V(.26,.54,.36), .01, PALE));
  r = U(r, box(p, V(0,.68,0), V(.26,.68,.36), .01, MID));
  r = U(r, box(p, V(.54,.46,0), V(.26,.46,.36), .01, PALE));
  r = U(r, M(sdPrism(p - V(-.54,1.08,0), .29, .22, .4), TONE));
  r = U(r, M(sdPrism(p - V(0,1.36,0), .29, .22, .4), TONE));
  r = U(r, M(sdPrism(p - V(.54,.92,0), .29, .22, .4), TONE));
  r = U(r, box(p, V(0,.15,.37), V(.07,.15,.012), .01, DARK));
  r = U(r, box(p, V(-.54,.15,.37), V(.07,.15,.012), .01, DARK));
  r = U(r, box(p, V(.54,.15,.37), V(.07,.15,.012), .01, DARK));
  return r;
}
